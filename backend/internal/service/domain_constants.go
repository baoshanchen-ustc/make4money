package service

// Status constants
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusError    = "error"
	StatusUnused   = "unused"
	StatusUsed     = "used"
	StatusExpired  = "expired"
)

// Role constants
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// Platform constants
const (
	PlatformAnthropic   = "anthropic"
	PlatformOpenAI      = "openai"
	PlatformGemini      = "gemini"
	PlatformAntigravity = "antigravity"
)

// Account type constants
const (
	AccountTypeOAuth      = "oauth"       // OAuth类型账号（full scope: profile + inference）
	AccountTypeSetupToken = "setup-token" // Setup Token类型账号（inference only scope）
	AccountTypeAPIKey     = "apikey"      // API Key类型账号
)

// Redeem type constants
const (
	RedeemTypeBalance      = "balance"
	RedeemTypeConcurrency  = "concurrency"
	RedeemTypeSubscription = "subscription"
)

// PromoCode status constants
const (
	PromoCodeStatusActive   = "active"
	PromoCodeStatusDisabled = "disabled"
)

// Admin adjustment type constants
const (
	AdjustmentTypeAdminBalance     = "admin_balance"     // 管理员调整余额
	AdjustmentTypeAdminConcurrency = "admin_concurrency" // 管理员调整并发数
)

// Group subscription type constants
const (
	SubscriptionTypeStandard     = "standard"     // 标准计费模式（按余额扣费）
	SubscriptionTypeSubscription = "subscription" // 订阅模式（按限额控制）
)

// Subscription status constants
const (
	SubscriptionStatusActive    = "active"
	SubscriptionStatusExpired   = "expired"
	SubscriptionStatusSuspended = "suspended"
)

// LinuxDoConnectSyntheticEmailDomain 是 LinuxDo Connect 用户的合成邮箱后缀（RFC 保留域名）。
const LinuxDoConnectSyntheticEmailDomain = "@linuxdo-connect.invalid"

