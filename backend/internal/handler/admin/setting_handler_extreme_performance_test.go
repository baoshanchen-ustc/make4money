package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

func newExtremePerformanceTestHandler(t *testing.T) (*SettingHandler, *extremePerformanceHandlerRepoStub) {
	t.Helper()
	repo := &extremePerformanceHandlerRepoStub{}
	svc := service.NewSettingService(repo, &config.Config{})
	return NewSettingHandler(svc, nil, nil, nil, nil), repo
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
