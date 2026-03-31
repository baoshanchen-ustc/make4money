# Sub2API Copilot Gateway 代码关键片段

## 1. Handler 入口 - ChatCompletions 完整流程

**文件**: `internal/handler/copilot_gateway_handler.go` (L117-425)

```go
func (h *CopilotGatewayHandler) ChatCompletions(c *gin.Context) {
    requestStart := time.Now()
    
    // [1] 认证
    apiKey, ok := middleware2.GetAPIKeyFromContext(c)
    if !ok {
        h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
        return
    }
    
    // [2] 读取请求体
    body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
    if err != nil {
        if maxErr, ok := extractMaxBytesError(err); ok {
            h.errorResponse(c, http.StatusRequestEntityTooLarge, ...)
            return
        }
        h.errorResponse(c, http.StatusBadRequest, ...)
        return
    }
    
    // [3] 验证 JSON
    if !gjson.ValidBytes(body) {
        h.errorResponse(c, http.StatusBadRequest, ...)
        return
    }
    
    // [4] 提取模型
    modelResult := gjson.GetBytes(body, "model")
    if !modelResult.Exists() || modelResult.String() == "" {
        h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
        return
    }
    reqModel := modelResult.String()
    reqStream := gjson.GetBytes(body, "stream").Bool()
    
    // [5] 检查计费资格
    subscription, _ := middleware2.GetSubscriptionFromContext(c)
    if err := h.billingCacheService.CheckBillingEligibility(
        c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription) {
        status, code, message := billingErrorDetails(err)
        h.errorResponse(c, status, code, message)
        return
    }
    
    // [6] 获取并发槽位
    ctx := c.Request.Context()
    userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlot(
        ctx, subject.UserID, subject.Concurrency)
    if !userAcquired {
        h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", ...)
        return
    }
    defer userReleaseFunc()
    service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
    
    // [7] 选择账号（支持故障转移）
    failedAccountIDs := make(map[int64]struct{})
    switchCount := 0
    
    for {
        // 选择账号，排除之前失败的
        account, err := h.gatewayService.SelectAccountForModelWithExclusions(
            ctx,
            apiKey.GroupID,
            "",          // sessionHash
            reqModel,
            failedAccountIDs,
        )
        if err != nil || account == nil {
            h.errorResponse(c, http.StatusServiceUnavailable, "api_error", ...)
            return
        }
        
        // [8] 检查请求体大小
        if h.checkCopilotBodySize(c, body, account, false) {
            return
        }
        
        // [9] 转发到 Copilot API
        forwardStart := time.Now()
        result, fwdErr := h.copilotGatewayService.ForwardChatCompletions(ctx, c, account, body)
        forwardDurationMs := time.Since(forwardStart).Milliseconds()
        
        // [10] 处理转发错误（含故障转移）
        if fwdErr != nil {
            if ctx.Err() == context.Canceled {
                h.errorResponse(c, StatusClientClosedRequest, "client_closed", ...)
                return
            }
            failedAccountIDs[account.ID] = struct{}{}
            switchCount++
            if switchCount >= h.maxAccountSwitches {
                h.errorResponse(c, http.StatusBadGateway, "upstream_error", ...)
                return
            }
            continue  // 重新循环，选择不同的账号
        }
        
        // [11] 处理特定的 HTTP 错误码
        if result.StatusCode == http.StatusMisdirectedRequest || result.StatusCode == http.StatusTooManyRequests {
            failedAccountIDs[account.ID] = struct{}{}
            switchCount++
            if switchCount >= h.maxAccountSwitches {
                h.errorResponse(c, http.StatusBadGateway, "upstream_error", ...)
                return
            }
            continue
        }
        
        if result.StatusCode != http.StatusOK {
            return  // 错误已由 service 转发给客户端
        }
        
        // [12] 异步记录使用量
        if result != nil && result.Usage != nil {
            inboundEp := snapshotInboundForUsageLog(c)
            userAgent := c.GetHeader("User-Agent")
            clientIP := ip.GetClientIP(c)
            capturedResult := result
            capturedAccount := account
            
            // 在 goroutine 外捕获 gin context 的值（context 不能跨 goroutine）
            authLatencyMs := getContextLatencyMsPtr(c, service.OpsAuthLatencyMsKey)
            routingLatencyMs := getContextLatencyMsPtr(c, service.OpsRoutingLatencyMsKey)
            upstreamLatencyMsVal := getContextLatencyMsPtr(c, service.OpsUpstreamLatencyMsKey)
            responseLatencyMsVal := getContextLatencyMsPtr(c, service.OpsResponseLatencyMsKey)
            capturedInitiator := service.CopilotInitiatorFromBody(body)
            capturedReqBody := body
            capturedUpstreamReqBody, capturedUpstreamRespBody := service.GetOpsUpstreamBodies(c)
            
            go func() {
                recordCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
                defer cancel()
                
                // 转换 Copilot 响应格式到通用格式
                fwdResult := &service.ForwardResult{
                    Model:         capturedResult.Model,
                    UpstreamModel: capturedResult.UpstreamModel,
                    Stream:        reqStream,
                    Usage: service.ClaudeUsage{
                        InputTokens:  capturedResult.Usage.PromptTokens,       // ← token 来源
                        OutputTokens: capturedResult.Usage.CompletionTokens,   // ← token 来源
                    },
                    Duration:     capturedResult.Duration,
                    FirstTokenMs: capturedResult.FirstTokenMs,
                }
                
                // 计费和记录使用量
                requestID, usageLogID, err := h.gatewayService.RecordUsage(recordCtx, 
                    &service.RecordUsageInput{
                        Result:            fwdResult,
                        APIKey:            apiKey,
                        User:              apiKey.User,
                        Account:           capturedAccount,
                        Subscription:      subscription,
                        InboundEndpoint:   inboundEp,
                        UpstreamEndpoint:  EndpointChatCompletions,
                        UserAgent:         userAgent,
                        IPAddress:         clientIP,
                        RequestBodyBytes:  intPtr(len(body)),
                        APIKeyService:     h.apiKeyService,
                        AuthLatencyMs:     authLatencyMs,
                        RoutingLatencyMs:  routingLatencyMs,
                        UpstreamLatencyMs: upstreamLatencyMsVal,
                        ResponseLatencyMs: responseLatencyMsVal,
                        Initiator:         capturedInitiator,
                    })
                if err != nil {
                    reqLog.Error("copilot.record_usage_failed", zap.Error(err))
                }
                
                // 异常检测
                if h.anomalyService != nil {
                    userID := apiKey.UserID
                    apiKeyID := apiKey.ID
                    accountID := capturedAccount.ID
                    var usageLogIDPtr *int64
                    if usageLogID != 0 {
                        usageLogIDPtr = &usageLogID
                    }
                    h.anomalyService.WriteAnomalyLog(
                        recordCtx,
                        capturedResult.Usage.PromptTokens,           // inputTokens
                        capturedResult.Usage.CompletionTokens,       // outputTokens
                        capturedResult.Duration.Milliseconds(),      // durationMs
                        200,                                         // statusCode
                        &service.RequestLogInput{
                            RequestID:            requestID,
                            UsageLogID:           usageLogIDPtr,
                            UserID:               &userID,
                            APIKeyID:             &apiKeyID,
                            AccountID:            &accountID,
                            GroupID:              apiKey.GroupID,
                            RequestBody:          capturedReqBody,
                            UpstreamRequestBody:  capturedUpstreamReqBody,
                            UpstreamResponseBody: capturedUpstreamRespBody,
                        },
                    )
                }
            }()
        }
        
        return  // 成功返回
    }
}
```

