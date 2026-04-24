package admin

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type CardsIssueHandler struct {
	settingService *service.SettingService
}

type UpdateCardsIssueConfigRequest struct {
	Enabled          bool   `json:"enabled"`
	ResponseTemplate string `json:"response_template"`
}

func NewCardsIssueHandler(settingService *service.SettingService) *CardsIssueHandler {
	return &CardsIssueHandler{settingService: settingService}
}

func (h *CardsIssueHandler) GetConfig(c *gin.Context) {
	cfg, err := h.settingService.GetCardsIssueAdminConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *CardsIssueHandler) UpdateConfig(c *gin.Context) {
	var req UpdateCardsIssueConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if len(strings.TrimSpace(req.ResponseTemplate)) > 4000 {
		response.BadRequest(c, "response_template is too long (max 4000 characters)")
		return
	}
	if err := h.settingService.UpdateCardsIssueConfig(c.Request.Context(), service.UpdateCardsIssueConfigInput{
		Enabled:          req.Enabled,
		ResponseTemplate: req.ResponseTemplate,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	cfg, err := h.settingService.GetCardsIssueAdminConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *CardsIssueHandler) RegenerateKey(c *gin.Context) {
	key, err := h.settingService.GenerateCardsIssueBearerKey(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"key": key})
}

func (h *CardsIssueHandler) DeleteKey(c *gin.Context) {
	if err := h.settingService.DeleteCardsIssueBearerKey(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Cards issue bearer key deleted"})
}
