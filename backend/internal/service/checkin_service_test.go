//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type checkInSettingRepoTestStub struct {
	values map[string]string
}

func (s *checkInSettingRepoTestStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *checkInSettingRepoTestStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *checkInSettingRepoTestStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *checkInSettingRepoTestStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *checkInSettingRepoTestStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *checkInSettingRepoTestStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *checkInSettingRepoTestStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

type checkInRepoTestStub struct {
	tryCreateFn        func(ctx context.Context, userID int64, checkInDate string, rewardAmount float64, createdAt time.Time) (*UserCheckIn, bool, error)
	getByUserAndDateFn func(ctx context.Context, userID int64, checkInDate string) (*UserCheckIn, bool, error)
	getLatestByUserFn  func(ctx context.Context, userID int64) (*UserCheckIn, bool, error)
	listRecentByUserFn func(ctx context.Context, userID int64, limit int) ([]UserCheckIn, error)
	listByUserPageFn   func(ctx context.Context, userID int64, params pagination.PaginationParams) ([]UserCheckIn, *pagination.PaginationResult, error)
	countByUserFn      func(ctx context.Context, userID int64) (int64, error)
	sumRewardByUserFn  func(ctx context.Context, userID int64) (float64, error)

	lastTryCreateDate string
	lastGetByDate     string
}

func (s *checkInRepoTestStub) TryCreate(ctx context.Context, userID int64, checkInDate string, rewardAmount float64, createdAt time.Time) (*UserCheckIn, bool, error) {
	s.lastTryCreateDate = checkInDate
	if s.tryCreateFn == nil {
		panic("unexpected TryCreate call")
	}
	return s.tryCreateFn(ctx, userID, checkInDate, rewardAmount, createdAt)
}

func (s *checkInRepoTestStub) GetByUserAndDate(ctx context.Context, userID int64, checkInDate string) (*UserCheckIn, bool, error) {
	s.lastGetByDate = checkInDate
	if s.getByUserAndDateFn == nil {
		panic("unexpected GetByUserAndDate call")
	}
	return s.getByUserAndDateFn(ctx, userID, checkInDate)
}

func (s *checkInRepoTestStub) GetLatestByUser(ctx context.Context, userID int64) (*UserCheckIn, bool, error) {
	if s.getLatestByUserFn == nil {
		return nil, false, nil
	}
	return s.getLatestByUserFn(ctx, userID)
}

func (s *checkInRepoTestStub) ListRecentByUser(ctx context.Context, userID int64, limit int) ([]UserCheckIn, error) {
	if s.listRecentByUserFn == nil {
		panic("unexpected ListRecentByUser call")
	}
	return s.listRecentByUserFn(ctx, userID, limit)
}

func (s *checkInRepoTestStub) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams) ([]UserCheckIn, *pagination.PaginationResult, error) {
	if s.listByUserPageFn == nil {
		return nil, &pagination.PaginationResult{Total: 0, Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
	}
	return s.listByUserPageFn(ctx, userID, params)
}

func (s *checkInRepoTestStub) CountByUser(ctx context.Context, userID int64) (int64, error) {
	if s.countByUserFn == nil {
		panic("unexpected CountByUser call")
	}
	return s.countByUserFn(ctx, userID)
}

func (s *checkInRepoTestStub) SumRewardByUser(ctx context.Context, userID int64) (float64, error) {
	if s.sumRewardByUserFn == nil {
		return 0, nil
	}
	return s.sumRewardByUserFn(ctx, userID)
}

type checkInUserRepoTestStub struct {
	user               *User
	updateBalanceCalls int
}

func (s *checkInUserRepoTestStub) Create(ctx context.Context, user *User) error {
	panic("unexpected Create call")
}

func (s *checkInUserRepoTestStub) GetByID(ctx context.Context, id int64) (*User, error) {
	if s.user == nil || s.user.ID != id {
		return nil, ErrUserNotFound
	}
	copyUser := *s.user
	return &copyUser, nil
}

func (s *checkInUserRepoTestStub) GetByEmail(ctx context.Context, email string) (*User, error) {
	panic("unexpected GetByEmail call")
}

func (s *checkInUserRepoTestStub) GetFirstAdmin(ctx context.Context) (*User, error) {
	panic("unexpected GetFirstAdmin call")
}

func (s *checkInUserRepoTestStub) Update(ctx context.Context, user *User) error {
	panic("unexpected Update call")
}

func (s *checkInUserRepoTestStub) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (s *checkInUserRepoTestStub) List(ctx context.Context, params pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *checkInUserRepoTestStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *checkInUserRepoTestStub) UpdateBalance(ctx context.Context, id int64, amount float64) error {
	if s.user == nil || s.user.ID != id {
		return ErrUserNotFound
	}
	s.updateBalanceCalls++
	s.user.Balance += amount
	return nil
}

func (s *checkInUserRepoTestStub) DeductBalance(ctx context.Context, id int64, amount float64) error {
	panic("unexpected DeductBalance call")
}

func (s *checkInUserRepoTestStub) UpdateConcurrency(ctx context.Context, id int64, amount int) error {
	panic("unexpected UpdateConcurrency call")
}

func (s *checkInUserRepoTestStub) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	panic("unexpected ExistsByEmail call")
}

func (s *checkInUserRepoTestStub) RemoveGroupFromAllowedGroups(ctx context.Context, groupID int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}

func (s *checkInUserRepoTestStub) AddGroupToAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}

func (s *checkInUserRepoTestStub) RemoveGroupFromUserAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	panic("unexpected RemoveGroupFromUserAllowedGroups call")
}

