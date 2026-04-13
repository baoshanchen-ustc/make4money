package service

import (
	"sync"
	"sync/atomic"
)

// ShadowPermanentFailureStats summarizes shadow-marked OAuth account failures.
type ShadowPermanentFailureStats struct {
	Total    int64            `json:"total"`
	ByReason map[string]int64 `json:"by_reason"`
}

var (
	shadowPermanentFailureTotal    atomic.Int64
	shadowPermanentFailureMu       sync.Mutex
	shadowPermanentFailureByReason = make(map[string]int64)
)

func recordShadowPermanentFailure(accountID int64, reason string) {
	if reason == "" {
		reason = "unknown"
	}
	shadowPermanentFailureTotal.Add(1)
	shadowPermanentFailureMu.Lock()
	shadowPermanentFailureByReason[reason]++
	shadowPermanentFailureMu.Unlock()
}

func SnapshotShadowPermanentFailureStats() ShadowPermanentFailureStats {
	shadowPermanentFailureMu.Lock()
	defer shadowPermanentFailureMu.Unlock()
	result := ShadowPermanentFailureStats{
		Total:    shadowPermanentFailureTotal.Load(),
		ByReason: make(map[string]int64, len(shadowPermanentFailureByReason)),
	}
	for reason, count := range shadowPermanentFailureByReason {
		result.ByReason[reason] = count
	}
	return result
}
