//go:build unit

package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func TestAdminService_CreateUser_Success(t *testing.T) {
	repo := &userRepoStub{nextID: 10}
	svc := &adminServiceImpl{userRepo: repo}

	input := &CreateUserInput{
		Email:         "user@test.com",
		Password:      "strong-pass",
		Username:      "tester",
		Notes:         "note",
		Balance:       12.5,
		Concurrency:   7,
		AllowedGroups: []int64{3, 5},
	}

	user, err := svc.CreateUser(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, int64(10), user.ID)
	require.Equal(t, input.Email, user.Email)
	require.Equal(t, input.Username, user.Username)
	require.Equal(t, input.Notes, user.Notes)
	require.Equal(t, input.Balance, user.Balance)
	require.Equal(t, input.Concurrency, user.Concurrency)
	require.Equal(t, input.AllowedGroups, user.AllowedGroups)
	require.Equal(t, RoleUser, user.Role)
	require.Equal(t, StatusActive, user.Status)
	require.True(t, user.CheckPassword(input.Password))
	require.Len(t, repo.created, 1)
	require.Equal(t, user, repo.created[0])
}

func TestAdminService_CreateUser_ChannelAdminPersistsAllowedGroups(t *testing.T) {
	client, db := newAdminServiceUserAllowedGroupsTestClient(t, "create_channel_admin_allowed_groups")
	repo := repository.NewUserRepository(client, db)
	svc := &adminServiceImpl{userRepo: repo, entClient: client}
	ctx := context.Background()
	groupThree := createAdminServiceUserAllowedGroupsTestGroup(t, client, ctx, "group-3")
	groupOne := createAdminServiceUserAllowedGroupsTestGroup(t, client, ctx, "group-1")
	groupTwo := createAdminServiceUserAllowedGroupsTestGroup(t, client, ctx, "group-2")

	user, err := svc.CreateUser(ctx, &CreateUserInput{
		Email:         "channel-admin-create@test.com",
		Password:      "strong-pass",
		Role:          RoleChannelAdmin,
		AllowedGroups: []int64{groupThree.ID, groupOne.ID, groupThree.ID, -2, groupTwo.ID},
	})
	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, RoleChannelAdmin, user.Role)
	require.Equal(t, []int64{groupOne.ID, groupTwo.ID, groupThree.ID}, user.AllowedGroups)

	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{groupOne.ID, groupTwo.ID, groupThree.ID}, got.AllowedGroups)
}

func TestAdminService_CreateUser_NonChannelAdminPersistsAllowedGroups(t *testing.T) {
	client, db := newAdminServiceUserAllowedGroupsTestClient(t, "create_admin_allowed_groups")
	repo := repository.NewUserRepository(client, db)
	svc := &adminServiceImpl{userRepo: repo, entClient: client}
	ctx := context.Background()
	groupOne := createAdminServiceUserAllowedGroupsTestGroup(t, client, ctx, "admin-group-1")
	groupTwo := createAdminServiceUserAllowedGroupsTestGroup(t, client, ctx, "admin-group-2")

	user, err := svc.CreateUser(ctx, &CreateUserInput{
		Email:         "admin-create@test.com",
		Password:      "strong-pass",
		Role:          RoleAdmin,
		AllowedGroups: []int64{groupTwo.ID, groupOne.ID, groupTwo.ID},
	})
	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, RoleAdmin, user.Role)
	require.Equal(t, []int64{groupOne.ID, groupTwo.ID}, user.AllowedGroups)

	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{groupOne.ID, groupTwo.ID}, got.AllowedGroups)
}

func TestAdminService_CreateUser_InvalidRole(t *testing.T) {
	repo := &userRepoStub{nextID: 10}
	svc := &adminServiceImpl{userRepo: repo}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "user@test.com",
		Password: "strong-pass",
		Role:     "super-admin",
	})
	require.EqualError(t, err, "invalid role: super-admin")
	require.Empty(t, repo.created)
}

