package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelPricingHandler handles admin model pricing management.
type ModelPricingHandler struct {
	svc        *service.ModelPricingService
	pricingSvc *service.PricingService  // 可选，nil 时 LiteLLM 列显示 null
	billingSvc *service.BillingService  // 新增
}

func NewModelPricingHandler(svc *service.ModelPricingService) *ModelPricingHandler {
	return &ModelPricingHandler{svc: svc}
}

// SetPricingService 注入 PricingService（启动后调用，非必须）
func (h *ModelPricingHandler) SetPricingService(svc *service.PricingService) {
	h.pricingSvc = svc
}

// SetBillingService 注入 BillingService（启动后调用，非必须）
func (h *ModelPricingHandler) SetBillingService(svc *service.BillingService) {
	h.billingSvc = svc
}

// modelPricingResponse is the JSON shape returned to the frontend.
type modelPricingResponse struct {
	ID          int64  `json:"id"`
	ModelKey    string `json:"model_key"`
	DisplayName string `json:"display_name"`
	// per-million prices (USD) — easier to display/edit in UI
	InputPricePerMillion             float64 `json:"input_price_per_million"`
	OutputPricePerMillion            float64 `json:"output_price_per_million"`
	InputPricePerMillionPriority     float64 `json:"input_price_per_million_priority"`
	OutputPricePerMillionPriority    float64 `json:"output_price_per_million_priority"`
	CacheReadPricePerMillion         float64 `json:"cache_read_price_per_million"`
	CacheReadPricePerMillionPriority float64 `json:"cache_read_price_per_million_priority"`
	CacheCreationPricePerMillion     float64 `json:"cache_creation_price_per_million"`
	Enabled                          bool    `json:"enabled"`
	Note                             string  `json:"note"`
	CreatedAt                        string  `json:"created_at"`
	UpdatedAt                        string  `json:"updated_at"`
}

type upsertModelPricingRequest struct {
	ModelKey                         string  `json:"model_key" binding:"required"`
	DisplayName                      string  `json:"display_name"`
	InputPricePerMillion             float64 `json:"input_price_per_million" binding:"min=0"`
	OutputPricePerMillion            float64 `json:"output_price_per_million" binding:"min=0"`
	InputPricePerMillionPriority     float64 `json:"input_price_per_million_priority" binding:"min=0"`
	OutputPricePerMillionPriority    float64 `json:"output_price_per_million_priority" binding:"min=0"`
	CacheReadPricePerMillion         float64 `json:"cache_read_price_per_million" binding:"min=0"`
	CacheReadPricePerMillionPriority float64 `json:"cache_read_price_per_million_priority" binding:"min=0"`
	CacheCreationPricePerMillion     float64 `json:"cache_creation_price_per_million" binding:"min=0"`
	Enabled                          bool    `json:"enabled"`
	Note                             string  `json:"note"`
}

// List handles GET /admin/model-pricings
func (h *ModelPricingHandler) List(c *gin.Context) {
	entries, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]modelPricingResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, entryToResponse(e))
	}
	response.Success(c, out)
}

// Create handles POST /admin/model-pricings
func (h *ModelPricingHandler) Create(c *gin.Context) {
	var req upsertModelPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	entry := requestToEntry(req)
	created, err := h.svc.Create(c.Request.Context(), &entry)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, entryToResponse(*created))
}

// Update handles PUT /admin/model-pricings/:id
func (h *ModelPricingHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req upsertModelPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	entry := requestToEntry(req)
	updated, err := h.svc.Update(c.Request.Context(), id, &entry)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, entryToResponse(*updated))
}

// Delete handles DELETE /admin/model-pricings/:id
func (h *ModelPricingHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}

