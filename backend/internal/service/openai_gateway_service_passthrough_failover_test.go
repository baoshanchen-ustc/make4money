package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldFailoverOpenAIPassthroughResponse(t *testing.T) {
	for _, statusCode := range []int{
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		529,
	} {
		require.Truef(t, shouldFailoverOpenAIPassthroughResponse(statusCode), "status=%d should fail over", statusCode)
	}

	for _, statusCode := range []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusConflict,
	} {
		require.Falsef(t, shouldFailoverOpenAIPassthroughResponse(statusCode), "status=%d should not fail over", statusCode)
	}
}
