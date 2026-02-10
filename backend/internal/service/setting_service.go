package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	// DefaultRechargeCacheTTL 充值配置缓存有效期
	DefaultRechargeCacheTTL = 60 * time.Second
)

// rechargeSettingsCache 充值配置缓存
type rechargeSettingsCache struct {
	settings  *RechargeSettings
	updatedAt time.Time
}

var (
	ErrRegistrationDisabled = infraerrors.Forbidden("REGISTRATION_DISABLED", "registration is currently disabled")
	ErrSettingNotFound      = infraerrors.NotFound("SETTING_NOT_FOUND", "setting not found")
)

type SettingRepository interface {
	Get(ctx context.Context, key string) (*Setting, error)
	GetValue(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetMultiple(ctx context.Context, keys []string) (map[string]string, error)
	SetMultiple(ctx context.Context, settings map[string]string) error
	GetAll(ctx context.Context) (map[string]string, error)
	Delete(ctx context.Context, key string) error
}

// SettingService 系统设置服务
type SettingService struct {
	settingRepo SettingRepository
	cfg         *config.Config
	onUpdate    func() // Callback when settings are updated (for cache invalidation)
	version     string // Application version

	// 充值配置缓存
	rechargeCache    *rechargeSettingsCache
	rechargeCacheMu  sync.RWMutex
	rechargeCacheTTL time.Duration

	// 后台任务控制
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewSettingService 创建系统设置服务实例
func NewSettingService(settingRepo SettingRepository, cfg *config.Config) *SettingService {
	s := &SettingService{
		settingRepo:      settingRepo,
		cfg:              cfg,
		rechargeCacheTTL: DefaultRechargeCacheTTL,
		stopCh:           make(chan struct{}),
	}

	// 启动后台缓存刷新任务
	s.startCacheRefreshLoop()

	return s
}

// startCacheRefreshLoop 启动后台缓存刷新循环
func (s *SettingService) startCacheRefreshLoop() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.rechargeCacheTTL)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 定期刷新缓存
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := s.refreshRechargeSettingsCache(ctx); err != nil {
					log.Printf("[SettingService] Failed to refresh recharge settings cache: %v", err)
				}
				cancel()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// refreshRechargeSettingsCache 刷新充值配置缓存
func (s *SettingService) refreshRechargeSettingsCache(ctx context.Context) error {
	settings, err := s.loadRechargeSettingsFromDB(ctx)
	if err != nil {
		return err
	}

	s.rechargeCacheMu.Lock()
	s.rechargeCache = &rechargeSettingsCache{
		settings:  settings,
		updatedAt: time.Now(),
	}
	s.rechargeCacheMu.Unlock()

	return nil
}

// loadRechargeSettingsFromDB 从数据库加载充值配置（内部方法）
// 配置优先级：数据库 > config.yaml > 代码默认值
func (s *SettingService) loadRechargeSettingsFromDB(ctx context.Context) (*RechargeSettings, error) {
	keys := []string{
		SettingKeyRechargeMinAmount,
		SettingKeyRechargeMaxAmount,
		SettingKeyRechargeDefaultAmounts,
		SettingKeyRechargeOrderExpireMinutes,
		SettingKeyRechargeExchangeRate,
	}

	dbSettings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get recharge settings from db: %w", err)
	}

	result := &RechargeSettings{}

	// 1. 最小充值金额：DB > config.yaml > 代码默认值
	result.MinAmount = s.getFloatWithFallback(
		dbSettings[SettingKeyRechargeMinAmount],
		s.cfg.Recharge.MinAmount,
		DefaultRechargeMinAmount,
	)

	// 2. 最大充值金额：DB > config.yaml > 代码默认值
	result.MaxAmount = s.getFloatWithFallback(
		dbSettings[SettingKeyRechargeMaxAmount],
		s.cfg.Recharge.MaxAmount,
		DefaultRechargeMaxAmount,
	)

	// 3. 默认金额选项：DB > config.yaml > 代码默认值
	result.DefaultAmounts = s.getAmountsWithFallback(
		dbSettings[SettingKeyRechargeDefaultAmounts],
		s.cfg.Recharge.DefaultAmounts,
		DefaultRechargeAmounts,
	)

	// 4. 订单过期时间：DB > config.yaml > 代码默认值
	result.OrderExpireMinutes = s.getIntWithFallback(
		dbSettings[SettingKeyRechargeOrderExpireMinutes],
		s.cfg.Recharge.OrderExpireMinutes,
		DefaultRechargeOrderExpireMinutes,
	)

	// 5. 汇率：DB > config.yaml > 代码默认值
	result.ExchangeRate = s.getFloatWithFallback(
		dbSettings[SettingKeyRechargeExchangeRate],
		s.cfg.Recharge.ExchangeRate,
		DefaultRechargeExchangeRate,
	)

	return result, nil
}

// getFloatWithFallback 获取浮点数配置，支持三层回退
func (s *SettingService) getFloatWithFallback(dbValue string, configValue float64, defaultValue float64) float64 {
	// 尝试从数据库读取
	if dbValue != "" {
		if v, err := strconv.ParseFloat(dbValue, 64); err == nil && v > 0 {
			return v
		}
	}

	// 尝试从 config.yaml 读取
	if configValue > 0 {
		return configValue
	}

	// 使用代码默认值
	return defaultValue
}

// getIntWithFallback 获取整数配置，支持三层回退
func (s *SettingService) getIntWithFallback(dbValue string, configValue int, defaultValue int) int {
	// 尝试从数据库读取
	if dbValue != "" {
		if v, err := strconv.Atoi(dbValue); err == nil && v > 0 {
			return v
		}
	}

	// 尝试从 config.yaml 读取
	if configValue > 0 {
		return configValue
	}

	// 使用代码默认值
	return defaultValue
}

// getAmountsWithFallback 获取金额数组配置，支持三层回退
func (s *SettingService) getAmountsWithFallback(dbValue string, configValue []float64, defaultValue []float64) []float64 {
	// 尝试从数据库读取
	if dbValue != "" {
		var amounts []float64
		if err := json.Unmarshal([]byte(dbValue), &amounts); err == nil && len(amounts) > 0 {
			return amounts
		}
	}

	// 尝试从 config.yaml 读取
	if len(configValue) > 0 {
		return append([]float64{}, configValue...)
	}

	// 使用代码默认值
	return append([]float64{}, defaultValue...)
}

// GetRechargeConfigSources 获取各配置项的实际来源（用于调试）
// 返回 map[字段名]来源，来源可能是 "database", "config.yaml", "default"
func (s *SettingService) GetRechargeConfigSources(ctx context.Context) (map[string]string, error) {
	keys := []string{
		SettingKeyRechargeMinAmount,
		SettingKeyRechargeMaxAmount,
		SettingKeyRechargeDefaultAmounts,
		SettingKeyRechargeOrderExpireMinutes,
		SettingKeyRechargeExchangeRate,
	}

	dbSettings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, err
	}

	sources := make(map[string]string)

	// 判断各配置项的来源
	if dbSettings[SettingKeyRechargeMinAmount] != "" {
		sources["min_amount"] = "database"
	} else if s.cfg.Recharge.MinAmount > 0 {
		sources["min_amount"] = "config.yaml"
	} else {
		sources["min_amount"] = "default"
	}

	if dbSettings[SettingKeyRechargeMaxAmount] != "" {
		sources["max_amount"] = "database"
	} else if s.cfg.Recharge.MaxAmount > 0 {
		sources["max_amount"] = "config.yaml"
	} else {
		sources["max_amount"] = "default"
	}

	if dbSettings[SettingKeyRechargeDefaultAmounts] != "" {
		sources["default_amounts"] = "database"
	} else if len(s.cfg.Recharge.DefaultAmounts) > 0 {
		sources["default_amounts"] = "config.yaml"
	} else {
		sources["default_amounts"] = "default"
	}

	if dbSettings[SettingKeyRechargeOrderExpireMinutes] != "" {
		sources["order_expire_minutes"] = "database"
	} else if s.cfg.Recharge.OrderExpireMinutes > 0 {
		sources["order_expire_minutes"] = "config.yaml"
	} else {
		sources["order_expire_minutes"] = "default"
	}

	if dbSettings[SettingKeyRechargeExchangeRate] != "" {
		sources["exchange_rate"] = "database"
	} else if s.cfg.Recharge.ExchangeRate > 0 {
		sources["exchange_rate"] = "config.yaml"
	} else {
		sources["exchange_rate"] = "default"
	}

	return sources, nil
}

