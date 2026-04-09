package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const CheckInRewardTypeBalance = "balance"

var (
	ErrCheckInDisabled        = infraerrors.Forbidden("CHECK_IN_DISABLED", "check-in is currently disabled")
	ErrCheckInHistoryDisabled = infraerrors.Forbidden("CHECK_IN_HISTORY_DISABLED", "check-in history is currently hidden")
	ErrCheckInConfigInvalid   = infraerrors.BadRequest("CHECK_IN_CONFIG_INVALID", "check-in settings are invalid")
)

type UserCheckIn struct {
	ID           int64
	UserID       int64
	CheckInDate  string
	RewardAmount float64
	CreatedAt    time.Time
}

type CheckInReward struct {
	Type       string
	Amount     float64
	NewBalance float64
}

type CheckInStatus struct {
	Enabled         bool
	RewardType      string
	RewardAmount    float64
	Timezone        string
	HistoryVisible  bool
	CheckedInToday  bool
	CurrentStreak   int
	TotalCheckIns   int64
	StreakBroken    bool
	CheckInDate     string
	LastCheckInDate *string
	LastCheckInAt   *time.Time
	NextAvailableAt *time.Time
}

type CheckInResult struct {
	CheckedIn        bool
	AlreadyCheckedIn bool
	CheckInDate      string
	CheckedInAt      time.Time
	CurrentStreak    int
	TotalCheckIns    int64
	StreakBroken     bool
	Reward           CheckInReward
}

type CheckInRepository interface {
	TryCreate(ctx context.Context, userID int64, checkInDate string, rewardAmount float64, createdAt time.Time) (*UserCheckIn, bool, error)
	GetByUserAndDate(ctx context.Context, userID int64, checkInDate string) (*UserCheckIn, bool, error)
	GetLatestByUser(ctx context.Context, userID int64) (*UserCheckIn, bool, error)
	ListRecentByUser(ctx context.Context, userID int64, limit int) ([]UserCheckIn, error)
	ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams) ([]UserCheckIn, *pagination.PaginationResult, error)
	CountByUser(ctx context.Context, userID int64) (int64, error)
	SumRewardByUser(ctx context.Context, userID int64) (float64, error)
}
