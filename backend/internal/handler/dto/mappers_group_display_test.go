package dto

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupFromService_RateMultiplierFallbackToReal(t *testing.T) {
	t.Parallel()

	g := &service.Group{
		ID:             1,
		Name:           "default",
		Platform:       "anthropic",
		RateMultiplier: 2.0,
	}
	out := GroupFromService(g)
	require.NotNil(t, out)
	require.Equal(t, 2.0, out.RateMultiplier, "no display set: user-facing rate falls back to real")
}

func TestGroupFromService_RateMultiplierReplacedByDisplay(t *testing.T) {
	t.Parallel()

	display := 1.5
	g := &service.Group{
		ID:                    2,
		Name:                  "biased",
		Platform:              "openai",
		RateMultiplier:        2.0,
		DisplayRateMultiplier: &display,
	}
	out := GroupFromService(g)
	require.NotNil(t, out)
	require.Equal(t, 1.5, out.RateMultiplier, "user-facing rate must reflect display value")
}

func TestGroupFromService_UserPayloadHidesAdminFields(t *testing.T) {
	t.Parallel()

	display := 0.5
	g := &service.Group{
		ID:                    3,
		Name:                  "stealth",
		Platform:              "openai",
		RateMultiplier:        3.0,
		DisplayRateMultiplier: &display,
		ClaudeCodePersona:     true,
		MCPXMLInject:          true,
	}
	out := GroupFromService(g)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	body := string(raw)
	require.NotContains(t, body, "display_rate_multiplier", "user payload must not leak display_rate_multiplier field name")
	require.NotContains(t, body, "claude_code_persona", "user payload must not leak persona toggle")
	require.NotContains(t, body, "mcp_xml_inject", "user payload must not leak mcp_xml_inject")
	require.NotContains(t, body, "real_rate_multiplier", "user payload must not leak real_rate_multiplier")
}

func TestGroupFromServiceAdmin_ExposesBothRates(t *testing.T) {
	t.Parallel()

	display := 1.2
	g := &service.Group{
		ID:                    4,
		Name:                  "admin-view",
		Platform:              "anthropic",
		RateMultiplier:        2.5,
		DisplayRateMultiplier: &display,
		ClaudeCodePersona:     true,
	}
	admin := GroupFromServiceAdmin(g)
	require.NotNil(t, admin)

	require.Equal(t, 2.5, admin.RateMultiplier, "admin Group.RateMultiplier carries real value")
	require.Equal(t, 2.5, admin.RealRateMultiplier, "admin RealRateMultiplier explicit field")
	require.NotNil(t, admin.DisplayRateMultiplier)
	require.Equal(t, 1.2, *admin.DisplayRateMultiplier)
	require.True(t, admin.ClaudeCodePersona)

	raw, err := json.Marshal(admin)
	require.NoError(t, err)
	body := string(raw)
	require.True(t, strings.Contains(body, `"display_rate_multiplier":1.2`), "admin payload must include display_rate_multiplier")
	require.True(t, strings.Contains(body, `"claude_code_persona":true`), "admin payload must expose persona toggle")
}

func TestGroupFromServiceAdmin_NoDisplay(t *testing.T) {
	t.Parallel()

	g := &service.Group{
		ID:             5,
		Name:           "no-display",
		Platform:       "openai",
		RateMultiplier: 1.0,
	}
	admin := GroupFromServiceAdmin(g)
	require.NotNil(t, admin)
	require.Nil(t, admin.DisplayRateMultiplier, "DisplayRateMultiplier nil when not configured")
	require.Equal(t, 1.0, admin.RealRateMultiplier)
}

func TestEffectiveDisplayRateMultiplier(t *testing.T) {
	t.Parallel()

	display := 0.9
	require.Equal(t, 0.9, (&service.Group{RateMultiplier: 2.0, DisplayRateMultiplier: &display}).EffectiveDisplayRateMultiplier())
	require.Equal(t, 2.0, (&service.Group{RateMultiplier: 2.0}).EffectiveDisplayRateMultiplier())
}

func TestGroupFromService_DisplayIconAndName(t *testing.T) {
	t.Parallel()

	g := &service.Group{
		ID:          6,
		Platform:    "openai",
		Name:        "internal-name",
		DisplayName: "Pro Plan",
		DisplayIcon: "claude-code",
	}
	out := GroupFromService(g)
	require.Equal(t, "Pro Plan", out.DisplayName)
	require.Equal(t, "claude-code", out.DisplayIcon)
}
