package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AnthropicToResponses converts an Anthropic Messages request directly into
// a Responses API request. This preserves fields that would be lost in a
// Chat Completions intermediary round-trip (e.g. thinking, cache_control,
// structured system prompts).
func AnthropicToResponses(req *AnthropicRequest) (*ResponsesRequest, error) {
	input, err := convertAnthropicToResponsesInput(req.System, req.Messages)
	if err != nil {
		return nil, err
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	out := &ResponsesRequest{
		Model:       req.Model,
		Input:       inputJSON,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		Include:     responsesIncludeForAnthropicTools(req.Tools),
	}

	storeFalse := false
	out.Store = &storeFalse

	if req.MaxTokens > 0 {
		v := req.MaxTokens
		if v < minMaxOutputTokens {
			v = minMaxOutputTokens
		}
		out.MaxOutputTokens = &v
	}

	if len(req.Tools) > 0 {
		out.Tools = convertAnthropicToolsToResponses(req.Tools)
	}

	// Determine reasoning effort: only output_config.effort controls the
	// level; thinking.type is ignored. Default is high when unset (both
	// Anthropic and OpenAI default to high).
	// Anthropic levels map 1:1 to OpenAI: low→low, medium→medium, high→high, max→xhigh.
	effort := "high" // default → both sides' default
	if req.OutputConfig != nil && req.OutputConfig.Effort != "" {
		effort = req.OutputConfig.Effort
	}
	out.Reasoning = &ResponsesReasoning{
		Effort:  mapAnthropicEffortToResponses(effort),
		Summary: "auto",
	}

	// Convert tool_choice
	if len(req.ToolChoice) > 0 {
		tc, parallelToolCalls, err := convertAnthropicToolChoiceToResponses(req.ToolChoice)
		if err != nil {
			return nil, fmt.Errorf("convert tool_choice: %w", err)
		}
		out.ToolChoice = tc
		out.ParallelToolCalls = parallelToolCalls
	}

	return out, nil
}

func responsesIncludeForAnthropicTools(tools []AnthropicTool) []string {
	include := []string{"reasoning.encrypted_content"}
	for _, tool := range tools {
		if strings.HasPrefix(tool.Type, "web_search") {
			include = append(include, "web_search_call.action.sources")
			break
		}
	}
	return include
}

// convertAnthropicToolChoiceToResponses maps Anthropic tool_choice to Responses format.
//
//	{"type":"auto"}            → "auto"
//	{"type":"any"}             → "required"
//	{"type":"none"}            → "none"
//	{"type":"tool","name":"X"} → {"type":"function","function":{"name":"X"}}
func convertAnthropicToolChoiceToResponses(raw json.RawMessage) (json.RawMessage, *bool, error) {
	var tc struct {
		Type                   string `json:"type"`
		Name                   string `json:"name"`
		DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
	}
	if err := json.Unmarshal(raw, &tc); err != nil {
		return nil, nil, err
	}

	var parallelToolCalls *bool
	if tc.DisableParallelToolUse {
		v := false
		parallelToolCalls = &v
	}

	switch tc.Type {
	case "auto":
		out, err := json.Marshal("auto")
		return out, parallelToolCalls, err
	case "any":
		out, err := json.Marshal("required")
		return out, parallelToolCalls, err
	case "none":
		out, err := json.Marshal("none")
		return out, parallelToolCalls, err
	case "tool":
		out, err := json.Marshal(map[string]any{
			"type":     "function",
			"function": map[string]string{"name": tc.Name},
		})
		return out, parallelToolCalls, err
	default:
		// Pass through unknown types as-is
		return raw, parallelToolCalls, nil
	}
}

// convertAnthropicToResponsesInput builds the Responses API input items array
// from the Anthropic system field and message list.
func convertAnthropicToResponsesInput(system json.RawMessage, msgs []AnthropicMessage) ([]ResponsesInputItem, error) {
	var out []ResponsesInputItem

	// System prompt → system role input item.
	if len(system) > 0 {
		sysText, err := parseAnthropicSystemPrompt(system)
		if err != nil {
			return nil, err
		}
		if sysText != "" {
			content, _ := json.Marshal(sysText)
			out = append(out, ResponsesInputItem{
				Role:    "system",
				Content: content,
			})
		}
	}

	for _, m := range msgs {
		items, err := anthropicMsgToResponsesItems(m)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

// parseAnthropicSystemPrompt handles the Anthropic system field which can be
// a plain string or an array of text blocks.
func parseAnthropicSystemPrompt(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", err
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

// anthropicMsgToResponsesItems converts a single Anthropic message into one
// or more Responses API input items.
func anthropicMsgToResponsesItems(m AnthropicMessage) ([]ResponsesInputItem, error) {
	switch m.Role {
	case "user":
		return anthropicUserToResponses(m.Content)
	case "assistant":
		return anthropicAssistantToResponses(m.Content)
	default:
		return anthropicUserToResponses(m.Content)
	}
}

// anthropicUserToResponses handles an Anthropic user message. Content can be a
// plain string or an array of blocks. tool_result blocks are extracted into
// function_call_output items. Image blocks are converted to input_image parts.
func anthropicUserToResponses(raw json.RawMessage) ([]ResponsesInputItem, error) {
	// Try plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		content, _ := json.Marshal(s)
		return []ResponsesInputItem{{Role: "user", Content: content}}, nil
	}

	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}

	var out []ResponsesInputItem
	var toolResultImageParts []ResponsesContentPart

	// Extract tool_result blocks → function_call_output items.
	// Images inside tool_results are extracted separately because the
	// Responses API function_call_output.output only accepts strings.
	for _, b := range blocks {
		if b.Type != "tool_result" {
			continue
		}
		outputText, imageParts := convertToolResultOutput(b)
		out = append(out, ResponsesInputItem{
			Type:   "function_call_output",
			CallID: toResponsesCallID(b.ToolUseID),
			Output: outputText,
		})
		toolResultImageParts = append(toolResultImageParts, imageParts...)
	}

	// Remaining text + image blocks → user message with content parts.
	// Also include images extracted from tool_results so the model can see them.
	var parts []ResponsesContentPart
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				parts = append(parts, ResponsesContentPart{Type: "input_text", Text: b.Text})
			}
		case "document":
			if part := anthropicDocumentToResponsesPart(b); part != nil {
				parts = append(parts, *part)
			}
		case "image":
			if uri := anthropicImageToDataURI(b.Source); uri != "" {
				parts = append(parts, ResponsesContentPart{Type: "input_image", ImageURL: uri})
			}
		}
	}
	parts = append(parts, toolResultImageParts...)

	if len(parts) > 0 {
		if text, ok := collapseResponsesPlainTextParts(parts); ok {
			content, _ := json.Marshal(text)
			out = append(out, ResponsesInputItem{Role: "user", Content: content})
			return out, nil
		}

		content, err := json.Marshal(parts)
		if err != nil {
			return nil, err
		}
		out = append(out, ResponsesInputItem{Role: "user", Content: content})
	}

	return out, nil
}