// Stop 停止后台任务
func (s *SettingService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// GetAllSettings 获取所有系统设置
func (s *SettingService) GetAllSettings(ctx context.Context) (*SystemSettings, error) {
	settings, err := s.settingRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all settings: %w", err)
	}

	return s.parseSettings(settings), nil
}

// GetPublicSettings 获取公开设置（无需登录）
func (s *SettingService) GetPublicSettings(ctx context.Context) (*PublicSettings, error) {
	keys := []string{
		SettingKeyRegistrationEnabled,
		SettingKeyEmailVerifyEnabled,
		SettingKeyPromoCodeEnabled,
		SettingKeyPasswordResetEnabled,
		SettingKeyInvitationCodeEnabled,
		SettingKeyTotpEnabled,
		SettingKeyTurnstileEnabled,
		SettingKeyTurnstileSiteKey,
		SettingKeySiteName,
		SettingKeySiteLogo,
		SettingKeySiteLogoDark,
		SettingKeySiteSubtitle,
		SettingKeyAPIBaseURL,
		SettingKeyContactInfo,
		SettingKeyContactQRCodeWechat,
		SettingKeyContactQRCodeGroup,
		SettingKeyDocURL,
		SettingKeyHomeContent,
		SettingKeyHideCcsImportButton,
		SettingKeyPurchaseSubscriptionEnabled,
		SettingKeyPurchaseSubscriptionURL,
		SettingKeyLinuxDoConnectEnabled,
		SettingKeyWeChatAuthEnabled,
		SettingKeyWeChatAccountType,
		SettingKeyWeChatAccountQRCodeURL,
		SettingKeyWeChatAccountQRCodeData,
		SettingKeyForceEmailBind,
		SettingKeyInstallGuideVideos,
		SettingKeyHomeTestimonials,
	}

	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get public settings: %w", err)
	}

	linuxDoEnabled := false
	if raw, ok := settings[SettingKeyLinuxDoConnectEnabled]; ok {
		linuxDoEnabled = raw == "true"
	} else {
		linuxDoEnabled = s.cfg != nil && s.cfg.LinuxDo.Enabled
	}

	// Password reset requires email verification to be enabled
	emailVerifyEnabled := settings[SettingKeyEmailVerifyEnabled] == "true"
	passwordResetEnabled := emailVerifyEnabled && settings[SettingKeyPasswordResetEnabled] == "true"

	return &PublicSettings{
		RegistrationEnabled:         settings[SettingKeyRegistrationEnabled] == "true",
		EmailVerifyEnabled:          emailVerifyEnabled,
		PromoCodeEnabled:            settings[SettingKeyPromoCodeEnabled] != "false", // 默认启用
		PasswordResetEnabled:        passwordResetEnabled,
		InvitationCodeEnabled:       settings[SettingKeyInvitationCodeEnabled] == "true",
		TotpEnabled:                 settings[SettingKeyTotpEnabled] == "true",
		TurnstileEnabled:            settings[SettingKeyTurnstileEnabled] == "true",
		TurnstileSiteKey:            settings[SettingKeyTurnstileSiteKey],
		SiteName:                    s.getStringOrDefault(settings, SettingKeySiteName, "Code80"),
		SiteLogo:                    settings[SettingKeySiteLogo],
		SiteLogoDark:                settings[SettingKeySiteLogoDark],
		SiteSubtitle:                s.getStringOrDefault(settings, SettingKeySiteSubtitle, "Subscription to API Conversion Platform"),
		APIBaseURL:                  settings[SettingKeyAPIBaseURL],
		ContactInfo:                 settings[SettingKeyContactInfo],
		ContactQRCodeWechat:         settings[SettingKeyContactQRCodeWechat],
		ContactQRCodeGroup:          settings[SettingKeyContactQRCodeGroup],
		DocURL:                      settings[SettingKeyDocURL],
		HomeContent:                 settings[SettingKeyHomeContent],
		HideCcsImportButton:         settings[SettingKeyHideCcsImportButton] == "true",
		PurchaseSubscriptionEnabled: settings[SettingKeyPurchaseSubscriptionEnabled] == "true",
		PurchaseSubscriptionURL:     strings.TrimSpace(settings[SettingKeyPurchaseSubscriptionURL]),
		LinuxDoOAuthEnabled:         linuxDoEnabled,
		WeChatAuthEnabled:           settings[SettingKeyWeChatAuthEnabled] == "true",
		WeChatAccountType:           s.getStringOrDefault(settings, SettingKeyWeChatAccountType, WeChatAccountTypeSubscription),
		WeChatAccountQRCodeURL:      settings[SettingKeyWeChatAccountQRCodeURL],
		WeChatAccountQRCodeData:     settings[SettingKeyWeChatAccountQRCodeData],
		ForceEmailBind:              settings[SettingKeyForceEmailBind] == "true",
		InstallGuideVideos:          settings[SettingKeyInstallGuideVideos],
		HomeTestimonials:            settings[SettingKeyHomeTestimonials],
	}, nil
}

// SetOnUpdateCallback sets a callback function to be called when settings are updated
// This is used for cache invalidation (e.g., HTML cache in frontend server)
func (s *SettingService) SetOnUpdateCallback(callback func()) {
	s.onUpdate = callback
}

// SetVersion sets the application version for injection into public settings
func (s *SettingService) SetVersion(version string) {
	s.version = version
}

