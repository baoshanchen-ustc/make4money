package admin

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SettingHandler 系统设置处理器
type SettingHandler struct {
	settingService      *service.SettingService
	emailService        *service.EmailService
	turnstileService    *service.TurnstileService
	opsService          *service.OpsService
	wechatQRCodeService *service.WeChatQRCodeService
	wechatPayService    *service.WeChatPayService
}

// NewSettingHandler 创建系统设置处理器
func NewSettingHandler(settingService *service.SettingService, emailService *service.EmailService, turnstileService *service.TurnstileService, opsService *service.OpsService, wechatQRCodeService *service.WeChatQRCodeService, wechatPayService *service.WeChatPayService) *SettingHandler {
	return &SettingHandler{
		settingService:      settingService,
		emailService:        emailService,
		turnstileService:    turnstileService,
		opsService:          opsService,
		wechatQRCodeService: wechatQRCodeService,
		wechatPayService:    wechatPayService,
	}
}

// GetSettings 获取所有系统设置
// GET /api/v1/admin/settings
func (h *SettingHandler) GetSettings(c *gin.Context) {
	settings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Check if ops monitoring is enabled (respects config.ops.enabled)
	opsEnabled := h.opsService != nil && h.opsService.IsMonitoringEnabled(c.Request.Context())

	response.Success(c, dto.SystemSettings{
		RegistrationEnabled:                  settings.RegistrationEnabled,
		EmailVerifyEnabled:                   settings.EmailVerifyEnabled,
		PromoCodeEnabled:                     settings.PromoCodeEnabled,
		PasswordResetEnabled:                 settings.PasswordResetEnabled,
		InvitationCodeEnabled:                settings.InvitationCodeEnabled,
		TotpEnabled:                          settings.TotpEnabled,
		TotpEncryptionKeyConfigured:          h.settingService.IsTotpEncryptionKeyConfigured(),
		SMTPHost:                             settings.SMTPHost,
		SMTPPort:                             settings.SMTPPort,
		SMTPUsername:                         settings.SMTPUsername,
		SMTPPasswordConfigured:               settings.SMTPPasswordConfigured,
		SMTPFrom:                             settings.SMTPFrom,
		SMTPFromName:                         settings.SMTPFromName,
		SMTPUseTLS:                           settings.SMTPUseTLS,
		TurnstileEnabled:                     settings.TurnstileEnabled,
		TurnstileSiteKey:                     settings.TurnstileSiteKey,
		TurnstileSecretKeyConfigured:         settings.TurnstileSecretKeyConfigured,
		LinuxDoConnectEnabled:                settings.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:               settings.LinuxDoConnectClientID,
		LinuxDoConnectClientSecretConfigured: settings.LinuxDoConnectClientSecretConfigured,
		LinuxDoConnectRedirectURL:            settings.LinuxDoConnectRedirectURL,
		WeChatAuthEnabled:                    settings.WeChatAuthEnabled,
		WeChatAccountType:                    settings.WeChatAccountType,
		WeChatServerAddress:                  settings.WeChatServerAddress,
		WeChatServerTokenConfigured:          settings.WeChatServerTokenConfigured,
		WeChatAccountQRCodeURL:               settings.WeChatAccountQRCodeURL,
		WeChatAccountQRCodeData:              settings.WeChatAccountQRCodeData,
		WeChatAppID:                          settings.WeChatAppID,
		WeChatAppSecretConfigured:            settings.WeChatAppSecretConfigured,
		SiteName:                             settings.SiteName,
		SiteLogo:                             settings.SiteLogo,
		SiteLogoDark:                         settings.SiteLogoDark,
		SiteSubtitle:                         settings.SiteSubtitle,
		APIBaseURL:                           settings.APIBaseURL,
		ContactInfo:                          settings.ContactInfo,
		ContactQRCodeWechat:                  settings.ContactQRCodeWechat,
		ContactQRCodeGroup:                   settings.ContactQRCodeGroup,
		DocURL:                               settings.DocURL,
		HomeContent:                          settings.HomeContent,
		HideCcsImportButton:                  settings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:          settings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:              settings.PurchaseSubscriptionURL,
		DefaultConcurrency:                   settings.DefaultConcurrency,
		DefaultBalance:                       settings.DefaultBalance,
		EnableModelFallback:                  settings.EnableModelFallback,
		FallbackModelAnthropic:               settings.FallbackModelAnthropic,
		FallbackModelOpenAI:                  settings.FallbackModelOpenAI,
		FallbackModelGemini:                  settings.FallbackModelGemini,
		FallbackModelAntigravity:             settings.FallbackModelAntigravity,
		EnableIdentityPatch:                  settings.EnableIdentityPatch,
		IdentityPatchPrompt:                  settings.IdentityPatchPrompt,
		OpsMonitoringEnabled:                 opsEnabled && settings.OpsMonitoringEnabled,
		OpsRealtimeMonitoringEnabled:         settings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:                  settings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:            settings.OpsMetricsIntervalSeconds,
		UsageReportGlobalEnabled:             settings.UsageReportGlobalEnabled,
		UsageReportTargetScope:               settings.UsageReportTargetScope,
		UsageReportGlobalSchedule:            settings.UsageReportGlobalSchedule,
	})
}

