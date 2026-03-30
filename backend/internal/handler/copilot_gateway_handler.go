package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	copilotpkg "github.com/Wei-Shaw/sub2api/internal/pkg/copilot"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// CopilotGatewayHandler handles GitHub Copilot API gateway requests.
//
// It exposes OpenAI-compatible endpoints (/copilot/v1/chat/completions, /copilot/v1/models)
// that proxy to the GitHub Copilot API via CopilotGatewayService.

// fallbackCopilotMaxBodyBytes is the built-in fallback used when
// gateway.copilot_default_max_body_kb is not set in the configuration.
const fallbackCopilotMaxBodyBytes = 1024 * 1024 * 1024 // 1 GB

// StatusClientClosedRequest is the HTTP status code used when the client
// disconnects before the upstream response is received (nginx convention 499).
const StatusClientClosedRequest = 499

const (
	// copilotModelCacheTTL is the TTL for model list entries fetched from the
	// upstream Copilot /models endpoint.  One hour is intentionally long because
	// the model catalogue changes rarely and Claude Code fetches /models on every
	// startup, so a short TTL would cause unnecessary upstream traffic.
	copilotModelCacheTTL = 1 * time.Hour

	// copilotModelCacheFailedTTL is the shorter TTL used when the cache entry
	// contains a fallback static model list (upstream was unreachable).  Two
	// minutes allows the cache to self-heal quickly once the upstream recovers.
	copilotModelCacheFailedTTL = 2 * time.Minute
)

// copilotModelCacheEntry holds a cached model-list response for a single group.
type copilotModelCacheEntry struct {
	data         []byte // raw JSON as returned to the client
	cachedAt     time.Time
	fromUpstream bool // true = populated from upstream (long TTL); false = static fallback (short TTL)
}

type CopilotGatewayHandler struct {
	gatewayService        *service.GatewayService
	copilotGatewayService *service.CopilotGatewayService
	billingCacheService   *service.BillingCacheService
	apiKeyService         *service.APIKeyService
	concurrencyHelper     *ConcurrencyHelper
	anomalyService        *service.AnomalyService
	maxAccountSwitches    int
	// defaultMaxBodyBytes is the system-level body size limit for Copilot requests.
	// It is read from gateway.copilot_default_max_body_kb at startup and falls back
	// to fallbackCopilotMaxBodyBytes when the config value is 0 / absent.
	// Per-account overrides via account.Extra["max_body_bytes"] take precedence.
	defaultMaxBodyBytes int

	// modelCacheMu protects modelCache.
	modelCacheMu sync.RWMutex
	// modelCache stores per-group model list cache entries keyed by groupID.
	// groupID 0 is used when apiKey.GroupID is nil.
	modelCache map[int64]*copilotModelCacheEntry
}

// NewCopilotGatewayHandler creates a new CopilotGatewayHandler.
func NewCopilotGatewayHandler(
	gatewayService *service.GatewayService,
	copilotGatewayService *service.CopilotGatewayService,
	concurrencyService *service.ConcurrencyService,
	billingCacheService *service.BillingCacheService,
	apiKeyService *service.APIKeyService,
	cfg *config.Config,
	anomalyService *service.AnomalyService,
) *CopilotGatewayHandler {
	pingInterval := time.Duration(0)
	maxAccountSwitches := 3
	defaultMaxBodyBytes := fallbackCopilotMaxBodyBytes
	if cfg != nil {
		pingInterval = time.Duration(cfg.Concurrency.PingInterval) * time.Second
		if cfg.Gateway.MaxAccountSwitches > 0 {
			maxAccountSwitches = cfg.Gateway.MaxAccountSwitches
		}
		if cfg.Gateway.CopilotDefaultMaxBodyKB > 0 {
			defaultMaxBodyBytes = int(cfg.Gateway.CopilotDefaultMaxBodyKB * 1024)
		}
	}
	return &CopilotGatewayHandler{
		gatewayService:        gatewayService,
		copilotGatewayService: copilotGatewayService,
		billingCacheService:   billingCacheService,
		apiKeyService:         apiKeyService,
		concurrencyHelper:     NewConcurrencyHelper(concurrencyService, SSEPingFormatComment, pingInterval),
		anomalyService:        anomalyService,
		maxAccountSwitches:    maxAccountSwitches,
		defaultMaxBodyBytes:   defaultMaxBodyBytes,
		modelCache:            make(map[int64]*copilotModelCacheEntry),
	}
}

