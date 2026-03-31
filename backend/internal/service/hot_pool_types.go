package service

import (
	"context"
	"time"
)

const hotPoolDeadAccountTTL = 24 * time.Hour

// HotPoolPlatformMeta stores runtime metadata for a platform hot pool.
type HotPoolPlatformMeta struct {
	TargetSize    int        `json:"target_size"`
	CurrentSize   int        `json:"current_size"`
	LastRefillAt  *time.Time `json:"last_refill_at,omitempty"`
	LastRebuildAt *time.Time `json:"last_rebuild_at,omitempty"`
	Version       int64      `json:"version"`
}

// HotPoolPlatformStatus is the read model exposed to callers for one platform.
type HotPoolPlatformStatus struct {
	Platform       string     `json:"platform"`
	TargetSize     int        `json:"target_size"`
	CurrentSize    int        `json:"current_size"`
	ColdCandidates int        `json:"cold_candidates,omitempty"`
	LastRefillAt   *time.Time `json:"last_refill_at,omitempty"`
	LastRebuildAt  *time.Time `json:"last_rebuild_at,omitempty"`
	Version        int64      `json:"version"`
}

// HotPoolCache defines the Redis-backed runtime store for hot pools.
type HotPoolCache interface {
	ListMembers(ctx context.Context, platform string) ([]int64, error)
	AddMembers(ctx context.Context, platform string, ids []int64) error
	ReplaceMembers(ctx context.Context, platform string, ids []int64) error
	RemoveMember(ctx context.Context, platform string, accountID int64) error
	CountMembers(ctx context.Context, platform string) (int, error)
	GetMeta(ctx context.Context, platform string) (*HotPoolPlatformMeta, error)
	SetMeta(ctx context.Context, platform string, meta *HotPoolPlatformMeta) error
	MarkDeadAccount(ctx context.Context, accountID int64, ttl time.Duration) error
	IsDeadAccount(ctx context.Context, accountID int64) (bool, error)
	TryAcquireRefillLock(ctx context.Context, platform string, ttl time.Duration) (bool, error)
}
