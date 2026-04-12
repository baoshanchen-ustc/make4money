# Copilot Session Quota Optimization 方案评审报告

## 基本信息
- 评审日期：2026-04-13
- 评审对象：`docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md`
- 评审类型：实施前方案 review
- 评审方式：静态审阅计划文档，并对照当前 Copilot gateway / usage 记录链路核对实现影响面与风险

## 评审结论（摘要）
- 方案方向明确，目标也清楚：把“按请求计费”进一步压缩成“按 session 首轮计费”。
- 但当前计划里仍有两处阻断级问题，会直接影响安全边界和实际落地范围。
- 当前结论建议为：`REQUEST CHANGES`。

---

## 主要发现（按严重级别）

### HIGH

#### H1. `sessionCache` 只按客户端给的 session key 全局去重，且明确不与 API key 绑定，会造成跨用户/跨租户配额串用

#### 问题
计划把 session cache 设计成 `CopilotGatewayService` 上的进程级全局状态：

- `docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md:345`

session key 的优先级是：
- 先读 `X-Session-ID`
- 再读 body 里的 session 信息
- `docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md:463`

同时文档还明确写了：

- `X-Session-ID` “与 API key 无关，纯客户端侧标识”
- `docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md:848`

这意味着：
- 两个不同用户/不同 API key，只要在同一进程里发送同一个 `X-Session-ID`
- 第二个请求开始就会共享同一个 `sessionCache` entry
- 直接被覆盖成 `agent`

因为当前计划里的 cache key 只是一个裸字符串，没有任何 user / apiKey / group 维度的 namespacing。

#### 风险
- 这是一个真正的计费/配额隔离问题，而不只是“命中率不稳定”。
- 任何客户端都可以故意复用固定的 `X-Session-ID`，让同进程内后续请求持续命中免费 `agent` 配额。
- 即使不是恶意行为，不同调用方若恰好选到相同 session header，也会相互污染。

#### 建议
- cache key 至少要带上租户维度，例如：
  - `api_key_id + ":" + session_key`
  - 或 `user_id + ":" + session_key`
  - 或 `group_id + ":" + session_key`
- 不要把一个完全由客户端控制的 header 值直接当作全局 session identity。

---

#### H2. 计划声称覆盖 `Responses / Codex CLI`，但任务列表没有把真正的 `ForwardResponses` 上游链路接入 session cache，`result.Initiator` 也未明确在该路径填充

#### 问题
文档一开始就把范围写到了：

- ChatCompletions / Responses 从 OpenAI `user` 字段或 `X-Session-ID` 提取 session
- `docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md:7`
- `docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md:41`

但 Task 2 真正要求修改的只有四处 `copilotInitiator` 调用：

- `forwardChatCompletionsDirect`
- `forwardChatCompletionsViaResponses`
- `forwardChatCompletionsViaMessages`
- `ForwardMessages`
- `docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md:420`

没有包含实际的 `ForwardResponses` 路径。

而当前真实上游 `Responses` 请求头是在这里设置的：

- `backend/internal/service/copilot_gateway_service.go:1848`

也就是说，按当前计划：
- `ForwardResponses` 实际发往上游的 `X-Initiator` 不会使用 session cache
- 仍只走现有的 `copilotInitiatorFromResponsesBody(body)` 判定

更进一步，Task 3 又要求 handler 的 Responses analytics 改成：

- `capturedInitiatorResp := result.Initiator`
- `docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md:604`

但 Task 3.2 只写了“四个转发函数”补 `Initiator`，仍未明确包含 `ForwardResponses`：

- `docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md:562`

当前系统里，空字符串 initiator 最终会被规范化成 `"user"`：

- `backend/internal/service/gateway_service.go:7368`

#### 风险
- 计划对 `Codex CLI / Responses` 的覆盖范围被高估了。
- 如果 `ForwardResponses` 没有同步补 session cache 和 `result.Initiator`，Responses 路径的 analytics 仍会继续偏向 `"user"`。
- 最终会出现：文档声称“覆盖所有客户端类型”，但真正生效的只有 chat/messages 相关路径。

#### 建议
- 明确把 `ForwardResponses` 加入修改清单。
- 至少补两件事：
  - 实际上游 `X-Initiator` 是否要接入 session cache，文档必须写清楚
  - `ForwardResponses` 返回的 `CopilotForwardResult.Initiator` 必须被显式填充

---

## 建议的修订方向

1. 先收紧 session cache key
- 给 `X-Session-ID` / 解析出的 session key 加上用户或 API key 维度，避免跨租户串用。

2. 明确 Responses 路径策略
- 如果 Responses 继续只依赖 `previous_response_id`，那就不要在 Goal/Architecture 里写成“Responses 也走 session cache”。
- 如果 Responses 也要纳入 session cache，就把 `ForwardResponses` 的真正上游 header 路径补进任务。

---

## 审阅结论

当前方案最大的风险不是“实现复杂”，而是边界定义过宽：一边把客户端提供的 session key 当作全局身份，一边又把 `Responses` 路径写进了目标范围但没有真正改到上游链路。建议先收紧这两处，再进入实现。
