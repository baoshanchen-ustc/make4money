# 请求排查耗时改造第三轮复审 — 修复报告

**日期**：2026-03-25
**关联复审**：`docs/reports/2026-03-25-ops-latency-rereview-round3/review-report.md`
**状态**：✅ 全部修复完成，后端测试 100% 通过

---

## 一、修复概览

| 编号 | 级别 | 问题描述 | 修复状态 |
|------|------|----------|----------|
| F1 | 中   | WS 路径 upstream/response 阶段耗时未实际采集，排查页大概率为空 | ✅ 已修复 |
| F2 | 低   | WS 多轮 turn 记录复用首轮 auth/routing 指标（粒度偏粗，架构限制） | ✅ 已说明（不修复） |

**已验证通过延续**（本轮不涉及，保持通过状态）：R1、R2、R4。

---

## 二、逐项修复详情

### F1（中）：WS 路径 upstream/response 采集补全

**问题**：`AfterTurn` 闭包中通过 `getContextLatencyMsPtr` 读取 `OpsUpstreamLatencyMsKey` / `OpsResponseLatencyMsKey`，但整个 WS 转发链路从未调用过对应的 `SetOpsLatencyMs`，导致这两个值始终为 nil，`upstream_latency_ms` / `response_latency_ms` 永远不入库。

同时发现，原代码的 `wsAuthLatency`、`wsRoutingLatency`、`wsUpstreamLatency`、`wsResponseLatency` 变量声明及 `h.submitUsageRecordTask` 调用处于 `AfterTurn` 闭包内部，但缩进错误（与外层代码对齐而非内层），造成阅读混乱。

**根本原因分析**：WS 路径是全双工长连接，没有 HTTP 请求-响应的 TTFB 概念。`openai_ws_forwarder.go` 只采集了连接池层面的 `ConnPickDuration` 和 `QueueWaitDuration`，不设置上游耗时键。`result.Duration` 是每轮 turn 的转发总时长（`time.Since(turnStart)`），是最合适的"上游耗时"替代指标。

**修复方案**：

- 不再从 gin context 读取 `OpsUpstreamLatencyMsKey`（WS 链路不写该键）
- 改为在 `AfterTurn` 内直接使用 `intPtr(int(result.Duration.Milliseconds()))` 作为 `UpstreamLatencyMs`
- `ResponseLatencyMs` 设为 nil（WS 全双工无独立响应传输阶段，上游时间即全部转发时间）

**修复文件**：

*`backend/internal/handler/openai_gateway_handler.go`*（`AfterTurn` 闭包）
- 移除 `getContextLatencyMsPtr(c, service.OpsUpstreamLatencyMsKey)` 读取
- 移除 `getContextLatencyMsPtr(c, service.OpsResponseLatencyMsKey)` 读取
- 新增：`turnDurationMs := intPtr(int(result.Duration.Milliseconds()))`，赋给 `UpstreamLatencyMs`
- `ResponseLatencyMs` 设为 nil，附注释说明原因
- 修正所有相关变量的缩进，置于闭包内部正确位置

**修复前后对比**：

| 字段 | 修复前 | 修复后 |
|------|--------|--------|
| `upstream_latency_ms` | 永远 NULL | turn 转发总时长（ms） |
| `response_latency_ms` | 永远 NULL | NULL（有意，附注释） |
| `auth_latency_ms` | ✅ 上轮已修复 | ✅ 不变 |
| `routing_latency_ms` | ✅ 上轮已修复 | ✅ 不变 |

---

### F2（低）：WS 多轮 turn 复用首轮 auth/routing 指标

**问题**：WS 会话可能有多轮 `BeforeTurn` → `AfterTurn` 循环，每轮都写一条 usage 记录。`auth_latency_ms` / `routing_latency_ms` 在首轮连接建立时写入 gin context，后续 turn 读到的是同一个值。

**决定：不修复，记录为架构限制。**

理由：
1. WS 会话的 auth 和 routing 只在连接初始化时发生一次。对同一会话的所有 turn，auth/routing 时间本来就是共享的（会话级指标，非 turn 级指标）。
2. 要支持 per-turn auth/routing，需要修改 `BeforeTurn` 回调在每轮重新打点并覆盖 context，但：
   - `BeforeTurn` 里的并发槽位重新抢占本质上是"排队等待"，不等同于完整的 auth 流程
   - 覆盖 context 值会导致上一轮读取的引用失效（若读取已放入 goroutine closure）
3. 影响范围有限：WS 会话通常只有 1 轮，多轮场景下该字段属于会话级参考值，不影响核心排查能力。

**处置**：在修复报告中记录，后续如有需要可单独评估 per-turn 计时方案。

---

## 三、测试验证

```bash
cd backend && go test ./internal/handler/... ./internal/service/... ./internal/repository/...
```

结果：

```
ok   github.com/Wei-Shaw/sub2api/internal/handler         18.485s
ok   github.com/Wei-Shaw/sub2api/internal/handler/admin    (cached)
ok   github.com/Wei-Shaw/sub2api/internal/handler/dto      (cached)
ok   github.com/Wei-Shaw/sub2api/internal/service          (cached)
ok   github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2  (cached)
ok   github.com/Wei-Shaw/sub2api/internal/repository       (cached)
```

---

## 四、修改文件清单

| 文件 | 修改性质 |
|------|----------|
| `backend/internal/handler/openai_gateway_handler.go` | F1：AfterTurn 改用 result.Duration 派生 upstream_latency_ms；修正缩进；移除无效 context 读取 |

---

## 五、各路径采集完整状况（累计全轮）

| 路径 | auth | routing | upstream | response |
|------|------|---------|----------|----------|
| `POST /v1/chat/completions` (OpenAI) | ✅ | ✅ | ✅ | ✅ |
| `POST /v1/responses` (OpenAI) | ✅ | ✅ | ✅ | ✅ |
| `POST /v1/messages` (OpenAI) | ✅ | ✅ | ✅ | ✅ |
| `GET /v1/responses` WebSocket | ✅ | ✅ | ✅ turn.Duration | nil（有意） |
| `POST /v1/chat/completions` Copilot | ✅ | ✅ | ✅ | ✅ |
| `POST /v1/responses` Copilot | ✅ | ✅ | ✅ | ✅ |
| `POST /v1/messages` Copilot | ✅ | ✅ | ✅ | ✅ |
| `POST /v1/messages` Anthropic direct | ✅ | ✅ | ✅ | ✅ |

> WS 路径 `response_latency_ms` 故意为 nil：WS 全双工无独立传输阶段，`upstream_latency_ms`（turn 总时长）已覆盖该信息。

---

## 六、后续建议（非本次范围）

1. **WS per-turn 精度**：若需要 per-turn auth/routing 粒度，建议在 `BeforeTurn` 中重置一个 turn 级时钟变量并传入 AfterTurn，而非复写 gin context。
2. **Antigravity 路径**：`antigravityGatewayService.Forward` 的 upstream 打点尚未确认，建议跟进。
