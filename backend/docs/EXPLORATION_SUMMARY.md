# Sub2API Copilot Gateway 代码探索总结

## 📋 核心文件清单

| 文件 | 行数 | 职责 |
|------|------|------|
| `copilot_gateway_handler.go` | 1282 | 请求入口、认证、计费检查、账号选择、故障转移 |
| `copilot_gateway_service.go` | 1900+ | Token 提取、请求转发、流式/非流式响应处理 |
| `gateway_service.go` | 8566 | 计费逻辑、使用量记录、成本计算 |
| `anomaly_service.go` | 238 | 异常检测、zero_token 识别、慢请求检测 |

## 🔄 请求完整流程

```
客户端请求
    ↓
认证 (middleware)
    ↓ 
权限检查 (Billing)
    ↓
并发限制 (Concurrency)
    ↓
账号选择 (SelectAccount + Failover)
    ↓
请求转发 (ForwardChatCompletions)
    ├─ 获取 Copilot Token
    ├─ 构建 HTTP 请求
    └─ 发送到 Copilot API
        ├─ 流式响应: parseStreamUsage 逐行累积
        └─ 非流式响应: parseNonStreamUsage 一次提取
    ↓
响应返回给客户端
    ↓
异步后台处理 (Goroutine)
    ├─ RecordUsage: 计费 & 存储使用量日志
    └─ WriteAnomalyLog: 检测异常 & 存储异常日志
```

## ⚡ Token 处理核心

### Token 来源路径

```
Copilot API 响应
    ↓
parseStreamUsage() 或 parseNonStreamUsage()
    ↓
CopilotUsage { PromptTokens, CompletionTokens }
    ↓
CopilotForwardResult.Usage
    ↓
映射到 ForwardResult.Usage { InputTokens, OutputTokens }
    ↓
RecordUsage 计费
    ↓
UsageLog 记录存储
```

### 为什么 Token 会显示为 0？

1. **上游 Copilot API 没有返回 usage** (最常见)
   - 响应体格式不符合预期
   - 某些模型不返回 usage
   - Copilot API 返回了错误

2. **流式响应没有收集到数据**
   - SSE 流中没有 usage 数据行
   - 客户端提前断开连接

3. **强制缓存计费** (ForceCacheBilling)
   - InputTokens 被转为 CacheReadInputTokens
   - InputTokens 被清零

4. **解析失败**
   - parseNonStreamUsage() 返回 `&CopilotUsage{}` (全 0)

## 🎯 异常检测触发条件

```go
detectAnomalies() 检查以下场景:

1. Zero Token: inputTokens == 0 AND outputTokens == 0
   ├─ 默认启用 (DetectZeroToken: true)
   └─ 写入 request_logs 表

2. 超时: durationMs > TimeoutThresholdMs (默认 60s)
   └─ 写入 request_logs 表

3. 慢请求: durationMs > SlowRequestThresholdMs (默认 20s)
   └─ 写入 request_logs 表

4. 错误: statusCode >= 500
   └─ 写入 request_logs 表

异常日志可选包含: 原始请求体、上游请求体、上游响应体
```

## 🐌 响应慢的排查清单

### 1. 分解后的延迟指标 (在 OpsSpan 中)

- `auth_latency_ms`: 认证鉴权阶段
  - APIKey 验证
  - Billing 检查

- `routing_latency_ms`: 路由选择阶段
  - SelectAccountForModelWithExclusions SQL
  - 并发槽位等待

- `upstream_latency_ms`: 上游请求阶段
  - Token 获取
  - HTTP POST 发送
  - 首字节响应时间

- `response_latency_ms`: 响应处理阶段
  - SSE 流传输或响应体读取
  - JSON 解析

### 2. 瓶颈定位

```
如果 upstream_latency_ms 占大头
    └─ 问题在 Copilot API (不是 sub2api 的责任)

如果 auth_latency_ms 占大头
    └─ 检查:
       - APIKey 表查询 (是否有索引?)
       - Billing 检查 (缓存是否失效?)

如果 routing_latency_ms 占大头
    └─ 检查:
       - 账号池是否足够
       - 是否频繁做账号切换 (failover)
       - 是否有并发限制 (槽位耗尽)

如果 response_latency_ms 占大头
    └─ 检查:
       - 客户端网络是否慢
       - 大响应体是否需要优化
```

