# 运维异常检测与请求排查增强 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在运维请求列表增加异常高亮与多选筛选，在请求详情增加身份信息与异常请求原始数据展示。

**Architecture:** 后端新增 `request_logs` 表存储异常请求原始数据，`AnomalyService` 封装异常检测逻辑与 30s 缓存设置读取，在网关 handler 异步写入；列表 API 在查询层动态计算 `anomaly_types`；前端新增 4 个独立组件，改造列表页与详情面板。

**Tech Stack:** Go (Ent ORM, database/sql, gin, wire DI), PostgreSQL, Vue 3 + TypeScript + Element Plus

---

## File Structure

### New Files
| File | Responsibility |
|------|---------------|
| `backend/migrations/080_add_request_logs.sql` | DB migration: request_logs table |
| `backend/ent/schema/request_log.go` | Ent schema for request_log entity |
| `backend/internal/repository/request_log_repo.go` | Save + GetByRequestID data access |
| `backend/internal/service/anomaly_service.go` | AnomalySettings cache + detection logic + async write |
| `frontend/src/views/admin/ops/components/AnomalyBadge.vue` | Single anomaly type badge |
| `frontend/src/views/admin/ops/components/AnomalyFilterChips.vue` | Multi-select anomaly filter chips |
| `frontend/src/views/admin/ops/components/AnomalySettingsModal.vue` | Admin threshold config modal |
| `frontend/src/views/admin/ops/components/RawDataAccordion.vue` | Accordion JSON data display |

### Modified Files
| File | Changes |
|------|---------|
| `backend/internal/service/ops_request_details.go` | Add anomaly fields to OpsRequestDetail, OpsUsageInspectDetail, filter |
| `backend/internal/repository/ops_repo_request_details.go` | Add users/api_keys JOIN + anomaly_types dynamic computation |
| `backend/internal/repository/ops_repo_usage_inspect.go` | Add user_name/api_key_label JOIN + request_logs left join |
| `backend/internal/handler/admin/ops_handler.go` | GET/PUT anomaly settings endpoints + anomaly_types filter param |
| `backend/internal/handler/copilot_gateway_handler.go` | Inject AnomalyService, call async write post-response |
| `backend/internal/handler/sora_gateway_handler.go` | Inject AnomalyService, call async write post-response |
| `backend/cmd/server/wire_gen.go` | Wire RequestLogRepository + AnomalyService |
| `frontend/src/api/admin/ops.ts` | New types + getAnomalySettings/updateAnomalySettings |
| `frontend/src/views/admin/ops/OpsRequestInspectView.vue` | Filter bar, table columns, row color class |
| `frontend/src/views/admin/ops/components/OpsRequestDetailPanel.vue` | Identity block + raw data block |

---

### Task 1: DB Migration — request_logs table

**Files:**
- Create: `backend/migrations/080_add_request_logs.sql`

- [ ] **Step 1: Create migration file**

```sql
-- backend/migrations/080_add_request_logs.sql
CREATE TABLE IF NOT EXISTS request_logs (
    id                     BIGSERIAL    PRIMARY KEY,
    request_id             VARCHAR(64)  NOT NULL,
    usage_log_id           BIGINT       REFERENCES usage_logs(id) ON DELETE SET NULL,

    user_id                BIGINT,
    api_key_id             BIGINT,
    account_id             BIGINT,
    group_id               BIGINT,

    -- anomaly_types stores which anomaly conditions triggered this record
    -- possible values: 'zero_token', 'slow_request', 'timeout', 'error'
    anomaly_types          TEXT[]       NOT NULL DEFAULT '{}',

    request_body           JSONB,
    upstream_request_body  JSONB,
    upstream_response_body JSONB,

    created_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_request_logs_request_id ON request_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_request_logs_created_at ON request_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_logs_user_id    ON request_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_request_logs_api_key_id ON request_logs(api_key_id);
```

- [ ] **Step 2: Apply migration locally**

```bash
cd /Users/ziji/personal/github/sub2api
just migrate-up
# or: go run ./backend/cmd/migrate/main.go up
```

Expected: migration 080 applied successfully, no errors.

- [ ] **Step 3: Verify table exists**

```bash
psql $DATABASE_URL -c "\d request_logs"
```

Expected: table columns id, request_id, usage_log_id, user_id, api_key_id, account_id, group_id, anomaly_types, request_body, upstream_request_body, upstream_response_body, created_at.

- [ ] **Step 4: Commit**

```bash
git add backend/migrations/080_add_request_logs.sql
git commit -m "Feature: 添加 request_logs 表存储异常请求原始数据"
```

---

### Task 2: Ent Schema — RequestLog

**Files:**
- Create: `backend/ent/schema/request_log.go`

- [ ] **Step 1: Write the Ent schema**

```go
// backend/ent/schema/request_log.go
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RequestLog stores raw request/response bodies for anomalous requests only.
type RequestLog struct {
	ent.Schema
}

func (RequestLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Positive().Immutable(),
		field.String("request_id").MaxLen(64).NotEmpty(),
		field.Int64("usage_log_id").Optional().Nillable(),
		field.Int64("user_id").Optional().Nillable(),
		field.Int64("api_key_id").Optional().Nillable(),
		field.Int64("account_id").Optional().Nillable(),
		field.Int64("group_id").Optional().Nillable(),
		field.Strings("anomaly_types").Default([]string{}),
		field.Bytes("request_body").Optional().Nillable(),
		field.Bytes("upstream_request_body").Optional().Nillable(),
		field.Bytes("upstream_response_body").Optional().Nillable(),
		field.Time("created_at").Immutable(),
	}
}

func (RequestLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("request_id"),
		index.Fields("created_at"),
		index.Fields("user_id"),
		index.Fields("api_key_id"),
	}
}
```

- [ ] **Step 2: Generate Ent code**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go generate ./ent/...
```

Expected: `backend/ent/request_log.go`, `backend/ent/request_log_create.go`, etc. generated without errors.

- [ ] **Step 3: Verify compilation**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

Expected: no compilation errors.

- [ ] **Step 4: Commit**

```bash
git add backend/ent/schema/request_log.go backend/ent/
git commit -m "Feature: 添加 RequestLog Ent schema 定义"
```

---

### Task 3: RequestLogRepository — Save and GetByRequestID

**Files:**
- Create: `backend/internal/repository/request_log_repo.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/repository/request_log_repo_test.go`:

```go
package repository_test

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestRequestLogRepository_SaveAndGet(t *testing.T) {
	// Integration test: requires real DB connection via test helper
	// Skip if no test DB available
	db := getTestDB(t) // uses existing test DB helper pattern in this package
	repo := NewRequestLogRepository(db)

	input := &service.RequestLogInput{
		RequestID:    "test-req-001",
		AnomalyTypes: []string{"zero_token", "slow_request"},
		RequestBody:  []byte(`{"model":"gpt-4","messages":[]}`),
	}

	err := repo.Save(context.Background(), input)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, err := repo.GetByRequestID(context.Background(), "test-req-001")
	if err != nil {
		t.Fatalf("GetByRequestID failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected result, got nil")
	}
	if got.RequestID != "test-req-001" {
		t.Errorf("got RequestID %q, want %q", got.RequestID, "test-req-001")
	}
	if len(got.AnomalyTypes) != 2 {
		t.Errorf("got %d anomaly types, want 2", len(got.AnomalyTypes))
	}
}
```

- [ ] **Step 2: Add RequestLogInput and RequestLogData to service layer**

Add to `backend/internal/service/anomaly_service.go` (created in Task 4), but define the types here first.

Create `backend/internal/repository/request_log_repo.go`:

```go
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type requestLogRepository struct {
	db *sql.DB
}

// NewRequestLogRepository creates a repository backed by a raw *sql.DB.
func NewRequestLogRepository(db *sql.DB) service.RequestLogRepository {
	return &requestLogRepository{db: db}
}

const maxRequestBodyBytes = 1024 * 1024 // 1 MB

// truncateBody truncates body to maxRequestBodyBytes and appends a truncation marker.
func truncateBody(b []byte) []byte {
	if len(b) <= maxRequestBodyBytes {
		return b
	}
	// Try to produce valid JSON with truncation marker
	marker := []byte(`{"_truncated":true,"_original_size":` + fmt.Sprintf("%d", len(b)) + `}`)
	return marker
}