func collapseResponsesPlainTextParts(parts []ResponsesContentPart) (string, bool) {
	if len(parts) == 0 {
		return "", false
	}

	var b strings.Builder
	for _, part := range parts {
		if part.Type != "input_text" {
			return "", false
		}
		_, _ = b.WriteString(part.Text)
	}
	return b.String(), true
}

// anthropicAssistantToResponses handles an Anthropic assistant message.
// Text content → assistant message with output_text parts.
// tool_use blocks → function_call items.
// thinking blocks → ignored (OpenAI doesn't accept them as input).
func anthropicAssistantToResponses(raw json.RawMessage) ([]ResponsesInputItem, error) {
	// Try plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		parts := []ResponsesContentPart{{Type: "output_text", Text: s}}
		partsJSON, err := json.Marshal(parts)
		if err != nil {
			return nil, err
		}
		return []ResponsesInputItem{{Role: "assistant", Content: partsJSON}}, nil
	}

	var blocks []AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, err
	}

	var items []ResponsesInputItem

	// Text content → assistant message with output_text content parts.
	text := extractAnthropicTextFromBlocks(blocks)
	if text != "" {
		parts := []ResponsesContentPart{{Type: "output_text", Text: text}}
		partsJSON, err := json.Marshal(parts)
		if err != nil {
			return nil, err
		}
		items = append(items, ResponsesInputItem{Role: "assistant", Content: partsJSON})
	}

	// tool_use → function_call items.
	for _, b := range blocks {
		if b.Type != "tool_use" {
			continue
		}
		args := "{}"
		if len(b.Input) > 0 {
			args = string(b.Input)
		}
		fcID := toResponsesCallID(b.ID)
		items = append(items, ResponsesInputItem{
			Type:      "function_call",
			CallID:    fcID,
			Name:      b.Name,
			Arguments: args,
		})
	}

	return items, nil
}