// ChatCompletions handles Copilot /v1/chat/completions endpoint (OpenAI-compatible).
//
// The ForcePlatform middleware must be applied on the route group to set the
// platform context to "copilot" so that account selection picks only Copilot accounts.
func (h *CopilotGatewayHandler) ChatCompletions(c *gin.Context) {
	requestStart := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.copilot_gateway.chat_completions",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	// Read request body
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	setOpsRequestContext(c, "", false, body)

	// Validate JSON
	if !gjson.ValidBytes(body) {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	// Extract model
	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqModel := modelResult.String()

	reqStream := gjson.GetBytes(body, "stream").Bool()
	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))

	setOpsRequestContext(c, reqModel, reqStream, body)

	// Check billing eligibility
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription); err != nil {
		reqLog.Info("copilot.billing_eligibility_check_failed", zap.Error(err))
		status, code, message := billingErrorDetails(err)
		h.errorResponse(c, status, code, message)
		return
	}

	// Acquire user concurrency slot
	ctx := c.Request.Context()
	userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlot(ctx, subject.UserID, subject.Concurrency)
	if err != nil {
		reqLog.Warn("copilot.user_slot_acquire_failed", zap.Error(err))
		h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Concurrency limit exceeded, please retry later")
		return
	}
	if !userAcquired {
		h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Too many concurrent requests, please retry later")
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	routingStart := time.Now()

	// Select a Copilot account.
	// ForcePlatform middleware sets platform = "copilot" in context, so
	// SelectAccountForModelWithExclusions will only pick copilot accounts.
	failedAccountIDs := make(map[int64]struct{})
	switchCount := 0

	for {
		account, err := h.gatewayService.SelectAccountForModelWithExclusions(
			ctx,
			apiKey.GroupID,
			"", // sessionHash — no sticky session for Copilot
			reqModel,
			failedAccountIDs,
		)
		if err != nil || account == nil {
			if ctx.Err() == context.Canceled {
				reqLog.Info("copilot.client_disconnected", zap.Error(err))
				h.errorResponse(c, StatusClientClosedRequest, "client_closed", "Client disconnected")
			} else if len(failedAccountIDs) == 0 {
				reqLog.Warn("copilot.account_select_failed", zap.Error(err))
				h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "No available Copilot accounts")
			} else {
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "All Copilot accounts failed")
			}
			return
		}
		reqLog.Debug("copilot.account_selected",
			zap.Int64("account_id", account.ID),
			zap.String("account_name", account.Name))
		setOpsSelectedAccount(c, account.ID, account.Platform)

		if switchCount == 0 {
			service.AppendOpsSpan(c, service.OpsSpan{
				Name:        "routing.select",
				StartUnixMs: routingStart.UnixMilli(),
				DurationMs:  time.Since(routingStart).Milliseconds(),
				Status:      "ok",
				Attrs:       map[string]any{"account_id": account.ID, "platform": "copilot"},
			})
		} else {
			service.AppendOpsSpan(c, service.OpsSpan{
				Name:        "failover.select",
				StartUnixMs: time.Now().UnixMilli(),
				DurationMs:  0,
				Status:      "ok",
				Attrs:       map[string]any{"attempt": switchCount, "account_id": account.ID},
			})
		}

		// Reject oversized request bodies before hitting the upstream.
		if h.checkCopilotBodySize(c, body, account, false) {
			return
		}

		// Forward request to Copilot API
		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()
		result, fwdErr := h.copilotGatewayService.ForwardChatCompletions(ctx, c, account, body)
		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		if fwdErr == nil {
			upstreamMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
			responseMs := forwardDurationMs
			if upstreamMs > 0 && forwardDurationMs > upstreamMs {
				responseMs = forwardDurationMs - upstreamMs
			}
			service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseMs)
		}
		if fwdErr != nil {
			// Client disconnected — skip failover, no one is listening.
			if ctx.Err() == context.Canceled {
				reqLog.Info("copilot.client_disconnected",
					zap.Int64("account_id", account.ID),
					zap.Int("switch_count", switchCount),
					zap.Error(fwdErr))
				h.errorResponse(c, StatusClientClosedRequest, "client_closed", "Client disconnected before upstream response")
				return
			}
			failedAccountIDs[account.ID] = struct{}{}
			switchCount++
			if switchCount >= h.maxAccountSwitches {
				reqLog.Warn("copilot.failover_exhausted",
					zap.Int64("account_id", account.ID),
					zap.Int("switch_count", switchCount),
					zap.Error(fwdErr))
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
				return
			}
			reqLog.Warn("copilot.upstream_failover_switching",
				zap.Int64("account_id", account.ID),
				zap.Int("switch_count", switchCount),
				zap.Error(fwdErr))
			continue
		}

		// 421 Misdirected Request: HTTP/2 connection-level error, failover to another account.
		if result != nil && result.StatusCode == http.StatusMisdirectedRequest {
			reqLog.Warn("copilot.misdirected_request_failover",
				zap.Int64("account_id", account.ID),
				zap.Int("switch_count", switchCount))
			failedAccountIDs[account.ID] = struct{}{}
			switchCount++
			if switchCount >= h.maxAccountSwitches {
				reqLog.Warn("copilot.failover_exhausted",
					zap.Int64("account_id", account.ID),
					zap.Int("switch_count", switchCount))
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
				return
			}
			continue
		}

		// 429 Too Many Requests: rate limited — failover to another account.
		if result != nil && result.StatusCode == http.StatusTooManyRequests {
			reqLog.Warn("copilot.rate_limited_failover",
				zap.Int64("account_id", account.ID),
				zap.Int("switch_count", switchCount))
			failedAccountIDs[account.ID] = struct{}{}
			switchCount++
			if switchCount >= h.maxAccountSwitches {
				reqLog.Warn("copilot.failover_exhausted",
					zap.Int64("account_id", account.ID),
					zap.Int("switch_count", switchCount))
				h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "All Copilot accounts are rate limited")
				return
			}
			continue
		}

		// Handle upstream error responses (non-2xx already forwarded to client by service)
		if result != nil && result.StatusCode != http.StatusOK {
			reqLog.Debug("copilot.request_completed_with_error",
				zap.Int64("account_id", account.ID),
				zap.Int("status", result.StatusCode))
			return
		}

		// Record usage to database.
		if result != nil && result.Usage != nil {
			inboundEp := snapshotInboundForUsageLog(c)
			userAgent := c.GetHeader("User-Agent")
			clientIP := ip.GetClientIP(c)
			capturedResult := result
			capturedAccount := account
			// 在 goroutine 外捕获阶段耗时（gin.Context 仅在请求 goroutine 中有效）
			authLatencyMs := getContextLatencyMsPtr(c, service.OpsAuthLatencyMsKey)
			routingLatencyMs := getContextLatencyMsPtr(c, service.OpsRoutingLatencyMsKey)
			upstreamLatencyMsVal := getContextLatencyMsPtr(c, service.OpsUpstreamLatencyMsKey)
			responseLatencyMsVal := getContextLatencyMsPtr(c, service.OpsResponseLatencyMsKey)
			capturedInitiator := service.CopilotInitiatorFromBody(body)
			// Capture request-scoped values before entering goroutine (gin.Context not safe across goroutines).
			capturedRequestID := c.GetHeader("X-Request-ID")
			capturedReqBody := body
			capturedUpstreamReqBody, capturedUpstreamRespBody := service.GetOpsUpstreamBodies(c)
			go func() {
				recordCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				fwdResult := &service.ForwardResult{
					Model:         capturedResult.Model,
					UpstreamModel: capturedResult.UpstreamModel,
					Stream:        reqStream,
					Usage: service.ClaudeUsage{
						InputTokens:  capturedResult.Usage.PromptTokens,
						OutputTokens: capturedResult.Usage.CompletionTokens,
					},
					Duration:     capturedResult.Duration,
					FirstTokenMs: capturedResult.FirstTokenMs,
				}
				if err := h.gatewayService.RecordUsage(recordCtx, &service.RecordUsageInput{
					Result:            fwdResult,
					APIKey:            apiKey,
					User:              apiKey.User,
					Account:           capturedAccount,
					Subscription:      subscription,
					InboundEndpoint:   inboundEp,
					UpstreamEndpoint:  EndpointChatCompletions,
					UserAgent:         userAgent,
					IPAddress:         clientIP,
					RequestBodyBytes:  intPtr(len(body)),
					APIKeyService:     h.apiKeyService,
					AuthLatencyMs:     authLatencyMs,
					RoutingLatencyMs:  routingLatencyMs,
					UpstreamLatencyMs: upstreamLatencyMsVal,
					ResponseLatencyMs: responseLatencyMsVal,
					Initiator:         capturedInitiator,
				}); err != nil {
					reqLog.Error("copilot.record_usage_failed", zap.Error(err))
				}
				// Async anomaly detection — already in a goroutine, no extra go needed.
				if h.anomalyService != nil {
					userID := apiKey.UserID
					apiKeyID := apiKey.ID
					accountID := capturedAccount.ID
					h.anomalyService.WriteAnomalyLog(
						recordCtx,
						capturedResult.Usage.PromptTokens,
						capturedResult.Usage.CompletionTokens,
						capturedResult.Duration.Milliseconds(),
						200,
						&service.RequestLogInput{
							RequestID:            capturedRequestID,
							UserID:               &userID,
							APIKeyID:             &apiKeyID,
							AccountID:            &accountID,
							GroupID:              apiKey.GroupID,
							RequestBody:          capturedReqBody,
							UpstreamRequestBody:  capturedUpstreamReqBody,
							UpstreamResponseBody: capturedUpstreamRespBody,
						},
					)
				}
			}()
		}

		reqLog.Debug("copilot.chat_completions.completed",
			zap.Int64("account_id", account.ID),
			zap.Int("switch_count", switchCount))
		return
	}
}

