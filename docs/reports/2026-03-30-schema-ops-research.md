# Sub2API Database Schema & Ops Implementation Research

**Date**: March 30, 2026  
**Project**: sub2api  
**Scope**: Research only, no modifications made

---

## 1. CREATE TABLE STATEMENTS

### 1.1 ops_error_logs (Migration: 033_ops_monitoring_vnext.sql)

```sql
CREATE TABLE IF NOT EXISTS ops_error_logs (
    id BIGSERIAL PRIMARY KEY,

    -- Correlation / identities
    request_id VARCHAR(64),
    client_request_id VARCHAR(64),
    user_id BIGINT,
    api_key_id BIGINT,
    account_id BIGINT,
    group_id BIGINT,
    client_ip inet,

    -- Dimensions for global filtering
    platform VARCHAR(32),

    -- Request metadata
    model VARCHAR(100),
    request_path VARCHAR(256),
    stream BOOLEAN NOT NULL DEFAULT false,
    user_agent TEXT,

    -- Core error classification
    error_phase VARCHAR(32) NOT NULL,
    error_type VARCHAR(64) NOT NULL,
    severity VARCHAR(8) NOT NULL DEFAULT 'P2',
    status_code INT,

    -- vNext metric semantics
    is_business_limited BOOLEAN NOT NULL DEFAULT false,

    -- Error details (sanitized/truncated at ingest time)
    error_message TEXT,
    error_body TEXT,

    -- Provider/upstream details (optional; useful for trends & account health)
    error_source VARCHAR(64),
    error_owner VARCHAR(32),
    account_status VARCHAR(50),
    upstream_status_code INT,
    upstream_error_message TEXT,
    upstream_error_detail TEXT,
    provider_error_code VARCHAR(64),
    provider_error_type VARCHAR(64),
    network_error_type VARCHAR(50),
    retry_after_seconds INT,

    -- Timings (ms) - optional
    duration_ms INT,
    time_to_first_token_ms BIGINT,
    auth_latency_ms BIGINT,
    routing_latency_ms BIGINT,
    upstream_latency_ms BIGINT,
    response_latency_ms BIGINT,

    -- Retry context (only stored for error requests)
    request_body JSONB,
    request_headers JSONB,
    request_body_truncated BOOLEAN NOT NULL DEFAULT false,
    request_body_bytes INT,

    -- Retryability flags (best-effort classification)
    is_retryable BOOLEAN NOT NULL DEFAULT false,
    retry_count INT NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE ops_error_logs IS 
  'Ops error logs (vNext). Stores sanitized error details and request_body for retries (errors only).';

-- INDEXES (from migration 033)
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_created_at ON ops_error_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_platform_time ON ops_error_logs (platform, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_group_time ON ops_error_logs (group_id, created_at DESC)
    WHERE group_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_account_time ON ops_error_logs (account_id, created_at DESC)
    WHERE account_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_status_time ON ops_error_logs (status_code, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_phase_time ON ops_error_logs (error_phase, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_type_time ON ops_error_logs (error_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_request_id ON ops_error_logs (request_id);
CREATE INDEX IF NOT EXISTS idx_ops_error_logs_client_request_id ON ops_error_logs (client_request_id);

-- Optional pg_trgm fuzzy search indexes (created if extension is available)
-- CREATE INDEX IF NOT EXISTS idx_ops_error_logs_request_id_trgm ON ops_error_logs USING gin (request_id gin_trgm_ops);
-- CREATE INDEX IF NOT EXISTS idx_ops_error_logs_client_request_id_trgm ON ops_error_logs USING gin (client_request_id gin_trgm_ops);
-- CREATE INDEX IF NOT EXISTS idx_ops_error_logs_error_message_trgm ON ops_error_logs USING gin (error_message gin_trgm_ops);
```

---

### 1.2 usage_logs (Migration: 001_init.sql)

