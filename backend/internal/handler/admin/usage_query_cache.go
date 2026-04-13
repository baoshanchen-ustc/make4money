package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

const usageStatsCacheTTL = 15 * time.Second

type usageStatsCacheKey struct {
	RangeMode   string `json:"range_mode"`
	StartTime   string `json:"start_time,omitempty"`
	EndTime     string `json:"end_time,omitempty"`
	Period      string `json:"period,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
	BucketStart string `json:"bucket_start,omitempty"`

	UserID      int64  `json:"user_id"`
	APIKeyID    int64  `json:"api_key_id"`
	AccountID   int64  `json:"account_id"`
	GroupID     int64  `json:"group_id"`
	Model       string `json:"model"`
	Provider    string `json:"provider"`
	BillingMode string `json:"billing_mode"`

	RequestType *int16 `json:"request_type"`
	Stream      *bool  `json:"stream"`
	BillingType *int8  `json:"billing_type"`
}

func normalizeUsageStatsPeriod(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "week":
		return "week"
	case "month":
		return "month"
	default:
		return "today"
	}
}

func buildUsageStatsCacheKey(filters usagestats.UsageLogFilters, period, userTZ string, explicitRange bool, now time.Time) string {
	key := usageStatsCacheKey{
		UserID:      filters.UserID,
		APIKeyID:    filters.APIKeyID,
		AccountID:   filters.AccountID,
		GroupID:     filters.GroupID,
		Model:       strings.TrimSpace(filters.Model),
		Provider:    strings.TrimSpace(filters.Provider),
		BillingMode: strings.TrimSpace(filters.BillingMode),
		RequestType: filters.RequestType,
		Stream:      filters.Stream,
		BillingType: filters.BillingType,
	}
	if explicitRange {
		key.RangeMode = "explicit"
		if filters.StartTime != nil {
			key.StartTime = filters.StartTime.UTC().Format(time.RFC3339)
		}
		if filters.EndTime != nil {
			key.EndTime = filters.EndTime.UTC().Format(time.RFC3339)
		}
	} else {
		key.RangeMode = "relative"
		key.Period = normalizeUsageStatsPeriod(period)
		key.Timezone = strings.TrimSpace(userTZ)
		key.BucketStart = now.UTC().Truncate(usageStatsCacheTTL).Format(time.RFC3339)
	}
	raw, err := json.Marshal(key)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (h *UsageHandler) getUsageStatsCached(ctx context.Context, filters usagestats.UsageLogFilters, cacheKey string) (*usagestats.UsageStats, bool, error) {
	if h == nil || h.usageService == nil {
		return nil, false, fmt.Errorf("usage service not available")
	}
	if h.statsCache == nil || cacheKey == "" {
		stats, err := h.usageService.GetStatsWithFilters(ctx, filters)
		return stats, false, err
	}
	entry, hit, err := h.statsCache.GetOrLoad(cacheKey, func() (any, error) {
		return h.usageService.GetStatsWithFilters(ctx, filters)
	})
	if err != nil {
		return nil, hit, err
	}
	stats, err := snapshotPayloadAs[*usagestats.UsageStats](entry.Payload)
	return stats, hit, err
}
