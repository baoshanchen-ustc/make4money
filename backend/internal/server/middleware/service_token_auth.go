package middleware

import (
	"crypto/subtle"
	"os"

	"github.com/gin-gonic/gin"
)

// ServiceTokenAuthMiddleware is the middleware type for service-to-service token auth
type ServiceTokenAuthMiddleware gin.HandlerFunc

// NewServiceTokenAuthMiddleware creates a middleware that validates X-Service-Token header
// against the PROVISION_SERVICE_TOKEN environment variable.
func NewServiceTokenAuthMiddleware() ServiceTokenAuthMiddleware {
	return ServiceTokenAuthMiddleware(func(c *gin.Context) {
		token := c.GetHeader("X-Service-Token")
		if token == "" {
			AbortWithError(c, 401, "UNAUTHORIZED", "X-Service-Token header required")
			return
		}

		expected := os.Getenv("PROVISION_SERVICE_TOKEN")
		if expected == "" {
			AbortWithError(c, 503, "SERVICE_UNAVAILABLE", "provision service not configured")
			return
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			AbortWithError(c, 401, "INVALID_SERVICE_TOKEN", "invalid service token")
			return
		}

		c.Next()
	})
}
