# Sub2API Backend 代码探索文档

## 📚 文档导航

本目录包含对 Sub2API Copilot Gateway 后端的深度代码分析文档。

### 1. 📖 [EXPLORATION_SUMMARY.md](./EXPLORATION_SUMMARY.md) - 快速开始 ⭐ **从这里开始**

**适合**: 想快速了解整个系统的人

- 核心文件清单
- 请求完整流程 (流程图)
- Token 处理核心逻辑
- 异常检测触发条件
- 响应慢的排查清单
- 调试要点
- 配置优化建议
- 推荐阅读顺序

**长度**: ~7KB (5分钟阅读)

---

### 2. 🔍 [FLOW_ANALYSIS.md](./FLOW_ANALYSIS.md) - 完整系统分析

**适合**: 想深入理解各个组件的人

包含 8 个详细章节:

1. **请求流程概览** - 从客户端到响应的完整链路
2. **关键文件与职责** - 4个核心文件的职责分解
   - CopilotGatewayHandler (L117-1263)
   - CopilotGatewayService (Token 提取流程)
   - GatewayService.RecordUsage (计费逻辑)
   - AnomalyService.WriteAnomalyLog (异常检测)
3. **Token 显示为 0 的原因** - 3类根本原因分析
4. **响应慢的原因** - 4个可能的瓶颈
5. **数据流总结** - 完整的流程图
6. **关键配置项** - Gateway、Anomaly、Concurrency 配置
7. **调试提示** - 追踪 token 和性能问题的方法
8. **代码关键片段** - 关键部分的代码列表

**长度**: ~17KB (15分钟阅读)

---

### 3. 💻 [CODE_SNIPPETS.md](./CODE_SNIPPETS.md) - 代码参考手册

**适合**: 需要看具体代码实现的人

包含 8 个详细的代码示例（带注释）:

1. **Handler 入口** - ChatCompletions 完整流程 (L117-425)
   - 12 个步骤的详细注释
   - Token 从哪里来

2. **转发服务 - Token 提取**
   - ForwardChatCompletions 主流程 (L112-241)
   - parseStreamUsage 流式 Token 提取 (L546-577)
   - parseNonStreamUsage 非流式 Token 提取 (L582-609)

3. **计费服务** - RecordUsage 完整流程 (L7514-7702)
   - 10 个步骤的计费逻辑

4. **异常检测** - AnomalyService.WriteAnomalyLog (L200-238)
   - 异常检测逻辑

5. **类型定义** - 核心数据结构
   - CopilotUsage
   - CopilotForwardResult
   - RecordUsageInput
   - AnomalySettings

6. **关键常量** - 默认值和阈值

7. **错误处理** - 故障转移条件

8. **性能监测** - OpsSpan 和延迟指标

**长度**: ~25KB (参考手册)

---

## 🎯 快速问题解答

### Q1: Token 为什么显示为 0？

**参考**: EXPLORATION_SUMMARY.md → Token 处理核心 → 为什么 Token 会显示为 0？

**答案**: 4 个可能的原因
- 上游 Copilot API 没有返回 usage (最常见)
- 流式响应没有收集到数据
- 强制缓存计费导致 InputTokens 被清零
- 解析失败返回 `&CopilotUsage{}`

**查询命令**:
```sql
-- 查看 zero_token 异常
SELECT * FROM request_logs 
WHERE anomaly_types LIKE '%zero_token%'
ORDER BY created_at DESC LIMIT 10;

-- 查看原始响应体
SELECT upstream_response_body FROM request_logs
WHERE request_id = '{request_id}';
```

---

### Q2: 响应为什么很慢？

**参考**: FLOW_ANALYSIS.md → 4. 响应慢的原因

**答案**: 4 个性能阶段，查看 OpsSpan 指标:

```sql
SELECT ops_spans FROM usage_logs 
WHERE request_id = '{request_id}';
```

OpsSpans 包含:
- `routing.select`: 账号选择
- `token.fetch`: Copilot Token 获取
- `upstream.post`: 上游请求 (通常最长)
- `translate.req`: 请求转换

**快速排查**:
- 如果 upstream.post 占 90%+ → 问题在 Copilot API
- 如果 routing.select 很长 → 检查账号池是否充足
- 如果经常看到 failover_switching → 账号频繁失败

---

### Q3: 请求流程是怎样的？

**参考**: EXPLORATION_SUMMARY.md → 请求完整流程

**流程图**:
```
请求 → 认证 → 权限检查 → 并发限制 → 账号选择(+故障转移)
  → Token获取 → 构建请求 → 转发到Copilot API
  → 流式/非流式响应处理 → 返回给客户端
  → 异步计费 & 异常检测
```

---

### Q4: 代码怎么读？

**参考**: EXPLORATION_SUMMARY.md → 代码阅读顺序建议

**推荐顺序**:
1. ✅ 先读这个文档 (5分钟)
2. `copilot_gateway_handler.go` ChatCompletions (20分钟)
3. `copilot_gateway_service.go` parseStreamUsage/parseNonStreamUsage (15分钟)
4. `gateway_service.go` RecordUsage (15分钟)
5. `anomaly_service.go` WriteAnomalyLog (10分钟)

**总耗时**: ~65分钟掌握全貌

---

## 🔑 关键概念

### CopilotUsage (上游响应数据结构)

```go
type CopilotUsage struct {
    PromptTokens     int  // 输入 token
    CompletionTokens int  // 输出 token
    TotalTokens      int  // 总 token
}
```

