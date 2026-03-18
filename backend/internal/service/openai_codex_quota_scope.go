package service

import "strings"

type openAICodexQuotaScope string

const (
	openAICodexQuotaScopeCodex openAICodexQuotaScope = "codex"
	openAICodexQuotaScopeSpark openAICodexQuotaScope = "spark"

	openAICodexModelRateLimitKeyCodex = "openai_codex"
	openAICodexModelRateLimitKeySpark = "openai_codex_spark"

	openAICodexModelIDCodex = "gpt-5.3-codex"
	openAICodexModelIDSpark = "gpt-5.3-codex-spark"
)

func normalizeOpenAIModelID(model string) string {
	value := strings.TrimSpace(strings.ToLower(model))
	if value == "" {
		return ""
	}
	if strings.Contains(value, "/") {
		parts := strings.Split(value, "/")
		value = strings.TrimSpace(parts[len(parts)-1])
	}
	value = strings.NewReplacer("_", "-", " ", "-").Replace(value)
	return value
}

func isOpenAICodexQuotaModel(model string) bool {
	normalized := normalizeOpenAIModelID(model)
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "codex") {
		return true
	}
	// OpenAI OAuth Codex path主要使用 gpt-5 系列。
	return strings.HasPrefix(normalized, "gpt-5")
}

func classifyOpenAICodexQuotaScope(model string) openAICodexQuotaScope {
	normalized := normalizeOpenAIModelID(model)
	if normalized == "" {
		return openAICodexQuotaScopeCodex
	}
	if strings.Contains(normalized, "gpt-5.3-codex-spark") {
		return openAICodexQuotaScopeSpark
	}
	if strings.Contains(normalized, "codex") && strings.Contains(normalized, "spark") {
		return openAICodexQuotaScopeSpark
	}
	return openAICodexQuotaScopeCodex
}

func codexQuotaScopeFromModels(requestedModel, mappedModel string) openAICodexQuotaScope {
	if classifyOpenAICodexQuotaScope(requestedModel) == openAICodexQuotaScopeSpark {
		return openAICodexQuotaScopeSpark
	}
	if classifyOpenAICodexQuotaScope(mappedModel) == openAICodexQuotaScopeSpark {
		return openAICodexQuotaScopeSpark
	}
	return openAICodexQuotaScopeCodex
}

func codexScopeExtraPrefix(scope openAICodexQuotaScope) string {
	if scope == openAICodexQuotaScopeSpark {
		return "codex_spark"
	}
	return "codex"
}

func codexScopeExtraKey(scope openAICodexQuotaScope, suffix string) string {
	return codexScopeExtraPrefix(scope) + "_" + suffix
}

func codexScopeModelRateLimitKey(scope openAICodexQuotaScope) string {
	if scope == openAICodexQuotaScopeSpark {
		return openAICodexModelRateLimitKeySpark
	}
	return openAICodexModelRateLimitKeyCodex
}

func resolveOpenAICodexModelRateLimitKey(requestedModel, mappedModel string) string {
	if !isOpenAICodexQuotaModel(requestedModel) && !isOpenAICodexQuotaModel(mappedModel) {
		return ""
	}
	scope := codexQuotaScopeFromModels(requestedModel, mappedModel)
	return codexScopeModelRateLimitKey(scope)
}

func isOpenAICodexScopeSupported(account *Account, scope openAICodexQuotaScope) bool {
	if account == nil || !account.IsOpenAI() {
		return false
	}
	switch scope {
	case openAICodexQuotaScopeSpark:
		return account.IsModelSupported(openAICodexModelIDSpark)
	default:
		return account.IsModelSupported(openAICodexModelIDCodex)
	}
}
