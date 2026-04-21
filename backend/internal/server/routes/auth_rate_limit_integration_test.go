//go:build integration

package routes

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const authRouteRedisImageTag = "redis:8.4-alpine"

func TestAuthRegisterRateLimitThresholdHitReturns429(t *testing.T) {
	ctx := context.Background()
	rdb := startAuthRouteRedis(t, ctx)

	router := newAuthRoutesTestRouter(rdb)
	assertAuthRouteRateLimitThreshold(t, router, "/api/v1/auth/register", 5, http.StatusBadRequest)
}

func TestAuthPasskeyLoginBeginRateLimitThresholdHitReturns429(t *testing.T) {
	ctx := context.Background()
	rdb := startAuthRouteRedis(t, ctx)

	router := newAuthRoutesTestRouter(rdb)
	assertAuthRouteRateLimitThreshold(t, router, "/api/v1/auth/passkeys/login/begin", 20, http.StatusInternalServerError)
}

func TestAuthPasskeyLoginFinishRateLimitThresholdHitReturns429(t *testing.T) {
	ctx := context.Background()
	rdb := startAuthRouteRedis(t, ctx)

	router := newAuthRoutesTestRouter(rdb)
	assertAuthRouteRateLimitThreshold(t, router, "/api/v1/auth/passkeys/login/finish?flow_id=test-flow", 20, http.StatusInternalServerError)
}

func assertAuthRouteRateLimitThreshold(t *testing.T, router http.Handler, path string, allowed int, allowedStatus int) {
	t.Helper()

	for i := 1; i <= allowed+1; i++ {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "198.51.100.10:23456"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if i <= allowed {
			require.Equal(t, allowedStatus, w.Code, "request %d should reach handler before rate limit", i)
			continue
		}
		require.Equal(t, http.StatusTooManyRequests, w.Code, "request %d should hit rate limit", i)
		require.Contains(t, w.Body.String(), "rate limit exceeded")
	}
}

func startAuthRouteRedis(t *testing.T, ctx context.Context) *redis.Client {
	t.Helper()
	ensureAuthRouteDockerAvailable(t)

	redisContainer, err := tcredis.Run(ctx, authRouteRedisImageTag)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = redisContainer.Terminate(ctx)
	})

	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, redisPort.Int()),
		DB:   0,
	})
	require.NoError(t, rdb.Ping(ctx).Err())
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	return rdb
}

func ensureAuthRouteDockerAvailable(t *testing.T) {
	t.Helper()
	if authRouteDockerAvailable() {
		return
	}
	t.Skip("Docker 未启用，跳过认证限流集成测试")
}

func authRouteDockerAvailable() bool {
	if dockerHost := os.Getenv("DOCKER_HOST"); dockerHost != "" {
		return authRouteDockerHostReachable(dockerHost)
	}

	socketCandidates := []string{
		"/var/run/docker.sock",
		filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "docker.sock"),
		filepath.Join(authRouteUserHomeDir(), ".docker", "run", "docker.sock"),
		filepath.Join(authRouteUserHomeDir(), ".docker", "desktop", "docker.sock"),
		filepath.Join("/run/user", strconv.Itoa(os.Getuid()), "docker.sock"),
	}

	for _, socket := range socketCandidates {
		if socket == "" {
			continue
		}
		if _, err := os.Stat(socket); err == nil && authRouteDockerHostReachable("unix://"+socket) {
			return true
		}
	}
	return false
}

func authRouteDockerHostReachable(dockerHost string) bool {
	const timeout = 200 * time.Millisecond

	switch {
	case strings.HasPrefix(dockerHost, "unix://"):
		conn, err := net.DialTimeout("unix", strings.TrimPrefix(dockerHost, "unix://"), timeout)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	case strings.HasPrefix(dockerHost, "tcp://"):
		conn, err := net.DialTimeout("tcp", strings.TrimPrefix(dockerHost, "tcp://"), timeout)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	default:
		return false
	}
}

func authRouteUserHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
