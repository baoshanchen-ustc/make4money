//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type loginUserRepoStub struct {
	userRepoStub
	expectedEmail string
	requestedEmail string
}

func (s *loginUserRepoStub) GetByEmail(_ context.Context, email string) (*User, error) {
	s.requestedEmail = email
	if email != s.expectedEmail || s.user == nil {
		return nil, ErrUserNotFound
	}
	return s.user, nil
}

func TestAuthService_Login_NormalizesEmailBeforeLookup(t *testing.T) {
	repo := &loginUserRepoStub{expectedEmail: "admin@example.com"}
	user := &User{
		ID:     1,
		Email:  "admin@example.com",
		Role:   RoleAdmin,
		Status: StatusActive,
	}
	require.NoError(t, user.SetPassword("secret123"))
	repo.user = user

	service := newAuthService(&repo.userRepoStub, nil, nil)
	service.userRepo = repo

	token, gotUser, err := service.Login(context.Background(), " Admin@Example.com ", "secret123")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.Equal(t, user, gotUser)
	require.Equal(t, "admin@example.com", repo.requestedEmail)
}