func TestAdminService_UpdateUser_ChannelAdminRolePersistsAllowedGroups(t *testing.T) {
	client, db := newAdminServiceUserAllowedGroupsTestClient(t, "update_channel_admin_allowed_groups")
	repo := repository.NewUserRepository(client, db)
	ctx := context.Background()
	seedUser := createAdminServiceUserAllowedGroupsTestUser(t, client, ctx, "update-channel-admin@test.com", RoleUser)
	svc := &adminServiceImpl{userRepo: repo, entClient: client, redeemCodeRepo: &redeemRepoStub{}}
	groupFour := createAdminServiceUserAllowedGroupsTestGroup(t, client, ctx, "group-4")
	groupTwo := createAdminServiceUserAllowedGroupsTestGroup(t, client, ctx, "group-2")
	groupOne := createAdminServiceUserAllowedGroupsTestGroup(t, client, ctx, "group-1")

	role := RoleChannelAdmin
	groupIDs := []int64{groupFour.ID, groupTwo.ID, groupFour.ID, groupOne.ID}
	updated, err := svc.UpdateUser(ctx, seedUser.ID, &UpdateUserInput{
		Role:          &role,
		AllowedGroups: &groupIDs,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, RoleChannelAdmin, updated.Role)
	require.Equal(t, []int64{groupOne.ID, groupTwo.ID, groupFour.ID}, updated.AllowedGroups)

	got, err := svc.GetUser(ctx, seedUser.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{groupOne.ID, groupTwo.ID, groupFour.ID}, got.AllowedGroups)
}

func TestAdminService_UpdateUser_RoleChangeAwayFromChannelAdminKeepsAllowedGroupsWithoutInput(t *testing.T) {
	client, db := newAdminServiceUserAllowedGroupsTestClient(t, "update_keep_allowed_groups_on_role_change")
	repo := repository.NewUserRepository(client, db)
	ctx := context.Background()
	groupTwo := createAdminServiceUserAllowedGroupsTestGroup(t, client, ctx, "group-2")
	groupFive := createAdminServiceUserAllowedGroupsTestGroup(t, client, ctx, "group-5")
	seedUser := createAdminServiceUserAllowedGroupsTestUser(t, client, ctx, "clear-channel-admin@test.com", RoleChannelAdmin, groupTwo.ID, groupFive.ID)
	svc := &adminServiceImpl{userRepo: repo, entClient: client, redeemCodeRepo: &redeemRepoStub{}}

	role := RoleUser
	updated, err := svc.UpdateUser(ctx, seedUser.ID, &UpdateUserInput{Role: &role})
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, RoleUser, updated.Role)
	require.Equal(t, []int64{groupTwo.ID, groupFive.ID}, updated.AllowedGroups)

	got, err := svc.GetUser(ctx, seedUser.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{groupTwo.ID, groupFive.ID}, got.AllowedGroups)
}

func TestAdminService_UpdateUser_ChannelAdminWithoutAllowedGroupInputKeepsExistingAllowedGroups(t *testing.T) {
	client, db := newAdminServiceUserAllowedGroupsTestClient(t, "update_keep_allowed_groups")
	repo := repository.NewUserRepository(client, db)
	ctx := context.Background()
	groupOne := createAdminServiceUserAllowedGroupsTestGroup(t, client, ctx, "keep-group-1")
	groupThree := createAdminServiceUserAllowedGroupsTestGroup(t, client, ctx, "keep-group-3")
	seedUser := createAdminServiceUserAllowedGroupsTestUser(t, client, ctx, "keep-channel-admin@test.com", RoleChannelAdmin, groupThree.ID, groupOne.ID)
	svc := &adminServiceImpl{userRepo: repo, entClient: client, redeemCodeRepo: &redeemRepoStub{}}

	name := "renamed"
	updated, err := svc.UpdateUser(ctx, seedUser.ID, &UpdateUserInput{Username: &name})
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, RoleChannelAdmin, updated.Role)
	require.Equal(t, []int64{groupOne.ID, groupThree.ID}, updated.AllowedGroups)

	got, err := svc.GetUser(ctx, seedUser.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{groupOne.ID, groupThree.ID}, got.AllowedGroups)
}

func TestAdminService_UpdateUser_InvalidatesAuthCacheOnAllowedGroupsChange(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 42, Email: "allowed-groups@test.com", AllowedGroups: []int64{1, 2}, Status: StatusActive, Role: RoleUser}}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             baseRepo,
		redeemCodeRepo:       &redeemRepoStub{},
		authCacheInvalidator: invalidator,
	}

	allowedGroups := []int64{1, 3}
	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{AllowedGroups: &allowedGroups})
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, []int64{1, 3}, updated.AllowedGroups)
	require.Equal(t, []int64{42}, invalidator.userIDs, "修改 AllowedGroups 应失效认证缓存")
}

func TestAdminService_UpdateUser_InvalidatesAuthCacheWhenGroupRatesProvided(t *testing.T) {
	baseRepo := &userRepoStub{user: &User{ID: 42, Email: "group-rates@test.com", Status: StatusActive, Role: RoleUser}}
	rateRepo := &userGroupRateSyncStubForUpdateUser{}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             baseRepo,
		userGroupRateRepo:    rateRepo,
		redeemCodeRepo:       &redeemRepoStub{},
		authCacheInvalidator: invalidator,
	}

	rate := 1.25
	groupRates := map[int64]*float64{7: &rate}
	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{GroupRates: groupRates})
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, []int64{42}, invalidator.userIDs, "提供 GroupRates 应失效认证缓存")
	require.Equal(t, int64(42), rateRepo.lastUserID)
	require.Len(t, rateRepo.lastRates, 1)
	require.NotNil(t, rateRepo.lastRates[7])
	require.Equal(t, 1.25, *rateRepo.lastRates[7])
}

func TestAdminService_UpdateUser_InvalidRole(t *testing.T) {
	repo := &userRepoStub{user: &User{ID: 12, Email: "update-role@test.com", Role: RoleUser, Status: StatusActive}}
	svc := &adminServiceImpl{userRepo: repo, redeemCodeRepo: &redeemRepoStub{}}

	role := "owner"
	_, err := svc.UpdateUser(context.Background(), 12, &UpdateUserInput{Role: &role})
	require.EqualError(t, err, "invalid role: owner")
	require.Empty(t, repo.updated)
}