// GetPublicSettingsForInjection returns public settings in a format suitable for HTML injection
// This implements the web.PublicSettingsProvider interface
func (s *SettingService) GetPublicSettingsForInjection(ctx context.Context) (any, error) {
	settings, err := s.GetPublicSettings(ctx)
	if err != nil {
		return nil, err
	}

	// Return a struct that matches the frontend's expected format
	return &struct {
		RegistrationEnabled         bool   `json:"registration_enabled"`
		EmailVerifyEnabled          bool   `json:"email_verify_enabled"`
		PromoCodeEnabled            bool   `json:"promo_code_enabled"`
		PasswordResetEnabled        bool   `json:"password_reset_enabled"`
		InvitationCodeEnabled       bool   `json:"invitation_code_enabled"`
		TotpEnabled                 bool   `json:"totp_enabled"`
		TurnstileEnabled            bool   `json:"turnstile_enabled"`
		TurnstileSiteKey            string `json:"turnstile_site_key,omitempty"`
		SiteName                    string `json:"site_name"`
		SiteLogo                    string `json:"site_logo,omitempty"`
		SiteLogoDark                string `json:"site_logo_dark,omitempty"`
		SiteSubtitle                string `json:"site_subtitle,omitempty"`
		APIBaseURL                  string `json:"api_base_url,omitempty"`
		ContactInfo                 string `json:"contact_info,omitempty"`
		ContactQRCodeWechat         string `json:"contact_qrcode_wechat,omitempty"`
		ContactQRCodeGroup          string `json:"contact_qrcode_group,omitempty"`
		DocURL                      string `json:"doc_url,omitempty"`
		HomeContent                 string `json:"home_content,omitempty"`
		HideCcsImportButton         bool   `json:"hide_ccs_import_button"`
		PurchaseSubscriptionEnabled bool   `json:"purchase_subscription_enabled"`
		PurchaseSubscriptionURL     string `json:"purchase_subscription_url,omitempty"`
		LinuxDoOAuthEnabled         bool   `json:"linuxdo_oauth_enabled"`
		WeChatAuthEnabled           bool   `json:"wechat_auth_enabled"`
		WeChatAccountType           string `json:"wechat_account_type,omitempty"`
		WeChatAccountQRCodeURL      string `json:"wechat_account_qrcode_url,omitempty"`
		WeChatAccountQRCodeData     string `json:"wechat_account_qrcode_data,omitempty"`
		ForceEmailBind              bool   `json:"force_email_bind"`
		Version                     string `json:"version,omitempty"`
		InstallGuideVideos          string `json:"install_guide_videos,omitempty"`
		HomeTestimonials            string `json:"home_testimonials,omitempty"`
	}{
		RegistrationEnabled:         settings.RegistrationEnabled,
		EmailVerifyEnabled:          settings.EmailVerifyEnabled,
		PromoCodeEnabled:            settings.PromoCodeEnabled,
		PasswordResetEnabled:        settings.PasswordResetEnabled,
		InvitationCodeEnabled:       settings.InvitationCodeEnabled,
		TotpEnabled:                 settings.TotpEnabled,
		TurnstileEnabled:            settings.TurnstileEnabled,
		TurnstileSiteKey:            settings.TurnstileSiteKey,
		SiteName:                    settings.SiteName,
		SiteLogo:                    settings.SiteLogo,
		SiteLogoDark:                settings.SiteLogoDark,
		SiteSubtitle:                settings.SiteSubtitle,
		APIBaseURL:                  settings.APIBaseURL,
		ContactInfo:                 settings.ContactInfo,
		ContactQRCodeWechat:         settings.ContactQRCodeWechat,
		ContactQRCodeGroup:          settings.ContactQRCodeGroup,
		DocURL:                      settings.DocURL,
		HomeContent:                 settings.HomeContent,
		HideCcsImportButton:         settings.HideCcsImportButton,
		PurchaseSubscriptionEnabled: settings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:     settings.PurchaseSubscriptionURL,
		LinuxDoOAuthEnabled:         settings.LinuxDoOAuthEnabled,
		WeChatAuthEnabled:           settings.WeChatAuthEnabled,
		WeChatAccountType:           settings.WeChatAccountType,
		WeChatAccountQRCodeURL:      settings.WeChatAccountQRCodeURL,
		WeChatAccountQRCodeData:     settings.WeChatAccountQRCodeData,
		ForceEmailBind:              settings.ForceEmailBind,
		Version:                     s.version,
		InstallGuideVideos:          settings.InstallGuideVideos,
		HomeTestimonials:            settings.HomeTestimonials,
	}, nil
}

// UpdateSettings 更新系统设置
func (s *SettingService) UpdateSettings(ctx context.Context, settings *SystemSettings) error {
	updates := make(map[string]string)

	// 注册设置
	updates[SettingKeyRegistrationEnabled] = strconv.FormatBool(settings.RegistrationEnabled)
	updates[SettingKeyEmailVerifyEnabled] = strconv.FormatBool(settings.EmailVerifyEnabled)
	updates[SettingKeyPromoCodeEnabled] = strconv.FormatBool(settings.PromoCodeEnabled)
	updates[SettingKeyPasswordResetEnabled] = strconv.FormatBool(settings.PasswordResetEnabled)
	updates[SettingKeyInvitationCodeEnabled] = strconv.FormatBool(settings.InvitationCodeEnabled)
	updates[SettingKeyTotpEnabled] = strconv.FormatBool(settings.TotpEnabled)

	// 邮件服务设置（只有非空才更新密码）
	updates[SettingKeySMTPHost] = settings.SMTPHost
	updates[SettingKeySMTPPort] = strconv.Itoa(settings.SMTPPort)
	updates[SettingKeySMTPUsername] = settings.SMTPUsername
	if settings.SMTPPassword != "" {
		updates[SettingKeySMTPPassword] = settings.SMTPPassword
	}
	updates[SettingKeySMTPFrom] = settings.SMTPFrom
	updates[SettingKeySMTPFromName] = settings.SMTPFromName
	updates[SettingKeySMTPUseTLS] = strconv.FormatBool(settings.SMTPUseTLS)

	// Cloudflare Turnstile 设置（只有非空才更新密钥）
	updates[SettingKeyTurnstileEnabled] = strconv.FormatBool(settings.TurnstileEnabled)
	updates[SettingKeyTurnstileSiteKey] = settings.TurnstileSiteKey
	if settings.TurnstileSecretKey != "" {
		updates[SettingKeyTurnstileSecretKey] = settings.TurnstileSecretKey
	}

	// LinuxDo Connect OAuth 登录
	updates[SettingKeyLinuxDoConnectEnabled] = strconv.FormatBool(settings.LinuxDoConnectEnabled)
	updates[SettingKeyLinuxDoConnectClientID] = settings.LinuxDoConnectClientID
	updates[SettingKeyLinuxDoConnectRedirectURL] = settings.LinuxDoConnectRedirectURL
	if settings.LinuxDoConnectClientSecret != "" {
		updates[SettingKeyLinuxDoConnectClientSecret] = settings.LinuxDoConnectClientSecret
	}

	// 强制绑定邮箱
	updates[SettingKeyForceEmailBind] = strconv.FormatBool(settings.ForceEmailBind)

	// 微信公众号验证码登录
	updates[SettingKeyWeChatAuthEnabled] = strconv.FormatBool(settings.WeChatAuthEnabled)
	updates[SettingKeyWeChatAccountType] = settings.WeChatAccountType
	updates[SettingKeyWeChatServerAddress] = settings.WeChatServerAddress
	if settings.WeChatServerToken != "" {
		updates[SettingKeyWeChatServerToken] = settings.WeChatServerToken
	}
	updates[SettingKeyWeChatAccountQRCodeURL] = settings.WeChatAccountQRCodeURL
	updates[SettingKeyWeChatAccountQRCodeData] = settings.WeChatAccountQRCodeData
	updates[SettingKeyWeChatAppID] = settings.WeChatAppID
	if settings.WeChatAppSecret != "" {
		updates[SettingKeyWeChatAppSecret] = settings.WeChatAppSecret
	}

	// OEM设置
	updates[SettingKeySiteName] = settings.SiteName
	updates[SettingKeySiteLogo] = settings.SiteLogo
	updates[SettingKeySiteLogoDark] = settings.SiteLogoDark
	updates[SettingKeySiteSubtitle] = settings.SiteSubtitle
	updates[SettingKeyAPIBaseURL] = settings.APIBaseURL
	updates[SettingKeyContactInfo] = settings.ContactInfo
	updates[SettingKeyContactQRCodeWechat] = settings.ContactQRCodeWechat
	updates[SettingKeyContactQRCodeGroup] = settings.ContactQRCodeGroup
	updates[SettingKeyDocURL] = settings.DocURL
	updates[SettingKeyHomeContent] = settings.HomeContent
	updates[SettingKeyHideCcsImportButton] = strconv.FormatBool(settings.HideCcsImportButton)
	updates[SettingKeyPurchaseSubscriptionEnabled] = strconv.FormatBool(settings.PurchaseSubscriptionEnabled)
	updates[SettingKeyPurchaseSubscriptionURL] = strings.TrimSpace(settings.PurchaseSubscriptionURL)

	// 默认配置
	updates[SettingKeyDefaultConcurrency] = strconv.Itoa(settings.DefaultConcurrency)
	updates[SettingKeyDefaultBalance] = strconv.FormatFloat(settings.DefaultBalance, 'f', 8, 64)

	// Model fallback configuration
	updates[SettingKeyEnableModelFallback] = strconv.FormatBool(settings.EnableModelFallback)
	updates[SettingKeyFallbackModelAnthropic] = settings.FallbackModelAnthropic
	updates[SettingKeyFallbackModelOpenAI] = settings.FallbackModelOpenAI
	updates[SettingKeyFallbackModelGemini] = settings.FallbackModelGemini
	updates[SettingKeyFallbackModelAntigravity] = settings.FallbackModelAntigravity

	// Identity patch configuration (Claude -> Gemini)
	updates[SettingKeyEnableIdentityPatch] = strconv.FormatBool(settings.EnableIdentityPatch)
	updates[SettingKeyIdentityPatchPrompt] = settings.IdentityPatchPrompt

	// Ops monitoring (vNext)
	updates[SettingKeyOpsMonitoringEnabled] = strconv.FormatBool(settings.OpsMonitoringEnabled)
	updates[SettingKeyOpsRealtimeMonitoringEnabled] = strconv.FormatBool(settings.OpsRealtimeMonitoringEnabled)
	updates[SettingKeyOpsQueryModeDefault] = string(ParseOpsQueryMode(settings.OpsQueryModeDefault))
	if settings.OpsMetricsIntervalSeconds > 0 {
		updates[SettingKeyOpsMetricsIntervalSeconds] = strconv.Itoa(settings.OpsMetricsIntervalSeconds)
	}

	// Usage report settings
	updates[SettingKeyUsageReportGlobalEnabled] = strconv.FormatBool(settings.UsageReportGlobalEnabled)
	updates[SettingKeyUsageReportTargetScope] = settings.UsageReportTargetScope
	updates[SettingKeyUsageReportGlobalSchedule] = settings.UsageReportGlobalSchedule

	// Homepage & Install Guide
	updates[SettingKeyInstallGuideVideos] = settings.InstallGuideVideos
	updates[SettingKeyHomeTestimonials] = settings.HomeTestimonials

	// Account expiry reminder
	updates[SettingKeyAccountExpiryReminderEmail] = strings.TrimSpace(settings.AccountExpiryReminderEmail)
	updates[SettingKeyAccountExpiryReminderAdvanceDays] = strconv.Itoa(settings.AccountExpiryReminderAdvanceDays)

	// Balance lot expiry
	updates[SettingKeyBalanceLotExpiryDays] = strconv.Itoa(settings.BalanceLotExpiryDays)
	updates[SettingKeyBalanceExpiryReminderEnabled] = strconv.FormatBool(settings.BalanceExpiryReminderEnabled)
	updates[SettingKeyBalanceExpiryReminderAdvanceDays] = strconv.Itoa(settings.BalanceExpiryReminderAdvanceDays)

	err := s.settingRepo.SetMultiple(ctx, updates)
	if err == nil && s.onUpdate != nil {
		s.onUpdate() // Invalidate cache after settings update
	}
	return err
}