```sql
CREATE TABLE IF NOT EXISTS usage_logs (
    id                          BIGSERIAL PRIMARY KEY,
    user_id                     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id                  BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    account_id                  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    request_id                  VARCHAR(64),
    model                       VARCHAR(100) NOT NULL,

    -- Token使用量（4类）
    input_tokens                INT NOT NULL DEFAULT 0,
    output_tokens               INT NOT NULL DEFAULT 0,
    cache_creation_tokens       INT NOT NULL DEFAULT 0,
    cache_read_tokens           INT NOT NULL DEFAULT 0,

    -- 详细的缓存创建分类
    cache_creation_5m_tokens    INT NOT NULL DEFAULT 0,
    cache_creation_1h_tokens    INT NOT NULL DEFAULT 0,

    -- 费用（USD）
    input_cost                  DECIMAL(20, 10) NOT NULL DEFAULT 0,
    output_cost                 DECIMAL(20, 10) NOT NULL DEFAULT 0,
    cache_creation_cost         DECIMAL(20, 10) NOT NULL DEFAULT 0,
    cache_read_cost             DECIMAL(20, 10) NOT NULL DEFAULT 0,
    total_cost                  DECIMAL(20, 10) NOT NULL DEFAULT 0,  -- 原始总费用
    actual_cost                 DECIMAL(20, 10) NOT NULL DEFAULT 0,  -- 实际扣除费用

    -- 元数据
    stream                      BOOLEAN NOT NULL DEFAULT FALSE,
    duration_ms                 INT,

    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_usage_logs_user_id ON usage_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_api_key_id ON usage_logs(api_key_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_account_id ON usage_logs(account_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_model ON usage_logs(model);
CREATE INDEX IF NOT EXISTS idx_usage_logs_created_at ON usage_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_usage_logs_user_created ON usage_logs(user_id, created_at);
```

**Note**: `usage_logs` has been extended via migrations 009, 028-031, 046-047, 055, 060-062, 070-079 with additional columns including:
- `auth_latency_ms`, `routing_latency_ms`, `upstream_latency_ms`, `response_latency_ms` (stage latencies)
- `user_agent`, `ip_address` (tracking)
- `upstream_model`, `inbound_endpoint`, `upstream_endpoint`, `request_type`, `request_body_bytes`, `service_tier`, `reasoning_effort`, `cache_ttl_overridden` (metadata)

---

### 1.3 request_logs (Migration: 080_add_request_logs.sql)

```sql
CREATE TABLE IF NOT EXISTS request_logs (
    id                     BIGSERIAL    PRIMARY KEY,
    request_id             VARCHAR(64)  NOT NULL,
    usage_log_id           BIGINT       REFERENCES usage_logs(id) ON DELETE SET NULL,

    user_id                BIGINT,
    api_key_id             BIGINT,
    account_id             BIGINT,
    group_id               BIGINT,

    anomaly_types          TEXT[]       NOT NULL,

    request_body           JSONB,
    upstream_request_body  JSONB,
    upstream_response_body JSONB,

    created_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT request_logs_anomaly_types_nonempty
        CHECK (cardinality(anomaly_types) > 0),
    CONSTRAINT request_logs_anomaly_types_valid
        CHECK (anomaly_types <@ ARRAY['zero_token','slow_request','timeout','error']::text[])
);

CREATE INDEX IF NOT EXISTS idx_request_logs_request_id ON request_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_request_logs_created_at ON request_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_user_id    ON request_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_api_key_id ON request_logs(api_key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_account_id ON request_logs(account_id, created_at DESC);
```

**Purpose**: Stores requests matching anomaly detection criteria (zero tokens, slow/timeout requests, errors).

**Constraints**:
- `anomaly_types` must be non-empty (checked via `cardinality()`)
- `anomaly_types` values must be one of: `'zero_token'`, `'slow_request'`, `'timeout'`, `'error'`

---

## 2. setOps* FUNCTIONS IN copilot_gateway_handler.go

### 2.1 setOpsRequestContext

**File**: `backend/internal/handler/ops_error_logger.go`  
**Lines**: 332-346

```go
func setOpsRequestContext(c *gin.Context, model string, stream bool, requestBody []byte) {
    if c == nil {
        return
    }
    model = strings.TrimSpace(model)
    c.Set(opsModelKey, model)
    c.Set(opsStreamKey, stream)
    if len(requestBody) > 0 {
        c.Set(opsRequestBodyKey, requestBody)
    }
    if c.Request != nil && model != "" {
        ctx := context.WithValue(c.Request.Context(), ctxkey.Model, model)
        c.Request = c.Request.WithContext(ctx)
    }
}
```

**Parameters**:
1. `c` (*gin.Context) - HTTP request context
2. `model` (string) - AI model name (e.g., "claude-3-sonnet", "gpt-4")
3. `stream` (bool) - Whether response is streamed
4. `requestBody` ([]byte) - Raw HTTP request body

