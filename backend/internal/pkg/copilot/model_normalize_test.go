package copilot

import "testing"

func TestNormalizeModelIDForCopilotUpstream(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"gpt-4o", "gpt-4o"},
		{"gemini-2.0-flash-001", "gemini-2.0-flash-001"},
		{"claude-haiku-4-5-20251001", "claude-haiku-4.5"},
		{"claude-haiku-4-5", "claude-haiku-4.5"},
		{"claude-haiku-4.5", "claude-haiku-4.5"},
		{"claude-sonnet-4-5-20250929", "claude-sonnet-4.5"},
		{"claude-sonnet-4-5", "claude-sonnet-4.5"},
		{"claude-sonnet-4-6", "claude-sonnet-4.5"},
		{"claude-sonnet-4.6", "claude-sonnet-4.5"},
		{"claude-sonnet-4-6-20260218", "claude-sonnet-4.5"},
		{"claude-opus-4-5-20251101", "claude-opus-4.5"},
		{"claude-opus-4-6", "claude-opus-4.5"},
		{"claude-opus-4.6", "claude-opus-4.5"},
		{"claude-sonnet-4", "claude-sonnet-4"},
		{"claude-3.5-sonnet", "claude-3.5-sonnet"},
		{"claude-haiku-4-5-20991231", "claude-haiku-4.5"},
		{"opus[1m]", "claude-opus-4.5"},
		{"Opus[1m]", "claude-opus-4.5"},
		{"opus", "claude-opus-4.5"},
		{"sonnet", "claude-sonnet-4.5"},
		{"sonnet[1m]", "claude-sonnet-4.5"},
		{"haiku", "claude-haiku-4.5"},
		{"Haiku[1m]", "claude-haiku-4.5"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := NormalizeModelIDForCopilotUpstream(tt.in); got != tt.want {
				t.Errorf("NormalizeModelIDForCopilotUpstream(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