// IsRegistrationEnabled 检查是否开放注册
func (s *SettingService) IsRegistrationEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyRegistrationEnabled)
	if err != nil {
		// 安全默认：如果设置不存在或查询出错，默认关闭注册
		return false
	}
	return value == "true"
}

// IsEmailVerifyEnabled 检查是否开启邮件验证
func (s *SettingService) IsEmailVerifyEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyEmailVerifyEnabled)
	if err != nil {
		return false
	}
	return value == "true"
}

// IsPromoCodeEnabled 检查是否启用优惠码功能
func (s *SettingService) IsPromoCodeEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyPromoCodeEnabled)
	if err != nil {
		return true // 默认启用
	}
	return value != "false"
}

// IsInvitationCodeEnabled 检查是否启用邀请码注册功能
func (s *SettingService) IsInvitationCodeEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyInvitationCodeEnabled)
	if err != nil {
		return false // 默认关闭
	}
	return value == "true"
}

// IsPasswordResetEnabled 检查是否启用密码重置功能
// 要求：必须同时开启邮件验证
func (s *SettingService) IsPasswordResetEnabled(ctx context.Context) bool {
	// Password reset requires email verification to be enabled
	if !s.IsEmailVerifyEnabled(ctx) {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyPasswordResetEnabled)
	if err != nil {
		return false // 默认关闭
	}
	return value == "true"
}

// IsTotpEnabled 检查是否启用 TOTP 双因素认证功能
func (s *SettingService) IsTotpEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyTotpEnabled)
	if err != nil {
		return false // 默认关闭
	}
	return value == "true"
}

// IsTotpEncryptionKeyConfigured 检查 TOTP 加密密钥是否已手动配置
// 只有手动配置了密钥才允许在管理后台启用 TOTP 功能
func (s *SettingService) IsTotpEncryptionKeyConfigured() bool {
	return s.cfg.Totp.EncryptionKeyConfigured
}

// GetSettingValue 获取指定 key 的设置值
func (s *SettingService) GetSettingValue(ctx context.Context, key string) (string, error) {
	return s.settingRepo.GetValue(ctx, key)
}

// GetSiteName 获取网站名称
func (s *SettingService) GetSiteName(ctx context.Context) string {
	value, err := s.settingRepo.GetValue(ctx, SettingKeySiteName)
	if err != nil || value == "" {
		return "Code80"
	}
	return value
}

// GetDefaultConcurrency 获取默认并发量
func (s *SettingService) GetDefaultConcurrency(ctx context.Context) int {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyDefaultConcurrency)
	if err != nil {
		return s.cfg.Default.UserConcurrency
	}
	if v, err := strconv.Atoi(value); err == nil && v > 0 {
		return v
	}
	return s.cfg.Default.UserConcurrency
}

// GetDefaultBalance 获取默认余额
func (s *SettingService) GetDefaultBalance(ctx context.Context) float64 {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyDefaultBalance)
	if err != nil {
		return s.cfg.Default.UserBalance
	}
	if v, err := strconv.ParseFloat(value, 64); err == nil && v >= 0 {
		return v
	}
	return s.cfg.Default.UserBalance
}