**Sets on context**:
- `"ops_model"` → model name
- `"ops_stream"` → streaming flag
- `"ops_request_body"` → request body bytes (if non-empty)
- Gin context Request → ctxkey.Model for downstream middleware

**Calls in copilot_gateway_handler.go**:
- Line 154: `setOpsRequestContext(c, "", false, body)` (initial, before parsing)
- Line 173: `setOpsRequestContext(c, reqModel, reqStream, body)` (after model extraction) 
- Line 578: `setOpsRequestContext(c, "", false, body)` (Responses endpoint, initial)
- Line 594: `setOpsRequestContext(c, reqModel, reqStream, body)` (Responses endpoint, after parsing)
- Line 925: `setOpsRequestContext(c, reqModel, reqStream, body)` (Messages/Anthropic endpoint)
- Line 1017: `setOpsRequestContext(c, reqModel, reqStream, body)` (Messages, after truncation)

---

### 2.2 setOpsSelectedAccount

**File**: `backend/internal/handler/ops_error_logger.go`  
**Lines**: 364-379

```go
func setOpsSelectedAccount(c *gin.Context, accountID int64, platform ...string) {
    if c == nil || accountID <= 0 {
        return
    }
    c.Set(opsAccountIDKey, accountID)
    if c.Request != nil {
        ctx := context.WithValue(c.Request.Context(), ctxkey.AccountID, accountID)
        if len(platform) > 0 {
            p := strings.TrimSpace(platform[0])
            if p != "" {
                ctx = context.WithValue(ctx, ctxkey.Platform, p)
            }
        }
        c.Request = c.Request.WithContext(ctx)
    }
}
```

**Parameters**:
1. `c` (*gin.Context) - HTTP request context
2. `accountID` (int64) - Upstream account ID
3. `platform` (variadic string) - Optional platform name (e.g., "copilot", "openai", "anthropic")

**Sets on context**:
- `"ops_account_id"` → account ID
- Gin context Request → ctxkey.AccountID 
- Gin context Request → ctxkey.Platform (if provided)

**Calls in copilot_gateway_handler.go**:
- Line 231: `setOpsSelectedAccount(c, account.ID, account.Platform)` (ChatCompletions)
- Line 647: `setOpsSelectedAccount(c, account.ID, account.Platform)` (Responses)
- Line 989: `setOpsSelectedAccount(c, account.ID, account.Platform)` (Messages/Anthropic)

---

### 2.3 Additional Latency Setters (via service.SetOpsLatencyMs)

**File**: `backend/internal/service/ops_upstream_context.go`  
**Lines**: 95-100

```go
func SetOpsLatencyMs(c *gin.Context, key string, value int64) {
    if c == nil || strings.TrimSpace(key) == "" || value < 0 {
        return
    }
    c.Set(key, value)
}
```

**Latency Keys** (constants from ops_upstream_context.go):
- `OpsAuthLatencyMsKey` = `"ops_auth_latency_ms"` - Auth/billing check latency
- `OpsRoutingLatencyMsKey` = `"ops_routing_latency_ms"` - Account selection latency
- `OpsUpstreamLatencyMsKey` = `"ops_upstream_latency_ms"` - Upstream request latency
- `OpsResponseLatencyMsKey` = `"ops_response_latency_ms"` - Response handling latency

**Calls in copilot_gateway_handler.go (ChatCompletions)**:
- Line 199: `SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())`
- Line 239: `SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())`
- Lines 246-249: Compute `OpsResponseLatencyMsKey` from forward duration and upstream latency

**Similar patterns in**:
- Responses endpoint (lines 618-664)
- Messages/Anthropic endpoint (lines 959-1043)

---

## 3. OpsErrorLoggerMiddleware

**File**: `backend/internal/handler/ops_error_logger.go`  
**Lines**: 445-863

### 3.1 Middleware Setup

```go
func OpsErrorLoggerMiddleware(ops *service.OpsService) gin.HandlerFunc {
    return func(c *gin.Context) {
        originalWriter := c.Writer
        w := acquireOpsCaptureWriter(originalWriter)
        defer func() { ... }()
        c.Writer = w
        c.Next()
        
        if ops == nil { return }
        if !ops.IsMonitoringEnabled(c.Request.Context()) { return }
        
        // ... error processing logic ...
    }
}
```

