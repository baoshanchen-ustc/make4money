package apicompat

import (
	"encoding/json"
	"fmt"
)

// ResponsesToChatCompletionsRequest converts an OpenAI Responses request into a
// Chat Completions request so OpenAI-compatible upstreams that only expose
// /chat/completions can still serve /v1/responses callers.
func ResponsesToChatCompletionsRequest(req *ResponsesRequest) (*ChatCompletionsRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("responses request is nil")
	}

	out := &ChatCompletionsRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		ToolChoice:  req.ToolChoice,
		ServiceTier: req.ServiceTier,
	}

	if req.MaxOutputTokens != nil {
		v := *req.MaxOutputTokens
		out.MaxTokens = &v
	}
	if req.Reasoning != nil {
		out.ReasoningEffort = req.Reasoning.Effort
	}
	if len(req.Tools) > 0 {
		out.Tools = convertResponsesToolsToChat(req.Tools)
	}
	if req.Instructions != "" {
		content, _ := json.Marshal(req.Instructions)
		out.Messages = append(out.Messages, ChatMessage{
			Role:    "system",
			Content: content,
		})
	}

	if len(req.Input) == 0 {
		return out, nil
	}

	var inputString string
	if err := json.Unmarshal(req.Input, &inputString); err == nil {
		content, _ := json.Marshal(inputString)
		out.Messages = append(out.Messages, ChatMessage{
			Role:    "user",
			Content: content,
		})
		return out, nil
	}

	var inputItems []ResponsesInputItem
	if err := json.Unmarshal(req.Input, &inputItems); err != nil {
		return nil, fmt.Errorf("parse responses input: %w", err)
	}

	for _, item := range inputItems {
		msg, err := responsesInputItemToChatMessages(item)
		if err != nil {
			return nil, err
		}
		out.Messages = append(out.Messages, msg...)
	}

	return out, nil
}

func convertResponsesToolsToChat(tools []ResponsesTool) []ChatTool {
	out := make([]ChatTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" {
			continue
		}
		out = append(out, ChatTool{
			Type: "function",
			Function: &ChatFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
				Strict:      tool.Strict,
			},
		})
	}
	return out
}

func responsesInputItemToChatMessages(item ResponsesInputItem) ([]ChatMessage, error) {
	switch item.Type {
	case "function_call":
		return []ChatMessage{{
			Role: "assistant",
			ToolCalls: []ChatToolCall{{
				ID:   item.CallID,
				Type: "function",
				Function: ChatFunctionCall{
					Name:      item.Name,
					Arguments: defaultJSONObjectString(item.Arguments),
				},
			}},
		}}, nil
	case "function_call_output":
		outputContent, _ := json.Marshal(item.Output)
		return []ChatMessage{{
			Role:       "tool",
			ToolCallID: item.CallID,
			Content:    outputContent,
		}}, nil
	}

	role := item.Role
	if role == "" {
		role = "user"
	}
	content, err := responsesMessageContentToChat(item.Content)
	if err != nil {
		return nil, fmt.Errorf("convert responses item content: %w", err)
	}
	return []ChatMessage{{
		Role:    role,
		Content: content,
	}}, nil
}

func responsesMessageContentToChat(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage(`""`), nil
	}

	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return json.Marshal(plain)
	}

	var parts []ResponsesContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, err
	}

	chatParts := make([]ChatContentPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "input_text", "output_text", "text":
			chatParts = append(chatParts, ChatContentPart{
				Type: "text",
				Text: part.Text,
			})
		case "input_image":
			chatParts = append(chatParts, ChatContentPart{
				Type: "image_url",
				ImageURL: &ChatImageURL{
					URL: part.ImageURL,
				},
			})
		}
	}

	return json.Marshal(chatParts)
}

func defaultJSONObjectString(raw string) string {
	if raw == "" {
		return "{}"
	}
	return raw
}

// ChatCompletionsToResponsesResponse converts a Chat Completions response into
// a Responses response for OpenAI-compatible upstreams that only speak
// /chat/completions.
func ChatCompletionsToResponsesResponse(resp *ChatCompletionsResponse, model string) *ResponsesResponse {
	if resp == nil {
		return &ResponsesResponse{
			Object: "response",
			Model:  model,
			Status: "completed",
			Output: []ResponsesOutput{{
				Type:    "message",
				Role:    "assistant",
				Status:  "completed",
				Content: []ResponsesContentPart{{Type: "output_text", Text: ""}},
			}},
		}
	}

	out := &ResponsesResponse{
		ID:     resp.ID,
		Object: "response",
		Model:  model,
		Status: "completed",
	}

	if len(resp.Choices) == 0 {
		out.Output = []ResponsesOutput{{
			Type:    "message",
			Role:    "assistant",
			Status:  "completed",
			Content: []ResponsesContentPart{{Type: "output_text", Text: ""}},
		}}
		return out
	}

	choice := resp.Choices[0]
	var outputs []ResponsesOutput

	if choice.Message.ReasoningContent != "" {
		outputs = append(outputs, ResponsesOutput{
			Type: "reasoning",
			Summary: []ResponsesSummary{{
				Type: "summary_text",
				Text: choice.Message.ReasoningContent,
			}},
		})
	}

	if text := extractChatMessageText(choice.Message.Content); text != "" || len(choice.Message.ToolCalls) == 0 {
		outputs = append(outputs, ResponsesOutput{
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []ResponsesContentPart{{
				Type: "output_text",
				Text: text,
			}},
		})
	}

	for _, toolCall := range choice.Message.ToolCalls {
		outputs = append(outputs, ResponsesOutput{
			Type:      "function_call",
			ID:        toolCall.ID,
			CallID:    toolCall.ID,
			Name:      toolCall.Function.Name,
			Arguments: defaultJSONObjectString(toolCall.Function.Arguments),
			Status:    "completed",
		})
	}

	out.Output = outputs
	switch choice.FinishReason {
	case "length":
		out.Status = "incomplete"
		out.IncompleteDetails = &ResponsesIncompleteDetails{Reason: "max_output_tokens"}
	default:
		out.Status = "completed"
	}

	if resp.Usage != nil {
		out.Usage = &ResponsesUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
		if resp.Usage.PromptTokensDetails != nil && resp.Usage.PromptTokensDetails.CachedTokens > 0 {
			out.Usage.InputTokensDetails = &ResponsesInputTokensDetails{
				CachedTokens: resp.Usage.PromptTokensDetails.CachedTokens,
			}
		}
	}

	return out
}

func extractChatMessageText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return plain
	}

	var parts []ChatContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}

	var text string
	for _, part := range parts {
		if part.Type == "text" {
			text += part.Text
		}
	}
	return text
}
