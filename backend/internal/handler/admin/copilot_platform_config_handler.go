package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CopilotPlatformConfigHandler 处理 Copilot 平台配置的 HTTP 请求。
type CopilotPlatformConfigHandler struct {
	svc *service.CopilotPlatformConfigService
}

func NewCopilotPlatformConfigHandler(svc *service.CopilotPlatformConfigService) *CopilotPlatformConfigHandler {
	return &CopilotPlatformConfigHandler{svc: svc}
}

// copilotPlatformConfigResponse 是返回给前端的 JSON 结构。
type copilotPlatformConfigResponse struct {
	PlanType        string            `json:"plan_type"`
	MaxOutputTokens *int64            `json:"max_output_tokens"`
	MaxBodyKB       *int              `json:"max_body_kb"`
	ModelMapping    map[string]string `json:"model_mapping"`
	ModelWhitelist  []string          `json:"model_whitelist"`
}

// updateCopilotPlatformConfigRequest 是 PUT 请求体。
type updateCopilotPlatformConfigRequest struct {
	MaxOutputTokens *int64            `json:"max_output_tokens"`
	MaxBodyKB       *int              `json:"max_body_kb"`
	ModelMapping    map[string]string `json:"model_mapping"`
	ModelWhitelist  []string          `json:"model_whitelist"`
}

// List 处理 GET /admin/copilot/platform-config
func (h *CopilotPlatformConfigHandler) List(c *gin.Context) {
	entries, err := h.svc.GetAll(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]copilotPlatformConfigResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, entryToConfigResponse(e))
	}
	response.Success(c, out)
}

// Update 处理 PUT /admin/copilot/platform-config/:plan_type
func (h *CopilotPlatformConfigHandler) Update(c *gin.Context) {
	planType := c.Param("plan_type")
	if !isValidCopilotPlanType(planType) {
		response.BadRequest(c, "invalid plan_type, must be one of: individual_free, individual_pro, individual_pro_plus, business, enterprise")
		return
	}

	var req updateCopilotPlatformConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	patch := service.CopilotPlatformConfigPatch{
		MaxOutputTokens:    req.MaxOutputTokens,
		MaxBodyKB:          req.MaxBodyKB,
		ModelMapping:       req.ModelMapping,
		ModelWhitelist:     req.ModelWhitelist,
		SetMaxOutputTokens: true,
		SetMaxBodyKB:       true,
		SetModelMapping:    true,
		SetModelWhitelist:  true,
	}

	updated, err := h.svc.UpdateByPlanType(c.Request.Context(), planType, patch)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, entryToConfigResponse(*updated))
}

func entryToConfigResponse(e service.CopilotPlatformConfigEntry) copilotPlatformConfigResponse {
	mapping := e.ModelMapping
	if mapping == nil {
		mapping = map[string]string{}
	}
	whitelist := e.ModelWhitelist
	if whitelist == nil {
		whitelist = []string{}
	}
	return copilotPlatformConfigResponse{
		PlanType:        e.PlanType,
		MaxOutputTokens: e.MaxOutputTokens,
		MaxBodyKB:       e.MaxBodyKB,
		ModelMapping:    mapping,
		ModelWhitelist:  whitelist,
	}
}

func isValidCopilotPlanType(planType string) bool {
	for _, valid := range service.AllCopilotPlanTypes {
		if planType == valid {
			return true
		}
	}
	return false
}
