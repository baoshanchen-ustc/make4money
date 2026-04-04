package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestParseRawAPIKeyImportLines(t *testing.T) {
	total, lines, results, err := parseRawAPIKeyImportLines(`
# comment
sk-proj-123
sk-ant-456,https://api.anthropic.com
AIzaSy789
bad-key
`)
	require.NoError(t, err)
	require.Equal(t, 4, total)
	require.Len(t, lines, 3)
	require.Len(t, results, 1)
	require.Equal(t, service.PlatformOpenAI, lines[0].Platform)
	require.Equal(t, service.PlatformAnthropic, lines[1].Platform)
	require.Equal(t, service.PlatformGemini, lines[2].Platform)
	require.Contains(t, results[0].Error, "could not detect platform")
}

func TestBuildAPIKeyIdentityUsesDefaultBaseURL(t *testing.T) {
	a := buildAPIKeyIdentity(service.PlatformOpenAI, "sk-proj-1", "")
	b := buildAPIKeyIdentity(service.PlatformOpenAI, "sk-proj-1", "https://api.openai.com/")
	require.Equal(t, a, b)
}
