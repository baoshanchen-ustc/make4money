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

// NormalizeModelIDForCopilotUpstream converts client / Anthropic-style model ids to
// ids the GitHub Copilot API accepts:
//
//   - Known dated Anthropic ids (via claude.DenormalizeModelID) → short dash form
//   - Any claude-{sonnet|opus|haiku}-MAJOR-MINOR-YYYYMMDD → short dash form
//   - claude-{sonnet|opus|haiku}-MAJOR-MINOR → claude-*-MAJOR.MINOR
//
// Non-Claude ids (gpt-*, gemini-*, etc.) are returned unchanged.
func NormalizeModelIDForCopilotUpstream(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return model
	}
	model = claude.DenormalizeModelID(model)
	if m := claudeDatedMajorMinor.FindStringSubmatch(model); m != nil {
		model = "claude-" + m[1] + "-" + m[2] + "-" + m[3]
	}
	if m := claudeMajorMinorDash.FindStringSubmatch(model); m != nil {
		return "claude-" + m[1] + "-" + m[2] + "." + m[3]
	}
	return model
}
