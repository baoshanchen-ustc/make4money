# Sub2API Copilot Gateway 完整请求流程分析

## 1. 请求流程概览

```
客户端请求
    ↓
[CopilotGatewayHandler]
    ├─ 认证 (APIKey 验证)
    ├─ 权限检查 (Billing Eligibility)
    ├─ 并发限制 (Concurrency Check)
    ├─ 账号选择 (Account Selection with Failover)
    ├─ 请求体大小检查
    └─ [CopilotGatewayService] 转发请求
        ├─ Token 获取 (GetAccessToken)
        ├─ 请求重写 (Model Mapping, Max Tokens Clamping)
        ├─ 上游请求发送
        └─ 流式/非流式 响应处理
            ├─ parseStreamUsage 或 parseNonStreamUsage
            └─ 返回 CopilotForwardResult (包含 Usage)
    ↓
异步后台处理 (Goroutine)
    ├─ RecordUsage (计费)
    └─ WriteAnomalyLog (异常检测)
    ↓
响应给客户端
```

## 2. 关键文件与职责

### 2.1 CopilotGatewayHandler (`copilot_gateway_handler.go`)

**三个主要入口点：**

1. **ChatCompletions** (L117-425)
   - OpenAI 兼容 /v1/chat/completions 端点
   - 处理请求体、模型验证、认证、账号选择、转发
   - 异步记录 usage 和异常日志

2. **Messages** (L914-1263)
   - Anthropic 兼容 /v1/messages 端点
   - 支持 Claude Code probe 拦截（max_tokens=1 + haiku）
   - 请求体截断处理（大 context 自动裁剪）

3. **Responses** (L565-861)
   - OpenAI Responses API（Codex CLI 使用）
   - 返回结构中包含 reasoning_effort

4. **Models** (L439-503)
   - 模型列表端点，支持分组级别缓存
   - 降级策略：新鲜缓存 → 上游成功 → 陈旧缓存 → 静态默认值

### 2.2 CopilotGatewayService (`copilot_gateway_service.go`)

**Token 提取流程：**

#### 流式响应 - parseStreamUsage (L546-577)
```go
// SSE 数据行格式
data: {"usage": {"prompt_tokens": N, "completion_tokens": M}}
// 或 Responses API 格式
data: {"type": "response.completed", "response": {"usage": {"input_tokens": N, "output_tokens": M}}}

// 处理逻辑
parseStreamUsage() 循环扫描每一行 SSE 数据
    ├─ 如果包含 "usage" 字段 → 提取 prompt_tokens, completion_tokens
    ├─ 如果是 "response.completed" → 映射 input_tokens → prompt_tokens
    └─ 累积到 CopilotUsage 对象中
```

#### 非流式响应 - parseNonStreamUsage (L582-609)
```go
parseNonStreamUsage(body []byte) *CopilotUsage
    ├─ 尝试 Chat Completions 格式: {"usage": {...}}
    │   └─ 提取 prompt_tokens, completion_tokens
    ├─ 尝试 Responses API 格式: {"usage": {"input_tokens": N, "output_tokens": M}}
    │   └─ 映射到 prompt_tokens, completion_tokens
    └─ 返回 &CopilotUsage{} 如果都失败（**这是 token 为 0 的原因之一**）
```

### 2.3 GatewayService - RecordUsage (L7514-7702)

**Token 计费流程：**

