package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseCPAImportCandidate_Claude(t *testing.T) {
	raw := `{
		"type":"claude",
		"email":"claude@example.com",
		"access_token":"claude-access",
		"refresh_token":"claude-refresh",
		"expired":"2026-03-23T10:00:00Z",
		"org_uuid":"org-123",
		"account_uuid":"acc-456"
	}`

	candidate, err := parseCPAImportCandidate("claude-auth.json", raw)
	require.NoError(t, err)

	require.Equal(t, PlatformAnthropic, candidate.platform)
	require.Equal(t, AccountTypeOAuth, candidate.accountType)
	require.Equal(t, "claude-access", candidate.credentials["access_token"])
	require.Equal(t, "claude-refresh", candidate.credentials["refresh_token"])
	require.Equal(t, "2026-03-23T10:00:00Z", candidate.credentials["expires_at"])
	require.Equal(t, "org-123", candidate.extra["org_uuid"])
	require.Equal(t, "acc-456", candidate.extra["account_uuid"])
	require.Equal(t, "claude:claude@example.com", candidate.sourceKey)
}

func TestParseCPAImportCandidate_Codex(t *testing.T) {
	raw := `{
		"type":"codex",
		"email":"openai@example.com",
		"account_id":"acct_123",
		"access_token":"openai-access",
		"refresh_token":"openai-refresh",
		"id_token":"header.payload.signature",
		"expired":"2026-03-23T10:00:00Z"
	}`

	candidate, err := parseCPAImportCandidate("codex-auth.json", raw)
	require.NoError(t, err)

	require.Equal(t, PlatformOpenAI, candidate.platform)
	require.Equal(t, AccountTypeOAuth, candidate.accountType)
	require.Equal(t, "openai-access", candidate.credentials["access_token"])
	require.Equal(t, "openai-refresh", candidate.credentials["refresh_token"])
	require.Equal(t, "acct_123", candidate.credentials["chatgpt_account_id"])
	require.Equal(t, "codex:openai@example.com:acct_123", candidate.sourceKey)
	require.Contains(t, candidate.matchKeys, "codex:openai@example.com:acct_123")
	require.Contains(t, candidate.matchKeys, "codex:acct_123")
	require.Contains(t, candidate.matchKeys, "codex:openai@example.com")
}

func TestParseCPAImportCandidate_Gemini(t *testing.T) {
	raw := `{
		"type":"gemini",
		"email":"gemini@example.com",
		"project_id":"proj-a, proj-b",
		"token":{
			"access_token":"gemini-access",
			"refresh_token":"gemini-refresh",
			"token_type":"Bearer",
			"scope":"scope-a scope-b",
			"expiry":"2026-03-23T10:00:00Z"
		}
	}`

	candidate, err := parseCPAImportCandidate("gemini-auth.json", raw)
	require.NoError(t, err)

	require.Equal(t, PlatformGemini, candidate.platform)
	require.Equal(t, AccountTypeOAuth, candidate.accountType)
	require.Equal(t, "gemini-access", candidate.credentials["access_token"])
	require.Equal(t, "gemini-refresh", candidate.credentials["refresh_token"])
	require.Equal(t, "code_assist", candidate.credentials["oauth_type"])
	require.Equal(t, "proj-a, proj-b", candidate.credentials["project_id"])
	require.Equal(t, "gemini:gemini@example.com:proj-a,proj-b", candidate.sourceKey)
}

func TestParseCPAImportCandidate_AntigravityDerivesExpiry(t *testing.T) {
	now := time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC)
	raw := fmt.Sprintf(`{
		"type":"antigravity",
		"email":"ag@example.com",
		"project_id":"project-123",
		"access_token":"ag-access",
		"refresh_token":"ag-refresh",
		"expires_in":3600,
		"timestamp":%d
	}`, now.UnixMilli())

	candidate, err := parseCPAImportCandidate("antigravity-auth.json", raw)
	require.NoError(t, err)

	require.Equal(t, PlatformAntigravity, candidate.platform)
	require.Equal(t, AccountTypeOAuth, candidate.accountType)
	require.Equal(t, "ag-access", candidate.credentials["access_token"])
	require.Equal(t, "ag-refresh", candidate.credentials["refresh_token"])
	require.Equal(t, now.Add(time.Hour).Format(time.RFC3339), candidate.credentials["expires_at"])
	require.Equal(t, "antigravity:ag@example.com:project-123", candidate.sourceKey)
}

func TestFilterImportableCPARemoteFiles(t *testing.T) {
	files := []cpaRemoteAuthFile{
		{Name: "codex-active.json", Provider: "codex", Status: "active"},
		{Name: "codex-error.json", Provider: "codex", Status: "error"},
		{Name: "qwen-active.json", Provider: "qwen", Status: "active"},
		{Name: "claude-disabled.json", Provider: "claude", Status: "active", Disabled: true},
	}

	filtered, stats := filterImportableCPARemoteFiles(files)

	require.Len(t, filtered, 1)
	require.Equal(t, "codex-active.json", filtered[0].Name)
	require.Equal(t, 4, stats.Total)
	require.Equal(t, 1, stats.Importable)
	require.Equal(t, 2, stats.SkippedNonNormal)
	require.Equal(t, 1, stats.SkippedUnsupported)
}
