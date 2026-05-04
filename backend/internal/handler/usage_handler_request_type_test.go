package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userUsageRepoCapture struct {
	service.UsageLogRepository
	listParams                pagination.PaginationParams
	listFilters               usagestats.UsageLogFilters
	statsUserID               int64
	statsStart                time.Time
	statsEnd                  time.Time
	dashboardTrendUserID      int64
	dashboardTrendStart       time.Time
	dashboardTrendEnd         time.Time
	dashboardTrendGranularity string
	dashboardModelsUserID     int64
	dashboardModelsStart      time.Time
	dashboardModelsEnd        time.Time
}

func (s *userUsageRepoCapture) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters usagestats.UsageLogFilters) ([]service.UsageLog, *pagination.PaginationResult, error) {
	s.listParams = params
	s.listFilters = filters
	return []service.UsageLog{}, &pagination.PaginationResult{
		Total:    0,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    0,
	}, nil
}

func (s *userUsageRepoCapture) GetUserStatsAggregated(ctx context.Context, userID int64, startTime, endTime time.Time) (*usagestats.UsageStats, error) {
	s.statsUserID = userID
	s.statsStart = startTime
	s.statsEnd = endTime
	return &usagestats.UsageStats{}, nil
}

func (s *userUsageRepoCapture) GetUserUsageTrendByUserID(ctx context.Context, userID int64, startTime, endTime time.Time, granularity string) ([]usagestats.TrendDataPoint, error) {
	s.dashboardTrendUserID = userID
	s.dashboardTrendStart = startTime
	s.dashboardTrendEnd = endTime
	s.dashboardTrendGranularity = granularity
	return []usagestats.TrendDataPoint{}, nil
}

func (s *userUsageRepoCapture) GetUserModelStats(ctx context.Context, userID int64, startTime, endTime time.Time) ([]usagestats.ModelStat, error) {
	s.dashboardModelsUserID = userID
	s.dashboardModelsStart = startTime
	s.dashboardModelsEnd = endTime
	return []usagestats.ModelStat{}, nil
}

func newUserUsageRequestTypeTestRouter(repo *userUsageRepoCapture) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage", handler.List)
	router.GET("/usage/stats", handler.Stats)
	router.GET("/usage/dashboard/trend", handler.DashboardTrend)
	router.GET("/usage/dashboard/models", handler.DashboardModels)
	return router
}

func TestUserUsageListRequestTypePriority(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?request_type=ws_v2&stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.listFilters.UserID)
	require.NotNil(t, repo.listFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeWSV2), *repo.listFilters.RequestType)
	require.Nil(t, repo.listFilters.Stream)
}

func TestUserUsageListInvalidRequestType(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?request_type=invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserUsageListInvalidStream(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?stream=invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserUsageListLast24HoursPeriod(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	before := time.Now().UTC()
	req := httptest.NewRequest(http.MethodGet, "/usage?period=last24hours&timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	after := time.Now().UTC()

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.listFilters.StartTime)
	require.NotNil(t, repo.listFilters.EndTime)
	require.WithinDuration(t, before.Add(-24*time.Hour), *repo.listFilters.StartTime, 2*time.Second)
	require.WithinDuration(t, after, *repo.listFilters.EndTime, 2*time.Second)
	require.Equal(t, 24*time.Hour, repo.listFilters.EndTime.Sub(*repo.listFilters.StartTime))
}

func TestUserUsageStatsLast24HoursPeriod(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	before := time.Now().UTC()
	req := httptest.NewRequest(http.MethodGet, "/usage/stats?period=last24hours&timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	after := time.Now().UTC()

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.statsUserID)
	require.WithinDuration(t, before.Add(-24*time.Hour), repo.statsStart, 2*time.Second)
	require.WithinDuration(t, after, repo.statsEnd, 2*time.Second)
	require.Equal(t, 24*time.Hour, repo.statsEnd.Sub(repo.statsStart))
}

func TestUserDashboardTrendLast24HoursResponseMetadata(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/trend?period=24h&timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.dashboardTrendUserID)
	require.Equal(t, "day", repo.dashboardTrendGranularity)
	require.Contains(t, rec.Body.String(), "\"period\":\"last24hours\"")
	require.Contains(t, rec.Body.String(), "\"start_time\"")
	require.Contains(t, rec.Body.String(), "\"end_time\"")
}

func TestUserDashboardModelsDateRangeResponseMetadata(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/models?start_date=2025-01-01&end_date=2025-01-02&timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.dashboardModelsUserID)
	require.Equal(t, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), repo.dashboardModelsStart)
	require.Equal(t, time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC), repo.dashboardModelsEnd)
	require.Contains(t, rec.Body.String(), "\"start_date\":\"2025-01-01\"")
	require.Contains(t, rec.Body.String(), "\"end_date\":\"2025-01-02\"")
	require.Contains(t, rec.Body.String(), "\"start_time\":\"2025-01-01T00:00:00Z\"")
	require.Contains(t, rec.Body.String(), "\"end_time\":\"2025-01-03T00:00:00Z\"")
}
