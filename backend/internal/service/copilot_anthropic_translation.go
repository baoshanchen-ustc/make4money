package service

// copilot_anthropic_translation.go
//
// Implements Anthropic Messages API ↔ OpenAI Chat Completions translation
// for the GitHub Copilot gateway.
//
// Translation direction:
//   Incoming:  Anthropic /v1/messages  →  OpenAI /chat/completions  →  Copilot API
//   Outgoing:  Copilot API response    →  OpenAI response           →  Anthropic response
//
// Reference implementation: https://github.com/ericc-ch/copilot-api (TypeScript)

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// Anthropic request types
// ─────────────────────────────────────────────────────────────────────────────

// AnthropicMessagesRequest is the body of a POST /v1/messages request.
type AnthropicMessagesRequest struct {
	Model         string               `json:"model"`
	Messages      []AnthropicMessage   `json:"messages"`
	MaxTokens     int                  `json:"max_tokens"`
	System        json.RawMessage      `json:"system,omitempty"` // string or []AnthropicTextBlock
	Metadata      *AnthropicMetadata   `json:"metadata,omitempty"`
	StopSequences []string             `json:"stop_sequences,omitempty"`
	Stream        bool                 `json:"stream,omitempty"`
	Temperature   *float64             `json:"temperature,omitempty"`
	TopP          *float64             `json:"top_p,omitempty"`
	Tools         []AnthropicTool      `json:"tools,omitempty"`
	ToolChoice    *AnthropicToolChoice `json:"tool_choice,omitempty"`
}

type AnthropicMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

// AnthropicMessage is a single turn in the conversation.
type AnthropicMessage struct {
	Role    string          `json:"role"`    // "user" | "assistant"
	Content json.RawMessage `json:"content"` // string or []block
}

// AnthropicTextBlock is a plain text content block.
type AnthropicTextBlock struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// AnthropicImageBlock is a base64-encoded image block.
type AnthropicImageBlock struct {
	Type   string                    `json:"type"` // "image"
	Source AnthropicImageBlockSource `json:"source"`
}

// AnthropicImageBlockSource holds image data.
type AnthropicImageBlockSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/jpeg" etc.
	Data      string `json:"data"`
}

// AnthropicToolResultBlock is the result of a tool call.
type AnthropicToolResultBlock struct {
	Type      string `json:"type"` // "tool_result"
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// AnthropicToolUseBlock represents the model calling a tool.
type AnthropicToolUseBlock struct {
	Type  string          `json:"type"` // "tool_use"
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// AnthropicThinkingBlock represents a thinking/reasoning block (not used in Copilot).
type AnthropicThinkingBlock struct {
	Type     string `json:"type"` // "thinking"
	Thinking string `json:"thinking"`
}

// AnthropicTool is a tool definition in the request.
type AnthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// AnthropicToolChoice specifies how the model should use tools.
type AnthropicToolChoice struct {
	Type string `json:"type"` // "auto" | "any" | "tool" | "none"
	Name string `json:"name,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Anthropic response types
// ─────────────────────────────────────────────────────────────────────────────

// AnthropicMessagesResponse is the response body for a non-streaming request.
type AnthropicMessagesResponse struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"` // "message"
	Role         string            `json:"role"` // "assistant"
	Model        string            `json:"model"`
	Content      []json.RawMessage `json:"content"` // []AnthropicTextBlock | []AnthropicToolUseBlock
	StopReason   string            `json:"stop_reason"`
	StopSequence *string           `json:"stop_sequence"`
	Usage        AnthropicUsage    `json:"usage"`
}

// AnthropicUsage holds token usage counts.
type AnthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// OpenAI (Copilot API) types
// ─────────────────────────────────────────────────────────────────────────────

// openAIChatRequest is the body sent to Copilot's /chat/completions.
type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
	Stream      bool            `json:"stream"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	User        string          `json:"user,omitempty"`
	Tools       []openAITool    `json:"tools,omitempty"`
	ToolChoice  any             `json:"tool_choice,omitempty"`
}

// openAIMessage is a single message in an OpenAI chat request.
type openAIMessage struct {
	Role       string           `json:"role"`    // system | user | assistant | tool
	Content    any              `json:"content"` // string or []openAIContentPart
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
}

// openAIContentPart is a part of a multi-modal message.
type openAIContentPart struct {
	Type     string                `json:"type"` // "text" | "image_url"
	Text     string                `json:"text,omitempty"`
	ImageURL *openAIImageURLObject `json:"image_url,omitempty"`
}

// openAIImageURLObject holds a base64-encoded image URL.
type openAIImageURLObject struct {
	URL string `json:"url"`
}

// openAITool is a function tool definition.
type openAITool struct {
	Type     string         `json:"type"` // "function"
	Function openAIFunction `json:"function"`
}

// openAIFunction describes a callable function.
type openAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// openAIToolCall is a tool call made by the model.
type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"` // "function"
	Function openAIFunctionCall `json:"function"`
}

// openAIFunctionCall is the function being called.
type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// openAIFunctionCallDelta is an incremental update to a function call during streaming.
type openAIFunctionCallDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// openAIToolCallDelta is an incremental tool call chunk during streaming.
type openAIToolCallDelta struct {
	Index    int                     `json:"index"`
	ID       string                  `json:"id,omitempty"`
	Type     string                  `json:"type,omitempty"`
	Function openAIFunctionCallDelta `json:"function,omitempty"`
}

// openAIChatResponse is a non-streaming response from Copilot.
type openAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   *openAIUsage   `json:"usage,omitempty"`
}

