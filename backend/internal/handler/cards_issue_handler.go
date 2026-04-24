package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type CardsIssueHandler struct {
	cardsIssueService *service.CardsIssueService
	settingService    *service.SettingService
}

func NewCardsIssueHandler(cardsIssueService *service.CardsIssueService, settingService *service.SettingService) *CardsIssueHandler {
	return &CardsIssueHandler{
		cardsIssueService: cardsIssueService,
		settingService:    settingService,
	}
}

func (h *CardsIssueHandler) Issue(c *gin.Context) {
	cfg, err := h.settingService.GetCardsIssueRuntimeConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !cfg.Enabled {
		response.ErrorFrom(c, infraerrors.Forbidden("CARDS_ISSUE_DISABLED", "cards issue endpoint is disabled"))
		return
	}
	if !validateCardsIssueBearer(c, cfg.BearerKey) {
		return
	}

	req, err := parseCardsIssueRequest(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	executeCardsIssueIdempotentJSON(c, req, func(ctx context.Context) (any, error) {
		return h.cardsIssueService.IssueOrder(ctx, req, *cfg)
	})
}

func validateCardsIssueBearer(c *gin.Context, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("CARDS_ISSUE_KEY_UNCONFIGURED", "cards issue bearer key is not configured"))
		return false
	}
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader == "" {
		response.ErrorFrom(c, infraerrors.Unauthorized("CARDS_ISSUE_UNAUTHORIZED", "authorization required"))
		return false
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		response.ErrorFrom(c, infraerrors.Unauthorized("CARDS_ISSUE_UNAUTHORIZED", "authorization must use Bearer token"))
		return false
	}
	provided := strings.TrimSpace(parts[1])
	if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		response.ErrorFrom(c, infraerrors.Unauthorized("CARDS_ISSUE_UNAUTHORIZED", "invalid bearer token"))
		return false
	}
	return true
}

func parseCardsIssueRequest(c *gin.Context) (service.CardsIssueRequest, error) {
	var req service.CardsIssueRequest
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return req, infraerrors.BadRequest("CARDS_ISSUE_INVALID_REQUEST", "request body is required")
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return req, infraerrors.BadRequest("CARDS_ISSUE_INVALID_REQUEST", "failed to read request body").WithCause(err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return req, infraerrors.BadRequest("CARDS_ISSUE_INVALID_REQUEST", "request body is required")
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return req, infraerrors.BadRequest("CARDS_ISSUE_INVALID_REQUEST", "invalid JSON request body").WithCause(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return req, infraerrors.BadRequest("CARDS_ISSUE_INVALID_REQUEST", "invalid JSON request body").WithCause(err)
	}
	for _, key := range []string{"buyer_id", "buyer_name", "order_id", "order_amount", "order_quantity"} {
		delete(payload, key)
	}
	req.Extra = payload
	return req, nil
}

func executeCardsIssueIdempotentJSON(
	c *gin.Context,
	req service.CardsIssueRequest,
	execute func(context.Context) (any, error),
) {
	coordinator := service.DefaultIdempotencyCoordinator()
	if coordinator == nil {
		data, err := execute(c.Request.Context())
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		response.Success(c, data)
		return
	}

	result, err := coordinator.Execute(c.Request.Context(), service.IdempotencyExecuteOptions{
		Scope:          "custom.cards.issue",
		ActorScope:     "cards_issue",
		Method:         c.Request.Method,
		Route:          c.FullPath(),
		IdempotencyKey: strings.TrimSpace(req.OrderID),
		Payload:        req,
		RequireKey:     true,
		TTL:            service.DefaultWriteIdempotencyTTL(),
	}, execute)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if result != nil && result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	response.Success(c, result.Data)
}
