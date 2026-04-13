//go:build integration

package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GeminiTokenCacheSuite struct {
	IntegrationRedisSuite
	cache service.GeminiTokenCache
}

func (s *GeminiTokenCacheSuite) SetupTest() {
	s.IntegrationRedisSuite.SetupTest()
	s.cache = NewGeminiTokenCache(s.rdb)
}

func (s *GeminiTokenCacheSuite) TestDeleteAccessToken() {
	cacheKey := "project-123"
	token := "token-value"
	require.NoError(s.T(), s.cache.SetAccessToken(s.ctx, cacheKey, token, time.Minute))

	got, err := s.cache.GetAccessToken(s.ctx, cacheKey)
	require.NoError(s.T(), err)
	require.Equal(s.T(), token, got)

	require.NoError(s.T(), s.cache.DeleteAccessToken(s.ctx, cacheKey))

	_, err = s.cache.GetAccessToken(s.ctx, cacheKey)
	require.True(s.T(), errors.Is(err, redis.Nil), "expected redis.Nil after delete")
}

func (s *GeminiTokenCacheSuite) TestDeleteAccessToken_MissingKey() {
	require.NoError(s.T(), s.cache.DeleteAccessToken(s.ctx, "missing-key"))
}

func (s *GeminiTokenCacheSuite) TestAcquireReleaseRefreshLock_OwnerMatch() {
	cacheKey := "lock-owner-match"
	owner := "owner-A"

	locked, err := s.cache.AcquireRefreshLock(s.ctx, cacheKey, time.Second, owner)
	s.Require().NoError(err)
	s.True(locked)

	s.Require().NoError(s.cache.ReleaseRefreshLock(s.ctx, cacheKey, owner))

	lockedAgain, err := s.cache.AcquireRefreshLock(s.ctx, cacheKey, time.Second, "owner-B")
	s.Require().NoError(err)
	s.True(lockedAgain)
	s.Require().NoError(s.cache.ReleaseRefreshLock(s.ctx, cacheKey, "owner-B"))
}

func (s *GeminiTokenCacheSuite) TestReleaseRefreshLock_OwnerMismatch() {
	cacheKey := "lock-owner-mismatch"
	ownerA := "owner-A"
	ownerB := "owner-B"

	locked, err := s.cache.AcquireRefreshLock(s.ctx, cacheKey, time.Second, ownerA)
	s.Require().NoError(err)
	s.True(locked)

	s.Require().NoError(s.cache.ReleaseRefreshLock(s.ctx, cacheKey, ownerB))

	val, err := s.rdb.Get(s.ctx, oauthRefreshLockKeyPrefix+cacheKey).Result()
	s.Require().NoError(err)
	s.Equal(ownerA, val)

	lockedB, err := s.cache.AcquireRefreshLock(s.ctx, cacheKey, time.Second, ownerB)
	s.Require().NoError(err)
	s.False(lockedB)

	s.Require().NoError(s.cache.ReleaseRefreshLock(s.ctx, cacheKey, ownerA))
}

func (s *GeminiTokenCacheSuite) TestReleaseRefreshLock_NoExistingLock() {
	cacheKey := "lock-release-no-op"
	owner := "owner-no-op"

	require.NoError(s.T(), s.cache.ReleaseRefreshLock(s.ctx, cacheKey, owner))

	locked, err := s.cache.AcquireRefreshLock(s.ctx, cacheKey, time.Second, owner)
	s.Require().NoError(err)
	s.True(locked)
	s.Require().NoError(s.cache.ReleaseRefreshLock(s.ctx, cacheKey, owner))
}

func (s *GeminiTokenCacheSuite) TestAcquireReleaseRefreshLock_ReacquireBeforeRelease() {
	cacheKey := "lock-acquire-reacquire"
	ownerA := "owner-A"
	ownerB := "owner-B"

	locked, err := s.cache.AcquireRefreshLock(s.ctx, cacheKey, time.Second, ownerA)
	s.Require().NoError(err)
	s.True(locked)

	lockedAgain, err := s.cache.AcquireRefreshLock(s.ctx, cacheKey, time.Second, ownerA)
	s.Require().NoError(err)
	s.False(lockedAgain, "re-acquiring while held should be rejected")

	lockedB, err := s.cache.AcquireRefreshLock(s.ctx, cacheKey, time.Second, ownerB)
	s.Require().NoError(err)
	s.False(lockedB, "another owner should not grab the lock while it is held")

	s.Require().NoError(s.cache.ReleaseRefreshLock(s.ctx, cacheKey, ownerA))

	lockedB, err = s.cache.AcquireRefreshLock(s.ctx, cacheKey, time.Second, ownerB)
	s.Require().NoError(err)
	s.True(lockedB)
	s.Require().NoError(s.cache.ReleaseRefreshLock(s.ctx, cacheKey, ownerB))
}

func TestGeminiTokenCacheSuite(t *testing.T) {
	suite.Run(t, new(GeminiTokenCacheSuite))
}