---

## 2. 转发服务 - Token 提取

**文件**: `internal/service/copilot_gateway_service.go`

### 2.1 ForwardChatCompletions 主流程 (L112-241)

```go
func (s *CopilotGatewayService) ForwardChatCompletions(
    ctx context.Context,
    c *gin.Context,
    account *Account,
    body []byte,
) (*CopilotForwardResult, error) {
    startTime := time.Now()
    
    // 模型重写
    body = mergeConsecutiveSameRoleMessagesInOpenAIBody(body)
    body, logModel := rewriteCopilotUpstreamModel(body, account)
    body = clampCopilotUpstreamMaxTokens(body, account)
    
    // 获取访问令牌
    token, err := s.tokenProvider.GetAccessToken(ctx, account)
    if err != nil {
        return nil, fmt.Errorf("copilot auth: %w", err)
    }
    
    // 构建并发送请求
    baseURL := copilot.CopilotAPIBase
    if customURL := strings.TrimSpace(account.GetCredential("base_url")); customURL != "" {
        baseURL = strings.TrimRight(customURL, "/")
    }
    
    upstreamURL := baseURL + "/chat/completions"
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")
    
    // 发送请求（5 分钟超时）
    upstreamStart := time.Now()
    resp, err := s.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("copilot: upstream request: %w", err)
    }
    
    // 处理响应
    if resp.StatusCode != http.StatusOK {
        return s.handleErrorResponse(c, resp, account)
    }
    
    // 检测流模式
    isStream := detectStreamMode(body)
    
    if isStream {
        return s.handleStreamingResponse(c, resp, logModel, "", startTime)
    } else {
        return s.handleNonStreamingResponse(c, resp, logModel, "", startTime)
    }
}
```

