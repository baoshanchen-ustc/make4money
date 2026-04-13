package middleware

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Logger 请求日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		startTime := time.Now()

		// 请求路径
		path := c.Request.URL.Path

		// 处理请求
		c.Next()

		// 跳过健康检查等高频探针路径的日志
		if path == "/health" || path == "/readyz" || path == "/livez" || path == "/api/health" || path == "/api/readyz" || path == "/metrics" || path == "/setup/status" {
			return
		}

		endTime := time.Now()
		latency := endTime.Sub(startTime)

		method := c.Request.Method
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		protocol := c.Request.Proto
		accountID, hasAccountID := c.Request.Context().Value(ctxkey.AccountID).(int64)
		platform, _ := c.Request.Context().Value(ctxkey.Platform).(string)
		model, _ := c.Request.Context().Value(ctxkey.Model).(string)

		fields := []zap.Field{
			zap.String("component", "http.access"),
			zap.Int("status_code", statusCode),
			zap.Int64("latency_ms", latency.Milliseconds()),
			zap.String("client_ip", clientIP),
			zap.String("protocol", protocol),
			zap.String("method", method),
			zap.String("path", path),
		}
		if hasAccountID && accountID > 0 {
			fields = append(fields, zap.Int64("account_id", accountID))
		}
		if platform != "" {
			fields = append(fields, zap.String("platform", platform))
		}
		if model != "" {
			fields = append(fields, zap.String("model", model))
		}

		fields = append(fields, zap.Time("completed_at", endTime))
		l := logger.FromContext(c.Request.Context()).With(fields...)
		l.Info("http request completed")
		requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
		clientRequestID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		sinkFields := map[string]any{
			"component":         "http.access",
			"request_id":        requestID,
			"client_request_id": clientRequestID,
			"status_code":       statusCode,
			"latency_ms":        latency.Milliseconds(),
			"client_ip":         clientIP,
			"protocol":          protocol,
			"method":            method,
			"path":              path,
		}
		if hasAccountID && accountID > 0 {
			sinkFields["account_id"] = accountID
		}
		if platform != "" {
			sinkFields["platform"] = platform
		}
		if model != "" {
			sinkFields["model"] = model
		}

		logger.WriteSinkEvent("info", "http.access", "http request completed", sinkFields)

		if len(c.Errors) > 0 {
			l.Warn("http request contains gin errors", zap.String("errors", c.Errors.String()))
			errorFields := map[string]any{
				"component": "http.access",
				"errors":    c.Errors.String(),
			}
			logger.WriteSinkEvent("warn", "http.access", "http request contains gin errors", errorFields)
		}
	}
}
