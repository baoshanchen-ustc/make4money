# Copilot Sub-Agent Premium 消耗优化实现方案

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 通过检测 Claude Code sub-agent 的 system prompt，将其 `X-Initiator` 从 `user`（消耗 Premium）改为 `agent`（走免费 Standard 配额），大幅减少 Claude Code 用户的 GitHub Copilot Premium 请求消耗。

**Architecture:** 给 `copilotInitiator()` 新增 `isClaudeCode bool` 参数，**只有当确认是 Claude Code 客户端时**才做 sub-agent system prompt 检测；否则跳过，直接走原有 assistant/tool 判断逻辑。门控强度分两层：ChatCompletions 路径使用**弱门控**（UA 正则匹配，`ValidateUserAgent(ua)`），Anthropic Messages 路径使用**强门控**（完整 `Validate()`，含 UA + system prompt 相似度 + `X-App`/`anthropic-beta`/`anthropic-version`/`metadata.user_id`）。强门控所需的 body map 通过 `anthropicBodyToValidatorMap(anthropicBody []byte)` 直接从 Anthropic body bytes 构造，不依赖 gin.Context 缓存。新增辅助函数 `isClaudeCodeSubAgentSystemPrompt` 封装特征匹配，其关键字从 `claudeCodeSubAgentPrefixes`（新增于 `gateway_service.go`）派生，避免多处重复维护。同步修正 handler 层的 `CopilotInitiatorFromBody` 调用，保证 analytics 统计口径与上游 X-Initiator 一致。

**Tech Stack:** Go 1.22+，标准库 `strings`/`encoding/json`，现有 `copilot_gateway_service.go`、`gateway_service.go`、`copilot_gateway_handler.go`。

---

## 背景与根因分析

### 问题现象

用户使用 Claude Code + sub2api → GitHub Copilot 时，仅发起"几次提问"却消耗了 162 次 Premium 请求（每月限额 300 次），导致配额快速耗尽。

### 根因

Claude Code 采用 **multi-agent 架构**：主 agent 处理用户请求时，会并发 spawn 多个 sub-agent（Explore Agent、General-purpose Agent 等）执行文件搜索、代码分析等子任务。

**每个 sub-agent 发起的首次 API 请求**具有以下特征：
- messages 数组只有 `system` + `user` 消息
- **没有** `assistant` 或 `tool` 消息

当前 `copilotInitiator()` 逻辑：
```go
for _, m := range req.Messages {
    if m.Role == "assistant" || m.Role == "tool" {
        return "agent"  // 免费
    }
}
return "user"  // Premium！
```

Sub-agent 首次请求因无 `assistant`/`tool` 消息，被判定为 `X-Initiator: user`，消耗 Premium 配额。

一次用户"提问"可能触发 10-30 个 sub-agent（并行），每个首次请求各消耗 1 次 Premium → 数倍放大效应。

### 为何 v0.1.140 前感觉"没问题"

代码逻辑在 v0.1.140 时就已经相同（`50cc083d` 提交已在 v0.1.140 中）。差异在于：
- v0.1.140 前用户通常使用普通客户端（Cherry Studio 等），每次发送完整对话历史（含 assistant 消息），大部分请求走免费配额
- 切换到 Claude Code 后，sub-agent 模式导致大量"首次请求"，Premium 消耗急剧增加

### 解决思路

**Claude Code sub-agent 的 system prompt 具有固定特征**（已在 `claude_code_validator.go` 中归档）：

| Sub-agent 类型 | System Prompt 关键特征 |
|---|---|
| Claude Agent SDK 子 agent | `"Claude Agent SDK"` |
| Explore Agent（文件搜索） | `"file search specialist for Claude Code"` |
| 对话摘要 Agent | `"summarizing conversations"` |

主 agent 的 system prompt（应继续消耗 Premium）：
- `"You are Claude Code, Anthropic's official CLI for Claude."` ← 不含上述特征
- `"You are an interactive CLI tool that helps users"` ← 不含上述特征

**通过检测 system prompt 特征，将 sub-agent 首次请求的 `X-Initiator` 设为 `agent`（免费）**。

---

## 文件改动范围

| 文件 | 操作 | 说明 |
|---|---|---|
| `backend/internal/service/gateway_service.go` | 修改 | 新增 `claudeCodeSubAgentPrefixes`（独立字面量，计费路由唯一来源）和 `claudeCodeSubAgentSDKMarker`；新增 `copilot_subagent_test.go` 保证与 `claudeCodePromptPrefixes` 不脱钩 |
| `backend/internal/service/copilot_gateway_service.go` | 修改 | 新增 `isClaudeCodeSubAgentSystemPrompt`、`anthropicBodyToValidatorMap`；修改 `copilotInitiator(body, isClaudeCode)`；修改 `CopilotInitiatorFromBody(body, userAgent)` 签名；新增 `CopilotInitiatorFromAnthropicBody(body, c)` 和 `CopilotInitiatorFromResponsesBody(body)` |
| `backend/internal/handler/copilot_gateway_handler.go` | 修改 | ChatCompletions 路径（第 370 行）传入 UA；Responses 路径（第 815 行）改用 `CopilotInitiatorFromResponsesBody(body)`；Messages 路径（第 1243 行）改用 `CopilotInitiatorFromAnthropicBody(body, c)` |
| `backend/internal/service/copilot_gateway_service_test.go` | 修改 | 新增子 agent 场景测试用例、反例测试，修改集成测试 case struct 加 `userAgent` 字段 |

**不需要修改：**
- `copilotInitiatorFromResponsesBody`：私有函数保持不变，新增公开 wrapper `CopilotInitiatorFromResponsesBody` 代理它
- `ClaudeCodeValidator`：职责是验证请求是否来自 Claude Code 客户端，无需修改

---

## Task 0：在 `gateway_service.go` 中拆分出 `claudeCodeSubAgentPrefixes`（M2）

**Files:**
- Modify: `backend/internal/service/gateway_service.go`（`claudeCodePromptPrefixes` 附近，约第 336 行）
- Test: `backend/internal/service/gateway_service_test.go`（或新增独立测试文件）

### Step 0.1：先写失败的单元测试

在 `backend/internal/service/` 下找到或新建 `copilot_subagent_test.go`，写入：

