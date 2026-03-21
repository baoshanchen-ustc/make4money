package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeOpenCodeText_RewritesCanonicalSentence(t *testing.T) {
	in := "You are OpenCode, the best coding agent on the planet."
	got := sanitizeSystemText(in)
	require.Equal(t, strings.TrimSpace(claudeCodeSystemPrompt), got)
}

func TestNormalizeAnthropicToolSchemas_AddsMissingObjectProperties(t *testing.T) {
	body := []byte(`{
		"tools":[
			{"name":"plain","input_schema":{"type":"object"}},
			{"name":"nested","input_schema":{"type":"object","properties":{"child":{"type":"object"}}}},
			{"name":"custom","custom":{"input_schema":{"type":"object"}}}
		]
	}`)

	got := normalizeAnthropicToolSchemas(body)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(got, &payload))

	tools, ok := payload["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 3)

	plain := tools[0].(map[string]any)["input_schema"].(map[string]any)
	require.Contains(t, plain, "properties")
	require.Equal(t, map[string]any{}, plain["properties"])

	nestedChild := tools[1].(map[string]any)["input_schema"].(map[string]any)["properties"].(map[string]any)["child"].(map[string]any)
	require.Contains(t, nestedChild, "properties")
	require.Equal(t, map[string]any{}, nestedChild["properties"])

	custom := tools[2].(map[string]any)["custom"].(map[string]any)["input_schema"].(map[string]any)
	require.Contains(t, custom, "properties")
	require.Equal(t, map[string]any{}, custom["properties"])
}

func TestNormalizeAnthropicToolSchemas_NoChangeForNonObjectSchemas(t *testing.T) {
	body := []byte(`{"tools":[{"name":"plain","input_schema":{"type":"string"}}]}`)
	got := normalizeAnthropicToolSchemas(body)
	require.JSONEq(t, string(body), string(got))
}
