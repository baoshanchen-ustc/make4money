# Copilot Sub-Agent Premium Fix 方案评审报告

## 基本信息
- 评审日期：2026-04-12
- 评审对象：`docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md`
- 评审类型：实施前方案 review
- 评审方式：静态审阅计划文档，并对照当前仓库代码路径核对实现影响面与回归风险

## 评审结论（摘要）
- 方案抓住了问题本质：Claude Code sub-agent 首轮请求确实可能因缺少 `assistant`/`tool` 消息而被误判为 `user`。
- 但当前计划把修复点设计成了一个过于宽的全局 heuristic，存在配额绕过面、统计口径失真和字符串源重复维护的问题。
- 当前结论建议为：`REQUEST CHANGES`。应先收紧适用范围并补齐观测面，再进入实施。

---

## 主要发现（按严重级别）

### HIGH

#### H1. 方案把 sub-agent prompt 检测并入全局 `copilotInitiator`，会把“任何客户端伪造 prompt”都判成免费 `agent`

#### 问题
计划要求直接修改 `copilotInitiator()`，并把新逻辑扩展到 ChatCompletions 路径：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:307`
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:469`

但当前 `copilotInitiator()` 并不只服务 Claude Code sub-agent：

- OpenAI ChatCompletions 直连路径会直接调用它：
  - `backend/internal/service/copilot_gateway_service.go:278`
- OpenAI Chat via Responses 路径也会调用它：
  - `backend/internal/service/copilot_gateway_service.go:436`
- OpenAI Chat via Messages 路径同样调用它：
  - `backend/internal/service/copilot_gateway_service.go:576`
- Anthropic `/messages` 入口本身也明确支持 “Claude Code and any Anthropic-compatible client”：
  - `backend/internal/service/copilot_gateway_service.go:1948`
  - `backend/internal/service/copilot_gateway_service.go:2062`

按计划实现后，只要任意客户端在 system prompt 中包含：
- `"Claude Agent SDK"`
- `"file search specialist for Claude Code"`
- `"summarizing conversations"`

就会被路由成 `X-Initiator: agent`。

#### 风险
- 这不是单纯的 Claude Code 修复，而是把一套计费/配额决策扩展成了“字符串触发的全局免费通道”。
- 第三方客户端或恶意请求可以伪造 system prompt，错误消耗标准配额而不是 premium 配额。
- 风险面覆盖所有复用 `copilotInitiator()` 的 Copilot OpenAI 路径，而不只是 Claude Code。

#### 建议
- 必须把这条 heuristic 收紧到“已确认是 Claude Code 客户端”的请求范围内。
- 优先复用已有客户端判定链路，而不是仅靠 prompt 文本：
  - `backend/internal/handler/gateway_helper.go:23`
  - `backend/internal/service/claude_code_validator.go:262`
- 测试中必须增加反例：
  - 非 Claude Code 客户端，即使携带同样的 system prompt，仍应返回 `user`。

---

### MEDIUM

#### M1. 文档认为 handler 侧 `CopilotInitiatorFromBody` “只用于内部日志，可不改”，这个判断不准确

#### 问题
计划中写道：

- handler 里的 `CopilotInitiatorFromBody` 只用于内部统计日志，不影响实际发给 GitHub 的 `X-Initiator`
  - `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:74`
  - `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:565`

但当前 handler 会把该值写入 `RecordUsageInput.Initiator`：

- ChatCompletions：
  - `backend/internal/handler/copilot_gateway_handler.go:370`
- Responses：
  - `backend/internal/handler/copilot_gateway_handler.go:815`
- Messages：
  - `backend/internal/handler/copilot_gateway_handler.go:1243`

而 analytics 会直接按 `usage_logs.initiator` 聚合 premium/agent 请求：

- `backend/internal/service/copilot_analytics_service.go:211`
- `backend/internal/service/copilot_analytics_service.go:500`

#### 风险
- 即使上游 `X-Initiator` 修好了，内部用量统计和看板仍可能继续把这类请求记成 premium。
- 修复后的验证会变得困难，因为“真实上游行为”和“系统内统计口径”会分叉。
- 后续排查 premium 异常消耗时，报表会继续误导。

