# 2026-03-25 请求排查耗时改造第三轮复审报告

## 结论
cc 本轮提交中，R1/R2/R4 已有效修复；R3 为**部分修复**。当前状态不是“全部完成”，仍有 WS 路径阶段耗时采集不完整的问题。

## 发现（按严重级别）

### [中] F1：WebSocket 路径仅补了 auth/routing，`upstream/response` 仍未采集（R3 部分修复）

**已修复部分**
- `ResponsesWebSocket` 已新增 `wsRequestStart`，并在计费后写入 `auth_latency_ms`。
  - `backend/internal/handler/openai_gateway_handler.go:1030`
  - `backend/internal/handler/openai_gateway_handler.go:1156`
- 已在拿到 token 后写入 `routing_latency_ms`。
  - `backend/internal/handler/openai_gateway_handler.go:1222`

**仍存在问题**
- WS usage 记录时会读取并写入 `upstream_latency_ms` / `response_latency_ms`：
  - `backend/internal/handler/openai_gateway_handler.go:1274`
  - `backend/internal/handler/openai_gateway_handler.go:1275`
- 但 WS 转发链路中未见对应阶段的 `SetOpsLatencyMs` 写入；当前仅写了 WS 连接挑选与排队指标：
  - `backend/internal/service/openai_ws_forwarder.go:1917`
  - `backend/internal/service/openai_ws_forwarder.go:1918`

**影响**
- WS 请求在排查页面中，上游/响应阶段大概率为 NULL，仍不满足“每个阶段都统计入库并展示”的目标。

---

### [低] F2：WS 多轮 turn 记录会复用首轮 auth/routing 指标，粒度偏粗

- 每轮 `AfterTurn` 都会写 usage：
  - `backend/internal/handler/openai_gateway_handler.go:1263`
  - `backend/internal/handler/openai_gateway_handler.go:1276`
- 但 `BeforeTurn` 只重抢并发槽位，没有更新 auth/routing 埋点：
  - `backend/internal/handler/openai_gateway_handler.go:1238`
  - `backend/internal/handler/openai_gateway_handler.go:1261`

**影响**
- 若一条 WS 会话产出多条 usage 记录，后续 turn 的 auth/routing 不是 turn 级真实值，排查精度受限。

## 已验证通过项

### R1（高）已修复
- `/chat/completions` 的 auth/routing 打点位置已调整到并发槽位+计费检查之后。
  - `backend/internal/handler/openai_chat_completions.go:87`
  - `backend/internal/handler/openai_chat_completions.go:95`
  - `backend/internal/handler/openai_chat_completions.go:102`
  - `backend/internal/handler/openai_chat_completions.go:103`

### R2（中）已修复
- 前端文案已从“端到端”改为“转发耗时 / Forward duration”，并明确不含 auth/routing 前置阶段。
  - `frontend/src/i18n/locales/zh.ts:4034`
  - `frontend/src/i18n/locales/zh.ts:4035`
  - `frontend/src/i18n/locales/en.ts:3869`
  - `frontend/src/i18n/locales/en.ts:3870`

### R4（低）已修复
- `ms == null` 时不再渲染进度条色块，消除“虚假 2% 色块”。
  - `frontend/src/views/admin/ops/components/OpsLatencyBreakdownCard.vue:59`

## 复验结果

后端：
```bash
cd backend && go test ./internal/handler/... ./internal/service/... ./internal/repository/...
```
通过（cached）。

前端：
```bash
cd frontend && pnpm typecheck
```
通过。
