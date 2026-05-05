package apicompat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// ResponsesToAnthropicRequest converts a Responses API request into an
// Anthropic Messages request. This is the reverse of AnthropicToResponses and
// enables Anthropic platform groups to accept OpenAI Responses API requests
// by converting them to the native /v1/messages format before forwarding upstream.
func ResponsesToAnthropicRequest(req *ResponsesRequest) (*AnthropicRequest, error) {
	system, messages, err := convertResponsesInputToAnthropic(req.Input)
	if err != nil {
		return nil, err
	}

	out := &AnthropicRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
	}

	if len(system) > 0 {
		out.System = system
	}

	// max_output_tokens → max_tokens
	if req.MaxOutputTokens != nil && *req.MaxOutputTokens > 0 {
		out.MaxTokens = *req.MaxOutputTokens
	}
	if out.MaxTokens == 0 {
		// Anthropic requires max_tokens; default to a sensible value.
		out.MaxTokens = 8192
	}

	// Convert tools
	if len(req.Tools) > 0 {
		out.Tools = convertResponsesToAnthropicTools(req.Tools)
	}

	// Convert tool_choice (reverse of convertAnthropicToolChoiceToResponses)
	if len(req.ToolChoice) > 0 {
		tc, err := convertResponsesToAnthropicToolChoice(req.ToolChoice)
		if err != nil {
			return nil, fmt.Errorf("convert tool_choice: %w", err)
		}
		out.ToolChoice = tc
	}

	// reasoning.effort → output_config.effort + thinking
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		effort := mapResponsesEffortToAnthropic(req.Reasoning.Effort)
		out.OutputConfig = &AnthropicOutputConfig{Effort: effort}
		// Enable thinking for non-low efforts
		if effort != "low" {
			out.Thinking = &AnthropicThinking{
				Type:         "enabled",
				BudgetTokens: defaultThinkingBudget(effort),
			}
		}
	}

	return out, nil
}

// defaultThinkingBudget returns a sensible thinking budget based on effort level.
func defaultThinkingBudget(effort string) int {
	switch effort {
	case "low":
		return 1024
	case "medium":
		return 4096
	case "high":
		return 10240
	case "max":
		return 32768
	default:
		return 10240
	}
}

// mapResponsesEffortToAnthropic converts OpenAI Responses reasoning effort to
// Anthropic effort levels. Reverse of mapAnthropicEffortToResponses.
//
//	low    → low
//	medium → medium
//	high   → high
//	xhigh  → max
func mapResponsesEffortToAnthropic(effort string) string {
	if effort == "xhigh" {
		return "max"
	}
	return effort // low→low, medium→medium, high→high, unknown→passthrough
}

