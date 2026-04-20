package repository

import (
	"testing"

	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"

	entsql "entgo.io/ent/dialect/sql"
)

func TestUserListOrder_LastLoginAtUsesNullsLastOrdering(t *testing.T) {
	selector := entsql.Select().From(entsql.Table(dbuser.Table))
	for _, order := range userListOrder(pagination.PaginationParams{SortBy: "last_login_at", SortOrder: "desc"}) {
		order(selector)
	}

	query, _ := selector.Query()
	require.Contains(t, query, "ORDER BY")
	require.Contains(t, query, "`users`.`last_login_at` DESC NULLS LAST")
	require.Contains(t, query, "`users`.`id` DESC")
}

func TestUserListOrder_LastUsedAtJoinsUsageSummary(t *testing.T) {
	selector := entsql.Select().From(entsql.Table(dbuser.Table))
	for _, order := range userListOrder(pagination.PaginationParams{SortBy: "last_used_at", SortOrder: "asc"}) {
		order(selector)
	}

	query, _ := selector.Query()
	require.Contains(t, query, "LEFT JOIN")
	require.Contains(t, query, "MAX(created_at) AS `last_used_at`")
	require.Contains(t, query, "user_last_used_activity.last_used_at ASC NULLS LAST")
	require.Contains(t, query, "`users`.`id` ASC")
}
