package dto

// SystemSettings represents the admin settings API response payload.
type SystemSettings struct {
	RegistrationEnabled         bool `json:"registration_enabled"`
	EmailVerifyEnabled          bool `json:"email_verify_enabled"`
	PromoCodeEnabled            bool `json:"promo_code_enabled"`
	PasswordResetEnabled        bool `json:"password_reset_enabled"`
	TotpEnabled                 bool `json:"totp_enabled"`                   // TOTP 双因素认证
	TotpEncryptionKeyConfigured bool `json:"totp_encryption_key_configured"` // TOTP 加密密钥是否已配置

	SMTPHost               string `json:"smtp_host"`
	SMTPPort               int    `json:"smtp_port"`
	SMTPUsername           string `json:"smtp_username"`
	SMTPPasswordConfigured bool   `json:"smtp_password_configured"`
	SMTPFrom               string `json:"smtp_from_email"`
	SMTPFromName           string `json:"smtp_from_name"`
	SMTPUseTLS             bool   `json:"smtp_use_tls"`

	TurnstileEnabled             bool   `json:"turnstile_enabled"`
	TurnstileSiteKey             string `json:"turnstile_site_key"`
	TurnstileSecretKeyConfigured bool   `json:"turnstile_secret_key_configured"`

	LinuxDoConnectEnabled                bool   `json:"linuxdo_connect_enabled"`
	LinuxDoConnectClientID               string `json:"linuxdo_connect_client_id"`
	LinuxDoConnectClientSecretConfigured bool   `json:"linuxdo_connect_client_secret_configured"`
	LinuxDoConnectRedirectURL            string `json:"linuxdo_connect_redirect_url"`

	// 微信公众号验证码登录
	WeChatAuthEnabled           bool   `json:"wechat_auth_enabled"`
	WeChatAccountType           string `json:"wechat_account_type"`
	WeChatServerAddress         string `json:"wechat_server_address"`
	WeChatServerTokenConfigured bool   `json:"wechat_server_token_configured"`
	WeChatAccountQRCodeURL      string `json:"wechat_account_qrcode_url"`
	WeChatAccountQRCodeData     string `json:"wechat_account_qrcode_data"`
	WeChatAppID                 string `json:"wechat_app_id"`
	WeChatAppSecretConfigured   bool   `json:"wechat_app_secret_configured"`

	SiteName                    string `json:"site_name"`
	SiteLogo                    string `json:"site_logo"`
	SiteLogoDark                string `json:"site_logo_dark"`
	SiteSubtitle                string `json:"site_subtitle"`
	APIBaseURL                  string `json:"api_base_url"`
	ContactInfo                 string `json:"contact_info"`
	ContactQRCodeWechat         string `json:"contact_qrcode_wechat"`
	ContactQRCodeGroup          string `json:"contact_qrcode_group"`
	DocURL                      string `json:"doc_url"`
	HomeContent                 string `json:"home_content"`
	HideCcsImportButton         bool   `json:"hide_ccs_import_button"`
	PurchaseSubscriptionEnabled bool   `json:"purchase_subscription_enabled"`
	PurchaseSubscriptionURL     string `json:"purchase_subscription_url"`

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
	OpsMonitoringEnabled         bool   `json:"ops_monitoring_enabled"`
	OpsRealtimeMonitoringEnabled bool   `json:"ops_realtime_monitoring_enabled"`
	OpsQueryModeDefault          string `json:"ops_query_mode_default"`
	OpsMetricsIntervalSeconds    int    `json:"ops_metrics_interval_seconds"`

	// Usage report settings
	UsageReportGlobalEnabled  bool   `json:"usage_report_global_enabled"`
	UsageReportTargetScope    string `json:"usage_report_target_scope"`
	UsageReportGlobalSchedule string `json:"usage_report_global_schedule"`
}

type PublicSettings struct {
	RegistrationEnabled         bool   `json:"registration_enabled"`
	EmailVerifyEnabled          bool   `json:"email_verify_enabled"`
	PromoCodeEnabled            bool   `json:"promo_code_enabled"`
	PasswordResetEnabled        bool   `json:"password_reset_enabled"`
	TotpEnabled                 bool   `json:"totp_enabled"` // TOTP 双因素认证
	TurnstileEnabled            bool   `json:"turnstile_enabled"`
	TurnstileSiteKey            string `json:"turnstile_site_key"`
	SiteName                    string `json:"site_name"`
	SiteLogo                    string `json:"site_logo"`
	SiteLogoDark                string `json:"site_logo_dark"`
	SiteSubtitle                string `json:"site_subtitle"`
	APIBaseURL                  string `json:"api_base_url"`
	ContactInfo                 string `json:"contact_info"`
	ContactQRCodeWechat         string `json:"contact_qrcode_wechat"`
	ContactQRCodeGroup          string `json:"contact_qrcode_group"`
	DocURL                      string `json:"doc_url"`
	HomeContent                 string `json:"home_content"`
	HideCcsImportButton         bool   `json:"hide_ccs_import_button"`
	PurchaseSubscriptionEnabled bool   `json:"purchase_subscription_enabled"`
	PurchaseSubscriptionURL     string `json:"purchase_subscription_url"`
	LinuxDoOAuthEnabled         bool   `json:"linuxdo_oauth_enabled"`
	WeChatAuthEnabled           bool   `json:"wechat_auth_enabled"`
	WeChatAccountType           string `json:"wechat_account_type,omitempty"`
	WeChatAccountQRCodeURL      string `json:"wechat_account_qrcode_url"`
	WeChatAccountQRCodeData     string `json:"wechat_account_qrcode_data"`
	Version                     string `json:"version"`
}

// StreamTimeoutSettings 流超时处理配置 DTO
type StreamTimeoutSettings struct {
	Enabled                bool   `json:"enabled"`
	Action                 string `json:"action"`
	TempUnschedMinutes     int    `json:"temp_unsched_minutes"`
	ThresholdCount         int    `json:"threshold_count"`
	ThresholdWindowMinutes int    `json:"threshold_window_minutes"`
}

// WeChatPayStatus 微信支付配置状态（只读，脱敏后返回）
type WeChatPayStatus struct {
	Enabled     bool   `json:"enabled"`       // 是否启用
	AppIDMasked string `json:"app_id_masked"` // 脱敏后的AppID
	MchIDMasked string `json:"mch_id_masked"` // 脱敏后的商户号
	NotifyURL   string `json:"notify_url"`    // 回调地址（可显示）
	Configured  bool   `json:"configured"`    // 是否已配置（所有必填字段都有值）
}

// RechargeSettings 充值业务配置 DTO
type RechargeSettings struct {
	MinAmount          float64   `json:"min_amount"`           // 最小充值金额（元）
	MaxAmount          float64   `json:"max_amount"`           // 最大充值金额（元）
	DefaultAmounts     []float64 `json:"default_amounts"`      // 默认金额选项
	OrderExpireMinutes int       `json:"order_expire_minutes"` // 订单过期时间（分钟）
	ExchangeRate       float64   `json:"exchange_rate"`        // 人民币兑额度汇率（例如 7.0 表示 ¥7 = $1）
}