// convertResponsesInputToAnthropic extracts system prompt and messages from
// a Responses API input array. Returns the system as raw JSON (for Anthropic's
// polymorphic system field) and a list of Anthropic messages.
func convertResponsesInputToAnthropic(inputRaw json.RawMessage) (json.RawMessage, []AnthropicMessage, error) {
	// Try as plain string input.
	var inputStr string
	if err := json.Unmarshal(inputRaw, &inputStr); err == nil {
		content, _ := json.Marshal(inputStr)
		return nil, []AnthropicMessage{{Role: "user", Content: content}}, nil
	}

	var rawItems []json.RawMessage
	trimmedInput := bytes.TrimSpace(inputRaw)
	switch {
	case len(trimmedInput) == 0:
		return nil, nil, fmt.Errorf("parse responses input: empty input")
	case trimmedInput[0] == '[':
		if err := json.Unmarshal(trimmedInput, &rawItems); err != nil {
			return nil, nil, fmt.Errorf("parse responses input: %w", err)
		}
	case trimmedInput[0] == '{':
		rawItems = []json.RawMessage{trimmedInput}
	default:
		return nil, nil, fmt.Errorf("parse responses input: expected string, object, or array")
	}

	var system json.RawMessage
	var messages []AnthropicMessage

	for _, rawItem := range rawItems {
		var item ResponsesInputItem
		if err := json.Unmarshal(rawItem, &item); err != nil {
			return nil, nil, fmt.Errorf("parse responses input item: %w", err)
		}

		switch {
		case item.Role == "system":
			// System prompt → Anthropic system field
			text := extractTextFromContent(item.Content)
			if text != "" {
				system, _ = json.Marshal(text)
			}

		case item.Type == "input_text" || item.Type == "text":
			blocks := convertResponsesContentPartToAnthropicRawBlocks(rawItem, false)
			if len(blocks) > 0 {
				content, _ := marshalAnthropicRawBlocks(blocks...)
				messages = append(messages, AnthropicMessage{
					Role:    "user",
					Content: content,
				})
			}

		case item.Type == "input_image" || item.Type == "image_url":
			blocks := convertResponsesContentPartToAnthropicRawBlocks(rawItem, false)
			if len(blocks) > 0 {
				content, _ := marshalAnthropicRawBlocks(blocks...)
				messages = append(messages, AnthropicMessage{
					Role:    "user",
					Content: content,
				})
			}

		case item.Type == "function_call":
			// function_call → assistant message with tool_use block
			input := json.RawMessage("{}")
			if item.Arguments != "" {
				input = json.RawMessage(item.Arguments)
			}
			block := AnthropicContentBlock{
				Type:  "tool_use",
				ID:    fromResponsesCallIDToAnthropic(item.CallID),
				Name:  item.Name,
				Input: input,
			}
			blockJSON, _ := json.Marshal([]AnthropicContentBlock{block})
			messages = append(messages, AnthropicMessage{
				Role:    "assistant",
				Content: blockJSON,
			})

		case item.Type == "function_call_output":
			// function_call_output → user message with tool_result block
			outputContent := item.Output
			if outputContent == "" {
				outputContent = "(empty)"
			}
			contentJSON, _ := json.Marshal(outputContent)
			block := AnthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: fromResponsesCallIDToAnthropic(item.CallID),
				Content:   contentJSON,
			}
			blockJSON, _ := json.Marshal([]AnthropicContentBlock{block})
			messages = append(messages, AnthropicMessage{
				Role:    "user",
				Content: blockJSON,
			})

		case item.Role == "user":
			content, err := convertResponsesUserToAnthropicContent(item.Content)
			if err != nil {
				return nil, nil, err
			}
			messages = append(messages, AnthropicMessage{
				Role:    "user",
				Content: content,
			})

		case item.Role == "assistant":
			content, err := convertResponsesAssistantToAnthropicContent(item.Content)
			if err != nil {
				return nil, nil, err
			}
			messages = append(messages, AnthropicMessage{
				Role:    "assistant",
				Content: content,
			})

		default:
			// Unknown role/type — attempt as user message
			if item.Content != nil {
				content, err := convertResponsesUserToAnthropicContent(item.Content)
				if err != nil {
					return nil, nil, err
				}
				messages = append(messages, AnthropicMessage{
					Role:    "user",
					Content: content,
				})
			}
		}
	}

	// Merge consecutive same-role messages (Anthropic requires alternating roles)
	messages = mergeConsecutiveMessages(messages)

	return system, messages, nil
}

// extractTextFromContent extracts text from a content field that may be a
// plain string or an array of content parts.
func extractTextFromContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []ResponsesContentPart
	if err := json.Unmarshal(raw, &parts); err == nil {
		var texts []string
		for _, p := range parts {
			if (p.Type == "input_text" || p.Type == "output_text" || p.Type == "text") && p.Text != "" {
				texts = append(texts, p.Text)
			}
		}
		return strings.Join(texts, "\n\n")
	}
	return ""
}

// convertResponsesUserToAnthropicContent converts a Responses user message
// content field into Anthropic content blocks JSON.
func convertResponsesUserToAnthropicContent(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.Marshal("") // empty string content
	}

	// Try plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return json.Marshal(s)
	}

	blocks := convertResponsesContentToAnthropicRawBlocks(raw, false)

	if len(blocks) == 0 {
		return json.Marshal("")
	}
	return marshalAnthropicRawBlocks(blocks...)
}

// convertResponsesAssistantToAnthropicContent converts a Responses assistant
// message content field into Anthropic content blocks JSON.
func convertResponsesAssistantToAnthropicContent(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.Marshal([]AnthropicContentBlock{{Type: "text", Text: ""}})
	}

	// Try plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return json.Marshal([]AnthropicContentBlock{{Type: "text", Text: s}})
	}

	blocks := convertResponsesContentToAnthropicRawBlocks(raw, true)

	if len(blocks) == 0 {
		blocks = append(blocks, makeAnthropicTextRawBlock("", nil))
	}
	return marshalAnthropicRawBlocks(blocks...)
}

