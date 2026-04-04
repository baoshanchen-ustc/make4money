//go:build unit

package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectAPIKeyPlatform(t *testing.T) {
	tests := []struct {
		key      string
		platform string
		ok       bool
	}{
		{key: "sk-ant-api03-abc", platform: PlatformAnthropic, ok: true},
		{key: "AIzaSyD-example", platform: PlatformGemini, ok: true},
		{key: "sk-proj-123", platform: PlatformOpenAI, ok: true},
		{key: "unknown-key", platform: "", ok: false},
	}

	for _, tt := range tests {
		platform, ok := DetectAPIKeyPlatform(tt.key)
		require.Equal(t, tt.platform, platform)
		require.Equal(t, tt.ok, ok)
	}
}

func TestShouldDisableAPIKeyAuthFailure_OpenAI403RequiresExplicitSignals(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	require.True(t, ShouldDisableAPIKeyAuthFailure(account, http.StatusForbidden, []byte(`{"error":{"message":"organization has been disabled","code":"account_deactivated"}}`)))
	require.False(t, ShouldDisableAPIKeyAuthFailure(account, http.StatusForbidden, []byte(`{"error":{"message":"model not allowed for this project","code":"forbidden"}}`)))
}

func TestClassifyAPIKeyProbeResponse(t *testing.T) {
	openAIAccount := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	geminiAccount := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}
	anthropicAccount := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}

	valid, invalid, _ := ClassifyAPIKeyProbeResponse(openAIAccount, http.StatusOK, []byte(`{}`))
	require.True(t, valid)
	require.False(t, invalid)

	valid, invalid, _ = ClassifyAPIKeyProbeResponse(openAIAccount, http.StatusPaymentRequired, []byte(`{"error":{"message":"insufficient balance"}}`))
	require.False(t, valid)
	require.True(t, invalid)

	valid, invalid, _ = ClassifyAPIKeyProbeResponse(geminiAccount, http.StatusBadRequest, []byte(`{"error":{"message":"API key not valid. Please pass a valid API key.","status":"API_KEY_INVALID"}}`))
	require.False(t, valid)
	require.True(t, invalid)

	valid, invalid, _ = ClassifyAPIKeyProbeResponse(anthropicAccount, http.StatusMethodNotAllowed, []byte(`method not allowed`))
	require.True(t, valid)
	require.False(t, invalid)
}
