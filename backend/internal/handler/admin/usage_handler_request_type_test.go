package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminUsageRepoCapture struct {
	service.UsageLogRepository
	listParams   pagination.PaginationParams
	listFilters  usagestats.UsageLogFilters
	statsFilters usagestats.UsageLogFilters
}

func (s *adminUsageRepoCapture) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters usagestats.UsageLogFilters) ([]service.UsageLog, *pagination.PaginationResult, error) {
	s.listParams = params
	s.listFilters = filters
	return []service.UsageLog{}, &pagination.PaginationResult{
		Total:    0,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    0,
	}, nil
}

func (s *adminUsageRepoCapture) GetStatsWithFilters(ctx context.Context, filters usagestats.UsageLogFilters) (*usagestats.UsageStats, error) {
	s.statsFilters = filters
	return &usagestats.UsageStats{}, nil
}

type adminUsageScopeServiceStub struct {
	authorizedGroupIDs []int64
}

func (s *adminUsageScopeServiceStub) AuthorizedGroupIDs(ctx context.Context, userID int64) ([]int64, error) {
	return s.authorizedGroupIDs, nil
}

func (s *adminUsageScopeServiceStub) CanManageAccountGroups(ctx context.Context, userID int64, groupIDs []int64) (bool, error) {
	panic("unexpected CanManageAccountGroups call")
}

func (s *adminUsageScopeServiceStub) AccountInScope(ctx context.Context, userID, accountID int64) (bool, error) {
	panic("unexpected AccountInScope call")
}

func newAdminUsageRequestTypeTestRouter(repo *adminUsageRepoCapture) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, nil, nil, nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Set(string(middleware.ContextKeyUserRole), service.RoleAdmin)
		c.Next()
	})
	router.GET("/admin/usage", handler.List)
	router.GET("/admin/usage/stats", handler.Stats)
	return router
}

func newAdminUsageScopeRouter(repo *adminUsageRepoCapture, adminSvc service.AdminService, scopeSvc service.ChannelAdminScopeService, settings *service.SystemSettings, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, nil, adminSvc, nil, scopeSvc, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		if role != "" {
			c.Set(string(middleware.ContextKeyUserRole), role)
		}
		c.Next()
	})
	router.GET("/admin/usage", handler.List)
	router.GET("/admin/usage/stats", handler.Stats)
	router.GET("/admin/usage/search-users", handler.SearchUsers)
	return router
}

func TestAdminUsageListRequestTypePriority(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?request_type=ws_v2&stream=false", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.listFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeWSV2), *repo.listFilters.RequestType)
	require.Nil(t, repo.listFilters.Stream)
}

