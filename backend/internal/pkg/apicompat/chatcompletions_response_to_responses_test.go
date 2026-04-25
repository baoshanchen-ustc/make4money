package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatCompletionsResponseToResponses_MessageAndUsage(t *testing.T) {
	content, _ := json.Marshal("hello world")
	resp := &ChatCompletionsResponse{
		ID:    "chatcmpl-123",
		Model: "deepseek-v4-pro",
		Choices: []ChatChoice{{
			Index: 0,
			Message: ChatMessage{
				Role:    "assistant",
				Content: content,
			},
			FinishReason: "stop",
		}},
		Usage: &ChatUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			PromptTokensDetails: &ChatTokenDetails{
				CachedTokens: 30,
			},
		},
	}

	out := ChatCompletionsResponseToResponses(resp, "claude-opus-4-7")
	require.NotNil(t, out)
	require.Equal(t, "claude-opus-4-7", out.Model)
	require.Equal(t, "completed", out.Status)
	require.Len(t, out.Output, 1)
	require.Equal(t, "message", out.Output[0].Type)
	require.Equal(t, "assistant", out.Output[0].Role)
	require.Len(t, out.Output[0].Content, 1)
	require.Equal(t, "output_text", out.Output[0].Content[0].Type)
	require.Equal(t, "hello world", out.Output[0].Content[0].Text)
	require.NotNil(t, out.Usage)
	require.Equal(t, 100, out.Usage.InputTokens)
	require.Equal(t, 50, out.Usage.OutputTokens)
	require.Equal(t, 150, out.Usage.TotalTokens)
	require.NotNil(t, out.Usage.InputTokensDetails)
	require.Equal(t, 30, out.Usage.InputTokensDetails.CachedTokens)
}

func TestChatCompletionsResponseToResponses_ReasoningAndToolCalls(t *testing.T) {
	resp := &ChatCompletionsResponse{
		ID:    "chatcmpl-456",
		Model: "deepseek-v4-pro",
		Choices: []ChatChoice{{
			Index: 0,
			Message: ChatMessage{
				Role:             "assistant",
				ReasoningContent: "Let me think about this.",
				ToolCalls: []ChatToolCall{{
					ID:   "call_abc",
					Type: "function",
					Function: ChatFunctionCall{
						Name:      "get_weather",
						Arguments: `{"city":"SF"}`,
					},
				}},
			},
			FinishReason: "tool_calls",
		}},
	}

	out := ChatCompletionsResponseToResponses(resp, "claude-opus-4-7")
	require.NotNil(t, out)
	require.Len(t, out.Output, 2)
	require.Equal(t, "reasoning", out.Output[0].Type)
	require.Len(t, out.Output[0].Summary, 1)
	require.Equal(t, "Let me think about this.", out.Output[0].Summary[0].Text)
	require.Equal(t, "function_call", out.Output[1].Type)
	require.Equal(t, "call_abc", out.Output[1].CallID)
	require.Equal(t, "get_weather", out.Output[1].Name)
	require.JSONEq(t, `{"city":"SF"}`, out.Output[1].Arguments)
}

func TestChatCompletionsResponseToResponses_LengthFinishReason(t *testing.T) {
	resp := &ChatCompletionsResponse{
		ID: "chatcmpl-789",
		Choices: []ChatChoice{{
			Index: 0,
			Message: ChatMessage{
				Role:    "assistant",
				Content: json.RawMessage(`"partial output"`),
			},
			FinishReason: "length",
		}},
	}
	out := ChatCompletionsResponseToResponses(resp, "claude-sonnet-4-6")
	require.Equal(t, "incomplete", out.Status)
	require.NotNil(t, out.IncompleteDetails)
	require.Equal(t, "max_output_tokens", out.IncompleteDetails.Reason)
}

func TestChatChunkToResponsesEvents_StreamingTextAndFinish(t *testing.T) {
	state := NewChatToResponsesEventState()
	state.Model = "claude-opus-4-7"

	// Chunk 1: role = assistant (no content)
	chunk1 := ChatCompletionsChunk{
		ID:      "chatcmpl-stream-1",
		Object:  "chat.completion.chunk",
		Model:   "deepseek-v4-pro",
		Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{Role: "assistant"}}},
	}
	events := ChatChunkToResponsesEvents(chunk1, state)
	// Expect response.created event.
	require.True(t, len(events) >= 1)
	require.Equal(t, "response.created", events[0].Type)

	// Chunk 2: text delta
	text := "hi "
	chunk2 := ChatCompletionsChunk{
		ID:      "chatcmpl-stream-1",
		Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{Content: &text}}},
	}
	events = ChatChunkToResponsesEvents(chunk2, state)
	require.Len(t, events, 2) // output_item.added + output_text.delta
	require.Equal(t, "response.output_item.added", events[0].Type)
	require.Equal(t, "response.output_text.delta", events[1].Type)
	require.Equal(t, "hi ", events[1].Delta)

	// Chunk 3: more text
	text2 := "there"
	chunk3 := ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{Content: &text2}}},
	}
	events = ChatChunkToResponsesEvents(chunk3, state)
	require.Len(t, events, 1)
	require.Equal(t, "response.output_text.delta", events[0].Type)
	require.Equal(t, "there", events[0].Delta)

	// Chunk 4: finish reason
	stopReason := "stop"
	chunk4 := ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{}, FinishReason: &stopReason}},
	}
	events = ChatChunkToResponsesEvents(chunk4, state)
	require.Empty(t, events) // completion is deferred
	require.True(t, state.FinishSeen)
	require.Equal(t, "stop", state.FinishReason)

	// Finalize: produces response.completed
	final := FinalizeChatToResponsesStream(state)
	require.Len(t, final, 1)
	require.Equal(t, "response.completed", final[0].Type)
	require.NotNil(t, final[0].Response)
	require.Equal(t, "completed", final[0].Response.Status)

	// Idempotent
	require.Empty(t, FinalizeChatToResponsesStream(state))
}