// UpdateSettingsRequest 更新设置请求
type UpdateSettingsRequest struct {
	// 注册设置
	RegistrationEnabled   bool `json:"registration_enabled"`
	EmailVerifyEnabled    bool `json:"email_verify_enabled"`
	PromoCodeEnabled      bool `json:"promo_code_enabled"`
	PasswordResetEnabled  bool `json:"password_reset_enabled"`
	InvitationCodeEnabled bool `json:"invitation_code_enabled"`
	TotpEnabled           bool `json:"totp_enabled"` // TOTP 双因素认证

	// 邮件服务设置
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from_email"`
	SMTPFromName string `json:"smtp_from_name"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`

	// Cloudflare Turnstile 设置
	TurnstileEnabled   bool   `json:"turnstile_enabled"`
	TurnstileSiteKey   string `json:"turnstile_site_key"`
	TurnstileSecretKey string `json:"turnstile_secret_key"`

	// LinuxDo Connect OAuth 登录
	LinuxDoConnectEnabled      bool   `json:"linuxdo_connect_enabled"`
	LinuxDoConnectClientID     string `json:"linuxdo_connect_client_id"`
	LinuxDoConnectClientSecret string `json:"linuxdo_connect_client_secret"`
	LinuxDoConnectRedirectURL  string `json:"linuxdo_connect_redirect_url"`

	// 微信公众号验证码登录
	WeChatAuthEnabled       bool   `json:"wechat_auth_enabled"`
	WeChatAccountType       string `json:"wechat_account_type"`
	WeChatServerAddress     string `json:"wechat_server_address"`
	WeChatServerToken       string `json:"wechat_server_token"`
	WeChatAccountQRCodeURL  string `json:"wechat_account_qrcode_url"`
	WeChatAccountQRCodeData string `json:"wechat_account_qrcode_data"`
	WeChatAppID             string `json:"wechat_app_id"`
	WeChatAppSecret         string `json:"wechat_app_secret"`

	// OEM设置
	SiteName                    string  `json:"site_name"`
	SiteLogo                    string  `json:"site_logo"`
	SiteLogoDark                string  `json:"site_logo_dark"`
	SiteSubtitle                string  `json:"site_subtitle"`
	APIBaseURL                  string  `json:"api_base_url"`
	ContactInfo                 string  `json:"contact_info"`
	ContactQRCodeWechat         string  `json:"contact_qrcode_wechat"`
	ContactQRCodeGroup          string  `json:"contact_qrcode_group"`
	DocURL                      string  `json:"doc_url"`
	HomeContent                 string  `json:"home_content"`
	HideCcsImportButton         bool    `json:"hide_ccs_import_button"`
	PurchaseSubscriptionEnabled *bool   `json:"purchase_subscription_enabled"`
	PurchaseSubscriptionURL     *string `json:"purchase_subscription_url"`

	// 默认配置
	DefaultConcurrency int     `json:"default_concurrency"`
	DefaultBalance     float64 `json:"default_balance"`

	// Model fallback configuration
	EnableModelFallback      bool   `json:"enable_model_fallback"`
	FallbackModelAnthropic   string `json:"fallback_model_anthropic"`
	FallbackModelOpenAI      string `json:"fallback_model_openai"`
	FallbackModelGemini      string `json:"fallback_model_gemini"`
	FallbackModelAntigravity string `json:"fallback_model_antigravity"`

	// Identity patch configuration (Claude -> Gemini)
	EnableIdentityPatch bool   `json:"enable_identity_patch"`
	IdentityPatchPrompt string `json:"identity_patch_prompt"`

	// Ops monitoring (vNext)
	OpsMonitoringEnabled         *bool   `json:"ops_monitoring_enabled"`
	OpsRealtimeMonitoringEnabled *bool   `json:"ops_realtime_monitoring_enabled"`
	OpsQueryModeDefault          *string `json:"ops_query_mode_default"`
	OpsMetricsIntervalSeconds    *int    `json:"ops_metrics_interval_seconds"`

	// Usage report settings
	UsageReportGlobalEnabled  *bool   `json:"usage_report_global_enabled"`
	UsageReportTargetScope    *string `json:"usage_report_target_scope"`
	UsageReportGlobalSchedule *string `json:"usage_report_global_schedule"`
}

