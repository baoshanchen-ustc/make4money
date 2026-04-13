package handler

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// handlerOpsRepoMock is a local test stub so handler tests do not depend on
// unexported mocks from the service package.
type handlerOpsRepoMock struct {
	InsertErrorLogFn              func(ctx context.Context, input *service.OpsInsertErrorLogInput) (int64, error)
	BatchInsertErrorLogsFn        func(ctx context.Context, inputs []*service.OpsInsertErrorLogInput) (int64, error)
	BatchInsertSystemLogsFn       func(ctx context.Context, inputs []*service.OpsInsertSystemLogInput) (int64, error)
	ListSystemLogsFn              func(ctx context.Context, filter *service.OpsSystemLogFilter) (*service.OpsSystemLogList, error)
	DeleteSystemLogsFn            func(ctx context.Context, filter *service.OpsSystemLogCleanupFilter) (int64, error)
	InsertSystemLogCleanupAuditFn func(ctx context.Context, input *service.OpsSystemLogCleanupAudit) error
}

func (m *handlerOpsRepoMock) InsertErrorLog(ctx context.Context, input *service.OpsInsertErrorLogInput) (int64, error) {
	if m.InsertErrorLogFn != nil {
		return m.InsertErrorLogFn(ctx, input)
	}
	return 0, nil
}

func (m *handlerOpsRepoMock) BatchInsertErrorLogs(ctx context.Context, inputs []*service.OpsInsertErrorLogInput) (int64, error) {
	if m.BatchInsertErrorLogsFn != nil {
		return m.BatchInsertErrorLogsFn(ctx, inputs)
	}
	return int64(len(inputs)), nil
}

func (m *handlerOpsRepoMock) ListErrorLogs(ctx context.Context, filter *service.OpsErrorLogFilter) (*service.OpsErrorLogList, error) {
	return &service.OpsErrorLogList{Errors: []*service.OpsErrorLog{}, Page: 1, PageSize: 20}, nil
}

func (m *handlerOpsRepoMock) GetErrorLogByID(ctx context.Context, id int64) (*service.OpsErrorLogDetail, error) {
	return &service.OpsErrorLogDetail{}, nil
}

func (m *handlerOpsRepoMock) ListRequestDetails(ctx context.Context, filter *service.OpsRequestDetailFilter) ([]*service.OpsRequestDetail, int64, error) {
	return []*service.OpsRequestDetail{}, 0, nil
}

func (m *handlerOpsRepoMock) BatchInsertSystemLogs(ctx context.Context, inputs []*service.OpsInsertSystemLogInput) (int64, error) {
	if m.BatchInsertSystemLogsFn != nil {
		return m.BatchInsertSystemLogsFn(ctx, inputs)
	}
	return int64(len(inputs)), nil
}

func (m *handlerOpsRepoMock) ListSystemLogs(ctx context.Context, filter *service.OpsSystemLogFilter) (*service.OpsSystemLogList, error) {
	if m.ListSystemLogsFn != nil {
		return m.ListSystemLogsFn(ctx, filter)
	}
	return &service.OpsSystemLogList{Logs: []*service.OpsSystemLog{}, Total: 0, Page: 1, PageSize: 50}, nil
}

func (m *handlerOpsRepoMock) DeleteSystemLogs(ctx context.Context, filter *service.OpsSystemLogCleanupFilter) (int64, error) {
	if m.DeleteSystemLogsFn != nil {
		return m.DeleteSystemLogsFn(ctx, filter)
	}
	return 0, nil
}

func (m *handlerOpsRepoMock) InsertSystemLogCleanupAudit(ctx context.Context, input *service.OpsSystemLogCleanupAudit) error {
	if m.InsertSystemLogCleanupAuditFn != nil {
		return m.InsertSystemLogCleanupAuditFn(ctx, input)
	}
	return nil
}

func (m *handlerOpsRepoMock) InsertRetryAttempt(ctx context.Context, input *service.OpsInsertRetryAttemptInput) (int64, error) {
	return 0, nil
}

func (m *handlerOpsRepoMock) UpdateRetryAttempt(ctx context.Context, input *service.OpsUpdateRetryAttemptInput) error {
	return nil
}

func (m *handlerOpsRepoMock) GetLatestRetryAttemptForError(ctx context.Context, sourceErrorID int64) (*service.OpsRetryAttempt, error) {
	return nil, nil
}

func (m *handlerOpsRepoMock) ListRetryAttemptsByErrorID(ctx context.Context, sourceErrorID int64, limit int) ([]*service.OpsRetryAttempt, error) {
	return []*service.OpsRetryAttempt{}, nil
}

func (m *handlerOpsRepoMock) UpdateErrorResolution(ctx context.Context, errorID int64, resolved bool, resolvedByUserID *int64, resolvedRetryID *int64, resolvedAt *time.Time) error {
	return nil
}

func (m *handlerOpsRepoMock) GetWindowStats(ctx context.Context, filter *service.OpsDashboardFilter) (*service.OpsWindowStats, error) {
	return &service.OpsWindowStats{}, nil
}

func (m *handlerOpsRepoMock) GetRealtimeTrafficSummary(ctx context.Context, filter *service.OpsDashboardFilter) (*service.OpsRealtimeTrafficSummary, error) {
	return &service.OpsRealtimeTrafficSummary{}, nil
}

