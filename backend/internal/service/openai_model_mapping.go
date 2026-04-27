package service

import "strings"

func resolveOpenAIForwardModel(account *Account, requestedModel, defaultMappedModel string) string {
	if account == nil {
		if defaultMappedModel != "" {
			return defaultMappedModel
		}
		return requestedModel
	}

	mapping := account.GetModelMapping()
	if len(mapping) > 0 {
		if mappedModel, exists := mapping[requestedModel]; exists {
			return mappedModel
		}
		for pattern := range mapping {
			if matchWildcard(pattern, requestedModel) {
				return matchWildcardMapping(mapping, requestedModel)
			}
		}
	}

	if defaultMappedModel != "" && !isExplicitCodexModel(requestedModel) {
		return defaultMappedModel
	}
	return requestedModel
}

func isExplicitCodexModel(model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	if strings.Contains(model, "/") {
		parts := strings.Split(model, "/")
		model = parts[len(parts)-1]
	}
	model = strings.ToLower(strings.TrimSpace(model))
	if getNormalizedCodexModel(model) != "" {
		return true
	}
	if strings.HasSuffix(model, "-openai-compact") {
		base := strings.TrimSuffix(model, "-openai-compact")
		return getNormalizedCodexModel(base) != ""
	}
	return false
}
