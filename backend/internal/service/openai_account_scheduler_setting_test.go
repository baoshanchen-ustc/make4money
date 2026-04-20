//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayService_SelectAccountWithScheduler_DisabledSettingFallsBackToLoadAwareSelection(t *testing.T) {
	settingSvc := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyOpenAIAdvancedSchedulerEnabled: "false",
	}}, nil)

	svc := &OpenAIGatewayService{
		accountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{
				ID:          8801,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 2,
				Priority:    0,
			},
		}},
		settingService: settingSvc,
	}

	selection, decision, err := svc.SelectAccountWithScheduler(context.Background(), nil, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(8801), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)

	svc.RecordOpenAIAccountSwitch()
	require.Equal(t, int64(0), svc.SnapshotOpenAIAccountSchedulerMetrics().AccountSwitchTotal)
}
