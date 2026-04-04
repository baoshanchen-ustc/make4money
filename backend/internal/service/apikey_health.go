package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

type APIKeyHealthCheckResult struct {
	Platform   string `json:"platform"`
	StatusCode int    `json:"status_code"`
	Valid      bool   `json:"valid"`
	Invalid    bool   `json:"invalid"`
	Message    string `json:"message,omitempty"`
}

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

func ShouldDisableAPIKeyAuthFailure(account *Account, statusCode int, responseBody []byte) bool {
	if account == nil || account.Type != AccountTypeAPIKey {
		return false
	}

	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(responseBody)))

	switch account.Platform {
	case PlatformOpenAI:
		if statusCode == http.StatusUnauthorized {
			return true
		}
		if statusCode != http.StatusForbidden {
			return false
		}
		if code == "invalid_api_key" || code == "token_invalidated" || code == "token_revoked" || code == "account_deactivated" || code == "deactivated_workspace" {
			return true
		}
		return containsAny(msg,
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
		)
	case PlatformAnthropic:
		if statusCode == http.StatusUnauthorized {
			return true
		}
		if statusCode != http.StatusForbidden {
			return false
		}
		return true
	case PlatformGemini:
		return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
	default:
		return statusCode == http.StatusUnauthorized
	}
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
	case PlatformAnthropic:
		switch statusCode {
		case http.StatusOK, http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusTooManyRequests, 529:
			return true, false, message
		case http.StatusUnauthorized, http.StatusForbidden:
			return false, true, message
		default:
			return false, false, message
		}
	case PlatformOpenAI:
		switch statusCode {
		case http.StatusOK, http.StatusTooManyRequests:
			return true, false, message
		case http.StatusPaymentRequired:
			return false, true, message
		case http.StatusUnauthorized:
			return false, true, message
		case http.StatusForbidden:
			return false, ShouldDisableAPIKeyAuthFailure(account, statusCode, responseBody), message
		default:
			return false, false, message
		}
	case PlatformGemini:
		switch statusCode {
		case http.StatusOK, http.StatusTooManyRequests:
			return true, false, message
		case http.StatusBadRequest:
			bodyUpper := strings.ToUpper(string(responseBody))
			msgLower := strings.ToLower(message)
			if strings.Contains(bodyUpper, "API_KEY_INVALID") || containsAny(msgLower, "api key not valid", "invalid api key", "api_key_invalid") {
				return false, true, message
			}
			return false, false, message
		case http.StatusUnauthorized, http.StatusForbidden:
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(normalizedBaseURL, "/")+"/v1/messages", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", claude.APIKeyBetaHeader)
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