// toResponsesCallID converts an Anthropic tool ID (toolu_xxx / call_xxx) to a
// Responses API function_call ID that starts with "fc_".
func toResponsesCallID(id string) string {
	if strings.HasPrefix(id, "fc_") {
		return id
	}
	return "fc_" + id
}

// fromResponsesCallID reverses toResponsesCallID, stripping the "fc_" prefix
// that was added during request conversion.
func fromResponsesCallID(id string) string {
	if after, ok := strings.CutPrefix(id, "fc_"); ok {
		// Only strip if the remainder doesn't look like it was already "fc_" prefixed.
		// E.g. "fc_toolu_xxx" → "toolu_xxx", "fc_call_xxx" → "call_xxx"
		if strings.HasPrefix(after, "toolu_") || strings.HasPrefix(after, "call_") {
			return after
		}
	}
	return id
}

// anthropicImageToDataURI converts an AnthropicImageSource to a data URI string.
// Returns "" if the source is nil or has no data.
func anthropicImageToDataURI(src *AnthropicImageSource) string {
	if src == nil || src.Data == "" {
		return ""
	}
	mediaType := src.MediaType
	if mediaType == "" {
		mediaType = "image/png"
	}
	return "data:" + mediaType + ";base64," + src.Data
}

func anthropicDocumentToResponsesPart(b AnthropicContentBlock) *ResponsesContentPart {
	if b.Source == nil {
		return nil
	}

	part := &ResponsesContentPart{
		Type:     "input_file",
		Filename: strings.TrimSpace(b.Title),
	}

	switch strings.ToLower(strings.TrimSpace(b.Source.Type)) {
	case "base64":
		if b.Source.Data == "" {
			return nil
		}
		part.FileData = b.Source.Data
	case "url":
		if b.Source.URL == "" {
			return nil
		}
		part.FileURL = b.Source.URL
	case "file":
		if b.Source.FileID == "" {
			return nil
		}
		part.FileID = b.Source.FileID
	default:
		return nil
	}

	return part
}

// convertToolResultOutput extracts text and image content from a tool_result
// block. Returns the text as a string for the function_call_output Output
// field, plus any image parts that must be sent in a separate user message
// (the Responses API output field only accepts strings).
func convertToolResultOutput(b AnthropicContentBlock) (string, []ResponsesContentPart) {
	if len(b.Content) == 0 {
		return "(empty)", nil
	}

	// Try plain string content.
	var s string
	if err := json.Unmarshal(b.Content, &s); err == nil {
		if s == "" {
			s = "(empty)"
		}
		return s, nil
	}

	// Array of content blocks — may contain text and/or images.
	var inner []AnthropicContentBlock
	if err := json.Unmarshal(b.Content, &inner); err != nil {
		return "(empty)", nil
	}

	// Separate text (for function_call_output) from images (for user message).
	var textParts []string
	var imageParts []ResponsesContentPart
	var toolReferences []anthropicToolReferenceEnvelope
	for _, ib := range inner {
		switch ib.Type {
		case "text":
			if ib.Text != "" {
				textParts = append(textParts, ib.Text)
			}
		case "image":
			if uri := anthropicImageToDataURI(ib.Source); uri != "" {
				imageParts = append(imageParts, ResponsesContentPart{Type: "input_image", ImageURL: uri})
			}
		case "tool_reference":
			if ib.ToolName != "" {
				toolReferences = append(toolReferences, anthropicToolReferenceEnvelope{ToolName: ib.ToolName})
			}
		}
	}

	if len(toolReferences) > 0 {
		payload, err := json.Marshal(anthropicToolResultEnvelope{
			Format:         anthropicToolResultEnvelopeFormat,
			Text:           textParts,
			ToolReferences: toolReferences,
		})
		if err == nil {
			return string(payload), imageParts
		}
	}

	text := strings.Join(textParts, "\n\n")
	if text == "" {
		text = "(empty)"
	}
	return text, imageParts
}

