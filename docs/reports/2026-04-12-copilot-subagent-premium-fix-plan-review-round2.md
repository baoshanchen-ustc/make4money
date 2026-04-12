# Copilot Sub-Agent Premium Fix 方案评审报告 Round 2

## 基本信息
- 评审日期：2026-04-12
- 评审对象：`docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md`
- 评审类型：实施前方案复审
- 评审方式：静态审阅更新后的计划文档，并对照当前仓库代码路径核对可实现性与回归风险

## 复审结论（摘要）
- 第二版计划已经修掉了第一轮 review 中的一部分核心方向问题，尤其是把“非 Claude Code 客户端伪造 prompt”列为反例，这个方向是对的。
- 但当前版本仍然有阻断级缺口：一处会导致真实修复根本不生效，一处会让目标子 agent 变体仍然漏判，还有一处代码片段按文档实现会直接编译失败。
- 当前结论仍为：`REQUEST CHANGES`。

---

## 主要发现（按严重级别）

### HIGH

#### H1. 计划把客户端门控改成 `IsClaudeCodeClient(ctx)`，但没有把该标记真正接入 Copilot handler，请求实际仍会走 `false`

#### 问题
更新后的计划要求 service 侧内部调用改成：

- `copilotInitiator(body, IsClaudeCodeClient(ctx))`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:497`

并在说明里写到：

- Copilot handler 目前未调用 `SetClaudeCodeClientContext`，直到 Task 4 修复
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:505`

但 Task 4 实际只修改了 handler 里的 usage 统计调用：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:622`
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:646`

它并没有要求在 Copilot handler 中调用 `SetClaudeCodeClientContext`，也没有任何步骤把 `IsClaudeCodeClient(ctx)` 设为 true。

当前仓库里：
- 普通 gateway handler 会调用 `SetClaudeCodeClientContext`
  - `backend/internal/handler/gateway_handler.go:170`
  - `backend/internal/handler/gateway_handler.go:1420`
- Copilot handler 没有这一步；`ChatCompletions` 和 `Messages` 入口从读 body 到转发服务都没有写入 Claude Code client 标记
  - `backend/internal/handler/copilot_gateway_handler.go:131`
  - `backend/internal/handler/copilot_gateway_handler.go:205`
  - `backend/internal/handler/copilot_gateway_handler.go:962`
  - `backend/internal/handler/copilot_gateway_handler.go:1046`

#### 风险
- 按当前计划实现后，真实上游请求路径里的 `IsClaudeCodeClient(ctx)` 仍会一直是 `false`。
- 结果是：计划新增的 sub-agent 检测逻辑在真实转发链路上根本不会触发，Premium 修复不会生效。
- 文档里的 Task 3 / Task 5 集成测试也无法按描述通过，因为当前 service 集成测试直接传的是 `context.Background()`，并不会从 `User-Agent` 自动推导 `IsClaudeCodeClient(ctx)`。

#### 建议
- 计划必须新增一个明确任务：在 `backend/internal/handler/copilot_gateway_handler.go` 里像主 gateway 一样设置 Claude Code client context。
- 或者改设计：service 不依赖 `IsClaudeCodeClient(ctx)`，而是显式从调用方传入 `userAgent` / `isClaudeCode`。

---

#### H2. `claudeCodeSubAgentPrefixes` 的拆分方案会漏掉 “CC + Agent SDK” 这个目标 case，和自己的测试互相矛盾

#### 问题
Task 0 计划把 `claudeCodeSubAgentPrefixes` 定义为：

- `claudeCodePromptPrefixes[1]`
- `claudeCodePromptPrefixes[2]`
- `claudeCodePromptPrefixes[3]`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:134`

而当前 `claudeCodePromptPrefixes` 的真实定义是：

- `[0] "You are Claude Code, Anthropic's official CLI for Claude"`
- `[1] "You are a Claude agent, built on Anthropic's Claude Agent SDK"`
- `[2] "You are a file search specialist for Claude Code"`
- `[3] "You are a helpful AI assistant tasked with summarizing conversations"`
- 见 `backend/internal/service/gateway_service.go:336`

但 Task 1 的测试又明确要求下面这个 case 返回 `true`：

- `"You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK."`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:194`

