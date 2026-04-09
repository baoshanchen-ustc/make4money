package openai

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultInstructions_IsFocusedClaudeCompatContract(t *testing.T) {
	text := strings.TrimSpace(DefaultInstructions)
	require.NotEmpty(t, text)

	words := len(strings.Fields(text))
	require.LessOrEqual(t, words, 1000, "default instructions should stay concise")

	require.Contains(t, text, "<communication_contract>")
	require.Contains(t, text, "<explanation_contract>")
	require.Contains(t, text, "<troubleshooting_contract>")
	require.Contains(t, text, "<research_contract>")
	require.Contains(t, text, "<tool_routing_contract>")
	require.Contains(t, text, "<control_plane_contract>")
	require.Contains(t, text, "Use natural, easy-to-follow language")
	require.Contains(t, text, "Lead with the answer or recommendation")
	require.Contains(t, text, "Prefer short paragraphs by default")
	require.Contains(t, text, "Do not default to numbered troubleshooting runbooks")
	require.Contains(t, text, "Explain things the way a capable teammate would in chat")
	require.Contains(t, text, "start with the likely diagnosis")
	require.Contains(t, text, "Do not invoke agent, explore, or team-style delegation")
	require.Contains(t, text, "Do not assume a default team exists")

	require.NotContains(t, text, "approval_policy")
	require.NotContains(t, text, "sandbox_mode")
	require.NotContains(t, text, "## Frontend tasks")
	require.NotContains(t, text, "## Codex CLI harness, sandboxing, and approvals")
	require.NotContains(t, text, "Stay concise, direct, and factual.")
}
