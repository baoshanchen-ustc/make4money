//go:build !unit

package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type mockGroupRepoForGateway struct {
	groups           map[int64]*Group
	getByIDCalls     int
	getByIDLiteCalls int
}

func (m *mockGroupRepoForGateway) GetByID(ctx context.Context, id int64) (*Group, error) {
	m.getByIDCalls++
	if g, ok := m.groups[id]; ok {
		return g, nil
	}
	return nil, ErrGroupNotFound
}

func (m *mockGroupRepoForGateway) GetByIDLite(ctx context.Context, id int64) (*Group, error) {
	m.getByIDLiteCalls++
	if g, ok := m.groups[id]; ok {
		return g, nil
	}
	return nil, ErrGroupNotFound
}

func (m *mockGroupRepoForGateway) Create(ctx context.Context, group *Group) error { return nil }
func (m *mockGroupRepoForGateway) Update(ctx context.Context, group *Group) error { return nil }
func (m *mockGroupRepoForGateway) Delete(ctx context.Context, id int64) error     { return nil }
func (m *mockGroupRepoForGateway) DeleteCascade(ctx context.Context, id int64) ([]int64, error) {
	return nil, nil
}
func (m *mockGroupRepoForGateway) List(ctx context.Context, params pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (m *mockGroupRepoForGateway) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, status, search string, isExclusive *bool) ([]Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (m *mockGroupRepoForGateway) ListActive(ctx context.Context) ([]Group, error) {
	return nil, nil
}
func (m *mockGroupRepoForGateway) ListActiveByPlatform(ctx context.Context, platform string) ([]Group, error) {
	return nil, nil
}
func (m *mockGroupRepoForGateway) ExistsByName(ctx context.Context, name string) (bool, error) {
	return false, nil
}
func (m *mockGroupRepoForGateway) GetAccountCount(ctx context.Context, groupID int64) (int64, int64, error) {
	return 0, 0, nil
}
func (m *mockGroupRepoForGateway) DeleteAccountGroupsByGroupID(ctx context.Context, groupID int64) (int64, error) {
	return 0, nil
}
func (m *mockGroupRepoForGateway) BindAccountsToGroup(ctx context.Context, groupID int64, accountIDs []int64) error {
	return nil
}
func (m *mockGroupRepoForGateway) GetAccountIDsByGroupIDs(ctx context.Context, groupIDs []int64) ([]int64, error) {
	return nil, nil
}
func (m *mockGroupRepoForGateway) UpdateSortOrders(ctx context.Context, updates []GroupSortOrderUpdate) error {
	return nil
}