// openAIChoice is a single completion choice.
type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// openAIUsage holds token usage from an OpenAI response.
type openAIUsage struct {
	PromptTokens     int                  `json:"prompt_tokens"`
	CompletionTokens int                  `json:"completion_tokens"`
	TotalTokens      int                  `json:"total_tokens"`
	PromptDetails    *openAIPromptDetails `json:"prompt_tokens_details,omitempty"`
}

// openAIPromptDetails holds details about prompt tokens.
type openAIPromptDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// openAIChatStreamChunk is a single SSE chunk in a streaming response.
type openAIChatStreamChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Model   string               `json:"model"`
	Choices []openAIStreamChoice `json:"choices"`
	Usage   *openAIUsage         `json:"usage,omitempty"`
}

// openAIStreamChoice is a choice delta in a streaming chunk.
type openAIStreamChoice struct {
	Index        int         `json:"index"`
	Delta        openAIDelta `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

// openAIDelta is the incremental content in a stream chunk.
type openAIDelta struct {
	Role      string                `json:"role,omitempty"`
	Content   string                `json:"content,omitempty"`
	ToolCalls []openAIToolCallDelta `json:"tool_calls,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Request translation: Anthropic → OpenAI
// ─────────────────────────────────────────────────────────────────────────────

// translateAnthropicToOpenAI converts an Anthropic /v1/messages request body to
// an OpenAI /chat/completions request body suitable for the Copilot API.
//
// The model field is copied verbatim from the Anthropic request; CopilotGatewayService
// then applies account model_mapping and copilot.NormalizeModelIDForCopilotUpstream
// before sending to GitHub (Anthropic dated / dash ids → Copilot wire ids).
func translateAnthropicToOpenAI(body []byte, _ map[string]string) ([]byte, error) {
	var req AnthropicMessagesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}

	openAIReq := openAIChatRequest{
		Model:       req.Model,
		Messages:    buildOpenAIMessages(req),
		MaxTokens:   req.MaxTokens,
		Stop:        req.StopSequences,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Tools:       translateTools(req.Tools),
		ToolChoice:  translateToolChoice(req.ToolChoice),
	}
	if req.Metadata != nil {
		openAIReq.User = req.Metadata.UserID
	}

	return json.Marshal(openAIReq)
}

// buildOpenAIMessages converts the Anthropic messages array (plus optional system
// prompt) into the OpenAI messages array.
func buildOpenAIMessages(req AnthropicMessagesRequest) []openAIMessage {
	var msgs []openAIMessage

	// System prompt
	if len(req.System) > 0 && string(req.System) != "null" {
		systemText := extractSystemText(req.System)
		if systemText != "" {
			msgs = append(msgs, openAIMessage{
				Role:    "system",
				Content: systemText,
			})
		}
	}

	// Conversation messages
	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			msgs = append(msgs, handleAnthropicUserMessage(m)...)
		case "assistant":
			msgs = append(msgs, handleAnthropicAssistantMessage(m))
		}
	}

	return sanitizeOpenAIMessages(msgs)
}

