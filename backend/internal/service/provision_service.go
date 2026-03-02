package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// ProvisionRequest represents a provision API request
type ProvisionRequest struct {
	Email  string `json:"email"`
	Source string `json:"source"`
}

// ProvisionResult represents the provision API response data
type ProvisionResult struct {
	APIKey string `json:"apiKey"`
	BaseURL string `json:"baseUrl"`
	UserID int64  `json:"userId"`
}

// ProvisionService handles user provisioning for service-to-service calls
type ProvisionService struct {
	userRepo       UserRepository
	apiKeyRepo     APIKeyRepository
	settingService *SettingService
	cfg            *config.Config
}

// NewProvisionService creates a new ProvisionService
func NewProvisionService(
	userRepo UserRepository,
	apiKeyRepo APIKeyRepository,
	settingService *SettingService,
	cfg *config.Config,
) *ProvisionService {
	return &ProvisionService{
		userRepo:       userRepo,
		apiKeyRepo:     apiKeyRepo,
		settingService: settingService,
		cfg:            cfg,
	}
}

// Provision creates or retrieves a user account and API key.
// Idempotent: calling twice with the same email returns the existing key.
func (s *ProvisionService) Provision(ctx context.Context, req ProvisionRequest) (*ProvisionResult, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		// User doesn't exist — create one with Free tier defaults
		defaultBalance := s.settingService.GetDefaultBalance(ctx)
		defaultConcurrency := s.settingService.GetDefaultConcurrency(ctx)

		user = &User{
			Email:       req.Email,
			Username:    req.Email,
			Role:        RoleUser,
			Balance:     defaultBalance,
			Concurrency: defaultConcurrency,
			Status:      StatusActive,
		}

		// Generate a random password (user won't need it — they use API key)
		randomPassword, err := generateRandomPassword()
		if err != nil {
			return nil, fmt.Errorf("generate random password: %w", err)
		}
		if err := user.SetPassword(randomPassword); err != nil {
			return nil, fmt.Errorf("set password: %w", err)
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}
	}

	// Look for an existing active API key named with the provision prefix
	apiKeyName := "provision-" + req.Source
	existingKey, err := s.findActiveAPIKey(ctx, user.ID, apiKeyName)
	if err != nil {
		return nil, fmt.Errorf("find existing api key: %w", err)
	}

	var keyString string
	if existingKey != nil {
		keyString = existingKey.Key
	} else {
		// Generate new API key
		keyString, err = s.generateKey()
		if err != nil {
			return nil, fmt.Errorf("generate api key: %w", err)
		}

		apiKey := &APIKey{
			UserID: user.ID,
			Key:    keyString,
			Name:   apiKeyName,
			Status: StatusActive,
		}
		if err := s.apiKeyRepo.Create(ctx, apiKey); err != nil {
			return nil, fmt.Errorf("create api key: %w", err)
		}
	}

	// Get the base URL from settings
	baseURL := s.settingService.GetAPIBaseURL(ctx)

	return &ProvisionResult{
		APIKey:  keyString,
		BaseURL: baseURL,
		UserID:  user.ID,
	}, nil
}

// findActiveAPIKey looks for an active API key with the given name for the user
func (s *ProvisionService) findActiveAPIKey(ctx context.Context, userID int64, name string) (*APIKey, error) {
	keys, err := s.apiKeyRepo.SearchAPIKeys(ctx, userID, name, 10)
	if err != nil {
		return nil, err
	}
	for i := range keys {
		if keys[i].Name == name && keys[i].IsActive() {
			return &keys[i], nil
		}
	}
	return nil, nil
}

// generateKey generates a random API key with the configured prefix
func (s *ProvisionService) generateKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	prefix := s.cfg.Default.APIKeyPrefix
	if prefix == "" {
		prefix = "sk-"
	}

	return prefix + hex.EncodeToString(bytes), nil
}

// generateRandomPassword generates a random password for provisioned users
func generateRandomPassword() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
