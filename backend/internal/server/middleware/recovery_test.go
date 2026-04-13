//go:build unit

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRecovery_PanicLogContainsInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 临时替换 DefaultErrorWriter 以捕获日志输出
	var buf bytes.Buffer
	originalWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &buf
	t.Cleanup(func() {
		gin.DefaultErrorWriter = originalWriter
	})

	r := gin.New()
	r.Use(Recovery())
	r.GET("/panic", func(c *gin.Context) {
		panic("custom panic message for test")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	logOutput := buf.String()
	require.Contains(t, logOutput, "custom panic message for test", "日志应包含 panic 信息")
	require.Contains(t, logOutput, "recovery_test.go", "日志应包含堆栈跟踪文件名")
}

func TestRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		handler      gin.HandlerFunc
		wantHTTPCode int
		wantBody     response.Response
	}{
		{
			name: "panic_returns_standard_json_500",
			handler: func(c *gin.Context) {
				panic("boom")
			},
			wantHTTPCode: http.StatusInternalServerError,
			wantBody: response.Response{
				Code:    http.StatusInternalServerError,
				Message: infraerrors.UnknownMessage,
			},
		},
		{
			name: "no_panic_passthrough",
			handler: func(c *gin.Context) {
				response.Success(c, gin.H{"ok": true})
			},
			wantHTTPCode: http.StatusOK,
			wantBody: response.Response{
				Code:    0,
				Message: "success",
				Data:    map[string]any{"ok": true},
			},
		},
		{
			name: "panic_after_write_does_not_override_body",
			handler: func(c *gin.Context) {
				response.Success(c, gin.H{"ok": true})
				panic("boom")
			},
			wantHTTPCode: http.StatusOK,
			wantBody: response.Response{
				Code:    0,
				Message: "success",
				Data:    map[string]any{"ok": true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(Recovery())
			r.GET("/t", tt.handler)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/t", nil)
			r.ServeHTTP(w, req)

			require.Equal(t, tt.wantHTTPCode, w.Code)

			var got response.Response
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			require.Equal(t, tt.wantBody, got)
		})
	}
}

func TestRecovery_LogsStructuredPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sink := &testSink{}
	logger.SetSink(sink)
	t.Cleanup(func() {
		logger.SetSink(nil)
	})

	r := gin.New()
	r.Use(Recovery())
	r.GET("/panic", func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), ctxkey.RequestID, "req-123")
		ctx = context.WithValue(ctx, ctxkey.ClientRequestID, "client-xyz")
		c.Request = c.Request.WithContext(ctx)
		panic("boom")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Len(t, sink.events, 1)
	event := sink.events[0]
	require.Equal(t, "panic recovered", event.Message)
	require.Equal(t, "middleware.recovery", event.Component)
	require.Equal(t, "req-123", event.Fields["request_id"])
	require.Equal(t, "client-xyz", event.Fields["client_request_id"])
}

type testSink struct {
	events []*logger.LogEvent
}

func (t *testSink) WriteLogEvent(event *logger.LogEvent) {
	t.events = append(t.events, event)
}