```go
RecordUsage(ctx, input *RecordUsageInput) (requestID, usageLogID, err)
    ↓
1. 强制缓存计费检查 (ForceCacheBilling)
   ├─ 如果启用 → input_tokens 转为 cache_read_input_tokens
   └─ 清零 input_tokens

2. Cache TTL 覆盖
   └─ 应用账号的缓存 TTL 设置

3. 获取费率倍数 (RateMultiplier)
   ├─ 用户专属 > 分组默认 > 系统默认
   └─ 默认 1.0

4. 选择计费方式
   ├─ MediaType 为 image/video → Sora 计费
   ├─ MediaType 为 prompt → 零成本
   └─ ImageCount > 0 → 图片生成计费
   └─ 其他 → Token 计费

5. Token 计费核心逻辑
   tokens := UsageTokens{
       InputTokens:           result.Usage.InputTokens,       ← **从上游解析**
       OutputTokens:          result.Usage.OutputTokens,      ← **从上游解析**
       CacheCreationTokens:   result.Usage.CacheCreationInputTokens,
       CacheReadTokens:       result.Usage.CacheReadInputTokens,
       CacheCreation5mTokens: result.Usage.CacheCreation5mTokens,
       CacheCreation1hTokens: result.Usage.CacheCreation1hTokens,
   }
   cost = billingService.CalculateCost(model, tokens, multiplier)

6. 创建 UsageLog 记录
   usageLog := &UsageLog{
       InputTokens:           result.Usage.InputTokens,
       OutputTokens:          result.Usage.OutputTokens,
       // ... 其他字段 ...
       CreatedAt:             time.Now(),
   }

7. 应用使用量计费 (applyUsageBilling)
   └─ 从用户余额/订阅配额中扣费

8. 写入使用日志
   writeUsageLogBestEffort(ctx, s.usageLogRepo, usageLog, "service.gateway")
```

### 2.4 AnomalyService - WriteAnomalyLog (L200-238)

**异常检测流程：**

```go
WriteAnomalyLog(ctx, inputTokens, outputTokens, durationMs, statusCode, input)
    ↓
1. 脱离请求生命周期
   bgCtx := context.WithTimeout(context.WithoutCancel(ctx), 30s)

2. 获取异常检测阈值
   settings = GetSettings(bgCtx)  // 缓存 30 秒
   
3. 检测异常类型
   anomalies = detectAnomalies(inputTokens, outputTokens, durationMs, statusCode, settings)
   
   函数逻辑 (L176-194):
   ├─ 如果 inputTokens == 0 AND outputTokens == 0 AND DetectZeroToken == true
   │  └─ 添加 "zero_token" 异常 ⚠️
   ├─ 如果 durationMs > TimeoutThresholdMs (默认 60s)
   │  └─ 添加 "timeout" 异常
   ├─ 否则如果 durationMs > SlowRequestThresholdMs (默认 20s)
   │  └─ 添加 "slow_request" 异常
   └─ 如果 statusCode >= 500
      └─ 添加 "error" 异常

4. 如果检测到异常
   └─ 保存到 request_logs 表 (如果 SaveRawData == true 则包含原始请求体)
```

## 3. Token 显示为 0 的可能原因

### 3.1 上游 Copilot API 没有返回 usage

```
问题: parseNonStreamUsage() 返回 &CopilotUsage{} (所有字段为 0)
原因:
  ├─ 响应体不符合预期格式
  ├─ Copilot API 返回了错误响应（但状态码是 200）
  ├─ 响应体为空或畸形 JSON
  └─ 某些模型或请求类型不包含 usage 字段
```

### 3.2 streamUsage 累积不成功（流式请求）

```
问题: 流式请求没有收集到任何 usage 数据
原因:
  ├─ SSE 流中没有包含 usage 数据的行
  ├─ parseStreamUsage() 无法解析数据行格式
  ├─ 客户端提前断开连接（streamDone == false）
  └─ 底层 scanner 错误
```

### 3.3 模型不支持 usage 返回

```
某些特殊模型可能不返回 usage 信息
```

## 4. 响应慢的可能原因

### 4.1 性能阶段分解 (OpsLatency)

Handler 中追踪四个关键阶段：

```go
// 在 recorderCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 中捕获的指标

OpsAuthLatencyMsKey      // 认证鉴权阶段
OpsRoutingLatencyMsKey   // 路由选择阶段（账号选择 + 并发槽位等待）
OpsUpstreamLatencyMsKey  // 上游请求阶段（发出请求→收到首字节）
OpsResponseLatencyMsKey  // 响应处理阶段（流式传输或读取响应体）
```

### 4.2 可能的瓶颈

