package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/googleapi"
)

type APIKeyHealthCheckResult struct {
	Platform   string `json:"platform"`
	StatusCode int    `json:"status_code"`
	Valid      bool   `json:"valid"`
	Invalid    bool   `json:"invalid"`
	Message    string `json:"message,omitempty"`
}

type APIKeyStatusAction int

const (
	APIKeyStatusActionIgnore APIKeyStatusAction = iota
	APIKeyStatusActionValid
	APIKeyStatusActionPermanentDisable
	APIKeyStatusActionTemporaryCooldown
)

func DetectAPIKeyPlatform(rawKey string) (string, bool) {
	key := strings.TrimSpace(rawKey)
	switch {
	case strings.HasPrefix(key, "sk-ant-"):
		return PlatformAnthropic, true
	case strings.HasPrefix(key, "AIza"):
		return PlatformGemini, true
	case strings.HasPrefix(strings.ToLower(key), "sk-"):
		return PlatformOpenAI, true
	default:
		return "", false
	}
}

func DefaultAPIKeyBaseURL(platform string) string {
	switch strings.TrimSpace(platform) {
	case PlatformAnthropic:
		return "https://api.anthropic.com"
	case PlatformOpenAI:
		return "https://api.openai.com"
	case PlatformGemini:
		return "https://generativelanguage.googleapis.com"
	default:
		return ""
	}
}

func ShouldDisableAPIKeyStatus(account *Account, statusCode int, responseBody []byte) bool {
	return ClassifyAPIKeyStatusAction(account, statusCode, responseBody) == APIKeyStatusActionPermanentDisable
}

func ClassifyAPIKeyStatusAction(account *Account, statusCode int, responseBody []byte) APIKeyStatusAction {
	if account == nil || account.Type != AccountTypeAPIKey {
		return APIKeyStatusActionIgnore
	}
	if statusCode == http.StatusOK {
		return APIKeyStatusActionValid
	}

	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(responseBody)))
	bodyUpper := strings.ToUpper(string(responseBody))

	// 5xx and 529 are always temporary cooldowns regardless of platform
	switch statusCode {
	case 529, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return APIKeyStatusActionTemporaryCooldown
	}

	switch account.Platform {
	case PlatformOpenAI:
		switch statusCode {
		case http.StatusUnauthorized, http.StatusPaymentRequired:
			return APIKeyStatusActionPermanentDisable
		case http.StatusTooManyRequests:
			// insufficient_quota is permanent billing exhaustion, not a temporary rate limit
			if code == "insufficient_quota" || containsAny(msg, "exceeded your current quota", "insufficient_quota") {
				return APIKeyStatusActionPermanentDisable
			}
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusBadRequest:
			if containsAny(msg,
				"organization has been disabled",
				"project has been disabled",
				"workspace has been deactivated",
				"workspace has been disabled",
				"account deactivated",
				"account has been deactivated",
				"key is disabled",
				"api key disabled",
				"account is not active",
				"billing details",
				"check your billing",
				"account has been suspended",
				"account suspended",
				"billing_hard_limit_reached",
				"billing hard limit",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			if code == "billing_hard_limit_reached" {
				return APIKeyStatusActionPermanentDisable
			}
		case http.StatusForbidden:
			if containsAny(code,
				"invalid_api_key",
				"token_invalidated",
				"token_revoked",
				"account_deactivated",
				"deactivated_workspace",
				"billing_not_active",
				"account_inactive",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			if containsAny(msg,
				"invalid api key",
				"incorrect api key",
				"token invalidated",
				"token revoked",
				"account deactivated",
				"workspace has been deactivated",
				"organization has been disabled",
				"project has been disabled",
				"key is disabled",
				"api key disabled",
				"account is not active",
				"billing details",
				"check your billing",
				"account has been suspended",
				"account suspended",
			) {
				return APIKeyStatusActionPermanentDisable
			}
		}
	case PlatformAnthropic:
		switch statusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return APIKeyStatusActionPermanentDisable
		case http.StatusTooManyRequests:
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusBadRequest:
			// Anthropic returns 400 for credit balance exhaustion (not 402/429)
			if containsAny(msg,
				"credit balance is too low",
				"your credit balance",
				"insufficient credits",
				"account has been disabled",
				"organization has been disabled",
			) {
				return APIKeyStatusActionPermanentDisable
			}
		}
	case PlatformGemini:
		switch statusCode {
		case http.StatusTooManyRequests:
			return APIKeyStatusActionTemporaryCooldown
		case http.StatusUnauthorized:
			return APIKeyStatusActionPermanentDisable
		case http.StatusForbidden:
			// Use structured reason check first (covers BILLING_DISABLED, CONSUMER_SUSPENDED, PROJECT_DISABLED, SERVICE_DISABLED)
			if googleapi.IsPermanentlyDisabledError(string(responseBody)) {
				return APIKeyStatusActionPermanentDisable
			}
			if containsAny(msg,
				"billing is disabled",
				"billing disabled",
				"consumer suspended",
				"project disabled",
				"project has been suspended",
				"does not have permission",
			) {
				return APIKeyStatusActionPermanentDisable
			}
			return APIKeyStatusActionPermanentDisable
		case http.StatusBadRequest:
			if strings.Contains(bodyUpper, "API_KEY_INVALID") || googleapi.IsServiceDisabledError(string(responseBody)) {
				return APIKeyStatusActionPermanentDisable
			}
			// FAILED_PRECONDITION: free tier not available in region, requires billing setup
			if strings.Contains(bodyUpper, "FAILED_PRECONDITION") {
				return APIKeyStatusActionPermanentDisable
			}
			if containsAny(msg,
				"api key not valid",
				"invalid api key",
				"api_key_invalid",
				"api key is invalid",
				"before or it is disabled",
				"service disabled",
				"api has not been used in project",
				"unregistered callers",
				"caller not registered",
				"free tier is not available",
				"enable billing",
			) {
				return APIKeyStatusActionPermanentDisable
			}
		}
	}

	return APIKeyStatusActionIgnore
}

