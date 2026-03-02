package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterProvisionRoutes registers the /api/provision endpoint
func RegisterProvisionRoutes(
	r *gin.Engine,
	h *handler.Handlers,
	serviceTokenAuth middleware.ServiceTokenAuthMiddleware,
) {
	api := r.Group("/api")
	api.Use(gin.HandlerFunc(serviceTokenAuth))
	{
		api.POST("/provision", h.Provision.Provision)
	}
}
