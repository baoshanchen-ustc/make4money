package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ProvisionHandler handles user provisioning for service-to-service calls
type ProvisionHandler struct {
	provisionService *service.ProvisionService
}

// NewProvisionHandler creates a new ProvisionHandler
func NewProvisionHandler(provisionService *service.ProvisionService) *ProvisionHandler {
	return &ProvisionHandler{
		provisionService: provisionService,
	}
}

// provisionRequest is the request body for the provision endpoint
type provisionRequest struct {
	Email  string `json:"email" binding:"required,email"`
	Source string `json:"source" binding:"required"`
}

// Provision handles POST /api/provision
func (h *ProvisionHandler) Provision(c *gin.Context) {
	var req provisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: email and source are required")
		return
	}

	result, err := h.provisionService.Provision(c.Request.Context(), service.ProvisionRequest{
		Email:  req.Email,
		Source: req.Source,
	})
	if err != nil {
		if response.ErrorFrom(c, err) {
			return
		}
		response.InternalError(c, "provision failed")
		return
	}

	response.Success(c, result)
}
