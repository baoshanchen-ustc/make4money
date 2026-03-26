# 请求排查耗时改造 — Code Review 修复报告

**日期**：2026-03-25
**关联 Review**：`docs/reports/2026-03-25-ops-request-latency-review/review-report.md`
**状态**：✅ 全部修复完成，后端测试 100% 通过，前端类型检查通过

---

## 一、修复概览

| 编号 | 级别 | 问题描述 | 修复状态 |
|------|------|----------|----------|
| F1 | 严重 | `createSingle` SQL 缺少 4 个阶段字段，参数数量不一致 | ✅ 已修复 |
| F2 | 严重 | Copilot service 未设置 `upstream_latency_ms`，导致该段数据永远为空 | ✅ 已修复 |
| F3 | 高   | 前端耗时分解总时间与分段口径不一致，百分比可能失真 | ✅ 已修复 |
| F4 | 中   | `auth_latency_ms` 在不同 handler 中含义不同，横向对比失真 | ✅ 已修复 |
| F5 | 中   | 错误请求详情页未展示阶段耗时，运维排查关键场景缺失 | ✅ 已修复 |
| F6 | 低   | `usageLogSelectColumns` 和 `scanUsageLog` 未包含新字段，通用读取路径数据不完整 | ✅ 已修复 |

---

## 二、逐项修复详情

### F1（严重）：createSingle SQL 参数不一致

**问题**：`prepareUsageLogInsert` 已产生 44 个参数，但 `createSingle` 的内联 INSERT SQL 仍为 40 列 / 40 占位符（`$1..$40`），导致单条写入路径（如事务路径、`request_id` 为空路径）在运行时失败。

**受影响测试**：
- `TestUsageLogRepositoryCreateSyncRequestTypeAndLegacyFields` — FAIL
- `TestUsageLogRepositoryCreate_PersistsServiceTier` — FAIL

**修复内容**：

*`backend/internal/repository/usage_log_repo.go`*
- `createSingle()` 内联 SQL 的列列表补充 `auth_latency_ms`, `routing_latency_ms`, `upstream_latency_ms`, `response_latency_ms`
- VALUES 占位符扩展至 `$1..$44`

*`backend/internal/repository/usage_log_repo_request_type_test.go`*
- `TestUsageLogRepositoryCreateSyncRequestTypeAndLegacyFields`：`WithArgs` 补充 4 个 `sqlmock.AnyArg()`
- `TestUsageLogRepositoryCreate_PersistsServiceTier`：`WithArgs` 补充 4 个 `sqlmock.AnyArg()`

---

### F2（严重）：Copilot upstream_latency_ms 永久为空

**问题**：`copilot_gateway_handler.go` 的三个函数（`ChatCompletions`、`Responses`、`Messages`）读取 `OpsUpstreamLatencyMsKey` 来计算响应阶段耗时，但 `copilot_gateway_service.go` 的上游 HTTP 调用处从未调用 `SetOpsLatencyMs`，导致该键永远为空。

**修复内容**：

*`backend/internal/service/copilot_gateway_service.go`*
- `ForwardChatCompletions`（`httpClient.Do` 附近）：添加 `upstreamStart := time.Now()` + `SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, ...)`
- `ForwardResponses`（同上）：同样处理
- `ForwardMessages`（同上）：同样处理

**效果**：Copilot 路径现在可以精确采集"从发出上游请求到收到响应头"的 TTFB 耗时，`response_latency_ms` 也可以正确从 `forwardDuration - upstreamLatency` 中派生。

---

### F3（高）：前端总耗时口径不一致

**问题**：前端用 `duration_ms`（来自 `ForwardResult.Duration`，仅包含 Forward 阶段）作为分段占比分母，但分段数据包含 `auth_latency_ms`（Forward 之前），导致各段占比之和可能超过 100%，Gantt 图视觉失真。

**修复内容**：

*`frontend/src/views/admin/ops/components/OpsLatencyBreakdownCard.vue`*
- 新增 `stageSum` 计算属性：对已知的4个阶段值求和
- `totalMs` 计算属性：优先用 `stageSum`（与分段同口径），无阶段数据时回退到 `duration_ms`
- tokens/s 速率保留使用 `duration_ms`（实际传输时间基准）
- 概要行新增"端到端"标签：当 `duration_ms` 与 `stageSum` 差值 > 50ms 时同时展示两者，帮助运维判断"隐藏开销"

*`frontend/src/i18n/locales/en.ts` / `zh.ts`*
- `latency.total` 改为"分段合计"（Stage total），避免与 `duration_ms` 混淆
- 新增 `latency.e2e`、`latency.e2eDesc` 键，解释端到端耗时的含义

---

### F4（中）：auth_latency_ms 各入口口径不一致

**问题**：
- `gateway_handler.go`：`auth_latency_ms` 包含并发等待（`AcquireUserSlotWithWait`）+ 二次计费检查
- `openai_gateway_handler.go`：`auth_latency_ms` 在并发等待**之前**就打点，未包含等待和二次检查
- `copilot_gateway_handler.go`：`auth_latency_ms` 在 `TryAcquireUserSlot` 之前打点

**修复方案**：统一语义为"从请求进入到获得处理资格（含并发等待 + 二次计费确认）"。

**修复内容**：

*`backend/internal/handler/openai_gateway_handler.go`*（两处）
- Responses 函数：将 `SetOpsLatencyMs(auth)` 移至 `acquireResponsesUserSlot` + `CheckBillingEligibility` **之后**
- Messages 函数：同上

