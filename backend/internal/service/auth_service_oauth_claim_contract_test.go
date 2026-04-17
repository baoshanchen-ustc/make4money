//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthService_CreateAccountFromPendingOAuthIdentity_UsesProviderEmailWithoutVerifyCodeForNewLinuxDoUser(t *testing.T) {
	repo := &oauthFlowUserRepoStub{
		usersByID:    map[int64]*User{},
		usersByEmail: map[string]*User{},
		nextID:       21,
	}
	svc := newAuthServiceForOAuthConfirmTest(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	tokenPair, user, err := svc.CreateAccountFromPendingOAuthIdentity(context.Background(), PendingOAuthIdentity{
		Email:    "linuxdo-owner@example.com",
		Provider: ExternalIdentityProviderLinuxDo,
		Subject:  "linuxdo-subject-21",
		Username: "linuxdo_owner",
	}, "", "", "")
	require.NoError(t, err)
	require.NotNil(t, tokenPair)
	require.NotNil(t, user)
	require.Equal(t, "linuxdo-owner@example.com", user.Email)
	require.Len(t, repo.created, 1)
	require.Len(t, repo.upserted, 1)
	require.Equal(t, ExternalIdentityProviderLinuxDo, repo.upserted[0].Provider)
	require.Equal(t, "linuxdo-subject-21", repo.upserted[0].ProviderUserID)
}

func TestAuthService_CreateAccountFromPendingOAuthIdentity_RejectsExistingProviderEmailWithoutOwnershipVerification(t *testing.T) {
	existing := &User{ID: 9, Email: "owner@example.com", Username: "owner", Role: RoleUser, Status: StatusActive}
	repo := &oauthFlowUserRepoStub{
		usersByID:    map[int64]*User{existing.ID: existing},
		usersByEmail: map[string]*User{existing.Email: existing},
		nextID:       10,
	}
	svc := newAuthServiceForOAuthConfirmTest(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	tokenPair, user, err := svc.CreateAccountFromPendingOAuthIdentity(context.Background(), PendingOAuthIdentity{
		Email:    "owner@example.com",
		Provider: ExternalIdentityProviderLinuxDo,
		Subject:  "linuxdo-subject-22",
		Username: "linuxdo_owner",
	}, "", "", "")
	require.ErrorIs(t, err, ErrEmailExists)
	require.Nil(t, tokenPair)
	require.Nil(t, user)
	require.Len(t, repo.upserted, 0)
}