func (r *requestLogRepository) Save(ctx context.Context, input *service.RequestLogInput) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil request log repository")
	}

	var requestBody, upstreamReqBody, upstreamRespBody []byte
	if input.RequestBody != nil {
		requestBody = truncateBody(input.RequestBody)
		if !json.Valid(requestBody) {
			requestBody = nil
		}
	}
	if input.UpstreamRequestBody != nil {
		upstreamReqBody = truncateBody(input.UpstreamRequestBody)
		if !json.Valid(upstreamReqBody) {
			upstreamReqBody = nil
		}
	}
	if input.UpstreamResponseBody != nil {
		upstreamRespBody = truncateBody(input.UpstreamResponseBody)
		if !json.Valid(upstreamRespBody) {
			upstreamRespBody = nil
		}
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO request_logs
  (request_id, usage_log_id, user_id, api_key_id, account_id, group_id,
   anomaly_types, request_body, upstream_request_body, upstream_response_body, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		input.RequestID,
		nullInt64(input.UsageLogID),
		nullInt64(input.UserID),
		nullInt64(input.APIKeyID),
		nullInt64(input.AccountID),
		nullInt64(input.GroupID),
		pq.Array(input.AnomalyTypes),
		nullBytes(requestBody),
		nullBytes(upstreamReqBody),
		nullBytes(upstreamRespBody),
		time.Now().UTC(),
	)
	return err
}

func (r *requestLogRepository) GetByRequestID(ctx context.Context, requestID string) (*service.RequestLogData, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil request log repository")
	}
	row := r.db.QueryRowContext(ctx, `
SELECT request_id, anomaly_types, request_body, upstream_request_body, upstream_response_body
FROM request_logs
WHERE request_id = $1
ORDER BY created_at DESC
LIMIT 1`, requestID)

	var out service.RequestLogData
	var anomalyTypes pq.StringArray
	var reqBody, upReqBody, upRespBody []byte

	if err := row.Scan(
		&out.RequestID,
		&anomalyTypes,
		&reqBody,
		&upReqBody,
		&upRespBody,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	out.AnomalyTypes = []string(anomalyTypes)
	if reqBody != nil {
		cp := make([]byte, len(reqBody))
		copy(cp, reqBody)
		out.RequestBody = cp
	}
	if upReqBody != nil {
		cp := make([]byte, len(upReqBody))
		copy(cp, upReqBody)
		out.UpstreamRequestBody = cp
	}
	if upRespBody != nil {
		cp := make([]byte, len(upRespBody))
		copy(cp, upRespBody)
		out.UpstreamResponseBody = cp
	}
	return &out, nil
}

// nullInt64 converts *int64 to sql.NullInt64.
func nullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

// nullBytes returns nil if b is empty (JSONB NULL), otherwise b.
func nullBytes(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return b
}
```

- [ ] **Step 3: Run test**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/repository/... -run TestRequestLogRepository -v
```

Expected: PASS (or SKIP if no test DB configured).

- [ ] **Step 4: Verify compilation**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/repository/request_log_repo.go
git commit -m "Feature: 添加 RequestLogRepository 实现异常请求原始数据的保存与查询"
```

---

### Task 4: AnomalyService — settings cache + detection + async write

**Files:**
- Create: `backend/internal/service/anomaly_service.go`

This file contains:
1. `AnomalyType` constants and `AnomalySettings` struct
2. `RequestLogInput` and `RequestLogData` types (used by repository)
3. `RequestLogRepository` interface
4. `AnomalyService` struct with cached settings + detection + async write

- [ ] **Step 1: Write failing tests**

Create `backend/internal/service/anomaly_service_test.go`:

```go
package service_test

import (
	"testing"
)

func TestDetectAnomalies_ZeroToken(t *testing.T) {
	settings := &AnomalySettings{
		SlowRequestThresholdMs: 20000,
		TimeoutThresholdMs:     60000,
		DetectZeroToken:        true,
		SaveRawData:            true,
	}
	types := detectAnomalies(0, 0, 5000, 200, settings)
	if len(types) != 1 || types[0] != AnomalyZeroToken {
		t.Errorf("expected [zero_token], got %v", types)
	}
}

func TestDetectAnomalies_SlowRequest(t *testing.T) {
	settings := &AnomalySettings{
		SlowRequestThresholdMs: 20000,
		TimeoutThresholdMs:     60000,
		DetectZeroToken:        true,
		SaveRawData:            true,
	}
	types := detectAnomalies(100, 200, 25000, 200, settings)
	if len(types) != 1 || types[0] != AnomalySlowRequest {
		t.Errorf("expected [slow_request], got %v", types)
	}
}

func TestDetectAnomalies_Timeout(t *testing.T) {
	settings := &AnomalySettings{
		SlowRequestThresholdMs: 20000,
		TimeoutThresholdMs:     60000,
		DetectZeroToken:        true,
	}
	types := detectAnomalies(0, 0, 70000, 200, settings)
	// zero_token + timeout (not slow_request because timeout supersedes it)
	found := map[string]bool{}
	for _, t2 := range types {
		found[string(t2)] = true
	}
	if !found["zero_token"] || !found["timeout"] || found["slow_request"] {
		t.Errorf("expected [zero_token, timeout], got %v", types)
	}
}

func TestDetectAnomalies_Error(t *testing.T) {
	settings := &AnomalySettings{SlowRequestThresholdMs: 20000, TimeoutThresholdMs: 60000}
	types := detectAnomalies(100, 200, 1000, 500, settings)
	if len(types) != 1 || types[0] != AnomalyError {
		t.Errorf("expected [error], got %v", types)
	}
}

