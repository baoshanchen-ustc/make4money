//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIAccountTestRepo struct {
	mockAccountRepoForGemini
	updatedExtra  map[string]any
	rateLimitedID int64
	rateLimitedAt *time.Time
}

func (r *openAIAccountTestRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updatedExtra = updates
	return nil
}

func (r *openAIAccountTestRepo) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.rateLimitedID = id
	r.rateLimitedAt = &resetAt
	return nil
}

func TestAccountTestService_OpenAISuccessPersistsSnapshotFromHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newSoraTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader(`data: {"type":"response.completed"}

`))
	resp.Header.Set("x-codex-primary-used-percent", "88")
	resp.Header.Set("x-codex-primary-reset-after-seconds", "604800")
	resp.Header.Set("x-codex-primary-window-minutes", "10080")
	resp.Header.Set("x-codex-secondary-used-percent", "42")
	resp.Header.Set("x-codex-secondary-reset-after-seconds", "18000")
	resp.Header.Set("x-codex-secondary-window-minutes", "300")

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	account := &Account{
		ID:          89,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4")
	require.NoError(t, err)
	require.NotEmpty(t, repo.updatedExtra)
	require.Equal(t, 42.0, repo.updatedExtra["codex_5h_used_percent"])
	require.Equal(t, 88.0, repo.updatedExtra["codex_7d_used_percent"])
	require.Contains(t, recorder.Body.String(), "test_complete")
}

func TestAccountTestService_OpenAI429PersistsSnapshotAndRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newSoraTestContext()

	resp := newJSONResponse(http.StatusTooManyRequests, `{"error":{"type":"usage_limit_reached","message":"limit reached"}}`)
	resp.Header.Set("x-codex-primary-used-percent", "100")
	resp.Header.Set("x-codex-primary-reset-after-seconds", "604800")
	resp.Header.Set("x-codex-primary-window-minutes", "10080")
	resp.Header.Set("x-codex-secondary-used-percent", "100")
	resp.Header.Set("x-codex-secondary-reset-after-seconds", "18000")
	resp.Header.Set("x-codex-secondary-window-minutes", "300")

	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{accountRepo: repo, httpUpstream: upstream}
	account := &Account{
		ID:          88,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4")
	require.Error(t, err)
	require.NotEmpty(t, repo.updatedExtra)
	require.Equal(t, 100.0, repo.updatedExtra["codex_5h_used_percent"])
	require.Equal(t, int64(88), repo.rateLimitedID)
	require.NotNil(t, repo.rateLimitedAt)
	require.NotNil(t, account.RateLimitResetAt)
	if account.RateLimitResetAt != nil && repo.rateLimitedAt != nil {
		require.WithinDuration(t, *repo.rateLimitedAt, *account.RateLimitResetAt, time.Second)
	}
}

func TestAccountTestService_OpenAIApiKeyUsesV1ResponsesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newSoraTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader(`data: {"type":"response.completed"}

`))

	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}
	account := &Account{
		ID:          90,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://api.openai.com"},
	}

	err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4")
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://api.openai.com/v1/responses", upstream.requests[0].URL.String())
	require.Contains(t, recorder.Body.String(), "test_complete")
}

// openAIStreamTextErrorRepo tracks SetError and SetSchedulable calls.
type openAIStreamTextErrorRepo struct {
	mockAccountRepoForGemini
	setErrorCalls       int
	lastErrorMsg        string
	setSchedulableCalls int
	lastSchedulable     bool
}

func (r *openAIStreamTextErrorRepo) SetError(_ context.Context, _ int64, errorMsg string) error {
	r.setErrorCalls++
	r.lastErrorMsg = errorMsg
	return nil
}

func (r *openAIStreamTextErrorRepo) SetSchedulable(_ context.Context, _ int64, schedulable bool) error {
	r.setSchedulableCalls++
	r.lastSchedulable = schedulable
	return nil
}

// TestAccountTestService_OpenAIApiKey_StreamTextAccountError verifies that when an
// OpenAI-compatible upstream returns an account-level error message as plain text
// delta content (HTTP 200 + stream), the account is permanently disabled.
func TestAccountTestService_OpenAIApiKey_StreamTextAccountError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name    string
		sseBody string
		wantErr bool
	}{
		{
			name: "account_not_active_via_completed_event",
			sseBody: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Your account is not active, please check your billing details on our website.\"}\n\n" +
				"data: {\"type\":\"response.completed\"}\n\n",
			wantErr: true,
		},
		{
			name: "account_not_active_via_done",
			sseBody: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Your account is not active, please check your billing details on our website.\"}\n\n" +
				"data: [DONE]\n\n",
			wantErr: true,
		},
		{
			name:    "account_not_active_via_eof",
			sseBody: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Your account is not active, please check your billing details on our website.\"}",
			wantErr: true,
		},
		{
			name: "normal_response_not_flagged",
			sseBody: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello! How can I help you?\"}\n\n" +
				"data: {\"type\":\"response.completed\"}\n\n",
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, recorder := newSoraTestContext()

			resp := newJSONResponse(http.StatusOK, "")
			resp.Body = io.NopCloser(strings.NewReader(tc.sseBody))

			repo := &openAIStreamTextErrorRepo{}
			upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
			svc := &AccountTestService{
				httpUpstream: upstream,
				accountRepo:  repo,
				cfg:          &config.Config{},
			}
			account := &Account{
				ID:          91,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Schedulable: true,
				Concurrency: 1,
				Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://api.openai.com"},
			}

			err := svc.testOpenAIAccountConnection(ctx, account, "gpt-5.4")
			if tc.wantErr {
				require.Error(t, err)
				require.Equal(t, 1, repo.setErrorCalls, "SetError should be called once")
				require.Equal(t, 1, repo.setSchedulableCalls, "SetSchedulable should be called once")
				require.False(t, repo.lastSchedulable, "account should be marked not schedulable")
				require.NotContains(t, recorder.Body.String(), "test_complete")
			} else {
				require.NoError(t, err)
				require.Equal(t, 0, repo.setErrorCalls, "SetError should NOT be called")
				require.Equal(t, 0, repo.setSchedulableCalls, "SetSchedulable should NOT be called")
				require.Contains(t, recorder.Body.String(), "test_complete")
			}
		})
	}
}

// TestClassifyOpenAIStreamTextAsAccountError verifies pattern matching.
func TestClassifyOpenAIStreamTextAsAccountError(t *testing.T) {
	account := &Account{ID: 1, Type: AccountTypeAPIKey, Platform: PlatformOpenAI}

	cases := []struct {
		text   string
		action APIKeyStatusAction
	}{
		{"Your account is not active, please check your billing details on our website.", APIKeyStatusActionPermanentDisable},
		{"account is not active", APIKeyStatusActionPermanentDisable},
		{"billing details", APIKeyStatusActionPermanentDisable},
		{"check your billing", APIKeyStatusActionPermanentDisable},
		{"Account has been suspended.", APIKeyStatusActionPermanentDisable},
		{"insufficient_quota exceeded", APIKeyStatusActionPermanentDisable},
		{"Hello, how can I assist you today?", APIKeyStatusActionIgnore},
		{"", APIKeyStatusActionIgnore},
	}

	for _, tc := range cases {
		got := classifyOpenAIStreamTextAsAccountError(account, tc.text)
		if got != tc.action {
			t.Errorf("text=%q: want %v, got %v", tc.text, tc.action, got)
		}
	}
}
