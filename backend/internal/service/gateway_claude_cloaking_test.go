package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestComputeClaudeCodeBillingFingerprint(t *testing.T) {
	message := "01234567890123456789012345"
	got := computeClaudeCodeBillingFingerprint(message, "2.1.88")
	require.Equal(t, "d4e", got)
}

func TestGenerateClaudeCodeBillingHeader(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"01234567890123456789012345"}]}]}`)
	got := generateClaudeCodeBillingHeader(body, "2.1.88", "cli", "")
	require.Equal(t, "x-anthropic-billing-header: cc_version=2.1.88.d4e; cc_entrypoint=cli;", got)
}

func TestEnsureClaudeOAuthSystemCloaking_InsertsBillingAndPrefix(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-6","system":[{"type":"text","text":"custom system"}],"messages":[{"role":"user","content":[{"type":"text","text":"01234567890123456789012345"}]}]}`)
	next, changed := ensureClaudeOAuthSystemCloaking(body, "2.1.88", "cli")
	require.True(t, changed)

	system := gjson.GetBytes(next, "system")
	require.True(t, system.Exists())
	require.True(t, system.IsArray())
	require.GreaterOrEqual(t, len(system.Array()), 3)

	first := system.Array()[0].Get("text").String()
	require.Contains(t, first, "x-anthropic-billing-header:")
	require.Contains(t, first, "cc_version=2.1.88.d4e")
	require.Contains(t, first, "cc_entrypoint=cli")

	second := system.Array()[1].Get("text").String()
	require.Equal(t, claudeCodeSystemPrompt, strings.TrimSpace(second))
	require.Equal(t, "custom system", system.Array()[2].Get("text").String())
}

func TestEnsureClaudeOAuthSystemCloaking_PreservesExistingBillingBlock(t *testing.T) {
	existingBilling := "x-anthropic-billing-header: cc_version=2.0.0.abc; cc_entrypoint=cli;"
	body := []byte(`{"model":"claude-sonnet-4-6","system":[{"type":"text","text":"` + existingBilling + `"},{"type":"text","text":"You are Claude Code, Anthropic's official CLI for Claude."},{"type":"text","text":"custom"}],"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	next, changed := ensureClaudeOAuthSystemCloaking(body, "2.1.88", "cli")
	require.True(t, changed)

	system := gjson.GetBytes(next, "system")
	require.True(t, system.IsArray())
	items := system.Array()
	require.GreaterOrEqual(t, len(items), 3)
	require.Equal(t, existingBilling, items[0].Get("text").String())
	require.Equal(t, "You are Claude Code, Anthropic's official CLI for Claude.", items[1].Get("text").String())
	require.Equal(t, "custom", items[2].Get("text").String())
	require.Equal(t, 1, strings.Count(system.Raw, "x-anthropic-billing-header"))
}

func TestBuildUpstreamRequest_OAuth_ForcesJSONMetadataAndSessionHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("x-api-key", "test-key-session-metadata")
	c.Request = req

	svc := &GatewayService{}
	account := &Account{
		ID:       4242,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"account_uuid": "acc-uuid-42",
		},
	}

	body := []byte(`{"model":"claude-sonnet-4-6","metadata":{"user_id":"legacy-user-id"},"messages":[{"role":"user","content":[{"type":"text","text":"01234567890123456789012345"}]}]}`)
	upstreamReq, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-6", false, false)
	require.NoError(t, err)

	rawBody, err := io.ReadAll(upstreamReq.Body)
	require.NoError(t, err)
	uidRaw := gjson.GetBytes(rawBody, "metadata.user_id").String()
	require.NotEmpty(t, uidRaw)

	parsed := ParseMetadataUserID(uidRaw)
	require.NotNil(t, parsed)
	require.True(t, parsed.IsNewFormat)
	require.NotEmpty(t, parsed.DeviceID)
	require.Equal(t, "acc-uuid-42", parsed.AccountUUID)
	require.NotEmpty(t, parsed.SessionID)

	sessionHeader := getHeaderRaw(upstreamReq.Header, "X-Claude-Code-Session-Id")
	require.NotEmpty(t, sessionHeader)
	require.Equal(t, parsed.SessionID, sessionHeader)
}

func TestBuildUpstreamRequest_OAuth_SessionStablePerAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{
		ID:       5001,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"account_uuid": "acc-uuid-sticky",
		},
	}
	svc := &GatewayService{}
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)

	makeReq := func(apiKey string) *http.Request {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		r := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		r.Header.Set("x-api-key", apiKey)
		c.Request = r
		req, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-6", false, false)
		require.NoError(t, err)
		return req
	}

	reqA1 := makeReq("stable-api-key-a")
	reqA2 := makeReq("stable-api-key-a")
	reqB := makeReq("stable-api-key-b")

	sessionA1 := getHeaderRaw(reqA1.Header, "X-Claude-Code-Session-Id")
	sessionA2 := getHeaderRaw(reqA2.Header, "X-Claude-Code-Session-Id")
	sessionB := getHeaderRaw(reqB.Header, "X-Claude-Code-Session-Id")

	require.NotEmpty(t, sessionA1)
	require.Equal(t, sessionA1, sessionA2)
	require.NotEqual(t, sessionA1, sessionB)
}
