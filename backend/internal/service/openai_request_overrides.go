package service

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

const openAIRequestOverridesExtraKey = "openai_request_overrides"

// GetOpenAIRequestOverrides returns the account-level OpenAI request overrides.
// Only top-level JSON objects are supported; invalid values are ignored.
func (a *Account) GetOpenAIRequestOverrides() map[string]any {
	if a == nil || a.Platform != PlatformOpenAI || a.Extra == nil {
		return nil
	}

	raw, ok := a.Extra[openAIRequestOverridesExtraKey]
	if !ok || raw == nil {
		return nil
	}

	overrides, ok := openAIRequestOverridesMap(raw)
	if !ok || len(overrides) == 0 {
		return nil
	}

	return sanitizeOpenAIRequestOverrides(overrides)
}

func applyOpenAIRequestOverridesToBody(body []byte, account *Account) ([]byte, bool, error) {
	overrides := account.GetOpenAIRequestOverrides()
	if len(body) == 0 || len(overrides) == 0 {
		return body, false, nil
	}

	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return nil, false, fmt.Errorf("parse request body for openai request overrides: %w", err)
	}
	if reqBody == nil {
		reqBody = make(map[string]any)
	}

	if !mergeOpenAIOverrideObjects(reqBody, overrides) {
		return body, false, nil
	}

	updatedBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, false, fmt.Errorf("serialize request body with openai request overrides: %w", err)
	}

	return updatedBody, true, nil
}

func mergeOpenAIOverrideObjects(dst map[string]any, src map[string]any) bool {
	changed := false

	for key, value := range src {
		if srcMap, ok := openAIRequestOverridesMap(value); ok {
			if dstMap, ok := openAIRequestOverridesMap(dst[key]); ok {
				if mergeOpenAIOverrideObjects(dstMap, srcMap) {
					changed = true
				}
				dst[key] = dstMap
				continue
			}

			dst[key] = cloneOpenAIOverrideValue(srcMap)
			changed = true
			continue
		}

		cloned := cloneOpenAIOverrideValue(value)
		if existing, ok := dst[key]; !ok || !reflect.DeepEqual(existing, cloned) {
			dst[key] = cloned
			changed = true
		}
	}

	return changed
}

func openAIRequestOverridesMap(value any) (map[string]any, bool) {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	return obj, true
}

func sanitizeOpenAIRequestOverrides(overrides map[string]any) map[string]any {
	if len(overrides) == 0 {
		return nil
	}

	sanitized := make(map[string]any, len(overrides))
	for key, value := range overrides {
		if !isAllowedOpenAIRequestOverrideKey(key) {
			continue
		}
		sanitized[key] = cloneOpenAIOverrideValue(value)
	}

	if len(sanitized) == 0 {
		return nil
	}

	return sanitized
}

func isAllowedOpenAIRequestOverrideKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "model":
		return false
	default:
		return true
	}
}

func cloneOpenAIOverrideValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, nested := range typed {
			cloned[key] = cloneOpenAIOverrideValue(nested)
		}
		return cloned
	case []any:
		cloned := make([]any, len(typed))
		for i, nested := range typed {
			cloned[i] = cloneOpenAIOverrideValue(nested)
		}
		return cloned
	default:
		return typed
	}
}