```go
package service

import (
    "strings"
    "testing"
)

func TestClaudeCodeSubAgentPrefixes_NotContainMainAgent(t *testing.T) {
    // 主 agent 前缀不应出现在 sub-agent 前缀列表中
    mainAgentPrefixes := []string{
        "You are Claude Code, Anthropic's official CLI for Claude.",
        "You are an interactive CLI tool that helps users",
    }
    for _, main := range mainAgentPrefixes {
        for _, sub := range claudeCodeSubAgentPrefixes {
            if sub == main {
                t.Errorf("claudeCodeSubAgentPrefixes contains main-agent prefix: %q", sub)
            }
        }
    }
}

func TestClaudeCodeSubAgentSDKMarker_CoversAgentSDKVariant(t *testing.T) {
    // "CC + Agent SDK" 变体必须被 claudeCodeSubAgentSDKMarker 覆盖
    agentSDKVariant := "You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK."
    if !strings.Contains(agentSDKVariant, claudeCodeSubAgentSDKMarker) {
        t.Errorf("claudeCodeSubAgentSDKMarker %q does not match CC+AgentSDK variant", claudeCodeSubAgentSDKMarker)
    }
    // 主 agent 标准变体不应被 marker 误命中
    mainAgentPrompt := "You are Claude Code, Anthropic's official CLI for Claude."
    if strings.Contains(mainAgentPrompt, claudeCodeSubAgentSDKMarker) {
        t.Errorf("claudeCodeSubAgentSDKMarker falsely matches main-agent prompt")
    }
}

// TestClaudeCodeSubAgentPrefixes_SubsetOfPromptPrefixes 验证 claudeCodeSubAgentPrefixes
// 中的每一条字面量都存在于 claudeCodePromptPrefixes 中，确保两套列表不脱钩。
// 当 Claude Code 升级 prompt 模板时，只需更新 claudeCodePromptPrefixes，
// 本测试会立刻捕获 claudeCodeSubAgentPrefixes 未同步的情况。
func TestClaudeCodeSubAgentPrefixes_SubsetOfPromptPrefixes(t *testing.T) {
    for _, sub := range claudeCodeSubAgentPrefixes {
        found := false
        for _, p := range claudeCodePromptPrefixes {
            if strings.HasPrefix(p, sub) || strings.HasPrefix(sub, p) || p == sub {
                found = true
                break
            }
        }
        if !found {
            t.Errorf("claudeCodeSubAgentPrefixes entry %q has no counterpart in claudeCodePromptPrefixes — update both lists together", sub)
        }
    }
}
```

- [ ] **Step 0.1：写完测试，保存**

### Step 0.2：运行测试，确认失败（变量未定义）

```bash
cd /path/to/sub2api/backend
go test ./internal/service/ -run "TestClaudeCodeSubAgentPrefixes|TestClaudeCodeSubAgentSDKMarker" -v
```

期望：`undefined: claudeCodeSubAgentPrefixes` 和 `undefined: claudeCodeSubAgentSDKMarker`

- [ ] **Step 0.2：确认失败**

### Step 0.3：在 `gateway_service.go` 中新增 `claudeCodeSubAgentPrefixes`

找到 `gateway_service.go` 约第 336 行的 `claudeCodePromptPrefixes`，在其**正下方**插入：

```go
// claudeCodeSubAgentPrefixes 是 Claude Code sub-agent 系统提示词的前缀列表。
// sub-agent 由 Claude Code 多代理调度层自动 spawn，其首次请求不应消耗 Premium
// Interaction 配额，应路由到免费 Standard 配额。
//
// 注意：不直接引用 claudeCodePromptPrefixes 的索引，因为 claudeCodePromptPrefixes[0]
// 同时是主 agent 和 "CC + Agent SDK" 变体的共同前缀，无法通过索引区分。
// "CC + Agent SDK" 变体（含 "running within the Claude Agent SDK"）通过
// claudeCodeSubAgentSDKMarker 额外检测，不在此列。
var claudeCodeSubAgentPrefixes = []string{
    "You are a Claude agent, built on Anthropic's Claude Agent SDK",
    "You are a file search specialist for Claude Code",
    "You are a helpful AI assistant tasked with summarizing conversations",
}

// claudeCodeSubAgentSDKMarker 用于识别 "CC + Agent SDK" 变体的 sub-agent。
// 这类 prompt 以主 agent 前缀开头（无法通过 claudeCodeSubAgentPrefixes 区分），
// 但包含此特征标记，表明是由 Agent SDK 调度层 spawn 的子任务。
const claudeCodeSubAgentSDKMarker = "running within the Claude Agent SDK"
```

> **说明：** 直接写字面量，不靠 `claudeCodePromptPrefixes` 索引，避免因 `claudeCodePromptPrefixes` 顺序调整导致静默错配。`claudeCodeSubAgentSDKMarker` 单独抽出以便测试和日后维护。

- [ ] **Step 0.3：写完实现，保存**

### Step 0.4：运行测试，确认通过

```bash
go test ./internal/service/ -run TestClaudeCodeSubAgentPrefixes -v
```

期望：`PASS`

- [ ] **Step 0.4：确认通过**

### Step 0.5：提交

按仓库提交协议提交：类型 `Feature`，中文描述。

```bash
git add backend/internal/service/gateway_service.go \
        backend/internal/service/copilot_subagent_test.go
```

- [ ] **Step 0.5：提交**

---

## Task 1：新增辅助函数 `isClaudeCodeSubAgentSystemPrompt`

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`（在 `copilotInitiator` 函数后面，约第 1327 行附近）
- Test: `backend/internal/service/copilot_gateway_service_test.go`

### Step 1.1：先写失败的单元测试

在 `TestCopilotInitiator` 函数**之前**，找到文件里 `func TestCopilotInitiator` 的位置（约第 45 行），在其**前面**插入新测试函数。

打开 `backend/internal/service/copilot_gateway_service_test.go`，在 `TestCopilotInitiator`（第 45 行）之前添加：

```go
func TestIsClaudeCodeSubAgentSystemPrompt(t *testing.T) {
    tests := []struct {
        name    string
        content string
        want    bool
    }{
        // ── sub-agent system prompts（期望返回 true）──────────────────
        {
            name:    "Agent SDK sub-agent",
            content: "You are a Claude agent, built on Anthropic's Claude Agent SDK.",
            want:    true,
        },
        {
            name:    "CC + Agent SDK sub-agent",
            content: "You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK.",
            want:    true,
        },
        {
            name:    "Explore agent (file search specialist)",
            content: "You are a file search specialist for Claude Code, Anthropic's official CLI for Claude.",
            want:    true,
        },
        {
            name:    "summarizing agent",
            content: "You are a helpful AI assistant tasked with summarizing conversations.",
            want:    true,
        },
        // ── 主 agent system prompts（期望返回 false）─────────────────
        {
            name:    "main agent primary prompt",
            content: "You are Claude Code, Anthropic's official CLI for Claude.",
            want:    false,
        },
        {
            name:    "main agent interactive prompt",
            content: "You are an interactive CLI tool that helps users with software engineering tasks.",
            want:    false,
        },
        // ── 关键反例：非 Claude Code 客户端伪造 sub-agent 关键字 ─────
        // （此测试在 TestCopilotInitiator 层面验证，这里只验证字符串函数本身
        //   不区分客户端来源 — 客户端门控在 copilotInitiator 的 isClaudeCode 参数）
        {
            name:    "generic helpful assistant (not CC)",
            content: "You are a helpful assistant.",
            want:    false,
        },
        {
            name:    "empty content",
            content: "",
            want:    false,
        },
        // ── 边界：content 为数组时调用方不应传入此函数（由 copilotInitiator 过滤）
        {
            name:    "Agent SDK with leading whitespace",
            content: "  You are a Claude agent, built on Anthropic's Claude Agent SDK.  ",
            want:    true,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := isClaudeCodeSubAgentSystemPrompt(tt.content)
            if got != tt.want {
                t.Errorf("isClaudeCodeSubAgentSystemPrompt(%q) = %v, want %v", tt.content, got, tt.want)
            }
        })
    }
}
```

- [ ] **Step 1.1：写完上述测试，保存文件**

### Step 1.2：运行测试，确认失败

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/ -run TestIsClaudeCodeSubAgentSystemPrompt -v
```