func convertResponsesContentToAnthropicRawBlocks(raw json.RawMessage, assistant bool) []json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}

	var parts []json.RawMessage
	switch trimmed[0] {
	case '[':
		if err := json.Unmarshal(trimmed, &parts); err != nil {
			return nil
		}
	case '{':
		parts = []json.RawMessage{trimmed}
	default:
		var s string
		if err := json.Unmarshal(trimmed, &s); err == nil {
			return []json.RawMessage{makeAnthropicTextRawBlock(s, nil)}
		}
		return nil
	}

	var blocks []json.RawMessage
	for _, part := range parts {
		blocks = append(blocks, convertResponsesContentPartToAnthropicRawBlocks(part, assistant)...)
	}
	return blocks
}

func convertResponsesContentPartToAnthropicRawBlocks(raw json.RawMessage, assistant bool) []json.RawMessage {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []json.RawMessage{makeAnthropicTextRawBlock(s, nil)}
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}

	partType := rawJSONFieldString(obj, "type")
	switch partType {
	case "input_text", "output_text", "text":
		if text := rawJSONFieldString(obj, "text"); text != "" {
			return []json.RawMessage{makeAnthropicTextRawBlock(text, obj["cache_control"])}
		}
	case "input_image", "image_url":
		if source := responsesImageSourceFromObject(obj); source != nil {
			return []json.RawMessage{makeAnthropicImageRawBlock(source, obj["cache_control"])}
		}
	case "image":
		if _, ok := obj["source"]; ok {
			return []json.RawMessage{raw}
		}
		if source := responsesImageSourceFromObject(obj); source != nil {
			return []json.RawMessage{makeAnthropicImageRawBlock(source, obj["cache_control"])}
		}
	case "tool_use":
		if assistant {
			return []json.RawMessage{raw}
		}
	case "tool_result":
		if !assistant {
			return []json.RawMessage{raw}
		}
	case "thinking":
		if assistant {
			return []json.RawMessage{raw}
		}
	case "document", "server_tool_use", "tool_reference", "web_search_tool_result":
		return []json.RawMessage{raw}
	}

	if text := rawJSONFieldString(obj, "text"); text != "" {
		return []json.RawMessage{makeAnthropicTextRawBlock(text, obj["cache_control"])}
	}
	return nil
}

func marshalAnthropicRawBlocks(blocks ...json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(blocks)
}

func makeAnthropicTextRawBlock(text string, cacheControl json.RawMessage) json.RawMessage {
	obj := map[string]json.RawMessage{
		"type": json.RawMessage(`"text"`),
		"text": mustMarshalRaw(text),
	}
	if len(cacheControl) > 0 {
		obj["cache_control"] = cacheControl
	}
	raw, _ := json.Marshal(obj)
	return raw
}

func makeAnthropicImageRawBlock(source *AnthropicImageSource, cacheControl json.RawMessage) json.RawMessage {
	obj := map[string]json.RawMessage{
		"type":   json.RawMessage(`"image"`),
		"source": mustMarshalRaw(source),
	}
	if len(cacheControl) > 0 {
		obj["cache_control"] = cacheControl
	}
	raw, _ := json.Marshal(obj)
	return raw
}

func responsesImageSourceFromObject(obj map[string]json.RawMessage) *AnthropicImageSource {
	if rawSource, ok := obj["source"]; ok {
		var source AnthropicImageSource
		if err := json.Unmarshal(rawSource, &source); err == nil && source.Type != "" {
			return &source
		}
	}

	if url := rawJSONFieldString(obj, "image_url"); url != "" {
		return dataURIToAnthropicImageSource(url)
	}

	var imageURLObj map[string]json.RawMessage
	if rawImageURL, ok := obj["image_url"]; ok && json.Unmarshal(rawImageURL, &imageURLObj) == nil {
		if url := rawJSONFieldString(imageURLObj, "url"); url != "" {
			return dataURIToAnthropicImageSource(url)
		}
	}

	if url := rawJSONFieldString(obj, "url"); url != "" {
		return dataURIToAnthropicImageSource(url)
	}
	return nil
}

