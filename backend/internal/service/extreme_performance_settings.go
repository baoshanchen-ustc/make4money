package service

import (
	"fmt"
	"strings"
)

const (
	// ExtremePerformanceSelectionOrderImportedFirst 表示热池补位按先导入优先（created_at ASC, id ASC）。
	ExtremePerformanceSelectionOrderImportedFirst = "imported_first"

	extremePerformanceMinPlatformLimit    = 1
	extremePerformanceMaxPlatformLimit    = 100000
	extremePerformanceMinRefillTriggerGap = 1
	extremePerformanceMaxRefillTriggerGap = 10000
	extremePerformanceMinRefillBatchSize  = 1
	extremePerformanceMaxRefillBatchSize  = 10000
	extremePerformanceMinCooldownMinutes  = 0
	extremePerformanceMaxCooldownMinutes  = 1440
)

// ExtremePerformanceSettings 极致性能模式设置。
type ExtremePerformanceSettings struct {
	Enabled       bool                                    `json:"enabled"`
	Admin         ExtremePerformanceAdminSettings         `json:"admin"`
	Pool          ExtremePerformancePoolSettings          `json:"pool"`
	AccountPolicy ExtremePerformanceAccountPolicySettings `json:"account_policy"`
}

// ExtremePerformanceAdminSettings 后台轻量化相关设置。
type ExtremePerformanceAdminSettings struct {
	DisableAutoUsageFetch      bool `json:"disable_auto_usage_fetch"`
	DisableAutoTodayStatsFetch bool `json:"disable_auto_today_stats_fetch"`
	AllowManualUsageFetch      bool `json:"allow_manual_usage_fetch"`
}

// ExtremePerformancePoolSettings 热池相关设置。
type ExtremePerformancePoolSettings struct {
	PlatformLimits   ExtremePerformancePlatformLimits `json:"platform_limits"`
	RefillTriggerGap int                              `json:"refill_trigger_gap"`
	RefillBatchSize  int                              `json:"refill_batch_size"`
	SelectionOrder   string                           `json:"selection_order"`
}

// ExtremePerformancePlatformLimits 各平台热池目标上限。
type ExtremePerformancePlatformLimits struct {
	OpenAI    int `json:"openai"`
	Gemini    int `json:"gemini"`
	Anthropic int `json:"anthropic"`
}

// ExtremePerformanceAccountPolicySettings 账号级策略设置。
type ExtremePerformanceAccountPolicySettings struct {
	DeleteOnAnyUpstream401               bool `json:"delete_on_any_upstream_401"`
	CooldownOn429Minutes                 int  `json:"cooldown_on_429_minutes"`
	CooldownOn5xxMinutes                 int  `json:"cooldown_on_5xx_minutes"`
	RemoveFromHotPoolOnOverload          bool `json:"remove_from_hot_pool_on_overload"`
	RemoveFromHotPoolOnTempUnschedulable bool `json:"remove_from_hot_pool_on_temp_unschedulable"`
}

// DefaultExtremePerformanceSettings 返回极致性能模式默认配置。
func DefaultExtremePerformanceSettings() *ExtremePerformanceSettings {
	return &ExtremePerformanceSettings{
		Enabled: false,
		Admin: ExtremePerformanceAdminSettings{
			DisableAutoUsageFetch:      true,
			DisableAutoTodayStatsFetch: true,
			AllowManualUsageFetch:      true,
		},
		Pool: ExtremePerformancePoolSettings{
			PlatformLimits: ExtremePerformancePlatformLimits{
				OpenAI:    2000,
				Gemini:    300,
				Anthropic: 100,
			},
			RefillTriggerGap: 50,
			RefillBatchSize:  100,
			SelectionOrder:   ExtremePerformanceSelectionOrderImportedFirst,
		},
		AccountPolicy: ExtremePerformanceAccountPolicySettings{
			DeleteOnAnyUpstream401:               true,
			CooldownOn429Minutes:                 5,
			CooldownOn5xxMinutes:                 1,
			RemoveFromHotPoolOnOverload:          true,
			RemoveFromHotPoolOnTempUnschedulable: true,
		},
	}
}