func TestDetectAnomalies_Normal(t *testing.T) {
	settings := &AnomalySettings{SlowRequestThresholdMs: 20000, TimeoutThresholdMs: 60000, DetectZeroToken: true}
	types := detectAnomalies(100, 200, 5000, 200, settings)
	if len(types) != 0 {
		t.Errorf("expected no anomalies, got %v", types)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestDetectAnomalies -v
```

Expected: FAIL with compilation errors (types not defined yet).

- [ ] **Step 3: Create anomaly_service.go**

```go
// backend/internal/service/anomaly_service.go
package service

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// AnomalyType enumerates the anomaly categories.
type AnomalyType string

const (
	AnomalyZeroToken   AnomalyType = "zero_token"
	AnomalySlowRequest AnomalyType = "slow_request"
	AnomalyTimeout     AnomalyType = "timeout"
	AnomalyError       AnomalyType = "error"
)

// AnomalySettings holds admin-configurable thresholds for anomaly detection.
type AnomalySettings struct {
	SlowRequestThresholdMs int64 `json:"slow_request_threshold_ms"`
	TimeoutThresholdMs     int64 `json:"timeout_threshold_ms"`
	DetectZeroToken        bool  `json:"detect_zero_token"`
	SaveRawData            bool  `json:"save_raw_data"`
}

// defaultAnomalySettings are used when DB is unreachable.
var defaultAnomalySettings = AnomalySettings{
	SlowRequestThresholdMs: 20000,
	TimeoutThresholdMs:     60000,
	DetectZeroToken:        true,
	SaveRawData:            true,
}

const (
	settingKeySlowRequestMs   = "ops.anomaly.slow_request_threshold_ms"
	settingKeyTimeoutMs        = "ops.anomaly.timeout_threshold_ms"
	settingKeyDetectZeroToken  = "ops.anomaly.detect_zero_token"
	settingKeySaveRawData      = "ops.anomaly.save_raw_data"
	anomalySettingsCacheTTL    = 30 * time.Second
)

// RequestLogInput is the data written to request_logs for an anomalous request.
type RequestLogInput struct {
	RequestID            string
	UsageLogID           *int64
	UserID               *int64
	APIKeyID             *int64
	AccountID            *int64
	GroupID              *int64
	AnomalyTypes         []string
	RequestBody          []byte
	UpstreamRequestBody  []byte
	UpstreamResponseBody []byte
}

// RequestLogData is what the repository returns when reading a request log.
type RequestLogData struct {
	RequestID            string
	AnomalyTypes         []string
	RequestBody          []byte
	UpstreamRequestBody  []byte
	UpstreamResponseBody []byte
}

// RequestLogRepository is the port for request log persistence.
type RequestLogRepository interface {
	Save(ctx context.Context, input *RequestLogInput) error
	GetByRequestID(ctx context.Context, requestID string) (*RequestLogData, error)
}

// AnomalyService handles anomaly settings (with cache) and async write of anomalous request logs.
type AnomalyService struct {
	settingRepo SettingRepository
	requestLogRepo RequestLogRepository

	mu       sync.RWMutex
	cached   *AnomalySettings
	expireAt time.Time
}

// NewAnomalyService creates a new AnomalyService.
func NewAnomalyService(settingRepo SettingRepository, requestLogRepo RequestLogRepository) *AnomalyService {
	return &AnomalyService{
		settingRepo:    settingRepo,
		requestLogRepo: requestLogRepo,
	}
}

// GetSettings returns cached anomaly settings, refreshing from DB if TTL has expired.
func (s *AnomalyService) GetSettings(ctx context.Context) *AnomalySettings {
	s.mu.RLock()
	if s.cached != nil && time.Now().Before(s.expireAt) {
		cp := *s.cached
		s.mu.RUnlock()
		return &cp
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	// Double-check after acquiring write lock.
	if s.cached != nil && time.Now().Before(s.expireAt) {
		cp := *s.cached
		return &cp
	}

	settings := s.loadFromDB(ctx)
	s.cached = settings
	s.expireAt = time.Now().Add(anomalySettingsCacheTTL)
	cp := *settings
	return &cp
}

// loadFromDB reads settings from the DB; returns defaults on error.
func (s *AnomalyService) loadFromDB(ctx context.Context) *AnomalySettings {
	if s.settingRepo == nil {
		def := defaultAnomalySettings
		return &def
	}
	keys := []string{settingKeySlowRequestMs, settingKeyTimeoutMs, settingKeyDetectZeroToken, settingKeySaveRawData}
	vals, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		def := defaultAnomalySettings
		return &def
	}
	out := defaultAnomalySettings
	if v, ok := vals[settingKeySlowRequestMs]; ok && v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			out.SlowRequestThresholdMs = n
		}
	}
	if v, ok := vals[settingKeyTimeoutMs]; ok && v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			out.TimeoutThresholdMs = n
		}
	}
	if v, ok := vals[settingKeyDetectZeroToken]; ok && v != "" {
		out.DetectZeroToken = v == "true" || v == "1"
	}
	if v, ok := vals[settingKeySaveRawData]; ok && v != "" {
		out.SaveRawData = v == "true" || v == "1"
	}
	return &out
}

// UpdateSettings persists new settings and invalidates cache.
func (s *AnomalyService) UpdateSettings(ctx context.Context, settings *AnomalySettings) error {
	kvs := map[string]string{
		settingKeySlowRequestMs:  strconv.FormatInt(settings.SlowRequestThresholdMs, 10),
		settingKeyTimeoutMs:       strconv.FormatInt(settings.TimeoutThresholdMs, 10),
		settingKeyDetectZeroToken: strconv.FormatBool(settings.DetectZeroToken),
		settingKeySaveRawData:     strconv.FormatBool(settings.SaveRawData),
	}
	if err := s.settingRepo.SetMultiple(ctx, kvs); err != nil {
		return err
	}
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
	return nil
}

// detectAnomalies computes which anomaly types apply for a completed request.
// Exported for testing via package-level function.
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

