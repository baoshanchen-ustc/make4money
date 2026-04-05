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

func TestClassifyAPIKeyStatusAction(t *testing.T) {
	openAI := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	anthropic := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	gemini := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}

	require.Equal(t, APIKeyStatusActionValid, ClassifyAPIKeyStatusAction(openAI, http.StatusOK, []byte(`{}`)))
	require.Equal(t, APIKeyStatusActionPermanentDisable, ClassifyAPIKeyStatusAction(openAI, http.StatusForbidden, []byte(`{"error":{"message":"organization has been disabled","code":"account_deactivated"}}`)))
	require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(openAI, http.StatusForbidden, []byte(`{"error":{"message":"model not allowed for this project","code":"forbidden"}}`)))
	require.Equal(t, APIKeyStatusActionIgnore, ClassifyAPIKeyStatusAction(anthropic, http.StatusMethodNotAllowed, []byte(`method not allowed`)))
	require.Equal(t, APIKeyStatusActionTemporaryCooldown, ClassifyAPIKeyStatusAction(gemini, http.StatusTooManyRequests, []byte(`{"error":{"message":"quota exceeded"}}`)))
	require.Equal(t, APIKeyStatusActionPermanentDisable, ClassifyAPIKeyStatusAction(gemini, http.StatusBadRequest, []byte(`{"error":{"message":"API key not valid. Please pass a valid API key.","status":"API_KEY_INVALID"}}`)))
}

func TestClassifyAPIKeyProbeResponse(t *testing.T) {
	openAIAccount := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	geminiAccount := &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}
	anthropicAccount := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}

	valid, invalid, cooldown, _ := ClassifyAPIKeyProbeResponse(openAIAccount, http.StatusOK, []byte(`{}`))
	require.True(t, valid)
	require.False(t, invalid)
	require.False(t, cooldown)

	valid, invalid, cooldown, _ = ClassifyAPIKeyProbeResponse(openAIAccount, http.StatusPaymentRequired, []byte(`{"error":{"message":"insufficient balance"}}`))
	require.False(t, valid)
	require.True(t, invalid)
	require.False(t, cooldown)

	valid, invalid, cooldown, _ = ClassifyAPIKeyProbeResponse(geminiAccount, http.StatusBadRequest, []byte(`{"error":{"message":"API key not valid. Please pass a valid API key.","status":"API_KEY_INVALID"}}`))
	require.False(t, valid)
	require.True(t, invalid)
	require.False(t, cooldown)

	// OpenAI 403 unrecognized → cooldown (not permanent disable, not valid)
	valid, invalid, cooldown, _ = ClassifyAPIKeyProbeResponse(openAIAccount, http.StatusForbidden, []byte(`{"error":{"message":"model not allowed for this project","code":"forbidden"}}`))
	require.False(t, valid)
	require.False(t, invalid)
	require.True(t, cooldown)

	// Unknown status code (405) → cooldown
	valid, invalid, cooldown, _ = ClassifyAPIKeyProbeResponse(anthropicAccount, http.StatusMethodNotAllowed, []byte(`method not allowed`))
	require.False(t, valid)
	require.False(t, invalid)
	require.True(t, cooldown)

	valid, invalid, cooldown, _ = ClassifyAPIKeyProbeResponse(openAIAccount, http.StatusTooManyRequests, []byte(`{"error":{"message":"rate limited"}}`))
	require.False(t, valid)
	require.False(t, invalid)
	require.True(t, cooldown)
}
