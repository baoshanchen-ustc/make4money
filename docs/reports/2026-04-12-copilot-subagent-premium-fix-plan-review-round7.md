# Copilot Sub-Agent Premium Fix 方案评审报告 Round 7

## 基本信息
- 评审日期：2026-04-12
- 评审对象：`docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md`
- 评审类型：实施前方案复审
- 评审方式：静态审阅更新后的计划文档，并对照当前 `ParseMetadataUserID` / `ClaudeCodeValidator` 的真实校验规则核对测试可行性

## 复审结论（摘要）
- 这版计划已经把上一轮 `anthropicBodyToValidatorMap` 的结构问题修正了，整体实现链路更接近最终可落地状态。
- 但现在还剩 1 个新的阻断级问题：Task 3 的正向 Messages 测试里使用的 `metadata.user_id` 样例不符合仓库当前的真实格式校验，按文档执行这些用例仍然会失败。
- 当前结论仍为：`REQUEST CHANGES`。

---

## 主要发现

### HIGH

#### H1. Task 3 中的 `metadata.user_id: "u_test_123"` 不满足 `ParseMetadataUserID()` 的真实格式要求，强门控正向用例仍会失败

#### 问题
计划现在已经正确意识到 Anthropic Messages 强门控不仅要求：
- `User-Agent`
- `X-App`
- `anthropic-beta`
- `anthropic-version`

还要求 `metadata.user_id`：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:663`

并且正向 case 里统一加了：

- `"metadata":{"user_id":"u_test_123"}`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:705`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:711`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:716`

但当前 validator 的真实逻辑并不是“非空字符串即可”。它还会继续调用：

- `ParseMetadataUserID(userID)`
- `backend/internal/service/claude_code_validator.go:124`

而 `ParseMetadataUserID()` 只接受两种格式：

1. 旧格式
- `user_{64hex}_account_{optional_uuid}_session_{uuid}`
- `backend/internal/service/metadata_userid.go:21`

2. 新格式
- 一个 JSON 字符串，至少包含 `device_id` 和 `session_id`
- `backend/internal/service/metadata_userid.go:41`

`"u_test_123"` 不满足其中任何一种格式，因此会直接返回 `nil`：

- `backend/internal/service/metadata_userid.go:35`
- `backend/internal/service/metadata_userid.go:60`

仓库里现有的有效样例也不是这种短字符串，而是完整的 legacy 格式，例如：

- `backend/internal/service/claude_code_detection_test.go:29`

#### 风险
- 即使当前计划的强门控链路已经接通，Task 3 的正向用例依然会因为 `metadata.user_id` 格式非法而全部失败。
- 这会让实现者误以为 sub-agent 检测本身有问题，但实际失败点是测试构造不符合 validator 真实契约。

#### 建议
- 把 Task 3 正向 case 中的 `metadata.user_id` 替换为一个真实可解析的值。
- 最稳妥的做法是直接复用仓库现有测试中的合法样例格式，例如：
  - legacy 格式：`user_<64hex>_account__session_<uuid>`
  - 或按 UA 版本使用 JSON 格式字符串
- 同时把文档里“非空字符串即可”的描述改掉，改成“必须通过 `ParseMetadataUserID()`”。

---

## 建议的修订方向

1. 修正 Task 3 中所有正向 Messages case 的 `metadata.user_id`
- 使用现有测试中已经验证过的合法格式。

2. 更新文档说明
- 把 `metadata.user_id` 的要求从“非空字符串”改成“必须满足 `ParseMetadataUserID()` 可解析格式”。

---

## 审阅结论

这版计划已经很接近最终可实施版本了，但 `metadata.user_id` 样例格式这一处仍会让强门控正向测试整体失败。把这一处样例和说明修正后，这份计划大概率就可以进入最终实施阶段。
