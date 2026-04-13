package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestDetailHintIncludesKindAndDuration(t *testing.T) {
	filter := &service.OpsRequestDetailFilter{
		Kind:          "error",
		MinDurationMs: func() *int { v := 200; return &v }(),
		RequestID:     "req-123",
	}
	hint := buildRequestDetailHint(filter)
	require.Contains(t, hint, "kind=error")
	require.Contains(t, hint, "200ms")
	require.Contains(t, hint, "req-123")
}

func TestBuildRequestDetailHintDefaults(t *testing.T) {
	hint := buildRequestDetailHint(nil)
	require.Contains(t, hint, "raw usage_logs")
}

func TestRequestErrorCorrelationHint(t *testing.T) {
	hint := buildRequestErrorCorrelationHint()
	require.Contains(t, hint, "failure-split guidance")
	require.Contains(t, hint, "upstream-errors")
}

func TestUpstreamErrorCorrelationHint(t *testing.T) {
	hint := buildUpstreamErrorCorrelationHint()
	require.Contains(t, hint, "matching request detail")
	require.Contains(t, hint, "consult failure split")
}