### 3.2 Fields Captured for Errors (status >= 400)

**From Request/Context** (lines 507-545):
- `request_id` - From X-Request-Id header
- `client_request_id` - From gin context
- `api_key_id`, `user_id` - From APIKey object
- `group_id` - From APIKey.GroupID
- `client_ip` - From ip.GetClientIP()
- `user_agent` - From User-Agent header
- `model` - From gin context `opsModelKey`
- `stream` - From gin context `opsStreamKey`
- `request_path` - From c.Request.URL.Path
- `account_id`, `platform` - From gin context `opsAccountIDKey` or inferred from path

**Error Response Parsing** (lines 695, 738-741):
- `error_message` - Extracted from response body
- `error_type` - Normalized error type ("invalid_request_error", "authentication_error", "billing_error", etc.)
- `status_code` - HTTP response status code
- `error_phase` - Classified phase ("request", "auth", "routing", "upstream", "network", "internal")
- `severity` - Classified severity ("P0", "P1", "P2", "P3")
- `is_business_limited` - Boolean: user-level business limits (insufficient balance, quota, etc.)
- `is_retryable` - Boolean: can be safely retried

**Latency Fields** (line 657, applyOpsLatencyFieldsFromContext):
- `auth_latency_ms` - From OpsAuthLatencyMsKey
- `routing_latency_ms` - From OpsRoutingLatencyMsKey
- `upstream_latency_ms` - From OpsUpstreamLatencyMsKey
- `response_latency_ms` - From OpsResponseLatencyMsKey
- `time_to_first_token_ms` - From OpsTimeToFirstTokenMsKey

**Request Body Storage** (lines 680-681):
- `request_body` (JSONB) - Sanitized & truncated request body (see `attachOpsRequestBodyToEntry`)
- `request_headers` (JSONB) - Whitelisted headers only (anthropic-beta, anthropic-version)
- `request_body_bytes` - Byte size of original request
- `request_body_truncated` - Boolean: whether body was truncated

**Upstream Error Context** (lines 784-833):
- `upstream_status_code` - Original upstream HTTP status
- `upstream_error_message` - Upstream error message
- `upstream_error_detail` - Upstream error detail
- `upstream_errors` (JSONB array) - Array of `OpsUpstreamErrorEvent` objects capturing failover/retry chain

**Skip Monitoring** (line 684):
- Checks `OpsSkipPassthroughKey` - if true, skips logging (set by error passthrough rules)

### 3.3 Special Case: Successful Request with Upstream Error Context (lines 469-505)

When status < 400 but upstream error events are recorded (due to retry/failover):
- Logs a "Recovered upstream error" message
- Creates ops_error_logs entry with phase="upstream", error_type="upstream_error"
- Records failover attempts as `upstream_errors` array

---

## 4. AnomalyTypes Computation at Query Time

**File**: `backend/internal/repository/ops_repo_request_details.go`  
**Lines**: 14-407

### 4.1 Anomaly Types Definition

**Four anomaly types** (from service/anomaly_service.go):
1. `"zero_token"` - Input + output tokens both = 0
2. `"slow_request"` - Duration > SlowRequestThresholdMs (default 20000ms) AND <= TimeoutThresholdMs (default 60000ms)
3. `"timeout"` - Duration > TimeoutThresholdMs (default 60000ms)
4. `"error"` - Status code >= 500

### 4.2 Query-Time Computation (SQL)

**Location**: ListRequestDetails, lines 245-250

```sql
ARRAY_REMOVE(ARRAY[
    CASE WHEN input_tokens = 0 AND output_tokens = 0 THEN 'zero_token' ELSE NULL END,
    CASE WHEN duration_ms > {slowMs} AND duration_ms <= {timeoutMs} THEN 'slow_request' ELSE NULL END,
    CASE WHEN duration_ms > {timeoutMs} THEN 'timeout' ELSE NULL END,
    CASE WHEN status_code >= 500 THEN 'error' ELSE NULL END
], NULL) AS anomaly_types
```

**Dynamic Thresholds** (lines 34-39):
```go
slowMs := int64(20000)
timeoutMs := int64(60000)
if filter != nil && filter.AnomalySettingsForFilter != nil {
    slowMs = filter.AnomalySettingsForFilter.SlowRequestThresholdMs
    timeoutMs = filter.AnomalySettingsForFilter.TimeoutThresholdMs
}
```