// sanitizeOpenAIMessages normalizes translated messages to satisfy Copilot API
// constraints that are stricter than the Anthropic protocol:
//
//  1. Every assistant tool_call must have a matching tool-role response.
//     Claude Code sometimes sends an empty user acknowledgement instead of a
//     tool_result (e.g. after ToolSearch deferred-tool lookup), which leaves
//     the tool_call "orphaned" in OpenAI format.  We inject a synthetic tool
//     response so the Copilot API accepts the conversation.
//
//  2. Consecutive same-role messages (user/user, system/system) are merged
//     because the Copilot API may reject them.
//
//  3. Empty user messages (content == "") that sit between two assistant
//     messages are removed when they serve no purpose.
func sanitizeOpenAIMessages(msgs []openAIMessage) []openAIMessage {
	// ── Step 1: inject synthetic tool responses for orphan tool_calls ────

	// Collect all tool_call IDs that already have a tool response.
	answered := make(map[string]struct{})
	for _, m := range msgs {
		if m.Role == "tool" && m.ToolCallID != "" {
			answered[m.ToolCallID] = struct{}{}
		}
	}

	// Walk messages; after each assistant with unanswered tool_calls, insert
	// synthetic tool responses immediately following that assistant message.
	var patched []openAIMessage
	for _, m := range msgs {
		patched = append(patched, m)
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				if _, ok := answered[tc.ID]; !ok {
					patched = append(patched, openAIMessage{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    "",
					})
					answered[tc.ID] = struct{}{}
				}
			}
		}
	}

	// ── Step 2: remove empty user messages that are now unnecessary ──────
	// An empty user message between two non-user messages serves no purpose
	// and can cause "consecutive same-role" issues once we injected synthetic
	// tool responses above.
	var cleaned []openAIMessage
	for i, m := range patched {
		if m.Role == "user" {
			if s, ok := m.Content.(string); ok && s == "" {
				// Keep if it is the only message, or the last message
				if len(patched) > 1 && i < len(patched)-1 {
					continue
				}
			}
		}
		cleaned = append(cleaned, m)
	}

	// ── Step 3: merge consecutive user/system messages ───────────────────
	if len(cleaned) == 0 {
		return cleaned
	}
	result := make([]openAIMessage, 0, len(cleaned))
	result = append(result, cleaned[0])

	for i := 1; i < len(cleaned); i++ {
		cur := cleaned[i]
		prev := &result[len(result)-1]

		canMerge := cur.Role == prev.Role &&
			(cur.Role == "user" || cur.Role == "system") &&
			len(cur.ToolCalls) == 0 &&
			len(prev.ToolCalls) == 0 &&
			!hasImageContentPart(cur.Content) &&
			!hasImageContentPart(prev.Content)

		if !canMerge {
			result = append(result, cur)
			continue
		}

		prevText := contentToString(prev.Content)
		curText := contentToString(cur.Content)
		merged := prevText
		if curText != "" {
			if merged != "" {
				merged += "\n\n"
			}
			merged += curText
		}
		prev.Content = merged
	}

	return result
}

// contentToString extracts a plain string from an openAIMessage Content value.
func contentToString(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []openAIContentPart:
		var parts []string
		for _, p := range v {
			if p.Type == "text" && p.Text != "" {
				parts = append(parts, p.Text)
			}
		}
		return strings.Join(parts, "\n\n")
	default:
		return ""
	}
}

// hasImageContentPart reports whether a message content value contains at least
// one image_url content part.  Used by sanitizeOpenAIMessages to prevent
// consecutive user messages that contain images from being merged via
// contentToString, which would silently drop the image_url parts.
func hasImageContentPart(content any) bool {
	parts, ok := content.([]openAIContentPart)
	if !ok {
		return false
	}
	for _, p := range parts {
		if p.Type == "image_url" {
			return true
		}
	}
	return false
}

// extractSystemText parses the Anthropic system field, which can be either a
// plain string or an array of text blocks.
func extractSystemText(raw json.RawMessage) string {
	// Try plain string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try array of text blocks.
	var blocks []AnthropicTextBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n\n")
	}

	return ""
}

// handleAnthropicUserMessage converts an Anthropic user message to zero or more
// OpenAI messages.  tool_result blocks become separate "tool" role messages.
func handleAnthropicUserMessage(m AnthropicMessage) []openAIMessage {
	// Attempt to decode as plain string.
	var text string
	if err := json.Unmarshal(m.Content, &text); err == nil {
		return []openAIMessage{{Role: "user", Content: text}}
	}

	// Decode as array of blocks.
	var blocks []json.RawMessage
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return []openAIMessage{{Role: "user", Content: string(m.Content)}}
	}

	var toolResults []openAIMessage
	// hasImage tracks whether any image block is present in non-tool_result blocks.
	// When true we must use content-parts array; when false we join text/thinking as a plain string.
	// This matches TypeScript mapContent() semantics exactly.
	var hasImage bool
	var textParts []string             // text + thinking joined when no image
	var imageParts []openAIContentPart // full content-parts when image present

	for _, raw := range blocks {
		var typed struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &typed); err != nil {
			continue
		}

		switch typed.Type {
		case "tool_result":
			var tr AnthropicToolResultBlock
			if err := json.Unmarshal(raw, &tr); err == nil {
				toolResults = append(toolResults, openAIMessage{
					Role:       "tool",
					Content:    tr.Content,
					ToolCallID: tr.ToolUseID,
				})
			}
		case "text":
			var tb AnthropicTextBlock
			if err := json.Unmarshal(raw, &tb); err == nil {
				textParts = append(textParts, tb.Text)
				imageParts = append(imageParts, openAIContentPart{Type: "text", Text: tb.Text})
			}
		case "thinking":
			// thinking blocks in user messages are rare but handled for completeness,
			// matching TypeScript mapContent() which includes block.type === "thinking".
			var thk AnthropicThinkingBlock
			if err := json.Unmarshal(raw, &thk); err == nil {
				textParts = append(textParts, thk.Thinking)
				imageParts = append(imageParts, openAIContentPart{Type: "text", Text: thk.Thinking})
			}
		case "image":
			hasImage = true
			var ib AnthropicImageBlock
			if err := json.Unmarshal(raw, &ib); err == nil {
				imageParts = append(imageParts, openAIContentPart{
					Type: "image_url",
					ImageURL: &openAIImageURLObject{
						URL: fmt.Sprintf("data:%s;base64,%s", ib.Source.MediaType, ib.Source.Data),
					},
				})
			}
		}
	}

	// tool_result messages must come first (protocol: tool_use → tool_result → next_user).
	var result []openAIMessage
	result = append(result, toolResults...)

	if len(textParts) > 0 || hasImage {
		var userContent any
		if hasImage {
			// Image present: use content-parts array (TypeScript path with hasImage=true)
			userContent = imageParts
		} else {
			// No image: join text+thinking as a plain string (TypeScript mapContent no-image path)
			userContent = strings.Join(textParts, "\n\n")
		}
		result = append(result, openAIMessage{Role: "user", Content: userContent})
	}

	if len(result) == 0 {
		result = append(result, openAIMessage{Role: "user", Content: ""})
	}

	return result
}

