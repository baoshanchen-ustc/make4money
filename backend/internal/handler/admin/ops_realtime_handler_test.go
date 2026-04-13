package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type opsRealtimeAccountRepoStub struct {
	accounts []service.Account
}

type opsRealtimeRepoStub struct {
	service.OpsRepository
	metrics    *service.OpsSystemMetricsSnapshot
	heartbeats []*service.OpsJobHeartbeat
}

func (s *opsRealtimeRepoStub) GetLatestSystemMetrics(context.Context, int) (*service.OpsSystemMetricsSnapshot, error) {
	return s.metrics, nil
}

func (s *opsRealtimeRepoStub) ListJobHeartbeats(context.Context) ([]*service.OpsJobHeartbeat, error) {
	return s.heartbeats, nil
}

func intPtrTest(v int) *int {
	return &v
}

func (s *opsRealtimeAccountRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode string) ([]service.Account, *pagination.PaginationResult, error) {
	start := (params.Page - 1) * params.PageSize
	if start < 0 {
		start = 0
	}
	if start >= len(s.accounts) {
		return []service.Account{}, &pagination.PaginationResult{Total: int64(len(s.accounts)), Page: params.Page, PageSize: params.PageSize}, nil
	}
	end := start + params.PageSize
	if end > len(s.accounts) {
		end = len(s.accounts)
	}
	out := make([]service.Account, 0, end-start)
	for _, acc := range s.accounts[start:end] {
		if platform != "" && acc.Platform != platform {
			continue
		}
		out = append(out, acc)
	}
	return out, &pagination.PaginationResult{Total: int64(len(s.accounts)), Page: params.Page, PageSize: params.PageSize}, nil
}

