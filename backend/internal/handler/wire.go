package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/handler/recharge"
	"github.com/Wei-Shaw/sub2api/internal/handler/subscription"
	"github.com/Wei-Shaw/sub2api/internal/handler/webhook"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/google/wire"
)

// ProvideAdminHandlers creates the AdminHandlers struct
func ProvideAdminHandlers(
	dashboardHandler *admin.DashboardHandler,
	userHandler *admin.UserHandler,
	groupHandler *admin.GroupHandler,
	accountHandler *admin.AccountHandler,
	announcementHandler *admin.AnnouncementHandler,
	oauthHandler *admin.OAuthHandler,
	openaiOAuthHandler *admin.OpenAIOAuthHandler,
	geminiOAuthHandler *admin.GeminiOAuthHandler,
	antigravityOAuthHandler *admin.AntigravityOAuthHandler,
	proxyHandler *admin.ProxyHandler,
	redeemHandler *admin.RedeemHandler,
	promoHandler *admin.PromoHandler,
	settingHandler *admin.SettingHandler,
	opsHandler *admin.OpsHandler,
	systemHandler *admin.SystemHandler,
	subscriptionHandler *admin.SubscriptionHandler,
	usageHandler *admin.UsageHandler,
	userAttributeHandler *admin.UserAttributeHandler,
	rechargeHandler *admin.RechargeHandler,
	errorPassthroughHandler *admin.ErrorPassthroughHandler,
) *AdminHandlers {
	return &AdminHandlers{
		Dashboard:        dashboardHandler,
		User:             userHandler,
		Group:            groupHandler,
		Account:          accountHandler,
		Announcement:     announcementHandler,
		OAuth:            oauthHandler,
		OpenAIOAuth:      openaiOAuthHandler,
		GeminiOAuth:      geminiOAuthHandler,
		AntigravityOAuth: antigravityOAuthHandler,
		Proxy:            proxyHandler,
		Redeem:           redeemHandler,
		Promo:            promoHandler,
		Setting:          settingHandler,
		Ops:              opsHandler,
		System:           systemHandler,
		Subscription:     subscriptionHandler,
		Usage:            usageHandler,
		UserAttribute:    userAttributeHandler,
		Recharge:         rechargeHandler,
		ErrorPassthrough: errorPassthroughHandler,
	}
}

// ProvideSystemHandler creates admin.SystemHandler with UpdateService
func ProvideSystemHandler(updateService *service.UpdateService) *admin.SystemHandler {
	return admin.NewSystemHandler(updateService)
}

// ProvideSettingHandler creates SettingHandler with version from BuildInfo
func ProvideSettingHandler(settingService *service.SettingService, buildInfo BuildInfo) *SettingHandler {
	return NewSettingHandler(settingService, buildInfo.Version)
}

// ProvideHandlers creates the Handlers struct
func ProvideHandlers(
	authHandler *AuthHandler,
	userHandler *UserHandler,
	apiKeyHandler *APIKeyHandler,
	usageHandler *UsageHandler,
	redeemHandler *RedeemHandler,
	subscriptionHandler *SubscriptionHandler,
	announcementHandler *AnnouncementHandler,
	adminHandlers *AdminHandlers,
	gatewayHandler *GatewayHandler,
	openaiGatewayHandler *OpenAIGatewayHandler,
	settingHandler *SettingHandler,
	totpHandler *TotpHandler,
	usageReportHandler *UserUsageReportHandler,
	rechargeHandler *recharge.RechargeHandler,
	wechatPayWebhookHandler *webhook.WeChatPayWebhookHandler,
	subscriptionPlanHandler *subscription.SubscriptionPlanHandler,
) *Handlers {
	return &Handlers{
		Auth:             authHandler,
		User:             userHandler,
		APIKey:           apiKeyHandler,
		Usage:            usageHandler,
		Redeem:           redeemHandler,
		Subscription:     subscriptionHandler,
		Announcement:     announcementHandler,
		Admin:            adminHandlers,
		Gateway:          gatewayHandler,
		OpenAIGateway:    openaiGatewayHandler,
		Setting:          settingHandler,
		Totp:             totpHandler,
		UsageReport:      usageReportHandler,
		Recharge:         rechargeHandler,
		WeChatPayWebhook: wechatPayWebhookHandler,
		SubscriptionPlan: subscriptionPlanHandler,
	}
}

// ProviderSet is the Wire provider set for all handlers
var ProviderSet = wire.NewSet(
	// Top-level handlers
	NewAuthHandler,
	NewUserHandler,
	NewAPIKeyHandler,
	NewUsageHandler,
	NewRedeemHandler,
	NewSubscriptionHandler,
	NewAnnouncementHandler,
	NewGatewayHandler,
	NewOpenAIGatewayHandler,
	NewTotpHandler,
	NewUserUsageReportHandler,
	ProvideSettingHandler,
	recharge.NewRechargeHandler,
	webhook.NewWeChatPayWebhookHandler,
	subscription.NewSubscriptionPlanHandler,

	// Admin handlers
	admin.NewDashboardHandler,
	admin.NewUserHandler,
	admin.NewGroupHandler,
	admin.NewAccountHandler,
	admin.NewAnnouncementHandler,
	admin.NewOAuthHandler,
	admin.NewOpenAIOAuthHandler,
	admin.NewGeminiOAuthHandler,
	admin.NewAntigravityOAuthHandler,
	admin.NewProxyHandler,
	admin.NewRedeemHandler,
	admin.NewPromoHandler,
	admin.NewSettingHandler,
	admin.NewOpsHandler,
	ProvideSystemHandler,
	admin.NewSubscriptionHandler,
	admin.NewUsageHandler,
	admin.NewUserAttributeHandler,
	admin.NewRechargeHandler,
	admin.NewErrorPassthroughHandler,

	// AdminHandlers and Handlers constructors
	ProvideAdminHandlers,
	ProvideHandlers,
)
