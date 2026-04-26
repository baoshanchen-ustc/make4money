//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type accountRepoStubForCreateOpenAIPlan struct {
	accountRepoStub
	created *Account
}

func (s *accountRepoStubForCreateOpenAIPlan) Create(_ context.Context, account *Account) error {
	copiedCredentials := map[string]any{}
	for key, value := range account.Credentials {
		copiedCredentials[key] = value
	}
	copied := *account
	copied.Credentials = copiedCredentials
	s.created = &copied
	return nil
}

type accountRepoStubForOpenAIPlanCatalog struct {
	accountRepoStub
}

func (s *accountRepoStubForOpenAIPlanCatalog) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, string, int64, string, string) ([]Account, *pagination.PaginationResult, error) {
	return []Account{}, &pagination.PaginationResult{Total: 0}, nil
}

type settingRepoStubForOpenAIPlanCatalog struct {
	settingRepoStub
}

func (s *settingRepoStubForOpenAIPlanCatalog) Set(_ context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func TestAdminService_CreateAccount_OpenAIOAuthPlanModelMapping(t *testing.T) {
	t.Run("无手动 model_mapping 时按 plan_type 写入默认映射", func(t *testing.T) {
		repo := &accountRepoStubForCreateOpenAIPlan{}
		svc := &adminServiceImpl{accountRepo: repo}

		account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
			Name:                 "openai-plus",
			Platform:             PlatformOpenAI,
			Type:                 AccountTypeOAuth,
			Credentials:          map[string]any{"plan_type": "plus"},
			Concurrency:          1,
			Priority:             1,
			SkipDefaultGroupBind: true,
		})

		require.NoError(t, err)
		require.NotNil(t, account)
		require.NotNil(t, repo.created)
		mapping, ok := repo.created.Credentials["model_mapping"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "gpt-5.3-codex", mapping["gpt-5.3-codex"])
		require.Equal(t, "gpt-5.4", mapping["gpt-5.4"])
	})

	t.Run("已有 model_mapping 时不覆盖", func(t *testing.T) {
		repo := &accountRepoStubForCreateOpenAIPlan{}
		svc := &adminServiceImpl{accountRepo: repo}
		manualMapping := map[string]any{"custom": "upstream-custom"}

		account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
			Name:                 "openai-manual",
			Platform:             PlatformOpenAI,
			Type:                 AccountTypeOAuth,
			Credentials:          map[string]any{"plan_type": "pro", "model_mapping": manualMapping},
			Concurrency:          1,
			Priority:             1,
			SkipDefaultGroupBind: true,
		})

		require.NoError(t, err)
		require.NotNil(t, account)
		require.Equal(t, manualMapping, repo.created.Credentials["model_mapping"])
	})

	t.Run("缓存目录存在时优先使用缓存模型", func(t *testing.T) {
		repo := &accountRepoStubForCreateOpenAIPlan{}
		settingSvc := NewSettingService(&settingRepoStub{values: map[string]string{
			SettingKeyOpenAIPlanModelCatalog: `{"plans":{"team":{"models":["cached-team-model"],"status":"ok"}}}`,
		}}, &config.Config{})
		svc := &adminServiceImpl{accountRepo: repo, settingService: settingSvc}

		_, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
			Name:                 "openai-team",
			Platform:             PlatformOpenAI,
			Type:                 AccountTypeOAuth,
			Credentials:          map[string]any{"plan_type": "team"},
			Concurrency:          1,
			Priority:             1,
			SkipDefaultGroupBind: true,
		})

		require.NoError(t, err)
		require.Equal(t, map[string]any{"cached-team-model": "cached-team-model"}, repo.created.Credentials["model_mapping"])
	})
}

func TestAdminService_RefreshOpenAIPlanModelCatalog_NoAccountKeepsCachedModels(t *testing.T) {
	settingRepo := &settingRepoStubForOpenAIPlanCatalog{settingRepoStub: settingRepoStub{values: map[string]string{
		SettingKeyOpenAIPlanModelCatalog: `{"plans":{"plus":{"models":["cached-plus-model"],"status":"ok"}}}`,
	}}}
	svc := &adminServiceImpl{
		accountRepo:    &accountRepoStubForOpenAIPlanCatalog{},
		settingService: NewSettingService(settingRepo, &config.Config{}),
	}

	err := svc.RefreshOpenAIPlanModelCatalog(context.Background())

	require.NoError(t, err)
	var catalog openAIPlanModelCatalog
	require.NoError(t, json.Unmarshal([]byte(settingRepo.values[SettingKeyOpenAIPlanModelCatalog]), &catalog))
	require.Equal(t, []string{"cached-plus-model"}, catalog.Plans["plus"].Models)
	require.Equal(t, "no_account", catalog.Plans["plus"].Status)
}