### 2.2 流式响应 - parseStreamUsage (L546-577)

```go
func (s *CopilotGatewayService) parseStreamUsage(data string, usage *CopilotUsage) {
    b := []byte(data)
    
    // 尝试 Chat Completions 格式
    var ccChunk struct {
        Usage *CopilotUsage `json:"usage"`
    }
    if err := json.Unmarshal(b, &ccChunk); err == nil && ccChunk.Usage != nil &&
        (ccChunk.Usage.PromptTokens > 0 || ccChunk.Usage.CompletionTokens > 0) {
        usage.PromptTokens = ccChunk.Usage.PromptTokens
        usage.CompletionTokens = ccChunk.Usage.CompletionTokens
        usage.TotalTokens = ccChunk.Usage.TotalTokens
        return
    }
    
    // 尝试 Responses API 格式
    var respChunk struct {
        Type     string `json:"type"`
        Response struct {
            Usage struct {
                InputTokens  int `json:"input_tokens"`
                OutputTokens int `json:"output_tokens"`
            } `json:"usage"`
        } `json:"response"`
    }
    if err := json.Unmarshal(b, &respChunk); err == nil &&
        respChunk.Type == "response.completed" {
        usage.PromptTokens = respChunk.Response.Usage.InputTokens
        usage.CompletionTokens = respChunk.Response.Usage.OutputTokens
        usage.TotalTokens = respChunk.Response.Usage.InputTokens + respChunk.Response.Usage.OutputTokens
    }
}
```

**流程**:
1. 逐行扫描 SSE 数据
2. 对每个 `data: {...}` 行调用 `parseStreamUsage`
3. 累积到 `usage` 对象中
4. 如果没有找到任何包含 usage 的行，返回 `&CopilotUsage{}` (全 0)

### 2.3 非流式响应 - parseNonStreamUsage (L582-609)

```go
func (s *CopilotGatewayService) parseNonStreamUsage(body []byte) *CopilotUsage {
    // 尝试 Chat Completions 格式：{"usage": {...}}
    var ccResp struct {
        Usage *CopilotUsage `json:"usage"`
    }
    if err := json.Unmarshal(body, &ccResp); err == nil && ccResp.Usage != nil &&
        (ccResp.Usage.PromptTokens > 0 || ccResp.Usage.CompletionTokens > 0) {
        return ccResp.Usage
    }
    
    // 尝试 Responses API 格式：{"usage": {"input_tokens": N, "output_tokens": M}}
    var respResp struct {
        Usage struct {
            InputTokens  int `json:"input_tokens"`
            OutputTokens int `json:"output_tokens"`
        } `json:"usage"`
    }
    if err := json.Unmarshal(body, &respResp); err == nil &&
        (respResp.Usage.InputTokens > 0 || respResp.Usage.OutputTokens > 0) {
        return &CopilotUsage{
            PromptTokens:     respResp.Usage.InputTokens,
            CompletionTokens: respResp.Usage.OutputTokens,
            TotalTokens:      respResp.Usage.InputTokens + respResp.Usage.OutputTokens,
        }
    }
    
    // ⚠️ 如果两种格式都失败，返回全 0 的 usage
    return &CopilotUsage{}
}
```

