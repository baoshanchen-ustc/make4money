# Copilot Sub-Agent Premium Fix 方案评审报告 Round 6

## 基本信息
- 评审日期：2026-04-12
- 评审对象：`docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md`
- 评审类型：实施前方案复审
- 评审方式：静态审阅更新后的计划文档，并对照当前 `ClaudeCodeValidator` 的真实输入契约核对可实施性

## 复审结论（摘要）
- 这版计划已经把前几轮的大部分结构问题收得很干净了，尤其是 Responses wrapper 和 Messages 测试输入都补得更完整。
- 但现在仍有 1 个新的阻断级问题：新引入的 `anthropicBodyToValidatorMap` 产出的数据结构和 `ClaudeCodeValidator.Validate()` 的真实输入契约不兼容。按文档实现后，Anthropic Messages 的“强门控”仍然会持续失败。
- 当前结论仍为：`REQUEST CHANGES`。

---

## 主要发现

### HIGH

#### H1. `anthropicBodyToValidatorMap` 生成的 body map 不符合 `ClaudeCodeValidator.Validate()` 的真实要求，强门控仍然无法生效

#### 问题
计划把 Anthropic Messages 强门控改成：

- `isCC := NewClaudeCodeValidator().Validate(c.Request, anthropicBodyToValidatorMap(anthropicBody))`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:582`

并把 helper 定义为：

- 只提取 `system` 和 `metadata`
- `system` 直接保留为 `json.RawMessage`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:593`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:595`

但当前 `ClaudeCodeValidator.hasClaudeCodeSystemPrompt` 的真实输入要求是：

1. `body["model"]` 必须是 `string`
- `backend/internal/service/claude_code_validator.go:138`

2. `body["system"]` 必须是 `[]any`
- `backend/internal/service/claude_code_validator.go:144`

而计划版 `anthropicBodyToValidatorMap`：
- 根本没有把 `model` 放进 map
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:603`
- 把 `system` 放成了 `json.RawMessage`，不是 `[]any`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:605`

更关键的是，helper 旁边的说明还写着“`Validate` 实际只读 `system` 和 `metadata`”，这和真实代码不一致：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:593`

#### 风险
- 按当前文档实现后，`Validate()` 会先在 `model` 检查处直接失败；就算补了 `model`，也还会在 `system` 类型断言处失败。
- 结果是：Anthropic Messages 路径宣称的“强门控”仍然实际拿不到 `true`，主修复路径继续不生效。
- Task 3 补的那些 `X-App` / `anthropic-*` / `metadata.user_id` 测试输入，也会因为这个更前面的结构不匹配而白补。

#### 建议
- `anthropicBodyToValidatorMap` 必须按 `Validate()` 的真实契约构造数据，而不是按当前假设：
  - 补上 `model string`
  - 把 `system` 解析成 `[]any`（或直接解析成 `map[string]any` 后取其 `system` 原生形态）
  - `metadata` 保持 `map[string]any`
- 更稳妥的做法是直接把 Anthropic body 反序列化成 `map[string]any`，然后只保留 validator 需要的字段，而不是手写一个结构体后再拼 map。

---

## 建议的修订方向

1. 先修 `anthropicBodyToValidatorMap`
- 让它输出与 `Validate()` 真实契约一致的 `map[string]any`。
- 至少补齐 `model`，并把 `system` 从 `json.RawMessage` 转成 `[]any`。

2. 修正文档说明
- 删掉“`Validate` 实际只读 `system` 和 `metadata`”这句不准确描述。
- 把 helper 的字段选择依据改成与真实 `hasClaudeCodeSystemPrompt` 实现一致。

---

## 审阅结论

这版计划已经很接近最终形态，但 `anthropicBodyToValidatorMap` 这一处输入契约错误会直接让强门控失效，所以仍然不能直接进入实现。把这一个结构问题修正后，这份计划大概率就能进入可实施状态了。
