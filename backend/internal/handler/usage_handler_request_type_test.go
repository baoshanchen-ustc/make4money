package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userUsageRepoCapture struct {
	service.UsageLogRepository
	listFilters    usagestats.UsageLogFilters
	userModelStats []usagestats.ModelStat
	apiKeyStats    []usagestats.ModelStat
	lastUserID     int64
	lastAPIKeyID   int64
	userModelCalls int
	apiKeyCalls    int
}

func (s *userUsageRepoCapture) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters usagestats.UsageLogFilters) ([]service.UsageLog, *pagination.PaginationResult, error) {
	s.listFilters = filters
	return []service.UsageLog{}, &pagination.PaginationResult{
		Total:    0,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    0,
	}, nil
}

func (s *userUsageRepoCapture) GetUserModelStats(ctx context.Context, userID int64, startTime, endTime time.Time) ([]usagestats.ModelStat, error) {
	s.lastUserID = userID
	s.userModelCalls++
	return s.userModelStats, nil
}

func (s *userUsageRepoCapture) GetModelStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, requestType *int16, stream *bool, billingType *int8) ([]usagestats.ModelStat, error) {
	s.lastAPIKeyID = apiKeyID
	s.apiKeyCalls++
	return s.apiKeyStats, nil
}

type usageDashboardAPIKeyRepoStub struct {
	service.APIKeyRepository
	keys map[int64]*service.APIKey
	err  error
}

func (s *usageDashboardAPIKeyRepoStub) GetByID(ctx context.Context, id int64) (*service.APIKey, error) {
	if s.err != nil {
		return nil, s.err
	}
	key, ok := s.keys[id]
	if !ok {
		return nil, errors.New("missing")
	}
	return key, nil
}

func newUserUsageDashboardRouter(repo *userUsageRepoCapture, apiKeyRepo *usageDashboardAPIKeyRepoStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo, nil, nil, nil, nil, nil, &config.Config{})
	handler := NewUsageHandler(usageSvc, apiKeySvc)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage/dashboard/models", handler.DashboardModels)
	return router
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

func TestUserUsageDashboardModelsSupportsAPIKeyID(t *testing.T) {
	repo := &userUsageRepoCapture{
		apiKeyStats: []usagestats.ModelStat{{Model: "gpt-5.2", RequestCount: 3}},
	}
	apiKeyRepo := &usageDashboardAPIKeyRepoStub{
		keys: map[int64]*service.APIKey{
			7: {ID: 7, UserID: 42},
		},
	}
	router := newUserUsageDashboardRouter(repo, apiKeyRepo)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/models?api_key_id=7&start_date=2026-03-01&end_date=2026-03-02", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(7), repo.lastAPIKeyID)
	require.Equal(t, 1, repo.apiKeyCalls)
	require.Equal(t, 0, repo.userModelCalls)
}

func TestUserUsageDashboardModelsRejectsInvalidAPIKeyID(t *testing.T) {
	router := newUserUsageDashboardRouter(&userUsageRepoCapture{}, &usageDashboardAPIKeyRepoStub{keys: map[int64]*service.APIKey{}})

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/models?api_key_id=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserUsageDashboardModelsRejectsForeignAPIKey(t *testing.T) {
	repo := &userUsageRepoCapture{}
	apiKeyRepo := &usageDashboardAPIKeyRepoStub{
		keys: map[int64]*service.APIKey{
			9: {ID: 9, UserID: 99},
		},
	}
	router := newUserUsageDashboardRouter(repo, apiKeyRepo)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/models?api_key_id=9", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, 0, repo.apiKeyCalls)
}

func TestUserUsageDashboardModelsReturnsNotFoundForMissingAPIKey(t *testing.T) {
	router := newUserUsageDashboardRouter(&userUsageRepoCapture{}, &usageDashboardAPIKeyRepoStub{keys: map[int64]*service.APIKey{}})

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/models?api_key_id=77", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}