func requestToEntry(req upsertModelPricingRequest) service.ModelPricingEntry {
	return service.ModelPricingEntry{
		ModelKey:                         req.ModelKey,
		DisplayName:                      req.DisplayName,
		InputPricePerMillion:             req.InputPricePerMillion,
		OutputPricePerMillion:            req.OutputPricePerMillion,
		InputPricePerMillionPriority:     req.InputPricePerMillionPriority,
		OutputPricePerMillionPriority:    req.OutputPricePerMillionPriority,
		CacheReadPricePerMillion:         req.CacheReadPricePerMillion,
		CacheReadPricePerMillionPriority: req.CacheReadPricePerMillionPriority,
		CacheCreationPricePerMillion:     req.CacheCreationPricePerMillion,
		Enabled:                          req.Enabled,
		Note:                             req.Note,
	}
}

func entryToResponse(e service.ModelPricingEntry) modelPricingResponse {
	return modelPricingResponse{
		ID:                               e.ID,
		ModelKey:                         e.ModelKey,
		DisplayName:                      e.DisplayName,
		InputPricePerMillion:             e.InputPricePerMillion,
		OutputPricePerMillion:            e.OutputPricePerMillion,
		InputPricePerMillionPriority:     e.InputPricePerMillionPriority,
		OutputPricePerMillionPriority:    e.OutputPricePerMillionPriority,
		CacheReadPricePerMillion:         e.CacheReadPricePerMillion,
		CacheReadPricePerMillionPriority: e.CacheReadPricePerMillionPriority,
		CacheCreationPricePerMillion:     e.CacheCreationPricePerMillion,
		Enabled:                          e.Enabled,
		Note:                             e.Note,
		CreatedAt:                        e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:                        e.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// liteLLMPriceSnapshot LiteLLM 动态价格快照（per-million USD）
type liteLLMPriceSnapshot struct {
	InputPerMillion         float64 `json:"input_per_million"`
	OutputPerMillion        float64 `json:"output_per_million"`
	CacheReadPerMillion     float64 `json:"cache_read_per_million"`
	CacheCreationPerMillion float64 `json:"cache_creation_per_million"`
	InputPriorityPerMillion  float64 `json:"input_priority_per_million"`
	OutputPriorityPerMillion float64 `json:"output_priority_per_million"`
}

// modelPricingCompareItem 对比视图的单行数据
type modelPricingCompareItem struct {
	modelPricingResponse
	LiteLLM *liteLLMPriceSnapshot `json:"litellm"`
}

// Compare handles GET /admin/model-pricings/compare
func (h *ModelPricingHandler) Compare(c *gin.Context) {
	entries, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]modelPricingCompareItem, 0, len(entries))
	for _, e := range entries {
		item := modelPricingCompareItem{
			modelPricingResponse: entryToResponse(e),
			LiteLLM:              h.fetchLiteLLMSnapshot(e.ModelKey),
		}
		out = append(out, item)
	}
	response.Success(c, out)
}

const toMillion = 1_000_000.0

func (h *ModelPricingHandler) fetchLiteLLMSnapshot(modelKey string) *liteLLMPriceSnapshot {
	if h.pricingSvc == nil {
		return nil
	}
	p := h.pricingSvc.GetModelPricing(modelKey)
	if p == nil {
		return nil
	}
	return &liteLLMPriceSnapshot{
		InputPerMillion:          p.InputCostPerToken * toMillion,
		OutputPerMillion:         p.OutputCostPerToken * toMillion,
		CacheReadPerMillion:      p.CacheReadInputTokenCost * toMillion,
		CacheCreationPerMillion:  p.CacheCreationInputTokenCost * toMillion,
		InputPriorityPerMillion:  p.InputCostPerTokenPriority * toMillion,
		OutputPriorityPerMillion: p.OutputCostPerTokenPriority * toMillion,
	}
}