// Models handles Copilot /v1/models endpoint.
//
// The response is cached per GroupID to avoid hammering the upstream Copilot
// /models endpoint on every Claude Code startup.  Degradation policy (high-
// availability first):
//
//   - Fresh cache hit                                 → 200 cached data
//   - Cache expired + upstream success                → 200 refresh cache
//   - Cache expired + upstream failure + stale exists → 200 stale data  + warn log
//   - Cache expired + upstream failure + no cache     → 200 static defaults + warn log
//   - No account available         + stale exists     → 200 stale data  + warn log
//   - No account available         + no cache         → 503 (truly unserviceable)
func (h *CopilotGatewayHandler) Models(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	reqLog := requestLogger(c, "handler.copilot_gateway.models",
		zap.Int64("api_key_id", apiKey.ID))

	ctx := c.Request.Context()

	groupID := int64(0)
	if apiKey.GroupID != nil {
		groupID = *apiKey.GroupID
	}

	// 1. Fresh cache hit — return immediately without touching the upstream.
	if cached, ok := h.getModelCache(groupID); ok {
		c.Data(http.StatusOK, "application/json", cached)
		return
	}

	// 2. Select any active Copilot account for this group.
	// ForcePlatform middleware ensures only copilot accounts are returned.
	account, err := h.gatewayService.SelectAccountForModelWithExclusions(ctx, apiKey.GroupID, "", "", nil)
	if err != nil || account == nil {
		if stale := h.getStaleModelCache(groupID); stale != nil {
			// Account pool is empty but we have an old cache — serve it and log.
			reqLog.Warn("copilot.models_no_account_stale_cache",
				zap.Int64("group_id", groupID), zap.Error(err))
			c.Data(http.StatusOK, "application/json", stale)
			return
		}
		// No account and no cache at all — genuinely unserviceable.
		reqLog.Warn("copilot.models_account_select_failed",
			zap.Int64("group_id", groupID), zap.Error(err))
		h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "No available Copilot accounts")
		return
	}

	// 3. Fetch model list from upstream.
	body, err := h.copilotGatewayService.ListModels(ctx, account)
	if err != nil {
		if stale := h.getStaleModelCache(groupID); stale != nil {
			// Upstream failed but we have a stale cache — serve it.
			reqLog.Warn("copilot.models_upstream_failed_stale_cache",
				zap.Int64("account_id", account.ID), zap.Error(err))
			c.Data(http.StatusOK, "application/json", stale)
			return
		}
		// Upstream failed and no cache at all — fall back to static defaults.
		// Use a short TTL so we retry the upstream soon.
		reqLog.Warn("copilot.models_upstream_failed_fallback_defaults",
			zap.Int64("account_id", account.ID), zap.Error(err))
		defaultJSON := buildDefaultCopilotModelsJSON()
		h.setModelCache(groupID, defaultJSON, false)
		c.Data(http.StatusOK, "application/json", defaultJSON)
		return
	}

	// 4. Success — refresh cache and respond.
	h.setModelCache(groupID, body, true)
	c.Data(http.StatusOK, "application/json", body)
}

