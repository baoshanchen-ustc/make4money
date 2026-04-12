# Copilot Sub-Agent Premium Fix 方案评审报告 Round 3

## 基本信息
- 评审日期：2026-04-12
- 评审对象：`docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md`
- 评审类型：实施前方案复审
- 评审方式：静态审阅更新后的计划文档，并对照当前仓库代码结构核对可实施性与验证闭环

## 复审结论（摘要）
- 这版计划已经把上一轮两个最关键的方向问题基本收住了：不再依赖 `IsClaudeCodeClient(ctx)`，`CC + Agent SDK` 变体也有了区分路径。
- 但文档仍有 1 个阻断级问题和 2 个中风险问题，主要集中在测试步骤未跟上新 UA 依赖、以及新增重复 helper / 重复字符串来源。
- 当前结论仍为：`REQUEST CHANGES`。

---

## 主要发现（按严重级别）

### HIGH

#### H1. 新设计已经依赖 `User-Agent`，但计划里的集成测试步骤没有把 UA 设置步骤写完整，按文档执行会直接跑不通

#### 问题
这版计划把 service 内部判断改成了直接读取：

- `NewClaudeCodeValidator().ValidateUserAgent(c.GetHeader("User-Agent"))`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:535`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:544`

这意味着只要集成测试没给 `gin.Context` 的 request header 设置 Claude Code UA，所有正向 sub-agent 用例都会继续得到 `user`。

当前仓库里的现有测试框架恰好没有设置 `User-Agent`：

- ChatCompletions 测试 case 结构体里目前没有 `nonCCUA` / `userAgent` 字段
  - `backend/internal/service/copilot_gateway_service_test.go:1550`
- ChatCompletions 测试 request 只创建了 request，没有设置任何 UA
  - `backend/internal/service/copilot_gateway_service_test.go:1604`
- Messages 测试 request 目前只设置了 `Accept`
  - `backend/internal/service/copilot_gateway_service_test.go:1765`
  - `backend/internal/service/copilot_gateway_service_test.go:1766`

而计划中：
- Task 3（Messages 路径）新增了正向 sub-agent 用例，却完全没有要求设置 Claude Code UA
  - `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:592`
- Task 5（ChatCompletions）虽然提到了 `nonCCUA` 概念，但只写了备注，没有把 case struct、默认正向 UA、以及 request header 设置步骤明确成可执行修改
  - `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:807`
  - `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:838`

#### 风险
- 按文档实现后，Task 3 的正向用例大概率会全部失败。
- Task 5 也仍然不足以指导实现者把现有测试 harness 改到可运行状态。
- 这不是“小文档瑕疵”，而是会直接阻断计划里的验证步骤。

#### 建议
- Task 3 和 Task 5 都需要把测试框架修改写完整：
  - case struct 新增 `userAgent string` 或 `nonCCUA bool`
  - 正向 case 明确默认设置 `claude-cli/x.y.z`
  - 反例 case 明确覆盖非 Claude Code UA
  - 在 `httptest.NewRequest` 后明确 `c.Request.Header.Set("User-Agent", ...)`

---

### MEDIUM

#### M1. `CopilotInitiatorFromAnthropicBody` 计划新增了一个重复的 system 提取 helper，而且行为比现有 helper 更弱

#### 问题
Task 4 计划新增：

- `extractAnthropicSystemText(raw json.RawMessage) string`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:742`

这个 helper 的实现只返回“第一个 text block”：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:750`

但当前包里已经有现成的 `extractSystemText(raw json.RawMessage)`：

- `backend/internal/service/copilot_anthropic_translation.go:456`

并且现有 helper 会遍历并拼接所有 text block：

- `backend/internal/service/copilot_anthropic_translation.go:463`
- `backend/internal/service/copilot_anthropic_translation.go:472`

#### 风险
- 计划新增的 helper 会和现有翻译路径对 `system` 的解释不一致。
- 如果 sub-agent 标识出现在第二个及之后的 text block，中转后的真实上游 header 可能判成 `agent`，但 analytics 侧 `CopilotInitiatorFromAnthropicBody` 仍可能记成 `user`，重新制造口径分叉。
- 同时也引入了第二个功能近似的 helper，增加后续维护成本。

#### 建议
- 不要新增 `extractAnthropicSystemText`。
- 直接复用包内已有的 `extractSystemText(raw)`，保持 analytics 路径和真实转发路径对 `system` 字段的解释一致。

---

#### M2. 计划重新引入了新的 prompt 字面量来源，和“避免多处重复维护”的目标又冲突了

#### 问题
计划顶部仍写着：

- `claudeCodeSubAgentPrefixes` 用来避免多处重复维护
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:7`

但 Task 0 现在改成了直接写一份新的字面量列表：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:150`

而当前仓库里本来已经有两套相关来源：

- `claudeCodePromptPrefixes`
  - `backend/internal/service/gateway_service.go:336`
- `claudeCodeSystemPrompts`
  - `backend/internal/service/claude_code_validator.go:30`

这次又新增：
- `claudeCodeSubAgentPrefixes`
- `claudeCodeSubAgentSDKMarker`

#### 风险
- 现在 prompt 来源至少有三处，未来 Claude Code prompt 升级时更容易出现只改一处、漏改另一处的情况。
- 这次虽然修掉了 “CC + Agent SDK 共前缀” 的问题，但代价是又把字符串源分叉扩大了。

#### 建议
- 如果保留 `claudeCodeSubAgentSDKMarker`，建议至少让 `claudeCodeSubAgentPrefixes` 从一个现有来源派生，避免再新增完整字面量列表。
- 或在计划里明确把这组新字面量视为唯一 billing 路由来源，并同步删掉/收敛其他重复来源。

---

## 建议的修订方向

1. 先补完集成测试步骤
- Task 3 / Task 5 要明确写出 request header 的 UA 设置方法。
- 不能只留“请确认测试框架设置 UA”这种提示语。

2. 删除重复 helper
- `CopilotInitiatorFromAnthropicBody` 直接复用 `extractSystemText(raw)`。
- 保持 analytics 与真实翻译路径一致。

3. 再收一轮字符串来源
- 当前新增的 `claudeCodeSubAgentPrefixes` 解决了分类问题，但重新引入了维护分叉。
- 需要在计划里明确最终的单一来源策略。

---

## 审阅结论

这版计划已经非常接近可实施状态，核心判定链路也比前两版合理得多。现在剩下的问题主要不在“思路错了”，而在于文档层面还没把验证和复用收干净：测试步骤缺关键 header 设置，Anthropic system 解析又多出一个重复 helper。

把这两块再收一轮，这份计划就比较适合直接进入实现了。
