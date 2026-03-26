# 请求排查耗时改造二次复审 — 修复报告

**日期**：2026-03-25
**关联复审**：`docs/reports/2026-03-25-ops-latency-rereview/review-report.md`
**状态**：✅ 全部修复完成，后端测试 100% 通过，前端类型检查通过

---

## 一、修复概览

| 编号 | 级别 | 问题描述 | 修复状态 |
|------|------|----------|----------|
| R1 | 高   | `/chat/completions` 路径 auth/routing 口径仍不一致（F4 未完全修复） | ✅ 已修复 |
| R2 | 中   | 前端把 `duration_ms` 标为"端到端"，但该字段实为 Forward 阶段耗时 | ✅ 已修复 |
| R3 | 中   | OpenAI WebSocket ingress 路径未采集 auth/routing 阶段耗时 | ✅ 已修复 |
| R4 | 低   | 阶段缺失时 Gantt 仍显示最小 2% 色块，视觉误导 | ✅ 已修复 |

---

## 二、逐项修复详情

### R1（高）：`/chat/completions` auth/routing 口径不一致

**问题**：`openai_chat_completions.go` 在 `acquireResponsesUserSlot`（并发等待）和 `CheckBillingEligibility`（二次计费检查）**之前**就设置了 `auth_latency_ms` 并启动了 `routingStart`，导致：
- auth 偏小（不含等待时间）
- routing 偏大（包含了等待和计费检查时间）
- 与 `openai_gateway_handler.go` 中 `/v1/responses` 和 `/v1/messages` 路径口径不一致

**修复内容**：

*`backend/internal/handler/openai_chat_completions.go`*
- 将 `service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, ...)` 移至 `acquireResponsesUserSlot` + `CheckBillingEligibility` **之后**
- `routingStart := time.Now()` 紧跟其后
- 添加注释：`// 记录认证鉴权阶段耗时（含并发等待 + 二次计费检查，与 gateway_handler / openai_gateway_handler 同口径）`

**效果**：三条 OpenAI 路径（`/chat/completions`、`/v1/responses`、`/v1/messages`）的 auth/routing 打点口径完全一致，可横向比较。

---

### R2（中）：前端 `duration_ms` 文案不准确

**问题**：前端将 `duration_ms` 展示为"端到端（End-to-end）"，但后端该值来源于 `ForwardResult.Duration`（即 service 层 Forward 函数的 `startTime := time.Now()` 到函数返回），**不包含** handler 前置的 auth / routing 阶段。

**修复内容**：

*`frontend/src/i18n/locales/zh.ts`*
- `latency.e2e`：`端到端` → `转发耗时`
- `latency.e2eDesc`：改为 `Forward 调用阶段总耗时（不含 auth/routing 前置阶段，仅供参考）`

*`frontend/src/i18n/locales/en.ts`*
- `latency.e2e`：`End-to-end` → `Forward duration`
- `latency.e2eDesc`：改为 `Total Forward call duration (excludes auth/routing pre-stages; for reference only)`

**效果**：UI 文案准确反映字段实际含义，运维不会误将该值理解为完整请求链路耗时。

---

### R3（中）：WebSocket ingress 路径无 auth/routing 阶段耗时

**问题**：`ResponsesWebSocket` handler 读取了4个阶段键并传入 `RecordUsage`，但从未写入 `auth_latency_ms` / `routing_latency_ms`，导致 WS 请求在排查页面该阶段数据始终为空。

**修复内容**：

*`backend/internal/handler/openai_gateway_handler.go`*（`ResponsesWebSocket` 函数）
- 函数入口处添加 `wsRequestStart := time.Now()`
- `CheckBillingEligibility` 之后添加：
  ```go
  service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(wsRequestStart).Milliseconds())
  wsRoutingStart := time.Now()
  ```
- `GetAccessToken` 成功之后添加：
  ```go
  service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(wsRoutingStart).Milliseconds())
  ```

**覆盖范围**：
- `auth_latency_ms`：从请求进入到 TryAcquireUserSlot + CheckBillingEligibility 完成
- `routing_latency_ms`：从 auth 结束到账号选择 + 并发槽位 + token 获取完成
- `upstream_latency_ms` / `response_latency_ms`：WS 路径每轮 turn 均可能重连，结构复杂，暂留后续跟进（见后续建议）