---

## 3. 计费服务 - RecordUsage

**文件**: `internal/service/gateway_service.go` (L7514-7702)

```go
func (s *GatewayService) RecordUsage(ctx context.Context, input *RecordUsageInput) (string, int64, error) {
    result := input.Result
    apiKey := input.APIKey
    user := input.User
    account := input.Account
    subscription := input.Subscription
    
    // [1] 强制缓存计费
    if input.ForceCacheBilling && result.Usage.InputTokens > 0 {
        result.Usage.CacheReadInputTokens += result.Usage.InputTokens
        result.Usage.InputTokens = 0  // ⚠️ 清零！
    }
    
    // [2] Cache TTL 覆盖
    cacheTTLOverridden := false
    if account.IsCacheTTLOverrideEnabled() {
        applyCacheTTLOverride(&result.Usage, account.GetCacheTTLOverrideTarget())
        cacheTTLOverridden = (result.Usage.CacheCreation5mTokens + result.Usage.CacheCreation1hTokens) > 0
    }
    
    // [3] 获取费率倍数
    multiplier := 1.0
    if s.cfg != nil {
        multiplier = s.cfg.Default.RateMultiplier
    }
    
    // [4] 选择计费方式
    var cost *CostBreakdown
    
    if result.MediaType == "image" || result.MediaType == "video" {
        // Sora 计费
        cost = s.billingService.CalculateSoraImageCost(...)
    } else if result.ImageCount > 0 {
        // 图片生成计费
        cost = s.billingService.CalculateImageCost(...)
    } else {
        // [5] Token 计费核心逻辑
        tokens := UsageTokens{
            InputTokens:           result.Usage.InputTokens,              // ← **从 Copilot 响应来**
            OutputTokens:          result.Usage.OutputTokens,             // ← **从 Copilot 响应来**
            CacheCreationTokens:   result.Usage.CacheCreationInputTokens,
            CacheReadTokens:       result.Usage.CacheReadInputTokens,
            CacheCreation5mTokens: result.Usage.CacheCreation5mTokens,
            CacheCreation1hTokens: result.Usage.CacheCreation1hTokens,
        }
        cost, err = s.billingService.CalculateCost(result.Model, tokens, multiplier)
        if err != nil {
            cost = &CostBreakdown{ActualCost: 0}
        }
    }
    
    // [6] 确定计费方式（订阅 vs 余额）
    isSubscriptionBilling := subscription != nil && apiKey.Group != nil && apiKey.Group.IsSubscriptionType()
    
    // [7] 创建使用日志
    durationMs := int(result.Duration.Milliseconds())
    usageLog := &UsageLog{
        UserID:                user.ID,
        APIKeyID:              apiKey.ID,
        AccountID:             account.ID,
        RequestID:             requestID,
        Model:                 result.Model,
        InputTokens:           result.Usage.InputTokens,              // ← 存储
        OutputTokens:          result.Usage.OutputTokens,             // ← 存储
        CacheCreationTokens:   result.Usage.CacheCreationInputTokens,
        CacheReadTokens:       result.Usage.CacheReadInputTokens,
        CacheCreation5mTokens: result.Usage.CacheCreation5mTokens,
        CacheCreation1hTokens: result.Usage.CacheCreation1hTokens,
        InputCost:             cost.InputCost,
        OutputCost:            cost.OutputCost,
        TotalCost:             cost.TotalCost,
        ActualCost:            cost.ActualCost,
        RateMultiplier:        multiplier,
        BillingType:           billingType,
        Stream:                result.Stream,
        DurationMs:            &durationMs,
        FirstTokenMs:          result.FirstTokenMs,
        AuthLatencyMs:         input.AuthLatencyMs,
        RoutingLatencyMs:      input.RoutingLatencyMs,
        UpstreamLatencyMs:     input.UpstreamLatencyMs,
        ResponseLatencyMs:     input.ResponseLatencyMs,
        CreatedAt:             time.Now(),
    }
    
    // [8] 在简单模式下直接记录（不计费）
    if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
        writeUsageLogBestEffort(ctx, s.usageLogRepo, usageLog, "service.gateway")
        return requestID, usageLog.ID, nil
    }
    
    // [9] 应用计费
    billingErr := func() error {
        _, err := applyUsageBilling(ctx, requestID, usageLog, &postUsageBillingParams{
            Cost:                  cost,
            User:                  user,
            APIKey:                apiKey,
            Account:               account,
            Subscription:          subscription,
            IsSubscriptionBill:    isSubscriptionBilling,
            AccountRateMultiplier: accountRateMultiplier,
            APIKeyService:         input.APIKeyService,
        }, s.billingDeps(), s.usageBillingRepo)
        return err
    }()
    
    if billingErr != nil {
        return requestID, usageLog.ID, billingErr
    }
    
    // [10] 写入数据库
    writeUsageLogBestEffort(ctx, s.usageLogRepo, usageLog, "service.gateway")
    
    return requestID, usageLog.ID, nil
}
```