// getModelCache returns the cached model list for groupID if it exists and has
// not yet expired.  The second return value is false on a cache miss or expiry.
func (h *CopilotGatewayHandler) getModelCache(groupID int64) ([]byte, bool) {
	h.modelCacheMu.RLock()
	defer h.modelCacheMu.RUnlock()
	entry, ok := h.modelCache[groupID]
	if !ok || entry == nil {
		return nil, false
	}
	ttl := copilotModelCacheTTL
	if !entry.fromUpstream {
		ttl = copilotModelCacheFailedTTL
	}
	if time.Since(entry.cachedAt) > ttl {
		return nil, false
	}
	return entry.data, true
}

// getStaleModelCache returns whatever is cached for groupID regardless of TTL.
// It returns nil when no entry exists at all.  Used for graceful degradation.
func (h *CopilotGatewayHandler) getStaleModelCache(groupID int64) []byte {
	h.modelCacheMu.RLock()
	defer h.modelCacheMu.RUnlock()
	if entry, ok := h.modelCache[groupID]; ok && entry != nil {
		return entry.data
	}
	return nil
}

// setModelCache stores the model list for groupID.
// fromUpstream indicates whether data came from the live upstream (true) or is
// a static fallback (false); it controls which TTL will be applied on read.
func (h *CopilotGatewayHandler) setModelCache(groupID int64, data []byte, fromUpstream bool) {
	h.modelCacheMu.Lock()
	defer h.modelCacheMu.Unlock()
	h.modelCache[groupID] = &copilotModelCacheEntry{
		data:         data,
		cachedAt:     time.Now(),
		fromUpstream: fromUpstream,
	}
}

// buildDefaultCopilotModelsJSON serialises copilotpkg.DefaultModels into the
// OpenAI /models list format, matching the shape returned by ListModels().
func buildDefaultCopilotModelsJSON() []byte {
	payload := map[string]any{
		"object": "list",
		"data":   copilotpkg.DefaultModels,
	}
	b, _ := json.Marshal(payload) // DefaultModels is a compile-time constant; Marshal cannot fail.
	return b
}

