package service

import "testing"

func TestResolveOpenAIForwardModel(t *testing.T) {
	tests := []struct {
		name               string
		account            *Account
		requestedModel     string
		defaultMappedModel string
		expectedModel      string
	}{
		{
			name: "falls back to group default when account has no mapping",
			account: &Account{
				Credentials: map[string]any{},
			},
			requestedModel:     "claude-opus-4-6",
			defaultMappedModel: "gpt-5.1",
			expectedModel:      "gpt-5.1",
		},
		{
			name: "preserves gpt-5.5 instead of group default",
			account: &Account{
				Credentials: map[string]any{},
			},
			requestedModel:     "gpt-5.5",
			defaultMappedModel: "gpt-5.1",
			expectedModel:      "gpt-5.5",
		},
		{
			name: "preserves exact passthrough mapping instead of group default",
			account: &Account{
				Credentials: map[string]any{
					"model_mapping": map[string]any{
						"gpt-5.5": "gpt-5.5",
					},
				},
			},
			requestedModel:     "gpt-5.5",
			defaultMappedModel: "gpt-5.1",
			expectedModel:      "gpt-5.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveOpenAIForwardModel(tt.account, tt.requestedModel, tt.defaultMappedModel); got != tt.expectedModel {
				t.Fatalf("resolveOpenAIForwardModel(...) = %q, want %q", got, tt.expectedModel)
			}
		})
	}
}