---

### R4（低）：Gantt ms=null 阶段显示虚假 2% 色块

**问题**：Gantt 进度条使用 `Math.max(segment.pct, 2)`，即使 `segment.ms == null`（无数据）也会渲染最小 2% 宽度的色块，视觉上误导为"该阶段有耗时"。

**修复内容**：

*`frontend/src/views/admin/ops/components/OpsLatencyBreakdownCard.vue`*
- 进度条 `<div>` 加 `v-if="segment.ms != null"`：ms 为 null 时不渲染色块（轨道背景仍在）
- 数值标注 `<span>` 的 `:class` 和 `:style` 同步加入 `segment.ms != null` 条件，null 时标注从左侧开始显示"—"

**效果**：无数据的阶段只显示"—"文字和灰色空轨道，不再出现虚假色块。

---

## 三、测试验证

```bash
cd backend && go test ./internal/handler/... ./internal/service/... ./internal/repository/...
```

结果：

```
ok   github.com/Wei-Shaw/sub2api/internal/handler         18.497s
ok   github.com/Wei-Shaw/sub2api/internal/handler/admin    (cached)
ok   github.com/Wei-Shaw/sub2api/internal/handler/dto      (cached)
ok   github.com/Wei-Shaw/sub2api/internal/service          (cached)
ok   github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2  (cached)
ok   github.com/Wei-Shaw/sub2api/internal/repository       (cached)
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
| `backend/internal/handler/openai_chat_completions.go` | R1：auth/routing 打点移至并发等待+计费检查之后 |
| `backend/internal/handler/openai_gateway_handler.go` | R3：ResponsesWebSocket 添加 wsRequestStart、auth/routing 打点 |

**前端**

| 文件 | 修改性质 |
|------|----------|
| `frontend/src/views/admin/ops/components/OpsLatencyBreakdownCard.vue` | R4：ms=null 时不渲染色块，标注条件修正 |
| `frontend/src/i18n/locales/zh.ts` | R2：e2e → 转发耗时，e2eDesc 修正 |
| `frontend/src/i18n/locales/en.ts` | R2：End-to-end → Forward duration，e2eDesc 修正 |

---

## 五、修复后各路径采集状况

| 路径 | auth | routing | upstream | response |
|------|------|---------|----------|----------|
| `POST /v1/chat/completions` (openai_chat_completions.go) | ✅ 修复后正确 | ✅ 修复后正确 | ✅ | ✅ |
| `POST /v1/responses` (openai_gateway_handler.go) | ✅ | ✅ | ✅ | ✅ |
| `POST /v1/messages` (openai_gateway_handler.go) | ✅ | ✅ | ✅ | ✅ |
| `GET /v1/responses` WebSocket (openai_gateway_handler.go) | ✅ 本轮新增 | ✅ 本轮新增 | ✅ (ForwardResult) | ⚠️ 见注 |
| `POST /v1/chat/completions` Copilot (copilot_gateway_handler.go) | ✅ | ✅ | ✅ | ✅ |
| `POST /v1/responses` Copilot (copilot_gateway_handler.go) | ✅ | ✅ | ✅ | ✅ |
| `POST /v1/messages` Copilot (copilot_gateway_handler.go) | ✅ | ✅ | ✅ | ✅ |
| `POST /v1/messages` Anthropic direct (gateway_handler.go) | ✅ | ✅ | ✅ | ✅ |

> ⚠️ WS 路径的 `response_latency_ms`：WS 为多轮 turn 结构，每轮独立 forward，response 耗时难以在 per-request 维度聚合，暂不采集，保持 NULL。

---

## 六、后续建议（非本次范围）

1. **WS response_latency_ms**：如需采集，可在 `AfterTurn` hook 中对 `result.Duration` 求和，在最后一轮记录总值，但需注意多轮语义定义。
2. **Antigravity 路径**：`antigravityGatewayService.Forward` 是否设置了 `OpsUpstreamLatencyMsKey` 尚未验证，建议跟进。
3. **历史数据**：迁移前记录阶段字段为 NULL，前端已通过"noData"提示兼容，暂无回填需求。
