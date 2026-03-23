package admin

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PreviewFromCPARequest struct {
	FileName string `json:"file_name" binding:"required"`
	RawJSON  string `json:"raw_json" binding:"required"`
}

type ImportFromCPARequest struct {
	FileName            string  `json:"file_name" binding:"required"`
	RawJSON             string  `json:"raw_json" binding:"required"`
	ProxyID             *int64  `json:"proxy_id"`
	Concurrency         int     `json:"concurrency"`
	UseDefaultGroupBind *bool   `json:"use_default_group_bind"`
	GroupIDs            []int64 `json:"group_ids"`
}

type PreviewRemoteFromCPARequest struct {
	BaseURL       string `json:"base_url" binding:"required"`
	ManagementKey string `json:"management_key" binding:"required"`
}

type ImportRemoteFromCPARequest struct {
	BaseURL             string   `json:"base_url" binding:"required"`
	ManagementKey       string   `json:"management_key" binding:"required"`
	SelectedSourceKeys  []string `json:"selected_source_keys"`
	ProxyID             *int64   `json:"proxy_id"`
	Concurrency         int      `json:"concurrency"`
	UseDefaultGroupBind *bool    `json:"use_default_group_bind"`
	GroupIDs            []int64  `json:"group_ids"`
}

// PreviewFromCPA parses a CPA auth file and returns the target account mapping.
// POST /api/v1/admin/accounts/import/cpa/preview
func (h *AccountHandler) PreviewFromCPA(c *gin.Context) {
	var req PreviewFromCPARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if h.cpaImportService == nil {
		response.InternalError(c, "CPA import service unavailable")
		return
	}

	result, err := h.cpaImportService.PreviewFromCPA(c.Request.Context(), service.PreviewFromCPAInput{
		FileName: req.FileName,
		RawJSON:  req.RawJSON,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// ImportFromCPA imports a CPA auth file as a sub2api account.
// POST /api/v1/admin/accounts/import/cpa
func (h *AccountHandler) ImportFromCPA(c *gin.Context) {
	var req ImportFromCPARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if h.cpaImportService == nil {
		response.InternalError(c, "CPA import service unavailable")
		return
	}

	useDefaultGroupBind := true
	if req.UseDefaultGroupBind != nil {
		useDefaultGroupBind = *req.UseDefaultGroupBind
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_cpa", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.cpaImportService.ImportFromCPA(ctx, service.ImportFromCPAInput{
			FileName:            req.FileName,
			RawJSON:             req.RawJSON,
			ProxyID:             req.ProxyID,
			Concurrency:         req.Concurrency,
			UseDefaultGroupBind: useDefaultGroupBind,
			GroupIDs:            req.GroupIDs,
		})
	})
}

// PreviewRemoteFromCPA fetches active auth files from a CPA instance and returns importable accounts.
// POST /api/v1/admin/accounts/import/cpa/remote/preview
func (h *AccountHandler) PreviewRemoteFromCPA(c *gin.Context) {
	var req PreviewRemoteFromCPARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if h.cpaImportService == nil {
		response.InternalError(c, "CPA import service unavailable")
		return
	}

	result, err := h.cpaImportService.PreviewRemoteFromCPA(c.Request.Context(), service.PreviewRemoteFromCPAInput{
		BaseURL:       req.BaseURL,
		ManagementKey: req.ManagementKey,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// ImportRemoteFromCPA imports selected active auth files from a CPA instance.
// POST /api/v1/admin/accounts/import/cpa/remote
func (h *AccountHandler) ImportRemoteFromCPA(c *gin.Context) {
	var req ImportRemoteFromCPARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if h.cpaImportService == nil {
		response.InternalError(c, "CPA import service unavailable")
		return
	}

	useDefaultGroupBind := true
	if req.UseDefaultGroupBind != nil {
		useDefaultGroupBind = *req.UseDefaultGroupBind
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_cpa_remote", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.cpaImportService.ImportRemoteFromCPA(ctx, service.ImportRemoteFromCPAInput{
			BaseURL:             req.BaseURL,
			ManagementKey:       req.ManagementKey,
			SelectedSourceKeys:  req.SelectedSourceKeys,
			ProxyID:             req.ProxyID,
			Concurrency:         req.Concurrency,
			UseDefaultGroupBind: useDefaultGroupBind,
			GroupIDs:            req.GroupIDs,
		})
	})
}