func TestAdminUsageListInvalidRequestType(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?request_type=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsageListInvalidStream(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsageListExactTotalTrue(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?exact_total=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, repo.listFilters.ExactTotal)
}

func TestAdminUsageListInvalidExactTotal(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?exact_total=oops", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsageStatsRequestTypePriority(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?request_type=stream&stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.statsFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeStream), *repo.statsFilters.RequestType)
	require.Nil(t, repo.statsFilters.Stream)
}

func TestAdminUsageStatsInvalidRequestType(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?request_type=oops", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsageStatsInvalidStream(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?stream=oops", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsageListChannelAdminIgnoresAuthorizedChannelsScope(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageScopeRouter(
		repo,
		nil,
		&adminUsageScopeServiceStub{},
		&service.SystemSettings{ChannelAdminUsageScope: service.ChannelAdminUsageScopeAuthorizedChannels},
		service.RoleChannelAdmin,
	)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Nil(t, repo.listFilters.ChannelIDs)
	require.Nil(t, repo.listFilters.GroupIDs)
}

func TestAdminUsageListChannelAdminIgnoresAuthorizedGroupsScope(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageScopeRouter(
		repo,
		nil,
		&adminUsageScopeServiceStub{authorizedGroupIDs: []int64{21, 22}},
		&service.SystemSettings{ChannelAdminUsageScope: service.ChannelAdminUsageScopeAuthorizedGroups},
		service.RoleChannelAdmin,
	)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Nil(t, repo.listFilters.ChannelIDs)
	require.Nil(t, repo.listFilters.GroupIDs)
}

func TestAdminUsageListChannelAdminIgnoresAllScope(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageScopeRouter(
		repo,
		nil,
		&adminUsageScopeServiceStub{authorizedGroupIDs: []int64{41}},
		&service.SystemSettings{ChannelAdminUsageScope: service.ChannelAdminUsageScopeAll},
		service.RoleChannelAdmin,
	)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Nil(t, repo.listFilters.ChannelIDs)
	require.Nil(t, repo.listFilters.GroupIDs)
}

func TestAdminUsageListChannelAdminPreservesExplicitGroupFilter(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageScopeRouter(
		repo,
		nil,
		&adminUsageScopeServiceStub{authorizedGroupIDs: []int64{51}},
		&service.SystemSettings{ChannelAdminUsageScope: service.ChannelAdminUsageScopeAuthorizedGroups},
		service.RoleChannelAdmin,
	)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?group_id=77", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(77), repo.listFilters.GroupID)
	require.Nil(t, repo.listFilters.ChannelIDs)
	require.Nil(t, repo.listFilters.GroupIDs)
}

func TestAdminUsageStatsChannelAdminIgnoresAuthorizedChannelsScope(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageScopeRouter(
		repo,
		nil,
		&adminUsageScopeServiceStub{},
		&service.SystemSettings{ChannelAdminUsageScope: service.ChannelAdminUsageScopeAuthorizedChannels},
		service.RoleChannelAdmin,
	)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Nil(t, repo.statsFilters.ChannelIDs)
	require.Nil(t, repo.statsFilters.GroupIDs)
}

func TestAdminUsageStatsChannelAdminPreservesExplicitGroupFilter(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageScopeRouter(
		repo,
		nil,
		&adminUsageScopeServiceStub{authorizedGroupIDs: []int64{71}},
		&service.SystemSettings{ChannelAdminUsageScope: service.ChannelAdminUsageScopeAuthorizedChannels},
		service.RoleChannelAdmin,
	)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?group_id=88", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(88), repo.statsFilters.GroupID)
	require.Nil(t, repo.statsFilters.ChannelIDs)
	require.Nil(t, repo.statsFilters.GroupIDs)
}

func TestAdminUsageSearchUsersChannelAdminForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewUsageHandler(nil, nil, nil, nil, &adminUsageScopeServiceStub{}, nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/usage/search-users?q=test", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
	c.Set(string(middleware.ContextKeyUserRole), service.RoleChannelAdmin)

	handler.SearchUsers(c)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminUsageSearchAPIKeysChannelAdminForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewUsageHandler(nil, nil, nil, nil, &adminUsageScopeServiceStub{}, nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/usage/search-api-keys?user_id=123&q=test", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
	c.Set(string(middleware.ContextKeyUserRole), service.RoleChannelAdmin)

	handler.SearchAPIKeys(c)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminUsageListAdminUnchanged(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageScopeRouter(
		repo,
		nil,
		&adminUsageScopeServiceStub{authorizedGroupIDs: []int64{71}},
		&service.SystemSettings{ChannelAdminUsageScope: service.ChannelAdminUsageScopeAuthorizedChannels},
		service.RoleAdmin,
	)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?request_type=ws_v2&stream=false&exact_total=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.listFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeWSV2), *repo.listFilters.RequestType)
	require.Nil(t, repo.listFilters.Stream)
	require.True(t, repo.listFilters.ExactTotal)
	require.Nil(t, repo.listFilters.ChannelIDs)
	require.Nil(t, repo.listFilters.GroupIDs)
}

func TestAdminUsageStatsAdminUnchanged(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageScopeRouter(
		repo,
		nil,
		&adminUsageScopeServiceStub{},
		&service.SystemSettings{ChannelAdminUsageScope: service.ChannelAdminUsageScopeAuthorizedChannels},
		service.RoleAdmin,
	)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?request_type=stream&stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.statsFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeStream), *repo.statsFilters.RequestType)
	require.Nil(t, repo.statsFilters.Stream)
	require.Nil(t, repo.statsFilters.ChannelIDs)
	require.Nil(t, repo.statsFilters.GroupIDs)
}

func TestAdminUsageForbiddenWithoutRole(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageScopeRouter(
		repo,
		nil,
		&adminUsageScopeServiceStub{},
		&service.SystemSettings{ChannelAdminUsageScope: service.ChannelAdminUsageScopeAuthorizedChannels},
		"",
	)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}