// WriteAnomalyLog asynchronously checks for anomalies and writes to request_logs if any found.
// Must be called in a goroutine. Never blocks the caller.
func (s *AnomalyService) WriteAnomalyLog(
	ctx context.Context,
	inputTokens, outputTokens int,
	durationMs int64,
	statusCode int,
	input *RequestLogInput,
) {
	settings := s.GetSettings(ctx)
	anomalies := detectAnomalies(inputTokens, outputTokens, durationMs, statusCode, settings)
	if len(anomalies) == 0 {
		return
	}

	input.AnomalyTypes = make([]string, len(anomalies))
	for i, a := range anomalies {
		input.AnomalyTypes[i] = string(a)
	}

	if !settings.SaveRawData {
		// Still record the anomaly type but strip raw bodies.
		input.RequestBody = nil
		input.UpstreamRequestBody = nil
		input.UpstreamResponseBody = nil
	}

	if s.requestLogRepo == nil {
		return
	}
	if err := s.requestLogRepo.Save(ctx, input); err != nil {
		slog.Error("failed to write anomaly request log", "request_id", input.RequestID, "error", err)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestDetectAnomalies -v
```

Expected: 5 tests PASS.

- [ ] **Step 5: Verify compilation**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/anomaly_service.go backend/internal/service/anomaly_service_test.go
git commit -m "Feature: 添加 AnomalyService 异常检测与异步写入逻辑"
```

---

### Task 5: Extend OpsRequestDetail, OpsUsageInspectDetail, and OpsRequestDetailFilter

**Files:**
- Modify: `backend/internal/service/ops_request_details.go`

- [ ] **Step 1: Add new fields to OpsRequestDetail**

In `backend/internal/service/ops_request_details.go`, add to `OpsRequestDetail` struct after `ResponseLatencyMs`:

```go
// Identity fields — populated via JOIN with users and api_keys tables.
UserName    *string `json:"user_name,omitempty"`
APIKeyLabel *string `json:"api_key_label,omitempty"` // masked: last 4 chars of key
GroupName   *string `json:"group_name,omitempty"`
AccountName *string `json:"account_name,omitempty"`

// AnomalyTypes is dynamically computed at query layer from duration_ms and token fields.
AnomalyTypes []string `json:"anomaly_types,omitempty"`
```

- [ ] **Step 2: Add new fields to OpsUsageInspectDetail**

In `OpsUsageInspectDetail` struct after `IPAddress`:

```go
// Identity fields — populated via JOIN.
UserName    *string `json:"user_name,omitempty"`
APIKeyLabel *string `json:"api_key_label,omitempty"`

// Anomaly data — from request_logs table.
AnomalyTypes         []string        `json:"anomaly_types,omitempty"`
RequestBody          json.RawMessage `json:"request_body,omitempty"`
UpstreamRequestBody  json.RawMessage `json:"upstream_request_body,omitempty"`
UpstreamResponseBody json.RawMessage `json:"upstream_response_body,omitempty"`
```

Add `"encoding/json"` import if not already present.

- [ ] **Step 3: Add AnomalyTypes field to OpsRequestDetailFilter**

In `OpsRequestDetailFilter` struct after `MaxDurationMs`:

```go
// AnomalyTypes filters rows matching any of the given anomaly types.
// Computed dynamically at the SQL layer (no dependency on request_logs).
AnomalyTypes []string
```

Also add anomaly threshold fields needed for the dynamic WHERE clause:

```go
// AnomalySettingsForFilter holds thresholds used to compute anomaly conditions
// in the SQL WHERE clause. Populated by the handler from AnomalyService.GetSettings.
AnomalySettingsForFilter *AnomalySettings
```

- [ ] **Step 4: Verify compilation**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/ops_request_details.go
git commit -m "Feature: 扩展 OpsRequestDetail/OpsUsageInspectDetail/Filter 添加异常与身份字段"
```

---

### Task 6: Extend ops_repo_request_details.go — identity JOINs + anomaly_types computation

**Files:**
- Modify: `backend/internal/repository/ops_repo_request_details.go`

The existing query uses a CTE that already JOINs `groups` and `accounts`. We need to:
1. Add `users` and `api_keys` LEFT JOINs to both branches of the UNION ALL
2. Include `user_name` and `api_key_label` in the SELECT
3. Add `anomaly_types` as a computed expression
4. Add a WHERE condition for `anomaly_types` filter

- [ ] **Step 1: Rewrite the CTE in ListRequestDetails**

Replace the `cte` string constant. The key changes are:
- Add `u.username AS user_name` and `RIGHT(ak.key, 4) AS api_key_label` to both branches
- Add `LEFT JOIN users u ON u.id = ul.user_id` (and `o.user_id` for error branch)
- Add `LEFT JOIN api_keys ak ON ak.id = ul.api_key_id` (and `o.api_key_id`)
- Add `ul.input_tokens, ul.output_tokens` to success branch (NULL for error branch)

New CTE (replace the existing `cte := ` assignment):

```go
cte := `
WITH combined AS (
  SELECT
    'success'::TEXT AS kind,
    ul.created_at AS created_at,
    ul.request_id AS request_id,
    COALESCE(NULLIF(g.platform, ''), NULLIF(a.platform, ''), '') AS platform,
    ul.model AS model,
    ul.duration_ms AS duration_ms,
    200 AS status_code,
    NULL::BIGINT AS error_id,
    NULL::TEXT AS phase,
    NULL::TEXT AS severity,
    NULL::TEXT AS message,
    ul.user_id AS user_id,
    ul.api_key_id AS api_key_id,
    ul.account_id AS account_id,
    ul.group_id AS group_id,
    ul.stream AS stream,
    ul.request_body_bytes AS request_body_bytes,
    ul.auth_latency_ms AS auth_latency_ms,
    ul.routing_latency_ms AS routing_latency_ms,
    ul.upstream_latency_ms AS upstream_latency_ms,
    ul.response_latency_ms AS response_latency_ms,
    u.username AS user_name,
    CASE WHEN ak.key IS NOT NULL THEN '***' || RIGHT(ak.key, 4) ELSE NULL END AS api_key_label,
    COALESCE(g.name, '') AS group_name,
    COALESCE(a.name, '') AS account_name,
    COALESCE(ul.input_tokens, 0) AS input_tokens,
    COALESCE(ul.output_tokens, 0) AS output_tokens
  FROM usage_logs ul
  LEFT JOIN groups g ON g.id = ul.group_id
  LEFT JOIN accounts a ON a.id = ul.account_id
  LEFT JOIN users u ON u.id = ul.user_id
  LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
  WHERE ul.created_at >= $1 AND ul.created_at < $2

  UNION ALL

  SELECT
    'error'::TEXT AS kind,
    o.created_at AS created_at,
    COALESCE(NULLIF(o.request_id,''), NULLIF(o.client_request_id,''), '') AS request_id,
    COALESCE(NULLIF(o.platform, ''), NULLIF(g.platform, ''), NULLIF(a.platform, ''), '') AS platform,
    o.model AS model,
    o.duration_ms AS duration_ms,
    o.status_code AS status_code,
    o.id AS error_id,
    o.error_phase AS phase,
    o.severity AS severity,
    o.error_message AS message,
    o.user_id AS user_id,
    o.api_key_id AS api_key_id,
    o.account_id AS account_id,
    o.group_id AS group_id,
    o.stream AS stream,
    o.request_body_bytes AS request_body_bytes,
    NULL::INT AS auth_latency_ms,
    NULL::INT AS routing_latency_ms,
    NULL::INT AS upstream_latency_ms,
    NULL::INT AS response_latency_ms,
    u.username AS user_name,
    CASE WHEN ak.key IS NOT NULL THEN '***' || RIGHT(ak.key, 4) ELSE NULL END AS api_key_label,
    COALESCE(g.name, '') AS group_name,
    COALESCE(a.name, '') AS account_name,
    0 AS input_tokens,
    0 AS output_tokens
  FROM ops_error_logs o
  LEFT JOIN groups g ON g.id = o.group_id
  LEFT JOIN accounts a ON a.id = o.account_id
  LEFT JOIN users u ON u.id = o.user_id
  LEFT JOIN api_keys ak ON ak.id = o.api_key_id
  WHERE o.created_at >= $1 AND o.created_at < $2
    AND COALESCE(o.status_code, 0) >= 400
)
`
```

- [ ] **Step 2: Add anomaly_types filter condition**

After the existing `addCondition` calls in the filter block, add:

```go
if filter != nil && len(filter.AnomalyTypes) > 0 && filter.AnomalySettingsForFilter != nil {
    s := filter.AnomalySettingsForFilter
    anomalyConditions := make([]string, 0, len(filter.AnomalyTypes))
    for _, at := range filter.AnomalyTypes {
        switch AnomalyType(at) {
        case AnomalyZeroToken:
            anomalyConditions = append(anomalyConditions, "(input_tokens = 0 AND output_tokens = 0)")
        case AnomalySlowRequest:
            anomalyConditions = append(anomalyConditions,
                fmt.Sprintf("(duration_ms > %d AND duration_ms <= %d)", s.SlowRequestThresholdMs, s.TimeoutThresholdMs))
        case AnomalyTimeout:
            anomalyConditions = append(anomalyConditions,
                fmt.Sprintf("(duration_ms > %d)", s.TimeoutThresholdMs))
        case AnomalyError:
            anomalyConditions = append(anomalyConditions, "(status_code >= 500)")
        }
    }
    if len(anomalyConditions) > 0 {
        conditions = append(conditions, "("+strings.Join(anomalyConditions, " OR ")+")")
    }
}
```

Note: `AnomalyZeroToken`, `AnomalySlowRequest`, etc. are defined in `service` package — import them as `service.AnomalyZeroToken`.

- [ ] **Step 3: Update the SELECT list in listQuery**

Replace the listQuery SELECT columns to include the 5 new columns + compute anomaly_types:

```go
listQuery := fmt.Sprintf(`
%s
SELECT
  kind,
  created_at,
  request_id,
  platform,
  model,
  duration_ms,
  status_code,
  error_id,
  phase,
  severity,
  message,
  user_id,
  api_key_id,
  account_id,
  group_id,
  stream,
  request_body_bytes,
  auth_latency_ms,
  routing_latency_ms,
  upstream_latency_ms,
  response_latency_ms,
  user_name,
  api_key_label,
  group_name,
  account_name,
  ARRAY_REMOVE(ARRAY[
    CASE WHEN input_tokens = 0 AND output_tokens = 0 THEN 'zero_token' ELSE NULL END,
    CASE WHEN duration_ms > %d AND duration_ms <= %d THEN 'slow_request' ELSE NULL END,
    CASE WHEN duration_ms > %d THEN 'timeout' ELSE NULL END,
    CASE WHEN status_code >= 500 THEN 'error' ELSE NULL END
  ], NULL) AS anomaly_types
FROM combined
%s
%s
LIMIT $%d OFFSET $%d
`, cte,
  slowMs, timeoutMs, timeoutMs,
  where, sort, len(args)+1, len(args)+2)
```

Where `slowMs` and `timeoutMs` come from `filter.AnomalySettingsForFilter` (or defaults if nil):

```go
slowMs := int64(20000)
timeoutMs := int64(60000)
if filter != nil && filter.AnomalySettingsForFilter != nil {
    slowMs = filter.AnomalySettingsForFilter.SlowRequestThresholdMs
    timeoutMs = filter.AnomalySettingsForFilter.TimeoutThresholdMs
}
```

Add this block just before building `listQuery`.

- [ ] **Step 4: Update the Scan call to include new columns**

Add new scan variables after `responseLatencyMs`:

```go
var (
    userName       sql.NullString
    apiKeyLabel    sql.NullString
    groupNameStr   sql.NullString
    accountNameStr sql.NullString
    anomalyTypes   pq.StringArray
)
```

Add to the `rows.Scan(...)` call at the end:
```
&userName,
&apiKeyLabel,
&groupNameStr,
&accountNameStr,
&anomalyTypes,
```

- [ ] **Step 5: Populate new fields in the item struct**

After the existing field assignments in `item := &service.OpsRequestDetail{...}`, add:

```go
if userName.Valid && userName.String != "" {
    s := userName.String
    item.UserName = &s
}
if apiKeyLabel.Valid && apiKeyLabel.String != "" {
    s := apiKeyLabel.String
    item.APIKeyLabel = &s
}
if groupNameStr.Valid && groupNameStr.String != "" {
    s := groupNameStr.String
    item.GroupName = &s
}
if accountNameStr.Valid && accountNameStr.String != "" {
    s := accountNameStr.String
    item.AccountName = &s
}
if len(anomalyTypes) > 0 {
    item.AnomalyTypes = []string(anomalyTypes)
}
```

Add `"github.com/lib/pq"` import.

- [ ] **Step 6: Build and verify**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/repository/ops_repo_request_details.go
git commit -m "Feature: ops 请求列表新增身份 JOIN 和动态 anomaly_types 计算"
```

---

### Task 7: Extend ops_repo_usage_inspect.go — identity fields + request_logs data

**Files:**
- Modify: `backend/internal/repository/ops_repo_usage_inspect.go`

- [ ] **Step 1: Add user_name and api_key_label JOINs**

Modify the query in `GetLatestUsageInspectByRequestID` to add 2 new columns and 2 new JOINs:

Add to SELECT (after `ip_address`):
```sql
  u.username AS user_name,
  CASE WHEN ak.key IS NOT NULL THEN '***' || RIGHT(ak.key, 4) ELSE NULL END AS api_key_label
```

Add to FROM clause (after `LEFT JOIN accounts a ON ...`):
```sql
LEFT JOIN users u ON u.id = ul.user_id
LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
```

- [ ] **Step 2: Update Scan call and field assignments**

Add scan variables:
```go
var userName    sql.NullString
var apiKeyLabel sql.NullString
```

Add to Scan: `&userName, &apiKeyLabel`

Add field assignments after `ipAddr` handling:
```go
if userName.Valid && userName.String != "" {
    s := userName.String
    out.UserName = &s
}
if apiKeyLabel.Valid && apiKeyLabel.String != "" {
    s := apiKeyLabel.String
    out.APIKeyLabel = &s
}
```

- [ ] **Step 3: Add GetByRequestID call to fetch raw body data**

After the existing `return &out, nil` line (before it), fetch request_logs data:

Change the return to:
```go
// Fetch raw anomaly data from request_logs (may not exist if save_raw_data was off).
if r.requestLogRepo != nil {
    logData, err := r.requestLogRepo.GetByRequestID(ctx, requestID)
    if err == nil && logData != nil {
        out.AnomalyTypes = logData.AnomalyTypes
        if logData.RequestBody != nil {
            out.RequestBody = json.RawMessage(logData.RequestBody)
        }
        if logData.UpstreamRequestBody != nil {
            out.UpstreamRequestBody = json.RawMessage(logData.UpstreamRequestBody)
        }
        if logData.UpstreamResponseBody != nil {
            out.UpstreamResponseBody = json.RawMessage(logData.UpstreamResponseBody)
        }
    }
}
return &out, nil
```

- [ ] **Step 4: Add requestLogRepo field to opsRepository**

In `backend/internal/repository/ops_repo.go` (or wherever `opsRepository` struct is defined), add:

```go
requestLogRepo service.RequestLogRepository
```

Update `NewOpsRepository` constructor to accept and assign this field.

- [ ] **Step 5: Build**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/repository/ops_repo_usage_inspect.go backend/internal/repository/
git commit -m "Feature: ops 详情查询新增用户/Key 身份字段和异常原始数据"
```

---

### Task 8: Wire injection — RequestLogRepository + AnomalyService

**Files:**
- Modify: `backend/cmd/server/wire_gen.go`

- [ ] **Step 1: Inspect current wire_gen.go structure**

```bash
grep -n "NewOpsRepository\|NewSettingRepository\|NewOpsService" /Users/ziji/personal/github/sub2api/backend/cmd/server/wire_gen.go | head -30
```

- [ ] **Step 2: Add RequestLogRepository and AnomalyService construction**

Find where `opsRepository` is created (look for `repository.NewOpsRepository`) and add before it:

```go
requestLogRepository := repository.NewRequestLogRepository(db)
anomalyService := service.NewAnomalyService(settingRepository, requestLogRepository)
```

Where `db` is the `*sql.DB` instance (found via `repository.ProvideSQLDB(client)`).

- [ ] **Step 3: Update NewOpsRepository call to pass requestLogRepo**

If the signature changed in Task 7, update:
```go
opsRepository := repository.NewOpsRepository(client, db, requestLogRepository)
```

- [ ] **Step 4: Update gateway handler constructors to pass anomalyService**

Find `NewCopilotGatewayHandler` call and add `anomalyService` as parameter (after updating the constructor signature in Task 9).

- [ ] **Step 5: Build**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add backend/cmd/server/wire_gen.go
git commit -m "Feature: wire_gen 注入 RequestLogRepository 和 AnomalyService"
```

---

### Task 9: Gateway handler integration — copilot + sora

**Files:**
- Modify: `backend/internal/handler/copilot_gateway_handler.go`
- Modify: `backend/internal/handler/sora_gateway_handler.go`

- [ ] **Step 1: Add anomalyService to CopilotGatewayHandler**

In `CopilotGatewayHandler` struct, add:
```go
anomalyService *service.AnomalyService
```

In `NewCopilotGatewayHandler` signature, add `anomalyService *service.AnomalyService`.

In the return statement:
```go
return &CopilotGatewayHandler{
    // ... existing fields ...
    anomalyService: anomalyService,
}
```

- [ ] **Step 2: Call WriteAnomalyLog after RecordUsage in Copilot handler**

Find the goroutine in the Copilot chat completions handler that calls `h.gatewayService.RecordUsage(...)`. After the `RecordUsage` call, add:

```go
if h.anomalyService != nil {
    inputTokens := 0
    outputTokens := 0
    if result.Usage != nil {
        inputTokens = result.Usage.InputTokens
        outputTokens = result.Usage.OutputTokens
    }
    durationMs := int64(result.Duration / time.Millisecond)

    // Get request body stored in context by setOpsRequestContext.
    var reqBody []byte
    if rb, exists := c.Get(string(opsRequestBodyKey)); exists {
        if b, ok := rb.([]byte); ok {
            reqBody = b
        }
    }

    userID := result.UserID    // these come from the RecordUsageInput
    apiKeyID := result.APIKeyID
    accountID := result.AccountID
    groupID := result.GroupID

    go h.anomalyService.WriteAnomalyLog(
        context.Background(),
        inputTokens,
        outputTokens,
        durationMs,
        200, // copilot success = 200
        &service.RequestLogInput{
            RequestID:   result.RequestID,
            UserID:      userID,
            APIKeyID:    apiKeyID,
            AccountID:   accountID,
            GroupID:     groupID,
            RequestBody: reqBody,
        },
    )
}
```

Note: `opsRequestBodyKey` is defined in `ops_error_logger.go` — use the same key. Check that package-level key type is accessible.

- [ ] **Step 3: Repeat for SoraGatewayHandler**

Apply same pattern to `backend/internal/handler/sora_gateway_handler.go`:
- Add `anomalyService *service.AnomalyService` field
- Update constructor
- Call `WriteAnomalyLog` after the RecordUsage goroutine

- [ ] **Step 4: Build**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Run existing gateway tests**

```bash
go test ./internal/handler/... -v -count=1 2>&1 | tail -30
```

Expected: all existing tests pass.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler/copilot_gateway_handler.go backend/internal/handler/sora_gateway_handler.go
git commit -m "Feature: 网关 handler 集成异常检测，异步写入 request_logs"
```

---

### Task 10: Admin ops handler — anomaly settings endpoints + filter param

**Files:**
- Modify: `backend/internal/handler/admin/ops_handler.go`

- [ ] **Step 1: Add anomalyService field to OpsHandler**

```go
type OpsHandler struct {
    opsService    *service.OpsService
    anomalyService *service.AnomalyService
}

func NewOpsHandler(opsService *service.OpsService, anomalyService *service.AnomalyService) *OpsHandler {
    return &OpsHandler{opsService: opsService, anomalyService: anomalyService}
}
```

Update `wire_gen.go` to pass `anomalyService` to `NewOpsHandler`.

- [ ] **Step 2: Add GetAnomalySettings endpoint**

```go
// GetAnomalySettings returns anomaly detection configuration.
// GET /api/v1/admin/ops/settings/anomaly
func (h *OpsHandler) GetAnomalySettings(c *gin.Context) {
    if h.anomalyService == nil {
        response.Error(c, http.StatusServiceUnavailable, "Anomaly service not available")
        return
    }
    settings := h.anomalyService.GetSettings(c.Request.Context())
    response.Success(c, settings)
}
```

- [ ] **Step 3: Add UpdateAnomalySettings endpoint**

```go
// UpdateAnomalySettings updates anomaly detection configuration.
// PUT /api/v1/admin/ops/settings/anomaly
func (h *OpsHandler) UpdateAnomalySettings(c *gin.Context) {
    if h.anomalyService == nil {
        response.Error(c, http.StatusServiceUnavailable, "Anomaly service not available")
        return
    }
    var req service.AnomalySettings
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "Invalid request body: "+err.Error())
        return
    }
    if req.SlowRequestThresholdMs <= 0 || req.TimeoutThresholdMs <= 0 {
        response.BadRequest(c, "Thresholds must be positive")
        return
    }
    if req.SlowRequestThresholdMs >= req.TimeoutThresholdMs {
        response.BadRequest(c, "slow_request_threshold_ms must be less than timeout_threshold_ms")
        return
    }
    if err := h.anomalyService.UpdateSettings(c.Request.Context(), &req); err != nil {
        response.ErrorFrom(c, err)
        return
    }
    response.Success(c, req)
}
```

- [ ] **Step 4: Find the ListRequestDetails handler and add anomaly_types param**

Find `GetRequestDetails` (or whatever the handler function for `GET /api/v1/admin/ops/requests` is called) and add after existing filter params:

```go
// anomaly_types: comma-separated list, e.g. "zero_token,slow_request"
if v := strings.TrimSpace(c.Query("anomaly_types")); v != "" {
    filter.AnomalyTypes = strings.Split(v, ",")
    filter.AnomalySettingsForFilter = h.anomalyService.GetSettings(c.Request.Context())
}
```

- [ ] **Step 5: Register routes**

Find the router setup file (likely `backend/internal/server/router.go` or similar) and add:

```go
opsGroup.GET("/settings/anomaly", opsHandler.GetAnomalySettings)
opsGroup.PUT("/settings/anomaly", opsHandler.UpdateAnomalySettings)
```

- [ ] **Step 6: Build**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

Expected: no errors.

- [ ] **Step 7: Test the endpoints manually**

```bash
# Start server, then:
curl -s http://localhost:8080/api/v1/admin/ops/settings/anomaly \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .

