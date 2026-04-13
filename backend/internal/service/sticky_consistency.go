package service

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
)

// StickyConsistencyMetricsSnapshot captures sticky session probe counters for drift analysis.
type StickyConsistencyMetricsSnapshot struct {
	TotalHits           int64                 `json:"total_hits"`
	GhostInvalidations  int64                 `json:"ghost_invalidations"`
	GhostRatio          float64               `json:"ghost_ratio"`
	GhostDelta          int64                 `json:"ghost_delta,omitempty"`
	GhostRatioDelta     float64               `json:"ghost_ratio_delta,omitempty"`
	GhostReasons        map[string]int64      `json:"ghost_reasons,omitempty"`
	PrimaryReasons      []StickyReasonSummary `json:"primary_reasons,omitempty"`
	PrimaryReasonTrends []StickyReasonTrend   `json:"primary_reason_trends,omitempty"`
	ActivityLevel       string                `json:"activity_level,omitempty"`
	ActivityTrend       string                `json:"activity_trend,omitempty"`
}

// StickyReasonSummary highlights the most impactful ghost invalidation causes.
type StickyReasonSummary struct {
	Reason     string  `json:"reason"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
	Severity   string  `json:"severity"`
}

// StickyReasonTrend highlights recent delta and direction for top reasons.
type StickyReasonTrend struct {
	Reason     string  `json:"reason"`
	Current    int64   `json:"current"`
	Delta      int64   `json:"delta"`
	DeltaRatio float64 `json:"delta_ratio"`
	Direction  string  `json:"direction"`
	Severity   string  `json:"severity"`
}

const (
	stickyActivityLevelLow    = "low"
	stickyActivityLevelMedium = "medium"
	stickyActivityLevelHigh   = "high"
)

var stickyReasonSeverity = map[string]string{
	openAIStickySessionShadowReasonAccountMissing:       "critical",
	openAIStickySessionShadowReasonClearedUnschedulable: "high",
	openAIStickySessionShadowReasonModelMismatch:        "medium",
	openAIStickySessionShadowReasonPlatformMismatch:     "medium",
	openAIStickySessionShadowReasonGroupMismatch:        "medium",
	openAIStickySessionShadowReasonTransportMismatch:    "medium",
	openAIStickySessionShadowReasonChannelRestricted:    "medium",
	openAIStickySessionShadowReasonPrivacyRequired:      "medium",
	openAIStickySessionShadowReasonDBRuntimeRecheck:     "high",
	openAIStickySessionShadowReasonWait:                 "low",
	openAIStickySessionShadowReasonAcquireError:         "high",
	openAIStickySessionShadowReasonExcluded:             "low",
}

var stickyConsistencyMetrics = &stickyConsistencyTracker{
	reasonTotals: make(map[string]int64),
}

type stickyConsistencyTracker struct {
	totalHits        atomic.Int64
	ghostTotal       atomic.Int64
	mu               sync.Mutex
	reasonTotals     map[string]int64
	lastSnapshot     StickyConsistencyMetricsSnapshot
	lastSnapshotSeen bool
	lastTotalHits    int64
	lastGhostTotal   int64
	lastGhostRatio   float64
	lastReasonTotals map[string]int64
}

func (t *stickyConsistencyTracker) recordHit() {
	t.totalHits.Add(1)
}

func (t *stickyConsistencyTracker) recordGhost(reason string) {
	t.ghostTotal.Add(1)
	if reason == "" {
		reason = "unknown"
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reasonTotals[reason]++
}

func SnapshotStickyConsistencyMetrics() StickyConsistencyMetricsSnapshot {
	total := stickyConsistencyMetrics.totalHits.Load()
	ghost := stickyConsistencyMetrics.ghostTotal.Load()
	ratio := 0.0
	if total > 0 {
		ratio = float64(ghost) / float64(total)
	}
	ghostDelta := int64(0)
	ghostRatioDelta := 0.0
	snapshot := StickyConsistencyMetricsSnapshot{
		TotalHits:          total,
		GhostInvalidations: ghost,
		GhostRatio:         roundFloat(ratio, 4),
	}
	stickyConsistencyMetrics.mu.Lock()
	currentTotals := cloneStickyReasonTotals(stickyConsistencyMetrics.reasonTotals)
	if len(currentTotals) > 0 {
		snapshot.GhostReasons = cloneStickyReasonTotals(currentTotals)
	}
	if stickyConsistencyMetrics.lastSnapshotSeen &&
		total == stickyConsistencyMetrics.lastTotalHits &&
		ghost == stickyConsistencyMetrics.lastGhostTotal &&
		stickyReasonTotalsEqual(currentTotals, stickyConsistencyMetrics.lastReasonTotals) {
		cached := cloneStickyConsistencySnapshot(stickyConsistencyMetrics.lastSnapshot)
		stickyConsistencyMetrics.mu.Unlock()
		return cached
	}

	var deltaReasons map[string]int64
	if stickyConsistencyMetrics.lastReasonTotals != nil {
		deltaReasons = make(map[string]int64, len(currentTotals))
		for reason, count := range currentTotals {
			deltaReasons[reason] = count - stickyConsistencyMetrics.lastReasonTotals[reason]
		}
		for reason, prevCount := range stickyConsistencyMetrics.lastReasonTotals {
			if _, ok := currentTotals[reason]; !ok {
				deltaReasons[reason] = -prevCount
			}
		}
	}
	ghostDelta = ghost - stickyConsistencyMetrics.lastGhostTotal
	ghostRatioDelta = roundFloat(ratio-stickyConsistencyMetrics.lastGhostRatio, 4)

	if len(currentTotals) > 0 {
		snapshot.PrimaryReasons = buildStickyPrimaryReasons(total, snapshot.GhostReasons)
		snapshot.PrimaryReasonTrends = buildStickyPrimaryReasonTrends(total, snapshot.GhostReasons, deltaReasons)
	} else {
		snapshot.PrimaryReasons = nil
		snapshot.PrimaryReasonTrends = nil
	}
	snapshot.ActivityLevel = deriveStickyActivityLevel(total, ghost)
	snapshot.ActivityTrend = deriveStickyActivityTrend(ghostDelta, ghostRatioDelta)
	snapshot.GhostDelta = ghostDelta
	snapshot.GhostRatioDelta = ghostRatioDelta
	stickyConsistencyMetrics.lastTotalHits = total
	stickyConsistencyMetrics.lastGhostTotal = ghost
	stickyConsistencyMetrics.lastGhostRatio = ratio
	stickyConsistencyMetrics.lastReasonTotals = cloneStickyReasonTotals(currentTotals)
	stickyConsistencyMetrics.lastSnapshot = cloneStickyConsistencySnapshot(snapshot)
	stickyConsistencyMetrics.lastSnapshotSeen = true
	stickyConsistencyMetrics.mu.Unlock()
	return cloneStickyConsistencySnapshot(snapshot)
}

func resetStickyConsistencyTracker() {
	stickyConsistencyMetrics.totalHits.Store(0)
	stickyConsistencyMetrics.ghostTotal.Store(0)
	stickyConsistencyMetrics.mu.Lock()
	defer stickyConsistencyMetrics.mu.Unlock()
	for k := range stickyConsistencyMetrics.reasonTotals {
		delete(stickyConsistencyMetrics.reasonTotals, k)
	}
	stickyConsistencyMetrics.lastSnapshot = StickyConsistencyMetricsSnapshot{}
	stickyConsistencyMetrics.lastSnapshotSeen = false
	stickyConsistencyMetrics.lastTotalHits = 0
	stickyConsistencyMetrics.lastGhostTotal = 0
	stickyConsistencyMetrics.lastGhostRatio = 0
	stickyConsistencyMetrics.lastReasonTotals = nil
}

func TrackStickyConsistencyHit() {
	stickyConsistencyMetrics.recordHit()
}

func TrackStickyConsistencyGhost(reason string) {
	stickyConsistencyMetrics.recordGhost(reason)
}

func buildStickyPrimaryReasons(total int64, reasons map[string]int64) []StickyReasonSummary {
	if total <= 0 || len(reasons) == 0 {
		return nil
	}
	list := make([]StickyReasonSummary, 0, len(reasons))
	for reason, count := range reasons {
		percentage := float64(count) / float64(total)
		severity := stickyReasonSeverity[reason]
		if severity == "" {
			severity = "low"
		}
		list = append(list, StickyReasonSummary{
			Reason:     reason,
			Count:      count,
			Percentage: roundFloat(percentage, 4),
			Severity:   severity,
		})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].Reason < list[j].Reason
	})
	if len(list) > 3 {
		list = list[:3]
	}
	return list
}

func deriveStickyActivityLevel(total, ghost int64) string {
	if total <= 0 {
		return ""
	}
	ratio := float64(ghost) / float64(total)
	switch {
	case ghost >= 50 || ratio >= 0.2:
		return stickyActivityLevelHigh
	case ghost >= 10 || ratio >= 0.05:
		return stickyActivityLevelMedium
	default:
		return stickyActivityLevelLow
	}
}

func deriveStickyActivityTrend(delta int64, ratioDelta float64) string {
	switch {
	case delta > 0 || ratioDelta > 0:
		return "increasing"
	case delta < 0 || ratioDelta < 0:
		return "decreasing"
	default:
		return "steady"
	}
}

func buildStickyPrimaryReasonTrends(total int64, current map[string]int64, delta map[string]int64) []StickyReasonTrend {
	if total <= 0 || len(current) == 0 || delta == nil {
		return nil
	}
	list := make([]StickyReasonTrend, 0, len(current))
	for reason, count := range current {
		deltaCount := delta[reason]
		ratioDelta := 0.0
		if total > 0 {
			ratioDelta = float64(deltaCount) / float64(total)
		}
		trend := StickyReasonTrend{
			Reason:     reason,
			Current:    count,
			Delta:      deltaCount,
			DeltaRatio: roundFloat(ratioDelta, 4),
			Direction:  deriveStickyDirection(deltaCount),
			Severity:   severityFromReason(reason),
		}
		list = append(list, trend)
	}
	sort.Slice(list, func(i, j int) bool {
		iDelta := absInt64(list[i].Delta)
		jDelta := absInt64(list[j].Delta)
		if iDelta != jDelta {
			return iDelta > jDelta
		}
		if list[i].Current != list[j].Current {
			return list[i].Current > list[j].Current
		}
		return list[i].Reason < list[j].Reason
	})
	if len(list) > 3 {
		list = list[:3]
	}
	return list
}

func deriveStickyDirection(delta int64) string {
	switch {
	case delta > 0:
		return "rising"
	case delta < 0:
		return "falling"
	default:
		return "steady"
	}
}

func severityFromReason(reason string) string {
	if severity := stickyReasonSeverity[reason]; severity != "" {
		return severity
	}
	return "low"
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func roundFloat(value float64, precision int) float64 {
	if precision < 0 {
		return value
	}
	factor := math.Pow10(precision)
	return math.Round(value*factor) / factor
}

func cloneStickyReasonTotals(src map[string]int64) map[string]int64 {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]int64, len(src))
	for reason, count := range src {
		dst[reason] = count
	}
	return dst
}

func stickyReasonTotalsEqual(left, right map[string]int64) bool {
	if len(left) != len(right) {
		return false
	}
	for reason, count := range left {
		if right[reason] != count {
			return false
		}
	}
	return true
}

func cloneStickyConsistencySnapshot(src StickyConsistencyMetricsSnapshot) StickyConsistencyMetricsSnapshot {
	src.GhostReasons = cloneStickyReasonTotals(src.GhostReasons)
	if len(src.PrimaryReasons) > 0 {
		src.PrimaryReasons = append([]StickyReasonSummary(nil), src.PrimaryReasons...)
	}
	if len(src.PrimaryReasonTrends) > 0 {
		src.PrimaryReasonTrends = append([]StickyReasonTrend(nil), src.PrimaryReasonTrends...)
	}
	return src
}