func (s *opsRealtimeAccountRepoStub) Create(context.Context, *service.Account) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) GetByID(context.Context, int64) (*service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) GetByIDs(context.Context, []int64) ([]*service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ExistsByID(context.Context, int64) (bool, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) GetByCRSAccountID(context.Context, string) (*service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) FindByExtraField(context.Context, string, any) ([]service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ListCRSAccountIDs(context.Context) (map[string]int64, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) Update(context.Context, *service.Account) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) Delete(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) List(context.Context, pagination.PaginationParams) ([]service.Account, *pagination.PaginationResult, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ListByGroup(context.Context, int64) ([]service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ListActive(context.Context) ([]service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ListByPlatform(context.Context, string) ([]service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) UpdateLastUsed(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) BatchUpdateLastUsed(context.Context, map[int64]time.Time) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) SetError(context.Context, int64, string) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ClearError(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) SetSchedulable(context.Context, int64, bool) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) AutoPauseExpiredAccounts(context.Context, time.Time) (int64, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) BindGroups(context.Context, int64, []int64) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ListSchedulable(context.Context) ([]service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ListSchedulableByPlatform(context.Context, string) ([]service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ListSchedulableByPlatforms(context.Context, []string) ([]service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ListSchedulableUngroupedByPlatform(context.Context, string) ([]service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]service.Account, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) SetRateLimited(context.Context, int64, time.Time) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) SetModelRateLimit(context.Context, int64, string, time.Time) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) SetOverloaded(context.Context, int64, time.Time) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ClearTempUnschedulable(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ClearRateLimit(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ClearAntigravityQuotaScopes(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ClearModelRateLimits(context.Context, int64) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) UpdateSessionWindow(context.Context, int64, *time.Time, *time.Time, string) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) UpdateExtra(context.Context, int64, map[string]any) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) BulkUpdate(context.Context, []int64, service.AccountBulkUpdate) (int64, error) {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) IncrementQuotaUsed(context.Context, int64, float64) error {
	panic("unexpected call")
}
func (s *opsRealtimeAccountRepoStub) ResetQuotaUsed(context.Context, int64) error {
	panic("unexpected call")
}

func TestOpsHandler_GetAccountAvailability_IncludesRealtimeSummaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	lastRun := now.Add(-2 * time.Minute)
	lastSuccess := now.Add(-90 * time.Second)
	lastDuration := int64(345)
	service.TrackStickyConsistencyHit()
	service.TrackStickyConsistencyGhost("transport_mismatch")

	opsSvc := service.NewOpsService(&opsRealtimeRepoStub{
		metrics: &service.OpsSystemMetricsSnapshot{
			DBConnActive:   intPtrTest(8),
			DBConnIdle:     intPtrTest(4),
			RedisConnTotal: intPtrTest(24),
			RedisConnIdle:  intPtrTest(6),
		},
		heartbeats: []*service.OpsJobHeartbeat{
			{
				JobName:        "ops_cleanup",
				LastRunAt:      &lastRun,
				LastSuccessAt:  &lastSuccess,
				LastDurationMs: &lastDuration,
				UpdatedAt:      now,
			},
		},
	}, nil, &config.Config{
		Ops: config.OpsConfig{
			Enabled: true,
			Cleanup: config.OpsCleanupConfig{
				Enabled:                    true,
				ErrorLogRetentionDays:      30,
				SystemLogMaxRows:           500000,
				ErrorLogMaxRows:            100000,
				MaxRowsEnabled:             false,
				MaxRowsDryRun:              true,
				MinuteMetricsRetentionDays: 7,
				HourlyMetricsRetentionDays: 30,
			},
		},
		Database: config.DatabaseConfig{MaxOpenConns: 64, MaxIdleConns: 16},
		Redis:    config.RedisConfig{PoolSize: 128, MinIdleConns: 24},
		DashboardAgg: config.DashboardAggregationConfig{
			Retention: config.DashboardAggregationRetentionConfig{
				UsageLogsDays:    30,
				UsageLogsMaxRows: 200000,
			},
		},
		UsageCleanup: config.UsageCleanupConfig{
			Enabled:               true,
			MaxRangeDays:          7,
			BatchSize:             500,
			WorkerIntervalSeconds: 60,
			TaskTimeoutSeconds:    120,
		},
	}, &opsRealtimeAccountRepoStub{
		accounts: []service.Account{
			{
				ID:          101,
				Name:        "oauth-bad",
				Platform:    service.PlatformOpenAI,
				Status:      service.StatusError,
				Schedulable: true,
				Extra: map[string]any{
					"token_refresh_failure_reason":  "token_revoked",
					"token_refresh_failure_class":   "permanent",
					"token_refresh_failed_at":       "2026-04-10T12:00:00Z",
					"account_auth_failure_reason":   "token_revoked",
					"account_auth_failure_class":    "permanent",
					"account_auth_failure_source":   "token_refresh",
					"account_auth_state":            "reauthorize_required",
					"account_auth_state_changed_at": "2026-04-10T12:00:00Z",
				},
			},
		},
	}, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/account-availability", handler.GetAccountAvailability)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/account-availability", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))

	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, true, payload["enabled"])

	tokenSummary, ok := payload["token_refresh_failures"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(1), tokenSummary["total_accounts"])
	require.Equal(t, float64(1), tokenSummary["permanent_count"])
	authSummary, ok := payload["auth_failures"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(1), authSummary["total_accounts"])
	require.Equal(t, float64(1), authSummary["permanent_count"])
	require.Equal(t, float64(1), authSummary["dispatch_suppressed"])
	require.Equal(t, float64(1), authSummary["manual_review_required"])
	authGuidance, ok := payload["auth_failure_guidance"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "warning", authGuidance["severity"])
	require.Equal(t, float64(1), authGuidance["dispatch_suppressed"])
	require.Equal(t, float64(1), authGuidance["manual_review_required"])
	require.Equal(t, "Avoid widening failover pressure while dispatch-suppressed accounts remain elevated.", authGuidance["dispatch_suppressed_action"])
	require.Equal(t, "Manual-review-required accounts should stay isolated until an admin reauthorizes or retires them.", authGuidance["manual_review_action"])
	require.Equal(t, "Keep background-refresh-eligible accounts out of scheduling until the refresh succeeds or their state escalates.", authGuidance["background_recovery_action"])
	require.Contains(t, authGuidance, "actions")

	resourceBudget, ok := payload["resource_budget_summary"].(map[string]any)
	require.True(t, ok)
	database, ok := resourceBudget["database"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(64), database["max_open_conns"])
	redisSummary, ok := resourceBudget["redis"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(128), redisSummary["pool_size"])

	storageGovernance, ok := payload["storage_governance"].(map[string]any)
	require.True(t, ok)
	opsCleanup, ok := storageGovernance["ops_cleanup"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, opsCleanup["enabled"])
	heartbeat, ok := opsCleanup["heartbeat"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(345), heartbeat["last_duration_ms"])
	dryRun, ok := storageGovernance["dry_run"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, dryRun, "usage_cleanup")

	stickyCleanup, ok := payload["sticky_session_cleanup"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, stickyCleanup, "cleanup_total")
	require.Contains(t, stickyCleanup, "compare_delete_miss_total")
	stickyConsistency, ok := payload["sticky_consistency"].(map[string]any)
	require.True(t, ok)
	totalHits, ok := stickyConsistency["total_hits"].(float64)
	require.True(t, ok)
	require.GreaterOrEqual(t, totalHits, float64(1))
	ghostInvalidations, ok := stickyConsistency["ghost_invalidations"].(float64)
	require.True(t, ok)
	require.GreaterOrEqual(t, ghostInvalidations, float64(1))
	failureSplit, ok := payload["failure_split_summary"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(1), failureSplit["account_or_auth"])
	require.Equal(t, "account_or_auth", failureSplit["likely_primary"])
	require.Contains(t, failureSplit["suggestion"], "account/auth")
	_, hasErrorFamily := payload["error_family_summary"]
	require.False(t, hasErrorFamily)
	_, hasUsageIntegrity := payload["usage_integrity"]
	require.False(t, hasUsageIntegrity)
}

func TestOpsHandler_GetAccountAvailability_FilteredRequestOmitsGlobalFailureSummaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	opsSvc := service.NewOpsService(&opsRealtimeRepoStub{
		metrics: &service.OpsSystemMetricsSnapshot{},
	}, nil, &config.Config{
		Ops: config.OpsConfig{Enabled: true},
	}, &opsRealtimeAccountRepoStub{
		accounts: []service.Account{
			{
				ID:          201,
				Name:        "openai-acc",
				Platform:    service.PlatformOpenAI,
				Status:      service.StatusError,
				Schedulable: true,
				Extra: map[string]any{
					"account_auth_failure_reason": "token_revoked",
					"account_auth_failure_class":  "permanent",
					"account_auth_state":          "reauthorize_required",
				},
			},
		},
	}, nil, nil, nil, nil, nil, nil, nil)
	handler := NewOpsHandler(opsSvc)
	router := gin.New()
	router.GET("/admin/ops/account-availability", handler.GetAccountAvailability)

	req := httptest.NewRequest(http.MethodGet, "/admin/ops/account-availability?platform=openai", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))

	raw, err := json.Marshal(envelope.Data)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	_, hasErrorFamily := payload["error_family_summary"]
	require.False(t, hasErrorFamily)
	_, hasFailureSplit := payload["failure_split_summary"]
	require.False(t, hasFailureSplit)
}