// InitializeDefaultSettings 初始化默认设置
func (s *SettingService) InitializeDefaultSettings(ctx context.Context) error {
	// 检查是否已有设置
	_, err := s.settingRepo.GetValue(ctx, SettingKeyRegistrationEnabled)
	if err == nil {
		// 已有设置，不需要初始化
		return nil
	}
	if !errors.Is(err, ErrSettingNotFound) {
		return fmt.Errorf("check existing settings: %w", err)
	}

	// 初始化默认设置
	defaults := map[string]string{
		SettingKeyRegistrationEnabled:         "true",
		SettingKeyEmailVerifyEnabled:          "false",
		SettingKeyPromoCodeEnabled:            "true", // 默认启用优惠码功能
		SettingKeySiteName:                    "Code80",
		SettingKeySiteLogo:                    "",
		SettingKeyPurchaseSubscriptionEnabled: "false",
		SettingKeyPurchaseSubscriptionURL:     "",
		SettingKeyDefaultConcurrency:          strconv.Itoa(s.cfg.Default.UserConcurrency),
		SettingKeyDefaultBalance:              strconv.FormatFloat(s.cfg.Default.UserBalance, 'f', 8, 64),
		SettingKeySMTPPort:                    "587",
		SettingKeySMTPUseTLS:                  "false",
		// Model fallback defaults
		SettingKeyEnableModelFallback:      "false",
		SettingKeyFallbackModelAnthropic:   "claude-3-5-sonnet-20241022",
		SettingKeyFallbackModelOpenAI:      "gpt-4o",
		SettingKeyFallbackModelGemini:      "gemini-2.5-pro",
		SettingKeyFallbackModelAntigravity: "gemini-2.5-pro",
		// Identity patch defaults
		SettingKeyEnableIdentityPatch: "true",
		SettingKeyIdentityPatchPrompt: "",

		// Ops monitoring defaults (vNext)
		SettingKeyOpsMonitoringEnabled:         "true",
		SettingKeyOpsRealtimeMonitoringEnabled: "true",
		SettingKeyOpsQueryModeDefault:          "auto",
		SettingKeyOpsMetricsIntervalSeconds:    "60",
	}

	return s.settingRepo.SetMultiple(ctx, defaults)
}

// parseSettings 解析设置到结构体
func (s *SettingService) parseSettings(settings map[string]string) *SystemSettings {
	emailVerifyEnabled := settings[SettingKeyEmailVerifyEnabled] == "true"
	result := &SystemSettings{
		RegistrationEnabled:          settings[SettingKeyRegistrationEnabled] == "true",
		EmailVerifyEnabled:           emailVerifyEnabled,
		PromoCodeEnabled:             settings[SettingKeyPromoCodeEnabled] != "false", // 默认启用
		PasswordResetEnabled:         emailVerifyEnabled && settings[SettingKeyPasswordResetEnabled] == "true",
		InvitationCodeEnabled:        settings[SettingKeyInvitationCodeEnabled] == "true",
		TotpEnabled:                  settings[SettingKeyTotpEnabled] == "true",
		SMTPHost:                     settings[SettingKeySMTPHost],
		SMTPUsername:                 settings[SettingKeySMTPUsername],
		SMTPFrom:                     settings[SettingKeySMTPFrom],
		SMTPFromName:                 settings[SettingKeySMTPFromName],
		SMTPUseTLS:                   settings[SettingKeySMTPUseTLS] == "true",
		SMTPPasswordConfigured:       settings[SettingKeySMTPPassword] != "",
		TurnstileEnabled:             settings[SettingKeyTurnstileEnabled] == "true",
		TurnstileSiteKey:             settings[SettingKeyTurnstileSiteKey],
		TurnstileSecretKeyConfigured: settings[SettingKeyTurnstileSecretKey] != "",
		SiteName:                     s.getStringOrDefault(settings, SettingKeySiteName, "Code80"),
		SiteLogo:                     settings[SettingKeySiteLogo],
		SiteLogoDark:                 settings[SettingKeySiteLogoDark],
		SiteSubtitle:                 s.getStringOrDefault(settings, SettingKeySiteSubtitle, "Subscription to API Conversion Platform"),
		APIBaseURL:                   settings[SettingKeyAPIBaseURL],
		ContactInfo:                  settings[SettingKeyContactInfo],
		ContactQRCodeWechat:          settings[SettingKeyContactQRCodeWechat],
		ContactQRCodeGroup:           settings[SettingKeyContactQRCodeGroup],
		DocURL:                       settings[SettingKeyDocURL],
		HomeContent:                  settings[SettingKeyHomeContent],
		HideCcsImportButton:          settings[SettingKeyHideCcsImportButton] == "true",
		PurchaseSubscriptionEnabled:  settings[SettingKeyPurchaseSubscriptionEnabled] == "true",
		PurchaseSubscriptionURL:      strings.TrimSpace(settings[SettingKeyPurchaseSubscriptionURL]),
	}

	// 解析整数类型
	if port, err := strconv.Atoi(settings[SettingKeySMTPPort]); err == nil {
		result.SMTPPort = port
	} else {
		result.SMTPPort = 587
	}

	if concurrency, err := strconv.Atoi(settings[SettingKeyDefaultConcurrency]); err == nil {
		result.DefaultConcurrency = concurrency
	} else {
		result.DefaultConcurrency = s.cfg.Default.UserConcurrency
	}

	// 解析浮点数类型
	if balance, err := strconv.ParseFloat(settings[SettingKeyDefaultBalance], 64); err == nil {
		result.DefaultBalance = balance
	} else {
		result.DefaultBalance = s.cfg.Default.UserBalance
	}

	// 敏感信息直接返回，方便测试连接时使用
	result.SMTPPassword = settings[SettingKeySMTPPassword]
	result.TurnstileSecretKey = settings[SettingKeyTurnstileSecretKey]

	// LinuxDo Connect 设置：
	// - 兼容 config.yaml/env（避免老部署因为未迁移到数据库设置而被意外关闭）
	// - 支持在后台“系统设置”中覆盖并持久化（存储于 DB）
	linuxDoBase := config.LinuxDoConnectConfig{}
	if s.cfg != nil {
		linuxDoBase = s.cfg.LinuxDo
	}

	if raw, ok := settings[SettingKeyLinuxDoConnectEnabled]; ok {
		result.LinuxDoConnectEnabled = raw == "true"
	} else {
		result.LinuxDoConnectEnabled = linuxDoBase.Enabled
	}

	if v, ok := settings[SettingKeyLinuxDoConnectClientID]; ok && strings.TrimSpace(v) != "" {
		result.LinuxDoConnectClientID = strings.TrimSpace(v)
	} else {
		result.LinuxDoConnectClientID = linuxDoBase.ClientID
	}

	if v, ok := settings[SettingKeyLinuxDoConnectRedirectURL]; ok && strings.TrimSpace(v) != "" {
		result.LinuxDoConnectRedirectURL = strings.TrimSpace(v)
	} else {
		result.LinuxDoConnectRedirectURL = linuxDoBase.RedirectURL
	}

	result.LinuxDoConnectClientSecret = strings.TrimSpace(settings[SettingKeyLinuxDoConnectClientSecret])
	if result.LinuxDoConnectClientSecret == "" {
		result.LinuxDoConnectClientSecret = strings.TrimSpace(linuxDoBase.ClientSecret)
	}
	result.LinuxDoConnectClientSecretConfigured = result.LinuxDoConnectClientSecret != ""

	// 微信公众号验证码登录设置
	result.WeChatAuthEnabled = settings[SettingKeyWeChatAuthEnabled] == "true"
	result.WeChatAccountType = s.getStringOrDefault(settings, SettingKeyWeChatAccountType, WeChatAccountTypeSubscription)
	result.WeChatServerAddress = strings.TrimSpace(settings[SettingKeyWeChatServerAddress])
	result.WeChatServerToken = strings.TrimSpace(settings[SettingKeyWeChatServerToken])
	result.WeChatServerTokenConfigured = result.WeChatServerToken != ""
	result.WeChatAccountQRCodeURL = strings.TrimSpace(settings[SettingKeyWeChatAccountQRCodeURL])
	result.WeChatAccountQRCodeData = settings[SettingKeyWeChatAccountQRCodeData]
	result.WeChatAppID = strings.TrimSpace(settings[SettingKeyWeChatAppID])
	result.WeChatAppSecret = strings.TrimSpace(settings[SettingKeyWeChatAppSecret])
	result.WeChatAppSecretConfigured = result.WeChatAppSecret != ""
	result.ForceEmailBind = settings[SettingKeyForceEmailBind] == "true"

	// Model fallback settings
	result.EnableModelFallback = settings[SettingKeyEnableModelFallback] == "true"
	result.FallbackModelAnthropic = s.getStringOrDefault(settings, SettingKeyFallbackModelAnthropic, "claude-3-5-sonnet-20241022")
	result.FallbackModelOpenAI = s.getStringOrDefault(settings, SettingKeyFallbackModelOpenAI, "gpt-4o")
	result.FallbackModelGemini = s.getStringOrDefault(settings, SettingKeyFallbackModelGemini, "gemini-2.5-pro")
	result.FallbackModelAntigravity = s.getStringOrDefault(settings, SettingKeyFallbackModelAntigravity, "gemini-2.5-pro")

	// Identity patch settings (default: enabled, to preserve existing behavior)
	if v, ok := settings[SettingKeyEnableIdentityPatch]; ok && v != "" {
		result.EnableIdentityPatch = v == "true"
	} else {
		result.EnableIdentityPatch = true
	}
	result.IdentityPatchPrompt = settings[SettingKeyIdentityPatchPrompt]

	// Ops monitoring settings (default: enabled, fail-open)
	result.OpsMonitoringEnabled = !isFalseSettingValue(settings[SettingKeyOpsMonitoringEnabled])
	result.OpsRealtimeMonitoringEnabled = !isFalseSettingValue(settings[SettingKeyOpsRealtimeMonitoringEnabled])
	result.OpsQueryModeDefault = string(ParseOpsQueryMode(settings[SettingKeyOpsQueryModeDefault]))
	result.OpsMetricsIntervalSeconds = 60
	if raw := strings.TrimSpace(settings[SettingKeyOpsMetricsIntervalSeconds]); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			if v < 60 {
				v = 60
			}
			if v > 3600 {
				v = 3600
			}
			result.OpsMetricsIntervalSeconds = v
		}
	}

	// Usage report settings
	result.UsageReportGlobalEnabled = settings[SettingKeyUsageReportGlobalEnabled] == "true"
	result.UsageReportTargetScope = s.getStringOrDefault(settings, SettingKeyUsageReportTargetScope, UsageReportScopeOptedIn)
	result.UsageReportGlobalSchedule = s.getStringOrDefault(settings, SettingKeyUsageReportGlobalSchedule, "09:00")

	// Homepage & Install Guide
	result.InstallGuideVideos = settings[SettingKeyInstallGuideVideos]
	result.HomeTestimonials = settings[SettingKeyHomeTestimonials]

	// Account expiry reminder
	result.AccountExpiryReminderEmail = strings.TrimSpace(settings[SettingKeyAccountExpiryReminderEmail])
	result.AccountExpiryReminderAdvanceDays = 7
	if raw := strings.TrimSpace(settings[SettingKeyAccountExpiryReminderAdvanceDays]); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			result.AccountExpiryReminderAdvanceDays = v
		}
	}

	// Balance lot expiry
	result.BalanceLotExpiryDays = 30
	if raw := strings.TrimSpace(settings[SettingKeyBalanceLotExpiryDays]); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			result.BalanceLotExpiryDays = v
		}
	}
	result.BalanceExpiryReminderEnabled = settings[SettingKeyBalanceExpiryReminderEnabled] == "true"
	result.BalanceExpiryReminderAdvanceDays = 3
	if raw := strings.TrimSpace(settings[SettingKeyBalanceExpiryReminderAdvanceDays]); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			result.BalanceExpiryReminderAdvanceDays = v
		}
	}

	return result
}

func isFalseSettingValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "off", "disabled":
		return true
	default:
		return false
	}
}

// getStringOrDefault 获取字符串值或默认值
func (s *SettingService) getStringOrDefault(settings map[string]string, key, defaultValue string) string {
	if value, ok := settings[key]; ok && value != "" {
		return value
	}
	return defaultValue
}

// IsTurnstileEnabled 检查是否启用 Turnstile 验证
func (s *SettingService) IsTurnstileEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyTurnstileEnabled)
	if err != nil {
		return false
	}
	return value == "true"
}

// GetTurnstileSecretKey 获取 Turnstile Secret Key
func (s *SettingService) GetTurnstileSecretKey(ctx context.Context) string {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyTurnstileSecretKey)
	if err != nil {
		return ""
	}
	return value
}

// IsIdentityPatchEnabled 检查是否启用身份补丁（Claude -> Gemini systemInstruction 注入）
func (s *SettingService) IsIdentityPatchEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyEnableIdentityPatch)
	if err != nil {
		// 默认开启，保持兼容
		return true
	}
	return value == "true"
}

// GetIdentityPatchPrompt 获取自定义身份补丁提示词（为空表示使用内置默认模板）
func (s *SettingService) GetIdentityPatchPrompt(ctx context.Context) string {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyIdentityPatchPrompt)
	if err != nil {
		return ""
	}
	return value
}

// GenerateAdminAPIKey 生成新的管理员 API Key
func (s *SettingService) GenerateAdminAPIKey(ctx context.Context) (string, error) {
	// 生成 32 字节随机数 = 64 位十六进制字符
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	key := AdminAPIKeyPrefix + hex.EncodeToString(bytes)

	// 存储到 settings 表
	if err := s.settingRepo.Set(ctx, SettingKeyAdminAPIKey, key); err != nil {
		return "", fmt.Errorf("save admin api key: %w", err)
	}

	return key, nil
}

// GetAdminAPIKeyStatus 获取管理员 API Key 状态
// 返回脱敏的 key、是否存在、错误
func (s *SettingService) GetAdminAPIKeyStatus(ctx context.Context) (maskedKey string, exists bool, err error) {
	key, err := s.settingRepo.GetValue(ctx, SettingKeyAdminAPIKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	if key == "" {
		return "", false, nil
	}

	// 脱敏：显示前 10 位和后 4 位
	if len(key) > 14 {
		maskedKey = key[:10] + "..." + key[len(key)-4:]
	} else {
		maskedKey = key
	}

	return maskedKey, true, nil
}

// GetAdminAPIKey 获取完整的管理员 API Key（仅供内部验证使用）
// 如果未配置返回空字符串和 nil 错误，只有数据库错误时才返回 error
func (s *SettingService) GetAdminAPIKey(ctx context.Context) (string, error) {
	key, err := s.settingRepo.GetValue(ctx, SettingKeyAdminAPIKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return "", nil // 未配置，返回空字符串
		}
		return "", err // 数据库错误
	}
	return key, nil
}

// DeleteAdminAPIKey 删除管理员 API Key
func (s *SettingService) DeleteAdminAPIKey(ctx context.Context) error {
	return s.settingRepo.Delete(ctx, SettingKeyAdminAPIKey)
}

