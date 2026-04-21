//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type settingPublicRepoStub struct {
	values        map[string]string
	requestedKeys []string
}

func (s *settingPublicRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingPublicRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *settingPublicRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingPublicRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	s.requestedKeys = append([]string(nil), keys...)
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *settingPublicRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *settingPublicRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *settingPublicRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestSettingService_GetPublicSettings_ExposesRegistrationEmailSuffixWhitelist(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyRegistrationEnabled:              "true",
			SettingKeyEmailVerifyEnabled:               "true",
			SettingKeyRegistrationEmailSuffixWhitelist: "[\"@EXAMPLE.com\",\" @foo.bar \",\"@invalid_domain\",\"\"]",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"@example.com", "@foo.bar"}, settings.RegistrationEmailSuffixWhitelist)
}

func TestSettingService_GetPublicSettings_ExposesTablePreferences(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyTableDefaultPageSize: "50",
			SettingKeyTablePageSizeOptions: "[20,50,100]",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 50, settings.TableDefaultPageSize)
	require.Equal(t, []int{20, 50, 100}, settings.TablePageSizeOptions)
}

func TestSettingService_GetPublicSettings_ExposesPasskeyEnabledOnly(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyPasskeyEnabled: "true",
			SettingKeyFrontendURL:    "https://example.com",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.PasskeyEnabled)
	require.Contains(t, repo.requestedKeys, SettingKeyPasskeyEnabled)
	require.Contains(t, repo.requestedKeys, SettingKeyFrontendURL)
}

func TestSettingService_GetPublicSettingsForInjection_DoesNotLeakPasskeyRPInternals(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyPasskeyEnabled: "true",
			SettingKeyFrontendURL:    "https://example.com",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	injected, err := svc.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)

	payload, err := json.Marshal(injected)
	require.NoError(t, err)

	obj := make(map[string]any)
	require.NoError(t, json.Unmarshal(payload, &obj))
	require.Equal(t, true, obj["passkey_enabled"])
	_, hasRPID := obj["passkey_rp_id"]
	require.False(t, hasRPID)
	_, hasRPName := obj["passkey_rp_name"]
	require.False(t, hasRPName)
	_, hasAllowedOrigins := obj["passkey_allowed_origins"]
	require.False(t, hasAllowedOrigins)
}

func TestSettingService_GetPublicSettings_HidesPasskeyWhenFrontendURLCannotBeDerived(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyPasskeyEnabled: "true",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.PasskeyEnabled)
}
