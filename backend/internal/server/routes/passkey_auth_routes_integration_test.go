//go:build integration

package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAuthPasskeyLoginBeginRouteReturnsBeginResult(t *testing.T) {
	ctx := context.Background()
	rdb := startAuthRouteRedis(t, ctx)
	stub := &passkeyRouteServiceStub{
		beginAuthenticationResult: &service.PasskeyAuthenticationBeginResult{
			FlowID:    "auth-flow-1",
			Countdown: 300,
		},
	}

	router, _ := newPasskeyAuthRoutesTestRouter(t, rdb, stub)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/login/begin", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.20:34567"

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	envelope := decodePasskeyRouteResponse[service.PasskeyAuthenticationBeginResult](t, recorder)
	require.Equal(t, 0, envelope.Code)
	require.Equal(t, "success", envelope.Message)
	require.Equal(t, "auth-flow-1", envelope.Data.FlowID)
	require.Equal(t, 300, envelope.Data.Countdown)
	require.Equal(t, 1, stub.beginAuthenticationCalls)
}

func TestAuthPasskeyLoginFinishRouteReturnsTokenPair(t *testing.T) {
	ctx := context.Background()
	rdb := startAuthRouteRedis(t, ctx)
	stub := &passkeyRouteServiceStub{
		finishAuthenticationUser: &service.User{
			ID:          7,
			Email:       "passkey@example.com",
			Username:    "passkey-user",
			Role:        service.RoleUser,
			Status:      service.StatusActive,
			Concurrency: 3,
		},
	}

	router, recentAuthCache := newPasskeyAuthRoutesTestRouter(t, rdb, stub)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/login/finish?flow_id=auth-flow-2", strings.NewReader(`{"id":"credential-login"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.21:34568"

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	envelope := decodePasskeyRouteResponse[handler.AuthResponse](t, recorder)
	require.Equal(t, 0, envelope.Code)
	require.Equal(t, "success", envelope.Message)
	require.NotEmpty(t, envelope.Data.AccessToken)
	require.True(t, strings.HasPrefix(envelope.Data.RefreshToken, "rt_"))
	require.Equal(t, "Bearer", envelope.Data.TokenType)
	require.NotNil(t, envelope.Data.User)
	require.Equal(t, int64(7), envelope.Data.User.ID)
	require.Equal(t, "passkey@example.com", envelope.Data.User.Email)
	require.Equal(t, "auth-flow-2", stub.lastFinishAuthenticationFlowID)
	require.JSONEq(t, `{"id":"credential-login"}`, stub.lastFinishAuthenticationBody)

	marker, err := recentAuthCache.GetRecentAuthMarker(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, marker)
	require.Equal(t, service.RecentAuthMethodPasskey, marker.Method)
}