---

## 4. 异常检测 - AnomalyService

**文件**: `internal/service/anomaly_service.go` (L200-238)

```go
func (s *AnomalyService) WriteAnomalyLog(
    ctx context.Context,
    inputTokens, outputTokens int,
    durationMs int64,
    statusCode int,
    input *RequestLogInput,
) {
    // [1] 脱离请求生命周期
    bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
    defer cancel()
    
    // [2] 获取异常检测配置
    settings := s.GetSettings(bgCtx)
    
    // [3] 检测异常
    anomalies := detectAnomalies(inputTokens, outputTokens, durationMs, statusCode, settings)
    if len(anomalies) == 0 {
        return
    }
    
    // [4] 准备日志输入
    logInput := *input
    logInput.AnomalyTypes = make([]string, len(anomalies))
    for i, a := range anomalies {
        logInput.AnomalyTypes[i] = string(a)
    }
    
    // [5] 可选清除原始数据
    if !settings.SaveRawData {
        logInput.RequestBody = nil
        logInput.UpstreamRequestBody = nil
        logInput.UpstreamResponseBody = nil
    }
    
    // [6] 写入数据库
    if s.requestLogRepo == nil {
        return
    }
    
    if err := s.requestLogRepo.Save(bgCtx, &logInput); err != nil {
        slog.Error("failed to write anomaly request log", "request_id", logInput.RequestID, "error", err)
    }
}

// 异常检测逻辑
func detectAnomalies(inputTokens, outputTokens int, durationMs int64, statusCode int, settings *AnomalySettings) []AnomalyType {
    var types []AnomalyType
    
    // ⚠️ Zero Token 检测：BOTH inputTokens AND outputTokens 都为 0
    if settings.DetectZeroToken && inputTokens == 0 && outputTokens == 0 {
        types = append(types, AnomalyZeroToken)
    }
    
    // 超时检测
    if durationMs > settings.TimeoutThresholdMs {
        types = append(types, AnomalyTimeout)
    } else if durationMs > settings.SlowRequestThresholdMs {
        // 慢请求检测
        types = append(types, AnomalySlowRequest)
    }
    
    // 错误检测
    if statusCode >= 500 {
        types = append(types, AnomalyError)
    }
    
    return types
}
```

**默认配置** (L31-36):
```go
var defaultAnomalySettings = AnomalySettings{
    SlowRequestThresholdMs: 20000,    // 20 秒
    TimeoutThresholdMs:     60000,    // 60 秒
    DetectZeroToken:        true,     // 启用 zero_token 检测 ✓
    SaveRawData:            true,     // 保存原始数据 ✓
}
```

---

## 5. 类型定义

### 5.1 CopilotUsage

```go
type CopilotUsage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}
```

### 5.2 CopilotForwardResult

```go
type CopilotForwardResult struct {
    StatusCode    int
    Model         string
    UpstreamModel string
    Usage         *CopilotUsage          // ← token 在这里
    Duration      time.Duration
    FirstTokenMs  *int
    ReasoningEffort *string
}
```

### 5.3 RecordUsageInput

