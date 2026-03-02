package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- stubs ---

type provisionUserRepoStub struct {
	UserRepository
	users     map[string]*User
	nextID    int64
	createErr error
}

func newProvisionUserRepoStub() *provisionUserRepoStub {
	return &provisionUserRepoStub{users: make(map[string]*User), nextID: 1}
}

func (r *provisionUserRepoStub) GetByEmail(_ context.Context, email string) (*User, error) {
	if u, ok := r.users[email]; ok {
		return u, nil
	}
	return nil, ErrUserNotFound
}

func (r *provisionUserRepoStub) Create(_ context.Context, user *User) error {
	if r.createErr != nil {
		return r.createErr
	}
	user.ID = r.nextID
	r.nextID++
	r.users[user.Email] = user
	return nil
}

type provisionAPIKeyRepoStub struct {
	APIKeyRepository
	keys   []APIKey
	nextID int64
}

func newProvisionAPIKeyRepoStub() *provisionAPIKeyRepoStub {
	return &provisionAPIKeyRepoStub{nextID: 1}
}

func (r *provisionAPIKeyRepoStub) SearchAPIKeys(_ context.Context, userID int64, keyword string, limit int) ([]APIKey, error) {
	var result []APIKey
	for _, k := range r.keys {
		if k.UserID == userID {
			result = append(result, k)
		}
	}
	return result, nil
}

func (r *provisionAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	key.ID = r.nextID
	r.nextID++
	r.keys = append(r.keys, *key)
	return nil
}

type provisionSettingRepoStub struct {
	SettingRepository
	values map[string]string
}

func newProvisionSettingRepoStub(values map[string]string) *provisionSettingRepoStub {
	return &provisionSettingRepoStub{values: values}
}

func (r *provisionSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

// newTestProvisionService creates a ProvisionService with stubs for testing
func newTestProvisionService() (*ProvisionService, *provisionUserRepoStub, *provisionAPIKeyRepoStub) {
	userRepo := newProvisionUserRepoStub()
	apiKeyRepo := newProvisionAPIKeyRepoStub()

	cfg := &config.Config{}
	cfg.Default.APIKeyPrefix = "sk-"
	cfg.Default.UserBalance = 1.0
	cfg.Default.UserConcurrency = 3

	settingRepo := newProvisionSettingRepoStub(map[string]string{
		SettingKeyAPIBaseURL:         "https://api.example.com",
		SettingKeyDefaultBalance:     "1.0",
		SettingKeyDefaultConcurrency: "3",
	})
	settingService := NewSettingService(settingRepo, cfg)

	svc := NewProvisionService(userRepo, apiKeyRepo, settingService, cfg)
	return svc, userRepo, apiKeyRepo
}

// --- tests ---

func TestProvisionService_NewUser(t *testing.T) {
	svc, _, _ := newTestProvisionService()
	ctx := context.Background()

	result, err := svc.Provision(ctx, ProvisionRequest{
		Email:  "newuser@example.com",
		Source: "openclaw-deploy",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.UserID)
	assert.NotEmpty(t, result.APIKey)
	assert.True(t, len(result.APIKey) > 16)
	assert.Equal(t, "https://api.example.com", result.BaseURL)
}

func TestProvisionService_ExistingUser(t *testing.T) {
	svc, userRepo, _ := newTestProvisionService()
	ctx := context.Background()

	// Pre-create user
	userRepo.users["existing@example.com"] = &User{
		ID:     42,
		Email:  "existing@example.com",
		Status: StatusActive,
	}
	userRepo.nextID = 43

	result, err := svc.Provision(ctx, ProvisionRequest{
		Email:  "existing@example.com",
		Source: "openclaw-deploy",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(42), result.UserID)
	assert.NotEmpty(t, result.APIKey)
}

func TestProvisionService_Idempotent(t *testing.T) {
	svc, userRepo, _ := newTestProvisionService()
	ctx := context.Background()

	userRepo.users["idempotent@example.com"] = &User{
		ID:     1,
		Email:  "idempotent@example.com",
		Status: StatusActive,
	}

	// First call
	result1, err := svc.Provision(ctx, ProvisionRequest{
		Email:  "idempotent@example.com",
		Source: "openclaw-deploy",
	})
	require.NoError(t, err)

	// Second call — should return same key
	result2, err := svc.Provision(ctx, ProvisionRequest{
		Email:  "idempotent@example.com",
		Source: "openclaw-deploy",
	})
	require.NoError(t, err)

	assert.Equal(t, result1.APIKey, result2.APIKey)
	assert.Equal(t, result1.UserID, result2.UserID)
}

func TestProvisionService_DifferentSource(t *testing.T) {
	svc, userRepo, _ := newTestProvisionService()
	ctx := context.Background()

	userRepo.users["test@example.com"] = &User{
		ID:     1,
		Email:  "test@example.com",
		Status: StatusActive,
	}

	// Provision with source A
	result1, err := svc.Provision(ctx, ProvisionRequest{
		Email:  "test@example.com",
		Source: "source-a",
	})
	require.NoError(t, err)

	// Provision with source B — should create a different key
	result2, err := svc.Provision(ctx, ProvisionRequest{
		Email:  "test@example.com",
		Source: "source-b",
	})
	require.NoError(t, err)

	assert.NotEqual(t, result1.APIKey, result2.APIKey)
	assert.Equal(t, result1.UserID, result2.UserID)
}

func TestProvisionService_GenerateKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.Default.APIKeyPrefix = "sk-"

	svc := &ProvisionService{cfg: cfg}

	key, err := svc.generateKey()
	require.NoError(t, err)
	assert.True(t, len(key) > 16)
	assert.Equal(t, "sk-", key[:3])
}