# Expected:
# {"slow_request_threshold_ms":20000,"timeout_threshold_ms":60000,"detect_zero_token":true,"save_raw_data":true}
```

- [ ] **Step 8: Commit**

```bash
git add backend/internal/handler/admin/ops_handler.go backend/internal/server/
git commit -m "Feature: 新增异常检测配置 GET/PUT 端点，列表支持 anomaly_types 筛选"
```

---

### Task 11: Frontend types and API functions — ops.ts

**Files:**
- Modify: `frontend/src/api/admin/ops.ts`

- [ ] **Step 1: Read existing ops.ts to understand current types**

```bash
head -100 /Users/ziji/personal/github/sub2api/frontend/src/api/admin/ops.ts
```

- [ ] **Step 2: Add new types and extend existing interfaces**

Add to `ops.ts`:

```typescript
// Anomaly types
export type AnomalyType = 'zero_token' | 'slow_request' | 'timeout' | 'error'

export interface AnomalySettings {
  slow_request_threshold_ms: number
  timeout_threshold_ms: number
  detect_zero_token: boolean
  save_raw_data: boolean
}

// Extend existing OpsRequestDetail interface — add these fields:
// user_name?: string
// api_key_label?: string
// group_name?: string
// account_name?: string
// anomaly_types?: AnomalyType[]
//
// (Add to the existing OpsRequestDetail interface definition)