// handleAnthropicAssistantMessage converts an Anthropic assistant message to a
// single OpenAI assistant message (with optional tool_calls).
func handleAnthropicAssistantMessage(m AnthropicMessage) openAIMessage {
	// Plain string content.
	var text string
	if err := json.Unmarshal(m.Content, &text); err == nil {
		return openAIMessage{Role: "assistant", Content: text}
	}

	// Array of blocks.
	var blocks []json.RawMessage
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return openAIMessage{Role: "assistant", Content: string(m.Content)}
	}

	var textParts []string
	var toolCalls []openAIToolCall

	for _, raw := range blocks {
		var typed struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &typed); err != nil {
			continue
		}

		switch typed.Type {
		case "text":
			var tb AnthropicTextBlock
			if err := json.Unmarshal(raw, &tb); err == nil {
				textParts = append(textParts, tb.Text)
			}
		case "thinking":
			var thk AnthropicThinkingBlock
			if err := json.Unmarshal(raw, &thk); err == nil {
				textParts = append(textParts, thk.Thinking)
			}
		case "tool_use":
			var tu AnthropicToolUseBlock
			if err := json.Unmarshal(raw, &tu); err == nil {
				argBytes, _ := json.Marshal(tu.Input)
				toolCalls = append(toolCalls, openAIToolCall{
					ID:   tu.ID,
					Type: "function",
					Function: openAIFunctionCall{
						Name:      tu.Name,
						Arguments: string(argBytes),
					},
				})
			}
		}
	}

	combined := strings.Join(textParts, "\n\n")
	msg := openAIMessage{Role: "assistant"}
	if combined != "" {
		msg.Content = combined
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	return msg
}

