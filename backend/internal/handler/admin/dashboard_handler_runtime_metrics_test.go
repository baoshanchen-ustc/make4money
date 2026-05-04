//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDashboardHandler_GetRuntimeMetrics_JSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)

	h := NewDashboardHandler(nil, nil, runtimeMetricsBindingRepoStub{
		fanout: &service.AccountUserFanoutSnapshot{
			AccountCount:     2,
			ExternalUsersP95: 3,
			ExternalUsersMax: 3,
		},
	})
	h.GetRuntimeMetrics(c)

	require.Equal(t, http.StatusOK, w.Code)
	var payload struct {
		Code int `json:"code"`
		Data struct {
			LongTermBinding struct {
				ResolveHitTotal int64   `json:"resolve_hit_total"`
				HitRate         float64 `json:"hit_rate"`
			} `json:"long_term_binding"`
			AccountUserFanout struct {
				AccountCount     int     `json:"account_count"`
				ExternalUsersP95 float64 `json:"external_users_p95"`
				ExternalUsersMax int64   `json:"external_users_max"`
			} `json:"account_user_fanout"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.GreaterOrEqual(t, payload.Data.LongTermBinding.ResolveHitTotal, int64(0))
	require.GreaterOrEqual(t, payload.Data.LongTermBinding.HitRate, float64(0))
	require.Equal(t, 2, payload.Data.AccountUserFanout.AccountCount)
	require.Equal(t, float64(3), payload.Data.AccountUserFanout.ExternalUsersP95)
	require.Equal(t, int64(3), payload.Data.AccountUserFanout.ExternalUsersMax)
}

func TestDashboardHandler_GetRuntimeMetrics_Prometheus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics?format=prometheus", nil)

	h := NewDashboardHandler(nil, nil, runtimeMetricsBindingRepoStub{
		fanout: &service.AccountUserFanoutSnapshot{
			AccountCount:     2,
			ExternalUsersP95: 3,
			ExternalUsersMax: 3,
		},
	})
	h.GetRuntimeMetrics(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	body := w.Body.String()
	require.Contains(t, body, "sub2api_long_term_binding_resolve_hit_total")
	require.Contains(t, body, "sub2api_long_term_binding_write_rebind_total")
	require.Contains(t, body, "sub2api_long_term_binding_hit_rate")
	require.Contains(t, body, "sub2api_account_user_fanout_external_users_p95 3")
	require.False(t, strings.Contains(body, "<html"))
}

type runtimeMetricsBindingRepoStub struct {
	fanout *service.AccountUserFanoutSnapshot
}

func (s runtimeMetricsBindingRepoStub) GetBinding(context.Context, string, int64) (*service.UserAccountBinding, error) {
	return nil, nil
}

func (s runtimeMetricsBindingRepoStub) UpsertBinding(context.Context, string, int64, int64, time.Time) error {
	return nil
}

func (s runtimeMetricsBindingRepoStub) DeleteBinding(context.Context, string, int64) error {
	return nil
}

func (s runtimeMetricsBindingRepoStub) DeleteByAccountID(context.Context, int64) (int, error) {
	return 0, nil
}

func (s runtimeMetricsBindingRepoStub) DeleteExpired(context.Context) (int, error) {
	return 0, nil
}

func (s runtimeMetricsBindingRepoStub) SnapshotAccountUserFanout(context.Context) (*service.AccountUserFanoutSnapshot, error) {
	return s.fanout, nil
}