// priceTier 单层价格数据（per-million USD）
type priceTier struct {
	InputPerMillion         float64 `json:"input_per_million"`
	OutputPerMillion        float64 `json:"output_per_million"`
	CacheReadPerMillion     float64 `json:"cache_read_per_million"`
	CacheCreationPerMillion float64 `json:"cache_creation_per_million"`
	InputPriorityPerMillion  float64 `json:"input_priority_per_million"`
	OutputPriorityPerMillion float64 `json:"output_priority_per_million"`
}

// modelPricingLookupResponse 三层价格对比
type modelPricingLookupResponse struct {
	Model        string     `json:"model"`
	LiteLLM      *priceTier `json:"litellm"`
	Database     *priceTier `json:"database"`
	Fallback     *priceTier `json:"fallback"`
	ActiveSource string     `json:"active_source"`
}

// Lookup handles GET /admin/model-pricings/lookup?model=xxx
func (h *ModelPricingHandler) Lookup(c *gin.Context) {
	model := strings.ToLower(strings.TrimSpace(c.Query("model")))
	if model == "" {
		response.BadRequest(c, "model query parameter is required")
		return
	}

	resp := modelPricingLookupResponse{Model: model}

	// 1. LiteLLM 层
	if h.pricingSvc != nil {
		if p := h.pricingSvc.GetModelPricing(model); p != nil {
			resp.LiteLLM = liteLLMToPriceTier(p)
		}
	}

	// 2. 数据库层（精确匹配 model_key，且 enabled=true）
	dbEntry, err := h.svc.GetByKey(c.Request.Context(), model)
	if err == nil && dbEntry != nil && dbEntry.Enabled {
		resp.Database = dbEntryToPriceTier(dbEntry)
	}

	// 3. Fallback 层
	if h.billingSvc != nil {
		if p := h.billingSvc.GetFallbackPricing(model); p != nil {
			resp.Fallback = modelPricingToPriceTier(p)
		}
	}

	// 确定生效层
	switch {
	case resp.LiteLLM != nil:
		resp.ActiveSource = "litellm"
	case resp.Database != nil:
		resp.ActiveSource = "database"
	case resp.Fallback != nil:
		resp.ActiveSource = "fallback"
	default:
		resp.ActiveSource = "none"
	}

	response.Success(c, resp)
}

func liteLLMToPriceTier(p *service.LiteLLMModelPricing) *priceTier {
	return &priceTier{
		InputPerMillion:         p.InputCostPerToken * toMillion,
		OutputPerMillion:        p.OutputCostPerToken * toMillion,
		CacheReadPerMillion:     p.CacheReadInputTokenCost * toMillion,
		CacheCreationPerMillion: p.CacheCreationInputTokenCost * toMillion,
		InputPriorityPerMillion:  p.InputCostPerTokenPriority * toMillion,
		OutputPriorityPerMillion: p.OutputCostPerTokenPriority * toMillion,
	}
}

func dbEntryToPriceTier(e *service.ModelPricingEntry) *priceTier {
	return &priceTier{
		InputPerMillion:         e.InputPricePerMillion,
		OutputPerMillion:        e.OutputPricePerMillion,
		CacheReadPerMillion:     e.CacheReadPricePerMillion,
		CacheCreationPerMillion: e.CacheCreationPricePerMillion,
		InputPriorityPerMillion:  e.InputPricePerMillionPriority,
		OutputPriorityPerMillion: e.OutputPricePerMillionPriority,
	}
}

func modelPricingToPriceTier(p *service.ModelPricing) *priceTier {
	return &priceTier{
		InputPerMillion:         p.InputPricePerToken * toMillion,
		OutputPerMillion:        p.OutputPricePerToken * toMillion,
		CacheReadPerMillion:     p.CacheReadPricePerToken * toMillion,
		CacheCreationPerMillion: p.CacheCreationPricePerToken * toMillion,
		InputPriorityPerMillion:  p.InputPricePerTokenPriority * toMillion,
		OutputPriorityPerMillion: p.OutputPricePerTokenPriority * toMillion,
	}
}
