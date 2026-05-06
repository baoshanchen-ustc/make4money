package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type channelAdminScopeUserRepoStub struct {
	user   *User
	getErr error
}

func (s *channelAdminScopeUserRepoStub) Create(ctx context.Context, user *User) error {
	panic("unexpected Create call")
}

func (s *channelAdminScopeUserRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.user == nil {
		return nil, ErrUserNotFound
	}
	return s.user, nil
}

func (s *channelAdminScopeUserRepoStub) GetByEmail(ctx context.Context, email string) (*User, error) {
	panic("unexpected GetByEmail call")
}

func (s *channelAdminScopeUserRepoStub) GetFirstAdmin(ctx context.Context) (*User, error) {
	panic("unexpected GetFirstAdmin call")
}

func (s *channelAdminScopeUserRepoStub) Update(ctx context.Context, user *User) error {
	panic("unexpected Update call")
}

func (s *channelAdminScopeUserRepoStub) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (s *channelAdminScopeUserRepoStub) GetUserAvatar(ctx context.Context, userID int64) (*UserAvatar, error) {
	panic("unexpected GetUserAvatar call")
}

func (s *channelAdminScopeUserRepoStub) UpsertUserAvatar(ctx context.Context, userID int64, input UpsertUserAvatarInput) (*UserAvatar, error) {
	panic("unexpected UpsertUserAvatar call")
}

func (s *channelAdminScopeUserRepoStub) DeleteUserAvatar(ctx context.Context, userID int64) error {
	panic("unexpected DeleteUserAvatar call")
}

func (s *channelAdminScopeUserRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *channelAdminScopeUserRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *channelAdminScopeUserRepoStub) GetLatestUsedAtByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*time.Time, error) {
	panic("unexpected GetLatestUsedAtByUserIDs call")
}

func (s *channelAdminScopeUserRepoStub) GetLatestUsedAtByUserID(ctx context.Context, userID int64) (*time.Time, error) {
	panic("unexpected GetLatestUsedAtByUserID call")
}

func (s *channelAdminScopeUserRepoStub) UpdateUserLastActiveAt(ctx context.Context, userID int64, activeAt time.Time) error {
	panic("unexpected UpdateUserLastActiveAt call")
}

func (s *channelAdminScopeUserRepoStub) UpdateBalance(ctx context.Context, id int64, amount float64) error {
	panic("unexpected UpdateBalance call")
}

func (s *channelAdminScopeUserRepoStub) DeductBalance(ctx context.Context, id int64, amount float64) error {
	panic("unexpected DeductBalance call")
}

func (s *channelAdminScopeUserRepoStub) UpdateConcurrency(ctx context.Context, id int64, amount int) error {
	panic("unexpected UpdateConcurrency call")
}

func (s *channelAdminScopeUserRepoStub) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	panic("unexpected ExistsByEmail call")
}

func (s *channelAdminScopeUserRepoStub) RemoveGroupFromAllowedGroups(ctx context.Context, groupID int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}

func (s *channelAdminScopeUserRepoStub) AddGroupToAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}

func (s *channelAdminScopeUserRepoStub) RemoveGroupFromUserAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	panic("unexpected RemoveGroupFromUserAllowedGroups call")
}

func (s *channelAdminScopeUserRepoStub) ListUserAuthIdentities(ctx context.Context, userID int64) ([]UserAuthIdentityRecord, error) {
	panic("unexpected ListUserAuthIdentities call")
}

func (s *channelAdminScopeUserRepoStub) UnbindUserAuthProvider(ctx context.Context, userID int64, provider string) error {
	panic("unexpected UnbindUserAuthProvider call")
}

func (s *channelAdminScopeUserRepoStub) UpdateTotpSecret(ctx context.Context, userID int64, encryptedSecret *string) error {
	panic("unexpected UpdateTotpSecret call")
}

func (s *channelAdminScopeUserRepoStub) EnableTotp(ctx context.Context, userID int64) error {
	panic("unexpected EnableTotp call")
}

func (s *channelAdminScopeUserRepoStub) DisableTotp(ctx context.Context, userID int64) error {
	panic("unexpected DisableTotp call")
}

