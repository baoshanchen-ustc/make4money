package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ReferralHandler handles user referral/invitation endpoints
type ReferralHandler struct {
	referralService *service.ReferralService
}

// NewReferralHandler creates a new ReferralHandler
func NewReferralHandler(referralService *service.ReferralService) *ReferralHandler {
	return &ReferralHandler{
		referralService: referralService,
	}
}

// GetMyReferralInfo handles getting the current user's referral info
// GET /api/v1/referral
func (h *ReferralHandler) GetMyReferralInfo(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// 使用请求的 Origin 作为站点基础 URL（用于构建邀请链接）
	siteBaseURL := c.Request.Header.Get("Origin")

	info, err := h.referralService.GetMyReferralInfo(c.Request.Context(), subject.UserID, siteBaseURL)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, info)
}

// ListMyInvitees handles listing the current user's invited users
// GET /api/v1/referral/invitees
func (h *ReferralHandler) ListMyInvitees(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}

	invitees, result, err := h.referralService.ListMyInvitees(c.Request.Context(), subject.UserID, params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Paginated(c, invitees, result.Total, params.Page, params.PageSize)
}
