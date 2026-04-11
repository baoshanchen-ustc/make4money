package admin

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// parseOptionalID parses an int64 query parameter, returning nil when empty.
func parseOptionalID(raw string) (*int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// parsePositiveOptionalID parses an int64 query parameter and rejects non-positive values.
func parsePositiveOptionalID(raw string) (*int64, error) {
	id, err := parseOptionalID(raw)
	if err != nil {
		return nil, err
	}
	if id != nil && *id <= 0 {
		return nil, errors.New("id must be positive")
	}
	return id, nil
}

func parseOpsAPIKeyAndGroupID(apiKeyRaw, groupRaw string) (apiKeyID, groupID *int64, invalidField string, err error) {
	apiKeyID, err = parsePositiveOptionalID(apiKeyRaw)
	if err != nil {
		return nil, nil, "api_key_id", err
	}
	groupID, err = parsePositiveOptionalID(groupRaw)
	if err != nil {
		return nil, nil, "group_id", err
	}
	return apiKeyID, groupID, "", nil
}

// applyOptionalFilters checks an API key or group ID against the log's extra fields.
func applyOptionalFilters(logExtra map[string]any, key string, target *int64) bool {
	if target == nil {
		return true
	}
	if logExtra == nil {
		return false
	}
	value, ok := logExtra[key]
	if !ok {
		return false
	}
	parsed, ok := coerceInt64(value)
	if !ok {
		return false
	}
	return parsed == *target
}

// opsSearchHint builds a unified hint payload for admin views.
func opsSearchHint(endpoint, detailEndpoint, note string) gin.H {
	return gin.H{
		"endpoint":                 endpoint,
		"detail_endpoint_template": detailEndpoint,
		"note":                     note,
	}
}

func attachOpsSearchLastDetailEndpoint(hint gin.H, requestID string) gin.H {
	if hint == nil {
		return nil
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return hint
	}
	template, _ := hint["detail_endpoint_template"].(string)
	if template == "" {
		return hint
	}
	hint["last_detail_endpoint"] = strings.Replace(template, ":request_id", requestID, 1)
	return hint
}