// UpdateSettings 更新系统设置
// PUT /api/v1/admin/settings
func (h *SettingHandler) UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	previousSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 验证参数
	if req.DefaultConcurrency < 1 {
		req.DefaultConcurrency = 1
	}
	if req.DefaultBalance < 0 {
		req.DefaultBalance = 0
	}
	if req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}

	// Turnstile 参数验证
	if req.TurnstileEnabled {
		// 检查必填字段
		if req.TurnstileSiteKey == "" {
			response.BadRequest(c, "Turnstile Site Key is required when enabled")
			return
		}
		// 如果未提供 secret key，使用已保存的值（留空保留当前值）
		if req.TurnstileSecretKey == "" {
			if previousSettings.TurnstileSecretKey == "" {
				response.BadRequest(c, "Turnstile Secret Key is required when enabled")
				return
			}
			req.TurnstileSecretKey = previousSettings.TurnstileSecretKey
		}

		// 当 site_key 或 secret_key 任一变化时验证（避免配置错误导致无法登录）
		siteKeyChanged := previousSettings.TurnstileSiteKey != req.TurnstileSiteKey
		secretKeyChanged := previousSettings.TurnstileSecretKey != req.TurnstileSecretKey
		if siteKeyChanged || secretKeyChanged {
			if err := h.turnstileService.ValidateSecretKey(c.Request.Context(), req.TurnstileSecretKey); err != nil {
				response.ErrorFrom(c, err)
				return
			}
		}
	}

	// TOTP 双因素认证参数验证
	// 只有手动配置了加密密钥才允许启用 TOTP 功能
	if req.TotpEnabled && !previousSettings.TotpEnabled {
		// 尝试启用 TOTP，检查加密密钥是否已手动配置
		if !h.settingService.IsTotpEncryptionKeyConfigured() {
			response.BadRequest(c, "Cannot enable TOTP: TOTP_ENCRYPTION_KEY environment variable must be configured first. Generate a key with 'openssl rand -hex 32' and set it in your environment.")
			return
		}
	}

	// LinuxDo Connect 参数验证
	if req.LinuxDoConnectEnabled {
		req.LinuxDoConnectClientID = strings.TrimSpace(req.LinuxDoConnectClientID)
		req.LinuxDoConnectClientSecret = strings.TrimSpace(req.LinuxDoConnectClientSecret)
		req.LinuxDoConnectRedirectURL = strings.TrimSpace(req.LinuxDoConnectRedirectURL)

		if req.LinuxDoConnectClientID == "" {
			response.BadRequest(c, "LinuxDo Client ID is required when enabled")
			return
		}
		if req.LinuxDoConnectRedirectURL == "" {
			response.BadRequest(c, "LinuxDo Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.LinuxDoConnectRedirectURL); err != nil {
			response.BadRequest(c, "LinuxDo Redirect URL must be an absolute http(s) URL")
			return
		}

		// 如果未提供 client_secret，则保留现有值（如有）。
		if req.LinuxDoConnectClientSecret == "" {
			if previousSettings.LinuxDoConnectClientSecret == "" {
				response.BadRequest(c, "LinuxDo Client Secret is required when enabled")
				return
			}
			req.LinuxDoConnectClientSecret = previousSettings.LinuxDoConnectClientSecret
		}
	}

	// 微信公众号验证码登录参数验证
	if req.WeChatAuthEnabled {
		req.WeChatServerAddress = strings.TrimSpace(req.WeChatServerAddress)
		req.WeChatServerToken = strings.TrimSpace(req.WeChatServerToken)

		// 校验公众号类型
		if req.WeChatAccountType != "" &&
			req.WeChatAccountType != service.WeChatAccountTypeSubscription &&
			req.WeChatAccountType != service.WeChatAccountTypeUnverifiedOfficial {
			response.BadRequest(c, "WeChat Account Type must be 'subscription' or 'unverified_official'")
			return
		}
		if req.WeChatAccountType == "" {
			req.WeChatAccountType = service.WeChatAccountTypeSubscription
		}

		if req.WeChatServerAddress == "" {
			response.BadRequest(c, "WeChat Server Address is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.WeChatServerAddress); err != nil {
			response.BadRequest(c, "WeChat Server Address must be an absolute http(s) URL")
			return
		}

		// 如果未提供 server_token，则保留现有值（如有）。
		if req.WeChatServerToken == "" {
			if previousSettings.WeChatServerToken == "" {
				response.BadRequest(c, "WeChat Server Token is required when enabled")
				return
			}
			req.WeChatServerToken = previousSettings.WeChatServerToken
		}
	}

	// "购买订阅"页面配置验证
	purchaseEnabled := previousSettings.PurchaseSubscriptionEnabled
	if req.PurchaseSubscriptionEnabled != nil {
		purchaseEnabled = *req.PurchaseSubscriptionEnabled
	}
	purchaseURL := previousSettings.PurchaseSubscriptionURL
	if req.PurchaseSubscriptionURL != nil {
		purchaseURL = strings.TrimSpace(*req.PurchaseSubscriptionURL)
	}

	// - 启用时要求 URL 合法且非空
	// - 禁用时允许为空；若提供了 URL 也做基本校验，避免误配置
	if purchaseEnabled {
		if purchaseURL == "" {
			response.BadRequest(c, "Purchase Subscription URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(purchaseURL); err != nil {
			response.BadRequest(c, "Purchase Subscription URL must be an absolute http(s) URL")
			return
		}
	} else if purchaseURL != "" {
		if err := config.ValidateAbsoluteHTTPURL(purchaseURL); err != nil {
			response.BadRequest(c, "Purchase Subscription URL must be an absolute http(s) URL")
			return
		}
	}

	// Ops metrics collector interval validation (seconds).
	if req.OpsMetricsIntervalSeconds != nil {
		v := *req.OpsMetricsIntervalSeconds
		if v < 60 {
			v = 60
		}
		if v > 3600 {
			v = 3600
		}
		req.OpsMetricsIntervalSeconds = &v
	}

	settings := &service.SystemSettings{
		RegistrationEnabled:         req.RegistrationEnabled,
		EmailVerifyEnabled:          req.EmailVerifyEnabled,
		PromoCodeEnabled:            req.PromoCodeEnabled,
		PasswordResetEnabled:        req.PasswordResetEnabled,
		InvitationCodeEnabled:       req.InvitationCodeEnabled,
		TotpEnabled:                 req.TotpEnabled,
		SMTPHost:                    req.SMTPHost,
		SMTPPort:                    req.SMTPPort,
		SMTPUsername:                req.SMTPUsername,
		SMTPPassword:                req.SMTPPassword,
		SMTPFrom:                    req.SMTPFrom,
		SMTPFromName:                req.SMTPFromName,
		SMTPUseTLS:                  req.SMTPUseTLS,
		TurnstileEnabled:            req.TurnstileEnabled,
		TurnstileSiteKey:            req.TurnstileSiteKey,
		TurnstileSecretKey:          req.TurnstileSecretKey,
		LinuxDoConnectEnabled:       req.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:      req.LinuxDoConnectClientID,
		LinuxDoConnectClientSecret:  req.LinuxDoConnectClientSecret,
		LinuxDoConnectRedirectURL:   req.LinuxDoConnectRedirectURL,
		WeChatAuthEnabled:           req.WeChatAuthEnabled,
		WeChatAccountType:           req.WeChatAccountType,
		WeChatServerAddress:         req.WeChatServerAddress,
		WeChatServerToken:           req.WeChatServerToken,
		WeChatAccountQRCodeURL:      req.WeChatAccountQRCodeURL,
		WeChatAccountQRCodeData:     req.WeChatAccountQRCodeData,
		WeChatAppID:                 req.WeChatAppID,
		WeChatAppSecret:             req.WeChatAppSecret,
		SiteName:                    req.SiteName,
		SiteLogo:                    req.SiteLogo,
		SiteLogoDark:                req.SiteLogoDark,
		SiteSubtitle:                req.SiteSubtitle,
		APIBaseURL:                  req.APIBaseURL,
		ContactInfo:                 req.ContactInfo,
		ContactQRCodeWechat:         req.ContactQRCodeWechat,
		ContactQRCodeGroup:          req.ContactQRCodeGroup,
		DocURL:                      req.DocURL,
		HomeContent:                 req.HomeContent,
		HideCcsImportButton:         req.HideCcsImportButton,
		PurchaseSubscriptionEnabled: purchaseEnabled,
		PurchaseSubscriptionURL:     purchaseURL,
		DefaultConcurrency:          req.DefaultConcurrency,
		DefaultBalance:              req.DefaultBalance,
		EnableModelFallback:         req.EnableModelFallback,
		FallbackModelAnthropic:      req.FallbackModelAnthropic,
		FallbackModelOpenAI:         req.FallbackModelOpenAI,
		FallbackModelGemini:         req.FallbackModelGemini,
		FallbackModelAntigravity:    req.FallbackModelAntigravity,
		EnableIdentityPatch:         req.EnableIdentityPatch,
		IdentityPatchPrompt:         req.IdentityPatchPrompt,
		OpsMonitoringEnabled: func() bool {
			if req.OpsMonitoringEnabled != nil {
				return *req.OpsMonitoringEnabled
			}
			return previousSettings.OpsMonitoringEnabled
		}(),
		OpsRealtimeMonitoringEnabled: func() bool {
			if req.OpsRealtimeMonitoringEnabled != nil {
				return *req.OpsRealtimeMonitoringEnabled
			}
			return previousSettings.OpsRealtimeMonitoringEnabled
		}(),
		OpsQueryModeDefault: func() string {
			if req.OpsQueryModeDefault != nil {
				return *req.OpsQueryModeDefault
			}
			return previousSettings.OpsQueryModeDefault
		}(),
		OpsMetricsIntervalSeconds: func() int {
			if req.OpsMetricsIntervalSeconds != nil {
				return *req.OpsMetricsIntervalSeconds
			}
			return previousSettings.OpsMetricsIntervalSeconds
		}(),
		UsageReportGlobalEnabled: func() bool {
			if req.UsageReportGlobalEnabled != nil {
				return *req.UsageReportGlobalEnabled
			}
			return previousSettings.UsageReportGlobalEnabled
		}(),
		UsageReportTargetScope: func() string {
			if req.UsageReportTargetScope != nil {
				return *req.UsageReportTargetScope
			}
			return previousSettings.UsageReportTargetScope
		}(),
		UsageReportGlobalSchedule: func() string {
			if req.UsageReportGlobalSchedule != nil {
				return *req.UsageReportGlobalSchedule
			}
			return previousSettings.UsageReportGlobalSchedule
		}(),
	}

	if err := h.settingService.UpdateSettings(c.Request.Context(), settings); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	h.auditSettingsUpdate(c, previousSettings, settings, req)

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.SystemSettings{
		RegistrationEnabled:                  updatedSettings.RegistrationEnabled,
		EmailVerifyEnabled:                   updatedSettings.EmailVerifyEnabled,
		PromoCodeEnabled:                     updatedSettings.PromoCodeEnabled,
		PasswordResetEnabled:                 updatedSettings.PasswordResetEnabled,
		InvitationCodeEnabled:                updatedSettings.InvitationCodeEnabled,
		TotpEnabled:                          updatedSettings.TotpEnabled,
		TotpEncryptionKeyConfigured:          h.settingService.IsTotpEncryptionKeyConfigured(),
		SMTPHost:                             updatedSettings.SMTPHost,
		SMTPPort:                             updatedSettings.SMTPPort,
		SMTPUsername:                         updatedSettings.SMTPUsername,
		SMTPPasswordConfigured:               updatedSettings.SMTPPasswordConfigured,
		SMTPFrom:                             updatedSettings.SMTPFrom,
		SMTPFromName:                         updatedSettings.SMTPFromName,
		SMTPUseTLS:                           updatedSettings.SMTPUseTLS,
		TurnstileEnabled:                     updatedSettings.TurnstileEnabled,
		TurnstileSiteKey:                     updatedSettings.TurnstileSiteKey,
		TurnstileSecretKeyConfigured:         updatedSettings.TurnstileSecretKeyConfigured,
		LinuxDoConnectEnabled:                updatedSettings.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:               updatedSettings.LinuxDoConnectClientID,
		LinuxDoConnectClientSecretConfigured: updatedSettings.LinuxDoConnectClientSecretConfigured,
		LinuxDoConnectRedirectURL:            updatedSettings.LinuxDoConnectRedirectURL,
		WeChatAuthEnabled:                    updatedSettings.WeChatAuthEnabled,
		WeChatAccountType:                    updatedSettings.WeChatAccountType,
		WeChatServerAddress:                  updatedSettings.WeChatServerAddress,
		WeChatServerTokenConfigured:          updatedSettings.WeChatServerTokenConfigured,
		WeChatAccountQRCodeURL:               updatedSettings.WeChatAccountQRCodeURL,
		WeChatAccountQRCodeData:              updatedSettings.WeChatAccountQRCodeData,
		WeChatAppID:                          updatedSettings.WeChatAppID,
		WeChatAppSecretConfigured:            updatedSettings.WeChatAppSecretConfigured,
		SiteName:                             updatedSettings.SiteName,
		SiteLogo:                             updatedSettings.SiteLogo,
		SiteLogoDark:                         updatedSettings.SiteLogoDark,
		SiteSubtitle:                         updatedSettings.SiteSubtitle,
		APIBaseURL:                           updatedSettings.APIBaseURL,
		ContactInfo:                          updatedSettings.ContactInfo,
		ContactQRCodeWechat:                  updatedSettings.ContactQRCodeWechat,
		ContactQRCodeGroup:                   updatedSettings.ContactQRCodeGroup,
		DocURL:                               updatedSettings.DocURL,
		HomeContent:                          updatedSettings.HomeContent,
		HideCcsImportButton:                  updatedSettings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:          updatedSettings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:              updatedSettings.PurchaseSubscriptionURL,
		DefaultConcurrency:                   updatedSettings.DefaultConcurrency,
		DefaultBalance:                       updatedSettings.DefaultBalance,
		EnableModelFallback:                  updatedSettings.EnableModelFallback,
		FallbackModelAnthropic:               updatedSettings.FallbackModelAnthropic,
		FallbackModelOpenAI:                  updatedSettings.FallbackModelOpenAI,
		FallbackModelGemini:                  updatedSettings.FallbackModelGemini,
		FallbackModelAntigravity:             updatedSettings.FallbackModelAntigravity,
		EnableIdentityPatch:                  updatedSettings.EnableIdentityPatch,
		IdentityPatchPrompt:                  updatedSettings.IdentityPatchPrompt,
		OpsMonitoringEnabled:                 updatedSettings.OpsMonitoringEnabled,
		OpsRealtimeMonitoringEnabled:         updatedSettings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:                  updatedSettings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:            updatedSettings.OpsMetricsIntervalSeconds,
		UsageReportGlobalEnabled:             updatedSettings.UsageReportGlobalEnabled,
		UsageReportTargetScope:               updatedSettings.UsageReportTargetScope,
		UsageReportGlobalSchedule:            updatedSettings.UsageReportGlobalSchedule,
	})
}

func (h *SettingHandler) auditSettingsUpdate(c *gin.Context, before *service.SystemSettings, after *service.SystemSettings, req UpdateSettingsRequest) {
	if before == nil || after == nil {
		return
	}

	changed := diffSettings(before, after, req)
	if len(changed) == 0 {
		return
	}

	subject, _ := middleware.GetAuthSubjectFromContext(c)
	role, _ := middleware.GetUserRoleFromContext(c)
	log.Printf("AUDIT: settings updated at=%s user_id=%d role=%s changed=%v",
		time.Now().UTC().Format(time.RFC3339),
		subject.UserID,
		role,
		changed,
	)
}

func diffSettings(before *service.SystemSettings, after *service.SystemSettings, req UpdateSettingsRequest) []string {
	changed := make([]string, 0, 20)
	if before.RegistrationEnabled != after.RegistrationEnabled {
		changed = append(changed, "registration_enabled")
	}
	if before.EmailVerifyEnabled != after.EmailVerifyEnabled {
		changed = append(changed, "email_verify_enabled")
	}
	if before.PasswordResetEnabled != after.PasswordResetEnabled {
		changed = append(changed, "password_reset_enabled")
	}
	if before.TotpEnabled != after.TotpEnabled {
		changed = append(changed, "totp_enabled")
	}
	if before.SMTPHost != after.SMTPHost {
		changed = append(changed, "smtp_host")
	}
	if before.SMTPPort != after.SMTPPort {
		changed = append(changed, "smtp_port")
	}
	if before.SMTPUsername != after.SMTPUsername {
		changed = append(changed, "smtp_username")
	}
	if req.SMTPPassword != "" {
		changed = append(changed, "smtp_password")
	}
	if before.SMTPFrom != after.SMTPFrom {
		changed = append(changed, "smtp_from_email")
	}
	if before.SMTPFromName != after.SMTPFromName {
		changed = append(changed, "smtp_from_name")
	}
	if before.SMTPUseTLS != after.SMTPUseTLS {
		changed = append(changed, "smtp_use_tls")
	}
	if before.TurnstileEnabled != after.TurnstileEnabled {
		changed = append(changed, "turnstile_enabled")
	}
	if before.TurnstileSiteKey != after.TurnstileSiteKey {
		changed = append(changed, "turnstile_site_key")
	}
	if req.TurnstileSecretKey != "" {
		changed = append(changed, "turnstile_secret_key")
	}
	if before.LinuxDoConnectEnabled != after.LinuxDoConnectEnabled {
		changed = append(changed, "linuxdo_connect_enabled")
	}
	if before.LinuxDoConnectClientID != after.LinuxDoConnectClientID {
		changed = append(changed, "linuxdo_connect_client_id")
	}
	if req.LinuxDoConnectClientSecret != "" {
		changed = append(changed, "linuxdo_connect_client_secret")
	}
	if before.LinuxDoConnectRedirectURL != after.LinuxDoConnectRedirectURL {
		changed = append(changed, "linuxdo_connect_redirect_url")
	}
	if before.WeChatAuthEnabled != after.WeChatAuthEnabled {
		changed = append(changed, "wechat_auth_enabled")
	}
	if before.WeChatAccountType != after.WeChatAccountType {
		changed = append(changed, "wechat_account_type")
	}
	if before.WeChatServerAddress != after.WeChatServerAddress {
		changed = append(changed, "wechat_server_address")
	}
	if req.WeChatServerToken != "" {
		changed = append(changed, "wechat_server_token")
	}
	if before.WeChatAccountQRCodeURL != after.WeChatAccountQRCodeURL {
		changed = append(changed, "wechat_account_qrcode_url")
	}
	if before.WeChatAccountQRCodeData != after.WeChatAccountQRCodeData {
		changed = append(changed, "wechat_account_qrcode_data")
	}
	if before.WeChatAppID != after.WeChatAppID {
		changed = append(changed, "wechat_app_id")
	}
	if req.WeChatAppSecret != "" {
		changed = append(changed, "wechat_app_secret")
	}
	if before.SiteName != after.SiteName {
		changed = append(changed, "site_name")
	}
	if before.SiteLogo != after.SiteLogo {
		changed = append(changed, "site_logo")
	}
	if before.SiteLogoDark != after.SiteLogoDark {
		changed = append(changed, "site_logo_dark")
	}
	if before.SiteSubtitle != after.SiteSubtitle {
		changed = append(changed, "site_subtitle")
	}
	if before.APIBaseURL != after.APIBaseURL {
		changed = append(changed, "api_base_url")
	}
	if before.ContactInfo != after.ContactInfo {
		changed = append(changed, "contact_info")
	}
	if before.DocURL != after.DocURL {
		changed = append(changed, "doc_url")
	}
	if before.HomeContent != after.HomeContent {
		changed = append(changed, "home_content")
	}
	if before.HideCcsImportButton != after.HideCcsImportButton {
		changed = append(changed, "hide_ccs_import_button")
	}
	if before.DefaultConcurrency != after.DefaultConcurrency {
		changed = append(changed, "default_concurrency")
	}
	if before.DefaultBalance != after.DefaultBalance {
		changed = append(changed, "default_balance")
	}
	if before.EnableModelFallback != after.EnableModelFallback {
		changed = append(changed, "enable_model_fallback")
	}
	if before.FallbackModelAnthropic != after.FallbackModelAnthropic {
		changed = append(changed, "fallback_model_anthropic")
	}
	if before.FallbackModelOpenAI != after.FallbackModelOpenAI {
		changed = append(changed, "fallback_model_openai")
	}
	if before.FallbackModelGemini != after.FallbackModelGemini {
		changed = append(changed, "fallback_model_gemini")
	}
	if before.FallbackModelAntigravity != after.FallbackModelAntigravity {
		changed = append(changed, "fallback_model_antigravity")
	}
	if before.EnableIdentityPatch != after.EnableIdentityPatch {
		changed = append(changed, "enable_identity_patch")
	}
	if before.IdentityPatchPrompt != after.IdentityPatchPrompt {
		changed = append(changed, "identity_patch_prompt")
	}
	if before.OpsMonitoringEnabled != after.OpsMonitoringEnabled {
		changed = append(changed, "ops_monitoring_enabled")
	}
	if before.OpsRealtimeMonitoringEnabled != after.OpsRealtimeMonitoringEnabled {
		changed = append(changed, "ops_realtime_monitoring_enabled")
	}
	if before.OpsQueryModeDefault != after.OpsQueryModeDefault {
		changed = append(changed, "ops_query_mode_default")
	}
	if before.OpsMetricsIntervalSeconds != after.OpsMetricsIntervalSeconds {
		changed = append(changed, "ops_metrics_interval_seconds")
	}
	if before.UsageReportGlobalEnabled != after.UsageReportGlobalEnabled {
		changed = append(changed, "usage_report_global_enabled")
	}
	if before.UsageReportTargetScope != after.UsageReportTargetScope {
		changed = append(changed, "usage_report_target_scope")
	}
	if before.UsageReportGlobalSchedule != after.UsageReportGlobalSchedule {
		changed = append(changed, "usage_report_global_schedule")
	}
	return changed
}

// TestSMTPRequest 测试SMTP连接请求
type TestSMTPRequest struct {
	SMTPHost     string `json:"smtp_host" binding:"required"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`
}

// TestSMTPConnection 测试SMTP连接
// POST /api/v1/admin/settings/test-smtp
func (h *SettingHandler) TestSMTPConnection(c *gin.Context) {
	var req TestSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}

	// 如果未提供密码，从数据库获取已保存的密码
	password := req.SMTPPassword
	if password == "" {
		savedConfig, err := h.emailService.GetSMTPConfig(c.Request.Context())
		if err == nil && savedConfig != nil {
			password = savedConfig.Password
		}
	}

	config := &service.SMTPConfig{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
		UseTLS:   req.SMTPUseTLS,
	}

	err := h.emailService.TestSMTPConnectionWithConfig(config)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "SMTP connection successful"})
}