1. **认证阶段耗时高**
   - APIKey 查询慢
   - Billing 检查慢
   
2. **路由选择慢**
   - 账号池为空或健康检查失败
   - 选择账号的 SQL 查询慢
   - 并发槽位竞争，等待释放

3. **上游请求慢**
   - Copilot API 响应慢（最常见）
   - Token 获取慢
   - 网络延迟

4. **响应处理慢**
   - 流式 SSE 传输慢（客户端接收慢）
   - 大响应体读取慢
   - JSON 解析慢

### 4.3 异步后台处理额外耗时

```go
go func() {
    recordCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    // RecordUsage 可能执行数据库操作
    requestID, usageLogID, err := h.gatewayService.RecordUsage(recordCtx, ...)
    
    // 异常日志写入也在此（第二个 goroutine 调用）
    h.anomalyService.WriteAnomalyLog(recordCtx, ...)
}()
```

**注意**: 这两个操作都在 HTTP 响应返回后执行，所以不应该直接影响客户端感知的响应时间。
但如果数据库超载，可能会看到错误日志。

## 5. 数据流总结

```
┌─────────────────────────────────────────────────────────────────┐
│  客户端请求                                                       │
│  POST /copilot/v1/chat/completions                               │
│  Content-Type: application/json                                  │
│  {"model": "gpt-4o", "messages": [...], "stream": false}        │
└────────────────────────────┬────────────────────────────────────┘
                             ↓
┌─────────────────────────────────────────────────────────────────┐
│  CopilotGatewayHandler.ChatCompletions                           │
│  1. 验证 APIKey (middleware)                                     │
│  2. 检查 Billing (billingCacheService)                          │
│  3. 获取并发槽位 (concurrencyHelper)                            │
│  4. 选择账号 (gatewayService.SelectAccountForModelWithExclusions)│
│     ├─ 失败时支持账号切换 (maxAccountSwitches = 3)              │
│     └─ 支持故障转移 (failover)                                  │
│  5. 验证请求体大小                                              │
└────────────────────────────┬────────────────────────────────────┘
                             ↓
┌─────────────────────────────────────────────────────────────────┐
│  CopilotGatewayService.ForwardChatCompletions                   │
│  1. 获取 Copilot 访问令牌 (tokenProvider.GetAccessToken)        │
│  2. 重写请求模型 (account.GetMappedModel)                       │
│  3. 构建 HTTP 请求，添加认证头                                  │
│  4. 发送 POST 到 Copilot API                                    │
│     └─ 5 分钟超时                                               │
│  5. 处理响应                                                    │
│     ├─ 流式 → handleStreamingResponse (SSE)                     │
│     │   └─ parseStreamUsage 累积 usage                          │
│     └─ 非流式 → handleNonStreamResponse                         │
│         └─ parseNonStreamUsage 提取 usage                       │
│  6. 返回 CopilotForwardResult(                                 │
│         StatusCode, Model, Usage,                               │
│         Duration, FirstTokenMs                                  │
│     )                                                           │
└────────────────────────────┬────────────────────────────────────┘
                             ↓
┌─────────────────────────────────────────────────────────────────┐
│  响应发送给客户端                                                 │
│  HTTP 200 OK                                                     │
│  Content-Type: application/json                                  │
│  {"choices": [...], "usage": {"prompt_tokens": N, ...}}         │
└────────────────────────────┬────────────────────────────────────┘
                             ↓
┌─────────────────────────────────────────────────────────────────┐
│  异步后台处理 (Goroutine，HTTP 响应后)                           │
│  1. GatewayService.RecordUsage                                   │
│     ├─ 应用费率倍数                                              │
│     ├─ 选择计费方式                                              │
│     ├─ 计算成本 (billingService.CalculateCost)                  │
│     ├─ 创建 UsageLog 记录                                        │
│     ├─ 应用计费 (applyUsageBilling)                             │
│     │   ├─ 扣除用户余额                                          │
│     │   └─ 更新订阅配额                                          │
│     └─ 写入数据库                                               │
│                                                                 │
│  2. AnomalyService.WriteAnomalyLog                              │
│     ├─ 读取异常检测配置 (30秒缓存)                              │
│     ├─ detectAnomalies 检查:                                    │
│     │   ├─ inputTokens == 0 && outputTokens == 0 ⚠️            │
│     │   ├─ durationMs > TimeoutThresholdMs                     │
│     │   ├─ durationMs > SlowRequestThresholdMs                 │
│     │   └─ statusCode >= 500                                   │
│     └─ 写入 request_logs 表 (如果检测到异常)                   │
│         └─ 可选保存原始请求/响应体                              │
└─────────────────────────────────────────────────────────────────┘
```

