package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/stretchr/testify/require"
)

// ========== Header normalization ==========

func TestNormalizeForAnthropicPersona_RenamesRateLimit(t *testing.T) {
	t.Parallel()
	dst := http.Header{}
	dst.Set("X-Ratelimit-Limit-Requests", "1000")
	dst.Set("X-Ratelimit-Remaining-Tokens", "5000")
	dst.Set("X-Ratelimit-Reset-Tokens", "60")

	responseheaders.NormalizeForAnthropicPersona(dst)

	require.Empty(t, dst.Get("X-Ratelimit-Limit-Requests"))
	require.Empty(t, dst.Get("X-Ratelimit-Remaining-Tokens"))
	require.Empty(t, dst.Get("X-Ratelimit-Reset-Tokens"))
	require.Equal(t, "1000", dst.Get("Anthropic-Ratelimit-Requests-Limit"))
	require.Equal(t, "5000", dst.Get("Anthropic-Ratelimit-Tokens-Remaining"))
	require.Equal(t, "60", dst.Get("Anthropic-Ratelimit-Tokens-Reset"))
}

func TestNormalizeForAnthropicPersona_RenamesXRequestID(t *testing.T) {
	t.Parallel()
	dst := http.Header{}
	dst.Set("X-Request-Id", "req-123")

	responseheaders.NormalizeForAnthropicPersona(dst)

	require.Empty(t, dst.Get("X-Request-Id"))
	require.Equal(t, "req-123", dst.Get("Request-Id"))
}

func TestNormalizeForAnthropicPersona_StripsVendorHeaders(t *testing.T) {
	t.Parallel()
	dst := http.Header{}
	dst.Set("OpenAI-Version", "2024-01-01")
	dst.Set("Openai-Organization", "org-x")
	dst.Set("X-Goog-Api-Key", "secret")
	dst.Set("X-Amzn-RequestId", "amz-id")
	dst.Set("Server", "envoy/1.0")
	dst.Set("Via", "1.1 vegur")
	dst.Set("Content-Type", "application/json") // should remain

	responseheaders.NormalizeForAnthropicPersona(dst)

	require.Empty(t, dst.Get("OpenAI-Version"))
	require.Empty(t, dst.Get("Openai-Organization"))
	require.Empty(t, dst.Get("X-Goog-Api-Key"))
	require.Empty(t, dst.Get("X-Amzn-RequestId"))
	require.Empty(t, dst.Get("Server"))
	require.Empty(t, dst.Get("Via"))
	require.Equal(t, "application/json", dst.Get("Content-Type"))
}

func TestNormalizeForAnthropicPersona_Idempotent(t *testing.T) {
	t.Parallel()
	dst := http.Header{}
	dst.Set("X-Ratelimit-Limit-Requests", "100")
	dst.Set("X-Request-Id", "abc")

	responseheaders.NormalizeForAnthropicPersona(dst)
	responseheaders.NormalizeForAnthropicPersona(dst) // 二次调用应是 no-op

	require.Equal(t, "100", dst.Get("Anthropic-Ratelimit-Requests-Limit"))
	require.Equal(t, "abc", dst.Get("Request-Id"))
	require.Empty(t, dst.Get("X-Ratelimit-Limit-Requests"))
	require.Empty(t, dst.Get("X-Request-Id"))
}

// ========== Response ID rewrite ==========

func TestConvertUpstreamIDToAnthropic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input        string
		expectChange bool
		mustPrefix   string
	}{
		{"resp_abc123def", true, "msg_"},
		{"chatcmpl-xyz789", true, "msg_"},
		{"cmpl-foo", true, "msg_"},
		{"msg_already", false, ""},        // already Anthropic
		{"plainhash", false, ""},           // no prefix indicator
		{"unknown_prefix_id", true, "msg_"}, // contains _ → wrap
	}

	for _, tt := range tests {
		got, changed := convertUpstreamIDToAnthropic(tt.input)
		require.Equal(t, tt.expectChange, changed, "input=%q", tt.input)
		if changed {
			require.True(t, strings.HasPrefix(got, tt.mustPrefix), "want prefix %s, got %s", tt.mustPrefix, got)
		}
	}
}

func TestRewriteResponseIDForPersona(t *testing.T) {
	t.Parallel()

	body := []byte(`{"id":"resp_abc123","model":"x","content":[]}`)
	out := rewriteResponseIDForPersona(body)
	require.Contains(t, string(out), `"id":"msg_abc123"`)
	require.NotContains(t, string(out), `"id":"resp_abc123"`)

	// 已经是 msg_ 前缀：保持
	body2 := []byte(`{"id":"msg_existing","model":"x"}`)
	out2 := rewriteResponseIDForPersona(body2)
	require.Equal(t, string(body2), string(out2))

	// 无 id 字段：原样返回
	body3 := []byte(`{"model":"x"}`)
	out3 := rewriteResponseIDForPersona(body3)
	require.Equal(t, string(body3), string(out3))
}