// Extend existing OpsUsageInspectDetail interface — add these fields:
// user_name?: string
// api_key_label?: string
// anomaly_types?: AnomalyType[]
// request_body?: unknown
// upstream_request_body?: unknown
// upstream_response_body?: unknown
//
// (Add to the existing OpsUsageInspectDetail interface definition)

// API functions
export function getAnomalySettings(): Promise<AnomalySettings> {
  return request.get('/admin/ops/settings/anomaly')
}

export function updateAnomalySettings(settings: AnomalySettings): Promise<AnomalySettings> {
  return request.put('/admin/ops/settings/anomaly', settings)
}
```

Also update the existing `listRequestDetails` function to accept `anomaly_types` in its params:
```typescript
// In the params type / function signature, add:
anomaly_types?: string  // comma-separated, e.g. "zero_token,slow_request"
```

- [ ] **Step 3: Build frontend**

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npm run type-check
```

Expected: no type errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/api/admin/ops.ts
git commit -m "Feature: ops.ts 新增异常类型、配置 API 函数和扩展现有接口字段"
```

---

### Task 12: AnomalyBadge.vue component

**Files:**
- Create: `frontend/src/views/admin/ops/components/AnomalyBadge.vue`

- [ ] **Step 1: Create the component**

```vue
<!-- frontend/src/views/admin/ops/components/AnomalyBadge.vue -->
<template>
  <el-tag
    :type="tagType"
    size="small"
    class="anomaly-badge"
    disable-transitions
  >{{ label }}</el-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AnomalyType } from '@/api/admin/ops'

const props = defineProps<{
  type: AnomalyType
}>()

const labelMap: Record<AnomalyType, string> = {
  zero_token:   '零Token',
  slow_request: '慢请求',
  timeout:      '超时',
  error:        '错误',
}

const tagTypeMap: Record<AnomalyType, 'danger' | 'warning'> = {
  zero_token:   'danger',
  slow_request: 'warning',
  timeout:      'danger',
  error:        'danger',
}

const label   = computed(() => labelMap[props.type]   ?? props.type)
const tagType = computed(() => tagTypeMap[props.type] ?? 'info')
</script>

<style scoped>
.anomaly-badge {
  cursor: default;
}
</style>
```

- [ ] **Step 2: Verify it renders in isolation (spot check)**

Import and use in any test page. No formal test required — just ensure it compiles:

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npm run type-check
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/admin/ops/components/AnomalyBadge.vue
git commit -m "Feature: 新增 AnomalyBadge 组件显示异常类型标签"
```

---

### Task 13: AnomalyFilterChips.vue component

**Files:**
- Create: `frontend/src/views/admin/ops/components/AnomalyFilterChips.vue`

- [ ] **Step 1: Create the component**

```vue
<!-- frontend/src/views/admin/ops/components/AnomalyFilterChips.vue -->
<template>
  <div class="anomaly-filter-chips">
    <span class="label">异常筛选：</span>
    <el-check-tag
      v-for="opt in options"
      :key="opt.value"
      :checked="modelValue.includes(opt.value)"
      :class="['chip', `chip--${opt.severity}`]"
      @change="toggle(opt.value)"
    >
      {{ opt.label }}
    </el-check-tag>
    <el-button
      v-if="modelValue.length > 0"
      link
      size="small"
      @click="$emit('update:modelValue', [])"
    >
      清除
    </el-button>
  </div>
</template>

<script setup lang="ts">
import type { AnomalyType } from '@/api/admin/ops'

interface Option {
  value: AnomalyType
  label: string
  severity: 'critical' | 'warning'
}

const props = defineProps<{
  modelValue: AnomalyType[]
  options: Option[]
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', val: AnomalyType[]): void
}>()

function toggle(value: AnomalyType) {
  const current = [...props.modelValue]
  const idx = current.indexOf(value)
  if (idx === -1) {
    current.push(value)
  } else {
    current.splice(idx, 1)
  }
  emit('update:modelValue', current)
}
</script>

<style scoped>
.anomaly-filter-chips {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}
.chip--critical.is-checked {
  background-color: var(--el-color-danger-light-7);
  border-color: var(--el-color-danger);
  color: var(--el-color-danger);
}
.chip--warning.is-checked {
  background-color: var(--el-color-warning-light-7);
  border-color: var(--el-color-warning);
  color: var(--el-color-warning);
}
</style>
```

