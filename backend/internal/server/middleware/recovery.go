package middleware

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Recovery converts panics into the project's standard JSON error envelope.
//
// It preserves Gin's broken-pipe handling by not attempting to write a response
// when the client connection is already gone.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(gin.DefaultErrorWriter, func(c *gin.Context, recovered any) {
		recoveredErr, _ := recovered.(error)

		logRecoveryPanic(c, recovered, recoveredErr)

		if isBrokenPipe(recoveredErr) {
			if recoveredErr != nil {
				_ = c.Error(recoveredErr)
			}
			c.Abort()
			return
		}

		if c.Writer.Written() {
			c.Abort()
			return
		}

		response.ErrorWithDetails(
			c,
			http.StatusInternalServerError,
			infraerrors.UnknownMessage,
			infraerrors.UnknownReason,
			nil,
		)
		c.Abort()
	})
}

func logRecoveryPanic(c *gin.Context, recovered any, err error) {
	if c == nil {
		return
	}
	message := fmt.Sprintf("%v", recovered)
	fields := map[string]any{
		"panic":             message,
		"stack":             string(debug.Stack()),
		"method":            c.Request.Method,
		"path":              firstNonEmpty(c.FullPath(), c.Request.URL.Path),
		"remote_addr":       c.ClientIP(),
		"request_id":        ctxStringValue(c.Request.Context(), ctxkey.RequestID),
		"client_request_id": ctxStringValue(c.Request.Context(), ctxkey.ClientRequestID),
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	logger.WriteSinkEvent("error", "middleware.recovery", "panic recovered", fields)
}

func ctxStringValue(ctx context.Context, key ctxkey.Key) string {
	if ctx == nil {
		return ""
	}
	val, _ := ctx.Value(key).(string)
	return strings.TrimSpace(val)
}

func firstNonEmpty(first, fallback string) string {
	if first != "" {
		return first
	}
	return fallback
}

func isBrokenPipe(err error) bool {
	if err == nil {
		return false
	}

	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}

	var syscallErr *os.SyscallError
	if !errors.As(opErr.Err, &syscallErr) {
		return false
	}

	msg := strings.ToLower(syscallErr.Error())
	return strings.Contains(msg, "broken pipe") || strings.Contains(msg, "connection reset by peer")
}
