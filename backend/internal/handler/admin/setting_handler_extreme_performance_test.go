package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type extremePerformanceHandlerRepoStub struct {
	values map[string]string
}

func (s *extremePerformanceHandlerRepoStub) Get(ctx context.Context, key string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *extremePerformanceHandlerRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.values == nil {
		return "", service.ErrSettingNotFound
	}
	value, ok := s.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (s *extremePerformanceHandlerRepoStub) Set(ctx context.Context, key, value string) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	s.values[key] = value
	return nil
}

func (s *extremePerformanceHandlerRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *extremePerformanceHandlerRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *extremePerformanceHandlerRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *extremePerformanceHandlerRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

type extremePerformanceResponseEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type extremePerformanceHotPoolCacheStub struct {
	members map[string]map[int64]struct{}
	dead    map[int64]bool
}

func (s *extremePerformanceHotPoolCacheStub) ListMembers(ctx context.Context, platform string) ([]int64, error) {
	set := s.members[platform]
	out := make([]int64, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out, nil
}
func (s *extremePerformanceHotPoolCacheStub) AddMembers(ctx context.Context, platform string, ids []int64) error {
	if s.members == nil {
		s.members = map[string]map[int64]struct{}{}
	}
	if s.members[platform] == nil {
		s.members[platform] = map[int64]struct{}{}
	}
	for _, id := range ids {
		s.members[platform][id] = struct{}{}
	}
	return nil
}
func (s *extremePerformanceHotPoolCacheStub) ReplaceMembers(ctx context.Context, platform string, ids []int64) error {
	return nil
}
func (s *extremePerformanceHotPoolCacheStub) RemoveMember(ctx context.Context, platform string, accountID int64) error {
	if s.members != nil && s.members[platform] != nil {
		delete(s.members[platform], accountID)
	}
	return nil
}
func (s *extremePerformanceHotPoolCacheStub) CountMembers(ctx context.Context, platform string) (int, error) {
	return len(s.members[platform]), nil
}
func (s *extremePerformanceHotPoolCacheStub) GetMeta(ctx context.Context, platform string) (*service.HotPoolPlatformMeta, error) {
	return &service.HotPoolPlatformMeta{}, nil
}
func (s *extremePerformanceHotPoolCacheStub) SetMeta(ctx context.Context, platform string, meta *service.HotPoolPlatformMeta) error {
	return nil
}
func (s *extremePerformanceHotPoolCacheStub) MarkDeadAccount(ctx context.Context, accountID int64, ttl time.Duration) error {
	return nil
}
func (s *extremePerformanceHotPoolCacheStub) IsDeadAccount(ctx context.Context, accountID int64) (bool, error) {
	return s.dead[accountID], nil
}
func (s *extremePerformanceHotPoolCacheStub) TryAcquireRefillLock(ctx context.Context, platform string, ttl time.Duration) (bool, error) {
	return true, nil
}

type extremePerformanceHotPoolRepoStub struct {
	service.AccountRepository
	cold map[string][]service.Account
}

func (s *extremePerformanceHotPoolRepoStub) ListColdCandidatesByPlatform(ctx context.Context, platform string, limit int, excludeIDs []int64) ([]service.Account, error) {
	return s.cold[platform], nil
}
func (s *extremePerformanceHotPoolRepoStub) CountColdCandidatesByPlatform(ctx context.Context, platform string, excludeIDs []int64) (int, error) {
	return len(s.cold[platform]), nil
}

func newExtremePerformanceTestHandler(t *testing.T) (*SettingHandler, *extremePerformanceHandlerRepoStub) {
	t.Helper()
	repo := &extremePerformanceHandlerRepoStub{}
	svc := service.NewSettingService(repo, &config.Config{})
	return NewSettingHandler(svc, nil, nil, nil, nil, nil), repo
}

func TestSettingHandler_GetExtremePerformanceSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newExtremePerformanceTestHandler(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/extreme-performance", nil)

	h.GetExtremePerformanceSettings(c)

	require.Equal(t, http.StatusOK, w.Code)
	var envelope extremePerformanceResponseEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, 0, envelope.Code)

	var data map[string]any
	require.NoError(t, json.Unmarshal(envelope.Data, &data))
	require.Equal(t, false, data["enabled"])
	admin, ok := data["admin"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, admin["disable_auto_usage_fetch"])
}

func TestSettingHandler_UpdateExtremePerformanceSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, repo := newExtremePerformanceTestHandler(t)
	body := []byte(`{
		"enabled": true,
		"admin": {
			"disable_auto_usage_fetch": true,
			"disable_auto_today_stats_fetch": true,
			"allow_manual_usage_fetch": false
		},
		"pool": {
			"platform_limits": {
				"openai": 1200,
				"gemini": 240,
				"anthropic": 80
			},
			"refill_trigger_gap": 25,
			"refill_batch_size": 40,
			"selection_order": "imported_first"
		},
		"account_policy": {
			"delete_on_any_upstream_401": true,
			"cooldown_on_429_minutes": 5,
			"cooldown_on_5xx_minutes": 1,
			"remove_from_hot_pool_on_overload": true,
			"remove_from_hot_pool_on_temp_unschedulable": true
		}
	}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/extreme-performance", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateExtremePerformanceSettings(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, repo.values, service.SettingKeyExtremePerformanceSettings)

	var envelope extremePerformanceResponseEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, 0, envelope.Code)

	var data map[string]any
	require.NoError(t, json.Unmarshal(envelope.Data, &data))
	require.Equal(t, true, data["enabled"])
}

func TestSettingHandler_UpdateExtremePerformanceSettings_RejectsInvalidSelectionOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newExtremePerformanceTestHandler(t)
	body := []byte(`{
		"enabled": true,
		"admin": {
			"disable_auto_usage_fetch": true,
			"disable_auto_today_stats_fetch": true,
			"allow_manual_usage_fetch": true
		},
		"pool": {
			"platform_limits": {
				"openai": 1200,
				"gemini": 240,
				"anthropic": 80
			},
			"refill_trigger_gap": 25,
			"refill_batch_size": 40,
			"selection_order": "random"
		},
		"account_policy": {
			"delete_on_any_upstream_401": true,
			"cooldown_on_429_minutes": 5,
			"cooldown_on_5xx_minutes": 1,
			"remove_from_hot_pool_on_overload": true,
			"remove_from_hot_pool_on_temp_unschedulable": true
		}
	}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/extreme-performance", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateExtremePerformanceSettings(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var envelope response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Contains(t, envelope.Message, "selection_order")
}

func TestSettingHandler_GetExtremePerformanceStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &extremePerformanceHandlerRepoStub{}
	svc := service.NewSettingService(repo, &config.Config{})
	hotPoolCache := &extremePerformanceHotPoolCacheStub{
		members: map[string]map[int64]struct{}{
			service.PlatformOpenAI: {1: {}, 2: {}},
		},
	}
	hotPoolRepo := &extremePerformanceHotPoolRepoStub{
		cold: map[string][]service.Account{
			service.PlatformOpenAI: {{ID: 3, Platform: service.PlatformOpenAI}},
		},
	}
	hotPoolService := service.NewHotPoolService(hotPoolCache, hotPoolRepo)
	h := NewSettingHandler(svc, nil, nil, nil, nil, hotPoolService)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/extreme-performance/status", nil)

	h.GetExtremePerformanceStatus(c)

	require.Equal(t, http.StatusOK, w.Code)
	var envelope extremePerformanceResponseEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	var data map[string]any
	require.NoError(t, json.Unmarshal(envelope.Data, &data))
	platforms, ok := data["platforms"].([]any)
	require.True(t, ok)
	require.Len(t, platforms, 3)
}