来自 Copilot API 的响应:
```json
{
    "usage": {
        "prompt_tokens": 100,
        "completion_tokens": 50,
        "total_tokens": 150
    }
}
```

---

### Token 完整流程

```
Copilot API 响应
    ↓
parseStreamUsage() 或 parseNonStreamUsage()
    ↓
CopilotUsage { PromptTokens: 100, CompletionTokens: 50 }
    ↓
CopilotForwardResult.Usage
    ↓
映射到 ForwardResult.Usage { InputTokens: 100, OutputTokens: 50 }
    ↓
handler 中捕获
    ↓
异步 goroutine 中调用 RecordUsage()
    ↓
UsageLog { InputTokens: 100, OutputTokens: 50 }
    ↓
数据库存储
```

---

### 异常检测规则

```go
if inputTokens == 0 AND outputTokens == 0 {
    // → 异常类型: "zero_token"
    // → 写入 request_logs 表
}

if durationMs > 60000 {
    // → 异常类型: "timeout"
}

if durationMs > 20000 {
    // → 异常类型: "slow_request"
}

if statusCode >= 500 {
    // → 异常类型: "error"
}
```

---

## 📊 核心文件一览

| 文件 | 行数 | 主要职责 | 核心方法 |
|------|------|---------|---------|
| `copilot_gateway_handler.go` | 1282 | HTTP 入口、认证、计费、账号选择 | ChatCompletions, Messages, Responses, Models |
| `copilot_gateway_service.go` | 1900+ | 请求转发、Token 提取、响应处理 | ForwardChatCompletions, parseStreamUsage, parseNonStreamUsage |
| `gateway_service.go` | 8566 | 计费、成本计算、使用量记录 | RecordUsage, CalculateCost, applyUsageBilling |
| `anomaly_service.go` | 238 | 异常检测、异常日志记录 | WriteAnomalyLog, detectAnomalies, GetSettings |

---

## 🚀 常用数据库查询

### 追踪单个请求

```sql
-- 1. 查看使用量日志
SELECT * FROM usage_logs 
WHERE request_id = '{request_id}';

-- 2. 查看异常日志 (如果有)
SELECT * FROM request_logs 
WHERE request_id = '{request_id}';

-- 3. 查看性能指标
SELECT 
    request_id,
    auth_latency_ms,
    routing_latency_ms,
    upstream_latency_ms,
    response_latency_ms,
    input_tokens,
    output_tokens
FROM usage_logs 
WHERE request_id = '{request_id}';
```

### 统计异常请求

```sql
-- Zero Token 异常
SELECT COUNT(*) as zero_token_count FROM request_logs 
WHERE anomaly_types LIKE '%zero_token%'
AND created_at > DATE_SUB(NOW(), INTERVAL 1 HOUR);

-- 慢请求异常
SELECT COUNT(*) as slow_request_count FROM request_logs 
WHERE anomaly_types LIKE '%slow_request%'
AND created_at > DATE_SUB(NOW(), INTERVAL 1 HOUR);

-- 超时异常
SELECT COUNT(*) as timeout_count FROM request_logs 
WHERE anomaly_types LIKE '%timeout%'
AND created_at > DATE_SUB(NOW(), INTERVAL 1 HOUR);
```

### 性能分析

```sql
-- 平均耗时分解
SELECT 
    AVG(auth_latency_ms) as avg_auth,
    AVG(routing_latency_ms) as avg_routing,
    AVG(upstream_latency_ms) as avg_upstream,
    AVG(response_latency_ms) as avg_response
FROM usage_logs 
WHERE created_at > DATE_SUB(NOW(), INTERVAL 1 HOUR);

-- P99 延迟
SELECT 
    PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY upstream_latency_ms) as p99_upstream
FROM usage_logs 
WHERE created_at > DATE_SUB(NOW(), INTERVAL 1 HOUR);
```

---

## 🐛 常见问题排查

### 问题: Token 计费不对

**排查步骤**:
1. 查看 usage_logs.input_tokens vs output_tokens
2. 查看 usage_logs.cache_read_tokens (可能被转换)
3. 查看 usage_logs.rate_multiplier (是否有特殊乘数)
4. 查看 usage_logs.actual_cost vs total_cost

---

### 问题: 请求经常返回 429

**排查步骤**:
1. 查看日志是否有 "rate_limited_failover"
2. 检查账号池大小 (Accounts 表)
3. 检查 MaxAccountSwitches 配置 (是否能充分转移)
4. 查看 upstream_latency_ms 是否过长

---

### 问题: 某些请求的 Token 为 0

**排查步骤**:
1. 启用异常日志: `settings.SaveRawData = true`
2. 查询 `request_logs` 表，过滤 anomaly_types 包含 'zero_token'
3. 查看 upstream_response_body 中是否有 usage 字段
4. 检查是否是流式请求且客户端断开了连接

---

## 📞 需要帮助？

- **性能问题**: 查看 FLOW_ANALYSIS.md → 4. 响应慢的原因
- **Token 问题**: 查看 FLOW_ANALYSIS.md → 3. Token 显示为 0 的原因
- **代码细节**: 参考 CODE_SNIPPETS.md
- **快速查找**: 使用 EXPLORATION_SUMMARY.md 的目录

---

## 📝 文档更新日期

- 最后更新: 2026-03-30
- 后端版本: Latest
- 涵盖内容: Copilot Gateway Handler, Service, Billing, Anomaly Detection

---

**Happy Debugging! 🚀**