func TestChatChunkToResponsesEvents_ReasoningAndToolCall(t *testing.T) {
	state := NewChatToResponsesEventState()

	// First: created + reasoning delta
	reasoning := "pondering..."
	chunk1 := ChatCompletionsChunk{
		ID: "x",
		Choices: []ChatChunkChoice{{
			Index: 0,
			Delta: ChatDelta{ReasoningContent: &reasoning},
		}},
	}
	events := ChatChunkToResponsesEvents(chunk1, state)
	// created + output_item.added (reasoning) + reasoning_summary_text.delta
	require.Len(t, events, 3)
	require.Equal(t, "response.created", events[0].Type)
	require.Equal(t, "response.output_item.added", events[1].Type)
	require.Equal(t, "reasoning", events[1].Item.Type)
	require.Equal(t, "response.reasoning_summary_text.delta", events[2].Type)

	// Tool call chunk
	idx := 0
	chunk2 := ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{
			Index: 0,
			Delta: ChatDelta{
				ToolCalls: []ChatToolCall{{
					Index: &idx,
					ID:    "call_xyz",
					Type:  "function",
					Function: ChatFunctionCall{
						Name:      "lookup",
						Arguments: `{"q":`,
					},
				}},
			},
		}},
	}
	events = ChatChunkToResponsesEvents(chunk2, state)
	require.Len(t, events, 2)
	require.Equal(t, "response.output_item.added", events[0].Type)
	require.Equal(t, "function_call", events[0].Item.Type)
	require.Equal(t, "call_xyz", events[0].Item.CallID)
	require.Equal(t, "lookup", events[0].Item.Name)
	require.Equal(t, "response.function_call_arguments.delta", events[1].Type)
	require.Equal(t, `{"q":`, events[1].Delta)

	// Finish
	finish := "tool_calls"
	chunk3 := ChatCompletionsChunk{
		Choices: []ChatChunkChoice{{Index: 0, FinishReason: &finish}},
	}
	ChatChunkToResponsesEvents(chunk3, state)

	// Usage-only tail chunk
	chunk4 := ChatCompletionsChunk{
		Usage: &ChatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
	events = ChatChunkToResponsesEvents(chunk4, state)
	require.Empty(t, events)
	require.NotNil(t, state.Usage)
	require.Equal(t, 10, state.Usage.InputTokens)

	final := FinalizeChatToResponsesStream(state)
	require.Len(t, final, 1)
	require.Equal(t, "response.completed", final[0].Type)
	require.NotNil(t, final[0].Response.Usage)
	require.Equal(t, 10, final[0].Response.Usage.InputTokens)
	require.Equal(t, 5, final[0].Response.Usage.OutputTokens)
}

func TestResponsesRequestToChatCompletions_Basic(t *testing.T) {
	// Build a Responses request as AnthropicToResponses would produce.
	inputItems := []ResponsesInputItem{
		{Role: "system", Content: mustJSONString("you are helpful")},
		{Role: "user", Content: mustJSONString("hi")},
		{Role: "assistant", Content: mustJSONString("hello!")},
	}
	inputJSON, _ := json.Marshal(inputItems)
	req := &ResponsesRequest{
		Model: "deepseek-v4-pro",
		Input: inputJSON,
	}
	max := 1024
	req.MaxOutputTokens = &max

	out, err := ResponsesRequestToChatCompletions(req)
	require.NoError(t, err)
	require.Equal(t, "deepseek-v4-pro", out.Model)
	require.Len(t, out.Messages, 3)
	require.Equal(t, "system", out.Messages[0].Role)
	require.Equal(t, "user", out.Messages[1].Role)
	require.Equal(t, "assistant", out.Messages[2].Role)
	require.NotNil(t, out.MaxTokens)
	require.Equal(t, 1024, *out.MaxTokens)
}

func TestResponsesRequestToChatCompletions_ToolCallAndResult(t *testing.T) {
	inputItems := []ResponsesInputItem{
		{Role: "user", Content: mustJSONString("what is the weather?")},
		// Assistant with a function call
		{Type: "function_call", CallID: "call_abc", Name: "get_weather", Arguments: `{"city":"SF"}`},
		{Type: "function_call_output", CallID: "call_abc", Output: `{"temp":72}`},
	}
	inputJSON, _ := json.Marshal(inputItems)
	req := &ResponsesRequest{
		Model: "deepseek-v4-pro",
		Input: inputJSON,
		Tools: []ResponsesTool{{
			Type: "function", Name: "get_weather",
			Description: "get weather",
			Parameters:  json.RawMessage(`{"type":"object"}`),
		}},
	}

	out, err := ResponsesRequestToChatCompletions(req)
	require.NoError(t, err)
	// user + assistant(with tool_call) + tool
	require.Len(t, out.Messages, 3)
	require.Equal(t, "user", out.Messages[0].Role)
	require.Equal(t, "assistant", out.Messages[1].Role)
	require.Len(t, out.Messages[1].ToolCalls, 1)
	require.Equal(t, "call_abc", out.Messages[1].ToolCalls[0].ID)
	require.Equal(t, "get_weather", out.Messages[1].ToolCalls[0].Function.Name)
	require.Equal(t, "tool", out.Messages[2].Role)
	require.Equal(t, "call_abc", out.Messages[2].ToolCallID)
	require.Len(t, out.Tools, 1)
	require.Equal(t, "function", out.Tools[0].Type)
	require.NotNil(t, out.Tools[0].Function)
	require.Equal(t, "get_weather", out.Tools[0].Function.Name)
}
