# Copilot Sub-Agent Premium Fix 方案评审报告 Round 5

## 基本信息
- 评审日期：2026-04-12
- 评审对象：`docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md`
- 评审类型：实施前方案复审
- 评审方式：静态审阅更新后的计划文档，并对照当前 Copilot handler / validator 实现核对真正可落地性

## 复审结论（摘要）
- 这版计划在“弱门控/强门控”分层和 Responses 统计口径上已经比前一版更成熟，方向是对的。
- 但现在又暴露出 2 个新的阻断级问题：Anthropic Messages 的强门控拿不到 validator 需要的 body/context 信息，且计划中的正向集成测试也没有补齐 `Validate()` 需要的 headers 和 `metadata.user_id`。
- 当前结论仍为：`REQUEST CHANGES`。

---

## 主要发现（按严重级别）

### HIGH

#### H1. `claudeCodeBodyMapForInitiator(c)` 目前拿不到 Copilot handler 的解析 body map，Messages 路径的“强门控”实际会长期返回 `false`

#### 问题
计划现在要求在 Anthropic Messages 路径使用完整校验：

- `isCC := NewClaudeCodeValidator().Validate(c.Request, claudeCodeBodyMapForInitiator(c))`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:581`

并给出了 helper：

- `claudeCodeBodyMapForInitiator(c)`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:588`

但这个 helper 的实现只尝试从 `gin.Context` 读取 `OpenAIParsedRequestBodyKey`，读不到就返回空 map：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:592`
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:598`

问题在于当前仓库里：
- `OpenAIParsedRequestBodyKey` 只在 OpenAI gateway handler 路径被设置
  - `backend/internal/handler/openai_gateway_handler.go:927`
- Copilot handler 并没有设置这个 key
  - 当前 `backend/internal/handler/copilot_gateway_handler.go` 中无对应 `c.Set(...)`
- 主 gateway helper 的上下文缓存逻辑其实还会读 `claudeCodeParsedRequestContextKey`
  - `backend/internal/handler/gateway_helper.go:95`
  - 但计划版 helper 完全没有复用这条分支

因此，按计划实现后，Copilot Messages 路径里 `claudeCodeBodyMapForInitiator(c)` 大概率长期拿到的是空 map。

而 `Validate()` 在 messages 路径下，空 body map 会直接失败：

- `backend/internal/service/claude_code_validator.go:86`
- `backend/internal/service/claude_code_validator.go:109`

#### 风险
- 计划声称 Anthropic Messages 路径升级成“强门控”，但按当前实现步骤，`isCC` 实际上大概率始终是 `false`。
- 结果就是：主修复目标之一的 `ForwardMessages` 路径仍然不会命中 sub-agent 检测，Premium 修复继续失效。

#### 建议
- `claudeCodeBodyMapForInitiator(c)` 不能只读 `OpenAIParsedRequestBodyKey`。
- 至少要做到下面三者之一：
  - 在 Copilot handler 明确把可用于 validator 的 body map 缓存进 `gin.Context`
  - 复用与 [gateway_helper.go:86](/Users/ziji/personal/github/sub2api/backend/internal/handler/gateway_helper.go:86) 等价的多来源读取逻辑
  - 或者在 `CopilotInitiatorFromAnthropicBody` 里直接从 `anthropicBody` 构造 validator 所需的 body map

---

#### H2. Task 3 的 Messages 正向集成测试仍然缺少 `Validate()` 必需的 headers 和 `metadata.user_id`，按文档执行仍无法通过

#### 问题
计划把 Messages 路径升级成完整 `Validate()`，并明确写了这个强门控会检查：
- system prompt 相似度
- `X-App`
- `anthropic-beta`
- `anthropic-version`
- `metadata.user_id`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:603`
- 对应真实实现见 `backend/internal/service/claude_code_validator.go:93`

但 Task 3 的测试修改只补了 `User-Agent`：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:664`

新增的正向请求体仍然没有 `metadata.user_id`：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:682`
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:689`
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:694`

同时也没有任何步骤要求在测试 request 上设置：
- `X-App`
- `anthropic-beta`
- `anthropic-version`

#### 风险
- 即使修好了 body map 来源，Task 3 的正向用例依然会因为缺少 validator 必需输入而全部失败。
- 这会让实现者误以为 service 逻辑有问题，但实际上是计划里的测试构造不完整。

#### 建议
- Task 3 必须同步补齐强门控所需输入：
  - 正向 case 的 body 中加入合法 `metadata.user_id`
  - request header 中加入 `X-App`、`anthropic-beta`、`anthropic-version`
- 如果不想在测试里构造完整 Claude Code 请求，则不应把该路径宣称为“完整 `Validate()` 强门控”。

---

### LOW

#### L1. 文档顶部的 Architecture 摘要仍停留在旧设计，和后文“弱门控/强门控分层”已经不一致

#### 问题
文档开头仍写：

- 客户端身份“由调用方通过 `ClaudeCodeValidator.ValidateUserAgent(ua)` 确定”
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:7`

但后文已经明确改成：
- ChatCompletions 路径走 UA-only 弱门控
- Anthropic Messages 路径走完整 `Validate()` 强门控
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:602`

#### 风险
- 计划摘要和具体任务不一致，后续实施者容易按顶部摘要理解错设计。

#### 建议
- 同步更新开头 `Architecture` 段，改成与 Task 2 / Task 4 一致的“分层门控”描述。

---

## 建议的修订方向

1. 先补齐 Messages 强门控的数据来源
- 明确 Copilot handler 如何把 validator 需要的 body map 传到 service。
- 不能只依赖当前不存在的 context cache。

2. 再补 Task 3 测试构造
- 正向 Messages case 需要完整的 Claude Code 校验输入，而不只是 `User-Agent`。

3. 更新摘要说明
- 把文档顶部 Architecture 段与当前的弱门控/强门控设计保持一致。

---

## 审阅结论

这版计划离最终可实施又近了一步，但现在的主阻断点非常明确：Anthropic Messages 的强门控设计还没有真正接上 validator 所需的数据输入，所以主修复路径仍不成立。把这条链路接通，并补齐 Task 3 的完整测试输入后，这份计划会更扎实。
