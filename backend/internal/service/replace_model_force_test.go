package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 验证 force=true 时上游回声任意名都被重写为 originalModel；
// force=false 时维持「精确匹配 fromModel」语义。
func TestGatewayService_ReplaceModelInResponseBody_ForceMode(t *testing.T) {
	t.Parallel()
	svc := &GatewayService{}

	t.Run("force=true rewrites any model value", func(t *testing.T) {
		body := []byte(`{"id":"msg_1","model":"gpt-5-codex","content":[]}`)
		out := svc.replaceModelInResponseBody(body, "claude-opus-4-5", "claude-opus-4-5", true)
		require.Contains(t, string(out), `"model":"claude-opus-4-5"`)
		require.NotContains(t, string(out), "gpt-5-codex")
	})

	t.Run("force=false strict match preserves non-matching", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5-codex"}`)
		out := svc.replaceModelInResponseBody(body, "claude-opus-4-5", "claude-opus-4-5", false)
		require.Contains(t, string(out), "gpt-5-codex")
	})

	t.Run("force=true still skips when no model field", func(t *testing.T) {
		body := []byte(`{"id":"msg_1"}`)
		out := svc.replaceModelInResponseBody(body, "x", "y", true)
		require.Equal(t, string(body), string(out))
	})

	t.Run("force=true is idempotent", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5"}`)
		once := svc.replaceModelInResponseBody(body, "ignored", "claude-opus-4-5", true)
		twice := svc.replaceModelInResponseBody(once, "ignored", "claude-opus-4-5", true)
		require.Equal(t, string(once), string(twice))
		require.Contains(t, string(twice), `"model":"claude-opus-4-5"`)
	})
}

func TestOpenAIGatewayService_ReplaceModelInResponseBody_ForceMode(t *testing.T) {
	t.Parallel()
	svc := &OpenAIGatewayService{}

	body := []byte(`{"id":"resp_1","model":"gpt-5-codex","output":[]}`)
	out := svc.replaceModelInResponseBody(body, "irrelevant", "claude-sonnet-4-5", true)
	require.Contains(t, string(out), `"model":"claude-sonnet-4-5"`)
	require.NotContains(t, string(out), "gpt-5-codex")

	// non-force keeps mismatch
	out2 := svc.replaceModelInResponseBody(body, "irrelevant", "claude-sonnet-4-5", false)
	require.Contains(t, string(out2), "gpt-5-codex")
}

func TestOpenAIGatewayService_ReplaceModelInSSELine_ForceMode(t *testing.T) {
	t.Parallel()
	svc := &OpenAIGatewayService{}

	t.Run("rewrites top-level model regardless of from", func(t *testing.T) {
		line := `data: {"id":"chatcmpl-1","model":"gpt-5","choices":[]}`
		out := svc.replaceModelInSSELine(line, "irrelevant", "claude-haiku-4-5", true)
		require.Contains(t, out, `"model":"claude-haiku-4-5"`)
		require.NotContains(t, out, "gpt-5")
	})

	t.Run("rewrites nested response.model", func(t *testing.T) {
		line := `data: {"type":"response.completed","response":{"model":"gpt-5","id":"r"}}`
		out := svc.replaceModelInSSELine(line, "irrelevant", "claude-opus-4-5", true)
		require.Contains(t, out, `"model":"claude-opus-4-5"`)
		require.NotContains(t, out, `"model":"gpt-5"`)
	})

	t.Run("force=false ignores mismatched", func(t *testing.T) {
		line := `data: {"model":"gpt-5"}`
		out := svc.replaceModelInSSELine(line, "claude-opus-4-5", "claude-opus-4-5", false)
		require.Equal(t, line, out)
	})
}

