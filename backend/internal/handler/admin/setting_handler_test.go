package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMaskString(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		keepPrefix int
		keepSuffix int
		expected   string
	}{
		{
			name:       "empty string",
			input:      "",
			keepPrefix: 6,
			keepSuffix: 4,
			expected:   "",
		},
		{
			name:       "short string not masked",
			input:      "abc123",
			keepPrefix: 6,
			keepSuffix: 4,
			expected:   "abc123",
		},
		{
			name:       "exact length not masked",
			input:      "1234567890",
			keepPrefix: 6,
			keepSuffix: 4,
			expected:   "1234567890",
		},
		{
			name:       "typical appid mask",
			input:      "wx0b35f0a1b2fb07e",
			keepPrefix: 6,
			keepSuffix: 4,
			expected:   "wx0b35*******b07e",
		},
		{
			name:       "long string mask",
			input:      "abcdefghijklmnopqrstuvwxyz",
			keepPrefix: 4,
			keepSuffix: 4,
			expected:   "abcd******************wxyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskString(tt.input, tt.keepPrefix, tt.keepSuffix)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMaskAppID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty appid",
			input:    "",
			expected: "",
		},
		{
			name:     "short appid",
			input:    "wx12345",
			expected: "wx12345",
		},
		{
			name:     "typical appid",
			input:    "wx0b35f0a1b2fb07e",
			expected: "wx0b35*******b07e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAppID(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMaskMchID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty mchid",
			input:    "",
			expected: "",
		},
		{
			name:     "short mchid",
			input:    "12345",
			expected: "12345",
		},
		{
			name:     "typical mchid",
			input:    "1234567890",
			expected: "1234**7890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskMchID(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSettingHandler_BalanceLotSettingsScenarios(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		run  func(t *testing.T, ctx *settingHandlerTestContext)
	}{
		{
			name: "update and get",
			run: func(t *testing.T, ctx *settingHandlerTestContext) {
				putResp := ctx.putSettings(t, map[string]any{
					"balance_lot_expiry_days":              45,
					"balance_expiry_reminder_enabled":      true,
					"balance_expiry_reminder_advance_days": 7,
				})

				require.Equal(t, 0, putResp.Code)
				require.Equal(t, 45, putResp.Data.BalanceLotExpiryDays)
				require.True(t, putResp.Data.BalanceExpiryReminderEnabled)
				require.Equal(t, 7, putResp.Data.BalanceExpiryReminderAdvanceDays)

				getResp := ctx.getSettings(t)
				require.Equal(t, 0, getResp.Code)
				require.Equal(t, 45, getResp.Data.BalanceLotExpiryDays)
				require.True(t, getResp.Data.BalanceExpiryReminderEnabled)
				require.Equal(t, 7, getResp.Data.BalanceExpiryReminderAdvanceDays)

				require.Equal(t, "45", ctx.repo.values[service.SettingKeyBalanceLotExpiryDays])
				require.Equal(t, "true", ctx.repo.values[service.SettingKeyBalanceExpiryReminderEnabled])
				require.Equal(t, "7", ctx.repo.values[service.SettingKeyBalanceExpiryReminderAdvanceDays])
			},
		},
		{
			name: "boundary clamp to max/min",
			run: func(t *testing.T, ctx *settingHandlerTestContext) {
				putResp := ctx.putSettings(t, map[string]any{
					"balance_lot_expiry_days":              999,
					"balance_expiry_reminder_enabled":      true,
					"balance_expiry_reminder_advance_days": 0,
				})

				require.Equal(t, 0, putResp.Code)
				require.Equal(t, 365, putResp.Data.BalanceLotExpiryDays)
				require.True(t, putResp.Data.BalanceExpiryReminderEnabled)
				require.Equal(t, 1, putResp.Data.BalanceExpiryReminderAdvanceDays)

				require.Equal(t, "365", ctx.repo.values[service.SettingKeyBalanceLotExpiryDays])
				require.Equal(t, "true", ctx.repo.values[service.SettingKeyBalanceExpiryReminderEnabled])
				require.Equal(t, "1", ctx.repo.values[service.SettingKeyBalanceExpiryReminderAdvanceDays])
			},
		},
		{
			name: "boundary clamp to min/max",
			run: func(t *testing.T, ctx *settingHandlerTestContext) {
				putResp := ctx.putSettings(t, map[string]any{
					"balance_lot_expiry_days":              -5,
					"balance_expiry_reminder_enabled":      true,
					"balance_expiry_reminder_advance_days": 999,
				})

				require.Equal(t, 0, putResp.Code)
				require.Equal(t, 1, putResp.Data.BalanceLotExpiryDays)
				require.True(t, putResp.Data.BalanceExpiryReminderEnabled)
				require.Equal(t, 30, putResp.Data.BalanceExpiryReminderAdvanceDays)

				require.Equal(t, "1", ctx.repo.values[service.SettingKeyBalanceLotExpiryDays])
				require.Equal(t, "true", ctx.repo.values[service.SettingKeyBalanceExpiryReminderEnabled])
				require.Equal(t, "30", ctx.repo.values[service.SettingKeyBalanceExpiryReminderAdvanceDays])
			},
		},
		{
			name: "only balance lot update does not affect other defaults",
			run: func(t *testing.T, ctx *settingHandlerTestContext) {
				before := ctx.getSettings(t)
				require.Equal(t, 0, before.Code)
				require.Equal(t, "09:00", before.Data.UsageReportGlobalSchedule)
				require.Equal(t, "opted_in", before.Data.UsageReportTargetScope)
				require.Equal(t, 7, before.Data.AccountExpiryReminderAdvanceDays)

				_ = ctx.putSettings(t, map[string]any{
					"balance_lot_expiry_days":              20,
					"balance_expiry_reminder_enabled":      true,
					"balance_expiry_reminder_advance_days": 5,
				})

				after := ctx.getSettings(t)
				require.Equal(t, 0, after.Code)
				require.Equal(t, 20, after.Data.BalanceLotExpiryDays)
				require.True(t, after.Data.BalanceExpiryReminderEnabled)
				require.Equal(t, 5, after.Data.BalanceExpiryReminderAdvanceDays)
				require.Equal(t, before.Data.UsageReportGlobalSchedule, after.Data.UsageReportGlobalSchedule)
				require.Equal(t, before.Data.UsageReportTargetScope, after.Data.UsageReportTargetScope)
				require.Equal(t, before.Data.AccountExpiryReminderAdvanceDays, after.Data.AccountExpiryReminderAdvanceDays)
			},
		},
		{
			name: "empty payload preserves existing balance lot values",
			run: func(t *testing.T, ctx *settingHandlerTestContext) {
				firstResp := ctx.putSettings(t, map[string]any{
					"balance_lot_expiry_days":              33,
					"balance_expiry_reminder_enabled":      true,
					"balance_expiry_reminder_advance_days": 4,
				})
				require.Equal(t, 33, firstResp.Data.BalanceLotExpiryDays)
				require.True(t, firstResp.Data.BalanceExpiryReminderEnabled)
				require.Equal(t, 4, firstResp.Data.BalanceExpiryReminderAdvanceDays)

				emptyResp := ctx.putSettings(t, map[string]any{})
				require.Equal(t, 33, emptyResp.Data.BalanceLotExpiryDays)
				require.True(t, emptyResp.Data.BalanceExpiryReminderEnabled)
				require.Equal(t, 4, emptyResp.Data.BalanceExpiryReminderAdvanceDays)

				require.Equal(t, "33", ctx.repo.values[service.SettingKeyBalanceLotExpiryDays])
				require.Equal(t, "true", ctx.repo.values[service.SettingKeyBalanceExpiryReminderEnabled])
				require.Equal(t, "4", ctx.repo.values[service.SettingKeyBalanceExpiryReminderAdvanceDays])
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := newSettingHandlerTestContext(t)
			tt.run(t, ctx)
		})
	}
}