// Responses handles Copilot /responses endpoint (OpenAI Responses API, used by Codex CLI).
//
// Codex CLI sets wire_api = "responses" and base_url = ".../copilot", so it
// sends requests to {base_url}/responses — i.e. /copilot/responses.
// The request/response format is passed through as-is to the Copilot API's
// /responses endpoint; no protocol translation is needed.
func (h *CopilotGatewayHandler) Responses(c *gin.Context) {
	requestStartResponses := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.copilot_gateway.responses",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	setOpsRequestContext(c, "", false, body)

	if !gjson.ValidBytes(body) {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqModel := modelResult.String()
	reqStream := gjson.GetBytes(body, "stream").Bool()
	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))

	setOpsRequestContext(c, reqModel, reqStream, body)

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription); err != nil {
		reqLog.Info("copilot.responses.billing_eligibility_check_failed", zap.Error(err))
		status, code, message := billingErrorDetails(err)
		h.errorResponse(c, status, code, message)
		return
	}

	ctx := c.Request.Context()
	userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlot(ctx, subject.UserID, subject.Concurrency)
	if err != nil {
		reqLog.Warn("copilot.responses.user_slot_acquire_failed", zap.Error(err))
		h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Concurrency limit exceeded, please retry later")
		return
	}
	if !userAcquired {
		h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Too many concurrent requests, please retry later")
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStartResponses).Milliseconds())
	routingStartResponses := time.Now()

	failedAccountIDs := make(map[int64]struct{})
	switchCount := 0

	for {
		account, err := h.gatewayService.SelectAccountForModelWithExclusions(
			ctx,
			apiKey.GroupID,
			"",
			reqModel,
			failedAccountIDs,
		)
		if err != nil || account == nil {
			if ctx.Err() == context.Canceled {
				reqLog.Info("copilot.responses.client_disconnected", zap.Error(err))
				h.errorResponse(c, StatusClientClosedRequest, "client_closed", "Client disconnected")
			} else if len(failedAccountIDs) == 0 {
				reqLog.Warn("copilot.responses.account_select_failed", zap.Error(err))
				h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "No available Copilot accounts")
			} else {
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "All Copilot accounts failed")
			}
			return
		}
		reqLog.Debug("copilot.responses.account_selected",
			zap.Int64("account_id", account.ID),
			zap.String("account_name", account.Name))
		setOpsSelectedAccount(c, account.ID, account.Platform)

		if switchCount == 0 {
			service.AppendOpsSpan(c, service.OpsSpan{
				Name:        "routing.select",
				StartUnixMs: routingStartResponses.UnixMilli(),
				DurationMs:  time.Since(routingStartResponses).Milliseconds(),
				Status:      "ok",
				Attrs:       map[string]any{"account_id": account.ID, "platform": "copilot"},
			})
		} else {
			service.AppendOpsSpan(c, service.OpsSpan{
				Name:        "failover.select",
				StartUnixMs: time.Now().UnixMilli(),
				DurationMs:  0,
				Status:      "ok",
				Attrs:       map[string]any{"attempt": switchCount, "account_id": account.ID},
			})
		}

		// Reject oversized request bodies before hitting the upstream.
		if h.checkCopilotBodySize(c, body, account, false) {
			return
		}

		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStartResponses).Milliseconds())
		forwardStartResponses := time.Now()
		result, fwdErr := h.copilotGatewayService.ForwardResponses(ctx, c, account, body)
		forwardDurationMsResp := time.Since(forwardStartResponses).Milliseconds()
		if fwdErr == nil {
			upstreamMsResp, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
			responseMsResp := forwardDurationMsResp
			if upstreamMsResp > 0 && forwardDurationMsResp > upstreamMsResp {
				responseMsResp = forwardDurationMsResp - upstreamMsResp
			}
			service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseMsResp)
		}
		if fwdErr != nil {
			// Client disconnected — skip failover, no one is listening.
			if ctx.Err() == context.Canceled {
				reqLog.Info("copilot.responses.client_disconnected",
					zap.Int64("account_id", account.ID),
					zap.Int("switch_count", switchCount),
					zap.Error(fwdErr))
				h.errorResponse(c, StatusClientClosedRequest, "client_closed", "Client disconnected before upstream response")
				return
			}
			failedAccountIDs[account.ID] = struct{}{}
			switchCount++
			if switchCount >= h.maxAccountSwitches {
				reqLog.Warn("copilot.responses.failover_exhausted",
					zap.Int64("account_id", account.ID),
					zap.Int("switch_count", switchCount),
					zap.Error(fwdErr))
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
				return
			}
			reqLog.Warn("copilot.responses.upstream_failover_switching",
				zap.Int64("account_id", account.ID),
				zap.Int("switch_count", switchCount),
				zap.Error(fwdErr))
			continue
		}

		// 421 Misdirected Request: HTTP/2 connection-level error, failover to another account.
		if result != nil && result.StatusCode == http.StatusMisdirectedRequest {
			reqLog.Warn("copilot.responses.misdirected_request_failover",
				zap.Int64("account_id", account.ID),
				zap.Int("switch_count", switchCount))
			failedAccountIDs[account.ID] = struct{}{}
			switchCount++
			if switchCount >= h.maxAccountSwitches {
				reqLog.Warn("copilot.responses.failover_exhausted",
					zap.Int64("account_id", account.ID),
					zap.Int("switch_count", switchCount))
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
				return
			}
			continue
		}

		// 429 Too Many Requests: rate limited — failover to another account.
		if result != nil && result.StatusCode == http.StatusTooManyRequests {
			reqLog.Warn("copilot.responses.rate_limited_failover",
				zap.Int64("account_id", account.ID),
				zap.Int("switch_count", switchCount))
			failedAccountIDs[account.ID] = struct{}{}
			switchCount++
			if switchCount >= h.maxAccountSwitches {
				reqLog.Warn("copilot.responses.failover_exhausted",
					zap.Int64("account_id", account.ID),
					zap.Int("switch_count", switchCount))
				h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "All Copilot accounts are rate limited")
				return
			}
			continue
		}

		if result != nil && result.StatusCode != http.StatusOK {
			reqLog.Debug("copilot.responses.completed_with_error",
				zap.Int64("account_id", account.ID),
				zap.Int("status", result.StatusCode))
			return
		}

		if result != nil && result.Usage != nil {
			inboundEp := snapshotInboundForUsageLog(c)
			userAgent := c.GetHeader("User-Agent")
			clientIP := ip.GetClientIP(c)
			capturedResult := result
			capturedAccount := account
			authLatencyMsResp := getContextLatencyMsPtr(c, service.OpsAuthLatencyMsKey)
			routingLatencyMsResp := getContextLatencyMsPtr(c, service.OpsRoutingLatencyMsKey)
			upstreamLatencyMsRespVal := getContextLatencyMsPtr(c, service.OpsUpstreamLatencyMsKey)
			responseLatencyMsRespVal := getContextLatencyMsPtr(c, service.OpsResponseLatencyMsKey)
			capturedInitiatorResp := service.CopilotInitiatorFromBody(body)
			// Capture request-scoped values before entering goroutine (gin.Context not safe across goroutines).
			capturedRequestID := c.GetHeader("X-Request-ID")
			capturedReqBody := body
			capturedUpstreamReqBodyResp, capturedUpstreamRespBodyResp := service.GetOpsUpstreamBodies(c)
			go func() {
				recordCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				fwdResult := &service.ForwardResult{
					Model:         capturedResult.Model,
					UpstreamModel: capturedResult.UpstreamModel,
					Stream:        reqStream,
					Usage: service.ClaudeUsage{
						InputTokens:  capturedResult.Usage.PromptTokens,
						OutputTokens: capturedResult.Usage.CompletionTokens,
					},
					Duration:        capturedResult.Duration,
					FirstTokenMs:    capturedResult.FirstTokenMs,
					ReasoningEffort: capturedResult.ReasoningEffort,
				}
				if err := h.gatewayService.RecordUsage(recordCtx, &service.RecordUsageInput{
					Result:            fwdResult,
					APIKey:            apiKey,
					User:              apiKey.User,
					Account:           capturedAccount,
					Subscription:      subscription,
					InboundEndpoint:   inboundEp,
					UpstreamEndpoint:  EndpointResponses,
					UserAgent:         userAgent,
					IPAddress:         clientIP,
					RequestBodyBytes:  intPtr(len(body)),
					APIKeyService:     h.apiKeyService,
					AuthLatencyMs:     authLatencyMsResp,
					RoutingLatencyMs:  routingLatencyMsResp,
					UpstreamLatencyMs: upstreamLatencyMsRespVal,
					ResponseLatencyMs: responseLatencyMsRespVal,
					Initiator:         capturedInitiatorResp,
				}); err != nil {
					reqLog.Error("copilot.responses.record_usage_failed", zap.Error(err))
				}
				// Async anomaly detection — already in a goroutine, no extra go needed.
				if h.anomalyService != nil {
					userID := apiKey.UserID
					apiKeyID := apiKey.ID
					accountID := capturedAccount.ID
					h.anomalyService.WriteAnomalyLog(
						recordCtx,
						capturedResult.Usage.PromptTokens,
						capturedResult.Usage.CompletionTokens,
						capturedResult.Duration.Milliseconds(),
						200,
						&service.RequestLogInput{
							RequestID:            capturedRequestID,
							UserID:               &userID,
							APIKeyID:             &apiKeyID,
							AccountID:            &accountID,
							GroupID:              apiKey.GroupID,
							RequestBody:          capturedReqBody,
							UpstreamRequestBody:  capturedUpstreamReqBodyResp,
							UpstreamResponseBody: capturedUpstreamRespBodyResp,
						},
					)
				}
			}()
		}

		reqLog.Debug("copilot.responses.completed",
			zap.Int64("account_id", account.ID),
			zap.Int("switch_count", switchCount))
		return
	}
}