func ShouldDisableAPIKeyAuthFailure(account *Account, statusCode int, responseBody []byte) bool {
	return ShouldDisableAPIKeyStatus(account, statusCode, responseBody)
}

func ClassifyAPIKeyProbeResponse(account *Account, statusCode int, responseBody []byte) (valid bool, invalid bool, message string) {
	if account == nil || account.Type != AccountTypeAPIKey {
		return false, false, "unsupported account type"
	}

	message = strings.TrimSpace(extractUpstreamErrorMessage(responseBody))
	if message == "" {
		message = http.StatusText(statusCode)
	}
	message = sanitizeUpstreamErrorMessage(message)

	switch account.Platform {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini:
		switch ClassifyAPIKeyStatusAction(account, statusCode, responseBody) {
		case APIKeyStatusActionValid:
			return true, false, message
		case APIKeyStatusActionPermanentDisable:
			return false, true, message
		default:
			return false, false, message
		}
	default:
		return false, false, message
	}
}

func (s *AccountTestService) CheckAPIKeyValidity(ctx context.Context, account *Account) (*APIKeyHealthCheckResult, error) {
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}
	if account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("account %d is not an apikey account", account.ID)
	}
	if s == nil || s.httpUpstream == nil {
		return nil, fmt.Errorf("account test service is not configured")
	}

	req, err := s.buildAPIKeyProbeRequest(ctx, account)
	if err != nil {
		return nil, err
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	valid, invalid, message := ClassifyAPIKeyProbeResponse(account, resp.StatusCode, respBody)
	return &APIKeyHealthCheckResult{
		Platform:   account.Platform,
		StatusCode: resp.StatusCode,
		Valid:      valid,
		Invalid:    invalid,
		Message:    message,
	}, nil
}

func (s *AccountTestService) buildAPIKeyProbeRequest(ctx context.Context, account *Account) (*http.Request, error) {
	switch account.Platform {
	case PlatformAnthropic:
		return s.buildAnthropicAPIKeyProbeRequest(ctx, account)
	case PlatformOpenAI:
		return s.buildOpenAIAPIKeyProbeRequest(ctx, account)
	case PlatformGemini:
		return s.buildGeminiAPIKeyProbeRequest(ctx, account)
	default:
		return nil, fmt.Errorf("unsupported apikey platform: %s", account.Platform)
	}
}

func (s *AccountTestService) buildAnthropicAPIKeyProbeRequest(ctx context.Context, account *Account) (*http.Request, error) {
	baseURL := strings.TrimSpace(account.GetBaseURL())
	if baseURL == "" {
		baseURL = DefaultAPIKeyBaseURL(account.Platform)
	}
	normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid anthropic base url: %w", err)
	}

	// Use GET /v1/models for probe - no token consumption, pure auth check.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(normalizedBaseURL, "/")+"/v1/models",
		nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", account.GetCredential("api_key"))
	req.Header.Set("User-Agent", proxyQualityClientUserAgent)
	return req, nil
}

func (s *AccountTestService) buildOpenAIAPIKeyProbeRequest(ctx context.Context, account *Account) (*http.Request, error) {
	baseURL := strings.TrimSpace(account.GetOpenAIBaseURL())
	if baseURL == "" {
		baseURL = DefaultAPIKeyBaseURL(account.Platform)
	}
	normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid openai base url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(normalizedBaseURL, "/")+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+account.GetCredential("api_key"))
	req.Header.Set("User-Agent", proxyQualityClientUserAgent)
	return req, nil
}

func (s *AccountTestService) buildGeminiAPIKeyProbeRequest(ctx context.Context, account *Account) (*http.Request, error) {
	baseURL := strings.TrimSpace(account.GetBaseURL())
	if baseURL == "" {
		baseURL = DefaultAPIKeyBaseURL(account.Platform)
	}
	normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid gemini base url: %w", err)
	}

	endpoint := strings.TrimSuffix(normalizedBaseURL, "/") + "/v1beta/models?key=" + url.QueryEscape(account.GetCredential("api_key"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", proxyQualityClientUserAgent)
	return req, nil
}

func containsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}
