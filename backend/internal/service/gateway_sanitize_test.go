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

	plainTool := requireMap(t, tools[0])
	plain := requireMap(t, plainTool["input_schema"])
	require.Contains(t, plain, "properties")
	require.Equal(t, map[string]any{}, plain["properties"])

	nestedTool := requireMap(t, tools[1])
	nestedSchema := requireMap(t, nestedTool["input_schema"])
	nestedProperties := requireMap(t, nestedSchema["properties"])
	nestedChild := requireMap(t, nestedProperties["child"])
	require.Contains(t, nestedChild, "properties")
	require.Equal(t, map[string]any{}, nestedChild["properties"])

	customTool := requireMap(t, tools[2])
	customPayload := requireMap(t, customTool["custom"])
	custom := requireMap(t, customPayload["input_schema"])
	require.Contains(t, custom, "properties")
	require.Equal(t, map[string]any{}, custom["properties"])
}

func TestNormalizeAnthropicToolSchemas_NoChangeForNonObjectSchemas(t *testing.T) {
	body := []byte(`{"tools":[{"name":"plain","input_schema":{"type":"string"}}]}`)
	got := normalizeAnthropicToolSchemas(body)
	require.JSONEq(t, string(body), string(got))
}

func requireMap(t *testing.T, value any) map[string]any {
	t.Helper()
	out, ok := value.(map[string]any)
	require.True(t, ok)
	return out
}
