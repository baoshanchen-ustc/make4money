//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type extremePerformanceSettingRepoStub struct {
	values map[string]string
}

func (s *extremePerformanceSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *extremePerformanceSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.values == nil {
		return "", ErrSettingNotFound
	}
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *extremePerformanceSettingRepoStub) Set(ctx context.Context, key, value string) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	s.values[key] = value
	return nil
}

func (s *extremePerformanceSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *extremePerformanceSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *extremePerformanceSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *extremePerformanceSettingRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestGetExtremePerformanceSettings_DefaultsWhenMissing(t *testing.T) {
	repo := &extremePerformanceSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	got, err := svc.GetExtremePerformanceSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultExtremePerformanceSettings(), got)
}

func TestSetAndGetExtremePerformanceSettings_RoundTrip(t *testing.T) {
	repo := &extremePerformanceSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	updated := false
	svc.SetOnUpdateCallback(func() {
		updated = true
	})

	input := &ExtremePerformanceSettings{
		Enabled: true,
		Admin: ExtremePerformanceAdminSettings{
			DisableAutoUsageFetch:      true,
			DisableAutoTodayStatsFetch: true,
			AllowManualUsageFetch:      false,
		},
		Pool: ExtremePerformancePoolSettings{
			PlatformLimits: ExtremePerformancePlatformLimits{
				OpenAI:    1500,
				Gemini:    250,
				Anthropic: 90,
			},
			RefillTriggerGap: 25,
			RefillBatchSize:  50,
			SelectionOrder:   "IMPORTED_FIRST",
		},
		AccountPolicy: ExtremePerformanceAccountPolicySettings{
			DeleteOnAnyUpstream401:               true,
			CooldownOn429Minutes:                 7,
			CooldownOn5xxMinutes:                 2,
			RemoveFromHotPoolOnOverload:          true,
			RemoveFromHotPoolOnTempUnschedulable: false,
		},
	}

	err := svc.SetExtremePerformanceSettings(context.Background(), input)
	require.NoError(t, err)
	require.True(t, updated)
	require.Contains(t, repo.values, SettingKeyExtremePerformanceSettings)

	got, err := svc.GetExtremePerformanceSettings(context.Background())
	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.Equal(t, 1500, got.Pool.PlatformLimits.OpenAI)
	require.Equal(t, 250, got.Pool.PlatformLimits.Gemini)
	require.Equal(t, 90, got.Pool.PlatformLimits.Anthropic)
	require.Equal(t, 25, got.Pool.RefillTriggerGap)
	require.Equal(t, 50, got.Pool.RefillBatchSize)
	require.Equal(t, ExtremePerformanceSelectionOrderImportedFirst, got.Pool.SelectionOrder)
	require.False(t, got.Admin.AllowManualUsageFetch)
	require.Equal(t, 7, got.AccountPolicy.CooldownOn429Minutes)
}

func TestValidateExtremePerformanceSettings_RejectsInvalidSelectionOrder(t *testing.T) {
	settings := DefaultExtremePerformanceSettings()
	settings.Pool.SelectionOrder = "random"

	err := ValidateExtremePerformanceSettings(settings)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pool.selection_order")
}