// TestReplaceModelInResponseBody_ContextSuffixTolerance 验证 [1m] 容错匹配。
// 场景：mappedModel 因 ResolveMappedModel 拼回了 [1m]（如 deepseek-v4-pro[1m]），
// 但上游 DeepSeek 响应里的 model 字段是不带后缀的 deepseek-v4-pro。
// force=false 时也应能识别为同一模型并改写，避免 deepseek 字样泄露给客户端。
func TestReplaceModelInResponseBody_ContextSuffixTolerance(t *testing.T) {
	t.Parallel()

	t.Run("GatewayService: fromModel 带后缀，body 无后缀", func(t *testing.T) {
		svc := &GatewayService{}
		body := []byte(`{"id":"msg_1","model":"deepseek-v4-pro","content":[]}`)
		out := svc.replaceModelInResponseBody(body, "deepseek-v4-pro[1m]", "claude-opus-4-7[1m]", false)
		require.Contains(t, string(out), `"model":"claude-opus-4-7[1m]"`)
		require.NotContains(t, string(out), "deepseek-v4-pro")
	})

	t.Run("GatewayService: fromModel 无后缀，body 带后缀", func(t *testing.T) {
		svc := &GatewayService{}
		body := []byte(`{"id":"msg_1","model":"deepseek-v4-pro[1m]","content":[]}`)
		out := svc.replaceModelInResponseBody(body, "deepseek-v4-pro", "claude-opus-4-7", false)
		require.Contains(t, string(out), `"model":"claude-opus-4-7"`)
		require.NotContains(t, string(out), "deepseek-v4-pro")
	})

	t.Run("OpenAIGatewayService: fromModel 带后缀，body 无后缀", func(t *testing.T) {
		svc := &OpenAIGatewayService{}
		body := []byte(`{"id":"resp_1","model":"deepseek-v4-pro","output":[]}`)
		out := svc.replaceModelInResponseBody(body, "deepseek-v4-pro[1m]", "claude-opus-4-7[1m]", false)
		require.Contains(t, string(out), `"model":"claude-opus-4-7[1m]"`)
		require.NotContains(t, string(out), "deepseek-v4-pro")
	})

	t.Run("SSE: fromModel 带后缀，line 无后缀", func(t *testing.T) {
		svc := &OpenAIGatewayService{}
		line := `data: {"id":"chatcmpl-1","model":"deepseek-v4-pro","choices":[]}`
		out := svc.replaceModelInSSELine(line, "deepseek-v4-pro[1m]", "claude-opus-4-7[1m]", false)
		require.Contains(t, out, `"model":"claude-opus-4-7[1m]"`)
		require.NotContains(t, out, "deepseek-v4-pro")
	})

	t.Run("SSE: 嵌套 response.model 带后缀容错", func(t *testing.T) {
		svc := &OpenAIGatewayService{}
		line := `data: {"type":"response.completed","response":{"model":"deepseek-v4-pro","id":"r"}}`
		out := svc.replaceModelInSSELine(line, "deepseek-v4-pro[1m]", "claude-opus-4-7[1m]", false)
		require.Contains(t, out, `"model":"claude-opus-4-7[1m]"`)
		require.NotContains(t, out, `"model":"deepseek-v4-pro"`)
	})

	t.Run("不相关模型不应被误改写", func(t *testing.T) {
		svc := &GatewayService{}
		body := []byte(`{"id":"msg_1","model":"gpt-5-codex","content":[]}`)
		out := svc.replaceModelInResponseBody(body, "deepseek-v4-pro[1m]", "claude-opus-4-7[1m]", false)
		require.Contains(t, string(out), `"model":"gpt-5-codex"`)
	})
}

func TestReplaceOpenAIWSMessageModel_ForceMode(t *testing.T) {
	t.Parallel()

	t.Run("force=true rewrites foreign upstream name", func(t *testing.T) {
		msg := []byte(`{"type":"response.created","model":"gpt-5"}`)
		out := replaceOpenAIWSMessageModel(msg, "irrelevant", "claude-opus-4-5", true)
		require.Contains(t, string(out), `"model":"claude-opus-4-5"`)
	})

	t.Run("force=true rewrites nested response.model", func(t *testing.T) {
		msg := []byte(`{"type":"response.completed","response":{"id":"r","model":"gpt-5"}}`)
		out := replaceOpenAIWSMessageModel(msg, "irrelevant", "claude-opus-4-5", true)
		require.Contains(t, string(out), `"response":{"id":"r","model":"claude-opus-4-5"}`)
	})

	t.Run("force=false keeps strict semantics", func(t *testing.T) {
		msg := []byte(`{"type":"response.created","model":"gpt-5"}`)
		out := replaceOpenAIWSMessageModel(msg, "claude-opus-4-5", "claude-haiku-4-5", false)
		require.Equal(t, string(msg), string(out))
	})

	t.Run("force=true is idempotent", func(t *testing.T) {
		msg := []byte(`{"type":"response.created","model":"gpt-5"}`)
		once := replaceOpenAIWSMessageModel(msg, "x", "claude-opus-4-5", true)
		twice := replaceOpenAIWSMessageModel(once, "x", "claude-opus-4-5", true)
		require.Equal(t, string(once), string(twice))
	})
}

func TestClaudeCodePersonaContextHelpers(t *testing.T) {
	t.Parallel()

	require.False(t, IsClaudeCodePersonaForced(nil, nil))

	// nil ctx + enabled=true: returned ctx is still nil; IsClaudeCodePersonaForced reads false
	require.Nil(t, WithClaudeCodePersona(nil, true))

	// non-nil ctx
	enabled := WithClaudeCodePersona(context.Background(), true)
	require.True(t, IsClaudeCodePersonaForced(enabled, nil))

	// disabled flag is a no-op (returns the original ctx)
	disabled := WithClaudeCodePersona(context.Background(), false)
	require.False(t, IsClaudeCodePersonaForced(disabled, nil))
}