期望输出：
```
FAIL: undefined: isClaudeCodeSubAgentSystemPrompt
```

- [ ] **Step 1.2：确认测试因函数未定义而失败**

### Step 1.3：实现 `isClaudeCodeSubAgentSystemPrompt` 函数

在 `backend/internal/service/copilot_gateway_service.go` 中，找到 `copilotInitiator` 函数（约第 1312 行），在其**之后**（约 1327 行，`copilotInitiatorFromResponsesBody` 函数之前）插入：

```go
// isClaudeCodeSubAgentSystemPrompt reports whether the given system message
// content belongs to a Claude Code sub-agent (spawned automatically by the
// multi-agent orchestration layer).
//
// Detection uses claudeCodeSubAgentPrefixes and claudeCodeSubAgentSDKMarker
// (gateway_service.go) as the single source of truth — do NOT add literal
// strings here.
//
// Two detection paths:
//   1. HasPrefix match against claudeCodeSubAgentPrefixes — covers the three
//      pure sub-agent prompt variants.
//   2. Contains match for claudeCodeSubAgentSDKMarker — covers the "CC + Agent
//      SDK" variant whose prefix is shared with the main-agent prompt.
//
// Called only when the request is already confirmed to be from a Claude Code
// client (isClaudeCode == true in copilotInitiator).
func isClaudeCodeSubAgentSystemPrompt(content string) bool {
    trimmed := strings.TrimSpace(content)
    for _, prefix := range claudeCodeSubAgentPrefixes {
        if strings.HasPrefix(trimmed, prefix) {
            return true
        }
    }
    // "CC + Agent SDK" variant: prefix shared with main agent, distinguish by marker
    return strings.Contains(trimmed, claudeCodeSubAgentSDKMarker)
}
```

- [ ] **Step 1.3：写完实现，保存文件**

### Step 1.4：运行测试，确认通过

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/ -run TestIsClaudeCodeSubAgentSystemPrompt -v
```

期望输出：
```
--- PASS: TestIsClaudeCodeSubAgentSystemPrompt (0.00s)
    --- PASS: TestIsClaudeCodeSubAgentSystemPrompt/Agent_SDK_sub-agent (0.00s)
    --- PASS: TestIsClaudeCodeSubAgentSystemPrompt/CC_+_Agent_SDK_sub-agent (0.00s)
    --- PASS: TestIsClaudeCodeSubAgentSystemPrompt/Explore_agent_(file_search_specialist) (0.00s)
    --- PASS: TestIsClaudeCodeSubAgentSystemPrompt/summarizing_agent (0.00s)
    --- PASS: TestIsClaudeCodeSubAgentSystemPrompt/main_agent_primary_prompt (0.00s)
    --- PASS: TestIsClaudeCodeSubAgentSystemPrompt/main_agent_interactive_prompt (0.00s)
    --- PASS: TestIsClaudeCodeSubAgentSystemPrompt/generic_helpful_assistant_(not_CC) (0.00s)
    --- PASS: TestIsClaudeCodeSubAgentSystemPrompt/empty_content (0.00s)
    --- PASS: TestIsClaudeCodeSubAgentSystemPrompt/Agent_SDK_with_leading_whitespace (0.00s)
PASS
```

- [ ] **Step 1.4：确认所有测试通过**

### Step 1.5：提交

```bash
cd /Users/ziji/personal/github/sub2api
git add backend/internal/service/copilot_gateway_service.go \
        backend/internal/service/copilot_gateway_service_test.go
# 按仓库提交协议提交（类型: Feature，中文描述）
```

- [ ] **Step 1.5：提交**

---

## Task 2：修改 `copilotInitiator` 集成子 agent 检测逻辑

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`（`copilotInitiator` 函数，约第 1312 行；`CopilotInitiatorFromBody` 约第 1297 行）
- Test: `backend/internal/service/copilot_gateway_service_test.go`（`TestCopilotInitiator` 约第 45 行）

### Step 2.1：先在 `TestCopilotInitiator` 中添加新测试用例（失败状态）

找到 `backend/internal/service/copilot_gateway_service_test.go` 中的 `TestCopilotInitiator` 函数（约第 45 行）。

> **注意：** `copilotInitiator` 新签名为 `copilotInitiator(body []byte, isClaudeCode bool)`，测试中需要同步更新所有调用处，并在 sub-agent 用例中传 `true`，在反例中传 `false`。

在 `tests` slice 的**末尾**追加以下用例：

