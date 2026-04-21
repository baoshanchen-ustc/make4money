//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type passkeySettingsRepoStub struct {
	all             map[string]string
	setMultipleCall int
	setMultipleVals map[string]string
}

func (s *passkeySettingsRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *passkeySettingsRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if value, ok := s.all[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *passkeySettingsRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *passkeySettingsRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *passkeySettingsRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	s.setMultipleCall++
	s.setMultipleVals = make(map[string]string, len(settings))
	for k, v := range settings {
		s.setMultipleVals[k] = v
	}
	return nil
}

func (s *passkeySettingsRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.all))
	for k, v := range s.all {
		out[k] = v
	}
	return out, nil
}

func (s *passkeySettingsRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestSettingService_GetAllSettings_PasskeyRPConfig_DerivesFromFrontendURL(t *testing.T) {
	repo := &passkeySettingsRepoStub{
		all: map[string]string{
			SettingKeyFrontendURL: "https://frontend.example.com",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, "frontend.example.com", settings.PasskeyRPID)
	require.Equal(t, "Sub2API", settings.PasskeyRPName)
	require.Equal(t, []string{"https://frontend.example.com"}, settings.PasskeyAllowedOrigins)
}

func TestSettingService_GetAllSettings_PasskeyRPConfig_DerivedFromFrontendURL(t *testing.T) {
	repo := &passkeySettingsRepoStub{
		all: map[string]string{
			SettingKeyFrontendURL: "https://App.Example.com:7443/path",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, "app.example.com", settings.PasskeyRPID)
	require.Equal(t, "Sub2API", settings.PasskeyRPName)
	require.Equal(t, []string{"https://app.example.com:7443"}, settings.PasskeyAllowedOrigins)
}

func TestSettingService_GetAllSettings_PasskeyRPConfig_DerivedFromConfigFrontendURL(t *testing.T) {
	repo := &passkeySettingsRepoStub{all: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{Server: config.ServerConfig{FrontendURL: "https://config.example.com"}})

	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, "config.example.com", settings.PasskeyRPID)
	require.Equal(t, "Sub2API", settings.PasskeyRPName)
	require.Equal(t, []string{"https://config.example.com"}, settings.PasskeyAllowedOrigins)
}

func TestSettingService_GetAllSettings_PasskeyRPConfig_DoesNotInventDebugDefaults(t *testing.T) {
	repo := &passkeySettingsRepoStub{all: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{Server: config.ServerConfig{Mode: "debug"}})

	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, "", settings.PasskeyRPID)
	require.Equal(t, "Sub2API", settings.PasskeyRPName)
	require.Empty(t, settings.PasskeyAllowedOrigins)
}

func TestSettingService_GetAllSettings_PasskeyRPConfig_DoesNotUseLocalhostDefaultsInReleaseMode(t *testing.T) {
	repo := &passkeySettingsRepoStub{all: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{Server: config.ServerConfig{Mode: "release"}})

	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, "", settings.PasskeyRPID)
	require.Equal(t, "Sub2API", settings.PasskeyRPName)
	require.Empty(t, settings.PasskeyAllowedOrigins)
}

func TestSettingService_InitializeDefaultSettings_EnablesPasskeysByDefault(t *testing.T) {
	repo := &passkeySettingsRepoStub{all: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     12.5,
		},
	})

	err := svc.InitializeDefaultSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, repo.setMultipleCall)
	require.Equal(t, "true", repo.setMultipleVals[SettingKeyPasskeyEnabled])
	require.Equal(t, "true", repo.setMultipleVals[SettingKeyRegistrationEnabled])
}

func TestSettingService_GetPasskeyConfigValidation_DisabledPasskeysAreTreatedAsSafe(t *testing.T) {
	svc := NewSettingService(&passkeySettingsRepoStub{all: map[string]string{}}, &config.Config{})

	validation := svc.GetPasskeyConfigValidation(&SystemSettings{PasskeyEnabled: false})

	require.True(t, validation.Valid)
	require.Empty(t, validation.Error)
}

func TestSettingService_GetPasskeyConfigValidation_RejectsMissingResolvedRPID(t *testing.T) {
	svc := NewSettingService(&passkeySettingsRepoStub{all: map[string]string{}}, &config.Config{})

	validation := svc.GetPasskeyConfigValidation(&SystemSettings{
		PasskeyEnabled:        true,
		PasskeyRPName:         "Sub2API",
		PasskeyAllowedOrigins: []string{"https://app.example.com"},
	})

	require.False(t, validation.Valid)
	require.Contains(t, validation.Error, "Frontend URL")
}

func TestSettingService_GetPasskeyConfigValidation_RejectsInvalidOrigins(t *testing.T) {
	svc := NewSettingService(&passkeySettingsRepoStub{all: map[string]string{}}, &config.Config{})

	validation := svc.GetPasskeyConfigValidation(&SystemSettings{
		PasskeyEnabled:        true,
		PasskeyRPID:           "app.example.com",
		PasskeyRPName:         "Sub2API",
		PasskeyAllowedOrigins: []string{"http://app.example.com"},
	})

	require.False(t, validation.Valid)
	require.Contains(t, validation.Error, "http://app.example.com")
}
