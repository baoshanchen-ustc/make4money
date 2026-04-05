//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// rateLimitAccountRepoStubWithSchedulable extends rateLimitAccountRepoStub
// to track SetSchedulable calls.
type rateLimitAccountRepoStubWithSchedulable struct {
	rateLimitAccountRepoStub
	setSchedulableCalls int
	lastSchedulable     bool
}

func (r *rateLimitAccountRepoStubWithSchedulable) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	r.setSchedulableCalls++
	r.lastSchedulable = schedulable
	return nil
}

func TestRateLimitService_HandleUpstreamError_OpenAIAPIKey403ModelAccessIgnored(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:       104,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
	}

	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusForbidden,
		http.Header{},
		[]byte(`{"error":{"message":"model not allowed for this project","code":"forbidden"}}`),
	)

	require.False(t, shouldDisable)
	require.Equal(t, 0, repo.setErrorCalls)
}

func TestRateLimitService_HandleUpstreamError_GeminiAPIKey400InvalidDisables(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:       105,
		Platform: PlatformGemini,
		Type:     AccountTypeAPIKey,
	}

	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusBadRequest,
		http.Header{},
		[]byte(`{"error":{"message":"API key not valid. Please pass a valid API key.","status":"API_KEY_INVALID"}}`),
	)

	require.True(t, shouldDisable)
	require.Equal(t, 1, repo.setErrorCalls)
}

func TestRateLimitService_HandleUpstreamError_APIKey429UsesTemporaryCooldown(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:       106,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
	}

	before := time.Now()
	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusTooManyRequests,
		http.Header{},
		[]byte(`{"error":{"message":"rate limited"}}`),
	)
	after := time.Now()

	require.True(t, shouldDisable)
	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 1, repo.rateLimitedCalls)
	require.Equal(t, 0, repo.overloadedCalls)
	require.Equal(t, 0, repo.tempCalls)
	require.NotNil(t, repo.lastRateLimitResetAt)
	require.WithinDuration(t, before.Add(apiKey429Cooldown), *repo.lastRateLimitResetAt, after.Sub(before)+time.Second)
}

func TestRateLimitService_HandleUpstreamError_APIKey529UsesTemporaryCooldown(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:       108,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
	}

	before := time.Now()
	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		529,
		http.Header{},
		[]byte(`{"error":{"message":"overloaded"}}`),
	)
	after := time.Now()

	require.True(t, shouldDisable)
	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 0, repo.rateLimitedCalls)
	require.Equal(t, 1, repo.overloadedCalls)
	require.Equal(t, 0, repo.tempCalls)
	require.NotNil(t, repo.lastOverloadedUntil)
	require.WithinDuration(t, before.Add(apiKey529Cooldown), *repo.lastOverloadedUntil, after.Sub(before)+time.Second)
}

func TestRateLimitService_HandleUpstreamError_APIKey503UsesTemporaryCooldown(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:       107,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
	}

	before := time.Now()
	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusServiceUnavailable,
		http.Header{},
		[]byte(`{"error":{"message":"service temporarily unavailable"}}`),
	)
	after := time.Now()

	require.True(t, shouldDisable)
	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 0, repo.rateLimitedCalls)
	require.Equal(t, 0, repo.overloadedCalls)
	require.Equal(t, 1, repo.tempCalls)
	require.NotNil(t, repo.lastTempUntil)
	require.WithinDuration(t, before.Add(apiKeyServerErrorCooldown), *repo.lastTempUntil, after.Sub(before)+time.Second)
}

// TestRateLimitService_HandleUpstreamError_OpenAIAPIKey402AccountNotActivePermanentDisable
// 验证 OpenAI API key 账单欠费/账号未激活（402）触发永久禁用
func TestRateLimitService_HandleUpstreamError_OpenAIAPIKey402AccountNotActivePermanentDisable(t *testing.T) {
	repo := &rateLimitAccountRepoStubWithSchedulable{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:          201,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Schedulable: true,
	}

	shouldDisable := svc.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusPaymentRequired,
		http.Header{},
		[]byte(`{"error":{"message":"Your account is not active, please check your billing details on our website.","type":"invalid_request_error","code":"account_inactive"}}`),
	)

	require.True(t, shouldDisable)
	require.Equal(t, 1, repo.setErrorCalls)
}

