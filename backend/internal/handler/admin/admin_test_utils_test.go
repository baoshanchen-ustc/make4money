package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func performRequest(t *testing.T, router http.Handler, method, target string, body []byte, contentType string) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Reader
	if body == nil {
		reqBody = bytes.NewReader([]byte{})
	} else {
		reqBody = bytes.NewReader(body)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, reqBody)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	router.ServeHTTP(rec, req)
	return rec
}

func performJSONRequest(t *testing.T, router http.Handler, method, target string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return performRequest(t, router, method, target, body, "application/json")
}

func decodeJSONResponse(t *testing.T, rec *httptest.ResponseRecorder, out any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), out))
}

type settingHandlerTestContext struct {
	router *gin.Engine
	repo   *inMemorySettingRepository
}

func newSettingHandlerTestContext(t *testing.T) *settingHandlerTestContext {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := newInMemorySettingRepository()
	settingSvc := service.NewSettingService(repo, &config.Config{})
	t.Cleanup(settingSvc.Stop)

	handler := NewSettingHandler(settingSvc, nil, nil, nil, nil, nil)
	router := gin.New()
	router.PUT("/api/v1/admin/settings", handler.UpdateSettings)
	router.GET("/api/v1/admin/settings", handler.GetSettings)

	return &settingHandlerTestContext{router: router, repo: repo}
}

func (c *settingHandlerTestContext) putSettings(t *testing.T, payload map[string]any) settingHandlerResponse {
	t.Helper()
	rec := performJSONRequest(t, c.router, http.MethodPut, "/api/v1/admin/settings", payload)
	require.Equal(t, http.StatusOK, rec.Code)

	return decodeSettingHandlerResponse(t, rec)
}

func (c *settingHandlerTestContext) getSettings(t *testing.T) settingHandlerResponse {
	t.Helper()
	rec := performRequest(t, c.router, http.MethodGet, "/api/v1/admin/settings", nil, "")
	require.Equal(t, http.StatusOK, rec.Code)

	return decodeSettingHandlerResponse(t, rec)
}

type settingHandlerResponse struct {
	Code int                        `json:"code"`
	Data settingHandlerSettingsData `json:"data"`
}

type settingHandlerSettingsData struct {
	BalanceLotExpiryDays             int    `json:"balance_lot_expiry_days"`
	BalanceExpiryReminderEnabled     bool   `json:"balance_expiry_reminder_enabled"`
	BalanceExpiryReminderAdvanceDays int    `json:"balance_expiry_reminder_advance_days"`
	UsageReportGlobalSchedule        string `json:"usage_report_global_schedule"`
	UsageReportTargetScope           string `json:"usage_report_target_scope"`
	AccountExpiryReminderAdvanceDays int    `json:"account_expiry_reminder_advance_days"`
}

func decodeSettingHandlerResponse(t *testing.T, rec *httptest.ResponseRecorder) settingHandlerResponse {
	t.Helper()
	var out settingHandlerResponse
	decodeJSONResponse(t, rec, &out)
	return out
}

type inMemorySettingRepository struct {
	mu     sync.RWMutex
	values map[string]string
}

func newInMemorySettingRepository() *inMemorySettingRepository {
	return &inMemorySettingRepository{values: make(map[string]string)}
}

func (m *inMemorySettingRepository) Get(ctx context.Context, key string) (*service.Setting, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.values[key]
	if !ok {
		return nil, nil
	}
	return &service.Setting{Key: key, Value: value, UpdatedAt: time.Now()}, nil
}

func (m *inMemorySettingRepository) GetValue(ctx context.Context, key string) (string, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.values[key], nil
}

func (m *inMemorySettingRepository) Set(ctx context.Context, key, value string) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = value
	return nil
}

func (m *inMemorySettingRepository) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := m.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (m *inMemorySettingRepository) SetMultiple(ctx context.Context, settings map[string]string) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, value := range settings {
		m.values[key] = value
	}
	return nil
}

func (m *inMemorySettingRepository) GetAll(ctx context.Context) (map[string]string, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]string, len(m.values))
	for key, value := range m.values {
		result[key] = value
	}
	return result, nil
}

func (m *inMemorySettingRepository) Delete(ctx context.Context, key string) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, key)
	return nil
}