func rawJSONFieldString(obj map[string]json.RawMessage, key string) string {
	raw, ok := obj[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

func mustMarshalRaw(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}

// fromResponsesCallIDToAnthropic converts an OpenAI function call ID back to
// Anthropic format. Reverses toResponsesCallID.
func fromResponsesCallIDToAnthropic(id string) string {
	// If it has our "fc_" prefix wrapping a known Anthropic prefix, strip it
	if after, ok := strings.CutPrefix(id, "fc_"); ok {
		if strings.HasPrefix(after, "toolu_") || strings.HasPrefix(after, "call_") {
			return after
		}
	}
	// Generate a synthetic Anthropic tool ID
	if !strings.HasPrefix(id, "toolu_") && !strings.HasPrefix(id, "call_") {
		return "toolu_" + id
	}
	return id
}

// dataURIToAnthropicImageSource parses a data URI into an AnthropicImageSource.
func dataURIToAnthropicImageSource(dataURI string) *AnthropicImageSource {
	if !strings.HasPrefix(dataURI, "data:") {
		return nil
	}
	// Format: data:<media_type>;base64,<data>
	rest := strings.TrimPrefix(dataURI, "data:")
	semicolonIdx := strings.Index(rest, ";")
	if semicolonIdx < 0 {
		return nil
	}
	mediaType := rest[:semicolonIdx]
	rest = rest[semicolonIdx+1:]
	if !strings.HasPrefix(rest, "base64,") {
		return nil
	}
	data := strings.TrimPrefix(rest, "base64,")
	return &AnthropicImageSource{
		Type:      "base64",
		MediaType: mediaType,
		Data:      data,
	}
}

// mergeConsecutiveMessages merges consecutive messages with the same role
// because Anthropic requires alternating user/assistant turns.
func mergeConsecutiveMessages(messages []AnthropicMessage) []AnthropicMessage {
	if len(messages) <= 1 {
		return messages
	}

	var merged []AnthropicMessage
	for _, msg := range messages {
		if len(merged) == 0 || merged[len(merged)-1].Role != msg.Role {
			merged = append(merged, msg)
			continue
		}

		// Same role — merge content arrays
		last := &merged[len(merged)-1]
		lastBlocks := parseContentBlocks(last.Content)
		newBlocks := parseContentBlocks(msg.Content)
		combined := append(lastBlocks, newBlocks...)
		last.Content, _ = json.Marshal(combined)
	}
	return merged
}

// parseContentBlocks attempts to parse content as []AnthropicContentBlock.
// If it's a string, wraps it in a text block.
func parseContentBlocks(raw json.RawMessage) []AnthropicContentBlock {
	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		return blocks
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []AnthropicContentBlock{{Type: "text", Text: s}}
	}
	return nil
}

// convertResponsesToAnthropicTools maps Responses API tools to Anthropic format.
// Reverse of convertAnthropicToolsToResponses.
func convertResponsesToAnthropicTools(tools []ResponsesTool) []AnthropicTool {
	var out []AnthropicTool
	for _, t := range tools {
		switch t.Type {
		case "web_search", "google_search", "web_search_20250305":
			out = append(out, AnthropicTool{
				Type: "web_search_20250305",
				Name: "web_search",
			})
		case "function":
			out = append(out, AnthropicTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: normalizeAnthropicInputSchema(t.Parameters),
			})
		default:
			// Pass through unknown tool types
			out = append(out, AnthropicTool{
				Type:        t.Type,
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.Parameters,
			})
		}
	}
	return out
}

// normalizeAnthropicInputSchema ensures the input_schema has a "type" field.
func normalizeAnthropicInputSchema(schema json.RawMessage) json.RawMessage {
	if len(schema) == 0 || string(schema) == "null" {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return schema
}

// convertResponsesToAnthropicToolChoice maps Responses tool_choice to Anthropic format.
// Reverse of convertAnthropicToolChoiceToResponses.
//
//	"auto"                                     → {"type":"auto"}
//	"required"                                 → {"type":"any"}
//	"none"                                     → {"type":"none"}
//	{"type":"function","name":"X"}                 → {"type":"tool","name":"X"}
//	{"type":"function","function":{"name":"X"}}     → {"type":"tool","name":"X"} // legacy
func convertResponsesToAnthropicToolChoice(raw json.RawMessage) (json.RawMessage, error) {
	// Try as string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch s {
		case "auto":
			return json.Marshal(map[string]string{"type": "auto"})
		case "required":
			return json.Marshal(map[string]string{"type": "any"})
		case "none":
			return json.Marshal(map[string]string{"type": "none"})
		default:
			return raw, nil
		}
	}

	// Try as object with type=function
	var tc struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &tc); err == nil && tc.Type == "function" {
		name := strings.TrimSpace(tc.Name)
		if name == "" {
			name = strings.TrimSpace(tc.Function.Name)
		}
		if name == "" {
			return raw, nil
		}
		return json.Marshal(map[string]string{
			"type": "tool",
			"name": name,
		})
	}

	// Pass through unknown
	return raw, nil
}