// Setting keys
const (
	// 注册设置
	SettingKeyRegistrationEnabled  = "registration_enabled"   // 是否开放注册
	SettingKeyEmailVerifyEnabled   = "email_verify_enabled"   // 是否开启邮件验证
	SettingKeyPromoCodeEnabled     = "promo_code_enabled"     // 是否启用优惠码功能
	SettingKeyPasswordResetEnabled = "password_reset_enabled" // 是否启用忘记密码功能（需要先开启邮件验证）

	// 邮件服务设置
	SettingKeySMTPHost     = "smtp_host"      // SMTP服务器地址
	SettingKeySMTPPort     = "smtp_port"      // SMTP端口
	SettingKeySMTPUsername = "smtp_username"  // SMTP用户名
	SettingKeySMTPPassword = "smtp_password"  // SMTP密码（加密存储）
	SettingKeySMTPFrom     = "smtp_from"      // 发件人地址
	SettingKeySMTPFromName = "smtp_from_name" // 发件人名称
	SettingKeySMTPUseTLS   = "smtp_use_tls"   // 是否使用TLS

	// Cloudflare Turnstile 设置
	SettingKeyTurnstileEnabled   = "turnstile_enabled"    // 是否启用 Turnstile 验证
	SettingKeyTurnstileSiteKey   = "turnstile_site_key"   // Turnstile Site Key
	SettingKeyTurnstileSecretKey = "turnstile_secret_key" // Turnstile Secret Key

	// TOTP 双因素认证设置
	SettingKeyTotpEnabled = "totp_enabled" // 是否启用 TOTP 2FA 功能

	// LinuxDo Connect OAuth 登录设置
	SettingKeyLinuxDoConnectEnabled      = "linuxdo_connect_enabled"
	SettingKeyLinuxDoConnectClientID     = "linuxdo_connect_client_id"
	SettingKeyLinuxDoConnectClientSecret = "linuxdo_connect_client_secret"
	SettingKeyLinuxDoConnectRedirectURL  = "linuxdo_connect_redirect_url"

	// 微信公众号验证码登录设置
	SettingKeyWeChatAuthEnabled      = "wechat_auth_enabled"       // 是否启用微信登录
	SettingKeyWeChatServerAddress    = "wechat_server_address"     // 微信服务器地址
	SettingKeyWeChatServerToken      = "wechat_server_token"       // 微信服务器认证令牌
	SettingKeyWeChatAccountQRCodeURL  = "wechat_account_qrcode_url"  // 微信公众号二维码URL
	SettingKeyWeChatAccountQRCodeData = "wechat_account_qrcode_data" // 微信公众号二维码图片数据 (Base64)
	SettingKeyWeChatAppID             = "wechat_app_id"              // 微信公众号 AppID
	SettingKeyWeChatAppSecret         = "wechat_app_secret"          // 微信公众号 AppSecret

	// OEM设置
	SettingKeySiteName                    = "site_name"                     // 网站名称
	SettingKeySiteLogo                    = "site_logo"                     // 网站Logo (base64)
	SettingKeySiteLogoDark                = "site_logo_dark"                // 网站Logo 夜间模式 (base64)
	SettingKeySiteSubtitle                = "site_subtitle"                 // 网站副标题
	SettingKeyAPIBaseURL                  = "api_base_url"                  // API端点地址（用于客户端配置和导入）
	SettingKeyContactInfo                 = "contact_info"                  // 客服联系方式
	SettingKeyContactQRCodeWechat         = "contact_qrcode_wechat"         // 客服微信二维码 (base64)
	SettingKeyContactQRCodeGroup          = "contact_qrcode_group"          // 客服社群二维码 (base64)
	SettingKeyDocURL                      = "doc_url"                       // 文档链接
	SettingKeyHomeContent                 = "home_content"                  // 首页内容（支持 Markdown/HTML，或 URL 作为 iframe src）
	SettingKeyHideCcsImportButton         = "hide_ccs_import_button"        // 是否隐藏 API Keys 页面的导入 CCS 按钮
	SettingKeyPurchaseSubscriptionEnabled = "purchase_subscription_enabled" // 是否展示"购买订阅"页面入口
	SettingKeyPurchaseSubscriptionURL     = "purchase_subscription_url"     // "购买订阅"页面 URL（作为 iframe src）

	// 默认配置
	SettingKeyDefaultConcurrency = "default_concurrency" // 新用户默认并发量
	SettingKeyDefaultBalance     = "default_balance"     // 新用户默认余额

	// 管理员 API Key
	SettingKeyAdminAPIKey = "admin_api_key" // 全局管理员 API Key（用于外部系统集成）

	// Gemini 配额策略（JSON）
	SettingKeyGeminiQuotaPolicy = "gemini_quota_policy"

	// Model fallback settings
	SettingKeyEnableModelFallback      = "enable_model_fallback"
	SettingKeyFallbackModelAnthropic   = "fallback_model_anthropic"
	SettingKeyFallbackModelOpenAI      = "fallback_model_openai"
	SettingKeyFallbackModelGemini      = "fallback_model_gemini"
	SettingKeyFallbackModelAntigravity = "fallback_model_antigravity"

	// Request identity patch (Claude -> Gemini systemInstruction injection)
	SettingKeyEnableIdentityPatch = "enable_identity_patch"
	SettingKeyIdentityPatchPrompt = "identity_patch_prompt"

	// =========================
	// Ops Monitoring (vNext)
	// =========================

	// SettingKeyOpsMonitoringEnabled is a DB-backed soft switch to enable/disable ops module at runtime.
	SettingKeyOpsMonitoringEnabled = "ops_monitoring_enabled"

	// SettingKeyOpsRealtimeMonitoringEnabled controls realtime features (e.g. WS/QPS push).
	SettingKeyOpsRealtimeMonitoringEnabled = "ops_realtime_monitoring_enabled"

	// SettingKeyOpsQueryModeDefault controls the default query mode for ops dashboard (auto/raw/preagg).
	SettingKeyOpsQueryModeDefault = "ops_query_mode_default"

	// SettingKeyOpsEmailNotificationConfig stores JSON config for ops email notifications.
	SettingKeyOpsEmailNotificationConfig = "ops_email_notification_config"

	// SettingKeyOpsAlertRuntimeSettings stores JSON config for ops alert evaluator runtime settings.
	SettingKeyOpsAlertRuntimeSettings = "ops_alert_runtime_settings"

	// SettingKeyOpsMetricsIntervalSeconds controls the ops metrics collector interval (>=60).
	SettingKeyOpsMetricsIntervalSeconds = "ops_metrics_interval_seconds"

	// SettingKeyOpsAdvancedSettings stores JSON config for ops advanced settings (data retention, aggregation).
	SettingKeyOpsAdvancedSettings = "ops_advanced_settings"

	// =========================
	// Stream Timeout Handling
	// =========================

	// SettingKeyStreamTimeoutSettings stores JSON config for stream timeout handling.
	SettingKeyStreamTimeoutSettings = "stream_timeout_settings"

	// =========================
	// Usage Report Settings
	// =========================

	// SettingKeyUsageReportGlobalEnabled controls whether usage report emails are globally enabled.
	SettingKeyUsageReportGlobalEnabled = "usage_report_global_enabled"

	// SettingKeyUsageReportTargetScope controls who receives usage reports: "all", "active_today", or "opted_in".
	SettingKeyUsageReportTargetScope = "usage_report_target_scope"

	// SettingKeyUsageReportGlobalSchedule is the global send time (HH:MM) for non-opted_in modes.
	SettingKeyUsageReportGlobalSchedule = "usage_report_global_schedule"

	// =========================
	// Recharge Settings
	// =========================

	// SettingKeyRechargeMinAmount is the minimum recharge amount in yuan.
	SettingKeyRechargeMinAmount = "recharge.min_amount"

	// SettingKeyRechargeMaxAmount is the maximum recharge amount in yuan.
	SettingKeyRechargeMaxAmount = "recharge.max_amount"

	// SettingKeyRechargeDefaultAmounts stores JSON array of quick amount options.
	SettingKeyRechargeDefaultAmounts = "recharge.default_amounts"

	// SettingKeyRechargeOrderExpireMinutes is the order expiry time in minutes.
	SettingKeyRechargeOrderExpireMinutes = "recharge.order_expire_minutes"

	// SettingKeyRechargeExchangeRate is the CNY to credit exchange rate.
	SettingKeyRechargeExchangeRate = "recharge.exchange_rate"
)

// AdminAPIKeyPrefix is the prefix for admin API keys (distinct from user "sk-" keys).
const AdminAPIKeyPrefix = "admin-"

// Usage report target scope constants
const (
	UsageReportScopeAll         = "all"          // 全部绑定邮箱的用户
	UsageReportScopeActiveToday = "active_today" // 当天有使用的用户
	UsageReportScopeOptedIn     = "opted_in"     // 仅启用了报告功能的用户
)

// Recharge defaults
const (
	DefaultRechargeMinAmount          = 1.0    // 最小充值金额（元）
	DefaultRechargeMaxAmount          = 1000.0 // 最大充值金额（元）
	DefaultRechargeOrderExpireMinutes = 120    // 订单过期时间（分钟）
	DefaultRechargeExchangeRate       = 1.0    // 默认汇率（1:1，人民币直接转换为额度）
)

// DefaultRechargeAmounts is the default quick amount options
var DefaultRechargeAmounts = []float64{10, 50, 100, 200, 500}
