package routes

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RegisterCommonRoutes 注册通用路由（健康检查、状态等）
func RegisterCommonRoutes(r *gin.Engine) {
	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Claude Code 遥测日志：接管、清洗、异步放行
	telemetrySvc := service.NewTelemetryService()
	r.POST("/api/event_logging/batch", func(c *gin.Context) {
		token := c.GetHeader("x-api-key")

		bodyBytes, err := c.GetRawData()
		if err != nil {
			c.Status(http.StatusOK)
			return
		}
		if len(bodyBytes) == 0 {
			c.Status(http.StatusOK)
			return
		}

		// 同步清洗（微秒级），异步转发（ForwardBackground 内部管理 goroutine）
		cleanedBytes, err := telemetrySvc.DeepScrubPayload(bodyBytes)
		if err == nil {
			telemetrySvc.ForwardBackground(cleanedBytes, token)
		}

		c.Status(http.StatusOK)
	})

	// Setup status endpoint (always returns needs_setup: false in normal mode)
	// This is used by the frontend to detect when the service has restarted after setup
	r.GET("/setup/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"needs_setup": false,
				"step":        "completed",
			},
		})
	})
}