func TestSettingHandler_TriggerExtremePerformanceActions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &extremePerformanceHandlerRepoStub{}
	svc := service.NewSettingService(repo, &config.Config{})
	require.NoError(t, svc.SetExtremePerformanceSettings(context.Background(), &service.ExtremePerformanceSettings{
		Enabled:       true,
		Admin:         service.DefaultExtremePerformanceSettings().Admin,
		Pool:          service.DefaultExtremePerformanceSettings().Pool,
		AccountPolicy: service.DefaultExtremePerformanceSettings().AccountPolicy,
	}))
	hotPoolCache := &extremePerformanceHotPoolCacheStub{}
	hotPoolRepo := &extremePerformanceHotPoolRepoStub{
		cold: map[string][]service.Account{
			service.PlatformOpenAI:    {{ID: 10, Platform: service.PlatformOpenAI, Status: service.StatusActive, Schedulable: true}},
			service.PlatformGemini:    {{ID: 20, Platform: service.PlatformGemini, Status: service.StatusActive, Schedulable: true}},
			service.PlatformAnthropic: {{ID: 30, Platform: service.PlatformAnthropic, Status: service.StatusActive, Schedulable: true}},
		},
	}
	h := NewSettingHandler(svc, nil, nil, nil, nil, service.NewHotPoolService(hotPoolCache, hotPoolRepo))

	for _, path := range []string{"/api/v1/admin/settings/extreme-performance/refill", "/api/v1/admin/settings/extreme-performance/rebuild"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, path, nil)
		if path == "/api/v1/admin/settings/extreme-performance/refill" {
			h.TriggerExtremePerformanceRefill(c)
		} else {
			h.TriggerExtremePerformanceRebuild(c)
		}
		require.Equal(t, http.StatusOK, w.Code)
	}
}
