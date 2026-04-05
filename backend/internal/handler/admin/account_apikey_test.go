package admin

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestParseRawAPIKeyImportLines(t *testing.T) {
	total, lines, results, err := parseRawAPIKeyImportLines(`
# comment
sk-proj-123
sk-ant-456,https://api.anthropic.com
AIzaSy789
bad-key
`)
	require.NoError(t, err)
	require.Equal(t, 4, total)
	require.Len(t, lines, 3)
	require.Len(t, results, 1)
	require.Equal(t, service.PlatformOpenAI, lines[0].Platform)
	require.Equal(t, service.PlatformAnthropic, lines[1].Platform)
	require.Equal(t, service.PlatformGemini, lines[2].Platform)
	require.Contains(t, results[0].Error, "could not detect platform")
}

func TestBuildAPIKeyIdentityUsesDefaultBaseURL(t *testing.T) {
	a := buildAPIKeyIdentity(service.PlatformOpenAI, "sk-proj-1", "")
	b := buildAPIKeyIdentity(service.PlatformOpenAI, "sk-proj-1", "https://api.openai.com/")
	require.Equal(t, a, b)
}

func TestDisableInvalidAPIKeyAccount_DisablesSchedulingWhenEnabled(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := &AccountHandler{adminService: adminSvc}
	account := &service.Account{
		ID:          42,
		Platform:    service.PlatformOpenAI,
		Schedulable: true,
	}

	err := handler.disableInvalidAPIKeyAccount(context.Background(), account, "invalid api key")
	require.NoError(t, err)
	require.Len(t, adminSvc.setAccountErrCalls, 1)
	require.Equal(t, int64(42), adminSvc.setAccountErrCalls[0].id)
	require.Len(t, adminSvc.setSchedulableCalls, 1)
	require.Equal(t, int64(42), adminSvc.setSchedulableCalls[0].id)
	require.False(t, adminSvc.setSchedulableCalls[0].schedulable)
}

func TestDisableInvalidAPIKeyAccount_SkipsSchedulingUpdateWhenAlreadyDisabled(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := &AccountHandler{adminService: adminSvc}
	account := &service.Account{
		ID:          43,
		Platform:    service.PlatformAnthropic,
		Schedulable: false,
	}

	err := handler.disableInvalidAPIKeyAccount(context.Background(), account, "invalid x-api-key")
	require.NoError(t, err)
	require.Len(t, adminSvc.setAccountErrCalls, 1)
	require.Equal(t, int64(43), adminSvc.setAccountErrCalls[0].id)
	require.Empty(t, adminSvc.setSchedulableCalls)
}

func TestRecoverValidAPIKeyAccount_ClearsErrorAndEnablesScheduling(t *testing.T) {
	// When health check confirms the key is valid via real chat completions,
	// status=error accounts should be fully recovered.
	adminSvc := newStubAdminService()
	handler := &AccountHandler{adminService: adminSvc}
	account := &service.Account{
		ID:          44,
		Status:      service.StatusError,
		Schedulable: false,
	}

	err := handler.recoverValidAPIKeyAccount(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, []int64{44}, adminSvc.clearedAccountErrIDs)
	require.Len(t, adminSvc.setSchedulableCalls, 1)
	require.Equal(t, int64(44), adminSvc.setSchedulableCalls[0].id)
	require.True(t, adminSvc.setSchedulableCalls[0].schedulable)
}

func TestRecoverValidAPIKeyAccount_EnablesSchedulingWithoutClearingActiveAccount(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := &AccountHandler{adminService: adminSvc}
	account := &service.Account{
		ID:          45,
		Status:      service.StatusActive,
		Schedulable: false,
	}

	err := handler.recoverValidAPIKeyAccount(context.Background(), account)
	require.NoError(t, err)
	require.Empty(t, adminSvc.clearedAccountErrIDs)
	require.Len(t, adminSvc.setSchedulableCalls, 1)
	require.Equal(t, int64(45), adminSvc.setSchedulableCalls[0].id)
	require.True(t, adminSvc.setSchedulableCalls[0].schedulable)
}
