package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	defaultUserCheckInHistoryLimit = 25
	checkInRecentWindowSize        = 370
)

type CheckInService struct {
	repo                 CheckInRepository
	userRepo             UserRepository
	billingCacheService  *BillingCacheService
	settingService       *SettingService
	entClient            *dbent.Client
	authCacheInvalidator APIKeyAuthCacheInvalidator
}

func NewCheckInService(
	repo CheckInRepository,
	userRepo UserRepository,
	billingCacheService *BillingCacheService,
	settingService *SettingService,
	entClient *dbent.Client,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
) *CheckInService {
	return &CheckInService{
		repo:                 repo,
		userRepo:             userRepo,
		billingCacheService:  billingCacheService,
		settingService:       settingService,
		entClient:            entClient,
		authCacheInvalidator: authCacheInvalidator,
	}
}

func (s *CheckInService) GetStatus(ctx context.Context, userID int64) (*CheckInStatus, error) {
	settings, loc, bizDate, err := s.loadContext(ctx)
	if err != nil {
		return nil, err
	}

	totalCheckIns, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count user check-ins: %w", err)
	}
	recent, err := s.repo.ListRecentByUser(ctx, userID, checkInRecentWindowSize)
	if err != nil {
		return nil, fmt.Errorf("list recent check-ins: %w", err)
	}

	streak, checkedToday, streakBroken, lastDate, lastAt := calculateCheckInStreak(recent, bizDate, loc)
	var nextAvailableAt *time.Time
	if checkedToday {
		next := nextBusinessDayStart(bizDate, loc)
		nextAvailableAt = &next
	}

	return &CheckInStatus{
		Enabled:         settings.Enabled,
		RewardType:      CheckInRewardTypeBalance,
		RewardAmount:    settings.RewardBalance,
		Timezone:        settings.Timezone,
		HistoryVisible:  settings.HistoryVisible,
		CheckedInToday:  checkedToday,
		CurrentStreak:   streak,
		TotalCheckIns:   totalCheckIns,
		StreakBroken:    streakBroken,
		CheckInDate:     bizDate,
		LastCheckInDate: lastDate,
		LastCheckInAt:   lastAt,
		NextAvailableAt: nextAvailableAt,
	}, nil
}

func (s *CheckInService) CheckIn(ctx context.Context, userID int64) (*CheckInResult, error) {
	settings, loc, bizDate, err := s.loadContext(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, ErrCheckInDisabled
	}
	if settings.RewardBalance <= 0 {
		return nil, ErrCheckInConfigInvalid
	}

	now := time.Now().In(loc)
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin check-in tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	record, inserted, err := s.repo.TryCreate(txCtx, userID, bizDate, settings.RewardBalance, now)
	if err != nil {
		return nil, fmt.Errorf("create check-in record: %w", err)
	}
	if inserted {
		if err := s.userRepo.UpdateBalance(txCtx, userID, settings.RewardBalance); err != nil {
			return nil, fmt.Errorf("apply check-in reward: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit check-in tx: %w", err)
		}
		s.invalidateCheckInCaches(userID)
		return s.buildResult(ctx, userID, bizDate, record, true, false, loc)
	}

	existing, found, err := s.repo.GetByUserAndDate(ctx, userID, bizDate)
	if err != nil {
		return nil, fmt.Errorf("load existing check-in: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("check-in claim conflicted but no record found")
	}
	return s.buildResult(ctx, userID, bizDate, existing, false, true, loc)
}

func (s *CheckInService) GetHistory(ctx context.Context, userID int64, limit int) ([]UserCheckIn, error) {
	settings, _, _, err := s.loadContext(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.HistoryVisible {
		return nil, ErrCheckInHistoryDisabled
	}
	if limit <= 0 {
		limit = defaultUserCheckInHistoryLimit
	}
	return s.repo.ListRecentByUser(ctx, userID, limit)
}

func (s *CheckInService) buildResult(ctx context.Context, userID int64, bizDate string, record *UserCheckIn, checkedIn, alreadyCheckedIn bool, loc *time.Location) (*CheckInResult, error) {
	recent, err := s.repo.ListRecentByUser(ctx, userID, checkInRecentWindowSize)
	if err != nil {
		return nil, fmt.Errorf("list recent check-ins: %w", err)
	}
	totalCheckIns, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count user check-ins: %w", err)
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user after check-in: %w", err)
	}

	streak, _, streakBroken, _, _ := calculateCheckInStreak(recent, bizDate, loc)
	return &CheckInResult{
		CheckedIn:        checkedIn,
		AlreadyCheckedIn: alreadyCheckedIn,
		CheckInDate:      bizDate,
		CheckedInAt:      record.CreatedAt,
		CurrentStreak:    streak,
		TotalCheckIns:    totalCheckIns,
		StreakBroken:     streakBroken,
		Reward: CheckInReward{
			Type:       CheckInRewardTypeBalance,
			Amount:     record.RewardAmount,
			NewBalance: user.Balance,
		},
	}, nil
}

func (s *CheckInService) invalidateCheckInCaches(userID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			logger.LegacyPrintf("service.checkin", "invalidate user balance cache failed: user_id=%d err=%v", userID, err)
		}
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
}

func (s *CheckInService) loadContext(ctx context.Context) (*CheckInSettings, *time.Location, string, error) {
	settings, err := s.settingService.GetCheckInSettings(ctx)
	if err != nil {
		return nil, nil, "", fmt.Errorf("get check-in settings: %w", err)
	}
	loc, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return nil, nil, "", ErrCheckInConfigInvalid.WithCause(err)
	}
	now := time.Now().In(loc)
	return settings, loc, now.Format("2006-01-02"), nil
}

func calculateCheckInStreak(recent []UserCheckIn, today string, loc *time.Location) (int, bool, bool, *string, *time.Time) {
	if len(recent) == 0 {
		return 0, false, false, nil, nil
	}

	lastDate := recent[0].CheckInDate
	lastAt := recent[0].CreatedAt
	todayDate, err := parseBizDate(today, loc)
	if err != nil {
		return 0, false, false, &lastDate, &lastAt
	}
	latestDate, err := parseBizDate(lastDate, loc)
	if err != nil {
		return 0, false, false, &lastDate, &lastAt
	}

	checkedToday := lastDate == today
	expected := latestDate
	switch {
	case checkedToday:
	case latestDate.Equal(todayDate.AddDate(0, 0, -1)):
	default:
		return 0, false, true, &lastDate, &lastAt
	}

	streak := 0
	for _, item := range recent {
		d, parseErr := parseBizDate(item.CheckInDate, loc)
		if parseErr != nil {
			continue
		}
		if d.Equal(expected) {
			streak++
			expected = expected.AddDate(0, 0, -1)
			continue
		}
		if d.Before(expected) {
			break
		}
	}

	return streak, checkedToday, false, &lastDate, &lastAt
}

func parseBizDate(value string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, loc)
}

func nextBusinessDayStart(today string, loc *time.Location) time.Time {
	current, err := parseBizDate(today, loc)
	if err != nil {
		return time.Now().In(loc)
	}
	return current.AddDate(0, 0, 1)
}
