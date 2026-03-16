package service

import "testing"

func TestGatewayService_isModelSupportedByAccount_PassthroughBypassesRestrictions(t *testing.T) {
	svc := &GatewayService{}

	tests := []struct {
		name    string
		account *Account
		model   string
		want    bool
	}{
		{
			name: "OpenAI passthrough bypasses account model mapping",
			account: &Account{
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"model_mapping": map[string]any{"gpt-3.5-turbo": "gpt-3.5-turbo"},
				},
				Extra: map[string]any{"openai_passthrough": true},
			},
			model: "gpt-5.4",
			want:  true,
		},
		{
			name: "Anthropic passthrough bypasses account model mapping",
			account: &Account{
				Platform: PlatformAnthropic,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"model_mapping": map[string]any{"claude-3-5-sonnet-20241022": "claude-3-5-sonnet-20241022"},
				},
				Extra: map[string]any{"anthropic_passthrough": true},
			},
			model: "claude-opus-4-6",
			want:  true,
		},
		{
			name: "OpenAI non-passthrough still respects mapping allowlist",
			account: &Account{
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"model_mapping": map[string]any{"gpt-3.5-turbo": "gpt-3.5-turbo"},
				},
			},
			model: "gpt-5.4",
			want:  false,
		},
		{
			name: "Anthropic non-passthrough still respects mapping allowlist",
			account: &Account{
				Platform: PlatformAnthropic,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"model_mapping": map[string]any{"claude-3-5-sonnet-20241022": "claude-3-5-sonnet-20241022"},
				},
			},
			model: "claude-opus-4-6",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.isModelSupportedByAccount(tt.account, tt.model); got != tt.want {
				t.Fatalf("isModelSupportedByAccount(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}
