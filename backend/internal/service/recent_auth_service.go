package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrRecentAuthRequired = infraerrors.Unauthorized("RECENT_AUTH_REQUIRED", "recent authentication is required")
)

const (
	recentAuthTTL                = 5 * time.Minute
	RecentAuthMethodPassword     = "password"
	RecentAuthMethodPasswordTOTP = "password_totp"
	RecentAuthMethodPasskey      = "passkey"
)

type RecentAuthMarker struct {
	UserID   int64     `json:"user_id"`
	Method   string    `json:"method"`
	IssuedAt time.Time `json:"issued_at"`
}

type RecentAuthService struct {
	cache AuthStateCache
}

func NewRecentAuthService(cache AuthStateCache) *RecentAuthService {
	return &RecentAuthService{cache: cache}
}

func (s *RecentAuthService) IssueRecentAuth(ctx context.Context, userID int64, method string) error {
	if userID <= 0 {
		return infraerrors.BadRequest("RECENT_AUTH_USER_ID_INVALID", "user id is invalid")
	}
	if s.cache == nil {
		return fmt.Errorf("auth state cache is not configured")
	}

	method = strings.TrimSpace(method)
	if method == "" {
		method = RecentAuthMethodPassword
	}

	marker := &RecentAuthMarker{
		UserID:   userID,
		Method:   method,
		IssuedAt: time.Now().UTC(),
	}

	if err := s.cache.SetRecentAuthMarker(ctx, userID, marker, recentAuthTTL); err != nil {
		return fmt.Errorf("set recent auth marker: %w", err)
	}

	return nil
}

func (s *RecentAuthService) RequireRecentAuth(ctx context.Context, userID int64) error {
	marker, err := s.GetRecentAuth(ctx, userID)
	if err != nil {
		return err
	}
	if marker == nil {
		return ErrRecentAuthRequired
	}
	return nil
}

func (s *RecentAuthService) GetRecentAuth(ctx context.Context, userID int64) (*RecentAuthMarker, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("RECENT_AUTH_USER_ID_INVALID", "user id is invalid")
	}
	if s.cache == nil {
		return nil, fmt.Errorf("auth state cache is not configured")
	}

	marker, err := s.cache.GetRecentAuthMarker(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get recent auth marker: %w", err)
	}
	if marker == nil {
		return nil, nil
	}

	if marker.UserID != userID {
		return nil, nil
	}

	return marker, nil
}