// IsModelFallbackEnabled 检查是否启用模型兜底机制
func (s *SettingService) IsModelFallbackEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyEnableModelFallback)
	if err != nil {
		return false // Default: disabled
	}
	return value == "true"
}

// GetFallbackModel 获取指定平台的兜底模型
func (s *SettingService) GetFallbackModel(ctx context.Context, platform string) string {
	var key string
	var defaultModel string

	switch platform {
	case PlatformAnthropic:
		key = SettingKeyFallbackModelAnthropic
		defaultModel = "claude-3-5-sonnet-20241022"
	case PlatformOpenAI:
		key = SettingKeyFallbackModelOpenAI
		defaultModel = "gpt-4o"
	case PlatformGemini:
		key = SettingKeyFallbackModelGemini
		defaultModel = "gemini-2.5-pro"
	case PlatformAntigravity:
		key = SettingKeyFallbackModelAntigravity
		defaultModel = "gemini-2.5-pro"
	default:
		return ""
	}

	value, err := s.settingRepo.GetValue(ctx, key)
	if err != nil || value == "" {
		return defaultModel
	}
	return value
}

// GetLinuxDoConnectOAuthConfig 返回用于登录的"最终生效" LinuxDo Connect 配置。
//
// 优先级：
// - 若对应系统设置键存在，则覆盖 config.yaml/env 的值
// - 否则回退到 config.yaml/env 的值
func (s *SettingService) GetLinuxDoConnectOAuthConfig(ctx context.Context) (config.LinuxDoConnectConfig, error) {
	if s == nil || s.cfg == nil {
		return config.LinuxDoConnectConfig{}, infraerrors.ServiceUnavailable("CONFIG_NOT_READY", "config not loaded")
	}

	effective := s.cfg.LinuxDo

	keys := []string{
		SettingKeyLinuxDoConnectEnabled,
		SettingKeyLinuxDoConnectClientID,
		SettingKeyLinuxDoConnectClientSecret,
		SettingKeyLinuxDoConnectRedirectURL,
	}
	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return config.LinuxDoConnectConfig{}, fmt.Errorf("get linuxdo connect settings: %w", err)
	}

	if raw, ok := settings[SettingKeyLinuxDoConnectEnabled]; ok {
		effective.Enabled = raw == "true"
	}
	if v, ok := settings[SettingKeyLinuxDoConnectClientID]; ok && strings.TrimSpace(v) != "" {
		effective.ClientID = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyLinuxDoConnectClientSecret]; ok && strings.TrimSpace(v) != "" {
		effective.ClientSecret = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyLinuxDoConnectRedirectURL]; ok && strings.TrimSpace(v) != "" {
		effective.RedirectURL = strings.TrimSpace(v)
	}

	if !effective.Enabled {
		return config.LinuxDoConnectConfig{}, infraerrors.NotFound("OAUTH_DISABLED", "oauth login is disabled")
	}

	// 基础健壮性校验（避免把用户重定向到一个必然失败或不安全的 OAuth 流程里）。
	if strings.TrimSpace(effective.ClientID) == "" {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth client id not configured")
	}
	if strings.TrimSpace(effective.AuthorizeURL) == "" {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth authorize url not configured")
	}
	if strings.TrimSpace(effective.TokenURL) == "" {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth token url not configured")
	}
	if strings.TrimSpace(effective.UserInfoURL) == "" {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth userinfo url not configured")
	}
	if strings.TrimSpace(effective.RedirectURL) == "" {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth redirect url not configured")
	}
	if strings.TrimSpace(effective.FrontendRedirectURL) == "" {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth frontend redirect url not configured")
	}

	if err := config.ValidateAbsoluteHTTPURL(effective.AuthorizeURL); err != nil {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth authorize url invalid")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.TokenURL); err != nil {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth token url invalid")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.UserInfoURL); err != nil {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth userinfo url invalid")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.RedirectURL); err != nil {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth redirect url invalid")
	}
	if err := config.ValidateFrontendRedirectURL(effective.FrontendRedirectURL); err != nil {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth frontend redirect url invalid")
	}

	method := strings.ToLower(strings.TrimSpace(effective.TokenAuthMethod))
	switch method {
	case "", "client_secret_post", "client_secret_basic":
		if strings.TrimSpace(effective.ClientSecret) == "" {
			return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth client secret not configured")
		}
	case "none":
		if !effective.UsePKCE {
			return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth pkce must be enabled when token_auth_method=none")
		}
	default:
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth token_auth_method invalid")
	}

	return effective, nil
}

// GetStreamTimeoutSettings 获取流超时处理配置
func (s *SettingService) GetStreamTimeoutSettings(ctx context.Context) (*StreamTimeoutSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyStreamTimeoutSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultStreamTimeoutSettings(), nil
		}
		return nil, fmt.Errorf("get stream timeout settings: %w", err)
	}
	if value == "" {
		return DefaultStreamTimeoutSettings(), nil
	}

	var settings StreamTimeoutSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return DefaultStreamTimeoutSettings(), nil
	}

	// 验证并修正配置值
	if settings.TempUnschedMinutes < 1 {
		settings.TempUnschedMinutes = 1
	}
	if settings.TempUnschedMinutes > 60 {
		settings.TempUnschedMinutes = 60
	}
	if settings.ThresholdCount < 1 {
		settings.ThresholdCount = 1
	}
	if settings.ThresholdCount > 10 {
		settings.ThresholdCount = 10
	}
	if settings.ThresholdWindowMinutes < 1 {
		settings.ThresholdWindowMinutes = 1
	}
	if settings.ThresholdWindowMinutes > 60 {
		settings.ThresholdWindowMinutes = 60
	}

	// 验证 action
	switch settings.Action {
	case StreamTimeoutActionTempUnsched, StreamTimeoutActionError, StreamTimeoutActionNone:
		// valid
	default:
		settings.Action = StreamTimeoutActionTempUnsched
	}

	return &settings, nil
}

// SetStreamTimeoutSettings 设置流超时处理配置
func (s *SettingService) SetStreamTimeoutSettings(ctx context.Context, settings *StreamTimeoutSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	// 验证配置值
	if settings.TempUnschedMinutes < 1 || settings.TempUnschedMinutes > 60 {
		return fmt.Errorf("temp_unsched_minutes must be between 1-60")
	}
	if settings.ThresholdCount < 1 || settings.ThresholdCount > 10 {
		return fmt.Errorf("threshold_count must be between 1-10")
	}
	if settings.ThresholdWindowMinutes < 1 || settings.ThresholdWindowMinutes > 60 {
		return fmt.Errorf("threshold_window_minutes must be between 1-60")
	}

	switch settings.Action {
	case StreamTimeoutActionTempUnsched, StreamTimeoutActionError, StreamTimeoutActionNone:
		// valid
	default:
		return fmt.Errorf("invalid action: %s", settings.Action)
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal stream timeout settings: %w", err)
	}

	return s.settingRepo.Set(ctx, SettingKeyStreamTimeoutSettings, string(data))
}

// WeChatConfig 微信登录配置
type WeChatConfig struct {
	Enabled       bool
	ServerAddress string
	ServerToken   string
}

// GetWeChatConfig 获取微信登录配置
func (s *SettingService) GetWeChatConfig(ctx context.Context) (*WeChatConfig, error) {
	keys := []string{
		SettingKeyWeChatAuthEnabled,
		SettingKeyWeChatServerAddress,
		SettingKeyWeChatServerToken,
	}

	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get wechat config: %w", err)
	}

	return &WeChatConfig{
		Enabled:       settings[SettingKeyWeChatAuthEnabled] == "true",
		ServerAddress: strings.TrimSpace(settings[SettingKeyWeChatServerAddress]),
		ServerToken:   strings.TrimSpace(settings[SettingKeyWeChatServerToken]),
	}, nil
}

