package service

import "context"

type schedulerWindowCache interface {
	GetSnapshotWindow(ctx context.Context, bucket SchedulerBucket, offset, limit int) ([]*Account, bool, error)
}

type schedulableAccountWindowLoader interface {
	ListSchedulableByPlatformWindow(ctx context.Context, platform string, offset, limit int) ([]Account, error)
	ListSchedulableByGroupIDAndPlatformWindow(ctx context.Context, groupID int64, platform string, offset, limit int) ([]Account, error)
	ListSchedulableByPlatformsWindow(ctx context.Context, platforms []string, offset, limit int) ([]Account, error)
	ListSchedulableByGroupIDAndPlatformsWindow(ctx context.Context, groupID int64, platforms []string, offset, limit int) ([]Account, error)
	ListSchedulableUngroupedByPlatformWindow(ctx context.Context, platform string, offset, limit int) ([]Account, error)
	ListSchedulableUngroupedByPlatformsWindow(ctx context.Context, platforms []string, offset, limit int) ([]Account, error)
}

func sliceAccountsWindow(accounts []Account, offset, limit int) []Account {
	if len(accounts) == 0 {
		return []Account{}
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(accounts) {
		return []Account{}
	}
	if limit <= 0 {
		out := make([]Account, len(accounts)-offset)
		copy(out, accounts[offset:])
		return out
	}
	end := offset + limit
	if end > len(accounts) {
		end = len(accounts)
	}
	out := make([]Account, end-offset)
	copy(out, accounts[offset:end])
	return out
}
