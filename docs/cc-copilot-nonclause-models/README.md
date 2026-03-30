# Claude Code 使用 Copilot 非 Claude 模型

> 本文档记录：在 sub2api 中通过 Copilot 账户让 Claude Code (CC) 使用 GPT/Gemini 等非 Claude 模型的完整方案，包括问题排查过程、代码实现原理和计费机制说明。

---

## 目录

- [1. 需求背景](#1-需求背景)
- [2. 可用模型列表](#2-可用模型列表)
- [3. 正确配置方法](#3-正确配置方法)
- [4. 踩坑记录（排查过程）](#4-踩坑记录排查过程)
- [5. 代码实现原理](#5-代码实现原理)
- [6. Copilot 计费机制](#6-copilot-计费机制)
- [7. 混用 Claude + GPT 的注意事项](#7-混用-claude--gpt-的注意事项)

---

## 1. 需求背景

Copilot 账户下有大量非 Claude 模型（GPT-5.4、GPT-4o、Gemini 等），希望 CC 用户通过修改环境变量配置就能直接使用这些模型，无需修改客户端代码。

**目标**：同一个 API Key，同一个 `ANTHROPIC_BASE_URL`，只改 `ANTHROPIC_MODEL` 等环境变量，即可切换到任意 Copilot 支持的模型。

---

## 2. 可用模型列表

以下是 Copilot 支持的典型模型（通过 `/copilot/v1/models` 实时获取）：

### Chat Completions 端点（`/chat/completions`）
直接可用，无需任何特殊处理：

| 模型 ID | 说明 |
|---------|------|
| `gpt-4o` | 主力模型，速度快 |
| `gpt-4o-mini` | 轻量任务 |
| `gpt-4o-2024-11-20` | 日期版本 |
| `gpt-4.1` | 最新 GPT-4 系列 |
| `gpt-4.1-2025-04-14` | 日期版本 |
| `gpt-4.1-mini` | 轻量 |
| `gpt-41-copilot` | Copilot 专属版 |
| `gemini-3.1-pro-preview` | Gemini 系列 |
| `gemini-3-flash-preview` | Gemini 快速版 |
| `grok-code-fast-1` | Grok 代码版 |

### Responses 端点（`/responses`）
需要 sub2api v0.1.128+ 的端点自动路由支持：

| 模型 ID | 说明 |
|---------|------|
| `gpt-5.4` | GPT-5 最强版 |
| `gpt-5.4-mini` | GPT-5 轻量版 |
| `gpt-5.3-codex` | Codex 代码专用 |
| `gpt-5.1-codex-mini` | Codex 轻量版 |
| `gpt-5.2` | GPT-5 系列 |

> **注意**：哪些模型走哪个端点由 Copilot API 的 `supported_endpoints` 字段决定，sub2api 自动识别，无需手动配置。

---

## 3. 正确配置方法

### 基础配置（`~/.claude/settings.json`）

```json
{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "你的sub2api-api-key",
    "ANTHROPIC_BASE_URL": "http://你的sub2api地址/copilot",
    "ANTHROPIC_MODEL": "gpt-5.4",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "gpt-5.4",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "gpt-5.4",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "gpt-5.4-mini",
    "ANTHROPIC_REASONING_MODEL": "gpt-5.4"
  },
  "model": "claude-sonnet-4-6"
}
```

### 混用 Claude + GPT 配置

```json
{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "你的sub2api-api-key",
    "ANTHROPIC_BASE_URL": "http://你的sub2api地址/copilot",
    "ANTHROPIC_MODEL": "gpt-5.4",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "gpt-5.4",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "claude-opus-4-6",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "gpt-5.4-mini",
    "ANTHROPIC_REASONING_MODEL": "claude-opus-4-6"
  },
  "model": "claude-sonnet-4-6"
}
```

### 关键点

| 配置项 | 正确值 | 错误示例 | 原因 |
|--------|--------|----------|------|
| `ANTHROPIC_BASE_URL` | `http://host/copilot` | `http://host/copilot/v1` | CC 会自动追加 `/v1`，加了就变成 `/copilot/v1/v1/messages` → 404 |
| `ANTHROPIC_BASE_URL` | `http://host/copilot` | `http://host/v1` | `/v1` 是通用网关，不强制走 Copilot 平台 |
| `model` 字段 | `claude-sonnet-4-6` | `opusplan` | CC 启动时校验模型列表，`opusplan` 不在列表里 → 报错 |
| `ANTHROPIC_DEFAULT_HAIKU_MODEL` | `gpt-5.4-mini` | `5.4-mini` | 必须带完整前缀 `gpt-` |

---

## 4. 踩坑记录（排查过程）

### 坑 1：`ANTHROPIC_BASE_URL` 路径翻倍

**现象**：后端日志显示 `POST /copilot/v1/v1/messages → 404`

**原因**：CC SDK 会在 `ANTHROPIC_BASE_URL` 后自动追加 `/v1/messages`。配置了 `http://host/copilot/v1`，实际请求变成 `http://host/copilot/v1/v1/messages`。

**修复**：`ANTHROPIC_BASE_URL` 去掉末尾的 `/v1`，改为 `http://host/copilot`。

---

### 坑 2：`model: "opusplan"` 导致启动报错

**现象**：CC 启动时报 `There's an issue with the selected model (gpt-5.4). It may not exist or you may not have access to it.`（即便模型名是 `gpt-5.4` 也会显示此错误）

**原因**：`settings.json` 里的 `"model": "opusplan"` 是 CC 的 Plan Mode 别名，CC 启动时会验证该 ID 在 `/v1/models` 返回列表里是否存在。`/copilot/v1/models` 返回的是 Copilot 真实模型列表，不含 `opusplan`，验证失败。

**修复**：将 `"model"` 改为标准模型名，如 `"claude-sonnet-4-6"`，或删除该字段。

---

### 坑 3：`gpt-5.4` 报 `unsupported_api_for_model`

**现象**：请求到达后端，Copilot 返回 `model "gpt-5.4" is not accessible via the /chat/completions endpoint`

**原因**：`gpt-5.4` 等 GPT-5 系列模型在 Copilot API 中只支持 `/responses` 端点（OpenAI Responses API），不支持 `/chat/completions`。而 sub2api 的 `ForwardMessages` 原先写死走 `/chat/completions`。

**修复**：实现端点自动路由（见第 5 节）。

---

## 5. 代码实现原理

### 5.1 路由架构

```
CC 发来 POST /copilot/v1/messages（Anthropic 协议）
    ↓
copilot_gateway_handler.go → CopilotGatewayHandler.Messages()
    ↓
copilot_gateway_service.go → ForwardMessages()
    ↓
    ├─ 翻译 Anthropic body → OpenAI Chat Completions 格式
    ├─ 应用账户模型映射（account model_mapping）
    ├─ 规范化模型 ID（claude dash→dot 格式）
    ├─ 查询 getSupportedEndpointsForModel(model)
    │       ↓
    │   ┌── /responses 且无 /chat/completions？
    │   │
    │   ├── YES → forwardMessagesViaResponses()
    │   │           Anthropic → ResponsesRequest
    │   │           POST CopilotAPIBase/responses
    │   │           Responses SSE → Anthropic SSE/JSON
    │   │
    │   └── NO  → 原有路径
    │               POST baseURL/chat/completions
    │               OpenAI SSE → Anthropic SSE/JSON
```

### 5.2 端点自动路由实现

**文件**：`backend/internal/service/copilot_gateway_service.go`

**核心函数**：

```go
// 判断是否需要走 /responses（仅当只支持 responses，不支持 chat/completions 时）
func shouldUseResponsesEndpoint(supportedEndpoints []string) bool {
    hasResponses := false
    hasChatCompletions := false
    for _, ep := range supportedEndpoints {
        switch ep {
        case "/responses":      hasResponses = true
        case "/chat/completions": hasChatCompletions = true
        }
    }
    return hasResponses && !hasChatCompletions
}
```

**缓存策略**：
- 每次请求时查询账户缓存（`modelEndpointsCache`，按 `accountID` 区分）
- 命中缓存直接返回，无需调用 `/models`
- 缓存 TTL：成功 1 小时，失败 2 分钟（快速重试）
- 缓存未命中时调用 `ListModels()` 拉取并解析 `supported_endpoints`

**协议转换链**（利用已有 `apicompat` 包）：

```
Anthropic Request
    → apicompat.AnthropicToResponses()
    → ResponsesRequest（覆盖 model、强制 stream=true）
    → POST /responses
    → 逐行读 SSE
    → apicompat.ResponsesEventToAnthropicEvents()
    → apicompat.ResponsesAnthropicEventToSSE()
    → 推送给 CC
```

### 5.3 模型 ID 规范化

`backend/internal/pkg/copilot/model_normalize.go`：

```go
func NormalizeModelIDForCopilotUpstream(model string) string {
    // Claude 模型：claude-haiku-4-5 → claude-haiku-4.5（dash→dot）
    // 非 Claude 模型（gpt-*, gemini-*）：原样 passthrough
    if m := claudeMajorMinorDash.FindStringSubmatch(model); m != nil {
        return "claude-" + m[1] + "-" + m[2] + "." + m[3]
    }
    return model  // gpt-5.4 → gpt-5.4，不变
}
```

### 5.4 路由入口

`backend/internal/server/routes/gateway.go`：

```
/copilot/v1/messages  → CopilotGateway.Messages  (CC 使用，Anthropic 协议)
/copilot/v1/chat/completions → CopilotGateway.ChatCompletions (OpenAI 兼容)
/copilot/v1/responses → CopilotGateway.Responses (Codex CLI 使用)
/copilot/v1/models    → CopilotGateway.Models    (模型列表，实时拉取+缓存)
/v1/messages          → 通用网关（根据账户 platform 路由，非强制 Copilot）
```

---

## 6. Copilot 计费机制

### 核心机制：`X-Initiator` Header

Copilot **不按用户请求次数计费**，而是由每个 API 请求的 `X-Initiator` header 决定扣哪个 quota：

| `X-Initiator` | 触发条件 | 扣费 bucket |
|---------------|----------|-------------|
| `"user"` | 消息历史中**没有** `assistant`/`tool` 角色（首轮对话） | **Premium Interactions（付费）** |
| `"agent"` | 消息历史中**有** `assistant`/`tool` 角色（多轮/工具调用） | **Standard quota（免费）** |

sub2api 中的实现（`copilotInitiator` 函数）：

```go
func copilotInitiator(openAIBody []byte) string {
    for _, m := range req.Messages {
        if m.Role == "assistant" || m.Role == "tool" {
            return "agent"  // 多轮 → 免费标准 quota
        }
    }
    return "user"  // 首轮 → 扣 premium
}
```

### 一次用户操作的实际请求次数

CC 的一次"用户发消息"在后端是**多个串行请求**：

```
用户输入 "hello"
  ├─ 请求 A: 生成 session title  (system prompt 含 "Generate a concise title")
  │          model=haiku/gpt-5.4-mini, messages=[system, user]
  │          → initiator="user" → 扣 1 次 Premium Interaction
  │
  └─ 请求 B: 主响应
             model=sonnet/gpt-5.4, messages=[system, user]
             → initiator="user" → 扣 1 次 Premium Interaction

（如果有工具调用）
  └─ 请求 C: 工具结果后续
             messages=[system, user, assistant(tool_use), tool_result]
             → initiator="agent" → 扣标准 quota（不计 premium）
```

### 结论：是否只扣一次？

**不是，是两次**（title 生成 + 主响应各一次），但这与 Claude/GPT **混用无关** — 纯 Claude 配置也一样扣两次。这是 CC 客户端行为，不是 sub2api 的问题。

### 各计划的 Premium Interactions 配额

| 计划 | Premium Interactions | 是否无限 |
|------|---------------------|---------|
| Copilot Individual Pro+ | 无限制 | ✅ |
| Copilot Individual Pro | 有限额（通常 300/月） | ❌ |
| Copilot Business | 有限额 | ❌ |
| Copilot Enterprise | 有限额 | ❌ |

**建议**：使用 Pro+ 账户，premium interactions 无限，混用 Claude/GPT 无成本顾虑。

---

## 7. 混用 Claude + GPT 的注意事项

### 推荐混用策略

```json
{
  "ANTHROPIC_DEFAULT_HAIKU_MODEL": "gpt-5.4-mini",   // 轻量任务：快速、省 quota
  "ANTHROPIC_DEFAULT_SONNET_MODEL": "gpt-5.4",        // 主力：最强 GPT
  "ANTHROPIC_DEFAULT_OPUS_MODEL": "claude-opus-4-6",  // 复杂推理：Claude 更擅长
  "ANTHROPIC_REASONING_MODEL": "claude-opus-4-6"      // 深度思考：Claude
}
```

### 混用注意事项

1. **工具调用格式兼容性**：CC 使用 Anthropic 工具格式，sub2api 会自动翻译为 OpenAI 工具格式。GPT 模型对工具调用的支持与 Claude 行为略有差异，如遇异常可切回 Claude。

2. **上下文长度**：不同模型的上下文窗口不同。`gpt-5.4` 支持较长上下文，但 sub2api 有 `clampCopilotUpstreamMaxTokens` 保护（默认 8192 max_tokens，可在账户凭据里覆盖）。

3. **计费独立**：每个请求独立计费，混用不会"合并"成一次计费。

4. **模型列表动态更新**：`/copilot/v1/models` 每小时从 Copilot 实时拉取，Copilot 新增模型后无需升级 sub2api，直接改配置即可使用。

---

## 附：关键文件索引

| 文件 | 作用 |
|------|------|
| `backend/internal/handler/copilot_gateway_handler.go` | HTTP 入口，路由到 service |
| `backend/internal/service/copilot_gateway_service.go` | 核心转发逻辑，端点路由，协议翻译 |
| `backend/internal/pkg/copilot/model_normalize.go` | 模型 ID 规范化（Claude dash→dot，非 Claude passthrough）|
| `backend/internal/pkg/copilot/types.go` | Copilot 类型定义，包含 `SupportedEndpoints` |
| `backend/internal/pkg/apicompat/anthropic_to_responses.go` | Anthropic → Responses API 翻译 |
| `backend/internal/pkg/apicompat/responses_to_anthropic.go` | Responses API → Anthropic 翻译（含流式）|
| `backend/internal/server/routes/gateway.go` | 路由注册，`/copilot/v1/*` 路径定义 |
