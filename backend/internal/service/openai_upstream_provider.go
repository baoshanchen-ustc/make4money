package service

import (
	"net/url"
	"strings"
)

func isDeepSeekBaseURL(base string) bool {
	normalized := strings.TrimSpace(base)
	if normalized == "" {
		return false
	}

	if parsed, err := url.Parse(normalized); err == nil && parsed != nil {
		host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
		return host == "api.deepseek.com" || strings.HasSuffix(host, ".deepseek.com")
	}

	lower := strings.ToLower(normalized)
	return strings.Contains(lower, "api.deepseek.com")
}

func buildOpenAIChatCompletionsURL(base string) string {
	normalized := strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(normalized, "/chat/completions") {
		return normalized
	}
	if strings.HasSuffix(normalized, "/v1") {
		return normalized + "/chat/completions"
	}
	return normalized + "/chat/completions"
}

func shouldUseDirectOpenAIChatCompletionsUpstream(account *Account) bool {
	if account == nil || account.Type != AccountTypeAPIKey {
		return false
	}
	return isDeepSeekBaseURL(account.GetOpenAIBaseURL())
}