### 3. 异步后台不影响客户端响应

```go
// RecordUsage 和 WriteAnomalyLog 在这里执行
go func() {
    recordCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    h.gatewayService.RecordUsage(recordCtx, ...)
    h.anomalyService.WriteAnomalyLog(recordCtx, ...)
}()
// HTTP 响应已经返回给客户端
```

**因此不会直接影响客户端感知的响应时间**

## 🔍 调试要点

### 1. 追踪 Token 为 0

```bash
# 启用异常日志记录
settings.DetectZeroToken = true
settings.SaveRawData = true

# 查询 request_logs 表
SELECT * FROM request_logs 
WHERE anomaly_types LIKE '%zero_token%'
ORDER BY created_at DESC
LIMIT 10;

# 查看原始响应体
SELECT request_body, upstream_request_body, upstream_response_body
FROM request_logs
WHERE request_id = '{request_id}';
```

### 2. 查看 OpsSpan 日志

```bash
# 每个请求会记录性能 span
SELECT ops_spans 
FROM usage_logs 
WHERE request_id = '{request_id}';

# OpsSpans 包含:
# [
#   {"name": "routing.select", "duration_ms": 50, "status": "ok"},
#   {"name": "token.fetch", "duration_ms": 100, "status": "ok"},
#   {"name": "upstream.post", "duration_ms": 2500, "status": "ok"}
# ]
```

### 3. 检查故障转移

```bash
# 查看是否频繁做账号切换
grep "upstream_failover_switching\|failover_exhausted" logs/
# 如果频繁出现，说明账号池问题
```

## 📊 关键数据结构

### CopilotUsage (上游响应)

```go
{
    "prompt_tokens": 100,        // ← InputTokens 来源
    "completion_tokens": 50,     // ← OutputTokens 来源
    "total_tokens": 150
}
```

### UsageLog (数据库记录)

```go
{
    "input_tokens": 100,              // 计费用
    "output_tokens": 50,              // 计费用
    "cache_creation_tokens": 0,       // 缓存创建
    "cache_read_tokens": 0,           // 缓存读取
    "cache_creation_5m_tokens": 0,
    "cache_creation_1h_tokens": 0,
    "input_cost": 0.5,
    "output_cost": 0.25,
    "total_cost": 0.75,
    "actual_cost": 0.75,              // 扣费金额
    "auth_latency_ms": 10,
    "routing_latency_ms": 20,
    "upstream_latency_ms": 2500,
    "response_latency_ms": 100
}
```

## 🚀 配置优化建议

### 1. 故障转移

```go
// 默认 3 次切换，可调整
MaxAccountSwitches: 3  // ← 账号池不足时增加
```

### 2. 异常检测阈值

```go
// 根据实际情况调整
SlowRequestThresholdMs: 20000,    // 20 秒 → 可调
TimeoutThresholdMs:     60000,    // 60 秒 → 可调
```

### 3. 缓存策略

```go
// Model 列表缓存
copilotModelCacheTTL: 1 * time.Hour        // 新鲜数据
copilotModelCacheFailedTTL: 2 * time.Minute // 失败重试
```

## 📝 代码阅读顺序建议

1. **入门**: `copilot_gateway_handler.go` - ChatCompletions 方法
2. **核心**: `copilot_gateway_service.go` - parseStreamUsage & parseNonStreamUsage
3. **计费**: `gateway_service.go` - RecordUsage 方法
4. **异常**: `anomaly_service.go` - WriteAnomalyLog & detectAnomalies

## 📦 生成的文档

已为您生成了两份详细文档:

1. `/tmp/sub2api_flow_analysis.md` - 完整流程分析 (8个章节)
   - 请求流程概览
   - 关键文件职责
   - Token 为 0 的原因分析
   - 响应慢的排查指南
   - 数据流总结
   - 关键配置项
   - 调试提示
   - 代码关键片段

2. `/tmp/code_snippets.md` - 代码参考手册
   - ChatCompletions 完整流程 (带注释)
   - Token 提取流程
   - 计费核心逻辑
   - 异常检测逻辑
   - 类型定义
   - 错误处理与故障转移
   - 性能监测点