### 4.3 Data Sources

The CTE (Common Table Expression) unions two sources:

**FROM usage_logs** (lines 120-153):
- Successful requests (status_code=200)
- Columns: `duration_ms`, `input_tokens`, `output_tokens`, `status_code`
- Latency fields populated: `auth_latency_ms`, `routing_latency_ms`, `upstream_latency_ms`, `response_latency_ms`

**FROM ops_error_logs** (lines 157-191):
- Error requests (status_code >= 400)
- Columns: `duration_ms`, `status_code`
- Token fields: 0 for errors (since tokens only counted on success)
- Latency fields: NULL for errors (not tracked on error path)

### 4.4 Anomaly Filter Logic (lines 91-110)

When `filter.AnomalyTypes` is provided (e.g., ["slow_request", "timeout"]):

```go
anomalyConditions := []string{}
for _, at := range filter.AnomalyTypes {
    switch service.AnomalyType(at) {
    case service.AnomalyZeroToken:
        // (input_tokens = 0 AND output_tokens = 0)
    case service.AnomalySlowRequest:
        // (duration_ms > slowMs AND duration_ms <= timeoutMs)
    case service.AnomalyTimeout:
        // (duration_ms > timeoutMs)
    case service.AnomalyError:
        // (status_code >= 500)
    }
}
// Final WHERE: (cond1 OR cond2 OR cond3) — matches ANY anomaly type
```

**OR semantics**: A row matches if it satisfies at least one of the requested anomaly types.

---

## 5. Anomaly Detection (Background Write)

**File**: `backend/internal/service/anomaly_service.go`

### 5.1 Settings (Configurable Thresholds)

```go
type AnomalySettings struct {
    SlowRequestThresholdMs int64  // Default: 20000ms
    TimeoutThresholdMs     int64  // Default: 60000ms
    DetectZeroToken        bool   // Default: true
    SaveRawData            bool   // Default: true
}
```

**Storage**: Settings table with keys:
- `ops.anomaly.slow_request_threshold_ms`
- `ops.anomaly.timeout_threshold_ms`
- `ops.anomaly.detect_zero_token`
- `ops.anomaly.save_raw_data`

**Cache**: 30-second TTL on settings (GetSettings)

### 5.2 Detection Function

**Function**: `detectAnomalies` (lines 176-194)

```go
func detectAnomalies(inputTokens, outputTokens int, durationMs int64, statusCode int, settings *AnomalySettings) []AnomalyType {
    var types []AnomalyType
    
    if settings.DetectZeroToken && inputTokens == 0 && outputTokens == 0 {
        types = append(types, AnomalyZeroToken)
    }
    
    if durationMs > settings.TimeoutThresholdMs {
        types = append(types, AnomalyTimeout)
    } else if durationMs > settings.SlowRequestThresholdMs {
        types = append(types, AnomalySlowRequest)
    }
    
    if statusCode >= 500 {
        types = append(types, AnomalyError)
    }
    
    return types
}
```

**Note**: Timeout checked first; if duration > timeout, only "timeout" is added (not also "slow_request").

### 5.3 Background Write

**Function**: `WriteAnomalyLog` (lines 200-238)

Called asynchronously from handler goroutines after successful request completion:

```go
h.anomalyService.WriteAnomalyLog(
    recordCtx,
    capturedResult.Usage.PromptTokens,
    capturedResult.Usage.CompletionTokens,
    capturedResult.Duration.Milliseconds(),
    200,  // statusCode
    &service.RequestLogInput{
        RequestID:            capturedRequestID,
        UserID:               &userID,
        APIKeyID:             &apiKeyID,
        AccountID:            &accountID,
        GroupID:              apiKey.GroupID,
        RequestBody:          capturedReqBody,
        UpstreamRequestBody:  capturedUpstreamReqBody,
        UpstreamResponseBody: capturedUpstreamRespBody,
    },
)
```

**Logic**:
1. Get anomaly settings (with 30-sec cache)
2. Call `detectAnomalies()`
3. If any anomalies detected:
   - Save to request_logs table
   - Include request/upstream bodies (unless SaveRawData=false)
   - Set anomaly_types array

---

## 6. Summary Table: Context Keys & Latency Fields