// TestRateLimitService_HandleAuthError_ClosesSchedulingSwitch
// 验证 handleAuthError 在永久禁用 key 时同步关闭调度开关
func TestRateLimitService_HandleAuthError_ClosesSchedulingSwitch(t *testing.T) {
	repo := &rateLimitAccountRepoStubWithSchedulable{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:          202,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Schedulable: true,
	}

	svc.handleAuthError(context.Background(), account, "API key permanently disabled")

	require.Equal(t, 1, repo.setErrorCalls)
	require.Equal(t, 1, repo.setSchedulableCalls)
	require.False(t, repo.lastSchedulable)
}

// TestRateLimitService_HandleAuthError_SkipsSchedulableIfAlreadyFalse
// 验证调度开关已关闭时不重复调用 SetSchedulable
func TestRateLimitService_HandleAuthError_SkipsSchedulableIfAlreadyFalse(t *testing.T) {
	repo := &rateLimitAccountRepoStubWithSchedulable{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:          203,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Schedulable: false,
	}

	svc.handleAuthError(context.Background(), account, "already disabled")

	require.Equal(t, 1, repo.setErrorCalls)
	require.Equal(t, 0, repo.setSchedulableCalls)
}

// TestClassifyAPIKeyStatusAction_OpenAIAccountNotActive
// 验证 "account is not active" 消息被正确识别为永久禁用
func TestClassifyAPIKeyStatusAction_OpenAIAccountNotActive(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		expected   APIKeyStatusAction
	}{
		{
			name:       "403 account is not active",
			statusCode: http.StatusForbidden,
			body:       []byte(`{"error":{"message":"Your account is not active, please check your billing details on our website.","type":"invalid_request_error"}}`),
			expected:   APIKeyStatusActionPermanentDisable,
		},
		{
			name:       "400 account is not active",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error":{"message":"Your account is not active, please check your billing details.","type":"invalid_request_error"}}`),
			expected:   APIKeyStatusActionPermanentDisable,
		},
		{
			name:       "402 payment required",
			statusCode: http.StatusPaymentRequired,
			body:       []byte(`{"error":{"message":"You exceeded your current quota","type":"insufficient_quota"}}`),
			expected:   APIKeyStatusActionPermanentDisable,
		},
		{
			name:       "403 billing_not_active code",
			statusCode: http.StatusForbidden,
			body:       []byte(`{"error":{"message":"Billing not active","code":"billing_not_active"}}`),
			expected:   APIKeyStatusActionPermanentDisable,
		},
		{
			name:       "403 account suspended",
			statusCode: http.StatusForbidden,
			body:       []byte(`{"error":{"message":"Your account has been suspended","type":"invalid_request_error"}}`),
			expected:   APIKeyStatusActionPermanentDisable,
		},
		{
			name:       "403 model access forbidden should be ignored",
			statusCode: http.StatusForbidden,
			body:       []byte(`{"error":{"message":"model not allowed for this project","code":"forbidden"}}`),
			expected:   APIKeyStatusActionIgnore,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				ID:       999,
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
			}
			result := ClassifyAPIKeyStatusAction(account, tt.statusCode, tt.body)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestClassifyAPIKeyStatusAction_OpenAI429InsufficientQuota
// 验证 OpenAI 429 insufficient_quota 被识别为永久禁用（余额耗尽）而非临时限速
func TestClassifyAPIKeyStatusAction_OpenAI429InsufficientQuota(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	tests := []struct {
		name     string
		body     []byte
		expected APIKeyStatusAction
	}{
		{
			name:     "insufficient_quota code",
			body:     []byte(`{"error":{"message":"You exceeded your current quota, please check your plan and billing details.","type":"insufficient_quota","code":"insufficient_quota"}}`),
			expected: APIKeyStatusActionPermanentDisable,
		},
		{
			name:     "regular rate limit should be cooldown",
			body:     []byte(`{"error":{"message":"Rate limit reached for model","type":"requests","code":"rate_limit_exceeded"}}`),
			expected: APIKeyStatusActionTemporaryCooldown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyAPIKeyStatusAction(account, http.StatusTooManyRequests, tt.body)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestClassifyAPIKeyStatusAction_AnthropicCreditBalance
// 验证 Anthropic 400 余额不足被正确识别为永久禁用
func TestClassifyAPIKeyStatusAction_AnthropicCreditBalance(t *testing.T) {
	account := &Account{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}

	tests := []struct {
		name     string
		body     []byte
		expected APIKeyStatusAction
	}{
		{
			name:     "credit balance too low",
			body:     []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"Your credit balance is too low to access the Anthropic API. Please go to Plans \u0026 Billing to upgrade or purchase credits."}}`),
			expected: APIKeyStatusActionPermanentDisable,
		},
		{
			name:     "401 invalid key",
			body:     []byte(`{"type":"error","error":{"type":"authentication_error","message":"Invalid API key"}}`),
			expected: APIKeyStatusActionPermanentDisable,
		},
		{
			name:     "400 unrelated bad request should be ignored",
			body:     []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens is required"}}`),
			expected: APIKeyStatusActionIgnore,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusCode := http.StatusBadRequest
			if tt.name == "401 invalid key" {
				statusCode = http.StatusUnauthorized
			}
			result := ClassifyAPIKeyStatusAction(account, statusCode, tt.body)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestClassifyAPIKeyStatusAction_GeminiBillingDisabled
// 验证 Gemini 403 BILLING_DISABLED / CONSUMER_SUSPENDED 被识别为永久禁用
func TestClassifyAPIKeyStatusAction_GeminiBillingDisabled(t *testing.T) {
	account := &Account{ID: 3, Platform: PlatformGemini, Type: AccountTypeAPIKey}

	billingDisabledBody := []byte(`{
		"error": {
			"code": 403,
			"message": "Billing is disabled for this project.",
			"status": "PERMISSION_DENIED",
			"details": [{"@type": "type.googleapis.com/google.rpc.ErrorInfo","reason": "BILLING_DISABLED","domain": "googleapis.com"}]
		}
	}`)

	consumerSuspendedBody := []byte(`{
		"error": {
			"code": 403,
			"message": "The caller does not have permission",
			"status": "PERMISSION_DENIED",
			"details": [{"@type": "type.googleapis.com/google.rpc.ErrorInfo","reason": "CONSUMER_SUSPENDED","domain": "googleapis.com"}]
		}
	}`)

	failedPreconditionBody := []byte(`{
		"error": {
			"code": 400,
			"message": "Gemini API free tier is not available in your country. Please enable billing on your project in Google AI Studio.",
			"status": "FAILED_PRECONDITION"
		}
	}`)

	tests := []struct {
		name       string
		statusCode int
		body       []byte
		expected   APIKeyStatusAction
	}{
		{"403 BILLING_DISABLED", http.StatusForbidden, billingDisabledBody, APIKeyStatusActionPermanentDisable},
		{"403 CONSUMER_SUSPENDED", http.StatusForbidden, consumerSuspendedBody, APIKeyStatusActionPermanentDisable},
		{"400 FAILED_PRECONDITION free tier", http.StatusBadRequest, failedPreconditionBody, APIKeyStatusActionPermanentDisable},
		{"429 rate limit is cooldown", http.StatusTooManyRequests, []byte(`{"error":{"code":429,"message":"Resource exhausted","status":"RESOURCE_EXHAUSTED"}}`), APIKeyStatusActionTemporaryCooldown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyAPIKeyStatusAction(account, tt.statusCode, tt.body)
			require.Equal(t, tt.expected, result)
		})
	}
}
