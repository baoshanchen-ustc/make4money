package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ReferralHandler handles admin referral management endpoints
type ReferralHandler struct {
	referralService *service.ReferralService
}

// NewReferralHandler creates a new admin ReferralHandler
func NewReferralHandler(referralService *service.ReferralService) *ReferralHandler {
	return &ReferralHandler{
		referralService: referralService,
	}
}

// GetPlatformStats handles getting platform-wide referral statistics
// GET /api/v1/admin/referral/stats
func (h *ReferralHandler) GetPlatformStats(c *gin.Context) {
	stats, err := h.referralService.GetPlatformStats(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}
