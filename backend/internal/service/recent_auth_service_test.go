package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecentAuthService_IssueAndRequire(t *testing.T) {
	cache := newPasskeySvcAuthStateCacheStub()
	svc := NewRecentAuthService(cache)

	err := svc.IssueRecentAuth(context.Background(), 42, RecentAuthMethodPassword)
	require.NoError(t, err)

	err = svc.RequireRecentAuth(context.Background(), 42)
	require.NoError(t, err)

	marker, err := svc.GetRecentAuth(context.Background(), 42)
	require.NoError(t, err)
	require.NotNil(t, marker)
	require.Equal(t, int64(42), marker.UserID)
	require.Equal(t, RecentAuthMethodPassword, marker.Method)
}

func TestRecentAuthService_RequireRecentAuth_Expired(t *testing.T) {
	cache := newPasskeySvcAuthStateCacheStub()
	svc := NewRecentAuthService(cache)

	err := svc.IssueRecentAuth(context.Background(), 77, RecentAuthMethodPasswordTOTP)
	require.NoError(t, err)

	cache.now = time.Now().UTC().Add(6 * time.Minute)
	err = svc.RequireRecentAuth(context.Background(), 77)
	require.ErrorIs(t, err, ErrRecentAuthRequired)
}
