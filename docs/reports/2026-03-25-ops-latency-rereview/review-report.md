# 2026-03-25 请求排查耗时改造二次复审报告

## 结论
cc 的修复已解决上一轮大部分阻断问题（F1/F2/F5/F6 已落地，后端测试与前端 typecheck 均通过），但**仍有关键口径问题未完全闭环**，目前还不能算“全部修复完成”。

## 复审发现（按严重级别）

### [高] R1：`/chat/completions` 路径的 auth/routing 口径仍旧不一致（F4 未完全修复）

**现象**
- OpenAI Chat Completions 路由仍在用 `openai_chat_completions.go`，并且该路径里 `auth_latency_ms` 在并发槽位获取前就打点。
- `routingStart` 也在并发槽位获取前启动，导致路由阶段混入“并发等待+计费检查”。

**证据**
- 路由仍绑定此处理器：`backend/internal/server/routes/gateway.go:77`
- 提前打 auth：`backend/internal/handler/openai_chat_completions.go:87`
- 提前开始 routing：`backend/internal/handler/openai_chat_completions.go:88`
- 并发槽位获取发生在其后：`backend/internal/handler/openai_chat_completions.go:90`
- 计费检查也在其后：`backend/internal/handler/openai_chat_completions.go:98`

**影响**
- 同样是 OpenAI 流量，`/responses` 与 `/chat/completions` 的分段口径不可直接横向比较。
- 运维排查会出现“auth 偏小、routing 偏大”的误判。

---

### [中] R2：前端将 `duration_ms` 标为“端到端”，但后端该字段并非请求全链路耗时

**现象**
- UI 把 `duration_ms` 作为 `End-to-end/端到端` 展示与说明。
- 但后端 `duration_ms` 来源是 Forward 层 `startTime := time.Now()` 到 Forward 返回，不含 handler 前置阶段。

**证据**
- 前端展示：`frontend/src/views/admin/ops/components/OpsLatencyBreakdownCard.vue:25`
- 文案定义：
  - `frontend/src/i18n/locales/zh.ts:4035`
  - `frontend/src/i18n/locales/en.ts:3870`
- 后端计时起点（非请求进入点）：
  - `backend/internal/service/gateway_service.go:4008`
  - `backend/internal/service/openai_gateway_service.go:1645`
  - `backend/internal/service/openai_gateway_chat_completions.go:35`

**影响**
- 页面“端到端”语义与真实数据口径不一致，仍会误导定位。

---

### [中] R3：OpenAI WebSocket ingress 路径未采集 auth/routing/response 阶段耗时，仍不满足“每段都入库”

**现象**
- WS 路径会读取并写入 `AuthLatencyMs/RoutingLatencyMs/UpstreamLatencyMs/ResponseLatencyMs`。
- 但该路径没有看到对应 `SetOpsLatencyMs` 的写入（至少 auth/routing/response 未写）。

**证据**
- WS 记录 usage 时读取阶段键：`backend/internal/handler/openai_gateway_handler.go:1265`
- WS 处理流程中未见 auth/routing 的 `SetOpsLatencyMs` 调用（仅 responses/messages 两个 HTTP 路径有）：`backend/internal/handler/openai_gateway_handler.go:217`, `backend/internal/handler/openai_gateway_handler.go:581`

**影响**
- WS 请求在排查页阶段耗时可能为空，仍有观测盲区。

---

### [低] R4：缺失阶段也会显示 2% 色块，视觉上会被误读为“有耗时”

**现象**
- Gantt 条宽使用 `Math.max(segment.pct, 2)`，即使 `ms=null` 也会渲染 2% 最小宽度。

**证据**
- `frontend/src/views/admin/ops/components/OpsLatencyBreakdownCard.vue:61`
- `frontend/src/views/admin/ops/components/OpsLatencyBreakdownCard.vue:67`

**影响**
- 视觉上像是该阶段存在耗时，不利于精确排查。

---

## 已验证通过项（本轮确认）

- F1：`usage_log_repo.createSingle` 已扩展到 44 列/44 占位符。
  - `backend/internal/repository/usage_log_repo.go:309`
  - `backend/internal/repository/usage_log_repo.go:331`
- F2：Copilot 3 条主路径已设置 `OpsUpstreamLatencyMsKey`。
  - `backend/internal/service/copilot_gateway_service.go:160`
  - `backend/internal/service/copilot_gateway_service.go:681`
  - `backend/internal/service/copilot_gateway_service.go:847`
- F5：错误详情页已接入分段耗时卡片。
  - `frontend/src/views/admin/ops/components/OpsErrorDetailPanel.vue:92`
- F6：`usageLogSelectColumns` 与 `scanUsageLog` 已纳入 4 个新字段。
  - `backend/internal/repository/usage_log_repo.go:31`
  - `backend/internal/repository/usage_log_repo.go:3968`

## 验证命令

```bash
cd backend && go test ./internal/handler/... ./internal/service/... ./internal/repository/...
```

结果：通过（cached）。

```bash
cd frontend && npm run typecheck
```

结果：通过。
