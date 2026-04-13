package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildHourlyMetricsFilter_DefaultGroupNull(t *testing.T) {
	clause, args, nextIdx := buildHourlyMetricsFilter("", nil, 3)
	require.Equal(t, " AND platform IS NULL AND group_id IS NULL", clause)
	require.Equal(t, 0, len(args))
	require.Equal(t, 3, nextIdx)
}

func TestBuildHourlyMetricsFilter_PlatformFilterGroupNull(t *testing.T) {
	clause, args, nextIdx := buildHourlyMetricsFilter("openai", nil, 3)
	require.Equal(t, " AND platform = $3 AND group_id IS NULL", clause)
	require.Equal(t, []any{"openai"}, args)
	require.Equal(t, 4, nextIdx)
}

func TestBuildHourlyMetricsFilter_GroupSpecified(t *testing.T) {
	groupID := int64(42)
	clause, args, nextIdx := buildHourlyMetricsFilter("openai", &groupID, 3)
	require.Equal(t, " AND group_id = $3 AND platform = $4", clause)
	require.Equal(t, []any{groupID, "openai"}, args)
	require.Equal(t, 5, nextIdx)
}
