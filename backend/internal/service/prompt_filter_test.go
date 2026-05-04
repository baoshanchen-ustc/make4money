package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptTextFromJSONExtractsCommonPromptShapes(t *testing.T) {
	body := []byte(`{
		"instructions": "system note",
		"messages": [
			{"role": "user", "content": "hello forbidden"},
			{"role": "user", "content": [{"type": "text", "text": "gemini text"}]}
		],
		"contents": [{"parts": [{"text": "native gemini"}]}]
	}`)

	text := PromptTextFromJSON(body)
	require.Contains(t, text, "system note")
	require.Contains(t, text, "hello forbidden")
	require.Contains(t, text, "gemini text")
	require.Contains(t, text, "native gemini")
}

func TestPromptFilterContainsKeywordIsCaseInsensitive(t *testing.T) {
	require.True(t, PromptFilterContainsKeyword("please BLOCK this", []string{"block"}))
	require.True(t, PromptFilterContainsKeyword("请拦截这个词", []string{"拦截"}))
	require.False(t, PromptFilterContainsKeyword("clean prompt", []string{"blocked"}))
}

func TestParsePromptFilterRuntimeDefaults(t *testing.T) {
	runtime := ParsePromptFilterRuntime(map[string]string{
		SettingKeyPromptFilterEnabled:        "true",
		SettingKeyPromptFilterKeywords:       `[" One ","one","Two"]`,
		SettingKeyPromptFilterViolationLimit: "0",
	})

	require.True(t, runtime.Enabled)
	require.Equal(t, []string{"One", "Two"}, runtime.Keywords)
	require.Equal(t, DefaultPromptFilterViolationLimit, runtime.ViolationLimit)
	require.Equal(t, DefaultPromptFilterWarningMessage, runtime.WarningMessage)
	require.Equal(t, DefaultPromptFilterBanMessage, runtime.BanMessage)
}
