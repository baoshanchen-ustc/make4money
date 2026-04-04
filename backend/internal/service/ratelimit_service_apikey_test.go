//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRateLimitService_HandleUpstreamError_OpenAIAPIKey403ModelAccessDoesNotDisable(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	service := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:       104,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
	}

	shouldDisable := service.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusForbidden,
		http.Header{},
		[]byte(`{"error":{"message":"model not allowed for this project","code":"forbidden"}}`),
	)

	require.False(t, shouldDisable)
	require.Equal(t, 0, repo.setErrorCalls)
}