| Key | Set By | Used In | Type | Example Value |
|-----|--------|---------|------|---------------|
| `ops_model` | setOpsRequestContext | OpsErrorLoggerMiddleware | string | "claude-3-sonnet" |
| `ops_stream` | setOpsRequestContext | OpsErrorLoggerMiddleware | bool | true/false |
| `ops_request_body` | setOpsRequestContext | attachOpsRequestBodyToEntry | []byte | Raw JSON |
| `ops_account_id` | setOpsSelectedAccount | OpsErrorLoggerMiddleware | int64 | 12345 |
| `ops_auth_latency_ms` | SetOpsLatencyMs | OpsErrorLoggerMiddleware, usage_logs | int64 | 45 |
| `ops_routing_latency_ms` | SetOpsLatencyMs | OpsErrorLoggerMiddleware, usage_logs | int64 | 123 |
| `ops_upstream_latency_ms` | SetOpsLatencyMs (gateway service) | OpsErrorLoggerMiddleware, usage_logs | int64 | 1234 |
| `ops_response_latency_ms` | SetOpsLatencyMs | OpsErrorLoggerMiddleware, usage_logs | int64 | 89 |
| `ops_time_to_first_token_ms` | SetOpsLatencyMs (stream handler) | OpsErrorLoggerMiddleware | int64 | 567 |
| `ops_upstream_status_code` | SetOpsUpstreamError (gateway) | OpsErrorLoggerMiddleware | int | 503 |
| `ops_upstream_error_message` | SetOpsUpstreamError (gateway) | OpsErrorLoggerMiddleware | string | "Service unavailable" |
| `ops_upstream_error_detail` | SetOpsUpstreamError (gateway) | OpsErrorLoggerMiddleware | string | "Rate limited" |
| `ops_upstream_errors` | appendOpsUpstreamError (gateway) | OpsErrorLoggerMiddleware | []OpsUpstreamErrorEvent | Array of events |
| `ops_upstream_request_body` | setOpsUpstreamRequestBody | Request capture | []byte | Upstream request JSON |
| `ops_upstream_response_body` | setOpsUpstreamResponseBody | Request capture | []byte | Upstream response JSON |
| `ops_skip_passthrough` | applyErrorPassthroughRule / checkSkipMonitoringForUpstreamEvent | OpsErrorLoggerMiddleware | bool | true (skip logging) |

---

## 7. Files Modified During Research

All files are **read-only** (no modifications):

1. ✓ `backend/migrations/033_ops_monitoring_vnext.sql` - ops_error_logs schema + indexes
2. ✓ `backend/migrations/001_init.sql` - usage_logs schema
3. ✓ `backend/migrations/080_add_request_logs.sql` - request_logs schema
4. ✓ `backend/internal/handler/copilot_gateway_handler.go` - All setOps* calls documented
5. ✓ `backend/internal/handler/ops_error_logger.go` - setOpsRequestContext, setOpsSelectedAccount, OpsErrorLoggerMiddleware
6. ✓ `backend/internal/service/ops_upstream_context.go` - SetOpsLatencyMs, context key definitions
7. ✓ `backend/internal/service/anomaly_service.go` - AnomalySettings, detectAnomalies, WriteAnomalyLog
8. ✓ `backend/internal/service/ops_request_details.go` - OpsRequestDetail, OpsRequestDetailFilter
9. ✓ `backend/internal/repository/ops_repo_request_details.go` - ListRequestDetails SQL with anomaly computation

---

## 8. Key Findings

### ops_error_logs Characteristics
- High-write table (~500+ fields captured per error)
- No foreign keys (to reduce write locks)
- Intentionally loose schema to accommodate different error sources
- Request body + headers stored for retry capability
- Upstream error events stored as JSONB array for failover history

### usage_logs Characteristics
- Records only successful requests (status=200)
- Stores stage latencies separately from total duration
- Extended via 20+ migrations with tracking/analytics fields
- Supports both token and cost accounting

### request_logs Characteristics
- Sparse table (only anomalous requests)
- Enforced constraints on anomaly_types (non-empty, whitelisted values)
- Optional raw request bodies for debugging

### Anomaly Detection Strategy
- **At write time**: No anomalies written; raw tokens/duration captured
- **At query time**: Anomalies computed dynamically with configurable thresholds
- **Settings-driven**: Admin can adjust timeouts without schema change
- **Background service**: Can selectively save raw bodies if SaveRawData=false

---

**Research completed**: 2026-03-30
