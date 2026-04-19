package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UserHandler handles user-related requests
type UserHandler struct {
	userService  *service.UserService
	authService  *service.AuthService
	emailService *service.EmailService
	emailCache   service.EmailCache
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(
	userService *service.UserService,
	authService *service.AuthService,
	emailService *service.EmailService,
	emailCache service.EmailCache,
) *UserHandler {
	return &UserHandler{
		userService:  userService,
		authService:  authService,
		emailService: emailService,
		emailCache:   emailCache,
	}
}

// ChangePasswordRequest represents the change password request payload
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// UpdateProfileRequest represents the update profile request payload
type UpdateProfileRequest struct {
	Username               *string  `json:"username"`
	BalanceNotifyEnabled   *bool    `json:"balance_notify_enabled"`
	BalanceNotifyThreshold *float64 `json:"balance_notify_threshold"`
}

type ConfirmAccountBindingRequest struct {
	PendingAuthToken   string `json:"pending_auth_token,omitempty"`
	PendingOAuthToken  string `json:"pending_oauth_token,omitempty"`
	PendingAuthToken2  string `json:"pendingAuthToken,omitempty"`
	PendingOAuthToken2 string `json:"pendingOAuthToken,omitempty"`
	Email              string `json:"email,omitempty"`
	VerifyCode         string `json:"verify_code,omitempty"`
	Password           string `json:"password,omitempty"`
}

type CompleteIdentityAdoptionDecisionRequest struct {
	PendingAuthToken string `json:"pending_auth_token" binding:"required"`
	AdoptDisplayName bool   `json:"adopt_display_name"`
	AdoptAvatar      bool   `json:"adopt_avatar"`
}

// GetProfile handles getting user profile
// GET /api/v1/users/me
func (h *UserHandler) GetProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	userData, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.UserFromServiceWithBindingAvailability(
		userData,
		h.userService.GetExternalIdentityAvailability(c.Request.Context()),
	))
}

// ChangePassword handles changing user password
// POST /api/v1/users/me/password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.ChangePasswordRequest{
		CurrentPassword: req.OldPassword,
		NewPassword:     req.NewPassword,
	}
	err := h.userService.ChangePassword(c.Request.Context(), subject.UserID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Password changed successfully"})
}

// UpdateProfile handles updating user profile
// PUT /api/v1/users/me
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.UpdateProfileRequest{
		Username:               req.Username,
		BalanceNotifyEnabled:   req.BalanceNotifyEnabled,
		BalanceNotifyThreshold: req.BalanceNotifyThreshold,
	}
	updatedUser, err := h.userService.UpdateProfile(c.Request.Context(), subject.UserID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.UserFromServiceWithBindingAvailability(
		updatedUser,
		h.userService.GetExternalIdentityAvailability(c.Request.Context()),
	))
}

// ConfirmAccountBinding binds a validated third-party identity to the current user.
// POST /api/v1/user/account-bindings/:provider
func (h *UserHandler) ConfirmAccountBinding(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.authService == nil {
		response.InternalError(c, "Auth service not configured")
		return
	}

	var req ConfirmAccountBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if strings.EqualFold(strings.TrimSpace(c.Param("provider")), "email") {
		userData, err := h.authService.BindEmailIdentity(
			c.Request.Context(),
			subject.UserID,
			req.Email,
			req.VerifyCode,
			req.Password,
		)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}

		response.Success(c, gin.H{"user": dto.UserFromServiceWithBindingAvailability(
			userData,
			h.userService.GetExternalIdentityAvailability(c.Request.Context()),
		)})
		return
	}

	pendingAuthToken := req.PendingAuthToken
	if pendingAuthToken == "" {
		pendingAuthToken = req.PendingOAuthToken
	}
	if pendingAuthToken == "" {
		pendingAuthToken = req.PendingAuthToken2
	}
	if pendingAuthToken == "" {
		pendingAuthToken = req.PendingOAuthToken2
	}
	if pendingAuthToken == "" {
		response.BadRequest(c, "Invalid request: pending_auth_token is required")
		return
	}

	session, err := h.authService.GetPendingAuthSessionForProgress(c.Request.Context(), pendingAuthToken, nil)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if session.Intent != service.PendingAuthIntentBindCurrentUser {
		response.BadRequest(c, "Invalid pending auth session intent")
		return
	}
	if session.TargetUserID != nil && *session.TargetUserID != subject.UserID {
		response.ErrorFrom(c, service.ErrPendingAuthTargetMismatch)
		return
	}
	if !strings.EqualFold(strings.TrimSpace(session.ProviderType), strings.TrimSpace(c.Param("provider"))) {
		response.BadRequest(c, "Pending auth session provider does not match route provider")
		return
	}

	if _, err := h.authService.CompletePendingAuthSessionBind(c.Request.Context(), pendingAuthToken, subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	userData, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"user": dto.UserFromServiceWithBindingAvailability(
		userData,
		h.userService.GetExternalIdentityAvailability(c.Request.Context()),
	)})
}