func (m *handlerOpsRepoMock) GetDashboardOverview(ctx context.Context, filter *service.OpsDashboardFilter) (*service.OpsDashboardOverview, error) {
	return &service.OpsDashboardOverview{}, nil
}

func (m *handlerOpsRepoMock) GetThroughputTrend(ctx context.Context, filter *service.OpsDashboardFilter, bucketSeconds int) (*service.OpsThroughputTrendResponse, error) {
	return &service.OpsThroughputTrendResponse{}, nil
}

func (m *handlerOpsRepoMock) GetLatencyHistogram(ctx context.Context, filter *service.OpsDashboardFilter) (*service.OpsLatencyHistogramResponse, error) {
	return &service.OpsLatencyHistogramResponse{}, nil
}

func (m *handlerOpsRepoMock) GetErrorTrend(ctx context.Context, filter *service.OpsDashboardFilter, bucketSeconds int) (*service.OpsErrorTrendResponse, error) {
	return &service.OpsErrorTrendResponse{}, nil
}

func (m *handlerOpsRepoMock) GetErrorDistribution(ctx context.Context, filter *service.OpsDashboardFilter) (*service.OpsErrorDistributionResponse, error) {
	return &service.OpsErrorDistributionResponse{}, nil
}

func (m *handlerOpsRepoMock) GetOpenAITokenStats(ctx context.Context, filter *service.OpsOpenAITokenStatsFilter) (*service.OpsOpenAITokenStatsResponse, error) {
	return &service.OpsOpenAITokenStatsResponse{}, nil
}

func (m *handlerOpsRepoMock) InsertSystemMetrics(ctx context.Context, input *service.OpsInsertSystemMetricsInput) error {
	return nil
}

func (m *handlerOpsRepoMock) GetLatestSystemMetrics(ctx context.Context, windowMinutes int) (*service.OpsSystemMetricsSnapshot, error) {
	return &service.OpsSystemMetricsSnapshot{}, nil
}

func (m *handlerOpsRepoMock) UpsertJobHeartbeat(ctx context.Context, input *service.OpsUpsertJobHeartbeatInput) error {
	return nil
}

func (m *handlerOpsRepoMock) ListJobHeartbeats(ctx context.Context) ([]*service.OpsJobHeartbeat, error) {
	return []*service.OpsJobHeartbeat{}, nil
}

func (m *handlerOpsRepoMock) ListAlertRules(ctx context.Context) ([]*service.OpsAlertRule, error) {
	return []*service.OpsAlertRule{}, nil
}

func (m *handlerOpsRepoMock) CreateAlertRule(ctx context.Context, input *service.OpsAlertRule) (*service.OpsAlertRule, error) {
	return input, nil
}

func (m *handlerOpsRepoMock) UpdateAlertRule(ctx context.Context, input *service.OpsAlertRule) (*service.OpsAlertRule, error) {
	return input, nil
}

func (m *handlerOpsRepoMock) DeleteAlertRule(ctx context.Context, id int64) error {
	return nil
}

func (m *handlerOpsRepoMock) ListAlertEvents(ctx context.Context, filter *service.OpsAlertEventFilter) ([]*service.OpsAlertEvent, error) {
	return []*service.OpsAlertEvent{}, nil
}

func (m *handlerOpsRepoMock) GetAlertEventByID(ctx context.Context, eventID int64) (*service.OpsAlertEvent, error) {
	return &service.OpsAlertEvent{}, nil
}

func (m *handlerOpsRepoMock) GetActiveAlertEvent(ctx context.Context, ruleID int64) (*service.OpsAlertEvent, error) {
	return nil, nil
}

func (m *handlerOpsRepoMock) GetLatestAlertEvent(ctx context.Context, ruleID int64) (*service.OpsAlertEvent, error) {
	return nil, nil
}

func (m *handlerOpsRepoMock) CreateAlertEvent(ctx context.Context, event *service.OpsAlertEvent) (*service.OpsAlertEvent, error) {
	return event, nil
}

func (m *handlerOpsRepoMock) UpdateAlertEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	return nil
}

func (m *handlerOpsRepoMock) UpdateAlertEventEmailSent(ctx context.Context, eventID int64, emailSent bool) error {
	return nil
}

func (m *handlerOpsRepoMock) CreateAlertSilence(ctx context.Context, input *service.OpsAlertSilence) (*service.OpsAlertSilence, error) {
	return input, nil
}

func (m *handlerOpsRepoMock) IsAlertSilenced(ctx context.Context, ruleID int64, platform string, groupID *int64, region *string, now time.Time) (bool, error) {
	return false, nil
}

func (m *handlerOpsRepoMock) UpsertHourlyMetrics(ctx context.Context, startTime, endTime time.Time) error {
	return nil
}

func (m *handlerOpsRepoMock) UpsertDailyMetrics(ctx context.Context, startTime, endTime time.Time) error {
	return nil
}

func (m *handlerOpsRepoMock) GetLatestHourlyBucketStart(ctx context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func (m *handlerOpsRepoMock) GetLatestDailyBucketDate(ctx context.Context) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

var _ service.OpsRepository = (*handlerOpsRepoMock)(nil)
