package apicompat

import (
	"encoding/json"
	"time"
)

// This file is the inverse of responses_to_chatcompletions.go:
//   ChatCompletionsResponse   → ResponsesResponse     (non-streaming)
//   []ChatCompletionsChunk    → []ResponsesStreamEvent (streaming)
//
// It enables the gateway to receive a ChatCompletions-speaking upstream
// (DeepSeek /v1/chat/completions) and feed results into the existing
// Responses → Anthropic conversion pipeline unchanged.

// ---------------------------------------------------------------------------
// Non-streaming: ChatCompletionsResponse → ResponsesResponse
// ---------------------------------------------------------------------------

// ChatCompletionsResponseToResponses converts a Chat Completions response into
// a Responses API response. choices[0].message is split into reasoning /
// message / function_call output items following the Responses output layout.
func ChatCompletionsResponseToResponses(resp *ChatCompletionsResponse, model string) *ResponsesResponse {
	if resp == nil {
		return nil
	}

	out := &ResponsesResponse{
		ID:     resp.ID,
		Object: "response",
		Model:  model,
		Status: "completed",
	}
	if out.Model == "" {
		out.Model = resp.Model
	}

	var output []ResponsesOutput
	var finishReason string

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		finishReason = choice.FinishReason
		msg := choice.Message

		if msg.ReasoningContent != "" {
			output = append(output, ResponsesOutput{
				Type: "reasoning",
				Summary: []ResponsesSummary{{
					Type: "summary_text",
					Text: msg.ReasoningContent,
				}},
			})
		}

		if text := extractChatAssistantText(msg.Content); text != "" {
			output = append(output, ResponsesOutput{
				Type: "message",
				Role: "assistant",
				Content: []ResponsesContentPart{{
					Type: "output_text",
					Text: text,
				}},
				Status: "completed",
			})
		}

		for _, tc := range msg.ToolCalls {
			args := tc.Function.Arguments
			if args == "" {
				args = "{}"
			}
			output = append(output, ResponsesOutput{
				Type:      "function_call",
				CallID:    tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}

	out.Output = output

	switch finishReason {
	case "length":
		out.Status = "incomplete"
		out.IncompleteDetails = &ResponsesIncompleteDetails{Reason: "max_output_tokens"}
	case "content_filter":
		out.Status = "incomplete"
		out.IncompleteDetails = &ResponsesIncompleteDetails{Reason: "content_filter"}
	}

	if resp.Usage != nil {
		out.Usage = chatUsageToResponsesUsage(resp.Usage)
	}

	return out
}

// extractChatAssistantText returns the textual portion of a ChatMessage.Content.
// Content may be a JSON string or an array of content parts; non-text parts are
// dropped.
func extractChatAssistantText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []ChatContentPart
	if err := json.Unmarshal(raw, &parts); err == nil {
		return flattenChatContentParts(parts)
	}
	return ""
}

func chatUsageToResponsesUsage(u *ChatUsage) *ResponsesUsage {
	if u == nil {
		return nil
	}
	ru := &ResponsesUsage{
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
		TotalTokens:  u.TotalTokens,
	}
	if ru.TotalTokens == 0 {
		ru.TotalTokens = ru.InputTokens + ru.OutputTokens
	}
	if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens > 0 {
		ru.InputTokensDetails = &ResponsesInputTokensDetails{
			CachedTokens: u.PromptTokensDetails.CachedTokens,
		}
	}
	return ru
}

// ---------------------------------------------------------------------------
// Streaming: ChatCompletionsChunk → []ResponsesStreamEvent (stateful converter)
// ---------------------------------------------------------------------------

// ChatToResponsesEventState tracks streaming state while inverting a Chat
// Completions chunk sequence into Responses SSE events.
type ChatToResponsesEventState struct {
	ID      string
	Model   string
	Created int64

	CreatedSent     bool
	CompletedEmitted bool

	// Accumulated finish_reason from the choice chunk that carried it. We
	// buffer completion emission so that a later usage-only chunk can be
	// merged before we signal completion downstream.
	FinishReason string
	FinishSeen   bool

	// message (text) item tracking
	TextItemAdded    bool
	TextOutputIndex  int

	// reasoning item tracking
	ReasoningItemAdded   bool
	ReasoningOutputIndex int

	// tool_call tracking: chat tool_calls index → responses output_index
	ToolCallOutputIndex map[int]int

	NextOutputIndex int

	Usage *ResponsesUsage
}

// NewChatToResponsesEventState returns an initialised stream state.
func NewChatToResponsesEventState() *ChatToResponsesEventState {
	return &ChatToResponsesEventState{
		ToolCallOutputIndex: make(map[int]int),
		Created:             time.Now().Unix(),
	}
}

// ChatChunkToResponsesEvents converts a single Chat Completions chunk into
// zero or more Responses SSE events, updating state as it goes. Completion
// is deferred to FinalizeChatToResponsesStream so a trailing usage chunk can
// be merged.
func ChatChunkToResponsesEvents(chunk ChatCompletionsChunk, state *ChatToResponsesEventState) []ResponsesStreamEvent {
	var events []ResponsesStreamEvent

	if !state.CreatedSent {
		state.CreatedSent = true
		if chunk.ID != "" {
			state.ID = chunk.ID
		}
		if state.Model == "" && chunk.Model != "" {
			state.Model = chunk.Model
		}
		events = append(events, ResponsesStreamEvent{
			Type: "response.created",
			Response: &ResponsesResponse{
				ID:     state.ID,
				Object: "response",
				Model:  state.Model,
				Status: "in_progress",
			},
		})
	}

	// Usage is typically delivered on a trailing chunk with empty choices.
	if chunk.Usage != nil {
		state.Usage = chatUsageToResponsesUsage(chunk.Usage)
	}

	for _, choice := range chunk.Choices {
		delta := choice.Delta

		if delta.ReasoningContent != nil && *delta.ReasoningContent != "" {
			if !state.ReasoningItemAdded {
				state.ReasoningItemAdded = true
				state.ReasoningOutputIndex = state.NextOutputIndex
				state.NextOutputIndex++
				events = append(events, ResponsesStreamEvent{
					Type:        "response.output_item.added",
					OutputIndex: state.ReasoningOutputIndex,
					Item: &ResponsesOutput{
						Type: "reasoning",
					},
				})
			}
			events = append(events, ResponsesStreamEvent{
				Type:        "response.reasoning_summary_text.delta",
				OutputIndex: state.ReasoningOutputIndex,
				Delta:       *delta.ReasoningContent,
			})
		}

		if delta.Content != nil && *delta.Content != "" {
			if !state.TextItemAdded {
				state.TextItemAdded = true
				state.TextOutputIndex = state.NextOutputIndex
				state.NextOutputIndex++
				events = append(events, ResponsesStreamEvent{
					Type:        "response.output_item.added",
					OutputIndex: state.TextOutputIndex,
					Item: &ResponsesOutput{
						Type: "message",
						Role: "assistant",
					},
				})
			}
			events = append(events, ResponsesStreamEvent{
				Type:        "response.output_text.delta",
				OutputIndex: state.TextOutputIndex,
				Delta:       *delta.Content,
			})
		}

		for _, tc := range delta.ToolCalls {
			chatIdx := 0
			if tc.Index != nil {
				chatIdx = *tc.Index
			}
			outIdx, seen := state.ToolCallOutputIndex[chatIdx]
			if !seen {
				outIdx = state.NextOutputIndex
				state.NextOutputIndex++
				state.ToolCallOutputIndex[chatIdx] = outIdx
				events = append(events, ResponsesStreamEvent{
					Type:        "response.output_item.added",
					OutputIndex: outIdx,
					Item: &ResponsesOutput{
						Type:   "function_call",
						CallID: tc.ID,
						Name:   tc.Function.Name,
					},
				})
			}
			if tc.Function.Arguments != "" {
				events = append(events, ResponsesStreamEvent{
					Type:        "response.function_call_arguments.delta",
					OutputIndex: outIdx,
					Delta:       tc.Function.Arguments,
				})
			}
		}

		if choice.FinishReason != nil && *choice.FinishReason != "" {
			state.FinishSeen = true
			state.FinishReason = *choice.FinishReason
		}
	}

	return events
}

// FinalizeChatToResponsesStream emits the deferred response.completed /
// response.incomplete event once the upstream stream ends. It is idempotent.
func FinalizeChatToResponsesStream(state *ChatToResponsesEventState) []ResponsesStreamEvent {
	if state.CompletedEmitted {
		return nil
	}
	state.CompletedEmitted = true

	resp := &ResponsesResponse{
		ID:     state.ID,
		Object: "response",
		Model:  state.Model,
		Status: "completed",
	}

	eventType := "response.completed"
	switch state.FinishReason {
	case "length":
		resp.Status = "incomplete"
		resp.IncompleteDetails = &ResponsesIncompleteDetails{Reason: "max_output_tokens"}
		eventType = "response.incomplete"
	case "content_filter":
		resp.Status = "incomplete"
		resp.IncompleteDetails = &ResponsesIncompleteDetails{Reason: "content_filter"}
		eventType = "response.incomplete"
	}

	if state.Usage != nil {
		resp.Usage = state.Usage
	}

	return []ResponsesStreamEvent{{
		Type:     eventType,
		Response: resp,
	}}
}
