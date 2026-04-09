package routes

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserRoutes_RegisterCheckInEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterUserRoutes(
		v1,
		&handler.Handlers{
			User:         &handler.UserHandler{},
			Totp:         &handler.TotpHandler{},
			APIKey:       &handler.APIKeyHandler{},
			Usage:        &handler.UsageHandler{},
			Announcement: &handler.AnnouncementHandler{},
			Redeem:       &handler.RedeemHandler{},
			CheckIn:      &handler.CheckInHandler{},
			Subscription: &handler.SubscriptionHandler{},
		},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		nil,
	)

	require.True(t, routeExists(router, http.MethodGet, "/api/v1/check-in/status"), "missing GET /api/v1/check-in/status")
	require.True(t, routeExists(router, http.MethodPost, "/api/v1/check-in"), "missing POST /api/v1/check-in")
	require.True(t, routeExists(router, http.MethodGet, "/api/v1/check-in/history"), "missing GET /api/v1/check-in/history")
}

func TestAdminRoutes_RegisterUserCheckInHistoryEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	adminGroup := v1.Group("/admin")

	registerUserManagementRoutes(adminGroup, &handler.Handlers{
		Admin: &handler.AdminHandlers{
			User:          &adminhandler.UserHandler{},
			UserAttribute: &adminhandler.UserAttributeHandler{},
		},
	})

	require.True(
		t,
		routeExists(router, http.MethodGet, "/api/v1/admin/users/:id/check-in-history"),
		"missing GET /api/v1/admin/users/:id/check-in-history",
	)
}

func routeExists(router *gin.Engine, method, path string) bool {
	for _, route := range router.Routes() {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