// extractAnthropicTextFromBlocks joins all text blocks, ignoring thinking/
// tool_use/tool_result blocks.
func extractAnthropicTextFromBlocks(blocks []AnthropicContentBlock) string {
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// mapAnthropicEffortToResponses converts Anthropic reasoning effort levels to
// OpenAI Responses API effort levels.
//
// Both APIs default to "high". The mapping is 1:1 for shared levels;
// only Anthropic's "max" (Opus 4.6 exclusive) maps to OpenAI's "xhigh"
// (GPT-5.2+ exclusive) as both represent the highest reasoning tier.
//
//	low    → low
//	medium → medium
//	high   → high
//	max    → xhigh
func mapAnthropicEffortToResponses(effort string) string {
	if effort == "max" {
		return "xhigh"
	}
	return effort // low→low, medium→medium, high→high, unknown→passthrough
}

// convertAnthropicToolsToResponses maps Anthropic tool definitions to
// Responses API tools. Server-side tools like web_search are mapped to their
// OpenAI equivalents; regular tools become function tools.
func convertAnthropicToolsToResponses(tools []AnthropicTool) []ResponsesTool {
	var out []ResponsesTool
	for _, t := range tools {
		// Anthropic server tools like "web_search_20250305" → OpenAI {"type":"web_search"}
		if strings.HasPrefix(t.Type, "web_search") {
			out = append(out, ResponsesTool{Type: "web_search"})
			continue
		}
		out = append(out, ResponsesTool{
			Type:        "function",
			Name:        t.Name,
			Description: strings.TrimSpace(t.Description),
			Parameters:  normalizeToolParameters(t.InputSchema),
			Strict:      t.Strict,
		})
	}
	return out
}

func AugmentClaudeToolDescriptions(tools []ResponsesTool) {
	for i := range tools {
		if tools[i].Type != "function" || tools[i].Name == "" {
			continue
		}
		tools[i].Description = augmentClaudeToolDescription(tools[i].Name, tools[i].Description)
	}
}

func augmentClaudeToolDescription(name, description string) string {
	guidance := claudeToolRoutingGuidance(name)
	description = strings.TrimSpace(description)
	if guidance == "" {
		return description
	}
	if description == "" {
		return guidance
	}
	if strings.Contains(description, guidance) {
		return description
	}
	return guidance + "\n\n" + description
}

func claudeToolRoutingGuidance(name string) string {
	switch canonicalClaudeToolName(name) {
	case "read":
		return "Use this when you need the exact contents of a known file in the local workspace."
	case "grep":
		return "Use this when you need to search the local workspace for specific text, symbols, or patterns."
	case "glob":
		return "Use this when you need to discover files by path pattern, extension, or directory structure."
	case "edit":
		return "Use this when you need to modify an existing local file in place."
	case "write":
		return "Use this when you need to create a file or replace the full contents of a local file."
	case "bash":
		return "Use this when you need shell commands, build steps, tests, or repository state that Read, Grep, and Glob cannot provide."
	case "websearch":
		return "Use this when you need current external information across multiple sources."
	case "webfetch":
		return "Use this when a specific URL is already known and the exact page contents matter."
	case "toolsearch":
		return "Use this when you need to discover which local or MCP-backed tool should be called next."
	case "askuserquestion":
		return "Use this when a missing human decision would materially change the work."
	case "teamcreate":
		return "Use this when the task can be split into independent sub-agents that should work in parallel."
	case "sendmessage":
		return "Use this when you need to coordinate with an existing teammate or return structured status to the team lead."
	}
	return ""
}

// normalizeToolParameters ensures the tool parameter schema is valid for
// OpenAI's Responses API, which requires "properties" on object schemas.
//
//   - nil/empty → {"type":"object","properties":{}}
//   - type=object without properties → adds "properties": {}
//   - otherwise → returned unchanged
func normalizeToolParameters(schema json.RawMessage) json.RawMessage {
	if len(schema) == 0 || string(schema) == "null" {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(schema, &m); err != nil {
		return schema
	}

	typ := m["type"]
	if string(typ) != `"object"` {
		return schema
	}

	if _, ok := m["properties"]; ok {
		return schema
	}

	m["properties"] = json.RawMessage(`{}`)
	out, err := json.Marshal(m)
	if err != nil {
		return schema
	}
	return out
}
