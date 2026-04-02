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
	r.POST("/api/event_logging/batch", func(c *gin.Context) {
		// 1. 提取原始凭证
		token := c.GetHeader("x-api-key")

		// 2. 读取原始 payload
		bodyBytes, err := c.GetRawData()
		if err == nil && len(bodyBytes) > 0 {
			// 3. 进入拦截清洗协程（不阻塞给客户端返回 200）
			go func(body []byte, clientToken string) {
				importService := service.NewTelemetryService()
				// shadowDeviceID 现在由 DeepScrubPayload 在解析时根据原生 device_id 动态生成
				cleanedBytes, err := importService.DeepScrubPayload(body)
				if err == nil {
					importService.ForwardBackground(cleanedBytes, clientToken)
				}
			}(bodyBytes, token)
		}

		// 秒级放行，保证客户端侧无感
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