```go
        // ── Claude Code sub-agent system prompt + isClaudeCode=true → agent（免费）
        {
            "sub-agent: Agent SDK prompt – CC client first turn",
            `{"messages":[{"role":"system","content":"You are a Claude agent, built on Anthropic's Claude Agent SDK."},{"role":"user","content":"search for X"}]}`,
            true,  // isClaudeCode
            "agent",
        },
        {
            "sub-agent: Explore agent – CC client first turn",
            `{"messages":[{"role":"system","content":"You are a file search specialist for Claude Code, Anthropic's official CLI for Claude."},{"role":"user","content":"find files matching *.go"}]}`,
            true,
            "agent",
        },
        {
            "sub-agent: summarizing agent – CC client first turn",
            `{"messages":[{"role":"system","content":"You are a helpful AI assistant tasked with summarizing conversations."},{"role":"user","content":"summarize the above"}]}`,
            true,
            "agent",
        },
        {
            "sub-agent: CC + Agent SDK – CC client first turn",
            `{"messages":[{"role":"system","content":"You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK."},{"role":"user","content":"do task"}]}`,
            true,
            "agent",
        },
        // ── 主 agent system prompt + isClaudeCode=true → user（Premium，正确行为）
        {
            "main agent: primary prompt – CC client first turn stays Premium",
            `{"messages":[{"role":"system","content":"You are Claude Code, Anthropic's official CLI for Claude."},{"role":"user","content":"hello"}]}`,
            true,
            "user",
        },
        // ── 关键反例：isClaudeCode=false（UA 不含 claude-cli 或严格校验失败）→ user
        {
            "NEGATIVE: forged sub-agent prompt from non-CC client → user",
            `{"messages":[{"role":"system","content":"You are a Claude agent, built on Anthropic's Claude Agent SDK."},{"role":"user","content":"search for X"}]}`,
            false, // isClaudeCode = false
            "user",
        },
        {
            "NEGATIVE: forged explore agent prompt from non-CC client → user",
            `{"messages":[{"role":"system","content":"You are a file search specialist for Claude Code, Anthropic's official CLI for Claude."},{"role":"user","content":"find files"}]}`,
            false,
            "user",
        },
        // ── 已知弱门控边界（文档明确）：ChatCompletions 路径 UA-only 检测
        // isClaudeCode=true 时，即使是伪造 claude-cli UA 的客户端也会命中
        // 这是与 ClaudeCodeValidator.Validate() 在非 messages 路径的行为一致的设计决策
        {
            "KNOWN WEAK GATE: isClaudeCode=true allows any client with claude-cli UA to route as agent",
            `{"messages":[{"role":"system","content":"You are a Claude agent, built on Anthropic's Claude Agent SDK."},{"role":"user","content":"search"}]}`,
            true, // 即使是伪造 UA 的客户端，copilotInitiator 本身无法区分
            "agent",
        },
        // ── system content 为数组（非 string）→ 不 panic，跳过检测
        {
            "system content is array – no panic, fall through to role check",
            `{"messages":[{"role":"system","content":[{"type":"text","text":"You are a Claude agent, built on Anthropic's Claude Agent SDK."}]},{"role":"user","content":"q"}]}`,
            true,
            "user",  // content 不是字符串，跳过 sub-agent 检测；无 assistant/tool → user
        },
        // ── sub-agent 多轮（assistant 消息）→ agent（两个条件均满足）
        {
            "sub-agent with prior assistant turn → still agent",
            `{"messages":[{"role":"system","content":"You are a Claude agent, built on Anthropic's Claude Agent SDK."},{"role":"user","content":"q1"},{"role":"assistant","content":"a1"},{"role":"user","content":"q2"}]}`,
            true,
            "agent",
        },
```

同时，将 `tests` slice 的结构体定义从二字段改为三字段（加 `isClaudeCode bool`），并更新 `t.Run` 内的调用：

```go
// 原结构体（示意，实际根据文件内容调整）：
// { name string; body string; want string }
// 改为：
// { name string; body string; isClaudeCode bool; want string }

// 原调用：
// got := copilotInitiator([]byte(tt.body))
// 改为：
// got := copilotInitiator([]byte(tt.body), tt.isClaudeCode)
```

- [ ] **Step 2.1：添加测试用例 + 更新结构体和调用，保存文件**