func TestChannelAdminScopeServiceAuthorizedGroupIDsUsesUserAllowedGroups(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT DISTINCT uag.group_id
FROM user_allowed_groups uag
JOIN groups g ON g.id = uag.group_id
WHERE uag.user_id = $1
  AND g.deleted_at IS NULL
ORDER BY uag.group_id
`)).
		WithArgs(int64(21)).
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(int64(4)).AddRow(int64(8)))

	svc := NewChannelAdminScopeService(&channelAdminScopeUserRepoStub{user: &User{Role: RoleChannelAdmin}}, db)
	ids, err := svc.AuthorizedGroupIDs(context.Background(), 21)
	require.NoError(t, err)
	require.Equal(t, []int64{4, 8}, ids)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelAdminScopeServiceAuthorizedGroupIDsAdminReturnsEffectiveGroups(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(allAuthorizedGroupIDsQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(int64(5)).AddRow(int64(11)))

	svc := NewChannelAdminScopeService(&channelAdminScopeUserRepoStub{user: &User{Role: RoleAdmin}}, db)
	ids, err := svc.AuthorizedGroupIDs(context.Background(), 21)
	require.NoError(t, err)
	require.Equal(t, []int64{5, 11}, ids)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelAdminScopeServiceAuthorizedGroupIDsNormalUserReturnsEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewChannelAdminScopeService(&channelAdminScopeUserRepoStub{user: &User{Role: RoleUser}}, db)
	ids, err := svc.AuthorizedGroupIDs(context.Background(), 21)
	require.NoError(t, err)
	require.Empty(t, ids)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelAdminScopeServiceCanManageAccountGroupsAdminBypassesEmptyGroups(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewChannelAdminScopeService(&channelAdminScopeUserRepoStub{user: &User{Role: RoleAdmin}}, db)
	allowed, err := svc.CanManageAccountGroups(context.Background(), 12, nil)
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelAdminScopeServiceCanManageAccountGroupsAllowsNilForChannelAdmin(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewChannelAdminScopeService(&channelAdminScopeUserRepoStub{user: &User{Role: RoleChannelAdmin}}, db)
	allowed, err := svc.CanManageAccountGroups(context.Background(), 12, nil)
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelAdminScopeServiceCanManageAccountGroupsAllowsEmptySliceForChannelAdmin(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewChannelAdminScopeService(&channelAdminScopeUserRepoStub{user: &User{Role: RoleChannelAdmin}}, db)
	allowed, err := svc.CanManageAccountGroups(context.Background(), 12, []int64{})
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelAdminScopeServiceCanManageAccountGroupsAllowsNoPositiveIDsForChannelAdmin(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewChannelAdminScopeService(&channelAdminScopeUserRepoStub{user: &User{Role: RoleChannelAdmin}}, db)
	allowed, err := svc.CanManageAccountGroups(context.Background(), 9, []int64{0, -1, 0})
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestChannelAdminScopeServiceCanManageAccountGroupsChannelAdminQueriesNormalizedIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`
SELECT COUNT(DISTINCT uag.group_id)
FROM user_allowed_groups uag
JOIN groups g ON g.id = uag.group_id
WHERE uag.user_id = $1
  AND g.deleted_at IS NULL
  AND uag.group_id = ANY($2)
`)).
		WithArgs(int64(9), pq.Array([]int64{3, 8})).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	svc := NewChannelAdminScopeService(&channelAdminScopeUserRepoStub{user: &User{Role: RoleChannelAdmin}}, db)
	allowed, err := svc.CanManageAccountGroups(context.Background(), 9, []int64{8, -1, 8, 3})
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelAdminScopeServiceCanManageAccountGroupsNormalUserDenied(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewChannelAdminScopeService(&channelAdminScopeUserRepoStub{user: &User{Role: RoleUser}}, db)
	allowed, err := svc.CanManageAccountGroups(context.Background(), 9, []int64{8, 3})
	require.NoError(t, err)
	require.False(t, allowed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelAdminScopeServiceAccountInScopeAdminBypass(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewChannelAdminScopeService(&channelAdminScopeUserRepoStub{user: &User{Role: RoleAdmin}}, db)
	inScope, err := svc.AccountInScope(context.Background(), 10, 99)
	require.NoError(t, err)
	require.True(t, inScope)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelAdminScopeServiceAccountInScopeChannelAdminChecksQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(accountInScopeQuery)).
		WithArgs(int64(10), int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	svc := NewChannelAdminScopeService(&channelAdminScopeUserRepoStub{user: &User{Role: RoleChannelAdmin}}, db)
	inScope, err := svc.AccountInScope(context.Background(), 10, 99)
	require.NoError(t, err)
	require.True(t, inScope)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelAdminScopeServiceAccountInScopeNormalUserDenied(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := NewChannelAdminScopeService(&channelAdminScopeUserRepoStub{user: &User{Role: RoleUser}}, db)
	inScope, err := svc.AccountInScope(context.Background(), 10, 99)
	require.NoError(t, err)
	require.False(t, inScope)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNormalizePositiveInt64IDs(t *testing.T) {
	require.Equal(t, []int64{2, 3, 5}, normalizePositiveInt64IDs([]int64{5, 0, 3, 5, -1, 2}))
}