同时 Task 1 的实现改成了 `strings.HasPrefix(trimmed, prefix)`：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:280`

这个 prompt 的前缀是 `claudeCodePromptPrefixes[0]`，而不是 `[1]`、`[2]`、`[3]`。按计划实现，`CC + Agent SDK` 这个文档自己列出的目标子 agent 变体仍然不会命中。

#### 风险
- 方案声称修复了 Agent SDK 系列子 agent，但其中一个关键真实变体会继续被漏判为 `user`。
- 当前 Task 0 的“只从现有 prefix 列表拆分子集”与 Task 1 的“CC + Agent SDK 必须返回 true”是互相冲突的，实施者无法同时满足。

#### 建议
- 不要仅靠 `claudeCodePromptPrefixes` 的子数组来表达 sub-agent 集合。
- 至少要为 `running within the Claude Agent SDK` 单独建规则，或改为复用 `claudeCodeSystemPrompts` 中的完整模板来源。

---

### MEDIUM

#### M1. 文档中的 service 代码片段引用了不存在的 `claudeCodeValidator` 变量，按文档实现会直接编译失败

#### 问题
Task 2 和 Task 4 的代码片段都写了：

- `isClaudeCode := claudeCodeValidator.ValidateUserAgent(userAgent)`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:446`
- 见 `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:667`

但当前仓库里这个单例只存在于 `handler` 包：

- `backend/internal/handler/gateway_helper.go:19`

而 `ValidateUserAgent` 是 `ClaudeCodeValidator` 的实例方法：

- `backend/internal/service/claude_code_validator.go:251`

`service` 包里并没有可直接使用的 `claudeCodeValidator` 变量。

#### 风险
- 实施者如果照着文档贴代码，`backend/internal/service/copilot_gateway_service.go` 会直接编译失败。

#### 建议
- 文档应改成可编译的写法，比如：
  - 显式创建局部 validator 实例；
  - 或在 `service` 包提供统一 helper；
  - 或把 UA 校验放回调用方并显式传 `isClaudeCode bool`。

---

### LOW

#### L1. 文档仍残留多处 `git commit -m`，与“已全部改为按仓库提交协议提交”的说法不一致

#### 问题
口头说明称所有提交步骤都已改为按仓库提交协议提交，但文档里仍残留普通单行 commit 命令：

- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:323`
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:537`
- `docs/superpowers/plans/2026-04-12-copilot-subagent-premium-fix.md:606`

#### 风险
- 实施者若按文档逐步执行，仍会产出不符合 Lore commit protocol 的提交。

#### 建议
- 把残留的 `git commit -m` 示例全部去掉，统一改成“按仓库提交协议提交”。

---

## 建议的修订方向

1. 先修正客户端门控链路
- 明确由谁把 Claude Code client 标记写入 Copilot 请求上下文。
- 让 service 的 `IsClaudeCodeClient(ctx)` 真正可用，或放弃这一路径，改成显式传参。

2. 重新定义 sub-agent prompt 来源
- 当前“从 `claudeCodePromptPrefixes` 中抽子集”的方案无法同时覆盖 `running within the Claude Agent SDK` 和主 agent primary prompt。
- 需要一套能区分这两个分支的真实来源，而不是仅靠共享前缀数组。

3. 修正文档代码片段
- 去掉 `service` 包里不存在的 `claudeCodeValidator` 变量引用。
- 保证计划中的代码片段至少是可编译、可抄写的。

4. 清理提交流程残留
- 把剩余 `git commit -m` 示例全部替换为 Lore protocol 指引。

---

## 审阅结论

第二版计划比第一版更接近可实施状态，但还没有到可以直接开工的程度。当前最大的问题不是“细节遗漏”，而是修复链路仍未闭环：service 侧门控依赖的上下文没接上，sub-agent prompt 来源设计也还存在自相矛盾。

建议再修一轮计划，至少把以上三处问题收敛后再进入代码实现。