### Step 2.2：运行新增测试，确认当前失败

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/ -run TestCopilotInitiator -v
```

期望：sub-agent 相关用例 FAIL（期望 "agent"，实际返回 "user"）。

- [ ] **Step 2.2：确认失败**

### Step 2.3：修改 `copilotInitiator` 和 `CopilotInitiatorFromBody`

找到 `backend/internal/service/copilot_gateway_service.go` 中的两个函数（约第 1297 和 1312 行），整体替换：

**（a）替换 `CopilotInitiatorFromBody`（公开函数，供 handler 调用）：**

```go
// CopilotInitiatorFromBody returns the X-Initiator value for an OpenAI-format
// request body. Used by the ChatCompletions handler for analytics recording.
//
// Client detection is UA-only (weak gate): any client that sets
// User-Agent: claude-cli/x.y.z will be treated as Claude Code.
// This matches the behaviour of ClaudeCodeValidator.Validate() on non-messages
// paths (Step 2: UA match → pass). The stronger multi-header check is only
// applied on the Anthropic /messages path via CopilotInitiatorFromAnthropicBody.
func CopilotInitiatorFromBody(openAIBody []byte, userAgent string) string {
    isClaudeCode := NewClaudeCodeValidator().ValidateUserAgent(userAgent)
    return copilotInitiator(openAIBody, isClaudeCode)
}
```

**（b）替换 `copilotInitiator`（私有实现，也供 service 内部调用）：**

```go
// copilotInitiator returns the value for the X-Initiator header.
//
// Quota routing (highest-priority first):
//
//  1. Sub-agent system prompt AND isClaudeCode=true — "agent" (free).
//     The isClaudeCode gate is set by the caller: UA-only for ChatCompletions
//     paths (weak gate), full Validate() for Anthropic Messages path (strong gate).
//     Callers are responsible for choosing the appropriate validation strength.
//
//  2. Multi-turn conversation (assistant or tool message present) — "agent".
//     Original heuristic from the TypeScript copilot-api reference.
//
//  3. Everything else — "user" (Premium Interaction quota).
func copilotInitiator(openAIBody []byte, isClaudeCode bool) string {
    var req struct {
        Messages []struct {
            Role    string          `json:"role"`
            Content json.RawMessage `json:"content"`
        } `json:"messages"`
    }
    if err := json.Unmarshal(openAIBody, &req); err != nil {
        return "user"
    }
    for _, m := range req.Messages {
        switch m.Role {
        case "system":
            if isClaudeCode {
                var text string
                if err := json.Unmarshal(m.Content, &text); err == nil {
                    if isClaudeCodeSubAgentSystemPrompt(text) {
                        return "agent"
                    }
                }
                // content 不是字符串（如数组）：跳过，不 panic
            }
        case "assistant", "tool":
            return "agent"
        }
    }
    return "user"
}
```

**（c）同步更新 service 内部所有调用处（在同文件中搜索 `copilotInitiator(`）：**

这些函数分两类，门控强度不同：

**ChatCompletions 路径**（第 280、436、576 行，`forwardChatCompletionsDirect` / `forwardChatCompletionsViaResponses` / `forwardChatCompletionsViaMessages`）：

```go
// 修改前：
initiator := copilotInitiator(body)
// 修改后（UA 级门控；对非 messages 路径，Validate 本身也只做 UA 检查）：
initiator := copilotInitiator(body, NewClaudeCodeValidator().ValidateUserAgent(c.GetHeader("User-Agent")))
```

**Anthropic Messages 路径**（第 2062、2199 行，`ForwardMessages` / `forwardMessagesViaResponses`）：

`ForwardMessages` 收到的是 Anthropic 格式请求，此路径适合做更强的客户端验证。将 UA-only 升级为完整 `Validate()` 调用：

```go
// 修改前：
initiator := copilotInitiator(openAIBody)
// 修改后（完整验证，含 system prompt 相似度 + headers 检查）：
// 注意：传入 anthropicBody（原始 Anthropic 格式），不是 openAIBody（已翻译为 OpenAI 格式）
isCC := NewClaudeCodeValidator().Validate(c.Request, anthropicBodyToValidatorMap(anthropicBody))
initiator := copilotInitiator(openAIBody, isCC)
```

在同文件（`copilot_gateway_service.go`）新增一个小 helper，直接从 Anthropic body bytes 构造 validator 所需的 body map（**注意**：不依赖 gin.Context 缓存——Copilot handler 未设置 `OpenAIParsedRequestBodyKey`，读取缓存会始终拿到空 map）：

```go
// anthropicBodyToValidatorMap deserialises raw Anthropic request bytes into a
// map[string]any suitable for ClaudeCodeValidator.Validate.
//
// Using json.Unmarshal into map[string]any is intentional: the decoder
// automatically produces []any for JSON arrays and map[string]any for JSON
// objects, which matches the type assertions inside hasClaudeCodeSystemPrompt:
//   body["model"].(string)          — plain string, preserved as-is
//   body["system"].([]any)          — JSON array → []any  ✓
//   entry.(map[string]any)          — JSON object → map[string]any  ✓
//   body["metadata"].(map[string]any) — JSON object → map[string]any  ✓
//
// A hand-rolled struct with json.RawMessage fields would NOT satisfy these
// assertions, so we unmarshal the full body and rely on json's native mapping.
func anthropicBodyToValidatorMap(body []byte) map[string]any {
    var m map[string]any
    if err := json.Unmarshal(body, &m); err != nil {
        return map[string]any{}
    }
    return m
}
```

> **门控说明：**
> - ChatCompletions 路径（OpenAI format）：VA 只做 UA 正则匹配，这是**弱门控**——任意客户端伪造 `claude-cli/x.y.z` UA 即可通过。这与 `Validate()` 在非 messages 路径下的行为完全等效（Step 2 直接返回 true），不是倒退，只是继承了已有设计的边界。
> - Anthropic Messages 路径：使用完整 `Validate()`，额外校验 system prompt 相似度、`X-App`、`anthropic-beta`、`anthropic-version`、`metadata.user_id`，伪造难度显著更高。body map 由 `anthropicBodyToValidatorMap(body)` 直接从 Anthropic body bytes 构造，不依赖 handler 层缓存（Copilot handler 不设置 `OpenAIParsedRequestBodyKey`，依赖缓存会使 `Validate()` 始终拿到空 map）。
> - 计划中不声称"完全无法伪造"，而是收紧到"与仓库现有 Claude Code 检测能力保持一致"。

- [ ] **Step 2.3：替换函数实现 + 更新内部调用，保存文件**

### Step 2.4：运行测试，确认全部通过

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/ -run TestCopilotInitiator -v
```

期望：所有用例 PASS，包括原有用例和新增的 sub-agent 用例。

- [ ] **Step 2.4：确认测试通过**

### Step 2.5：运行全量相关测试，确认无回归

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/ -run "TestCopilotInitiator|TestXInitiatorHeader|TestIsClaudeCode" -v
```

期望：所有匹配测试 PASS。

- [ ] **Step 2.5：确认无回归**

### Step 2.6：提交

```bash
cd /Users/ziji/personal/github/sub2api
git add backend/internal/service/copilot_gateway_service.go \
        backend/internal/service/copilot_gateway_service_test.go
# 按仓库提交协议提交（类型: Feature，中文描述）
```

- [ ] **Step 2.6：提交**

---

## Task 3：更新 Messages 路径的集成测试，覆盖 sub-agent 场景

**Files:**
- Test: `backend/internal/service/copilot_gateway_service_test.go`（`TestXInitiatorHeader_MessagesEndpoint` 约第 1706 行）

### Step 3.1：扩展 `TestXInitiatorHeader_MessagesEndpoint` 的 case struct，加入 `userAgent` 字段

当前 case struct 只有 `name`、`body`、`wantInitiator`。Messages 路径使用**强门控**（完整 `Validate()`），正向 sub-agent 用例必须同时携带：
- Claude Code `User-Agent`
- `X-App: claude.ai` header
- `anthropic-beta` header（如 `computer-use-2024-10-22`）
- `anthropic-version` header（如 `2023-06-01`）
- body 中的 `metadata.user_id`（必须通过 `ParseMetadataUserID()` 校验，即符合 legacy `user_{64hex}_account_{uuid?}_session_{uuid}` 或 JSON `{"device_id":..., "session_id":...}` 格式；普通短字符串会被 validator 拒绝）

**（a）找到 `TestXInitiatorHeader_MessagesEndpoint` 函数内部的 case struct 定义（约第 1706 行），把结构体改为：**

```go
cases := []struct {
    name          string
    body          string
    userAgent     string // "" → 使用默认 Claude Code UA；"non-cc" → 使用非 CC UA
    wantInitiator string
}{
```

**（b）在 test loop 里（`c.Request = httptest.NewRequest(...)` 之后）根据字段设置 UA 和强门控所需 headers：**

```go
// 设置 User-Agent：默认使用 Claude Code UA；字段为 "non-cc" 时用非 CC UA
ua := "claude-cli/2.1.0"
if tc.userAgent == "non-cc" {
    ua = "Mozilla/5.0"
}
c.Request.Header.Set("User-Agent", ua)

// 设置强门控所需 headers（Anthropic Messages 路径 Validate() 检查项）
c.Request.Header.Set("X-App", "claude.ai")
c.Request.Header.Set("anthropic-beta", "computer-use-2024-10-22")
c.Request.Header.Set("anthropic-version", "2023-06-01")
```

> **注意：** 强门控所需 headers 对**所有** case 统一设置（正向 + 反向）——`Validate()` 仅在 UA 已通过时才检查后续字段，所以非 CC UA 的 case 仍会因 UA 失败而返回 false，不受多余 headers 影响。现有 assistant/tool 多轮用例也不影响：多轮判断先于 isClaudeCode 检查，先遇到 `assistant` role 就直接返回 `"agent"`。

- [ ] **Step 3.1：修改 case struct 和 test loop，保存文件**

### Step 3.2：在 `cases` slice 末尾追加 sub-agent 测试用例

正向 case 的 body 需要包含满足 `ParseMetadataUserID()` 的合法 `metadata.user_id`（`Validate()` Step 4.3 会调用该函数，返回 nil 即失败；"非空字符串"不够，必须符合 legacy 或 JSON 格式）。

使用 legacy 格式：`user_<64位hex>_account_<可选uuid>_session_<uuid>`，直接复用 `claude_code_detection_test.go:29` 中已验证的样例：

```go
// testValidUserID 是满足 ParseMetadataUserID() 的合法 legacy 格式样例。
// 格式：user_{64hex}_account_{optional_uuid}_session_{uuid}
// 参见 backend/internal/service/claude_code_detection_test.go:29
const testValidUserID = `user_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2_account__session_12345678-1234-1234-1234-123456789abc`
```

在 body 中内联该值（Go raw string 写法）：

```go
        {
            name:          "sub-agent: Agent SDK system prompt – CC client first turn → Standard quota",
            body:          `{"model":"claude-sonnet-4-5","max_tokens":1024,"system":[{"type":"text","text":"You are a Claude agent, built on Anthropic's Claude Agent SDK."}],"messages":[{"role":"user","content":"search for all Go test files"}],"metadata":{"user_id":"user_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2_account__session_12345678-1234-1234-1234-123456789abc"}}`,
            // userAgent 为空 → 使用默认 claude-cli/2.1.0
            wantInitiator: "agent",
        },
        {
            name:          "sub-agent: Explore agent system prompt – CC client first turn → Standard quota",
            body:          `{"model":"claude-sonnet-4-5","max_tokens":1024,"system":[{"type":"text","text":"You are a file search specialist for Claude Code, Anthropic's official CLI for Claude."}],"messages":[{"role":"user","content":"find *.go files"}],"metadata":{"user_id":"user_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2_account__session_12345678-1234-1234-1234-123456789abc"}}`,
            wantInitiator: "agent",
        },
        {
            name:          "sub-agent: CC + Agent SDK variant – CC client first turn → Standard quota",
            body:          `{"model":"claude-sonnet-4-5","max_tokens":1024,"system":[{"type":"text","text":"You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK."}],"messages":[{"role":"user","content":"do task"}],"metadata":{"user_id":"user_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2_account__session_12345678-1234-1234-1234-123456789abc"}}`,
            wantInitiator: "agent",
        },
        {
            name:          "main agent first turn – stays Premium quota",
            body:          `{"model":"claude-sonnet-4-5","max_tokens":1024,"system":[{"type":"text","text":"You are Claude Code, Anthropic's official CLI for Claude."}],"messages":[{"role":"user","content":"help me refactor this function"}],"metadata":{"user_id":"user_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2_account__session_12345678-1234-1234-1234-123456789abc"}}`,
            wantInitiator: "user",
        },
        {
            name:          "NEGATIVE: forged sub-agent prompt from non-CC client → Premium quota",
            body:          `{"model":"claude-sonnet-4-5","max_tokens":1024,"system":[{"type":"text","text":"You are a Claude agent, built on Anthropic's Claude Agent SDK."}],"messages":[{"role":"user","content":"search for all Go test files"}],"metadata":{"user_id":"user_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2_account__session_12345678-1234-1234-1234-123456789abc"}}`,
            userAgent:     "non-cc",
            wantInitiator: "user",
        },
```

- [ ] **Step 3.2：追加测试用例，保存文件**

### Step 3.3：运行集成测试，确认新用例通过

```bash
cd /path/to/sub2api/backend
go test ./internal/service/ -run TestXInitiatorHeader_MessagesEndpoint -v -timeout 60s
```

期望：所有新增用例 PASS，包括反例（非 CC UA + 伪造 prompt → `user`）。

- [ ] **Step 3.3：确认全部通过**

### Step 3.4：运行全量 service 包测试，确认无回归

```bash
cd /path/to/sub2api/backend
go test ./internal/service/ -timeout 120s -count=1
```

期望：`ok      github.com/Wei-Shaw/sub2api/internal/service`（无 FAIL）。

- [ ] **Step 3.4：确认全量测试无回归**

### Step 3.5：提交

```bash
git add backend/internal/service/copilot_gateway_service_test.go
# 按仓库提交协议提交（类型: Feature，中文描述）
```

- [ ] **Step 3.5：提交**

---

## Task 4：修复 handler 层 `CopilotInitiatorFromBody` 调用，使统计口径与上游 X-Initiator 一致（M1）

**Files:**
- Modify: `backend/internal/handler/copilot_gateway_handler.go`（第 370、815、1243 行）

**背景：** handler 层有三处调用 `service.CopilotInitiatorFromBody(body)` 把结果写入 `RecordUsage.Initiator`，该值最终进入 `usage_logs.initiator`，被 analytics 服务聚合为 premium/agent 用量看板。若 handler 侧不跟随 service 侧的修复同步传入 UA，统计口径会和上游实际 `X-Initiator` 产生分叉，导致修复后报表仍然误报。

`CopilotInitiatorFromBody` 已在 Task 2 中改为 `CopilotInitiatorFromBody(body []byte, userAgent string)`，本 Task 同步更新 handler 侧的三处调用。

### Step 4.1：更新三处 `CopilotInitiatorFromBody` 调用

打开 `backend/internal/handler/copilot_gateway_handler.go`，搜索 `CopilotInitiatorFromBody`，共三处：

**第 370 行（ChatCompletions 路径，OpenAI body）：**
```go
// 修改前：
capturedInitiator := service.CopilotInitiatorFromBody(body)

// 修改后：
capturedInitiator := service.CopilotInitiatorFromBody(body, c.GetHeader("User-Agent"))
```

**第 815 行（Responses 路径）：**

Responses body 使用 `input` + `previous_response_id` 格式，不是 OpenAI messages 格式。`CopilotInitiatorFromBody` 会错误解析此格式，需改用对应的 Responses 版本 wrapper：

```go
// 修改前：
capturedInitiatorResp := service.CopilotInitiatorFromBody(body)

// 修改后：
capturedInitiatorResp := service.CopilotInitiatorFromResponsesBody(body)
```

同时在 `backend/internal/service/copilot_gateway_service.go` 中新增（紧靠 `copilotInitiatorFromResponsesBody` 之后）：

```go
// CopilotInitiatorFromResponsesBody is the public wrapper around
// copilotInitiatorFromResponsesBody, used by the Responses handler for
// analytics recording — keeping it aligned with the actual upstream logic.
// Note: the Responses path is used by Codex CLI, not Claude Code sub-agents,
// so no Claude Code UA gate is applied here.
func CopilotInitiatorFromResponsesBody(body []byte) string {
    return copilotInitiatorFromResponsesBody(body)
}
```

**第 1243 行（Messages 路径，Anthropic body）：**

Messages 路径的 `body` 是 Anthropic 格式（含 `system` 字段，不含 `messages[].role=system`），而 `CopilotInitiatorFromBody` 接受 OpenAI 格式。需要改用新函数 `CopilotInitiatorFromAnthropicBody`：

```go
// 修改前：
capturedInitiatorMsg := service.CopilotInitiatorFromBody(body)

// 修改后（传入完整 gin.Context 以启用强验证）：
capturedInitiatorMsg := service.CopilotInitiatorFromAnthropicBody(body, c)
```

同时在 `backend/internal/service/copilot_gateway_service.go` 中新增：

```go
// CopilotInitiatorFromAnthropicBody returns the X-Initiator value for an
// Anthropic-format request body (used by the /v1/messages handler for
// analytics recording — the actual upstream call uses the translated OpenAI
// body via copilotInitiator).
//
// Client detection uses the full ClaudeCodeValidator.Validate() (UA + system
// prompt similarity + headers), matching the strength of the main gateway path.
// Uses extractSystemText (copilot_anthropic_translation.go) to parse the system
// field — same helper used by the real translation path.
func CopilotInitiatorFromAnthropicBody(anthropicBody []byte, c *gin.Context) string {
    // Full validation: UA + system prompt similarity + X-App/anthropic-* headers.
    // Falls back to false if c is nil (safe default → no sub-agent gate).
    // anthropicBodyToValidatorMap extracts system + metadata directly from body
    // bytes, bypassing the Context cache that Copilot handler never sets.
    isClaudeCode := false
    if c != nil && c.Request != nil {
        bodyMap := anthropicBodyToValidatorMap(anthropicBody)
        isClaudeCode = NewClaudeCodeValidator().Validate(c.Request, bodyMap)
    }

    var req struct {
        System   json.RawMessage `json:"system"`
        Messages []struct {
            Role string `json:"role"`
        } `json:"messages"`
    }
    if err := json.Unmarshal(anthropicBody, &req); err != nil {
        return "user"
    }

    // Multi-turn check: Anthropic assistant role maps directly.
    for _, m := range req.Messages {
        if m.Role == "assistant" {
            return "agent"
        }
    }

    // Sub-agent system prompt check (CC clients only, strong gate).
    if isClaudeCode && len(req.System) > 0 {
        if isClaudeCodeSubAgentSystemPrompt(extractSystemText(req.System)) {
            return "agent"
        }
    }
    return "user"
}
```

- [ ] **Step 4.1：更新 handler 三处调用 + 新增 `CopilotInitiatorFromAnthropicBody`，保存文件**

### Step 4.2：确认编译通过

```bash
cd /path/to/sub2api/backend
go build ./...
```

期望：无编译错误。

- [ ] **Step 4.2：确认编译通过**

### Step 4.3：运行相关测试

```bash
go test ./internal/service/ ./internal/handler/ -timeout 60s -count=1 2>&1 | tail -20
```

期望：无 FAIL。

- [ ] **Step 4.3：确认测试通过**

### Step 4.4：提交

按仓库提交协议提交。

```bash
git add backend/internal/handler/copilot_gateway_handler.go \
        backend/internal/service/copilot_gateway_service.go
```

- [ ] **Step 4.4：提交**

---

## Task 5：同步更新 ChatCompletions 路径集成测试（新增反例）

**Files:**
- Test: `backend/internal/service/copilot_gateway_service_test.go`（`TestXInitiatorHeader_ChatCompletions` 约第 1549 行）

### Step 5.1：扩展 `TestXInitiatorHeader_ChatCompletions` 的 case struct，加入 `userAgent` 字段

当前 case struct 只有 `name`、`body`、`wantInitiator`。

**（a）找到 case struct 定义（约第 1550 行），改为：**

```go
cases := []struct {
    name          string
    body          string
    userAgent     string // "" → 使用默认 Claude Code UA；"non-cc" → 使用非 CC UA
    wantInitiator string
}{
```

**（b）在 test loop 里（`c.Request = httptest.NewRequest(...)` 之后）设置 UA：**

```go
// 设置 User-Agent
ua := "claude-cli/2.1.0"
if tc.userAgent == "non-cc" {
    ua = "Mozilla/5.0"
}
c.Request.Header.Set("User-Agent", ua)
```

> **注意：** 现有三条正向用例（first turn、multi-turn assistant、tool result）不设 `userAgent`，走默认 CC UA。multi-turn 和 tool result 用例靠 `assistant`/`tool` 角色命中，不依赖 `isClaudeCode`，行为不受影响。

- [ ] **Step 5.1：修改 case struct 和 test loop，保存文件**

### Step 5.2：在 `cases` slice 末尾追加 sub-agent 用例和反例

```go
        {
            name:          "sub-agent: Explore agent in OpenAI body – CC client – first turn → agent",
            body:          `{"model":"claude-sonnet-4-5","stream":false,"messages":[{"role":"system","content":"You are a file search specialist for Claude Code, Anthropic's official CLI for Claude."},{"role":"user","content":"find all test files"}]}`,
            wantInitiator: "agent",
        },
        {
            name:          "sub-agent: Agent SDK in OpenAI body – CC client – first turn → agent",
            body:          `{"model":"claude-sonnet-4-5","stream":false,"messages":[{"role":"system","content":"You are a Claude agent, built on Anthropic's Claude Agent SDK."},{"role":"user","content":"execute task"}]}`,
            wantInitiator: "agent",
        },
        {
            name:          "sub-agent: CC + Agent SDK variant – CC client – first turn → agent",
            body:          `{"model":"claude-sonnet-4-5","stream":false,"messages":[{"role":"system","content":"You are Claude Code, Anthropic's official CLI for Claude, running within the Claude Agent SDK."},{"role":"user","content":"do task"}]}`,
            wantInitiator: "agent",
        },
        {
            name:          "main agent OpenAI body first turn – stays user",
            body:          `{"model":"claude-sonnet-4-5","stream":false,"messages":[{"role":"system","content":"You are Claude Code, Anthropic's official CLI for Claude."},{"role":"user","content":"hello"}]}`,
            wantInitiator: "user",
        },
        // ── 反例：非 CC UA + 伪造 sub-agent prompt → 必须是 user ──────
        {
            name:          "NEGATIVE: non-CC UA + forged sub-agent prompt → user",
            userAgent:     "non-cc",
            body:          `{"model":"claude-sonnet-4-5","stream":false,"messages":[{"role":"system","content":"You are a Claude agent, built on Anthropic's Claude Agent SDK."},{"role":"user","content":"execute task"}]}`,
            wantInitiator: "user",
        },
```

- [ ] **Step 5.2：追加测试用例，保存文件**

### Step 5.3：运行测试

```bash
cd /path/to/sub2api/backend
go test ./internal/service/ -run TestXInitiatorHeader_ChatCompletions -v -timeout 60s
```

期望：所有用例 PASS，包括正向 sub-agent 用例（CC UA）和反例（非 CC UA → `user`）。

- [ ] **Step 5.3：确认通过**

### Step 5.4：提交

```bash
git add backend/internal/service/copilot_gateway_service_test.go
# 按仓库提交协议提交（类型: Feature，中文描述）
```

- [ ] **Step 5.4：提交**

---

## Task 6：最终验证与构建

### Step 6.1：运行全量测试

```bash
cd /path/to/sub2api/backend
go test ./... -timeout 180s -count=1 2>&1 | tail -30
```

期望：所有包测试通过，无 FAIL。

- [ ] **Step 6.1：确认全量测试通过**

### Step 6.2：确认编译无错误

```bash
cd /path/to/sub2api/backend
go build ./...
```

期望：无输出（无编译错误）。

- [ ] **Step 6.2：确认编译通过**

### Step 6.3：运行 go vet

```bash
cd /path/to/sub2api/backend
go vet ./internal/service/ ./internal/handler/
```

期望：无输出（无 vet 警告）。

- [ ] **Step 6.3：确认 vet 通过**

---

## 关键设计决策说明

### 为什么 `copilotInitiatorFromResponsesBody` 不改但需要新增公开 wrapper？

`copilotInitiatorFromResponsesBody` 私有函数逻辑正确，解析 `input`/`previous_response_id` 格式。Responses handler 原来错误地用 `CopilotInitiatorFromBody`（OpenAI messages 格式解析器）来记录 analytics，导致口径分叉。新增 `CopilotInitiatorFromResponsesBody` 作为公开 wrapper，让 handler 正确使用对应的解析逻辑，同时不改动私有实现。

### 为什么 handler 侧也需要修改？

Handler 里三处调用的结果写入 `RecordUsage.Initiator`，最终进入 `usage_logs.initiator`，被 analytics 服务聚合为 premium/agent 用量看板（`copilot_analytics_service.go:211`、`500`）。若不同步修复，上游 `X-Initiator` 已改为 `agent`，但看板仍显示 `user` (premium)，造成分析口径分叉，排障时误导判断。

### ChatCompletions 路径（弱门控）与 Anthropic Messages 路径（强门控）的区别

本修复对两条路径的客户端验证强度不同，这是有意的设计选择：

- **ChatCompletions 路径**（OpenAI format）：只做 UA 正则匹配。任意伪造 `claude-cli/x.y.z` UA 的客户端均可命中弱门控。这与 `ClaudeCodeValidator.Validate()` 在非 messages 路径下的行为完全一致（Step 2：UA 匹配 → 直接通过）——不是本次修复引入的退步，而是继承了现有设计的已知边界。
- **Anthropic Messages 路径**（Anthropic format）：使用完整 `Validate()`，额外校验 system prompt 相似度、`X-App`、`anthropic-beta`、`anthropic-version`、`metadata.user_id`，伪造难度显著更高，与主 gateway 路径强度一致。

计划不声称"完全无法伪造"，而是"与仓库现有 Claude Code 检测能力保持一致"。

### 为什么 service 层不用 `IsClaudeCodeClient(ctx)` 而是直接校验？

`IsClaudeCodeClient(ctx)` 由 `SetClaudeCodeClientContext` 写入，但 Copilot handler 从未调用该函数（只有主 gateway handler 调用），因此 context 中标记始终为 false。改为 service 层直接从 `gin.Context` 即时校验，逻辑自洽，无需修改任何 handler 入口。

### 为什么 `claudeCodeSubAgentPrefixes` 不直接复用 `claudeCodePromptPrefixes` 的索引？

`claudeCodePromptPrefixes[0]`（`"You are Claude Code, Anthropic's official CLI for Claude"`）同时是主 agent 和 "CC + Agent SDK" 变体的共同前缀，无法靠索引区分。直接写独立字面量，并通过 `TestClaudeCodeSubAgentPrefixes_SubsetOfPromptPrefixes` 测试断言每一条在 `claudeCodePromptPrefixes` 中都有对应项，实现了"测试保证两套列表不脱钩"——如果 Claude Code 升级 prompt 模板，`claudeCodePromptPrefixes` 被更新后，该测试会立刻捕获 `claudeCodeSubAgentPrefixes` 未同步的情况。`claudeCodeSubAgentPrefixes` 作为**计费路由侧的唯一字面量来源**，`claudeCodePromptPrefixes` 继续作为 gateway 路由检测的来源，两者各司其职，用测试而非引用来保证一致性。

### 为什么 `CopilotInitiatorFromAnthropicBody` 复用 `extractSystemText` 而不新增 helper？

`extractSystemText`（`copilot_anthropic_translation.go:456`）是包内解析 Anthropic `system` 字段的标准实现，会拼接所有 text block。新增一个只取第一个 block 的 helper 会造成 analytics 和实际转发路径对 `system` 解释不一致——若 sub-agent marker 出现在第二个 block，实际会判成 `agent`，而 analytics 会记成 `user`。复用 `extractSystemText` 确保两条路径行为完全一致。

### 为什么选择 `strings.HasPrefix` + `strings.Contains` 两段逻辑？

三种"纯 sub-agent"前缀（Agent SDK、Explore、摘要）可用 `HasPrefix` 精确匹配，特异性强。"CC + Agent SDK"变体（`"You are Claude Code...running within the Claude Agent SDK."`）与主 agent 共享前缀，改用 `Contains` 检测 `claudeCodeSubAgentSDKMarker` 区分，主 agent 标准 prompt 不含该 marker，不会误命中。

### 为什么 "summarizing conversations" 也算 sub-agent？

对话摘要 agent 是 Claude Code 自动触发的上下文压缩机制（当对话过长时自动 spawn），不是用户主动发起的新对话。语义上属于工具调用，应走免费配额。

---

## 预期效果

| 请求类型 | 修改前 | 修改后 |
|---|---|---|
| 用户主动发起对话（主 agent 首轮） | `user` → Premium ✓ | `user` → Premium ✓（不变） |
| 主 agent 续轮（含 assistant/tool） | `agent` → 免费 ✓ | `agent` → 免费 ✓（不变） |
| Sub-agent 首轮（Agent SDK 系列） | `user` → Premium ✗ | `agent` → 免费 ✓（**修复**） |
| Sub-agent 首轮（Explore agent） | `user` → Premium ✗ | `agent` → 免费 ✓（**修复**） |
| Sub-agent 首轮（摘要 agent） | `user` → Premium ✗ | `agent` → 免费 ✓（**修复**） |
| Sub-agent 续轮（含 assistant/tool） | `agent` → 免费 ✓ | `agent` → 免费 ✓（不变） |
| **非 CC 客户端伪造 sub-agent prompt** | `user` → Premium | `user` → Premium ✓（**隔离**，不绕过） |

**理论减少幅度：** 一次 Claude Code 任务中，sub-agent 首次请求通常占 Premium 消耗的 **60-90%**，修复后这部分改走免费配额，Premium 消耗将大幅降低至接近"用户实际问题数"的水平。