// ValidateExtremePerformanceSettings 校验极致性能模式设置。
func ValidateExtremePerformanceSettings(settings *ExtremePerformanceSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	if err := validateExtremePerformancePlatformLimit("pool.platform_limits.openai", settings.Pool.PlatformLimits.OpenAI); err != nil {
		return err
	}
	if err := validateExtremePerformancePlatformLimit("pool.platform_limits.gemini", settings.Pool.PlatformLimits.Gemini); err != nil {
		return err
	}
	if err := validateExtremePerformancePlatformLimit("pool.platform_limits.anthropic", settings.Pool.PlatformLimits.Anthropic); err != nil {
		return err
	}
	if settings.Pool.RefillTriggerGap < extremePerformanceMinRefillTriggerGap || settings.Pool.RefillTriggerGap > extremePerformanceMaxRefillTriggerGap {
		return fmt.Errorf("pool.refill_trigger_gap must be between %d-%d", extremePerformanceMinRefillTriggerGap, extremePerformanceMaxRefillTriggerGap)
	}
	if settings.Pool.RefillBatchSize < extremePerformanceMinRefillBatchSize || settings.Pool.RefillBatchSize > extremePerformanceMaxRefillBatchSize {
		return fmt.Errorf("pool.refill_batch_size must be between %d-%d", extremePerformanceMinRefillBatchSize, extremePerformanceMaxRefillBatchSize)
	}

	selectionOrder := strings.ToLower(strings.TrimSpace(settings.Pool.SelectionOrder))
	switch selectionOrder {
	case ExtremePerformanceSelectionOrderImportedFirst:
		// valid
	default:
		return fmt.Errorf("pool.selection_order must be %q", ExtremePerformanceSelectionOrderImportedFirst)
	}

	if err := validateExtremePerformanceCooldown("account_policy.cooldown_on_429_minutes", settings.AccountPolicy.CooldownOn429Minutes); err != nil {
		return err
	}
	if err := validateExtremePerformanceCooldown("account_policy.cooldown_on_5xx_minutes", settings.AccountPolicy.CooldownOn5xxMinutes); err != nil {
		return err
	}

	return nil
}

func validateExtremePerformancePlatformLimit(field string, value int) error {
	if value < extremePerformanceMinPlatformLimit || value > extremePerformanceMaxPlatformLimit {
		return fmt.Errorf("%s must be between %d-%d", field, extremePerformanceMinPlatformLimit, extremePerformanceMaxPlatformLimit)
	}
	return nil
}

func validateExtremePerformanceCooldown(field string, value int) error {
	if value < extremePerformanceMinCooldownMinutes || value > extremePerformanceMaxCooldownMinutes {
		return fmt.Errorf("%s must be between %d-%d", field, extremePerformanceMinCooldownMinutes, extremePerformanceMaxCooldownMinutes)
	}
	return nil
}

func normalizeExtremePerformanceSettings(settings *ExtremePerformanceSettings) *ExtremePerformanceSettings {
	defaults := DefaultExtremePerformanceSettings()
	if settings == nil {
		return defaults
	}

	normalized := *settings
	normalized.Pool.SelectionOrder = strings.ToLower(strings.TrimSpace(normalized.Pool.SelectionOrder))

	if normalized.Pool.PlatformLimits.OpenAI < extremePerformanceMinPlatformLimit || normalized.Pool.PlatformLimits.OpenAI > extremePerformanceMaxPlatformLimit {
		normalized.Pool.PlatformLimits.OpenAI = defaults.Pool.PlatformLimits.OpenAI
	}
	if normalized.Pool.PlatformLimits.Gemini < extremePerformanceMinPlatformLimit || normalized.Pool.PlatformLimits.Gemini > extremePerformanceMaxPlatformLimit {
		normalized.Pool.PlatformLimits.Gemini = defaults.Pool.PlatformLimits.Gemini
	}
	if normalized.Pool.PlatformLimits.Anthropic < extremePerformanceMinPlatformLimit || normalized.Pool.PlatformLimits.Anthropic > extremePerformanceMaxPlatformLimit {
		normalized.Pool.PlatformLimits.Anthropic = defaults.Pool.PlatformLimits.Anthropic
	}
	if normalized.Pool.RefillTriggerGap < extremePerformanceMinRefillTriggerGap || normalized.Pool.RefillTriggerGap > extremePerformanceMaxRefillTriggerGap {
		normalized.Pool.RefillTriggerGap = defaults.Pool.RefillTriggerGap
	}
	if normalized.Pool.RefillBatchSize < extremePerformanceMinRefillBatchSize || normalized.Pool.RefillBatchSize > extremePerformanceMaxRefillBatchSize {
		normalized.Pool.RefillBatchSize = defaults.Pool.RefillBatchSize
	}
	if normalized.Pool.SelectionOrder == "" {
		normalized.Pool.SelectionOrder = defaults.Pool.SelectionOrder
	}
	if normalized.Pool.SelectionOrder != ExtremePerformanceSelectionOrderImportedFirst {
		normalized.Pool.SelectionOrder = defaults.Pool.SelectionOrder
	}
	if normalized.AccountPolicy.CooldownOn429Minutes < extremePerformanceMinCooldownMinutes || normalized.AccountPolicy.CooldownOn429Minutes > extremePerformanceMaxCooldownMinutes {
		normalized.AccountPolicy.CooldownOn429Minutes = defaults.AccountPolicy.CooldownOn429Minutes
	}
	if normalized.AccountPolicy.CooldownOn5xxMinutes < extremePerformanceMinCooldownMinutes || normalized.AccountPolicy.CooldownOn5xxMinutes > extremePerformanceMaxCooldownMinutes {
		normalized.AccountPolicy.CooldownOn5xxMinutes = defaults.AccountPolicy.CooldownOn5xxMinutes
	}

	return &normalized
}
