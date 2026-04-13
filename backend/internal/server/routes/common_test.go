package routes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterCommonRoutes_HealthReady(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	now := time.Date(2026, 4, 12, 18, 4, 0, 0, time.UTC)
	RegisterCommonRoutes(r, CommonRouteOptions{
		DBProbe:    func(_ context.Context) error { return nil },
		RedisProbe: func(_ context.Context) error { return nil },
		DBSnapshot: func() repository.DBPoolSnapshot {
			return repository.DBPoolSnapshot{
				OpenConnections:    12,
				InUse:              4,
				Idle:               8,
				MaxOpenConnections: 40,
				WaitCount:          2,
				UsagePercent:       float64Ptr(30),
				InUsePercent:       float64Ptr(10),
			}
		},
		RedisSnapshot: func() repository.RedisPoolSnapshot {
			return repository.RedisPoolSnapshot{
				TotalConns:                16,
				IdleConns:                 6,
				Stalls:                    1,
				Timeouts:                  0,
				UsagePercent:              float64Ptr(20),
				HitRatePercent:            float64Ptr(98.5),
				ConnectionPressurePercent: float64Ptr(20),
			}
		},
		SchedulerSnapshot: func() service.SchedulerOutboxRuntimeMetrics {
			return service.SchedulerOutboxRuntimeMetrics{}
		},
		Now: func() time.Time { return now },
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "ok", payload["status"])
	require.Equal(t, true, payload["ready"])
	require.Equal(t, now.Format(time.RFC3339), payload["checked_at"])

	checks, ok := payload["checks"].(map[string]any)
	require.True(t, ok)
	db, ok := checks["db"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ok", db["status"])
	require.Equal(t, true, db["ready"])
	require.Equal(t, "db probe healthy", db["note"])

	redis, ok := checks["redis"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ok", redis["status"])
	require.Equal(t, true, redis["ready"])

	scheduler, ok := checks["scheduler_outbox"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ok", scheduler["status"])
	require.Equal(t, "scheduler outbox healthy", scheduler["note"])
}

func TestRegisterCommonRoutes_HealthDegraded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterCommonRoutes(r, CommonRouteOptions{
		DBProbe:    func(_ context.Context) error { return errors.New("db timeout") },
		RedisProbe: func(_ context.Context) error { return nil },
		DBSnapshot: func() repository.DBPoolSnapshot {
			return repository.DBPoolSnapshot{
				OpenConnections:    90,
				InUse:              88,
				MaxOpenConnections: 100,
				UsagePercent:       float64Ptr(90),
				InUsePercent:       float64Ptr(88),
			}
		},
		RedisSnapshot: func() repository.RedisPoolSnapshot {
			return repository.RedisPoolSnapshot{
				TotalConns:                95,
				IdleConns:                 5,
				UsagePercent:              float64Ptr(95),
				ConnectionPressurePercent: float64Ptr(95),
			}
		},
		SchedulerSnapshot: func() service.SchedulerOutboxRuntimeMetrics {
			return service.SchedulerOutboxRuntimeMetrics{
				LagSeconds:               420,
				BacklogRows:              1200,
				CheckpointFallbackStreak: 3,
			}
		},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "degraded", payload["status"])
	require.Equal(t, false, payload["ready"])

	checks, ok := payload["checks"].(map[string]any)
	require.True(t, ok)
	db, ok := checks["db"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "critical", db["status"])
	require.Equal(t, false, db["ready"])
	require.Equal(t, "db timeout", db["note"])

	redis, ok := checks["redis"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "warning", redis["status"])
	require.Equal(t, true, redis["ready"])

	scheduler, ok := checks["scheduler_outbox"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "critical", scheduler["status"])
	require.Contains(t, scheduler["note"], "lag=420s")
	require.Contains(t, scheduler["note"], "backlog=1200")
}

func TestRegisterCommonRoutes_MetricsExposed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterCommonRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, prometheusContentType, w.Header().Get("Content-Type"))
	require.Contains(t, w.Body.String(), "# HELP sub2api_up")
	require.Contains(t, w.Body.String(), "sub2api_db_pool_open_connections")
	require.Contains(t, w.Body.String(), "sub2api_scheduler_outbox_lag_seconds")
	require.NotContains(t, w.Body.String(), "<!doctype html>")
}

func TestRegisterCommonRoutes_ProbeAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterCommonRoutes(r)

	for _, path := range []string{"/readyz", "/api/health", "/api/readyz"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			require.Contains(t, w.Body.String(), `"status":"`)
			require.Contains(t, w.Body.String(), `"ready":`)
		})
	}

	t.Run("/livez", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/livez", nil)
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), `"status":"ok"`)
		require.Contains(t, w.Body.String(), `"live":true`)
	})
}

func TestRenderRuntimeMetrics_IsPrometheusText(t *testing.T) {
	body := renderRuntimeMetrics()

	require.True(t, strings.HasPrefix(body, "# HELP sub2api_up"))
	require.Contains(t, body, "# TYPE sub2api_usage_cleanup_started_total counter")
	require.Contains(t, body, "sub2api_ops_cleanup_max_rows_hit ")
	require.Contains(t, body, "sub2api_scheduler_outbox_coalesced_event_saved_total ")
	require.Contains(t, body, "sub2api_scheduler_outbox_rebuild_cooldown_skip_total ")
}

func float64Ptr(v float64) *float64 {
	return &v
}
