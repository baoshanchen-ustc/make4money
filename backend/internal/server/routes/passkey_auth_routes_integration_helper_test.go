//go:build integration

package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func newPasskeyAuthRoutesTestRouter(t *testing.T, redisClient *redis.Client, stub *passkeyRouteServiceStub) (*gin.Engine, *passkeyRouteAuthStateCacheStub) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	settingSvc := newPasskeyRouteSettingService(t, false)
	recentAuthCache := newPasskeyRouteAuthStateCacheStub()
	recentAuthSvc := service.NewRecentAuthService(recentAuthCache)
	authSvc := service.NewAuthService(nil, nil, nil, newPasskeyRouteRefreshTokenCacheStub(), newPasskeyRouteConfig(), settingSvc, nil, nil, nil, nil, nil)
	authHandler := handler.NewAuthHandler(newPasskeyRouteConfig(), authSvc, nil, settingSvc, nil, nil, nil, recentAuthSvc)
	setPasskeyRouteField(authHandler, "passkeyService", stub)

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterAuthRoutes(
		v1,
		&handler.Handlers{
			Auth:    authHandler,
			Setting: &handler.SettingHandler{},
		},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		redisClient,
		settingSvc,
	)
	return router, recentAuthCache
}
