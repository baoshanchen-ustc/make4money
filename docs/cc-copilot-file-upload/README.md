# Cherry Studio / OpenAI 客户端 PDF 文件上传支持

> 本文档记录：通过 sub2api Copilot 账户，在 Cherry Studio（及其他 OpenAI 兼容客户端）上传 PDF 附件并让模型真正读取文件内容的完整实现过程，包括问题排查、根因分析、代码实现和计费影响说明。

---

## 目录

- [1. 需求背景](#1-需求背景)
- [2. 问题现象](#2-问题现象)
- [3. 根因分析（三层）](#3-根因分析三层)
- [4. 解决方案](#4-解决方案)
- [5. 代码实现原理](#5-代码实现原理)
- [6. 计费影响分析](#6-计费影响分析)
- [7. 排查工具：调试日志](#7-排查工具调试日志)
- [8. 关键文件索引](#8-关键文件索引)

---

## 1. 需求背景

用户使用 Cherry Studio 配置 sub2api 的 OpenAI 兼容接口（`/copilot/v1/chat/completions`），选择 `claude-opus-4.6` 模型，上传 PDF 附件后提问。模型回复"无法直接读取您上传的PDF文件内容，因为当前接口不支持处理二进制文件附件"。

**期望行为**：模型能够读取 PDF 内容并基于文档内容回答问题（Copilot 网页端本身是支持的）。

---

## 2. 问题现象

```
用户：这个文档讲的是什么内容  [附: Claude Mythos Preview System Card.pdf]
模型：我无法直接读取您上传的PDF文件内容，因为当前接口不支持处理二进制文件附件。
      不过，根据文件名 "Claude Mythos Preview System Card.pdf"，我可以推测……
```

模型只能猜测文件名含义，无法读取真实内容。

---

## 3. 根因分析（三层）

### 根因 #1：file parts 被降级为文本

**位置**：`ForwardChatCompletions` 早期版本

Cherry Studio 发送 OpenAI 格式请求时，PDF 附件以 `type:"file"` content part 形式携带（含 base64 编码的完整 PDF 数据）：

```json
{
  "messages": [{
    "role": "user",
    "content": [
      {"type": "text", "text": "这个文档讲的是什么内容"},
      {"type": "file", "file": {"filename": "xxx.pdf", "file_data": "data:application/pdf;base64,..."}}
    ]
  }]
}
```

`ForwardChatCompletions` 内部调用 `StripUnsupportedContentPartsFromOpenAIBody` 检测到 `type:"file"` 后，将其转成文本占位符（`convertFilePartsToText`），再发往 Copilot `/chat/completions`。Copilot 的 `/chat/completions` 不接受文件类型，所以 sub2api 主动降级，导致模型收不到文件内容。

**已知情况**：`has_file: true` 能被正确检测到，问题在于之后没有走文件支持的端点。

---

### 根因 #2：模型端点路由判断错误（token 失败导致缓存空）

**位置**：`getSupportedEndpointsForModel`

当检测到有文件时，代码会查询 Copilot `/models` 接口获取该模型支持的端点列表（`supported_endpoints`），然后决定路由：

```
supported_endpoints 含 /responses  →  forwardChatCompletionsViaResponses（支持文件）
supported_endpoints 为空           →  forwardChatCompletionsDirect（降级为文本）
```

问题：`getSupportedEndpointsForModel` 内部需要先获取 Copilot token，再请求 `/models`。如果此时 token 刚好过期，token exchange 会失败（`unexpected EOF`），导致：

1. `/models` 拉取失败
2. `supported_endpoints = []`（空）
3. `modelSupportsResponses([]) = false`
4. 回退到文本降级路径

**日志证据**：
```json
{
  "supported_endpoints": [],
  "ep_err": "copilot token exchange: request failed: unexpected EOF",
  "model_supports_responses": false
}
```

**修复**：token 失败时，对已知 Claude 4.x 模型使用静态端点后备知识（`staticSupportedEndpoints`），不因网络抖动降级。

---

### 根因 #3：`claude-opus-4.6` 不支持 `/responses`，而是 `/v1/messages`

**位置**：路由决策逻辑

即使 token 正常、`/models` 成功返回，`claude-opus-4.6` 的实际 `supported_endpoints` 是：

```json
["/v1/messages", "/chat/completions"]
```

**没有 `/responses`**。之前的代码只检查 `/responses`，不检查 `/v1/messages`，所以即使拿到了正确的端点列表，依然会落回文本降级。

**关键区别**：
- GPT-5 系列模型：`supported_endpoints = ["/responses"]`（无 `/chat/completions`）
- Claude 4.x 模型：`supported_endpoints = ["/v1/messages", "/chat/completions"]`（无 `/responses`）

`/v1/messages` 是 Anthropic Messages API 原生端点，Copilot 对 Claude 模型直接暴露该端点，支持 `type:"document"` block（即 PDF 文件的原生格式）。

**日志证据**：
```json
{
  "supported_endpoints": ["/v1/messages", "/chat/completions"],
  "model_supports_responses": false,
  "model_supports_messages": false   ← 修复前没有这个检查
}
```

---

## 4. 解决方案

### 修复 #1：token 失败时使用静态端点后备

新增 `staticSupportedEndpoints(modelID string)` 函数：

```go
func staticSupportedEndpoints(modelID string) []string {
    if strings.HasPrefix(modelID, "claude-sonnet-4") ||
        strings.HasPrefix(modelID, "claude-opus-4") ||
        strings.HasPrefix(modelID, "claude-haiku-4") {
        return []string{"/chat/completions", "/responses"}
    }
    return nil
}
```

当 `/models` 拉取失败时，对已知 Claude 4.x 模型返回静态端点，不因 token 抖动降级。

> **注意**：静态知识中包含 `/responses` 是为了触发路由，但实际上 Claude 4.x 最终走的是 `/v1/messages`（优先级更高），`/responses` 是兜底。

### 修复 #2：新增 `/v1/messages` 路由

新增 `modelSupportsMessages` 函数和 `forwardChatCompletionsViaMessages` 路径：

```
有文件请求
    │
    ├─ supported_endpoints 含 /responses  →  forwardChatCompletionsViaResponses
    │                                          （GPT-5 系列）
    │
    ├─ supported_endpoints 含 /v1/messages →  forwardChatCompletionsViaMessages  ← 新增
    │                                          （Claude 4.x 系列）
    │
    └─ 均不支持或 custom_base_url         →  forwardChatCompletionsDirect
                                              （降级：PDF 转文本描述）
```

### 修复 #3：双格式 model ID 索引

Copilot `/models` 返回 `claude-opus-4-6`（横杠），而规范化后查询用 `claude-opus-4.6`（点号），导致缓存 miss。

`parseModelEndpointsFromModelsResponse` 改为同时索引原始 ID 和规范化 ID：

```go
m[id] = eps  // "claude-opus-4-6"
if normalized := copilot.NormalizeModelIDForCopilotUpstream(id); normalized != id {
    m[normalized] = eps  // "claude-opus-4.6"
}
```

---

## 5. 代码实现原理

### 5.1 完整请求流程（修复后）

```
Cherry Studio 发送 POST /copilot/v1/chat/completions
    body: {model: "claude-opus-4.6", messages: [{content: [{type:"file", file:{file_data:"data:application/pdf;base64,..."}}}]}]

    ↓ ForwardChatCompletions
    ↓ StripUnsupportedContentPartsFromOpenAIBody → has_file=true
    ↓ convertFilePartsToText → chatFallbackBody（备用，仅降级时用）
    ↓ rewriteCopilotUpstreamModel → upstreamModelID="claude-opus-4.6"
    ↓ getSupportedEndpointsForModel → ["/v1/messages", "/chat/completions"]
    ↓ modelSupportsMessages=true → forwardChatCompletionsViaMessages

    ↓ convertOpenAIChatToAnthropicMessages
        type:"file" + file_data "data:application/pdf;base64,XXX"
        →  type:"document" + source:{type:"base64", media_type:"application/pdf", data:"XXX"}
    ↓ POST https://api.githubcopilot.com/v1/messages
        body: Anthropic Messages 格式（含 type:"document" block）

    ↓ Anthropic SSE 响应流
    ↓ handleChatViaMessagesStreamingResponse
        event: content_block_delta → data: {choices:[{delta:{content:"..."}}]}
        event: message_delta      → data: {choices:[{finish_reason:"stop"}]}
        → data: [DONE]

    ↓ 返回 OpenAI Chat Completions SSE 格式给 Cherry Studio
```

### 5.2 核心函数说明

#### `convertOpenAIChatToAnthropicMessages`（新增）

位于 `copilot_anthropic_translation.go`，将 OpenAI Chat Completions 格式（含 file parts）转成 Anthropic Messages 格式。

关键转换：

| OpenAI 格式 | Anthropic 格式 |
|-------------|----------------|
| `{"type":"file","file":{"file_data":"data:application/pdf;base64,XXX"}}` | `{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"XXX"}}` |
| `{"type":"text","text":"..."}` | `{"type":"text","text":"..."}` |
| system message | `AnthropicMessagesRequest.System` 字段 |

#### `forwardChatCompletionsViaMessages`（新增）

位于 `copilot_gateway_service.go`，完整处理链：

1. `mergeConsecutiveSameRoleMessagesInOpenAIBody` — 合并连续同角色消息
2. `rewriteCopilotUpstreamModel` — 模型 ID 规范化
3. `clampCopilotUpstreamMaxTokens` — 限制 max_tokens
4. `convertOpenAIChatToAnthropicMessages` — 格式转换（含文件）
5. 强制 `stream: true` — 避免 Anthropic 非流式接口超时
6. `copilotInitiator(body)` — 计算 X-Initiator header
7. POST `https://api.githubcopilot.com/v1/messages`
8. `handleChatViaMessagesStreamingResponse` / `handleChatViaMessagesNonStreamingResponse` — 翻译响应回 OpenAI 格式

#### `handleChatViaMessagesStreamingResponse`（新增）

读取 Anthropic SSE 事件流，翻译为 OpenAI Chat Completions SSE 格式：

| Anthropic 事件 | 对应 OpenAI SSE chunk |
|----------------|----------------------|
| `event: message_start` | 提取 `input_tokens` 计入 usage |
| `event: content_block_delta` + `text_delta` | `data: {choices:[{delta:{content:"..."}}]}` |
| `event: message_delta` + `stop_reason` | `data: {choices:[{finish_reason:"stop"}]}` + usage |
| 结束 | `data: [DONE]` |

### 5.3 Anthropic `/v1/messages` 端点的 Document Block

Anthropic Messages API 原生支持 PDF：

```json
{
  "type": "document",
  "source": {
    "type": "base64",
    "media_type": "application/pdf",
    "data": "<base64编码的PDF内容>"
  }
}
```

Copilot 对 Claude 模型直接暴露此端点（`/v1/messages`），所以 Claude 可以完整读取 PDF 内容，而不是仅看到文件名。

### 5.4 降级策略

为保证稳定性，每一步都有降级保障：

```
convertOpenAIChatToAnthropicMessages 失败
    → forwardChatCompletionsDirect（文本降级）

POST /v1/messages 返回非 200
    → forwardChatCompletionsDirect（文本降级）

getSupportedEndpointsForModel token 失败
    → staticSupportedEndpoints（静态后备）→ 继续尝试正确路由

supported_endpoints 不含 /v1/messages 且不含 /responses
    → forwardChatCompletionsDirect（文本降级）
```

---

## 6. 计费影响分析

**结论：与修复前完全一致，无额外消耗。**

### X-Initiator header（决定扣哪个 quota）

新路径 `forwardChatCompletionsViaMessages` 调用 `copilotInitiator(body)` 分析 OpenAI 格式的 messages：

| 场景 | X-Initiator | 扣费 bucket |
|------|-------------|-------------|
| 首轮对话（messages 中无 assistant/tool） | `user` | Premium Interactions |
| 多轮/工具调用（有 assistant/tool） | `agent` | Standard quota（免费） |

这与其他所有路径（`/chat/completions`、`/responses`）行为完全相同。

### 请求次数

- **正常路径**：1 次上游请求（POST `/v1/messages`）
- **降级路径**（仅在错误时）：最多 2 次（先试 `/v1/messages`，失败再试 `/chat/completions`）
- **模型端点查询**：首次请求触发 1 次 GET `/models`，之后 1 小时内缓存命中，无额外消耗

### 与修复前的对比

| 场景 | 修复前 | 修复后 |
|------|--------|--------|
| 有 PDF 附件 | 1 次 `/chat/completions`（文本降级，模型无法读文件） | 1 次 `/v1/messages`（原生传递 PDF） |
| 无附件 | 1 次 `/chat/completions` | 1 次 `/chat/completions`（不变） |
| 上游请求次数 | 相同 | 相同 |
| X-Initiator 逻辑 | 相同 | 相同 |

---

## 7. 排查工具：调试日志

修复过程中添加了若干调试日志，可以通过日志追踪路由决策：

### 关键日志条目

```
# 1. 检测到文件
copilot chat: file detection {"has_file": true, "body_len": 28971025}

# 2. 路由决策（修复后的完整版本）
copilot chat: file routing decision {
  "upstream_model_id": "claude-opus-4.6",
  "has_custom_base_url": false,
  "supported_endpoints": ["/v1/messages", "/chat/completions"],
  "ep_err": null,
  "model_supports_responses": false,
  "model_supports_messages": true        ← 走新路径
}

# 3. 上游响应（端点为 v1/messages）
copilot upstream response {
  "account_id": 10,
  "model": "claude-opus-4.6",
  "status": 200,
  "stream": true,
  "latency_ms": 8500
}
```

### 常见问题诊断

| 日志现象 | 原因 | 处理 |
|----------|------|------|
| `"supported_endpoints": [], "ep_err": "unexpected EOF"` | token exchange 抖动 | 静态后备会自动处理，无需干预 |
| `"model_supports_messages": false` + 无文件路由 | 模型不支持 `/v1/messages` | 该模型暂不支持 PDF，降级为文本描述 |
| `"has_file": false` | Cherry Studio 没有传文件 part | 检查客户端是否正确上传文件 |
| upstream 返回 non-200 | Copilot 端错误 | 查看 `body` 字段的错误信息 |

---

## 8. 关键文件索引

| 文件 | 变更内容 |
|------|----------|
| `backend/internal/service/copilot_gateway_service.go` | 新增 `modelSupportsMessages`、`forwardChatCompletionsViaMessages`、`handleChatViaMessagesStreamingResponse`、`handleChatViaMessagesNonStreamingResponse`、`staticSupportedEndpoints`；更新 `ForwardChatCompletions` 路由逻辑；`parseModelEndpointsFromModelsResponse` 双 key 索引 |
| `backend/internal/service/copilot_anthropic_translation.go` | 新增 `AnthropicDocumentBlock`、`convertOpenAIChatToAnthropicMessages`、`openAIContentToAnthropicBlocks`、`parseDataURI` 等转换函数 |
| `backend/internal/service/copilot_gateway_service_test.go` | 新增 `TestStaticSupportedEndpoints`、`TestParseModelEndpointsFromModelsResponse_NormalizedKeyAlias`、`TestForwardChatCompletions_FilePartsClaudeOpusDashID` |

---

## 附：Cherry Studio 配置方式

在 Cherry Studio 中配置 sub2api Copilot 接口：

| 字段 | 值 |
|------|-----|
| 接口类型 | OpenAI |
| API Base URL | `http://your-sub2api-host/copilot/v1` |
| API Key | sub2api 的 API Key |
| 模型 | `claude-opus-4.6`（或其他支持的模型） |

上传 PDF 后提问，模型即可读取文件内容。

**支持的文件类型**：当前实现支持 `application/pdf`（PDF 文档）。其他 MIME 类型（如图片）走独立的 `Copilot-Vision-Request` 路径，不受本次变更影响。

**支持的模型**：所有 `supported_endpoints` 含 `/v1/messages` 的 Copilot 模型（当前主要是 Claude 4.x 系列）。