func TestAdminService_CreateUser_EmailExists(t *testing.T) {
	repo := &userRepoStub{createErr: ErrEmailExists}
	svc := &adminServiceImpl{userRepo: repo}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "dup@test.com",
		Password: "password",
	})
	require.ErrorIs(t, err, ErrEmailExists)
	require.Empty(t, repo.created)
}

func TestAdminService_CreateUser_CreateError(t *testing.T) {
	createErr := errors.New("db down")
	repo := &userRepoStub{createErr: createErr}
	svc := &adminServiceImpl{userRepo: repo}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "user@test.com",
		Password: "password",
	})
	require.ErrorIs(t, err, createErr)
	require.Empty(t, repo.created)
}

func TestAdminService_CreateUser_AssignsDefaultSubscriptions(t *testing.T) {
	repo := &userRepoStub{nextID: 21}
	assigner := &defaultSubscriptionAssignerStub{}
	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserBalance:     0,
			UserConcurrency: 1,
		},
	}
	settingService := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyDefaultSubscriptions: `[{"group_id":5,"validity_days":30}]`,
	}}, cfg)
	svc := &adminServiceImpl{
		userRepo:           repo,
		settingService:     settingService,
		defaultSubAssigner: assigner,
	}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "new-user@test.com",
		Password: "password",
	})
	require.NoError(t, err)
	require.Len(t, assigner.calls, 1)
	require.Equal(t, int64(21), assigner.calls[0].UserID)
	require.Equal(t, int64(5), assigner.calls[0].GroupID)
	require.Equal(t, 30, assigner.calls[0].ValidityDays)
}

func newAdminServiceUserAllowedGroupsTestClient(t *testing.T, name string) (*dbent.Client, *sql.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name)
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client, db
}

func createAdminServiceUserAllowedGroupsTestUser(t *testing.T, client *dbent.Client, ctx context.Context, email string, role string, allowedGroups ...int64) *dbent.User {
	t.Helper()
	user, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetRole(role).
		SetStatus(StatusActive).
		Save(ctx)
	require.NoError(t, err)

	for _, groupID := range allowedGroups {
		_, err = client.UserAllowedGroup.Create().
			SetUserID(user.ID).
			SetGroupID(groupID).
			Save(ctx)
		require.NoError(t, err)
	}

	return user
}

type userGroupRateSyncStubForUpdateUser struct {
	lastUserID int64
	lastRates  map[int64]*float64
}

func (s *userGroupRateSyncStubForUpdateUser) GetByUserID(context.Context, int64) (map[int64]float64, error) {
	panic("unexpected GetByUserID call")
}

func (s *userGroupRateSyncStubForUpdateUser) GetByUserAndGroup(context.Context, int64, int64) (*float64, error) {
	panic("unexpected GetByUserAndGroup call")
}

func (s *userGroupRateSyncStubForUpdateUser) GetRPMOverrideByUserAndGroup(context.Context, int64, int64) (*int, error) {
	panic("unexpected GetRPMOverrideByUserAndGroup call")
}

func (s *userGroupRateSyncStubForUpdateUser) GetByGroupID(context.Context, int64) ([]UserGroupRateEntry, error) {
	panic("unexpected GetByGroupID call")
}

func (s *userGroupRateSyncStubForUpdateUser) SyncUserGroupRates(_ context.Context, userID int64, rates map[int64]*float64) error {
	s.lastUserID = userID
	s.lastRates = make(map[int64]*float64, len(rates))
	for groupID, rate := range rates {
		if rate == nil {
			s.lastRates[groupID] = nil
			continue
		}
		value := *rate
		s.lastRates[groupID] = &value
	}
	return nil
}

func (s *userGroupRateSyncStubForUpdateUser) SyncGroupRateMultipliers(context.Context, int64, []GroupRateMultiplierInput) error {
	panic("unexpected SyncGroupRateMultipliers call")
}

func (s *userGroupRateSyncStubForUpdateUser) SyncGroupRPMOverrides(context.Context, int64, []GroupRPMOverrideInput) error {
	panic("unexpected SyncGroupRPMOverrides call")
}

func (s *userGroupRateSyncStubForUpdateUser) ClearGroupRPMOverrides(context.Context, int64) error {
	panic("unexpected ClearGroupRPMOverrides call")
}

func (s *userGroupRateSyncStubForUpdateUser) DeleteByGroupID(context.Context, int64) error {
	panic("unexpected DeleteByGroupID call")
}

func (s *userGroupRateSyncStubForUpdateUser) DeleteByUserID(context.Context, int64) error {
	panic("unexpected DeleteByUserID call")
}

func createAdminServiceUserAllowedGroupsTestGroup(t *testing.T, client *dbent.Client, ctx context.Context, name string) *dbent.Group {
	t.Helper()
	group, err := client.Group.Create().
		SetName(name).
		SetPlatform(PlatformAnthropic).
		SetStatus(StatusActive).
		SetSubscriptionType(SubscriptionTypeStandard).
		SetRateMultiplier(1).
		SetIsExclusive(false).
		Save(ctx)
	require.NoError(t, err)
	return group
}