// SendTestEmailRequest 发送测试邮件请求
type SendTestEmailRequest struct {
	Email        string `json:"email" binding:"required,email"`
	SMTPHost     string `json:"smtp_host" binding:"required"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from_email"`
	SMTPFromName string `json:"smtp_from_name"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`
}

// SendTestEmail 发送测试邮件
// POST /api/v1/admin/settings/send-test-email
func (h *SettingHandler) SendTestEmail(c *gin.Context) {
	var req SendTestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}

	// 如果未提供密码，从数据库获取已保存的密码
	password := req.SMTPPassword
	if password == "" {
		savedConfig, err := h.emailService.GetSMTPConfig(c.Request.Context())
		if err == nil && savedConfig != nil {
			password = savedConfig.Password
		}
	}

	config := &service.SMTPConfig{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
		From:     req.SMTPFrom,
		FromName: req.SMTPFromName,
		UseTLS:   req.SMTPUseTLS,
	}

	siteName := h.settingService.GetSiteName(c.Request.Context())
	subject := "[" + siteName + "] Test Email"
	body := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f5f5f5; margin: 0; padding: 20px; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; }
        .content { padding: 40px 30px; text-align: center; }
        .success { color: #10b981; font-size: 48px; margin-bottom: 20px; }
        .footer { background-color: #f8f9fa; padding: 20px; text-align: center; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>` + siteName + `</h1>
        </div>
        <div class="content">
            <div class="success">✓</div>
            <h2>Email Configuration Successful!</h2>
            <p>This is a test email to verify your SMTP settings are working correctly.</p>
        </div>
        <div class="footer">
            <p>This is an automated test message.</p>
        </div>
    </div>
</body>
</html>
`

	if err := h.emailService.SendEmailWithConfig(config, req.Email, subject, body); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Test email sent successfully"})
}

// GetAdminAPIKey 获取管理员 API Key 状态
// GET /api/v1/admin/settings/admin-api-key
func (h *SettingHandler) GetAdminAPIKey(c *gin.Context) {
	maskedKey, exists, err := h.settingService.GetAdminAPIKeyStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"exists":     exists,
		"masked_key": maskedKey,
	})
}

// RegenerateAdminAPIKey 生成/重新生成管理员 API Key
// POST /api/v1/admin/settings/admin-api-key/regenerate
func (h *SettingHandler) RegenerateAdminAPIKey(c *gin.Context) {
	key, err := h.settingService.GenerateAdminAPIKey(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"key": key, // 完整 key 只在生成时返回一次
	})
}

// DeleteAdminAPIKey 删除管理员 API Key
// DELETE /api/v1/admin/settings/admin-api-key
func (h *SettingHandler) DeleteAdminAPIKey(c *gin.Context) {
	if err := h.settingService.DeleteAdminAPIKey(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Admin API key deleted"})
}

// GetStreamTimeoutSettings 获取流超时处理配置
// GET /api/v1/admin/settings/stream-timeout
func (h *SettingHandler) GetStreamTimeoutSettings(c *gin.Context) {
	settings, err := h.settingService.GetStreamTimeoutSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.StreamTimeoutSettings{
		Enabled:                settings.Enabled,
		Action:                 settings.Action,
		TempUnschedMinutes:     settings.TempUnschedMinutes,
		ThresholdCount:         settings.ThresholdCount,
		ThresholdWindowMinutes: settings.ThresholdWindowMinutes,
	})
}

// UpdateStreamTimeoutSettingsRequest 更新流超时配置请求
type UpdateStreamTimeoutSettingsRequest struct {
	Enabled                bool   `json:"enabled"`
	Action                 string `json:"action"`
	TempUnschedMinutes     int    `json:"temp_unsched_minutes"`
	ThresholdCount         int    `json:"threshold_count"`
	ThresholdWindowMinutes int    `json:"threshold_window_minutes"`
}

// UpdateStreamTimeoutSettings 更新流超时处理配置
// PUT /api/v1/admin/settings/stream-timeout
func (h *SettingHandler) UpdateStreamTimeoutSettings(c *gin.Context) {
	var req UpdateStreamTimeoutSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.StreamTimeoutSettings{
		Enabled:                req.Enabled,
		Action:                 req.Action,
		TempUnschedMinutes:     req.TempUnschedMinutes,
		ThresholdCount:         req.ThresholdCount,
		ThresholdWindowMinutes: req.ThresholdWindowMinutes,
	}

	if err := h.settingService.SetStreamTimeoutSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetStreamTimeoutSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.StreamTimeoutSettings{
		Enabled:                updatedSettings.Enabled,
		Action:                 updatedSettings.Action,
		TempUnschedMinutes:     updatedSettings.TempUnschedMinutes,
		ThresholdCount:         updatedSettings.ThresholdCount,
		ThresholdWindowMinutes: updatedSettings.ThresholdWindowMinutes,
	})
}

// GenerateWeChatQRCodeRequest 生成微信二维码请求
type GenerateWeChatQRCodeRequest struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
	SceneStr  string `json:"scene_str"` // 场景值，默认 "login"
}

// GenerateWeChatQRCodeResponse 生成微信二维码响应
type GenerateWeChatQRCodeResponse struct {
	QRCodeURL string `json:"qrcode_url"` // 二维码图片 URL
	Ticket    string `json:"ticket"`     // 二维码 ticket
}

// GenerateWeChatQRCode 生成微信公众号永久二维码
// POST /api/v1/admin/settings/wechat/generate-qrcode
func (h *SettingHandler) GenerateWeChatQRCode(c *gin.Context) {
	var req GenerateWeChatQRCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// 处理 AppID 和 AppSecret
	appID := strings.TrimSpace(req.AppID)
	appSecret := strings.TrimSpace(req.AppSecret)

	// 如果请求中未提供，从已保存的设置中获取
	if appID == "" || appSecret == "" {
		savedAppID, savedAppSecret, err := h.settingService.GetWeChatAppCredentials(c.Request.Context())
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if appID == "" {
			appID = savedAppID
		}
		if appSecret == "" {
			appSecret = savedAppSecret
		}
	}

	if appID == "" {
		response.BadRequest(c, "微信 AppID 未配置")
		return
	}
	if appSecret == "" {
		response.BadRequest(c, "微信 AppSecret 未配置")
		return
	}

	// 场景值默认为 "login"
	sceneStr := strings.TrimSpace(req.SceneStr)
	if sceneStr == "" {
		sceneStr = "login"
	}

	// 调用微信 API 生成二维码
	result, err := h.wechatQRCodeService.GeneratePermanentQRCode(c.Request.Context(), appID, appSecret, sceneStr)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 自动保存生成的二维码 URL 到设置中
	if err := h.settingService.SetWeChatQRCodeURL(c.Request.Context(), result.ImageURL); err != nil {
		log.Printf("[WeChat QRCode] Warning: failed to save qrcode url: %v", err)
		// 不阻止返回，二维码已生成成功
	}

	response.Success(c, GenerateWeChatQRCodeResponse{
		QRCodeURL: result.ImageURL,
		Ticket:    result.Ticket,
	})
}

// MaskString 脱敏字符串，保留前后指定位数
// 示例: MaskString("wx0b35f0a1b2fb07e", 6, 4) => "wx0b35****fb07e"
func MaskString(s string, keepPrefix, keepSuffix int) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	length := len(runes)

	if length <= keepPrefix+keepSuffix {
		return s // 太短不脱敏
	}

	masked := make([]rune, length)
	for i := 0; i < length; i++ {
		if i < keepPrefix || i >= length-keepSuffix {
			masked[i] = runes[i]
		} else {
			masked[i] = '*'
		}
	}
	return string(masked)
}

// MaskAppID 脱敏微信AppID（保留前6后4）
func MaskAppID(appID string) string {
	return MaskString(appID, 6, 4)
}

// MaskMchID 脱敏商户号（保留前4后4）
func MaskMchID(mchID string) string {
	return MaskString(mchID, 4, 4)
}

// GetWeChatPayStatus 获取微信支付配置状态（只读，脱敏）
// GET /api/v1/admin/settings/payment/wechat/status
func (h *SettingHandler) GetWeChatPayStatus(c *gin.Context) {
	cfg := h.wechatPayService.GetConfig()

	// 判断是否已配置（配置启用且客户端初始化成功）
	configured := h.wechatPayService.IsEnabled()

	status := &dto.WeChatPayStatus{
		Enabled:     cfg.Enabled,
		AppIDMasked: MaskAppID(cfg.AppID),
		MchIDMasked: MaskMchID(cfg.MchID),
		NotifyURL:   cfg.NotifyURL,
		Configured:  configured,
	}

	response.Success(c, status)
}

// GetRechargeSettings 获取充值业务配置
// GET /api/v1/admin/settings/recharge
func (h *SettingHandler) GetRechargeSettings(c *gin.Context) {
	settings, err := h.settingService.GetRechargeSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.RechargeSettings{
		MinAmount:          settings.MinAmount,
		MaxAmount:          settings.MaxAmount,
		DefaultAmounts:     settings.DefaultAmounts,
		OrderExpireMinutes: settings.OrderExpireMinutes,
		ExchangeRate:       settings.ExchangeRate,
	})
}

// UpdateRechargeSettingsRequest 更新充值配置请求
type UpdateRechargeSettingsRequest struct {
	MinAmount          float64   `json:"min_amount" binding:"required,gt=0"`
	MaxAmount          float64   `json:"max_amount" binding:"required,gt=0"`
	DefaultAmounts     []float64 `json:"default_amounts" binding:"required,min=1"`
	OrderExpireMinutes int       `json:"order_expire_minutes" binding:"required,min=1,max=1440"`
	ExchangeRate       float64   `json:"exchange_rate" binding:"required,gt=0"`
}

// UpdateRechargeSettings 更新充值业务配置
// PUT /api/v1/admin/settings/recharge
func (h *SettingHandler) UpdateRechargeSettings(c *gin.Context) {
	var req UpdateRechargeSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// 验证最小金额 <= 最大金额
	if req.MinAmount > req.MaxAmount {
		response.BadRequest(c, "Minimum amount must be less than or equal to maximum amount")
		return
	}

	// 验证金额选项在范围内
	for _, amount := range req.DefaultAmounts {
		if amount < req.MinAmount || amount > req.MaxAmount {
			response.BadRequest(c, fmt.Sprintf("Amount option %.2f is out of allowed range [%.2f, %.2f]",
				amount, req.MinAmount, req.MaxAmount))
			return
		}
	}

	settings := &service.RechargeSettings{
		MinAmount:          req.MinAmount,
		MaxAmount:          req.MaxAmount,
		DefaultAmounts:     req.DefaultAmounts,
		OrderExpireMinutes: req.OrderExpireMinutes,
		ExchangeRate:       req.ExchangeRate,
	}

	if err := h.settingService.UpdateRechargeSettings(c.Request.Context(), settings); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetRechargeSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.RechargeSettings{
		MinAmount:          updatedSettings.MinAmount,
		MaxAmount:          updatedSettings.MaxAmount,
		DefaultAmounts:     updatedSettings.DefaultAmounts,
		OrderExpireMinutes: updatedSettings.OrderExpireMinutes,
		ExchangeRate:       updatedSettings.ExchangeRate,
	})
}
