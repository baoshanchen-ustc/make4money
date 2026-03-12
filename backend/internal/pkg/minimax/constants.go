// Package minimax provides constants and helpers for MiniMax API integration.
package minimax

// DefaultBaseURL is the default MiniMax Anthropic-compatible API base URL (overseas).
const DefaultBaseURL = "https://api.minimax.io/anthropic"

// Model represents a MiniMax model.
type Model struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
}

// DefaultModels is the list of supported MiniMax models.
var DefaultModels = []Model{
	{
		ID:          "MiniMax-M2.5",
		Type:        "model",
		DisplayName: "MiniMax M2.5",
		CreatedAt:   "2025-06-01T00:00:00Z",
	},
	{
		ID:          "MiniMax-M2.5-highspeed",
		Type:        "model",
		DisplayName: "MiniMax M2.5 High Speed",
		CreatedAt:   "2025-06-01T00:00:00Z",
	},
}

// DefaultModelIDs returns the default model ID list.
func DefaultModelIDs() []string {
	ids := make([]string, len(DefaultModels))
	for i, m := range DefaultModels {
		ids[i] = m.ID
	}
	return ids
}

// DefaultTestModel is the default model for testing MiniMax accounts.
const DefaultTestModel = "MiniMax-M2.5"