#### 建议
- 计划不应把 handler 侧 initiator 采集排除在范围外。
- 至少要明确：
  - 要么同步修正统计口径；
  - 要么在计划里明确声明这是已知偏差，并补一个后续任务收敛。

---

#### M2. 方案重复维护一套 sub-agent prompt 关键字，和仓库现有来源分叉

#### 问题
计划已经明确说明这些特征字符串“已在 `claude_code_validator.go` 中归档”：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:49`

当前仓库里确实已有相关模板与前缀来源：

- `backend/internal/service/claude_code_validator.go:30`
- `backend/internal/service/gateway_service.go:336`
- `backend/internal/service/gateway_service.go:3829`

但计划又要求在 `copilot_gateway_service.go` 里新增 `isClaudeCodeSubAgentSystemPrompt()`，再 hardcode 一份新的关键字表：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:176`

#### 风险
- Claude Code prompt 模板升级时，可能只更新 validator / 注入逻辑，而忘记更新 initiator heuristic。
- 同一个业务语义出现多套字符串源，后续非常容易出现“验证通过，但 billing 路由没命中”或反过来的分叉。

#### 建议
- 优先复用已有 prompt 来源，避免新增第三套独立字符串表。
- 如果确实要新建 helper，至少应从统一来源派生匹配规则，而不是复制字面量。

---

#### M3. 测试覆盖缺少关键反例，无法证明修复“只影响 Claude Code sub-agent”

#### 问题
计划新增了大量正向测试：

- sub-agent 首轮返回 `agent`
- main agent 首轮返回 `user`

对应位置：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:249`
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:405`
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:474`

但缺少以下关键反例：
- 非 Claude Code 客户端，system prompt 恶意伪造为 Agent SDK prompt
- system content 为数组或非纯字符串时，是否会漏判或误判
- 统计链路是否与实际上游 header 保持一致

#### 风险
- 现有测试只能证明“字符串匹配生效了”，不能证明“作用域被限制正确了”。
- 修复上线后若被其他客户端命中，当前测试无法提前暴露。

#### 建议
- 增加至少三类反例测试：
  - 非 Claude Code 请求 + sub-agent prompt -> 仍是 `user`
  - system content 非 string -> 不应 panic，且行为明确
  - handler 记录的 `Initiator` 与真正发往上游的 `X-Initiator` 一致

---

### LOW

#### L1. 提交步骤没有遵守当前仓库的 Lore commit protocol

#### 问题
计划中的提交步骤仍然是普通单行 commit message：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:230`
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:387`
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:457`
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:510`

#### 风险
- 实施者若按文档逐步执行，会直接产出不符合仓库提交规范的 commit。

#### 建议
- 把提交示例改成符合 Lore protocol 的格式，或删除具体 `git commit -m` 示例，改为“按仓库提交协议提交”。

---

## 建议的修订方向

1. 收紧适用范围
- 不要把“只看 system prompt 文本”的规则直接作为全局 `copilotInitiator()` 新优先级。
- 应先确认请求属于 Claude Code，再做 sub-agent prompt 检测。

2. 统一数据口径
- 同步考虑上游 `X-Initiator` 与内部 `RecordUsage.Initiator` 的一致性。
- 避免修复上游计费后，内部分析面板继续误报 premium 请求。

3. 统一字符串来源
- 从现有 Claude Code prompt 模板或前缀来源复用匹配依据。
- 避免在不同文件中复制维护相似但不完全相同的关键字列表。

4. 补反例测试
- 增加“伪造 prompt 的非 Claude Code 客户端”测试。
- 增加“非 string system content”测试。
- 增加 usage 记录链路一致性测试。

---

## 审阅结论

这份计划的问题不在于方向错误，而在于修复作用域定义得过宽。当前文档足以指导实现一个“能把字符串命中请求改成 `agent`”的补丁，但还不足以保证这个补丁只修正 Claude Code sub-agent，不引入新的配额绕过面和统计偏差。

建议先完成以上修订，再进入代码实现阶段。