*`backend/internal/handler/copilot_gateway_handler.go`*（三处）
- `ChatCompletions`：将 `SetOpsLatencyMs(auth)` 移至 `TryAcquireUserSlot` **之后**
- `Responses`：同上
- `Messages`：同上

**结果**：所有入口的 `auth_latency_ms` 含义统一，可横向比较。

---

### F5（中）：错误请求详情页无阶段耗时展示

**问题**：错误请求是运维排查的高优先级场景，但 `OpsErrorDetailPanel.vue` 没有展示 `OpsLatencyBreakdownCard`，即使数据已存在于 `OpsErrorDetail` 接口中。

**修复内容**：

*`frontend/src/views/admin/ops/components/OpsLatencyBreakdownCard.vue`*
- Props 类型从 `OpsUsageInspectDetail` 改为内联 `LatencyData` 接口（duck typing），兼容 `OpsUsageInspectDetail` 和 `OpsErrorDetail`

*`frontend/src/views/admin/ops/components/OpsErrorDetailPanel.vue`*
- 导入 `OpsLatencyBreakdownCard`
- 在基本信息字段组之后、请求体之前插入分解卡片，条件：至少一个阶段字段非空时展示

---

### F6（低）：通用读取路径字段缺失

**问题**：`usageLogSelectColumns` 常量和 `scanUsageLog` 函数是所有 `SELECT * FROM usage_logs` 查询的通用基础，但均未包含4个新字段，导致通用读取路径（如 `ListWithFilters`、`GetByID`）拿不到阶段耗时数据。

**修复内容**：

*`backend/internal/repository/usage_log_repo.go`*
- `usageLogSelectColumns`：追加 `auth_latency_ms, routing_latency_ms, upstream_latency_ms, response_latency_ms`（在 `first_token_ms` 之后）
- `scanUsageLog()`：声明4个 `sql.NullInt64` 扫描变量，加入 `Scan()` 参数列表，添加对应的 `if valid { ... }` 映射到 `UsageLog` 结构体

*`backend/internal/repository/usage_log_repo_request_type_test.go`*
- 3个 `scanUsageLog` 的 mock 调用：在 `first_token_ms` 后各插入4个 `sql.NullInt64{}` 占位值

---

## 三、测试验证

```
go test ./internal/handler/... ./internal/service/... ./internal/repository/...
```

结果：

```
ok   github.com/Wei-Shaw/sub2api/internal/handler         18.526s
ok   github.com/Wei-Shaw/sub2api/internal/handler/admin    0.158s
ok   github.com/Wei-Shaw/sub2api/internal/handler/dto      0.018s
ok   github.com/Wei-Shaw/sub2api/internal/service         37.282s
ok   github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2  (cached)
ok   github.com/Wei-Shaw/sub2api/internal/repository       1.553s
```

前端类型检查：
```
pnpm typecheck → Exit: 0
```

---

## 四、修改文件清单

**后端**

| 文件 | 修改性质 |
|------|----------|
| `backend/internal/repository/usage_log_repo.go` | F1+F6：createSingle SQL + selectColumns + scanUsageLog |
| `backend/internal/repository/usage_log_repo_request_type_test.go` | F1+F6：修复 mock 数据参数数量 |
| `backend/internal/service/copilot_gateway_service.go` | F2：3处 ForwardXxx 添加 upstream TTFB 采集 |
| `backend/internal/handler/openai_gateway_handler.go` | F4：2处 auth 打点移至并发等待+计费检查之后 |
| `backend/internal/handler/copilot_gateway_handler.go` | F4：3处 auth 打点移至 slot 获取之后 |

**前端**

| 文件 | 修改性质 |
|------|----------|
| `frontend/src/views/admin/ops/components/OpsLatencyBreakdownCard.vue` | F3+F5：总耗时口径修正、props 类型泛化 |
| `frontend/src/views/admin/ops/components/OpsErrorDetailPanel.vue` | F5：导入并展示分解卡片 |
| `frontend/src/i18n/locales/en.ts` | F3：新增 e2e/e2eDesc 键，total 改为 stage total |
| `frontend/src/i18n/locales/zh.ts` | F3：同上 |

---

## 五、当前耗时字段含义（修复后定义）

| 字段 | 含义 | 起点 | 终点 |
|------|------|------|------|
| `auth_latency_ms` | 认证鉴权阶段（含并发等待） | 请求进入 handler | 并发槽位获取 + 二次计费检查完成 |
| `routing_latency_ms` | 路由选择阶段 | auth 结束 | Forward 调用前一刻 |
| `upstream_latency_ms` | 上游 TTFB | 发出 HTTP 请求 | 收到上游响应头 |
| `response_latency_ms` | 响应传输阶段 | 上游响应头收到 | Forward 返回（响应体传输完成） |
| `duration_ms` | 端到端总耗时（参考值） | Forward 调用开始 | Forward 返回 |

> 注意：`duration_ms` 不包含 `auth` 和 `routing` 阶段，因此"分段合计"（`auth+routing+upstream+response`）与 `duration_ms` 存在差异时，差值代表"Forward 之外的零散开销"。

---

## 六、后续建议（非本次范围）

1. **Antigravity 路径**：`antigravityGatewayService.Forward` 未检查是否也会设置 `OpsUpstreamLatencyMsKey`，建议跟进排查。
2. **WebSocket 路径**：`openai_gateway_handler.go` 的 WS 分支仅设置了 auth latency，其余阶段采集逻辑较复杂，建议单独跟进。
3. **历史数据回填**：迁移前的记录阶段字段全为 NULL，前端已通过"noData"提示兼容；如有运维需求可考虑基于 `duration_ms` 做近似回填。