```go
type RecordUsageInput struct {
    Result            *ForwardResult
    APIKey            *APIKey
    User              *User
    Account           *Account
    Subscription      *UserSubscription
    InboundEndpoint   string
    UpstreamEndpoint  string
    UserAgent         string
    IPAddress         string
    RequestBodyBytes  *int
    APIKeyService     APIKeyQuotaUpdater
    AuthLatencyMs     *int  // 认证耗时
    RoutingLatencyMs  *int  // 路由耗时
    UpstreamLatencyMs *int  // 上游耗时
    ResponseLatencyMs *int  // 响应耗时
    Initiator         string
}
```

### 5.4 AnomalySettings

```go
type AnomalySettings struct {
    SlowRequestThresholdMs int64 `json:"slow_request_threshold_ms"`
    TimeoutThresholdMs     int64 `json:"timeout_threshold_ms"`
    DetectZeroToken        bool  `json:"detect_zero_token"`
    SaveRawData            bool  `json:"save_raw_data"`
}
```

---

## 6. 关键常量

```go
// copilot_gateway_handler.go
const (
    fallbackCopilotMaxBodyBytes = 1024 * 1024 * 1024  // 1 GB
    StatusClientClosedRequest   = 499                  // nginx convention
    copilotModelCacheTTL        = 1 * time.Hour
    copilotModelCacheFailedTTL  = 2 * time.Minute
)

// copilot_gateway_service.go
const (
    copilotModelEndpointsCacheTTL      = 1 * time.Hour
    copilotModelEndpointsCacheFailedTTL = 2 * time.Minute
)

// anomaly_service.go
const (
    settingKeySlowRequestMs   = "ops.anomaly.slow_request_threshold_ms"
    settingKeyTimeoutMs       = "ops.anomaly.timeout_threshold_ms"
    settingKeyDetectZeroToken = "ops.anomaly.detect_zero_token"
    settingKeySaveRawData     = "ops.anomaly.save_raw_data"
    anomalySettingsCacheTTL   = 30 * time.Second
)

// 默认异常阈值
var defaultAnomalySettings = AnomalySettings{
    SlowRequestThresholdMs: 20000,    // 20 秒
    TimeoutThresholdMs:     60000,    // 60 秒
    DetectZeroToken:        true,
    SaveRawData:            true,
}
```

---

## 7. 错误处理与故障转移

### 7.1 故障转移条件 (L313-328)

```go
// 421 Misdirected Request（HTTP/2 连接重用错误）
if result != nil && result.StatusCode == http.StatusMisdirectedRequest {
    failedAccountIDs[account.ID] = struct{}{}
    switchCount++
    if switchCount >= h.maxAccountSwitches {
        // 所有账号都失败
        h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
        return
    }
    continue  // 尝试下一个账号
}

// 429 Too Many Requests（频率限制）
if result != nil && result.StatusCode == http.StatusTooManyRequests {
    failedAccountIDs[account.ID] = struct{}{}
    switchCount++
    if switchCount >= h.maxAccountSwitches {
        h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "All Copilot accounts are rate limited")
        return
    }
    continue
}
```

---

## 8. 性能监测点

### 8.1 OpsSpan 记录

```go
// 路由选择阶段
service.AppendOpsSpan(c, service.OpsSpan{
    Name:        "routing.select",
    StartUnixMs: routingStart.UnixMilli(),
    DurationMs:  time.Since(routingStart).Milliseconds(),
    Status:      "ok",
    Attrs:       map[string]any{"account_id": account.ID, "platform": "copilot"},
})

// 上游请求阶段
service.AppendOpsSpan(c, service.OpsSpan{
    Name:        "upstream.post",
    StartUnixMs: upstreamStart.UnixMilli(),
    DurationMs:  time.Since(upstreamStart).Milliseconds(),
    Status:      upstreamStatus,
    Attrs:       map[string]any{"account_id": account.ID, "endpoint": "chat/completions"},
})
```

### 8.2 延迟指标

```go
service.OpsAuthLatencyMsKey      // 认证鉴权
service.OpsRoutingLatencyMsKey   // 路由选择
service.OpsUpstreamLatencyMsKey  // 上游请求（记录在发送请求时）
service.OpsResponseLatencyMsKey  // 响应处理
```

