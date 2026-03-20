# 更新记录

## 2026-03-21（0.1.115）

- 版本号：`0.1.114` → `0.1.115`。
- **回滚** 0.1.114 对 Sonnet/Opus 4 的粗粒度 id（`claude-sonnet-4` / `claude-opus-4`）：对接真实 GitHub Copilot `/chat/completions` 时，上游会以 **`model_not_supported`** 拒绝该 `model` 字段；恢复为点分 wire id（如 **`claude-sonnet-4.6`**、**`claude-opus-4.6`**），与 Haiku 的 **`claude-haiku-4.5`** 一致。家族简写仍展开为上述带点分 id。

## 2026-03-21（0.1.114）

- 版本号：`0.1.113` → `0.1.114`。
- ~~Copilot 模型归一化：Sonnet/Opus 4 压为 `claude-sonnet-4` / `claude-opus-4`~~（已在 **0.1.115 回滚**，见上）。

## 2026-03-21（0.1.113）

- 版本号：`0.1.112` → `0.1.113`。
- Copilot 模型归一化：识别 Claude Code `settings.json` 中的家族简写（如 **`opus[1m]`**、裸 **`opus` / `sonnet` / `haiku`** 及可选 **`[1m]`**），在转发 GitHub Copilot 前映射为 Anthropic 风格 id 再规范化为 Copilot 点分 wire id；Haiku 为 `claude-haiku-4.5`。

## 2026-03-21（0.1.112）

- 版本号：`0.1.111` → `0.1.112`。
- Copilot 网关：Sonnet/Opus 的 `max_tokens` 上限默认仍为 8192；支持按账号凭证 **`copilot_max_output_tokens`** 覆盖（留空=默认；正整数=自定义上限；**0**=不裁剪，可能再次触发上游 400）。创建/编辑 Copilot 账号表单增加对应配置项与中英文案。

## 2026-03-21（0.1.111）

- 版本号：`0.1.110` → `0.1.111`。
- Copilot 网关：对转发至 GitHub `/chat/completions` 的请求，在 **Sonnet/Opus**（非 Haiku）上将过大的 `max_tokens`（如 Claude Code 默认 32000）**限制为 8192**，避免上游以泛型 HTTP 400 拒绝；`/v1/messages` 与 OpenAI `/chat/completions` 两条路径均生效。
- Copilot 上游非 200 时通过 `setOpsUpstreamError` 写入响应体摘要，便于「请求排查」中「上游错误」不再长期为空。

## 2026-03-20（0.1.110）

- 版本号：`0.1.109` → `0.1.110`。
- Copilot `/v1/messages`：`rewriteCopilotUpstreamModel` 之后增加与 `/chat/completions` 一致的 `mergeConsecutiveSameRoleMessagesInOpenAIBody`，减少上游因相邻同 role 消息返回 400；上游 400 时 WARN 输出 `openai_body_snip`、`upstream_model`、`x_request_id` 便于线上对照。
- 模型归一化单测：补充 `claude-sonnet-4.6`（点分形式）应保持不变的用例。

## 2026-03-20（0.1.109）

- 版本号：`0.1.108` → `0.1.109`。
- 修复 CI（golangci-lint）：`gofmt` 修正 `copilot_gateway_handler.go`、`copilot_gateway_service.go`、`ops_request_details.go`；`account_usage_service.go` OpenAI Codex 探测请求对固定 URL 的 `client.Do` 标注 `gosec` G704 例外；`embed_off` 下 `NewFrontendServer` 存根改为返回 `(*FrontendServer, nil)`，避免与 `embed` 实现混析时触发 staticcheck SA4023（`err != nil` 恒真）。生产带嵌入前端仍仅走 `embed` 构建且 `HasEmbeddedFrontend()` 为真时的真实实现。

## 2026-03-20

- 版本号：`0.1.107` → `0.1.108`（合并上游 Wei-Shaw/sub2api `main` 至 `a225a241`）。
- 合并上游：OpenAI `gpt-5.4-mini` / `gpt-5.4-nano` 定价与模型配置；日/周配额重置后用量展示不再沿用累计旧值；Anthropic→OpenAI 推理级别映射与 Codex 转换；OpenAI 默认模型转发；`UseKeyModal` 配额展示与单测等（对应上游 PR #1172、#1176、#1171、#1162 等）。

- Copilot 计费：`/copilot/v1`、`/copilot` 路由组挂载 `InboundEndpointMiddleware`；异步 `RecordUsage` 之前在请求协程内快照 `inbound`（禁止在 `go func()` 里读 `GetInboundEndpoint(c)`，避免 Gin 回收 Context 后入站路径为空）；上游路径与全局常量对齐为 `/v1/chat/completions`、`/v1/responses`。
- 运维 / 请求排查：左右分栏改为组件内 `scoped` 媒体查询（`grid-template-columns`），避免 Tailwind 任意 `grid-cols-[minmax(...,...)]` 因逗号解析失败而始终单列。
- 运维 / Copilot：`usage_logs` 写入时补齐 `inbound_endpoint`、`upstream_endpoint`（`/chat/completions`、`/responses` 等）及 `ForwardResult.UpstreamModel`（转发体中的 model）；`ops` 请求列表 CTE 中成功行 `status_code` 固定为 `200`。请求排查页自 `md` 断点起稳定左右分栏；详情与列表对成功行无状态码时显示 `200`；usage 卡片「上游模型」在库中未单独存（与客户端相同）时回退为客户端模型。
- 运维：新增管理端「请求排查」页 `/admin/ops/request-inspect`，列表与运维大盘一致的请求明细，右侧失败请求展示 ops 入库的客户端 `request_body` / `request_headers`、上游关联条目的转发请求体；成功请求调用新接口 `GET /admin/ops/usage-inspect` 展示 `usage_logs` 中的 `model` / `upstream_model` / `inbound_endpoint` / `upstream_endpoint` 等元数据（不含原始 JSON body）。侧栏在开启运维监控时增加入口；`/admin/ops` 菜单高亮改为仅精确匹配避免与子路径冲突。
- 修复 CI：`account_handler_available_models_test.go` 中 `NewAccountHandler` 缺少 `CopilotGatewayService` 参数导致单元测试无法编译。
- Copilot 网关：转发前对 `model` 应用账号 `model_mapping`，并将 Anthropic 风格 id（如 `claude-haiku-4-5-20251001`、短横线版）规范为 Copilot API 接受的点分版（如 `claude-haiku-4.5`）；覆盖 `/chat/completions`、`/responses` 与 Anthropic `/v1/messages` 翻译路径。实现见 `backend/internal/pkg/copilot/model_normalize.go` 与 `CopilotGatewayService.rewriteCopilotUpstreamModel`。