// IsWeChatAuthEnabled 检查是否启用微信登录
func (s *SettingService) IsWeChatAuthEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyWeChatAuthEnabled)
	if err != nil {
		return false
	}
	return value == "true"
}

// GetWeChatAppCredentials 获取微信 AppID 和 AppSecret
func (s *SettingService) GetWeChatAppCredentials(ctx context.Context) (appID, appSecret string, err error) {
	keys := []string{
		SettingKeyWeChatAppID,
		SettingKeyWeChatAppSecret,
	}

	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return "", "", fmt.Errorf("get wechat app credentials: %w", err)
	}

	return strings.TrimSpace(settings[SettingKeyWeChatAppID]),
		strings.TrimSpace(settings[SettingKeyWeChatAppSecret]),
		nil
}

// SetWeChatQRCodeURL 设置微信公众号二维码 URL
func (s *SettingService) SetWeChatQRCodeURL(ctx context.Context, qrcodeURL string) error {
	return s.settingRepo.Set(ctx, SettingKeyWeChatAccountQRCodeURL, qrcodeURL)
}

// UsageReportConfig holds usage report global settings
type UsageReportConfig struct {
	GlobalEnabled  bool
	TargetScope    string // "all", "active_today", "opted_in"
	GlobalSchedule string // HH:MM format
}

// GetUsageReportConfig 获取使用报告全局配置
func (s *SettingService) GetUsageReportConfig(ctx context.Context) (*UsageReportConfig, error) {
	keys := []string{
		SettingKeyUsageReportGlobalEnabled,
		SettingKeyUsageReportTargetScope,
		SettingKeyUsageReportGlobalSchedule,
	}

	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get usage report config: %w", err)
	}

	scope := strings.TrimSpace(settings[SettingKeyUsageReportTargetScope])
	if scope == "" {
		scope = UsageReportScopeOptedIn
	}

	schedule := strings.TrimSpace(settings[SettingKeyUsageReportGlobalSchedule])
	if schedule == "" {
		schedule = "09:00"
	}

	return &UsageReportConfig{
		GlobalEnabled:  settings[SettingKeyUsageReportGlobalEnabled] == "true",
		TargetScope:    scope,
		GlobalSchedule: schedule,
	}, nil
}

// IsUsageReportGlobalEnabled 检查使用报告是否全局启用
func (s *SettingService) IsUsageReportGlobalEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyUsageReportGlobalEnabled)
	if err != nil {
		return false
	}
	return value == "true"
}

// RechargeSettings 充值业务配置
type RechargeSettings struct {
	MinAmount          float64   `json:"min_amount"`           // 最小充值金额（元）
	MaxAmount          float64   `json:"max_amount"`           // 最大充值金额（元）
	DefaultAmounts     []float64 `json:"default_amounts"`      // 默认金额选项
	OrderExpireMinutes int       `json:"order_expire_minutes"` // 订单过期时间（分钟）
	ExchangeRate       float64   `json:"exchange_rate"`        // 人民币兑额度汇率（例如 7.0 表示 ¥7 = $1）
}

// GetRechargeSettings 获取充值业务配置（优先从缓存读取）
func (s *SettingService) GetRechargeSettings(ctx context.Context) (*RechargeSettings, error) {
	// 尝试从缓存读取
	s.rechargeCacheMu.RLock()
	cache := s.rechargeCache
	s.rechargeCacheMu.RUnlock()

	// 缓存有效
	if cache != nil && time.Since(cache.updatedAt) < s.rechargeCacheTTL {
		// 返回副本，避免外部修改
		return &RechargeSettings{
			MinAmount:          cache.settings.MinAmount,
			MaxAmount:          cache.settings.MaxAmount,
			DefaultAmounts:     append([]float64{}, cache.settings.DefaultAmounts...),
			OrderExpireMinutes: cache.settings.OrderExpireMinutes,
			ExchangeRate:       cache.settings.ExchangeRate,
		}, nil
	}

	// 缓存未命中或已过期，从数据库加载
	settings, err := s.loadRechargeSettingsFromDB(ctx)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	s.rechargeCacheMu.Lock()
	s.rechargeCache = &rechargeSettingsCache{
		settings:  settings,
		updatedAt: time.Now(),
	}
	s.rechargeCacheMu.Unlock()

	// 返回副本
	return &RechargeSettings{
		MinAmount:          settings.MinAmount,
		MaxAmount:          settings.MaxAmount,
		DefaultAmounts:     append([]float64{}, settings.DefaultAmounts...),
		OrderExpireMinutes: settings.OrderExpireMinutes,
		ExchangeRate:       settings.ExchangeRate,
	}, nil
}

// UpdateRechargeSettings 更新充值业务配置
func (s *SettingService) UpdateRechargeSettings(ctx context.Context, settings *RechargeSettings) error {
	// 验证逻辑
	if settings.MinAmount <= 0 {
		return fmt.Errorf("min_amount must be greater than 0")
	}
	if settings.MaxAmount <= 0 {
		return fmt.Errorf("max_amount must be greater than 0")
	}
	if settings.MinAmount > settings.MaxAmount {
		return fmt.Errorf("min_amount must be less than or equal to max_amount")
	}
	if settings.OrderExpireMinutes < 1 || settings.OrderExpireMinutes > 1440 {
		return fmt.Errorf("order_expire_minutes must be between 1 and 1440")
	}
	if settings.ExchangeRate <= 0 {
		return fmt.Errorf("exchange_rate must be greater than 0")
	}

	// 验证金额选项
	for _, amount := range settings.DefaultAmounts {
		if amount < settings.MinAmount || amount > settings.MaxAmount {
			return fmt.Errorf("default amount %.2f is out of allowed range [%.2f, %.2f]",
				amount, settings.MinAmount, settings.MaxAmount)
		}
	}

	// 序列化金额选项
	amountsJSON, err := json.Marshal(settings.DefaultAmounts)
	if err != nil {
		return fmt.Errorf("marshal default amounts: %w", err)
	}

	updates := map[string]string{
		SettingKeyRechargeMinAmount:          strconv.FormatFloat(settings.MinAmount, 'f', 2, 64),
		SettingKeyRechargeMaxAmount:          strconv.FormatFloat(settings.MaxAmount, 'f', 2, 64),
		SettingKeyRechargeDefaultAmounts:     string(amountsJSON),
		SettingKeyRechargeOrderExpireMinutes: strconv.Itoa(settings.OrderExpireMinutes),
		SettingKeyRechargeExchangeRate:       strconv.FormatFloat(settings.ExchangeRate, 'f', 4, 64),
	}

	// 保存到数据库
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return err
	}

	// 主动刷新缓存（确保实时生效）
	s.rechargeCacheMu.Lock()
	s.rechargeCache = &rechargeSettingsCache{
		settings: &RechargeSettings{
			MinAmount:          settings.MinAmount,
			MaxAmount:          settings.MaxAmount,
			DefaultAmounts:     append([]float64{}, settings.DefaultAmounts...),
			OrderExpireMinutes: settings.OrderExpireMinutes,
			ExchangeRate:       settings.ExchangeRate,
		},
		updatedAt: time.Now(),
	}
	s.rechargeCacheMu.Unlock()

	return nil
}
