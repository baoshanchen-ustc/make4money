//go:build unit

package service

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTestService_RouteAntigravityTest_RejectsOAuthOnlyGoogleBaseURLForAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newSoraTestContext()

	svc := &AccountTestService{}
	account := &Account{
		ID:       301,
		Platform: PlatformAntigravity,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://cloudcode-pa.googleapis.com",
			"api_key":  "test-key",
		},
	}

	err := svc.routeAntigravityTest(ctx, account, "claude-sonnet-4-5", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "cloudcode-pa.googleapis.com")
	require.Contains(t, recorder.Body.String(), "仅适用于 OAuth 账号")
}