- [ ] **Step 2: Type check**

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npm run type-check
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/admin/ops/components/AnomalyFilterChips.vue
git commit -m "Feature: 新增 AnomalyFilterChips 异常类型多选筛选组件"
```

---

### Task 14: AnomalySettingsModal.vue component

**Files:**
- Create: `frontend/src/views/admin/ops/components/AnomalySettingsModal.vue`

- [ ] **Step 1: Create the component**

```vue
<!-- frontend/src/views/admin/ops/components/AnomalySettingsModal.vue -->
<template>
  <el-dialog
    v-model="visible"
    title="异常检测配置"
    width="480px"
    :close-on-click-modal="false"
  >
    <el-form :model="form" label-width="160px" :disabled="saving">
      <el-form-item label="慢请求警告阈值">
        <el-input-number
          v-model="form.slow_request_threshold_ms"
          :min="1000"
          :max="form.timeout_threshold_ms - 1"
          :step="1000"
        />
        <span class="unit">ms</span>
      </el-form-item>
      <el-form-item label="超时严重阈值">
        <el-input-number
          v-model="form.timeout_threshold_ms"
          :min="form.slow_request_threshold_ms + 1"
          :max="300000"
          :step="1000"
        />
        <span class="unit">ms</span>
      </el-form-item>
      <el-form-item label="检测零Token请求">
        <el-switch v-model="form.detect_zero_token" />
      </el-form-item>
      <el-form-item label="保存异常原始数据">
        <el-switch v-model="form.save_raw_data" />
        <div class="hint">开启后为异常请求存储完整请求/响应 body</div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { getAnomalySettings, updateAnomalySettings } from '@/api/admin/ops'
import type { AnomalySettings } from '@/api/admin/ops'

const visible = defineModel<boolean>({ default: false })

const form = reactive<AnomalySettings>({
  slow_request_threshold_ms: 20000,
  timeout_threshold_ms:      60000,
  detect_zero_token:         true,
  save_raw_data:             true,
})
const saving = ref(false)

watch(visible, async (v) => {
  if (v) {
    try {
      const settings = await getAnomalySettings()
      Object.assign(form, settings)
    } catch {
      // use defaults
    }
  }
})

