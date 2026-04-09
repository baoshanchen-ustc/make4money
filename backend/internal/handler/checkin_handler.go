package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type CheckInHandler struct {
	checkInService *service.CheckInService
}

func NewCheckInHandler(checkInService *service.CheckInService) *CheckInHandler {
	return &CheckInHandler{checkInService: checkInService}
}

// GetStatus handles current user check-in status.
// GET /api/v1/check-in/status
func (h *CheckInHandler) GetStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	status, err := h.checkInService.GetStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.CheckInStatusFromService(status))
}

// CheckIn handles the daily user check-in action.
// POST /api/v1/check-in
func (h *CheckInHandler) CheckIn(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	result, err := h.checkInService.CheckIn(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.CheckInResultFromService(result))
}

// GetHistory returns the current user's recent check-in history.
// GET /api/v1/check-in/history
func (h *CheckInHandler) GetHistory(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	history, err := h.checkInService.GetHistory(c.Request.Context(), subject.UserID, 25)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.CheckInHistoryItemsFromService(history))
}