// checkCopilotBodySize checks if the request body exceeds the account's limit.
// Returns true and writes an error response if the limit is exceeded; the caller must return immediately.
// anthropicFmt selects the response format: true = Anthropic, false = OpenAI.
func (h *CopilotGatewayHandler) checkCopilotBodySize(c *gin.Context, body []byte, account *service.Account, anthropicFmt bool) bool {
	limit := account.GetMaxBodyBytes()
	if limit <= 0 {
		limit = h.defaultMaxBodyBytes
	}
	if len(body) <= limit {
		return false
	}
	msg := fmt.Sprintf(
		"Request body (%d KB) exceeds the limit for this account (%d KB). "+
			"Use /compact in Claude Code or reduce conversation history to shrink the context.",
		len(body)/1024, limit/1024,
	)
	if anthropicFmt {
		h.anthropicErrorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", msg)
	} else {
		h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", msg)
	}
	return true
}

// errorResponse returns OpenAI API format error response.
func (h *CopilotGatewayHandler) errorResponse(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

// anthropicErrorResponse returns an Anthropic-format error response.
func (h *CopilotGatewayHandler) anthropicErrorResponse(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

// Messages handles Copilot /v1/messages endpoint (Anthropic-compatible).
//
// This allows Claude Code and any Anthropic-protocol client to use GitHub
// Copilot accounts as the backend.  The handler translates the incoming
// Anthropic request to OpenAI format, forwards it to the Copilot API, and
// translates the response back to Anthropic format before returning.
func (h *CopilotGatewayHandler) Messages(c *gin.Context) {
	requestStartMessages := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.anthropicErrorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.anthropicErrorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.copilot_gateway.messages",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	// Read request body.
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.anthropicErrorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	if !gjson.ValidBytes(body) {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	// Extract model (required by Anthropic spec).
	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqModel := modelResult.String()
	reqStream := gjson.GetBytes(body, "stream").Bool()
	reqMaxTokens := int(gjson.GetBytes(body, "max_tokens").Int())
	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))
	reqLog.Debug("copilot.messages.request_size",
		zap.Int64("incoming_content_length", c.Request.ContentLength),
		zap.Int("incoming_body_bytes", len(body)),
		zap.Int("incoming_max_tokens", reqMaxTokens))

	setOpsRequestContext(c, reqModel, reqStream, body)

	// Intercept Claude Code probe requests (max_tokens=1 + haiku model, non-streaming).
	// Claude Code sends these to validate API connectivity; respond locally without hitting upstream.
	if isMaxTokensOneHaikuRequest(reqModel, reqMaxTokens, reqStream) {
		reqLog.Debug("copilot.messages.probe_intercept", zap.String("model", reqModel))
		sendMockInterceptResponse(c, reqModel, InterceptTypeMaxTokensOneHaiku)
		return
	}

	// Check billing eligibility.
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription); err != nil {
		reqLog.Info("copilot.messages.billing_eligibility_check_failed", zap.Error(err))
		status, code, message := billingErrorDetails(err)
		h.anthropicErrorResponse(c, status, code, message)
		return
	}

	// Acquire user concurrency slot.
	ctx := c.Request.Context()
	userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlot(ctx, subject.UserID, subject.Concurrency)
	if err != nil {
		reqLog.Warn("copilot.messages.user_slot_acquire_failed", zap.Error(err))
		h.anthropicErrorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Concurrency limit exceeded, please retry later")
		return
	}
	if !userAcquired {
		h.anthropicErrorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Too many concurrent requests, please retry later")
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStartMessages).Milliseconds())
	routingStartMessages := time.Now()

	// Select a Copilot account with failover.
	failedAccountIDs := make(map[int64]struct{})
	switchCount := 0

	for {
		account, err := h.gatewayService.SelectAccountForModelWithExclusions(
			ctx,
			apiKey.GroupID,
			"", // sessionHash
			reqModel,
			failedAccountIDs,
		)
		if err != nil || account == nil {
			if ctx.Err() == context.Canceled {
				reqLog.Info("copilot.messages.client_disconnected", zap.Error(err))
				h.anthropicErrorResponse(c, StatusClientClosedRequest, "client_closed", "Client disconnected")
			} else if len(failedAccountIDs) == 0 {
				reqLog.Warn("copilot.messages.account_select_failed", zap.Error(err))
				h.anthropicErrorResponse(c, http.StatusServiceUnavailable, "api_error", "No available Copilot accounts")
			} else {
				h.anthropicErrorResponse(c, http.StatusBadGateway, "api_error", "All Copilot accounts failed")
			}
			return
		}
		reqLog.Debug("copilot.messages.account_selected",
			zap.Int64("account_id", account.ID),
			zap.String("account_name", account.Name))
		setOpsSelectedAccount(c, account.ID, account.Platform)

		if switchCount == 0 {
			service.AppendOpsSpan(c, service.OpsSpan{
				Name:        "routing.select",
				StartUnixMs: routingStartMessages.UnixMilli(),
				DurationMs:  time.Since(routingStartMessages).Milliseconds(),
				Status:      "ok",
				Attrs:       map[string]any{"account_id": account.ID, "platform": "copilot"},
			})
		} else {
			service.AppendOpsSpan(c, service.OpsSpan{
				Name:        "failover.select",
				StartUnixMs: time.Now().UnixMilli(),
				DurationMs:  0,
				Status:      "ok",
				Attrs:       map[string]any{"attempt": switchCount, "account_id": account.ID},
			})
		}

		// Truncate oversized request bodies before forwarding to the upstream.
		// When the body exceeds the account limit we trim old conversation turns
		// (preserving the system prompt and tools) so the request can proceed
		// rather than immediately rejecting with 413.  If the body cannot be
		// brought within the limit even after truncation, we reject.
		limit := account.GetMaxBodyBytes()
		if limit <= 0 {
			limit = h.defaultMaxBodyBytes
		}
		if len(body) > limit {
			trimmed, wasTruncated, truncErr := service.TruncateAnthropicBodyToLimit(body, limit)
			if truncErr != nil {
				reqLog.Warn("copilot.messages.truncate_error",
					zap.Int64("account_id", account.ID),
					zap.Int("original_bytes", len(body)),
					zap.Int("limit_bytes", limit),
					zap.Error(truncErr))
				// Fall through to the size check which will reject.
			} else if wasTruncated {
				if len(trimmed) <= limit {
					reqLog.Info("copilot.messages.context_truncated",
						zap.Int64("account_id", account.ID),
						zap.Int("original_bytes", len(body)),
						zap.Int("trimmed_bytes", len(trimmed)),
						zap.Int("limit_bytes", limit))
					body = trimmed
					setOpsRequestContext(c, reqModel, reqStream, body)
				} else {
					// Even after truncation still over limit — reject.
					reqLog.Warn("copilot.messages.truncate_insufficient",
						zap.Int64("account_id", account.ID),
						zap.Int("trimmed_bytes", len(trimmed)),
						zap.Int("limit_bytes", limit))
				}
			}
		}
		// Final size guard — rejects only if truncation was insufficient or errored.
		if h.checkCopilotBodySize(c, body, account, true) {
			return
		}

		// Forward request, translating Anthropic ↔ Copilot.
		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStartMessages).Milliseconds())
		forwardStartMessages := time.Now()
		result, fwdErr := h.copilotGatewayService.ForwardMessages(ctx, c, account, body)
		forwardDurationMsMsg := time.Since(forwardStartMessages).Milliseconds()
		if fwdErr == nil {
			upstreamMsMsg, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
			responseMsMsg := forwardDurationMsMsg
			if upstreamMsMsg > 0 && forwardDurationMsMsg > upstreamMsMsg {
				responseMsMsg = forwardDurationMsMsg - upstreamMsMsg
			}
			service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseMsMsg)
		}
		if fwdErr != nil {
			// Client disconnected — skip failover, no one is listening.
			if ctx.Err() == context.Canceled {
				reqLog.Info("copilot.messages.client_disconnected",
					zap.Int64("account_id", account.ID),
					zap.Int("switch_count", switchCount),
					zap.Error(fwdErr))
				h.anthropicErrorResponse(c, StatusClientClosedRequest, "client_closed", "Client disconnected before upstream response")
				return
			}
			failedAccountIDs[account.ID] = struct{}{}
			switchCount++
			if switchCount >= h.maxAccountSwitches {
				reqLog.Warn("copilot.messages.failover_exhausted",
					zap.Int64("account_id", account.ID),
					zap.Int("switch_count", switchCount),
					zap.Error(fwdErr))
				h.anthropicErrorResponse(c, http.StatusBadGateway, "api_error", "Upstream request failed")
				return
			}
			reqLog.Warn("copilot.messages.upstream_failover_switching",
				zap.Int64("account_id", account.ID),
				zap.Int("switch_count", switchCount),
				zap.Error(fwdErr))
			continue
		}

		// 421 Misdirected Request: HTTP/2 connection-level error, failover to another account.
		if result != nil && result.StatusCode == http.StatusMisdirectedRequest {
			reqLog.Warn("copilot.messages.misdirected_request_failover",
				zap.Int64("account_id", account.ID),
				zap.Int("switch_count", switchCount))
			failedAccountIDs[account.ID] = struct{}{}
			switchCount++
			if switchCount >= h.maxAccountSwitches {
				reqLog.Warn("copilot.messages.failover_exhausted",
					zap.Int64("account_id", account.ID),
					zap.Int("switch_count", switchCount))
				h.anthropicErrorResponse(c, http.StatusBadGateway, "api_error", "Upstream request failed")
				return
			}
			continue
		}

		// 429 Too Many Requests: rate limited — failover to another account.
		if result != nil && result.StatusCode == http.StatusTooManyRequests {
			reqLog.Warn("copilot.messages.rate_limited_failover",
				zap.Int64("account_id", account.ID),
				zap.Int("switch_count", switchCount))
			failedAccountIDs[account.ID] = struct{}{}
			switchCount++
			if switchCount >= h.maxAccountSwitches {
				reqLog.Warn("copilot.messages.failover_exhausted",
					zap.Int64("account_id", account.ID),
					zap.Int("switch_count", switchCount))
				h.anthropicErrorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "All Copilot accounts are rate limited")
				return
			}
			continue
		}

		if result != nil && result.StatusCode != http.StatusOK {
			reqLog.Debug("copilot.messages.completed_with_error",
				zap.Int64("account_id", account.ID),
				zap.Int("status", result.StatusCode))
			return
		}

		// Record usage to database.
		if result != nil && result.Usage != nil {
			inboundEp := snapshotInboundForUsageLog(c)
			userAgent := c.GetHeader("User-Agent")
			clientIP := ip.GetClientIP(c)
			capturedResult := result
			capturedAccount := account
			authLatencyMsMsg := getContextLatencyMsPtr(c, service.OpsAuthLatencyMsKey)
			routingLatencyMsMsg := getContextLatencyMsPtr(c, service.OpsRoutingLatencyMsKey)
			upstreamLatencyMsMsgVal := getContextLatencyMsPtr(c, service.OpsUpstreamLatencyMsKey)
			responseLatencyMsMsgVal := getContextLatencyMsPtr(c, service.OpsResponseLatencyMsKey)
			capturedInitiatorMsg := service.CopilotInitiatorFromBody(body)
			// Capture request-scoped values before entering goroutine (gin.Context not safe across goroutines).
			capturedRequestIDMsg := c.GetHeader("X-Request-ID")
			capturedReqBodyMsg := body
			capturedUpstreamReqBodyMsg, capturedUpstreamRespBodyMsg := service.GetOpsUpstreamBodies(c)
			go func() {
				recordCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				fwdResult := &service.ForwardResult{
					Model:         capturedResult.Model,
					UpstreamModel: capturedResult.UpstreamModel,
					Stream:        reqStream,
					Usage: service.ClaudeUsage{
						InputTokens:  capturedResult.Usage.PromptTokens,
						OutputTokens: capturedResult.Usage.CompletionTokens,
					},
					Duration:     capturedResult.Duration,
					FirstTokenMs: capturedResult.FirstTokenMs,
				}
				if err := h.gatewayService.RecordUsage(recordCtx, &service.RecordUsageInput{
					Result:            fwdResult,
					APIKey:            apiKey,
					User:              apiKey.User,
					Account:           capturedAccount,
					Subscription:      subscription,
					InboundEndpoint:   inboundEp,
					UpstreamEndpoint:  EndpointChatCompletions,
					UserAgent:         userAgent,
					IPAddress:         clientIP,
					RequestBodyBytes:  intPtr(len(body)),
					APIKeyService:     h.apiKeyService,
					AuthLatencyMs:     authLatencyMsMsg,
					RoutingLatencyMs:  routingLatencyMsMsg,
					UpstreamLatencyMs: upstreamLatencyMsMsgVal,
					ResponseLatencyMs: responseLatencyMsMsgVal,
					Initiator:         capturedInitiatorMsg,
				}); err != nil {
					reqLog.Error("copilot.messages.record_usage_failed", zap.Error(err))
				}
				// Async anomaly detection — already in a goroutine, no extra go needed.
				if h.anomalyService != nil {
					userID := apiKey.UserID
					apiKeyID := apiKey.ID
					accountID := capturedAccount.ID
					h.anomalyService.WriteAnomalyLog(
						recordCtx,
						capturedResult.Usage.PromptTokens,
						capturedResult.Usage.CompletionTokens,
						capturedResult.Duration.Milliseconds(),
						200,
						&service.RequestLogInput{
							RequestID:            capturedRequestIDMsg,
							UserID:               &userID,
							APIKeyID:             &apiKeyID,
							AccountID:            &accountID,
							GroupID:              apiKey.GroupID,
							RequestBody:          capturedReqBodyMsg,
							UpstreamRequestBody:  capturedUpstreamReqBodyMsg,
							UpstreamResponseBody: capturedUpstreamRespBodyMsg,
						},
					)
				}
			}()
		}

		reqLog.Debug("copilot.messages.completed",
			zap.Int64("account_id", account.ID),
			zap.Int("switch_count", switchCount))
		return
	}
}

// snapshotInboundForUsageLog returns the canonical inbound path stored in usage_logs.
// Call it on the request goroutine before starting async RecordUsage: Gin may reuse
// *gin.Context after the handler returns, so do not call GetInboundEndpoint(c) inside go func().
func snapshotInboundForUsageLog(c *gin.Context) string {
	if c == nil {
		return ""
	}
	s := strings.TrimSpace(GetInboundEndpoint(c))
	if s != "" {
		return s
	}
	path := ""
	if c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	return NormalizeInboundEndpoint(path)
}