// translateTools converts Anthropic tool definitions to OpenAI format.
func translateTools(tools []AnthropicTool) []openAITool {
	if len(tools) == 0 {
		return nil
	}
	result := make([]openAITool, 0, len(tools))
	for _, t := range tools {
		result = append(result, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}
	return result
}

// translateToolChoice converts Anthropic tool_choice to OpenAI format.
func translateToolChoice(tc *AnthropicToolChoice) any {
	if tc == nil {
		return nil
	}
	switch tc.Type {
	case "auto":
		return "auto"
	case "any":
		return "required"
	case "none":
		return "none"
	case "tool":
		if tc.Name != "" {
			return map[string]any{
				"type":     "function",
				"function": map[string]string{"name": tc.Name},
			}
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Response translation: OpenAI → Anthropic
// ─────────────────────────────────────────────────────────────────────────────

// translateOpenAIToAnthropic converts a non-streaming OpenAI response body to
// an Anthropic response body.
func translateOpenAIToAnthropic(body []byte) ([]byte, error) {
	var resp openAIChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse openai response: %w", err)
	}

	var contentBlocks []json.RawMessage
	var stopReason string

	for i, choice := range resp.Choices {
		if i == 0 {
			stopReason = mapOpenAIFinishReasonToAnthropic(choice.FinishReason)
		}

		// Text content
		if text, ok := choice.Message.Content.(string); ok && text != "" {
			block, _ := json.Marshal(AnthropicTextBlock{Type: "text", Text: text})
			contentBlocks = append(contentBlocks, block)
		}

		// Tool use blocks
		for _, tc := range choice.Message.ToolCalls {
			var input json.RawMessage
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
			if input == nil {
				input = json.RawMessage("{}")
			}
			block, _ := json.Marshal(AnthropicToolUseBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
			contentBlocks = append(contentBlocks, block)
			if stopReason == "end_turn" {
				stopReason = "tool_use"
			}
		}
	}

	if len(contentBlocks) == 0 {
		empty, _ := json.Marshal(AnthropicTextBlock{Type: "text", Text: ""})
		contentBlocks = append(contentBlocks, empty)
	}

	anthropicResp := AnthropicMessagesResponse{
		ID:         resp.ID,
		Type:       "message",
		Role:       "assistant",
		Model:      resp.Model,
		Content:    contentBlocks,
		StopReason: stopReason,
		Usage:      buildAnthropicUsage(resp.Usage),
	}

	return json.Marshal(anthropicResp)
}

// buildAnthropicUsage converts OpenAI usage to Anthropic usage.
func buildAnthropicUsage(u *openAIUsage) AnthropicUsage {
	if u == nil {
		return AnthropicUsage{}
	}
	au := AnthropicUsage{
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
	}
	if u.PromptDetails != nil {
		au.CacheReadInputTokens = u.PromptDetails.CachedTokens
		au.InputTokens = u.PromptTokens - u.PromptDetails.CachedTokens
	}
	return au
}

// mapOpenAIFinishReasonToAnthropic maps OpenAI finish_reason to Anthropic stop_reason.
func mapOpenAIFinishReasonToAnthropic(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "end_turn"
	default:
		return "end_turn"
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Streaming state machine: OpenAI chunks → Anthropic SSE events
// ─────────────────────────────────────────────────────────────────────────────

// copilotStreamState tracks the state of an in-progress stream translation.
type copilotStreamState struct {
	messageStartSent bool
	blockIndex       int
	blockOpen        bool
	// toolCalls maps OpenAI tool_call index → Anthropic block index + metadata
	toolCalls map[int]copilotToolCallInfo
}

type copilotToolCallInfo struct {
	anthropicBlockIdx int
}

// translateChunkToAnthropicEvents converts a single OpenAI SSE chunk to zero or
// more Anthropic SSE event payloads (each is a JSON string to be written as
// "data: <json>\n\n").
func translateChunkToAnthropicEvents(
	chunk *openAIChatStreamChunk,
	state *copilotStreamState,
) []string {
	var events []string
	if len(chunk.Choices) == 0 {
		return events
	}
	choice := chunk.Choices[0]
	delta := choice.Delta

	// Send message_start once per stream.
	if !state.messageStartSent {
		inputTokens := 0
		cacheRead := 0
		if chunk.Usage != nil {
			inputTokens = chunk.Usage.PromptTokens
			if chunk.Usage.PromptDetails != nil {
				cacheRead = chunk.Usage.PromptDetails.CachedTokens
				inputTokens -= cacheRead
			}
		}
		evt := map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":            chunk.ID,
				"type":          "message",
				"role":          "assistant",
				"content":       []any{},
				"model":         chunk.Model,
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]any{
					"input_tokens":            inputTokens,
					"output_tokens":           0,
					"cache_read_input_tokens": cacheRead,
				},
			},
		}
		if b, err := json.Marshal(evt); err == nil {
			events = append(events, string(b))
		}
		state.messageStartSent = true

		// Anthropic expects a leading ping event.
		pingJSON, _ := json.Marshal(map[string]string{"type": "ping"})
		events = append(events, string(pingJSON))
	}

	// Text delta content.
	if delta.Content != "" {
		if isToolBlockOpen(state) {
			// Close the open tool block before starting text.
			events = append(events, blockStopEvent(state.blockIndex))
			state.blockIndex++
			state.blockOpen = false
		}
		if !state.blockOpen {
			events = append(events, contentBlockStart(state.blockIndex, "text"))
			state.blockOpen = true
		}
		events = append(events, textDeltaEvent(state.blockIndex, delta.Content))
	}

	// Tool call deltas.
	for _, tc := range delta.ToolCalls {
		if tc.ID != "" && tc.Function.Name != "" {
			// New tool call starting.
			if state.blockOpen {
				events = append(events, blockStopEvent(state.blockIndex))
				state.blockIndex++
				state.blockOpen = false
			}
			anthropicIdx := state.blockIndex
			state.toolCalls[tc.Index] = copilotToolCallInfo{
				anthropicBlockIdx: anthropicIdx,
			}
			events = append(events, toolUseBlockStart(anthropicIdx, tc.ID, tc.Function.Name))
			state.blockOpen = true
		}
		if tc.Function.Arguments != "" {
			if info, ok := state.toolCalls[tc.Index]; ok {
				events = append(events, inputJSONDeltaEvent(info.anthropicBlockIdx, tc.Function.Arguments))
			}
		}
	}

	// Stream finished.
	if choice.FinishReason != "" {
		if state.blockOpen {
			events = append(events, blockStopEvent(state.blockIndex))
			state.blockOpen = false
		}

		outputTokens := 0
		inputTokens := 0
		if chunk.Usage != nil {
			outputTokens = chunk.Usage.CompletionTokens
			inputTokens = chunk.Usage.PromptTokens
			if chunk.Usage.PromptDetails != nil {
				inputTokens -= chunk.Usage.PromptDetails.CachedTokens
			}
		}

		stopReason := mapOpenAIFinishReasonToAnthropic(choice.FinishReason)
		msgDelta := map[string]any{
			"type": "message_delta",
			"delta": map[string]any{
				"stop_reason":   stopReason,
				"stop_sequence": nil,
			},
			"usage": map[string]any{
				"input_tokens":  inputTokens,
				"output_tokens": outputTokens,
			},
		}
		if b, err := json.Marshal(msgDelta); err == nil {
			events = append(events, string(b))
		}

		msgStop, _ := json.Marshal(map[string]string{"type": "message_stop"})
		events = append(events, string(msgStop))
	}

	return events
}

// isToolBlockOpen returns true if the currently open block is a tool_use block.
func isToolBlockOpen(state *copilotStreamState) bool {
	if !state.blockOpen {
		return false
	}
	for _, info := range state.toolCalls {
		if info.anthropicBlockIdx == state.blockIndex {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// SSE event constructors
// ─────────────────────────────────────────────────────────────────────────────

func blockStopEvent(idx int) string {
	b, _ := json.Marshal(map[string]any{"type": "content_block_stop", "index": idx})
	return string(b)
}

func contentBlockStart(idx int, blockType string) string {
	b, _ := json.Marshal(map[string]any{
		"type":  "content_block_start",
		"index": idx,
		"content_block": map[string]any{
			"type": blockType,
			"text": "",
		},
	})
	return string(b)
}

func toolUseBlockStart(idx int, id, name string) string {
	b, _ := json.Marshal(map[string]any{
		"type":  "content_block_start",
		"index": idx,
		"content_block": map[string]any{
			"type":  "tool_use",
			"id":    id,
			"name":  name,
			"input": map[string]any{},
		},
	})
	return string(b)
}

func textDeltaEvent(idx int, text string) string {
	b, _ := json.Marshal(map[string]any{
		"type":  "content_block_delta",
		"index": idx,
		"delta": map[string]any{
			"type": "text_delta",
			"text": text,
		},
	})
	return string(b)
}

func inputJSONDeltaEvent(idx int, partial string) string {
	b, _ := json.Marshal(map[string]any{
		"type":  "content_block_delta",
		"index": idx,
		"delta": map[string]any{
			"type":         "input_json_delta",
			"partial_json": partial,
		},
	})
	return string(b)
}

// ─────────────────────────────────────────────────────────────────────────────
// Context window truncation
// ─────────────────────────────────────────────────────────────────────────────

// TruncateAnthropicBodyToLimit trims the messages array in an Anthropic-format
// request body so the serialised body fits within limitBytes.
//
// Strategy:
//  1. Keep the system prompt and tools untouched — they are necessary for
//     every turn and cannot be removed without breaking the request.
//  2. Drop the oldest conversation turns from the front of messages[].
//     Each "turn" is a contiguous block of one or more messages that forms a
//     coherent unit: user → (assistant + optional tool results) → next user.
//  3. After each drop, re-serialise and check the size.  Stop as soon as the
//     body is within the limit, or when fewer than minMessages remain (at
//     which point truncation is no longer safe and the caller should reject).
//
// The function returns (trimmedBody, truncated, err).
// When truncated == false and err == nil the original body already fits.
// When the body cannot be brought within the limit without dropping below
// minMessages the function returns the most-trimmed body with truncated==true
// so the caller can decide whether to reject or forward anyway.
func TruncateAnthropicBodyToLimit(body []byte, limitBytes int) (trimmed []byte, truncated bool, err error) {
	const minMessages = 2 // always keep at least one user + one assistant turn

	if len(body) <= limitBytes {
		return body, false, nil
	}

	// Parse only the fields we need to manipulate.
	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	// Use a generic map so we preserve every other field verbatim.
	var generic map[string]json.RawMessage
	if err = json.Unmarshal(body, &generic); err != nil {
		return body, false, fmt.Errorf("truncate: parse body: %w", err)
	}

	msgsRaw, ok := generic["messages"]
	if !ok {
		return body, false, nil // no messages field — cannot truncate
	}
	if err = json.Unmarshal(msgsRaw, &req.Messages); err != nil {
		return body, false, fmt.Errorf("truncate: parse messages: %w", err)
	}

	msgs := req.Messages
	for len(msgs) > minMessages {
		// Drop the oldest message (index 0).
		// Before dropping, also remove the immediately following messages if
		// they form the "response" half of the same turn — i.e. consecutive
		// assistant messages or tool-role messages that answered the dropped
		// user turn.  This keeps tool_use / tool_result pairs intact.
		dropCount := 1
		if len(msgs) > 1 {
			// Check role of msg[0] to decide how many to drop together.
			var m0 struct {
				Role string `json:"role"`
			}
			if jsonErr := json.Unmarshal(msgs[0], &m0); jsonErr == nil && m0.Role == "user" {
				// Drop this user message plus the next assistant + any following
				// tool messages so we don't leave an orphaned tool_result.
				i := 1
				for i < len(msgs) {
					var mi struct {
						Role string `json:"role"`
					}
					if jsonErr := json.Unmarshal(msgs[i], &mi); jsonErr != nil {
						break
					}
					if mi.Role == "assistant" || mi.Role == "tool" {
						i++
					} else {
						break
					}
				}
				dropCount = i
			}
		}
		if len(msgs)-dropCount < minMessages {
			// Dropping this batch would go below minimum — drop one at a time instead.
			dropCount = len(msgs) - minMessages
			if dropCount <= 0 {
				break
			}
		}
		msgs = msgs[dropCount:]

		// Re-serialise and check size.
		newMsgsJSON, marshalErr := json.Marshal(msgs)
		if marshalErr != nil {
			return body, false, fmt.Errorf("truncate: serialise messages: %w", marshalErr)
		}
		generic["messages"] = newMsgsJSON

		candidate, marshalErr := json.Marshal(generic)
		if marshalErr != nil {
			return body, false, fmt.Errorf("truncate: serialise body: %w", marshalErr)
		}
		if len(candidate) <= limitBytes {
			return candidate, true, nil
		}
		// Still too large — continue trimming.
		body = candidate
	}

	// Could not bring body within limit while keeping minMessages.
	// Return the most-trimmed version with truncated=true; caller decides.
	candidate, _ := json.Marshal(generic)
	if len(candidate) == 0 {
		candidate = body
	}
	return candidate, true, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// OpenAI Chat Completions → Anthropic Messages translation (for file uploads)
// ─────────────────────────────────────────────────────────────────────────────

// AnthropicDocumentBlock is an Anthropic content block for document (PDF) files.
// The Anthropic Messages API accepts these natively, preserving file content.
type AnthropicDocumentBlock struct {
	Type   string                        `json:"type"` // "document"
	Source AnthropicDocumentBlockSource  `json:"source"`
}

// AnthropicDocumentBlockSource holds the encoded document data.
type AnthropicDocumentBlockSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "application/pdf"
	Data      string `json:"data"`       // base64-encoded PDF bytes (no data: URI prefix)
}

// convertOpenAIChatToAnthropicMessages converts an OpenAI Chat Completions
// request body (which may contain type:"file" content parts with base64-encoded
// PDFs) to an Anthropic Messages API request body.
//
// type:"file" parts with a "file_data" URL ("data:<mime>;base64,<data>") are
// converted to type:"document" blocks so the Anthropic-compatible Copilot
// /v1/messages endpoint can process the file content natively.
//
// The returned body is ready to POST to Copilot /v1/messages.
func convertOpenAIChatToAnthropicMessages(body []byte) ([]byte, error) {
	// Parse as a generic map to preserve all fields we don't explicitly handle.
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(body, &generic); err != nil {
		return nil, fmt.Errorf("parse openai chat request: %w", err)
	}

	// Parse messages specifically.
	rawMsgs, ok := generic["messages"]
	if !ok {
		return nil, fmt.Errorf("messages field missing")
	}
	var msgs []openAIMessage
	if err := json.Unmarshal(rawMsgs, &msgs); err != nil {
		return nil, fmt.Errorf("parse messages: %w", err)
	}

	// Build Anthropic messages.
	var anthropicMsgs []AnthropicMessage
	var systemText string

	for _, m := range msgs {
		switch m.Role {
		case "system":
			// Collect as system prompt.
			switch v := m.Content.(type) {
			case string:
				systemText = v
			default:
				b, _ := json.Marshal(v)
				systemText = string(b)
			}
		case "user":
			blocks, err := openAIContentToAnthropicBlocks(m.Content)
			if err != nil {
				return nil, fmt.Errorf("convert user message: %w", err)
			}
			blocksJSON, err := json.Marshal(blocks)
			if err != nil {
				return nil, fmt.Errorf("marshal user blocks: %w", err)
			}
			anthropicMsgs = append(anthropicMsgs, AnthropicMessage{
				Role:    "user",
				Content: blocksJSON,
			})
		case "assistant":
			// Pass assistant content through as text.
			text := openAIContentToString(m.Content)
			textBlock := AnthropicTextBlock{Type: "text", Text: text}
			blockJSON, _ := json.Marshal([]AnthropicTextBlock{textBlock})
			anthropicMsgs = append(anthropicMsgs, AnthropicMessage{
				Role:    "assistant",
				Content: blockJSON,
			})
		}
	}

	// Build Anthropic request.
	anthropicReq := AnthropicMessagesRequest{
		Model:    extractStringField(generic, "model"),
		Messages: anthropicMsgs,
		Stream:   extractBoolField(generic, "stream"),
	}
	if systemText != "" {
		b, _ := json.Marshal(systemText)
		anthropicReq.System = b
	}
	if v := extractIntField(generic, "max_tokens"); v > 0 {
		anthropicReq.MaxTokens = v
	} else {
		anthropicReq.MaxTokens = 4096 // sensible default required by Anthropic API
	}

	return json.Marshal(anthropicReq)
}

// openAIContentToAnthropicBlocks converts OpenAI content (string or []parts)
// to a slice of Anthropic content blocks (text/document/image).
func openAIContentToAnthropicBlocks(content any) ([]json.RawMessage, error) {
	// Re-marshal and parse as parts to handle the any interface.
	raw, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}

	// Plain string → single text block.
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		tb, _ := json.Marshal(AnthropicTextBlock{Type: "text", Text: text})
		return []json.RawMessage{tb}, nil
	}

	// Array of parts.
	var parts []json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		// Fallback: treat as text.
		tb, _ := json.Marshal(AnthropicTextBlock{Type: "text", Text: string(raw)})
		return []json.RawMessage{tb}, nil
	}

	var blocks []json.RawMessage
	for _, p := range parts {
		var typed struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
			File *struct {
				Filename string `json:"filename"`
				FileData string `json:"file_data"` // "data:<mime>;base64,<b64>"
			} `json:"file,omitempty"`
		}
		if err := json.Unmarshal(p, &typed); err != nil {
			continue
		}
		switch typed.Type {
		case "text":
			tb, _ := json.Marshal(AnthropicTextBlock{Type: "text", Text: typed.Text})
			blocks = append(blocks, tb)
		case "file":
			if typed.File == nil || typed.File.FileData == "" {
				continue
			}
			mediaType, b64Data, ok := parseDataURI(typed.File.FileData)
			if !ok {
				// Try treating the raw value as plain base64.
				b64Data = typed.File.FileData
				mediaType = "application/pdf"
			}
			db, _ := json.Marshal(AnthropicDocumentBlock{
				Type: "document",
				Source: AnthropicDocumentBlockSource{
					Type:      "base64",
					MediaType: mediaType,
					Data:      b64Data,
				},
			})
			blocks = append(blocks, db)
		}
	}

	if len(blocks) == 0 {
		tb, _ := json.Marshal(AnthropicTextBlock{Type: "text", Text: string(raw)})
		return []json.RawMessage{tb}, nil
	}
	return blocks, nil
}

// parseDataURI splits a "data:<mediaType>;base64,<data>" URI into its components.
// Returns the media type and base64 data, plus ok=true on success.
func parseDataURI(uri string) (mediaType, data string, ok bool) {
	// Expected format: data:<mediaType>;base64,<data>
	if !strings.HasPrefix(uri, "data:") {
		return "", uri, false
	}
	rest := uri[len("data:"):]
	semi := strings.Index(rest, ";")
	if semi < 0 {
		return "", uri, false
	}
	mediaType = rest[:semi]
	rest = rest[semi+1:]
	if !strings.HasPrefix(rest, "base64,") {
		return "", uri, false
	}
	data = rest[len("base64,"):]
	return mediaType, data, true
}

// openAIContentToString converts OpenAI content (string or parts) to a plain string.
func openAIContentToString(content any) string {
	raw, err := json.Marshal(content)
	if err != nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Parts: extract text parts only.
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return string(raw)
	}
	var texts []string
	for _, p := range parts {
		if p.Type == "text" && p.Text != "" {
			texts = append(texts, p.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// extractStringField reads a string value from a map[string]json.RawMessage.
func extractStringField(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}

// extractBoolField reads a bool value from a map[string]json.RawMessage.
func extractBoolField(m map[string]json.RawMessage, key string) bool {
	raw, ok := m[key]
	if !ok {
		return false
	}
	var b bool
	_ = json.Unmarshal(raw, &b)
	return b
}

// extractIntField reads an int value from a map[string]json.RawMessage.
func extractIntField(m map[string]json.RawMessage, key string) int {
	raw, ok := m[key]
	if !ok {
		return 0
	}
	var n int
	_ = json.Unmarshal(raw, &n)
	return n
}