// CompleteIdentityAdoptionDecision stores the profile adoption choice for the current pending bind flow.
// POST /api/v1/user/account-bindings/:provider/adoption-decision
func (h *UserHandler) CompleteIdentityAdoptionDecision(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.authService == nil {
		response.InternalError(c, "Auth service not configured")
		return
	}

	var req CompleteIdentityAdoptionDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	session, err := h.authService.GetPendingAuthSessionForProgress(c.Request.Context(), req.PendingAuthToken, nil)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if session.Intent != service.PendingAuthIntentBindCurrentUser {
		response.BadRequest(c, "Invalid pending auth session intent")
		return
	}
	if !strings.EqualFold(strings.TrimSpace(session.ProviderType), strings.TrimSpace(c.Param("provider"))) {
		response.BadRequest(c, "Pending auth session provider does not match route provider")
		return
	}
	if session.TargetUserID != nil && *session.TargetUserID != subject.UserID {
		response.ErrorFrom(c, service.ErrPendingAuthTargetMismatch)
		return
	}

	updated, err := h.authService.UpdatePendingAuthSessionAdoptionDecision(
		c.Request.Context(),
		req.PendingAuthToken,
		req.AdoptDisplayName,
		req.AdoptAvatar,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"pending_auth_token": req.PendingAuthToken,
		"provider":           updated.ProviderType,
		"intent":             updated.Intent,
		"adopt_display_name": req.AdoptDisplayName,
		"adopt_avatar":       req.AdoptAvatar,
	})
}

// DeleteAccountBinding removes a third-party login binding from the current user.
// DELETE /api/v1/user/account-bindings/:provider
func (h *UserHandler) DeleteAccountBinding(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	updatedUser, err := h.userService.DeleteAccountBinding(c.Request.Context(), subject.UserID, c.Param("provider"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"user": dto.UserFromServiceWithBindingAvailability(
		updatedUser,
		h.userService.GetExternalIdentityAvailability(c.Request.Context()),
	)})
}

// SendNotifyEmailCodeRequest represents the request to send notify email verification code
type SendNotifyEmailCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// SendNotifyEmailCode sends verification code to extra notification email
// POST /api/v1/user/notify-email/send-code
func (h *UserHandler) SendNotifyEmailCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req SendNotifyEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := h.userService.SendNotifyEmailCode(c.Request.Context(), subject.UserID, req.Email, h.emailService, h.emailCache)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Verification code sent successfully"})
}

// VerifyNotifyEmailRequest represents the request to verify and add notify email
type VerifyNotifyEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

// VerifyNotifyEmail verifies code and adds email to notification list
// POST /api/v1/user/notify-email/verify
func (h *UserHandler) VerifyNotifyEmail(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req VerifyNotifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := h.userService.VerifyAndAddNotifyEmail(c.Request.Context(), subject.UserID, req.Email, req.Code, h.emailCache)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Return updated user
	updatedUser, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.UserFromService(updatedUser))
}

// RemoveNotifyEmailRequest represents the request to remove a notify email
type RemoveNotifyEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// RemoveNotifyEmail removes email from notification list
// DELETE /api/v1/user/notify-email
func (h *UserHandler) RemoveNotifyEmail(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req RemoveNotifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := h.userService.RemoveNotifyEmail(c.Request.Context(), subject.UserID, req.Email)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Return updated user
	updatedUser, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.UserFromService(updatedUser))
}

// ToggleNotifyEmailRequest represents the request to toggle a notify email's disabled state
type ToggleNotifyEmailRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Disabled bool   `json:"disabled"`
}

// ToggleNotifyEmail toggles the disabled state of a notification email
// PUT /api/v1/user/notify-email/toggle
func (h *UserHandler) ToggleNotifyEmail(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req ToggleNotifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	err := h.userService.ToggleNotifyEmail(c.Request.Context(), subject.UserID, req.Email, req.Disabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	updatedUser, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.UserFromService(updatedUser))
}
