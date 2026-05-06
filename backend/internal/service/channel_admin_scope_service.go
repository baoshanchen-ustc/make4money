package service

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/lib/pq"
)

var _ ChannelAdminScopeService = (*channelAdminScopeService)(nil)

var (
	allAuthorizedGroupIDsQuery = `
SELECT id
FROM groups
WHERE deleted_at IS NULL
ORDER BY id
`
	authorizedGroupIDsQuery = `
SELECT DISTINCT uag.group_id
FROM user_allowed_groups uag
JOIN groups g ON g.id = uag.group_id
WHERE uag.user_id = $1
  AND g.deleted_at IS NULL
ORDER BY uag.group_id
`
	canManageAccountGroupsQuery = `
SELECT COUNT(DISTINCT uag.group_id)
FROM user_allowed_groups uag
JOIN groups g ON g.id = uag.group_id
WHERE uag.user_id = $1
  AND g.deleted_at IS NULL
  AND uag.group_id = ANY($2)
`
	accountInScopeQuery = `
SELECT EXISTS (
    SELECT 1
    FROM accounts a
    WHERE a.id = $2
      AND a.deleted_at IS NULL
      AND (
          NOT EXISTS (
              SELECT 1
              FROM account_groups ag
              WHERE ag.account_id = a.id
          )
          OR EXISTS (
              SELECT 1
              FROM account_groups ag
              JOIN user_allowed_groups uag ON uag.group_id = ag.group_id
              JOIN groups g ON g.id = ag.group_id
              WHERE ag.account_id = a.id
                AND uag.user_id = $1
                AND g.deleted_at IS NULL
          )
      )
)
`
)

type ChannelAdminScopeService interface {
	AuthorizedGroupIDs(ctx context.Context, userID int64) ([]int64, error)
	CanManageAccountGroups(ctx context.Context, userID int64, groupIDs []int64) (bool, error)
	AccountInScope(ctx context.Context, userID, accountID int64) (bool, error)
}

type channelAdminScopeService struct {
	userRepo UserRepository
	db       *sql.DB
}

func NewChannelAdminScopeService(userRepo UserRepository, db *sql.DB) ChannelAdminScopeService {
	return &channelAdminScopeService{userRepo: userRepo, db: db}
}

func (s *channelAdminScopeService) AuthorizedGroupIDs(ctx context.Context, userID int64) ([]int64, error) {
	user, err := s.getUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	switch {
	case user.IsAdmin():
		return s.queryIDs(ctx, allAuthorizedGroupIDsQuery)
	case user.IsChannelAdmin():
		return s.queryIDs(ctx, authorizedGroupIDsQuery, userID)
	default:
		return []int64{}, nil
	}
}

func (s *channelAdminScopeService) CanManageAccountGroups(ctx context.Context, userID int64, groupIDs []int64) (bool, error) {
	user, err := s.getUser(ctx, userID)
	if err != nil {
		return false, err
	}

	switch {
	case user.IsAdmin():
		return true, nil
	case !user.IsChannelAdmin():
		return false, nil
	}

	normalized := normalizePositiveInt64IDs(groupIDs)
	if len(normalized) == 0 {
		return true, nil
	}

	var count int
	if err := s.db.QueryRowContext(ctx, canManageAccountGroupsQuery, userID, pq.Array(normalized)).Scan(&count); err != nil {
		return false, fmt.Errorf("check manageable account groups: %w", err)
	}
	return count == len(normalized), nil
}

func (s *channelAdminScopeService) AccountInScope(ctx context.Context, userID, accountID int64) (bool, error) {
	user, err := s.getUser(ctx, userID)
	if err != nil {
		return false, err
	}

	switch {
	case user.IsAdmin():
		return true, nil
	case !user.IsChannelAdmin():
		return false, nil
	}

	var inScope bool
	if err := s.db.QueryRowContext(ctx, accountInScopeQuery, userID, accountID).Scan(&inScope); err != nil {
		return false, fmt.Errorf("check account in scope: %w", err)
	}
	return inScope, nil
}

func (s *channelAdminScopeService) getUser(ctx context.Context, userID int64) (*User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user %d: %w", userID, err)
	}
	return user, nil
}

func (s *channelAdminScopeService) queryIDs(ctx context.Context, query string, args ...any) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query authorized group IDs: %w", err)
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan authorized group ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate authorized group IDs: %w", err)
	}
	return ids, nil
}

func normalizePositiveInt64IDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
