package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResponsesRequestToChatCompletions_ReasoningAttachedToAssistant verifies
// that a Responses input containing a reasoning item before an assistant
// message produces a Chat Completions assistant message with reasoning_content
// populated. This is the contract DeepSeek thinking mode requires.
func TestResponsesRequestToChatCompletions_ReasoningAttachedToAssistant(t *testing.T) {
	items := []ResponsesInputItem{
		{Role: "user", Content: json.RawMessage(`"hi"`)},
		{Type: "reasoning", Summary: []ResponsesSummary{{Type: "summary_text", Text: "step-by-step analysis"}}},
		{Role: "assistant", Content: json.RawMessage(`[{"type":"output_text","text":"hello"}]`)},
		{Role: "user", Content: json.RawMessage(`"and now?"`)},
	}
	inputJSON, err := json.Marshal(items)
	require.NoError(t, err)

	chat, err := ResponsesRequestToChatCompletions(&ResponsesRequest{
		Model: "deepseek-reasoner",
		Input: inputJSON,
	})
	require.NoError(t, err)
	require.Len(t, chat.Messages, 3)

	assert.Equal(t, "user", chat.Messages[0].Role)
	assert.Equal(t, "assistant", chat.Messages[1].Role)
	assert.Equal(t, "step-by-step analysis", chat.Messages[1].ReasoningContent,
		"reasoning item must be attached to the assistant message it belongs to")
	assert.Equal(t, "user", chat.Messages[2].Role)
}

// TestResponsesRequestToChatCompletions_ReasoningOnlyToolCallTurn covers an
// assistant turn that has reasoning + tool_use only (no text). The reasoning
// must still ride on the assistant message that holds the tool_calls.
func TestResponsesRequestToChatCompletions_ReasoningOnlyToolCallTurn(t *testing.T) {
	items := []ResponsesInputItem{
		{Role: "user", Content: json.RawMessage(`"call the tool"`)},
		{Type: "reasoning", Summary: []ResponsesSummary{{Type: "summary_text", Text: "plan: call get_x"}}},
		{Type: "function_call", CallID: "fc_call_1", Name: "get_x", Arguments: `{"q":"a"}`},
		{Type: "function_call_output", CallID: "fc_call_1", Output: "ok"},
	}
	inputJSON, err := json.Marshal(items)
	require.NoError(t, err)

	chat, err := ResponsesRequestToChatCompletions(&ResponsesRequest{
		Model: "deepseek-reasoner",
		Input: inputJSON,
	})
	require.NoError(t, err)
	require.Len(t, chat.Messages, 3)

	assert.Equal(t, "user", chat.Messages[0].Role)
	assert.Equal(t, "assistant", chat.Messages[1].Role)
	require.Len(t, chat.Messages[1].ToolCalls, 1)
	assert.Equal(t, "get_x", chat.Messages[1].ToolCalls[0].Function.Name)
	assert.Equal(t, "plan: call get_x", chat.Messages[1].ReasoningContent)
	assert.Equal(t, "tool", chat.Messages[2].Role)
	assert.Equal(t, "fc_call_1", chat.Messages[2].ToolCallID)
}

// TestResponsesRequestToChatCompletions_DanglingReasoningDropped guards against
// a stray reasoning item that is not followed by an assistant turn — it should
// not leak onto a subsequent user/system message.
func TestResponsesRequestToChatCompletions_DanglingReasoningDropped(t *testing.T) {
	items := []ResponsesInputItem{
		{Type: "reasoning", Summary: []ResponsesSummary{{Type: "summary_text", Text: "orphan"}}},
		{Role: "user", Content: json.RawMessage(`"hi"`)},
	}
	inputJSON, err := json.Marshal(items)
	require.NoError(t, err)

	chat, err := ResponsesRequestToChatCompletions(&ResponsesRequest{
		Model: "deepseek-reasoner",
		Input: inputJSON,
	})
	require.NoError(t, err)
	require.Len(t, chat.Messages, 1)
	assert.Equal(t, "user", chat.Messages[0].Role)
	assert.Empty(t, chat.Messages[0].ReasoningContent)
}

// TestAnthropicEmptyThinkingBlockStillEmitsReasoningContent guards the case
// where the client echoes back an empty thinking block (text="", possibly
// with a placeholder signature) — DeepSeek thinking mode still rejects the
// turn unless reasoning_content is present. The converter must emit a non-
// empty placeholder reasoning_content for that turn.
func TestAnthropicEmptyThinkingBlockStillEmitsReasoningContent(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "deepseek-v4-pro",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"first"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"thinking","thinking":"","signature":"xxxxxxxxxx"},{"type":"text","text":"answer"}]`)},
			{Role: "user", Content: json.RawMessage(`"again"`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	chat, err := ResponsesRequestToChatCompletions(resp)
	require.NoError(t, err)

	require.Len(t, chat.Messages, 3)
	assert.Equal(t, "assistant", chat.Messages[1].Role)
	assert.NotEmpty(t, chat.Messages[1].ReasoningContent,
		"empty thinking block must still produce non-empty reasoning_content placeholder for DeepSeek thinking mode")
}

// TestAnthropicThinkingRoundTripsToReasoningContent is the end-to-end
// regression for the DeepSeek "reasoning_content must be passed back" 400:
// Anthropic /v1/messages with a prior assistant turn containing a thinking
// block must, after Anthropic→Responses→ChatCompletions, end up with
// reasoning_content on the corresponding assistant message.
func TestAnthropicThinkingRoundTripsToReasoningContent(t *testing.T) {
	req := &AnthropicRequest{
		Model:     "deepseek-chat[thinking]",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"first turn"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"thinking","thinking":"prior reasoning"},{"type":"text","text":"prior answer"}]`)},
			{Role: "user", Content: json.RawMessage(`"follow up"`)},
		},
	}

	resp, err := AnthropicToResponses(req)
	require.NoError(t, err)

	chat, err := ResponsesRequestToChatCompletions(resp)
	require.NoError(t, err)

	require.Len(t, chat.Messages, 3, "expect: user, assistant(text+reasoning), user")
	assert.Equal(t, "user", chat.Messages[0].Role)
	assert.Equal(t, "assistant", chat.Messages[1].Role)
	assert.Equal(t, "prior reasoning", chat.Messages[1].ReasoningContent,
		"DeepSeek thinking mode requires the previous reasoning_content to be echoed back")
	var assistantText string
	require.NoError(t, json.Unmarshal(chat.Messages[1].Content, &assistantText))
	assert.Equal(t, "prior answer", assistantText)
	assert.Equal(t, "user", chat.Messages[2].Role)
}