## 6. 关键配置项

### 6.1 GatewayConfig

```go
type GatewayConfig struct {
    CopilotDefaultMaxBodyKB  int  // 默认请求体大小限制 (KB)
    MaxAccountSwitches       int  // 最大故障转移次数 (默认 3)
}
```

### 6.2 AnomalySettings (默认值)

```go
{
    SlowRequestThresholdMs: 20000,    // 20 秒
    TimeoutThresholdMs:     60000,    // 60 秒
    DetectZeroToken:        true,     // 启用 zero_token 检测
    SaveRawData:            true,     // 保存原始请求体
}
```

### 6.3 Concurrency Config

```go
type ConcurrencyConfig struct {
    PingInterval  int  // SSE 心跳间隔 (秒)
}
```

## 7. 调试提示

### 7.1 追踪 Token 为 0 的问题

1. **检查上游响应**
   ```go
   // 在 copilot_gateway_service.go handleNonStreamingResponse (L329)
   // 设置日志点查看返回的 body 内容
   slog.Warn("copilot response body", "body", string(body))
   ```

2. **启用异常日志记录**
   ```go
   // 在 anomaly_service.go GetSettings
   settings.DetectZeroToken = true
   settings.SaveRawData = true
   ```
   此时会在 request_logs 表中记录 zero_token 异常，包含原始请求/响应体。

3. **查看 OpsSpan 记录**
   - 检查各阶段耗时
   - 查看转发结果中 Usage 字段

### 7.2 排查响应慢

1. **查看分解后的延迟指标**
   ```
   auth_latency_ms      - 认证鉴权
   routing_latency_ms   - 账号选择
   upstream_latency_ms  - 上游请求
   response_latency_ms  - 响应处理
   ```

2. **检查上游是否慢**
   - upstream_latency_ms 是否占大部分
   - 如果是，问题在 Copilot API 而非 sub2api

3. **检查并发限制**
   - 是否有 "Too many concurrent requests" 错误
   - 查看用户当前并发数

4. **检查数据库慢查询**
   - RecordUsage 中的数据库操作可能超时（10秒）
   - 检查 WriteAnomalyLog 是否影响数据库

## 8. 代码关键片段

### 8.1 Token 映射

```go
// 在 copilot_gateway_handler.go ChatCompletions (L362-364)
fwdResult := &service.ForwardResult{
    Usage: service.ClaudeUsage{
        InputTokens:  capturedResult.Usage.PromptTokens,      // 来自 Copilot 的 prompt_tokens
        OutputTokens: capturedResult.Usage.CompletionTokens,  // 来自 Copilot 的 completion_tokens
    },
}
```

### 8.2 异常检测边界

```go
// 在 anomaly_service.go detectAnomalies (L176-194)
// 只有当 BOTH inputTokens AND outputTokens 都为 0 时才算 zero_token 异常
if settings.DetectZeroToken && inputTokens == 0 && outputTokens == 0 {
    types = append(types, AnomalyZeroToken)
}
```

### 8.3 响应处理

```go
// 在 copilot_gateway_service.go 
// 非流式 (L582-609)：直接返回整个响应体中的 usage
// 流式 (L546-577)：从每个 SSE 数据行中累积 usage
// 两种情况都可能返回 &CopilotUsage{} (全 0)
```

