package service

// openAIResponsesUnsupportedFields are top-level Responses API fields that
// need to be stripped defensively before forwarding to upstream OpenAI/Codex
// endpoints.
var openAIResponsesUnsupportedFields = []string{
	"prompt_cache_retention",
	"safety_identifier",
}