func TestRewriteAnthropicSSEMessageStartID(t *testing.T) {
	t.Parallel()

	dataLine := `{"type":"message_start","message":{"id":"resp_abc","model":"claude-opus","content":[]}}`
	out, changed := rewriteAnthropicSSEMessageStartID("message_start", dataLine)
	require.True(t, changed)
	require.Contains(t, out, `"id":"msg_abc"`)
	require.NotContains(t, out, `"id":"resp_abc"`)

	// 非 message_start 事件：跳过
	other, changed := rewriteAnthropicSSEMessageStartID("content_block_delta", `{"type":"content_block_delta"}`)
	require.False(t, changed)
	require.Equal(t, `{"type":"content_block_delta"}`, other)

	// 已是 msg_ 前缀：跳过
	already := `{"type":"message_start","message":{"id":"msg_existing"}}`
	out2, changed := rewriteAnthropicSSEMessageStartID("message_start", already)
	require.False(t, changed)
	require.Equal(t, already, out2)
}

// ========== Vendor name scrubbing ==========

func TestScrubVendorNames_BasicReplacements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		mustNot  []string // 替换后不应包含的子串
	}{
		{"OpenAI moderation rejected this request.", []string{"OpenAI"}},
		{"Failed to call ChatGPT api.", []string{"ChatGPT"}},
		{"GPT-5 returned an error.", []string{"GPT-5", "GPT"}},
		{"Gemini 2.5 Pro is not available.", []string{"Gemini"}},
		{"AWS Bedrock guardrail blocked content.", []string{"AWS", "Bedrock"}},
		{"Google Vertex AI quota exceeded.", []string{"Google", "Vertex AI"}},
		{"Llama-3.1 model not found.", []string{"Llama"}},
		{"DeepSeek timed out.", []string{"DeepSeek"}},
		{"Qwen rate limit exceeded.", []string{"Qwen"}},
	}

	for _, tt := range tests {
		out := scrubVendorNames(tt.input)
		for _, banned := range tt.mustNot {
			require.NotContains(t, out, banned, "input=%q output=%q", tt.input, out)
		}
	}
}

func TestScrubVendorNames_StripsURLs(t *testing.T) {
	t.Parallel()
	out := scrubVendorNames("Visit https://api.openai.com/v1/usage for details.")
	require.NotContains(t, out, "openai.com")
	require.Contains(t, out, "[upstream-url]")

	out2 := scrubVendorNames("see https://generativelanguage.googleapis.com/foo")
	require.NotContains(t, out2, "googleapis.com")
}

func TestScrubVendorNames_PreservesAnthropicAndCommonWords(t *testing.T) {
	t.Parallel()
	in := "Claude Code encountered an error while talking to the Anthropic API."
	out := scrubVendorNames(in)
	require.Contains(t, out, "Claude Code")
	require.Contains(t, out, "Anthropic") // we WANT to keep Anthropic identity intact
}

func TestScrubVendorNames_EmptyAndNoMatch(t *testing.T) {
	t.Parallel()
	require.Empty(t, scrubVendorNames(""))
	require.Equal(t, "Generic message with no vendor names.",
		scrubVendorNames("Generic message with no vendor names."))
}

// ========== Wrapper integration ==========

func TestWritePersonaAwareFilteredHeaders_Off(t *testing.T) {
	t.Parallel()
	src := http.Header{}
	src.Set("X-Ratelimit-Limit-Requests", "100")
	src.Set("X-Request-Id", "abc")
	dst := http.Header{}

	filter := responseheaders.CompileHeaderFilter(config.ResponseHeaderConfig{})

	WritePersonaAwareFilteredHeaders(dst, src, filter, false)

	require.Equal(t, "100", dst.Get("X-Ratelimit-Limit-Requests"))
	require.Equal(t, "abc", dst.Get("X-Request-Id"))
	require.Empty(t, dst.Get("Anthropic-Ratelimit-Requests-Limit"))
}

func TestWritePersonaAwareFilteredHeaders_On(t *testing.T) {
	t.Parallel()
	src := http.Header{}
	src.Set("X-Ratelimit-Limit-Requests", "100")
	src.Set("X-Request-Id", "abc")
	dst := http.Header{}

	WritePersonaAwareFilteredHeaders(dst, src, nil, true)

	require.Empty(t, dst.Get("X-Ratelimit-Limit-Requests"))
	require.Empty(t, dst.Get("X-Request-Id"))
	require.Equal(t, "100", dst.Get("Anthropic-Ratelimit-Requests-Limit"))
	require.Equal(t, "abc", dst.Get("Request-Id"))
}