func (s *checkInUserRepoTestStub) UpdateTotpSecret(ctx context.Context, userID int64, encryptedSecret *string) error {
	panic("unexpected UpdateTotpSecret call")
}

func (s *checkInUserRepoTestStub) EnableTotp(ctx context.Context, userID int64) error {
	panic("unexpected EnableTotp call")
}

func (s *checkInUserRepoTestStub) DisableTotp(ctx context.Context, userID int64) error {
	panic("unexpected DisableTotp call")
}

func newCheckInServiceEntClient(t *testing.T) (*dbent.Client, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))

	cleanup := func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = client.Close()
		_ = db.Close()
	}
	return client, mock, cleanup
}

func TestCheckInService_CheckIn_DuplicateSameDay_ReturnsAlreadyCheckedInSuccess(t *testing.T) {
	const timezoneName = "Pacific/Kiritimati"

	client, mock, cleanup := newCheckInServiceEntClient(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectRollback()

	settingService := NewSettingService(&checkInSettingRepoTestStub{
		values: map[string]string{
			SettingKeyCheckInEnabled:        "true",
			SettingKeyCheckInRewardBalance:  "1.25",
			SettingKeyCheckInTimezone:       timezoneName,
			SettingKeyCheckInHistoryVisible: "true",
		},
	}, &config.Config{Timezone: "UTC"})

	loc, err := time.LoadLocation(timezoneName)
	require.NoError(t, err)
	expectedBizDate := time.Now().In(loc).Format("2006-01-02")
	existing := &UserCheckIn{
		ID:           101,
		UserID:       1,
		CheckInDate:  expectedBizDate,
		RewardAmount: 1.25,
		CreatedAt:    time.Now().In(loc),
	}

	repo := &checkInRepoTestStub{
		tryCreateFn: func(ctx context.Context, userID int64, checkInDate string, rewardAmount float64, createdAt time.Time) (*UserCheckIn, bool, error) {
			return nil, false, nil
		},
		getByUserAndDateFn: func(ctx context.Context, userID int64, checkInDate string) (*UserCheckIn, bool, error) {
			return existing, true, nil
		},
		listRecentByUserFn: func(ctx context.Context, userID int64, limit int) ([]UserCheckIn, error) {
			return []UserCheckIn{*existing}, nil
		},
		countByUserFn: func(ctx context.Context, userID int64) (int64, error) {
			return 20, nil
		},
	}
	userRepo := &checkInUserRepoTestStub{
		user: &User{ID: 1, Balance: 99.5},
	}
	svc := NewCheckInService(repo, userRepo, nil, settingService, client, nil)

	result, err := svc.CheckIn(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.CheckedIn)
	require.True(t, result.AlreadyCheckedIn)
	require.Equal(t, expectedBizDate, result.CheckInDate)
	require.Equal(t, expectedBizDate, repo.lastTryCreateDate)
	require.Equal(t, expectedBizDate, repo.lastGetByDate)
	require.Equal(t, CheckInRewardTypeBalance, result.Reward.Type)
	require.InDelta(t, 1.25, result.Reward.Amount, 1e-9)
	require.InDelta(t, 99.5, result.Reward.NewBalance, 1e-9)
	require.Equal(t, 0, userRepo.updateBalanceCalls, "duplicate check-in should not re-apply reward")
}

func TestCheckInService_GetStatus_UsesConfiguredBusinessTimezone(t *testing.T) {
	const timezoneName = "Pacific/Kiritimati"

	client, _, cleanup := newCheckInServiceEntClient(t)
	defer cleanup()

	settingService := NewSettingService(&checkInSettingRepoTestStub{
		values: map[string]string{
			SettingKeyCheckInEnabled:        "true",
			SettingKeyCheckInRewardBalance:  "1.25",
			SettingKeyCheckInTimezone:       timezoneName,
			SettingKeyCheckInHistoryVisible: "true",
		},
	}, &config.Config{Timezone: "UTC"})

	loc, err := time.LoadLocation(timezoneName)
	require.NoError(t, err)
	today := time.Now().In(loc).Format("2006-01-02")
	recentItem := UserCheckIn{
		ID:           10,
		UserID:       1,
		CheckInDate:  today,
		RewardAmount: 1.25,
		CreatedAt:    time.Now().In(loc),
	}

	repo := &checkInRepoTestStub{
		countByUserFn: func(ctx context.Context, userID int64) (int64, error) {
			return 3, nil
		},
		listRecentByUserFn: func(ctx context.Context, userID int64, limit int) ([]UserCheckIn, error) {
			return []UserCheckIn{recentItem}, nil
		},
	}
	userRepo := &checkInUserRepoTestStub{
		user: &User{ID: 1, Balance: 50},
	}
	svc := NewCheckInService(repo, userRepo, nil, settingService, client, nil)

	status, err := svc.GetStatus(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.True(t, status.Enabled)
	require.Equal(t, CheckInRewardTypeBalance, status.RewardType)
	require.InDelta(t, 1.25, status.RewardAmount, 1e-9)
	require.Equal(t, timezoneName, status.Timezone)
	require.True(t, status.HistoryVisible)
	require.True(t, status.CheckedInToday)
	require.Equal(t, today, status.CheckInDate)
	require.EqualValues(t, 3, status.TotalCheckIns)
	require.NotNil(t, status.NextAvailableAt)

	todayStart, parseErr := time.ParseInLocation("2006-01-02", today, loc)
	require.NoError(t, parseErr)
	expectedNext := todayStart.AddDate(0, 0, 1)
	require.True(t, status.NextAvailableAt.Equal(expectedNext), "next available time should be next business day start")
}
