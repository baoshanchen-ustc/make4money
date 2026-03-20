package copilot

import (
	"regexp"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

// claudeDatedMajorMinor matches Anthropic-style Claude ids with an 8-digit snapshot
// suffix that Copilot does not accept (e.g. claude-haiku-4-5-20251001).
var claudeDatedMajorMinor = regexp.MustCompile(
	`^claude-(sonnet|opus|haiku)-(\d+)-(\d+)-\d{8}$`,
)

// claudeMajorMinorDash matches Copilot gateway dash form before dot conversion
// (e.g. claude-haiku-4-5 → claude-haiku-4.5).
var claudeMajorMinorDash = regexp.MustCompile(
	`^claude-(sonnet|opus|haiku)-(\d+)-(\d+)$`,
)

// claudeCodeFamilyShorthand matches Claude Code UI / settings shortcuts such as
// "opus[1m]" (1M context Opus) or bare "sonnet". These are not valid Copilot
// wire ids and cause HTTP 400 if forwarded unchanged.
var claudeCodeFamilyShorthand = regexp.MustCompile(
	`^(?i)(sonnet|opus|haiku)(\[1m\])?$`,
)

// expandClaudeCodeModelShorthand maps Claude Code family shortcuts to full
// claude-*-MAJOR-MINOR dash ids so NormalizeModelIDForCopilotUpstream can strip
// dated suffixes and emit Copilot dot form (e.g. claude-sonnet-4.6).
func expandClaudeCodeModelShorthand(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return model
	}
	m := claudeCodeFamilyShorthand.FindStringSubmatch(model)
	if m == nil {
		return model
	}
	switch strings.ToLower(m[1]) {
	case "opus":
		return "claude-opus-4-6"
	case "sonnet":
		return "claude-sonnet-4-6"
	case "haiku":
		return "claude-haiku-4-5"
	default:
		return model
	}
}

// NormalizeModelIDForCopilotUpstream converts client / Anthropic-style model ids to
// ids the GitHub Copilot API accepts:
//
//   - Known dated Anthropic ids (via claude.DenormalizeModelID) → short dash form
//   - Any claude-{sonnet|opus|haiku}-MAJOR-MINOR-YYYYMMDD → short dash form
//   - claude-{sonnet|opus|haiku}-MAJOR-MINOR → claude-*-MAJOR.MINOR (wire id)
//
// Do not collapse Sonnet/Opus 4 minor variants to claude-*-4 alone: the live
// Copilot /chat/completions endpoint returns model_not_supported for that id.
//
// Non-Claude ids (gpt-*, gemini-*, etc.) are returned unchanged.
func NormalizeModelIDForCopilotUpstream(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return model
	}
	model = expandClaudeCodeModelShorthand(model)
	model = claude.DenormalizeModelID(model)
	if m := claudeDatedMajorMinor.FindStringSubmatch(model); m != nil {
		model = "claude-" + m[1] + "-" + m[2] + "-" + m[3]
	}
	if m := claudeMajorMinorDash.FindStringSubmatch(model); m != nil {
		return "claude-" + m[1] + "-" + m[2] + "." + m[3]
	}
	return model
}
