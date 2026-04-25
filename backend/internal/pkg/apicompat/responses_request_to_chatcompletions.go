package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"
)

// This file is the request-direction inverse of chatcompletions_to_responses.go:
//   ResponsesRequest → ChatCompletionsRequest
//
// Used by the gateway when forwarding a request that was first converted from
// Anthropic to Responses (to pick up thinking/tool_use handling) and then
// needs to be re-expressed in OpenAI Chat Completions format for upstreams
// that only speak Chat Completions (e.g. DeepSeek /v1/chat/completions).

// ResponsesRequestToChatCompletions converts a Responses API request into a
// Chat Completions request. It accepts the input items layout produced by
// AnthropicToResponses and re-hydrates standard Chat Completions messages,
// collapsing consecutive function_call items onto their originating assistant
// message and mapping function_call_output back to role=tool messages.
func ResponsesRequestToChatCompletions(req *ResponsesRequest) (*ChatCompletionsRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("nil ResponsesRequest")
	}

	var items []ResponsesInputItem
	if len(req.Input) > 0 {
		var asString string
		if err := json.Unmarshal(req.Input, &asString); err == nil {
			items = append(items, ResponsesInputItem{Role: "user", Content: mustJSONString(asString)})
		} else if err := json.Unmarshal(req.Input, &items); err != nil {
			return nil, fmt.Errorf("parse responses input items: %w", err)
		}
	}

	messages, err := responsesInputItemsToChatMessages(items)
	if err != nil {
		return nil, err
	}

	// Prepend a system message if req.Instructions is populated and no explicit
	// system message already exists at position 0.
	if req.Instructions != "" {
		hasSystem := len(messages) > 0 && messages[0].Role == "system"
		if !hasSystem {
			messages = append([]ChatMessage{{
				Role:    "system",
				Content: mustJSONString(req.Instructions),
			}}, messages...)
		}
	}

	out := &ChatCompletionsRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		ServiceTier: req.ServiceTier,
	}

	if req.MaxOutputTokens != nil {
		v := *req.MaxOutputTokens
		out.MaxTokens = &v
	}

	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		out.ReasoningEffort = req.Reasoning.Effort
	}

	if len(req.Tools) > 0 {
		out.Tools = responsesToolsToChatTools(req.Tools)
	}

	// tool_choice is already OpenAI-compatible at the Responses layer; pass through.
	if len(req.ToolChoice) > 0 {
		out.ToolChoice = req.ToolChoice
	}

	return out, nil
}

func responsesInputItemsToChatMessages(items []ResponsesInputItem) ([]ChatMessage, error) {
	var out []ChatMessage

	// pendingReasoning carries a reasoning item's summary text forward until we
	// can attach it to the assistant message it belongs to (same turn). It is
	// cleared when we cross a turn boundary (user/system/tool) without having
	// attached it, because reasoning only has meaning when paired with its
	// owning assistant turn.
	var pendingReasoning string
	attachReasoning := func(msg *ChatMessage) {
		if pendingReasoning == "" {
			return
		}
		if msg.ReasoningContent == "" {
			msg.ReasoningContent = pendingReasoning
		} else {
			msg.ReasoningContent += "\n\n" + pendingReasoning
		}
		pendingReasoning = ""
	}

	for _, item := range items {
		switch {
		case item.Type == "reasoning":
			var buf []string
			for _, s := range item.Summary {
				if s.Type == "summary_text" && s.Text != "" {
					buf = append(buf, s.Text)
				}
			}
			text := strings.Join(buf, "\n\n")
			if text == "" {
				continue
			}
			if pendingReasoning == "" {
				pendingReasoning = text
			} else {
				pendingReasoning += "\n\n" + text
			}

		case item.Type == "function_call":
			// Attach to previous assistant message, or create one.
			tc := ChatToolCall{
				ID:   item.CallID,
				Type: "function",
				Function: ChatFunctionCall{
					Name:      item.Name,
					Arguments: item.Arguments,
				},
			}
			if len(out) > 0 && out[len(out)-1].Role == "assistant" {
				out[len(out)-1].ToolCalls = append(out[len(out)-1].ToolCalls, tc)
				attachReasoning(&out[len(out)-1])
			} else {
				msg := ChatMessage{
					Role:      "assistant",
					ToolCalls: []ChatToolCall{tc},
				}
				attachReasoning(&msg)
				out = append(out, msg)
			}

		case item.Type == "function_call_output":
			// Tool result belongs to a user turn; drop any stray pending
			// reasoning (it could not be paired with an assistant message).
			pendingReasoning = ""
			content := item.Output
			if content == "" {
				content = "(empty)"
			}
			out = append(out, ChatMessage{
				Role:       "tool",
				ToolCallID: item.CallID,
				Content:    mustJSONString(content),
			})

		case item.Role == "system" || item.Role == "user" || item.Role == "assistant":
			msgContent, err := responsesContentToChatContent(item.Content, item.Role)
			if err != nil {
				return nil, fmt.Errorf("convert %s content: %w", item.Role, err)
			}
			msg := ChatMessage{
				Role:    item.Role,
				Content: msgContent,
			}
			if item.Role == "assistant" {
				attachReasoning(&msg)
			} else {
				pendingReasoning = ""
			}
			out = append(out, msg)

		default:
			// Unknown item — fall back to user text if we can extract something.
			msgContent, err := responsesContentToChatContent(item.Content, "user")
			if err == nil && len(msgContent) > 0 {
				pendingReasoning = ""
				out = append(out, ChatMessage{Role: "user", Content: msgContent})
			}
		}
	}

	return out, nil
}

// responsesContentToChatContent converts a Responses content field (string or
// []ResponsesContentPart) into Chat Completions message content.
// Assistant text uses plain string; user/system multi-modal uses parts array.
func responsesContentToChatContent(raw json.RawMessage, role string) (json.RawMessage, error) {
	if len(raw) == 0 {
		return mustJSONString(""), nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return mustJSONString(s), nil
	}

	var parts []ResponsesContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, err
	}

	// Assistant content: Chat Completions expects a string. Concatenate text parts.
	if role == "assistant" {
		var buf string
		for _, p := range parts {
			if p.Text != "" {
				buf += p.Text
			}
		}
		return mustJSONString(buf), nil
	}

	// user / system: emit multi-modal parts array.
	chatParts := make([]ChatContentPart, 0, len(parts))
	for _, p := range parts {
		switch p.Type {
		case "input_text", "output_text", "text":
			if p.Text != "" {
				chatParts = append(chatParts, ChatContentPart{Type: "text", Text: p.Text})
			}
		case "input_image":
			if p.ImageURL != "" {
				chatParts = append(chatParts, ChatContentPart{
					Type:     "image_url",
					ImageURL: &ChatImageURL{URL: p.ImageURL},
				})
			}
		}
	}

	// Fallback to plain text concatenation if no recognized parts.
	if len(chatParts) == 0 {
		var buf string
		for _, p := range parts {
			if p.Text != "" {
				buf += p.Text
			}
		}
		return mustJSONString(buf), nil
	}

	return json.Marshal(chatParts)
}

func responsesToolsToChatTools(tools []ResponsesTool) []ChatTool {
	out := make([]ChatTool, 0, len(tools))
	for _, t := range tools {
		if t.Type != "function" {
			// Only function tools are representable in Chat Completions.
			// web_search / local_shell etc. are dropped with a silent skip.
			continue
		}
		out = append(out, ChatTool{
			Type: "function",
			Function: &ChatFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
				Strict:      t.Strict,
			},
		})
	}
	return out
}

func mustJSONString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}
