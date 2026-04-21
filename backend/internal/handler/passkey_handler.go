package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type passkeyManagementService interface {
	GetManagementStatus(ctx context.Context, userID int64) (*service.PasskeyManagementStatus, error)
	ListManagementCredentials(ctx context.Context, userID int64) (*service.PasskeyManagementListResult, error)
	RenameCredential(ctx context.Context, userID int64, credentialID, friendlyName string) (*service.PasskeyManagementCredential, error)
	RevokeCredential(ctx context.Context, userID int64, credentialID string) (*service.PasskeyManagementRevokeResult, error)
}

type PasskeyHandler struct {
	passkeyService passkeyManagementService
}

func NewPasskeyHandler(passkeyService *service.PasskeyService) *PasskeyHandler {
	return &PasskeyHandler{passkeyService: passkeyService}
}

type PasskeyStatusResponse struct {
	FeatureEnabled            bool `json:"feature_enabled"`
	CanManage                 bool `json:"can_manage"`
	HasPasskeys               bool `json:"has_passkeys"`
	ActiveCount               int  `json:"active_count"`
	PasswordFallbackAvailable bool `json:"password_fallback_available"`
}

type PasskeyCredentialResponse struct {
	CredentialID   string `json:"credential_id"`
	FriendlyName   string `json:"friendly_name"`
	CreatedAt      int64  `json:"created_at"`
	LastUsedAt     *int64 `json:"last_used_at,omitempty"`
	BackupEligible bool   `json:"backup_eligible"`
	Synced         bool   `json:"synced"`
}

type PasskeyListResponse struct {
	Items []PasskeyCredentialResponse `json:"items"`
}

type PasskeyRenameRequest struct {
	FriendlyName string `json:"friendly_name" binding:"required"`
}

type PasskeyRenameResponse struct {
	Credential PasskeyCredentialResponse `json:"credential"`
}

type PasskeyRevokeResponse struct {
	Success                   bool   `json:"success"`
	CredentialID              string `json:"credential_id"`
	RevokedAt                 int64  `json:"revoked_at"`
	PasswordFallbackAvailable bool   `json:"password_fallback_available"`
}

func (h *PasskeyHandler) GetStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.passkeyService == nil {
		response.InternalError(c, "Passkey service is not configured")
		return
	}

	status, err := h.passkeyService.GetManagementStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, PasskeyStatusResponse{
		FeatureEnabled:            status.FeatureEnabled,
		CanManage:                 status.CanManage,
		HasPasskeys:               status.HasPasskeys,
		ActiveCount:               status.ActiveCount,
		PasswordFallbackAvailable: status.PasswordFallbackAvailable,
	})
}

func (h *PasskeyHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.passkeyService == nil {
		response.InternalError(c, "Passkey service is not configured")
		return
	}

	result, err := h.passkeyService.ListManagementCredentials(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	items := make([]PasskeyCredentialResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, passkeyCredentialResponseFromService(item))
	}

	response.Success(c, PasskeyListResponse{Items: items})
}

func (h *PasskeyHandler) Rename(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.passkeyService == nil {
		response.InternalError(c, "Passkey service is not configured")
		return
	}

	var req PasskeyRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	credential, err := h.passkeyService.RenameCredential(c.Request.Context(), subject.UserID, c.Param("credentialId"), req.FriendlyName)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, PasskeyRenameResponse{Credential: passkeyCredentialResponseFromService(*credential)})
}

func (h *PasskeyHandler) Revoke(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.passkeyService == nil {
		response.InternalError(c, "Passkey service is not configured")
		return
	}

	result, err := h.passkeyService.RevokeCredential(c.Request.Context(), subject.UserID, c.Param("credentialId"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, PasskeyRevokeResponse{
		Success:                   true,
		CredentialID:              result.CredentialID,
		RevokedAt:                 result.RevokedAt.UTC().Unix(),
		PasswordFallbackAvailable: result.PasswordFallbackAvailable,
	})
}

func passkeyCredentialResponseFromService(item service.PasskeyManagementCredential) PasskeyCredentialResponse {
	resp := PasskeyCredentialResponse{
		CredentialID:   item.CredentialID,
		FriendlyName:   item.FriendlyName,
		CreatedAt:      item.CreatedAt.UTC().Unix(),
		BackupEligible: item.BackupEligible,
		Synced:         item.Synced,
	}
	if item.LastUsedAt != nil {
		ts := item.LastUsedAt.UTC().Unix()
		resp.LastUsedAt = &ts
	}
	return resp
}
