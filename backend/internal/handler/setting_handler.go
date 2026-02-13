package handler

import (
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SettingHandler 公开设置处理器（无需认证）
type SettingHandler struct {
	settingService *service.SettingService
	version        string
}

// NewSettingHandler 创建公开设置处理器
func NewSettingHandler(settingService *service.SettingService, version string) *SettingHandler {
	return &SettingHandler{
		settingService: settingService,
		version:        version,
	}
}

// GetPublicSettings 获取公开设置
// GET /api/v1/settings/public
func (h *SettingHandler) GetPublicSettings(c *gin.Context) {
	settings, err := h.settingService.GetPublicSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.PublicSettings{
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
		Version:                     h.version,
		InstallGuideVideos:          settings.InstallGuideVideos,
		HomeTestimonials:            settings.HomeTestimonials,
		BalanceLotExpiryDays:        settings.BalanceLotExpiryDays,
		HomeGalleryEnabled:          settings.HomeGalleryEnabled,
	})
}

// GetHomeGallery 获取首页画廊数据
// GET /api/v1/settings/gallery
func (h *SettingHandler) GetHomeGallery(c *gin.Context) {
	// Allow client/CDN caching since gallery data changes infrequently
	c.Header("Cache-Control", "public, max-age=300")

	data, err := h.settingService.GetHomeGallery(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if data == "" {
		response.Success(c, nil)
		return
	}

	if !json.Valid([]byte(data)) {
		response.Success(c, nil)
		return
	}
	response.Success(c, json.RawMessage(data))
}