async function save() {
  if (form.slow_request_threshold_ms >= form.timeout_threshold_ms) {
    ElMessage.error('慢请求阈值必须小于超时阈值')
    return
  }
  saving.value = true
  try {
    await updateAnomalySettings({ ...form })
    ElMessage.success('配置已保存')
    visible.value = false
  } catch (e: any) {
    ElMessage.error(e?.message ?? '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.unit {
  margin-left: 8px;
  color: var(--el-text-color-secondary);
}
.hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}
</style>
```

- [ ] **Step 2: Type check**

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npm run type-check
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/admin/ops/components/AnomalySettingsModal.vue
git commit -m "Feature: 新增 AnomalySettingsModal 异常阈值配置弹窗"
```

---

### Task 15: RawDataAccordion.vue component

**Files:**
- Create: `frontend/src/views/admin/ops/components/RawDataAccordion.vue`

- [ ] **Step 1: Create the component**

```vue
<!-- frontend/src/views/admin/ops/components/RawDataAccordion.vue -->
<template>
  <div class="raw-data-accordion">
    <div class="header" @click="open = !open">
      <el-icon class="arrow" :class="{ rotated: open }"><ArrowRight /></el-icon>
      <span class="title">{{ label }}</span>
      <el-tag v-if="!data && !error" size="small" type="info">无数据</el-tag>
      <el-tag v-if="error" size="small" type="danger">异常</el-tag>
    </div>
    <el-collapse-transition>
      <div v-show="open && (data || error)" class="body">
        <pre class="json-pre">{{ formatted }}</pre>
      </div>
    </el-collapse-transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { ArrowRight } from '@element-plus/icons-vue'

const props = defineProps<{
  label: string
  data?: unknown
  error?: boolean
}>()

const open = ref(false)

const formatted = computed(() => {
  if (!props.data) return ''
  try {
    return JSON.stringify(props.data, null, 2)
  } catch {
    return String(props.data)
  }
})
</script>

<style scoped>
.raw-data-accordion {
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
  margin-bottom: 8px;
  overflow: hidden;
}
.header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  cursor: pointer;
  background: var(--el-fill-color-lighter);
  user-select: none;
}
.header:hover {
  background: var(--el-fill-color-light);
}
.title {
  flex: 1;
  font-size: 13px;
  font-weight: 500;
}
.arrow {
  transition: transform 0.2s;
}
.arrow.rotated {
  transform: rotate(90deg);
}
.body {
  border-top: 1px solid var(--el-border-color-light);
}
.json-pre {
  margin: 0;
  padding: 12px;
  font-size: 12px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 400px;
  overflow-y: auto;
  background: var(--el-bg-color);
}
</style>
```

- [ ] **Step 2: Type check**

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npm run type-check
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/admin/ops/components/RawDataAccordion.vue
git commit -m "Feature: 新增 RawDataAccordion 手风琴式 JSON 数据展示组件"
```

---

### Task 16: OpsRequestInspectView.vue — filter bar + table columns + row colors

**Files:**
- Modify: `frontend/src/views/admin/ops/OpsRequestInspectView.vue`

- [ ] **Step 1: Read the current file**

```bash
wc -l /Users/ziji/personal/github/sub2api/frontend/src/views/admin/ops/OpsRequestInspectView.vue
head -60 /Users/ziji/personal/github/sub2api/frontend/src/views/admin/ops/OpsRequestInspectView.vue
```

- [ ] **Step 2: Add imports at top of script section**

```typescript
import AnomalyFilterChips from './components/AnomalyFilterChips.vue'
import AnomalyBadge from './components/AnomalyBadge.vue'
import AnomalySettingsModal from './components/AnomalySettingsModal.vue'
import type { AnomalyType, OpsRequestDetail } from '@/api/admin/ops'
```

- [ ] **Step 3: Add anomaly filter state**

```typescript
const selectedAnomalyTypes = ref<AnomalyType[]>([])
const showAnomalySettings  = ref(false)

const anomalyTypeOptions = [
  { value: 'zero_token'   as AnomalyType, label: '零Token',  severity: 'critical' as const },
  { value: 'slow_request' as AnomalyType, label: '慢请求',   severity: 'warning'  as const },
  { value: 'timeout'      as AnomalyType, label: '超时',     severity: 'critical' as const },
  { value: 'error'        as AnomalyType, label: '错误请求', severity: 'critical' as const },
]
```

- [ ] **Step 4: Add row class function**

```typescript
function getRowClass({ row }: { row: OpsRequestDetail }): string {
  const types = row.anomaly_types ?? []
  if (types.includes('zero_token') || types.includes('timeout') || types.includes('error')) {
    return 'row-critical'
  }
  if (types.includes('slow_request')) {
    return 'row-warning'
  }
  return ''
}
```

- [ ] **Step 5: Pass anomaly_types to the list API call**

In the fetch/loadData function, add to the request params:
```typescript
anomaly_types: selectedAnomalyTypes.value.length > 0
  ? selectedAnomalyTypes.value.join(',')
  : undefined,
```

Watch `selectedAnomalyTypes` to trigger reload:
```typescript
watch(selectedAnomalyTypes, () => loadData())
```

- [ ] **Step 6: Add AnomalyFilterChips to template filter bar**

After the existing platform/kind/search filters in the template:
```html
<AnomalyFilterChips
  v-model="selectedAnomalyTypes"
  :options="anomalyTypeOptions"
  style="margin-top: 8px;"
/>
<el-button
  :icon="Setting"
  circle
  size="small"
  title="配置异常阈值"
  style="margin-left: 8px;"
  @click="showAnomalySettings = true"
/>
<AnomalySettingsModal v-model="showAnomalySettings" />
```

- [ ] **Step 7: Add new table columns**

After the existing columns, add:
```html
<el-table-column label="用户" min-width="90" show-overflow-tooltip>
  <template #default="{ row }">
    <span v-if="row.user_name" class="text-user">{{ row.user_name }}</span>
    <span v-else class="text-muted">—</span>
  </template>
</el-table-column>
<el-table-column label="API Key" min-width="80" show-overflow-tooltip>
  <template #default="{ row }">
    <code v-if="row.api_key_label" class="text-mono">{{ row.api_key_label }}</code>
    <span v-else class="text-muted">—</span>
  </template>
</el-table-column>
<el-table-column label="分组" min-width="90" show-overflow-tooltip>
  <template #default="{ row }">
    <span v-if="row.group_name">{{ row.group_name }}</span>
    <span v-else class="text-muted">—</span>
  </template>
</el-table-column>
<el-table-column label="上游账户" min-width="100" show-overflow-tooltip>
  <template #default="{ row }">
    <span v-if="row.account_name" class="text-accent">{{ row.account_name }}</span>
    <span v-else class="text-muted">—</span>
  </template>
</el-table-column>
<el-table-column label="异常" min-width="140">
  <template #default="{ row }">
    <div v-if="row.anomaly_types?.length" class="anomaly-badges">
      <AnomalyBadge
        v-for="t in row.anomaly_types"
        :key="t"
        :type="t as AnomalyType"
      />
    </div>
  </template>
</el-table-column>
```

- [ ] **Step 8: Bind :row-class-name to el-table**

```html
<el-table
  ...
  :row-class-name="getRowClass"
  ...
>
```

- [ ] **Step 9: Add row color CSS**

```css
:deep(.row-critical) {
  background-color: var(--el-color-danger-light-9) !important;
  border-left: 3px solid var(--el-color-danger);
}
:deep(.row-warning) {
  background-color: var(--el-color-warning-light-9) !important;
  border-left: 3px solid var(--el-color-warning);
}
.anomaly-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.text-user   { color: var(--el-color-primary); }
.text-mono   { font-family: monospace; font-size: 12px; color: var(--el-text-color-secondary); }
.text-accent { color: var(--el-color-success-dark-2); }
.text-muted  { color: var(--el-text-color-placeholder); }
```

- [ ] **Step 10: Type check**

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npm run type-check
```

Expected: no errors.

- [ ] **Step 11: Commit**

```bash
git add frontend/src/views/admin/ops/OpsRequestInspectView.vue
git commit -m "Feature: 运维请求列表新增异常筛选、身份列和行颜色标注"
```

---

### Task 17: OpsRequestDetailPanel.vue — identity block + raw data block

**Files:**
- Modify: `frontend/src/views/admin/ops/components/OpsRequestDetailPanel.vue`

- [ ] **Step 1: Read current file**

```bash
wc -l /Users/ziji/personal/github/sub2api/frontend/src/views/admin/ops/components/OpsRequestDetailPanel.vue
head -80 /Users/ziji/personal/github/sub2api/frontend/src/views/admin/ops/components/OpsRequestDetailPanel.vue
```

- [ ] **Step 2: Add imports**

```typescript
import AnomalyBadge from './AnomalyBadge.vue'
import RawDataAccordion from './RawDataAccordion.vue'
```

- [ ] **Step 3: Add identity information section**

Find where `usageDetail` data is rendered (after request_id section). Insert after that block:

```html
<!-- Identity Information -->
<div v-if="usageDetail" class="detail-section">
  <div class="section-title">身份信息</div>
  <div class="detail-grid">
    <template v-if="usageDetail.user_name">
      <span class="detail-label">用户</span>
      <span class="detail-value text-user">{{ usageDetail.user_name }}</span>
    </template>
    <template v-if="usageDetail.api_key_label">
      <span class="detail-label">API Key</span>
      <code class="detail-value text-mono">{{ usageDetail.api_key_label }}</code>
    </template>
    <template v-if="usageDetail.group_name">
      <span class="detail-label">分组</span>
      <span class="detail-value">{{ usageDetail.group_name }}</span>
    </template>
    <template v-if="usageDetail.platform">
      <span class="detail-label">上游平台</span>
      <span class="detail-value">{{ usageDetail.platform }}</span>
    </template>
    <template v-if="usageDetail.account_name">
      <span class="detail-label">上游账户</span>
      <span class="detail-value text-accent">{{ usageDetail.account_name }}</span>
    </template>
  </div>
</div>
```

- [ ] **Step 4: Add anomaly badges to the detail panel header area**

Near the top of the detail panel (after request_id), display anomaly badges inline:

```html
<div v-if="usageDetail?.anomaly_types?.length" class="anomaly-row">
  <AnomalyBadge
    v-for="t in usageDetail.anomaly_types"
    :key="t"
    :type="t as AnomalyType"
  />
</div>
```

- [ ] **Step 5: Add raw data section at the bottom**

After all existing sections, add:

```html
<!-- Raw Data (only for anomalous requests) -->
<div
  v-if="usageDetail?.anomaly_types?.length"
  class="detail-section detail-section--danger"
>
  <div class="section-title">
    原始数据
    <el-tooltip content="仅异常请求保存原始数据，需在异常检测配置中开启「保存异常原始数据」" placement="top">
      <el-icon class="hint-icon"><QuestionFilled /></el-icon>
    </el-tooltip>
  </div>
  <RawDataAccordion
    label="用户原始请求"
    :data="usageDetail.request_body"
  />
  <RawDataAccordion
    label="上游请求"
    :data="usageDetail.upstream_request_body"
  />
  <RawDataAccordion
    label="上游响应"
    :data="usageDetail.upstream_response_body"
    :error="isTimeout(usageDetail)"
  />
</div>
```

Add helper function:
```typescript
import type { AnomalyType, OpsUsageInspectDetail } from '@/api/admin/ops'
import { QuestionFilled } from '@element-plus/icons-vue'

function isTimeout(detail: OpsUsageInspectDetail): boolean {
  return detail.anomaly_types?.includes('timeout') ?? false
}
```

- [ ] **Step 6: Add CSS for new sections**

```css
.detail-section--danger .section-title {
  color: var(--el-color-danger);
}
.detail-section--danger {
  border-left: 3px solid var(--el-color-danger-light-5);
  padding-left: 12px;
}
.anomaly-row {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.text-user   { color: var(--el-color-primary); }
.text-mono   { font-family: monospace; font-size: 12px; }
.text-accent { color: var(--el-color-success-dark-2); }
.hint-icon   { margin-left: 4px; vertical-align: middle; cursor: help; }
```

- [ ] **Step 7: Type check**

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npm run type-check
```

Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/views/admin/ops/components/OpsRequestDetailPanel.vue
git commit -m "Feature: 请求详情面板新增身份信息区块和异常原始数据展示"
```

---

### Task 18: End-to-End Validation

**Files:** None (validation only)

- [ ] **Step 1: Build entire backend**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

Expected: 0 errors.

- [ ] **Step 2: Run all backend tests**

```bash
go test -race ./... 2>&1 | tail -20
```

Expected: all PASS, no race conditions detected.

- [ ] **Step 3: Build frontend**

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npm run build
```

Expected: successful build, no TypeScript errors.

- [ ] **Step 4: Manual smoke test — anomaly detection**

Start the server and send a test request that should trigger zero_token:
```bash
# In a test environment: send a request through copilot gateway
# that returns 0 input + 0 output tokens.
# Check request_logs table:
psql $DATABASE_URL -c "SELECT request_id, anomaly_types FROM request_logs ORDER BY created_at DESC LIMIT 5;"
```

Expected: row with `anomaly_types = {zero_token}` (or other detected types).

- [ ] **Step 5: Manual smoke test — settings API**

```bash
curl -s -X GET http://localhost:8080/api/v1/admin/ops/settings/anomaly \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .

curl -s -X PUT http://localhost:8080/api/v1/admin/ops/settings/anomaly \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"slow_request_threshold_ms":15000,"timeout_threshold_ms":45000,"detect_zero_token":true,"save_raw_data":true}' | jq .

# GET again to verify cache cleared:
curl -s -X GET http://localhost:8080/api/v1/admin/ops/settings/anomaly \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

Expected: updated values returned.

- [ ] **Step 6: Manual smoke test — list anomaly filter**

```bash
curl -s "http://localhost:8080/api/v1/admin/ops/requests?anomaly_types=zero_token&start_time=2026-01-01T00:00:00Z" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.items[0]'
```

Expected: items with `anomaly_types` containing `"zero_token"`, and `user_name`/`api_key_label` fields populated.

- [ ] **Step 7: Manual smoke test — detail panel**

In the browser, open Admin > Ops > Request Inspection.
1. Verify anomaly filter chips appear above the table.
2. Verify table shows new identity columns (user, key, group, account).
3. Verify anomalous rows have red/yellow background tint.
4. Click an anomalous request to open detail panel.
5. Verify "身份信息" section shows user/key/group/account.
6. Verify "原始数据" section shows accordion for request/upstream data (if save_raw_data enabled).
7. Click "⚙" to open settings modal, change a threshold, save, verify success message.

- [ ] **Step 8: Final commit**

```bash
git add -A
git commit -m "Feature: 运维异常检测与请求排查增强完整实现"
```
