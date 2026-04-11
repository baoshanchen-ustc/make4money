package admin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOptionalID(t *testing.T) {
	value, err := parseOptionalID("")
	require.NoError(t, err)
	require.Nil(t, value)

	value, err = parseOptionalID("42")
	require.NoError(t, err)
	require.NotNil(t, value)
	require.Equal(t, int64(42), *value)

	value, err = parseOptionalID("bad")
	require.Error(t, err)
	require.Nil(t, value)
}

func TestParsePositiveOptionalID(t *testing.T) {
	value, err := parsePositiveOptionalID("")
	require.NoError(t, err)
	require.Nil(t, value)

	value, err = parsePositiveOptionalID("42")
	require.NoError(t, err)
	require.NotNil(t, value)
	require.Equal(t, int64(42), *value)

	value, err = parsePositiveOptionalID("0")
	require.Error(t, err)
	require.Nil(t, value)

	value, err = parsePositiveOptionalID("-7")
	require.Error(t, err)
	require.Nil(t, value)

	value, err = parsePositiveOptionalID("bad")
	require.Error(t, err)
	require.Nil(t, value)
}

func TestParseOpsAPIKeyAndGroupID(t *testing.T) {
	apiKeyID, groupID, invalidField, err := parseOpsAPIKeyAndGroupID("42", "7")
	require.NoError(t, err)
	require.Equal(t, "", invalidField)
	require.NotNil(t, apiKeyID)
	require.NotNil(t, groupID)
	require.Equal(t, int64(42), *apiKeyID)
	require.Equal(t, int64(7), *groupID)

	apiKeyID, groupID, invalidField, err = parseOpsAPIKeyAndGroupID("0", "7")
	require.Error(t, err)
	require.Equal(t, "api_key_id", invalidField)
	require.Nil(t, apiKeyID)
	require.Nil(t, groupID)

	apiKeyID, groupID, invalidField, err = parseOpsAPIKeyAndGroupID("42", "-3")
	require.Error(t, err)
	require.Equal(t, "group_id", invalidField)
	require.Nil(t, apiKeyID)
	require.Nil(t, groupID)
}

func TestApplyOptionalFilters(t *testing.T) {
	target := int64(7)
	require.True(t, applyOptionalFilters(nil, "api_key_id", nil))
	require.False(t, applyOptionalFilters(nil, "api_key_id", &target))
	require.False(t, applyOptionalFilters(map[string]any{"api_key_id": "bad"}, "api_key_id", &target))
	require.False(t, applyOptionalFilters(map[string]any{"api_key_id": int64(8)}, "api_key_id", &target))
	require.True(t, applyOptionalFilters(map[string]any{"api_key_id": int64(7)}, "api_key_id", &target))
}

func TestOpsSearchHint(t *testing.T) {
	hint := opsSearchHint("/list", "/detail/:request_id", "note")
	require.Equal(t, "/list", hint["endpoint"])
	require.Equal(t, "/detail/:request_id", hint["detail_endpoint_template"])
	require.Equal(t, "note", hint["note"])
}

func TestAttachOpsSearchLastDetailEndpoint(t *testing.T) {
	hint := opsSearchHint("/list", "/detail/:request_id", "note")
	hint = attachOpsSearchLastDetailEndpoint(hint, "req-123")
	require.Equal(t, "/detail/req-123", hint["last_detail_endpoint"])

	unchanged := attachOpsSearchLastDetailEndpoint(opsSearchHint("/list", "", "note"), "req-123")
	_, ok := unchanged["last_detail_endpoint"]
	require.False(t, ok)
}
