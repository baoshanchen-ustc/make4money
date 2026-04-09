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

	require.Contains(t, text, "<research_contract>")
	require.Contains(t, text, "<tool_routing_contract>")
	require.Contains(t, text, "<control_plane_contract>")

	require.NotContains(t, text, "approval_policy")
	require.NotContains(t, text, "sandbox_mode")
	require.NotContains(t, text, "## Frontend tasks")
	require.NotContains(t, text, "## Codex CLI harness, sandboxing, and approvals")
}
