package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/copilot"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// CopilotGatewayService handles forwarding requests to the GitHub Copilot API.
//
// It supports:
//   - /chat/completions (OpenAI-compatible format, streaming and non-streaming)
//   - /models (list available models)
//
// Authentication is handled via CopilotTokenProvider, which exchanges
// GitHub tokens for short-lived Copilot API tokens.
type CopilotGatewayService struct {
	tokenProvider *CopilotTokenProvider
	httpClient    *http.Client

	// modelEndpointsCacheMu protects modelEndpointsCache.
	modelEndpointsCacheMu sync.RWMutex
	// modelEndpointsCache maps accountID → per-model supported_endpoints, cached from /models.
	modelEndpointsCache map[int64]*copilotModelEndpointsCacheEntry

	// platformConfigSvc 用于读取账号级平台配置（继承等逻辑）。
	platformConfigSvc *CopilotPlatformConfigService

	// sessionCache tracks per-session Premium quota: first request pays user quota,
	// subsequent requests in the same session use free agent quota.
	sessionCache *copilotSessionCache
}

// copilotModelEndpointsCacheEntry holds a per-account cache of model→supported_endpoints.
type copilotModelEndpointsCacheEntry struct {
	// endpointsByModel maps model ID → supported_endpoints slice from Copilot /models response.
	endpointsByModel map[string][]string
	cachedAt         time.Time
	fromUpstream     bool // true = live data (long TTL); false = failed fetch (short retry TTL)
}

const (
	// copilotModelEndpointsCacheTTL is how long to keep a successful /models fetch.
	copilotModelEndpointsCacheTTL = 1 * time.Hour
	// copilotModelEndpointsCacheFailedTTL is the retry window after a failed fetch.
	copilotModelEndpointsCacheFailedTTL = 2 * time.Minute
	// copilotChatFileTextPreviewMaxBytes caps inline text extracted from text-like
	// file attachments when downgrading file parts to plain text for /chat/completions.
	copilotChatFileTextPreviewMaxBytes = 16 * 1024
)

// NewCopilotGatewayService creates a new CopilotGatewayService.
func NewCopilotGatewayService(
	tokenProvider *CopilotTokenProvider,
) *CopilotGatewayService {
	// Use HTTP/1.1 only to avoid 421 Misdirected Request errors.
	//
	// The Copilot API returns 421 when an HTTP/2 connection established for one
	// account token is subsequently reused with a different Bearer token.
	// Setting NextProtos to ["http/1.1"] prevents ALPN from negotiating HTTP/2
	// during TLS handshake, so every connection is HTTP/1.1.  ForceAttemptHTTP2
	// is also false to ensure the stdlib's http2 package never upgrades.
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			// Explicitly advertise only HTTP/1.1 during TLS ALPN negotiation.
			// Without this, Go's net/http will negotiate h2 even when
			// ForceAttemptHTTP2 is false, because the server supports h2.
			NextProtos: []string{"http/1.1"},
		},
		ForceAttemptHTTP2:   false,
		DisableKeepAlives:   false,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
	}
	svc := &CopilotGatewayService{
		tokenProvider: tokenProvider,
		httpClient: &http.Client{
			Timeout:   5 * time.Minute, // long timeout for streaming
			Transport: transport,
		},
		modelEndpointsCache: make(map[int64]*copilotModelEndpointsCacheEntry),
		sessionCache:        newCopilotSessionCache(2 * time.Hour),
	}

	// Start background eviction: runs every 10 minutes to prevent unbounded
	// memory growth from expired session entries.
	// The goroutine leaks only when the process exits — acceptable for a
	// long-lived service singleton.
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			svc.sessionCache.evictExpired()
		}
	}()

	return svc
}

// SetPlatformConfigService 注入平台配置服务（供继承逻辑使用）。
func (s *CopilotGatewayService) SetPlatformConfigService(svc *CopilotPlatformConfigService) {
	s.platformConfigSvc = svc
}

// CopilotForwardResult holds the result of a Copilot API request.
type CopilotForwardResult struct {
	StatusCode int
	Model      string
	// UpstreamModel is the model id in the JSON body sent to Copilot (after mapping/normalization).
	UpstreamModel   string
	Usage           *CopilotUsage
	Duration        time.Duration // 请求总耗时
	FirstTokenMs    *int          // 首token时间（流式请求，毫秒）
	ReasoningEffort *string       // 推理强度（Responses API 请求，如 "low"/"medium"/"high"）
}

// CopilotUsage tracks token usage from a Copilot API response.
type CopilotUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ForwardChatCompletions forwards a chat/completions request to the Copilot API.
func (s *CopilotGatewayService) ForwardChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
) (*CopilotForwardResult, error) {
	startTime := time.Now()

	// GitHub Copilot /chat/completions rejects "file" content parts (only
	// "text" and "image_url" are accepted). When the client sends file
	// attachments (e.g. Cherry Studio PDF upload), we check the model's
	// supported_endpoints first:
	//   - Models that support /responses (including dual-endpoint models such as
	//     claude-opus-4.6): bridge through /responses which accepts inline
	//     base64 file input, then translate the response back to Chat
	//     Completions format.
	//   - Models that only support /chat/completions: convert file parts to
	//     descriptive text parts so /chat/completions can still receive the
	//     attachment context.
	//
	// chatFallbackBody holds the same request with "file" parts rewritten to
	// text parts, used as the direct path for chat-only models and as the
	// fallback if /responses later rejects the model.
	strippedBody, hasFile := StripUnsupportedContentPartsFromOpenAIBody(body)
	slog.Info("copilot chat: file detection", "has_file", hasFile, "body_len", len(body),
		"body_preview", func() string {
			if len(body) > 500 {
				return string(body[:500])
			}
			return string(body)
		}())
	if hasFile {
		chatFallbackBody := convertFilePartsToText(body)
		if bytes.Equal(chatFallbackBody, body) {
			// Text conversion failed unexpectedly — use stripped body as safety net.
			chatFallbackBody = strippedBody
		}

		// For file attachments, prefer /responses whenever the model supports it
		// so Copilot can preserve the original file input instead of downgrading
		// it to a text placeholder.
		//
		// Exception: accounts with a custom base_url bypass the canonical
		// api.githubcopilot.com, so /responses (which is only available there)
		// would silently route around their configured proxy. In that case fall
		// back to the textified /chat/completions path instead.
		hasCustomBaseURL := strings.TrimSpace(account.GetCredential("base_url")) != ""

		modelProbeBody, _ := rewriteCopilotUpstreamModel(body, account, s.getCopilotPlatformModelMapping(ctx, account))
		upstreamModelID := strings.TrimSpace(extractModelFromBody(modelProbeBody))
		supportedEndpoints, epErr := s.getSupportedEndpointsForModel(ctx, account, upstreamModelID)
		slog.Info("copilot chat: file routing decision",
			"upstream_model_id", upstreamModelID,
			"has_custom_base_url", hasCustomBaseURL,
			"supported_endpoints", supportedEndpoints,
			"ep_err", epErr,
			"model_supports_responses", modelSupportsResponses(supportedEndpoints),
			"model_supports_messages", modelSupportsMessages(supportedEndpoints))
		if epErr != nil {
			slog.Warn("copilot chat: could not resolve supported endpoints for file request, falling back to /chat/completions",
				"account_id", account.ID,
				"model", upstreamModelID,
				"error", epErr)
		}

		if !hasCustomBaseURL && modelSupportsResponses(supportedEndpoints) {
			return s.forwardChatCompletionsViaResponses(ctx, c, account, body, chatFallbackBody, startTime)
		}
		// /responses not available — try native Anthropic /v1/messages which accepts
		// document blocks (type:"document") and can therefore process PDF attachments.
		if !hasCustomBaseURL && modelSupportsMessages(supportedEndpoints) {
			return s.forwardChatCompletionsViaMessages(ctx, c, account, body, chatFallbackBody, startTime)
		}
		// No file-capable endpoint available: downgrade to text and send via /chat/completions.
		return s.forwardChatCompletionsDirect(ctx, c, account, chatFallbackBody, startTime)
	}

	return s.forwardChatCompletionsDirect(ctx, c, account, body, startTime)
}

// forwardChatCompletionsDirect handles the standard Copilot /chat/completions
// forwarding flow.  It is used both by the normal path (no file attachments) and
// as a fallback from forwardChatCompletionsViaResponses when the upstream model
// does not support the Responses API.
func (s *CopilotGatewayService) forwardChatCompletionsDirect(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	startTime time.Time,
) (*CopilotForwardResult, error) {
	translateStart := time.Now()
	// Normalize the OpenAI request body before forwarding:
	// merge consecutive same-role messages so Copilot API doesn't reject with 400.
	// Claude Code (via cc-switch OpenAI mode) sends adjacent user messages
	// (e.g. <available-deferred-tools> + actual message) that Copilot rejects.
	body = mergeConsecutiveSameRoleMessagesInOpenAIBody(body)

	body, logModel := rewriteCopilotUpstreamModel(body, account, s.getCopilotPlatformModelMapping(ctx, account))
	body = s.clampCopilotUpstreamMaxTokens(ctx, body, account)
	// before ensureStreamIncludeUsage may add it.
	clientWantsUsageChunk := gjson.GetBytes(body, "stream_options.include_usage").Bool()
	// Inject stream_options.include_usage=true so Copilot appends a usage-summary
	// SSE chunk. Must be called before http.NewRequestWithContext builds the request.
	body = ensureStreamIncludeUsage(body)

	upstreamSent := strings.TrimSpace(extractModelFromBody(body))
	AppendOpsSpan(c, OpsSpan{
		Name:        "translate.req",
		StartUnixMs: translateStart.UnixMilli(),
		DurationMs:  time.Since(translateStart).Milliseconds(),
		Status:      "ok",
	})

	// Get Copilot API token
	tokenStart := time.Now()
	token, err := s.tokenProvider.GetAccessToken(ctx, account)
	AppendOpsSpan(c, OpsSpan{
		Name:        "token.fetch",
		StartUnixMs: tokenStart.UnixMilli(),
		DurationMs:  time.Since(tokenStart).Milliseconds(),
		Status: func() string {
			if err != nil {
				return "error"
			}
			return "ok"
		}(),
		Attrs: map[string]any{"account_id": account.ID},
	})
	if err != nil {
		return nil, fmt.Errorf("copilot auth: %w", err)
	}

	// Determine base URL for /chat/completions.
	// Priority: explicit base_url credential (legacy) → plan_type credential → default.
	// The plan_type credential ("individual" / "business" / "enterprise") maps to the
	// corresponding subdomain; unknown or empty values fall back to CopilotAPIBase.
	baseURL := copilot.CopilotAPIBase
	if customURL := strings.TrimSpace(account.GetCredential("base_url")); customURL != "" {
		baseURL = strings.TrimRight(customURL, "/")
	} else if planType := strings.TrimSpace(account.GetCredential("plan_type")); planType != "" {
		baseURL = copilot.ChatBaseURLForPlan(planType)
	}

	// Model for billing / logs: client-facing name before upstream rewrite.
	model := logModel
	if model == "" {
		model = extractModelFromBody(body)
	}

	// Detect streaming mode
	isStream := detectStreamMode(body)

	// Determine X-Initiator: "agent" for multi-turn conversations (has assistant/tool messages),
	// "user" for the first turn. Matches TypeScript copilot-api reference logic.
	initiator := copilotInitiator(body)
	// Session-level quota optimization: if this request belongs to a known
	// session (scoped to this account), override to "agent" regardless of
	// message history.
	// Priority: X-Session-ID header > metadata.user_id in body.user field.
	// Cache key is namespaced by account.ID to prevent cross-tenant pollution.
	if rawSK := sessionKeyFromHeader(c); rawSK != "" {
		sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
		if s.sessionCache.markAndCheckSeen(sk) {
			initiator = "agent"
		}
	} else if rawSK := extractSessionKeyFromOpenAIBody(body); rawSK != "" {
		sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
		if s.sessionCache.markAndCheckSeen(sk) {
			initiator = "agent"
		}
	}

	// Build upstream request
	upstreamURL := baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("copilot: build request: %w", err)
	}

	// Set Copilot-specific headers
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	for k, vals := range copilot.CopilotHeaders(initiator, false) {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	// Store the upstream request body (post-rewrite) for ops anomaly capture.
	setOpsUpstreamRequestBody(c, body)

	// Send request
	upstreamStart := time.Now()
	resp, err := s.httpClient.Do(req) //nolint:gosec // URL is from trusted Copilot API config
	upstreamDurationMs := time.Since(upstreamStart).Milliseconds()
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, upstreamDurationMs)
	upstreamStatus := "ok"
	if err != nil {
		upstreamStatus = "error"
	}
	AppendOpsSpan(c, OpsSpan{
		Name:        "upstream.post",
		StartUnixMs: upstreamStart.UnixMilli(),
		DurationMs:  upstreamDurationMs,
		Status:      upstreamStatus,
		Attrs: map[string]any{
			"account_id": account.ID,
			"endpoint":   "chat/completions",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("copilot: upstream request: %w", err)
	}

	slog.Debug("copilot upstream response",
		"account_id", account.ID,
		"model", model,
		"status", resp.StatusCode,
		"stream", isStream,
		"latency_ms", time.Since(startTime).Milliseconds())

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		return s.handleErrorResponse(c, resp, account)
	}

	// Handle streaming response
	if isStream {
		return s.handleStreamingResponse(c, resp, model, upstreamSent, startTime, clientWantsUsageChunk)
	}

	// Handle non-streaming response
	return s.handleNonStreamingResponse(c, resp, model, upstreamSent, startTime)
}

// forwardChatCompletionsViaResponses handles Chat Completions requests that
// contain "file" content parts (e.g. PDF attachments from Cherry Studio).
//
// GitHub Copilot /chat/completions does not accept file parts, but /responses
// supports inline base64 file input as "input_file". This function:
//  1. Converts the OpenAI Chat Completions request to Responses API format
//     (file parts become input_file parts via apicompat.ChatCompletionsToResponses).
//  2. Forwards the request to Copilot /responses (always streaming upstream).
//  3. Translates the Responses SSE stream back to Chat Completions format
//     (streaming or non-streaming depending on what the client requested).
func (s *CopilotGatewayService) forwardChatCompletionsViaResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	strippedBody []byte,
	startTime time.Time,
) (*CopilotForwardResult, error) {
	translateStart := time.Now()

	body = mergeConsecutiveSameRoleMessagesInOpenAIBody(body)
	body, logModel := rewriteCopilotUpstreamModel(body, account, s.getCopilotPlatformModelMapping(ctx, account))
	body = s.clampCopilotUpstreamMaxTokens(ctx, body, account)

	var chatReq apicompat.ChatCompletionsRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		return nil, fmt.Errorf("copilot chat via responses: parse request: %w", err)
	}

	clientWantsStream := chatReq.Stream
	clientWantsUsageChunk := chatReq.StreamOptions != nil && chatReq.StreamOptions.IncludeUsage

	responsesReq, err := apicompat.ChatCompletionsToResponses(&chatReq)
	if err != nil {
		// Conversion errors are client-side problems (e.g. malformed file parts).
		// Write 400 directly so the handler loop does not mistake this for an
		// upstream/account failure and attempt unnecessary failover.
		c.JSON(http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"type":    "invalid_request_error",
				"message": "Invalid file attachment: " + err.Error(),
			},
		})
		return &CopilotForwardResult{StatusCode: http.StatusBadRequest}, nil
	}
	// Always stream upstream — consistent with forwardMessagesViaResponses strategy.
	responsesReq.Stream = true

	responsesBody, err := json.Marshal(responsesReq)
	if err != nil {
		return nil, fmt.Errorf("copilot chat via responses: marshal responses body: %w", err)
	}

	upstreamModel := strings.TrimSpace(responsesReq.Model)
	model := logModel
	if model == "" {
		model = upstreamModel
	}

	AppendOpsSpan(c, OpsSpan{
		Name:        "translate.req",
		StartUnixMs: translateStart.UnixMilli(),
		DurationMs:  time.Since(translateStart).Milliseconds(),
		Status:      "ok",
	})

	tokenStart := time.Now()
	token, err := s.tokenProvider.GetAccessToken(ctx, account)
	AppendOpsSpan(c, OpsSpan{
		Name:        "token.fetch",
		StartUnixMs: tokenStart.UnixMilli(),
		DurationMs:  time.Since(tokenStart).Milliseconds(),
		Status: func() string {
			if err != nil {
				return "error"
			}
			return "ok"
		}(),
		Attrs: map[string]any{"account_id": account.ID},
	})
	if err != nil {
		return nil, fmt.Errorf("copilot chat via responses: auth: %w", err)
	}

	// /responses is only available on the canonical Copilot base URL.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, copilot.CopilotAPIBase+"/responses", bytes.NewReader(responsesBody))
	if err != nil {
		return nil, fmt.Errorf("copilot chat via responses: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	initiator := copilotInitiator(body)
	// Session-level quota optimization: override to "agent" if session already seen.
	if rawSK := sessionKeyFromHeader(c); rawSK != "" {
		sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
		if s.sessionCache.markAndCheckSeen(sk) {
			initiator = "agent"
		}
	} else if rawSK := extractSessionKeyFromOpenAIBody(body); rawSK != "" {
		sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
		if s.sessionCache.markAndCheckSeen(sk) {
			initiator = "agent"
		}
	}
	for k, vals := range copilot.CopilotHeaders(initiator, false) {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	setOpsUpstreamRequestBody(c, responsesBody)

	upstreamStart := time.Now()
	resp, err := s.httpClient.Do(req) //nolint:gosec // URL is from trusted Copilot API config
	upstreamDurationMs := time.Since(upstreamStart).Milliseconds()
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, upstreamDurationMs)
	AppendOpsSpan(c, OpsSpan{
		Name:        "upstream.post",
		StartUnixMs: upstreamStart.UnixMilli(),
		DurationMs:  upstreamDurationMs,
		Status: func() string {
			if err != nil {
				return "error"
			}
			return "ok"
		}(),
		Attrs: map[string]any{"account_id": account.ID, "endpoint": "responses"},
	})
	if err != nil {
		return nil, fmt.Errorf("copilot chat via responses: upstream request: %w", err)
	}

	slog.Debug("copilot upstream response",
		"account_id", account.ID,
		"model", upstreamModel,
		"status", resp.StatusCode,
		"stream", true,
		"latency_ms", upstreamDurationMs)

	if resp.StatusCode != http.StatusOK {
		// When the model doesn't support the Responses API (e.g. claude-opus-4.6),
		// Copilot returns 400 with code "unsupported_api_for_model".  Fall back to
		// /chat/completions using the textified body so the model still receives
		// the attachment context as descriptive text rather than silently losing it.
		if resp.StatusCode == http.StatusBadRequest {
			errBody, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			if strings.TrimSpace(gjson.GetBytes(errBody, "error.code").String()) == "unsupported_api_for_model" {
				slog.Warn("copilot chat via responses: model does not support /responses, falling back to /chat/completions",
					"account_id", account.ID,
					"model", upstreamModel,
					"x_request_id", resp.Header.Get("x-request-id"))
				return s.forwardChatCompletionsDirect(ctx, c, account, strippedBody, startTime)
			}

			// Not the unsupported-model error — restore body so handleErrorResponse can read it.
			resp.Body = io.NopCloser(bytes.NewReader(errBody))
		}
		return s.handleErrorResponse(c, resp, account)
	}

	if clientWantsStream {
		return s.handleChatViaResponsesStreamingResponse(c, resp, model, upstreamModel, clientWantsUsageChunk, startTime)
	}
	return s.handleChatViaResponsesNonStreamingResponse(c, resp, model, upstreamModel, startTime)
}

// forwardChatCompletionsViaMessages handles Chat Completions requests that
// contain "file" content parts for models that support the Anthropic /v1/messages
// endpoint (e.g. claude-opus-4.6 on Copilot).
//
// Flow:
//  1. Convert OpenAI Chat Completions request (with type:"file" parts) to
//     Anthropic Messages format (type:"document" blocks).
//  2. POST to Copilot baseURL+/v1/messages with the Anthropic body.
//  3. Translate the Anthropic SSE stream back to Chat Completions format.
//
// Falls back to strippedBody via /chat/completions on any conversion or
// upstream error, preserving at least the text context.
func (s *CopilotGatewayService) forwardChatCompletionsViaMessages(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	strippedBody []byte,
	startTime time.Time,
) (*CopilotForwardResult, error) {
	translateStart := time.Now()

	body = mergeConsecutiveSameRoleMessagesInOpenAIBody(body)
	body, logModel := rewriteCopilotUpstreamModel(body, account, s.getCopilotPlatformModelMapping(ctx, account))
	body = s.clampCopilotUpstreamMaxTokens(ctx, body, account)

	clientWantsStream := gjson.GetBytes(body, "stream").Bool()

	// Convert OpenAI (with file parts) → Anthropic Messages format.
	anthropicBody, err := convertOpenAIChatToAnthropicMessages(body)
	if err != nil {
		slog.Warn("copilot chat via messages: failed to convert to anthropic format, falling back to /chat/completions",
			"account_id", account.ID,
			"model", logModel,
			"error", err)
		return s.forwardChatCompletionsDirect(ctx, c, account, strippedBody, startTime)
	}

	// Force streaming upstream — non-streaming Anthropic endpoint can hang.
	anthropicBody, _ = sjson.SetBytes(anthropicBody, "stream", true)

	upstreamModel := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	model := logModel
	if model == "" {
		model = upstreamModel
	}

	AppendOpsSpan(c, OpsSpan{
		Name:        "translate.req",
		StartUnixMs: translateStart.UnixMilli(),
		DurationMs:  time.Since(translateStart).Milliseconds(),
		Status:      "ok",
	})

	tokenStart := time.Now()
	token, err := s.tokenProvider.GetAccessToken(ctx, account)
	AppendOpsSpan(c, OpsSpan{
		Name:        "token.fetch",
		StartUnixMs: tokenStart.UnixMilli(),
		DurationMs:  time.Since(tokenStart).Milliseconds(),
		Status: func() string {
			if err != nil {
				return "error"
			}
			return "ok"
		}(),
		Attrs: map[string]any{"account_id": account.ID},
	})
	if err != nil {
		return nil, fmt.Errorf("copilot chat via messages: auth: %w", err)
	}

	// Use canonical base URL — /v1/messages is only available there.
	baseURL := copilot.CopilotAPIBase

	initiator := copilotInitiator(body)
	// Session-level quota optimization: override to "agent" if session already seen.
	if rawSK := sessionKeyFromHeader(c); rawSK != "" {
		sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
		if s.sessionCache.markAndCheckSeen(sk) {
			initiator = "agent"
		}
	} else if rawSK := extractSessionKeyFromOpenAIBody(body); rawSK != "" {
		sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
		if s.sessionCache.markAndCheckSeen(sk) {
			initiator = "agent"
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/messages", bytes.NewReader(anthropicBody))
	if err != nil {
		return nil, fmt.Errorf("copilot chat via messages: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	for k, vals := range copilot.CopilotHeaders(initiator, false) {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	setOpsUpstreamRequestBody(c, anthropicBody)

	upstreamStart := time.Now()
	resp, err := s.httpClient.Do(req) //nolint:gosec // URL is from trusted Copilot API config
	upstreamDurationMs := time.Since(upstreamStart).Milliseconds()
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, upstreamDurationMs)
	AppendOpsSpan(c, OpsSpan{
		Name:        "upstream.post",
		StartUnixMs: upstreamStart.UnixMilli(),
		DurationMs:  upstreamDurationMs,
		Status: func() string {
			if err != nil {
				return "error"
			}
			return "ok"
		}(),
		Attrs: map[string]any{"account_id": account.ID, "endpoint": "v1/messages"},
	})
	if err != nil {
		return nil, fmt.Errorf("copilot chat via messages: upstream request: %w", err)
	}

	slog.Debug("copilot upstream response",
		"account_id", account.ID,
		"model", upstreamModel,
		"status", resp.StatusCode,
		"stream", true,
		"latency_ms", upstreamDurationMs)

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		slog.Warn("copilot chat via messages: upstream error, falling back to /chat/completions",
			"account_id", account.ID,
			"model", upstreamModel,
			"status", resp.StatusCode,
			"body", truncateString(string(errBody), 512))
		return s.forwardChatCompletionsDirect(ctx, c, account, strippedBody, startTime)
	}

	// The upstream response is an Anthropic SSE stream.
	// Translate it to OpenAI Chat Completions format.
	if clientWantsStream {
		return s.handleChatViaMessagesStreamingResponse(c, resp, model, upstreamModel, startTime)
	}
	return s.handleChatViaMessagesNonStreamingResponse(c, resp, model, upstreamModel, startTime)
}
func (s *CopilotGatewayService) handleChatViaResponsesStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	model string,
	upstreamModel string,
	includeUsage bool,
	startTime time.Time,
) (*CopilotForwardResult, error) {
	defer func() { _ = resp.Body.Close() }()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("copilot chat via responses stream: writer does not support flushing")
	}

	state := apicompat.NewResponsesEventToChatState()
	state.Model = model
	state.IncludeUsage = includeUsage

	var firstTokenMs *int
	var usage *CopilotUsage

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	for scanner.Scan() {
		if err := c.Request.Context().Err(); err != nil {
			slog.Debug("copilot chat via responses stream: client disconnected", "error", err)
			break
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}

		data := line[6:]
		var evt apicompat.ResponsesStreamEvent
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			continue
		}

		chunks := apicompat.ResponsesEventToChatChunks(&evt, state)
		for _, chunk := range chunks {
			if firstTokenMs == nil {
				ms := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &ms
			}
			sse, err := apicompat.ChatChunkToSSE(chunk)
			if err != nil {
				continue
			}
			fmt.Fprint(c.Writer, sse)
			flusher.Flush()
		}

		// Capture usage from completion event.
		if state.Usage != nil && usage == nil {
			usage = &CopilotUsage{
				PromptTokens:     state.Usage.PromptTokens,
				CompletionTokens: state.Usage.CompletionTokens,
				TotalTokens:      state.Usage.TotalTokens,
			}
		}
	}

	// Emit finish chunk if the stream ended without a proper completion event.
	for _, chunk := range apicompat.FinalizeResponsesChatStream(state) {
		sse, err := apicompat.ChatChunkToSSE(chunk)
		if err == nil {
			fmt.Fprint(c.Writer, sse)
			flusher.Flush()
		}
	}
	fmt.Fprint(c.Writer, "data: [DONE]\n\n")
	flusher.Flush()

	if usage == nil && state.Usage != nil {
		usage = &CopilotUsage{
			PromptTokens:     state.Usage.PromptTokens,
			CompletionTokens: state.Usage.CompletionTokens,
			TotalTokens:      state.Usage.TotalTokens,
		}
	}

	return &CopilotForwardResult{
		StatusCode:    http.StatusOK,
		Model:         model,
		UpstreamModel: upstreamModel,
		Usage:         usage,
		Duration:      time.Since(startTime),
		FirstTokenMs:  firstTokenMs,
	}, nil
}

// handleChatViaResponsesNonStreamingResponse buffers the entire Responses API
// SSE stream from Copilot and returns a single Chat Completions JSON response.
func (s *CopilotGatewayService) handleChatViaResponsesNonStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	model string,
	upstreamModel string,
	startTime time.Time,
) (*CopilotForwardResult, error) {
	defer func() { _ = resp.Body.Close() }()

	// Accumulate the final response from terminal SSE events.
	// response.completed, response.incomplete and response.failed all carry a
	// Response object; we capture the first one we see as the terminal event.
	var finalResp *apicompat.ResponsesResponse
	var terminalEventType string

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}

		var evt apicompat.ResponsesStreamEvent
		if err := json.Unmarshal([]byte(line[6:]), &evt); err != nil {
			continue
		}

		switch evt.Type {
		case "response.completed", "response.incomplete", "response.failed":
			if finalResp == nil && evt.Response != nil {
				finalResp = evt.Response
				terminalEventType = evt.Type
			}
		}
	}

	// No terminal event received — the stream ended unexpectedly.
	if finalResp == nil {
		return nil, fmt.Errorf("copilot chat via responses: upstream stream ended without terminal event")
	}

	// Propagate upstream failures as gateway errors.
	if terminalEventType == "response.failed" {
		errMsg := "Upstream response failed"
		if finalResp.Error != nil {
			errMsg = finalResp.Error.Message
		}
		c.JSON(http.StatusBadGateway, map[string]any{
			"error": map[string]any{
				"type":    "upstream_error",
				"message": errMsg,
			},
		})
		return &CopilotForwardResult{StatusCode: http.StatusBadGateway}, nil
	}

	chatResp := apicompat.ResponsesToChatCompletions(finalResp, model)
	chatResp.Model = model

	var usage *CopilotUsage
	if finalResp.Usage != nil {
		usage = &CopilotUsage{
			PromptTokens:     finalResp.Usage.InputTokens,
			CompletionTokens: finalResp.Usage.OutputTokens,
			TotalTokens:      finalResp.Usage.InputTokens + finalResp.Usage.OutputTokens,
		}
	}

	respBytes, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("copilot chat via responses: marshal response: %w", err)
	}
	setOpsUpstreamResponseBody(c, respBytes)
	c.Data(http.StatusOK, "application/json", respBytes)

	return &CopilotForwardResult{
		StatusCode:    http.StatusOK,
		Model:         model,
		UpstreamModel: upstreamModel,
		Usage:         usage,
		Duration:      time.Since(startTime),
	}, nil
}

// handleChatViaMessagesStreamingResponse reads an Anthropic Messages API SSE
// stream from Copilot /v1/messages and translates each event into OpenAI Chat
// Completions SSE chunks, writing them directly to the client.
func (s *CopilotGatewayService) handleChatViaMessagesStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	model string,
	upstreamModel string,
	startTime time.Time,
) (*CopilotForwardResult, error) {
	defer func() { _ = resp.Body.Close() }()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("copilot chat via messages stream: writer does not support flushing")
	}

	usage := &CopilotUsage{}
	var firstTokenMs *int
	msgID := "chatcmpl-" + fmt.Sprintf("%d", time.Now().UnixMilli())
	// Emit the opening chunk with role.
	openChunk := map[string]any{
		"id":      msgID,
		"object":  "chat.completion.chunk",
		"model":   model,
		"choices": []map[string]any{{"index": 0, "delta": map[string]any{"role": "assistant", "content": ""}, "finish_reason": nil}},
	}
	openBytes, _ := json.Marshal(openChunk)
	fmt.Fprintf(c.Writer, "data: %s\n\n", openBytes)
	flusher.Flush()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 512*1024), 512*1024)

	var currentEventType string
	streamDone := false

	for scanner.Scan() {
		if err := c.Request.Context().Err(); err != nil {
			break
		}
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEventType = strings.TrimPrefix(line, "event: ")
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]
		if data == "[DONE]" {
			streamDone = true
			break
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			continue
		}

		switch currentEventType {
		case "content_block_delta":
			// Extract text_delta.
			var delta struct {
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &delta); err != nil || delta.Delta.Type != "text_delta" {
				continue
			}
			if firstTokenMs == nil {
				ms := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &ms
			}
			chunk := map[string]any{
				"id":      msgID,
				"object":  "chat.completion.chunk",
				"model":   model,
				"choices": []map[string]any{{"index": 0, "delta": map[string]any{"content": delta.Delta.Text}, "finish_reason": nil}},
			}
			chunkBytes, _ := json.Marshal(chunk)
			fmt.Fprintf(c.Writer, "data: %s\n\n", chunkBytes)
			flusher.Flush()

		case "message_delta":
			// Extract stop_reason and usage.
			var md struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &md); err != nil {
				continue
			}
			usage.CompletionTokens = md.Usage.OutputTokens
			finishReason := "stop"
			if md.Delta.StopReason == "max_tokens" {
				finishReason = "length"
			}
			finalChunk := map[string]any{
				"id":      msgID,
				"object":  "chat.completion.chunk",
				"model":   model,
				"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": finishReason}},
				"usage": map[string]any{
					"prompt_tokens":     usage.PromptTokens,
					"completion_tokens": usage.CompletionTokens,
					"total_tokens":      usage.PromptTokens + usage.CompletionTokens,
				},
			}
			finalBytes, _ := json.Marshal(finalChunk)
			fmt.Fprintf(c.Writer, "data: %s\n\n", finalBytes)
			flusher.Flush()

		case "message_start":
			// Extract input token usage.
			var ms struct {
				Message struct {
					Usage struct {
						InputTokens int `json:"input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(data), &ms); err == nil {
				usage.PromptTokens = ms.Message.Usage.InputTokens
			}
		}
	}

	if !streamDone {
		if err := scanner.Err(); err != nil {
			slog.Warn("copilot chat via messages stream scanner error", "error", err)
		}
	}

	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	flusher.Flush()

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return &CopilotForwardResult{
		StatusCode:    http.StatusOK,
		Model:         model,
		UpstreamModel: upstreamModel,
		Usage:         usage,
		Duration:      time.Since(startTime),
		FirstTokenMs:  firstTokenMs,
	}, nil
}

// handleChatViaMessagesNonStreamingResponse reads a full Anthropic Messages
// response (or Anthropic SSE stream) from Copilot /v1/messages and returns a
// single OpenAI Chat Completions JSON response.
func (s *CopilotGatewayService) handleChatViaMessagesNonStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	model string,
	upstreamModel string,
	startTime time.Time,
) (*CopilotForwardResult, error) {
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("copilot chat via messages: read response: %w", err)
	}

	// The upstream is always streaming (we force stream=true), so collect
	// all SSE lines and extract the final message_stop text.
	var fullText strings.Builder
	var inputTokens, outputTokens int
	var stopReason = "stop"

	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 512*1024), 512*1024)
	var currentEventType string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			currentEventType = strings.TrimPrefix(line, "event: ")
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]
		switch currentEventType {
		case "content_block_delta":
			var delta struct {
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &delta); err == nil && delta.Delta.Type == "text_delta" {
				_, _ = fullText.WriteString(delta.Delta.Text)
			}
		case "message_start":
			var ms struct {
				Message struct {
					Usage struct {
						InputTokens int `json:"input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(data), &ms); err == nil {
				inputTokens = ms.Message.Usage.InputTokens
			}
		case "message_delta":
			var md struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &md); err == nil {
				outputTokens = md.Usage.OutputTokens
				if md.Delta.StopReason == "max_tokens" {
					stopReason = "length"
				}
			}
		}
	}

	chatResp := map[string]any{
		"id":      "chatcmpl-" + fmt.Sprintf("%d", time.Now().UnixMilli()),
		"object":  "chat.completion",
		"model":   model,
		"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": fullText.String()}, "finish_reason": stopReason}},
		"usage": map[string]any{
			"prompt_tokens":     inputTokens,
			"completion_tokens": outputTokens,
			"total_tokens":      inputTokens + outputTokens,
		},
	}
	respBytes, err := json.Marshal(chatResp)
	if err != nil {
		return nil, fmt.Errorf("copilot chat via messages: marshal response: %w", err)
	}
	setOpsUpstreamResponseBody(c, respBytes)
	c.Data(http.StatusOK, "application/json", respBytes)

	return &CopilotForwardResult{
		StatusCode:    http.StatusOK,
		Model:         model,
		UpstreamModel: upstreamModel,
		Usage: &CopilotUsage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      inputTokens + outputTokens,
		},
		Duration: time.Since(startTime),
	}, nil
}

// handleStreamingResponse proxies SSE streaming from Copilot API to the client.
//
// forwardUsageChunk controls whether a Copilot-injected usage-summary chunk
// (appended when stream_options.include_usage=true is set upstream) is forwarded
// to the downstream client. Set to true only when the original client request
// contained stream_options.include_usage=true; otherwise the extra chunk is
// consumed internally for token tracking and filtered out to preserve protocol
// fidelity for clients that did not request it.
func (s *CopilotGatewayService) handleStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	model string,
	upstreamModel string,
	startTime time.Time,
	forwardUsageChunk bool,
) (*CopilotForwardResult, error) {
	defer func() { _ = resp.Body.Close() }()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("copilot: response writer does not support flushing")
	}

	usage := &CopilotUsage{}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	var firstTokenMs *int
	streamDone := false

	for scanner.Scan() {
		// Abort early if the client disconnected.
		if err := c.Request.Context().Err(); err != nil {
			slog.Debug("copilot stream: client disconnected", "error", err)
			return &CopilotForwardResult{
				StatusCode:    http.StatusOK,
				Model:         model,
				UpstreamModel: upstreamModel,
				Usage:         usage,
				Duration:      time.Since(startTime),
				FirstTokenMs:  firstTokenMs,
			}, nil
		}

		line := scanner.Text()

		// Parse usage from SSE data
		if strings.HasPrefix(line, "data: ") {
			data := line[6:]
			if data == "[DONE]" {
				streamDone = true
				// Forward [DONE] to the client, then stop reading.
				fmt.Fprintf(c.Writer, "%s\n", line)
				flusher.Flush()
				break
			}
			// Record first token time on first data chunk
			if firstTokenMs == nil {
				ms := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &ms
			}
			s.parseStreamUsage(data, usage)

			// Filter the upstream-injected usage-only chunk when the client did not
			// request it. Still parsed above so token counts are always recorded.
			if !forwardUsageChunk && isUsageOnlyChunk(data) {
				continue
			}
		}

		// Forward line to client
		fmt.Fprintf(c.Writer, "%s\n", line)
		flusher.Flush()
	}

	// Only treat scanner errors as failures when we never saw [DONE].
	// After a clean [DONE] the server closes the connection, which makes
	// scanner.Err() return an unexpected EOF even though the stream finished
	// successfully.
	if !streamDone {
		if err := scanner.Err(); err != nil {
			slog.Warn("copilot stream scanner error", "error", err)
		}
	}

	return &CopilotForwardResult{
		StatusCode:    http.StatusOK,
		Model:         model,
		UpstreamModel: upstreamModel,
		Usage:         usage,
		Duration:      time.Since(startTime),
		FirstTokenMs:  firstTokenMs,
	}, nil
}

// handleNonStreamingResponse proxies a non-streaming response from Copilot API.
func (s *CopilotGatewayService) handleNonStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	model string,
	upstreamModel string,
	startTime time.Time,
) (*CopilotForwardResult, error) {
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("copilot: read response: %w", err)
	}

	// Extract usage
	usage := s.parseNonStreamUsage(body)

	// Store upstream response body for ops anomaly capture (best-effort).
	setOpsUpstreamResponseBody(c, body)

	// Forward only safe, client-relevant headers (whitelist).
	// Forwarding all upstream headers would leak internal routing details
	// (X-Request-Id, X-Ratelimit-*, X-GitHub-*, etc.) to external clients.
	for _, k := range []string{"Content-Type", "Content-Length"} {
		if v := resp.Header.Get(k); v != "" {
			c.Header(k, v)
		}
	}
	c.Data(http.StatusOK, "application/json", body)

	return &CopilotForwardResult{
		StatusCode:    http.StatusOK,
		Model:         model,
		UpstreamModel: upstreamModel,
		Usage:         usage,
		Duration:      time.Since(startTime),
	}, nil
}

// handleErrorResponse handles non-200 responses from the Copilot API.
//
// For most error codes the response body is forwarded to the client immediately.
// 421 Misdirected Request is a connection-level error (HTTP/2 connection reused
// for a different virtual host) — we must NOT write the 421 to the client yet,
// because the handler loop needs to failover to another account first.  After
// calling CloseIdleConnections the next request will use a fresh connection.
func (s *CopilotGatewayService) handleErrorResponse(
	c *gin.Context,
	resp *http.Response,
	account *Account,
) (*CopilotForwardResult, error) {
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	slog.Warn("copilot upstream error",
		"account_id", account.ID,
		"status", resp.StatusCode,
		"body", string(body))

	// So ops / 请求排查 can show upstream text (Copilot often returns plain "Bad Request").
	if c != nil {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 2048 {
			msg = msg[:2048] + "..."
		}
		setOpsUpstreamError(c, resp.StatusCode, msg, "")
	}

	// Handle specific error codes
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		// Token may have expired, invalidate cache
		s.tokenProvider.InvalidateToken(account.ID)
	case http.StatusTooManyRequests:
		// Rate limited — do NOT write to client; return the status code so
		// the handler loop can failover to another account.
		return &CopilotForwardResult{StatusCode: resp.StatusCode}, nil
	case http.StatusMisdirectedRequest:
		// 421 is a connection-level error (HTTP/2 connection reused across
		// different Bearer tokens / virtual hosts).  Flush idle connections so
		// the next attempt gets a fresh TCP+TLS connection, then return without
		// writing anything to the client — the handler loop will failover.
		if t, ok := s.httpClient.Transport.(*http.Transport); ok {
			t.CloseIdleConnections()
		}
		return &CopilotForwardResult{StatusCode: resp.StatusCode}, nil
	}

	// Forward error to client
	c.Data(resp.StatusCode, "application/json", body)

	return &CopilotForwardResult{
		StatusCode: resp.StatusCode,
	}, nil
}

// CopilotInitiatorFromBody returns the Copilot initiator type for the given
// OpenAI-format request body: "agent" when the conversation already contains an
// assistant or tool message (multi-turn / sub-agent call), "user" otherwise.
//
// The value maps directly to the X-Initiator request header sent to GitHub
// Copilot and determines which quota bucket the request draws from:
//   - "user"  → Premium Interaction quota (paid)
//   - "agent" → standard quota (free sub-request)
func CopilotInitiatorFromBody(openAIBody []byte) string {
	return copilotInitiator(openAIBody)
}

// copilotInitiator returns the value for the X-Initiator header.
//
// When the conversation already includes an assistant or tool message
// (multi-turn / sub-agent call), Copilot expects "agent" which draws from the
// standard quota instead of the premium-interaction quota.  For fresh user
// messages the value is "user".
//
// This matches the logic in the TypeScript copilot-api reference:
//
//	const isAgentCall = payload.messages.some(msg =>
//	  ["assistant", "tool"].includes(msg.role))
func copilotInitiator(openAIBody []byte) string {
	var req struct {
		Messages []struct {
			Role string `json:"role"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(openAIBody, &req); err != nil {
		return "user"
	}
	for _, m := range req.Messages {
		if m.Role == "assistant" || m.Role == "tool" {
			return "agent"
		}
	}
	return "user"
}

// copilotInitiatorFromResponsesBody returns the value for the X-Initiator
// header for an OpenAI Responses API request body.
//
// The Responses API input may be either a plain string (first-turn text) or an
// array of typed input items. Copilot expects "agent" when the request already
// contains a previous_response_id, assistant content, or tool call/result
// items, otherwise "user".
func copilotInitiatorFromResponsesBody(body []byte) string {
	var req struct {
		PreviousResponseID string          `json:"previous_response_id"`
		Input              json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return "user"
	}
	if req.PreviousResponseID != "" {
		return "agent"
	}

	input := bytes.TrimSpace(req.Input)
	if len(input) == 0 || input[0] == '"' || input[0] != '[' {
		return "user"
	}

	var items []struct {
		Type string `json:"type"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(input, &items); err != nil {
		return "user"
	}
	for _, item := range items {
		if item.Role == "assistant" || item.Type == "function_call" || item.Type == "function_call_output" {
			return "agent"
		}
	}
	return "user"
}

// extractModelFromBody reads the model field from a JSON request body.
func extractModelFromBody(body []byte) string {
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}
	return req.Model
}

// rewriteCopilotUpstreamModel applies account model_mapping (then optional platform
// model_mapping as fallback) then Copilot API id normalization (Anthropic dated /
// dash ids → Copilot dot form). Returns the possibly updated body and the original
// model string from the body for logging.
//
// platformMapping may be nil; when provided it is checked only when the account-level
// mapping does not match.
func rewriteCopilotUpstreamModel(body []byte, account *Account, platformMapping map[string]string) ([]byte, string) {
	if len(body) == 0 || account == nil {
		return body, extractModelFromBody(body)
	}
	original := strings.TrimSpace(extractModelFromBody(body))
	if original == "" {
		return body, original
	}
	mapped, accountMatched := account.ResolveMappedModel(original)
	// Fall back to platform-level mapping when account mapping did not match.
	if !accountMatched && len(platformMapping) > 0 {
		if pm, ok := platformMapping[original]; ok && pm != "" {
			mapped = pm
		}
	}
	upstream := copilot.NormalizeModelIDForCopilotUpstream(mapped)
	if upstream == original {
		return body, original
	}
	newBody, err := sjson.SetBytes(body, "model", upstream)
	if err != nil {
		slog.Warn("copilot: rewrite model in body failed", "error", err)
		return body, original
	}
	return newBody, original
}

// getCopilotPlatformModelMapping returns the platform-level model_mapping for the given
// account (looked up by plan_type). Returns nil when not configured or on error.
func (s *CopilotGatewayService) getCopilotPlatformModelMapping(ctx context.Context, account *Account) map[string]string {
	if s.platformConfigSvc == nil || account == nil {
		return nil
	}
	planType := account.GetCredential("plan_type")
	if planType == "" {
		return nil
	}
	cfg, err := s.platformConfigSvc.GetByPlanType(ctx, planType)
	if err != nil || cfg == nil || len(cfg.ModelMapping) == 0 {
		return nil
	}
	return cfg.ModelMapping
}

// detectStreamMode checks if the request body has "stream": true.
func detectStreamMode(body []byte) bool {
	var req struct {
		Stream any `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	switch v := req.Stream.(type) {
	case bool:
		return v
	default:
		return false
	}
}

// extractCopilotReasoningEffort extracts the reasoning_effort from a Responses API
// request body. Checks both "reasoning.effort" (nested) and flat "reasoning_effort".
// Returns nil if not present or not a recognized value.
func extractCopilotReasoningEffort(body []byte) *string {
	// Try nested: {"reasoning": {"effort": "high"}}
	effort := strings.TrimSpace(gjson.GetBytes(body, "reasoning.effort").String())
	if effort == "" {
		// Fallback: flat field {"reasoning_effort": "high"}
		effort = strings.TrimSpace(gjson.GetBytes(body, "reasoning_effort").String())
	}
	if effort == "" {
		return nil
	}
	// Normalize to lowercase.
	normalized := strings.ToLower(effort)
	return &normalized
}

// parseStreamUsage extracts usage data from an SSE data line.
// Handles two formats:
//
// Chat Completions format (most models):
//
//	{"usage": {"prompt_tokens": N, "completion_tokens": M}}
//
// Responses API format (Codex and similar):
//
//	{"type": "response.completed", "response": {"usage": {"input_tokens": N, "output_tokens": M}}}
func (s *CopilotGatewayService) parseStreamUsage(data string, usage *CopilotUsage) {
	b := []byte(data)

	// Try Chat Completions format first.
	var ccChunk struct {
		Usage *CopilotUsage `json:"usage"`
	}
	if err := json.Unmarshal(b, &ccChunk); err == nil && ccChunk.Usage != nil &&
		(ccChunk.Usage.PromptTokens > 0 || ccChunk.Usage.CompletionTokens > 0) {
		usage.PromptTokens = ccChunk.Usage.PromptTokens
		usage.CompletionTokens = ccChunk.Usage.CompletionTokens
		usage.TotalTokens = ccChunk.Usage.TotalTokens
		return
	}

	// Try Responses API format: type = "response.completed".
	var respChunk struct {
		Type     string `json:"type"`
		Response struct {
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		} `json:"response"`
	}
	if err := json.Unmarshal(b, &respChunk); err == nil &&
		respChunk.Type == "response.completed" {
		usage.PromptTokens = respChunk.Response.Usage.InputTokens
		usage.CompletionTokens = respChunk.Response.Usage.OutputTokens
		usage.TotalTokens = respChunk.Response.Usage.InputTokens + respChunk.Response.Usage.OutputTokens
	}
}

// parseNonStreamUsage extracts usage data from a non-streaming response body.
// Handles both Chat Completions format (prompt_tokens/completion_tokens)
// and Responses API format (input_tokens/output_tokens).
func (s *CopilotGatewayService) parseNonStreamUsage(body []byte) *CopilotUsage {
	// Try Chat Completions format.
	var ccResp struct {
		Usage *CopilotUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &ccResp); err == nil && ccResp.Usage != nil &&
		(ccResp.Usage.PromptTokens > 0 || ccResp.Usage.CompletionTokens > 0) {
		return ccResp.Usage
	}

	// Try Responses API format.
	var respResp struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &respResp); err == nil &&
		(respResp.Usage.InputTokens > 0 || respResp.Usage.OutputTokens > 0) {
		return &CopilotUsage{
			PromptTokens:     respResp.Usage.InputTokens,
			CompletionTokens: respResp.Usage.OutputTokens,
			TotalTokens:      respResp.Usage.InputTokens + respResp.Usage.OutputTokens,
		}
	}

	return &CopilotUsage{}
}

// ListModels returns the list of models available on the Copilot API.
func (s *CopilotGatewayService) ListModels(
	ctx context.Context,
	account *Account,
) ([]byte, error) {
	token, err := s.tokenProvider.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("copilot auth: %w", err)
	}

	baseURL := copilot.CopilotAPIBase
	if customURL := strings.TrimSpace(account.GetCredential("base_url")); customURL != "" {
		baseURL = strings.TrimRight(customURL, "/")
	} else if planType := strings.TrimSpace(account.GetCredential("plan_type")); planType != "" {
		baseURL = copilot.ChatBaseURLForPlan(planType)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("copilot: build models request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	for k, vals := range copilot.CopilotHeaders("user", false) {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	resp, err := s.httpClient.Do(req) //nolint:gosec // URL is from trusted Copilot API config
	if err != nil {
		return nil, fmt.Errorf("copilot: models request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("copilot: read models response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("copilot: models HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Deduplicate model IDs — the Copilot API may return the same model ID
	// more than once (e.g. gpt-4o appears twice in some responses).
	body = deduplicateModelsList(body)

	return body, nil
}

// deduplicateModelsList removes duplicate model entries (by id) from an
// OpenAI-format /models JSON response. The first occurrence of each ID is kept.
func deduplicateModelsList(body []byte) []byte {
	var resp struct {
		Object string            `json:"object"`
		Data   []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return body
	}

	seen := make(map[string]struct{}, len(resp.Data))
	deduped := make([]json.RawMessage, 0, len(resp.Data))
	for _, raw := range resp.Data {
		var m struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &m); err != nil || m.ID == "" {
			deduped = append(deduped, raw)
			continue
		}
		if _, exists := seen[m.ID]; exists {
			continue
		}
		seen[m.ID] = struct{}{}
		deduped = append(deduped, raw)
	}

	resp.Data = deduped
	out, err := json.Marshal(resp)
	if err != nil {
		return body
	}
	return out
}

// OpenAI Responses API gateway (Codex CLI)
// ─────────────────────────────────────────────────────────────────────────────

// getModelEndpointsCache returns the cached endpoints map for the given account, if still valid.
func (s *CopilotGatewayService) getModelEndpointsCache(accountID int64) (map[string][]string, bool) {
	s.modelEndpointsCacheMu.RLock()
	defer s.modelEndpointsCacheMu.RUnlock()
	entry, ok := s.modelEndpointsCache[accountID]
	if !ok || entry == nil {
		return nil, false
	}
	ttl := copilotModelEndpointsCacheTTL
	if !entry.fromUpstream {
		ttl = copilotModelEndpointsCacheFailedTTL
	}
	if time.Since(entry.cachedAt) > ttl {
		return nil, false
	}
	return entry.endpointsByModel, true
}

// setModelEndpointsCache stores an endpoints map for the given account.
func (s *CopilotGatewayService) setModelEndpointsCache(accountID int64, endpointsByModel map[string][]string, fromUpstream bool) {
	s.modelEndpointsCacheMu.Lock()
	defer s.modelEndpointsCacheMu.Unlock()
	s.modelEndpointsCache[accountID] = &copilotModelEndpointsCacheEntry{
		endpointsByModel: endpointsByModel,
		cachedAt:         time.Now(),
		fromUpstream:     fromUpstream,
	}
}

// getSupportedEndpointsForModel returns the supported_endpoints list for modelID from the
// Copilot /models response.  Results are cached per account (1 hour TTL).
//
// On error it falls back to staticSupportedEndpoints so that well-known
// models (e.g. Claude sonnet/opus/haiku 4.x) are still routed correctly even
// when the /models fetch fails due to a transient token-exchange error.
func (s *CopilotGatewayService) getSupportedEndpointsForModel(ctx context.Context, account *Account, modelID string) ([]string, error) {
	if account == nil || strings.TrimSpace(modelID) == "" {
		return nil, nil
	}
	if cached, ok := s.getModelEndpointsCache(account.ID); ok {
		return cached[modelID], nil
	}
	body, err := s.ListModels(ctx, account)
	if err != nil {
		s.setModelEndpointsCache(account.ID, map[string][]string{}, false)
		// Fall back to static knowledge so a transient token-exchange error
		// does not permanently prevent file routing for known Claude models.
		return staticSupportedEndpoints(modelID), err
	}
	endpointsByModel := parseModelEndpointsFromModelsResponse(body)
	s.setModelEndpointsCache(account.ID, endpointsByModel, true)
	return endpointsByModel[modelID], nil
}

// staticSupportedEndpoints returns a best-effort static list of supported
// endpoints for well-known Copilot models, used as a fallback when the live
// /models fetch fails.  It covers Claude sonnet/opus/haiku 4.x which are
// known to support both /chat/completions and /responses.
func staticSupportedEndpoints(modelID string) []string {
	// All Claude 4.x models (any variant) support /responses.
	// Match both dash form ("claude-opus-4-6") and dot form ("claude-opus-4.6").
	if strings.HasPrefix(modelID, "claude-sonnet-4") ||
		strings.HasPrefix(modelID, "claude-opus-4") ||
		strings.HasPrefix(modelID, "claude-haiku-4") {
		return []string{"/chat/completions", "/responses"}
	}
	return nil
}

// parseModelEndpointsFromModelsResponse extracts a modelID→supported_endpoints map from
// the raw JSON returned by the Copilot /models endpoint.
//
// Each model ID is indexed under both the raw ID returned by the API (e.g.
// "claude-opus-4-6" with dashes) and its normalized upstream form (e.g.
// "claude-opus-4.6" with a dot) so that lookups using either variant succeed.
// This is necessary because the Copilot /models response uses dash-separated
// Claude IDs while NormalizeModelIDForCopilotUpstream converts them to
// dot-separated form for request forwarding.
func parseModelEndpointsFromModelsResponse(body []byte) map[string][]string {
	var resp struct {
		Data []struct {
			ID                 string   `json:"id"`
			SupportedEndpoints []string `json:"supported_endpoints"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return map[string][]string{}
	}
	m := make(map[string][]string, len(resp.Data)*2)
	for _, item := range resp.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		eps := append([]string(nil), item.SupportedEndpoints...)
		m[id] = eps
		// Also index under the normalized upstream form so callers using the
		// dot-separated variant (after NormalizeModelIDForCopilotUpstream) hit
		// the same entry.
		if normalized := copilot.NormalizeModelIDForCopilotUpstream(id); normalized != id {
			m[normalized] = eps
		}
	}
	return m
}

// modelSupportsResponses returns true when the model's supported_endpoints list
// contains "/responses", regardless of whether "/chat/completions" is also
// present. File attachments should prefer /responses whenever available so
// Copilot can preserve inline file input instead of downgrading it to text.
func modelSupportsResponses(supportedEndpoints []string) bool {
	if len(supportedEndpoints) == 0 {
		return false
	}
	for _, ep := range supportedEndpoints {
		if ep == "/responses" {
			return true
		}
	}
	return false
}

// modelSupportsMessages returns true when the model's supported_endpoints list
// contains "/v1/messages" (Anthropic Messages API).  Copilot exposes the
// Anthropic-native endpoint for Claude models, which accepts document blocks
// (type:"document") and can therefore process PDF attachments.
func modelSupportsMessages(supportedEndpoints []string) bool {
	for _, ep := range supportedEndpoints {
		if ep == "/v1/messages" {
			return true
		}
	}
	return false
}

// shouldUseResponsesEndpoint returns true when the model's supported_endpoints list
// contains "/responses" but NOT "/chat/completions" — meaning the model is
// responses-only and cannot be reached via the chat completions path.
func shouldUseResponsesEndpoint(supportedEndpoints []string) bool {
	if len(supportedEndpoints) == 0 {
		return false
	}
	hasResponses := false
	hasChatCompletions := false
	for _, ep := range supportedEndpoints {
		switch ep {
		case "/responses":
			hasResponses = true
		case "/chat/completions":
			hasChatCompletions = true
		}
	}
	return hasResponses && !hasChatCompletions
}

// ForwardResponses forwards an OpenAI Responses API request to the Copilot API.
//
// Codex CLI uses wire_api = "responses" which sends requests to {base_url}/responses.
// The request and response formats are passed through as-is — no translation needed
// since both sides speak the same Responses API protocol.
func (s *CopilotGatewayService) ForwardResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
) (*CopilotForwardResult, error) {
	startTime := time.Now()

	tokenStart := time.Now()
	token, err := s.tokenProvider.GetAccessToken(ctx, account)
	AppendOpsSpan(c, OpsSpan{
		Name:        "token.fetch",
		StartUnixMs: tokenStart.UnixMilli(),
		DurationMs:  time.Since(tokenStart).Milliseconds(),
		Status: func() string {
			if err != nil {
				return "error"
			}
			return "ok"
		}(),
		Attrs: map[string]any{"account_id": account.ID},
	})
	if err != nil {
		return nil, fmt.Errorf("copilot auth: %w", err)
	}

	// The /responses endpoint is only available on the canonical api.githubcopilot.com.
	// Plan-specific subdomains (individual, business, enterprise) and any legacy
	// custom base_url that points to a subdomain do NOT expose /responses — they
	// return 421 Misdirected Request.  Always use CopilotAPIBase here regardless of
	// the account's plan_type or base_url setting.
	baseURL := copilot.CopilotAPIBase

	translateStart := time.Now()
	// Extract reasoning_effort before model rewrite (body still has original model).
	// Uses the same gjson-based approach as extractOpenAIReasoningEffortFromBody.
	reasoningEffort := extractCopilotReasoningEffort(body)

	body, logModel := rewriteCopilotUpstreamModel(body, account, s.getCopilotPlatformModelMapping(ctx, account))
	upstreamSent := strings.TrimSpace(extractModelFromBody(body))
	model := logModel
	if model == "" {
		model = extractModelFromBody(body)
	}

	isStream := detectStreamMode(body)
	AppendOpsSpan(c, OpsSpan{
		Name:        "translate.req",
		StartUnixMs: translateStart.UnixMilli(),
		DurationMs:  time.Since(translateStart).Milliseconds(),
		Status:      "ok",
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("copilot responses: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	initiatorResp := copilotInitiatorFromResponsesBody(body)
	// Session-level quota optimization for Responses path (Codex CLI).
	// previous_response_id already causes copilotInitiatorFromResponsesBody to return
	// "agent" for chained responses. Session cache further handles concurrent sub-tasks
	// within the same session.
	if rawSK := sessionKeyFromHeader(c); rawSK != "" {
		sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
		if s.sessionCache.markAndCheckSeen(sk) {
			initiatorResp = "agent"
		}
	} else if rawSK := extractSessionKeyFromOpenAIBody(body); rawSK != "" {
		sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
		if s.sessionCache.markAndCheckSeen(sk) {
			initiatorResp = "agent"
		}
	}
	for k, vals := range copilot.CopilotHeaders(initiatorResp, false) {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	// Store the upstream request body for ops anomaly capture.
	setOpsUpstreamRequestBody(c, body)

	upstreamStartResponses := time.Now()
	resp, err := s.httpClient.Do(req) //nolint:gosec // URL is from trusted Copilot API config
	upstreamDurationMsResponses := time.Since(upstreamStartResponses).Milliseconds()
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, upstreamDurationMsResponses)
	upstreamStatusResponses := "ok"
	if err != nil {
		upstreamStatusResponses = "error"
	}
	AppendOpsSpan(c, OpsSpan{
		Name:        "upstream.post",
		StartUnixMs: upstreamStartResponses.UnixMilli(),
		DurationMs:  upstreamDurationMsResponses,
		Status:      upstreamStatusResponses,
		Attrs: map[string]any{
			"account_id": account.ID,
			"endpoint":   "responses",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("copilot responses: upstream request: %w", err)
	}

	slog.Debug("copilot responses upstream response",
		"account_id", account.ID,
		"model", model,
		"status", resp.StatusCode,
		"stream", isStream,
		"latency_ms", time.Since(startTime).Milliseconds())

	if resp.StatusCode != http.StatusOK {
		return s.handleErrorResponse(c, resp, account)
	}

	var result *CopilotForwardResult
	var fwdErr error
	if isStream {
		// ForwardResponses: usage comes from response.completed terminal event, not
		// stream_options.include_usage. forwardUsageChunk=false: no usage SSE chunk expected.
		result, fwdErr = s.handleStreamingResponse(c, resp, model, upstreamSent, startTime, false)
	} else {
		result, fwdErr = s.handleNonStreamingResponse(c, resp, model, upstreamSent, startTime)
	}
	if fwdErr != nil || result == nil {
		return result, fwdErr
	}
	// Attach reasoning_effort extracted from request body.
	result.ReasoningEffort = reasoningEffort
	return result, nil
}

// Anthropic /v1/messages gateway
// ─────────────────────────────────────────────────────────────────────────────

// containsImageBlock reports whether an Anthropic /v1/messages request body
// contains at least one top-level "image" content block inside messages[].
// It is used to set the Copilot-Vision-Request header so that Copilot enables
// vision capabilities for the request.
//
// Only direct "image" blocks are checked (i.e. content blocks whose type is
// "image").  Images nested inside tool_result content are intentionally
// excluded from this check; if Copilot requires the Vision header for those
// too, extend this function to recurse into tool_result.content.
func containsImageBlock(anthropicBody []byte) bool {
	var req struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(anthropicBody, &req); err != nil {
		return false
	}
	for _, m := range req.Messages {
		var blocks []struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(m.Content, &blocks); err != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type == "image" {
				return true
			}
		}
	}
	return false
}

// ForwardMessages receives an Anthropic /v1/messages request, translates it to
// OpenAI /chat/completions format, forwards it to the Copilot API, and
// translates the response back to Anthropic format.
//
// This allows Claude Code (and any Anthropic-compatible client) to use GitHub
// Copilot accounts as the backend without any client-side changes.
func (s *CopilotGatewayService) ForwardMessages(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	anthropicBody []byte,
) (*CopilotForwardResult, error) {
	startTime := time.Now()

	// Detect whether the *client* wants streaming (we need to know for the response path).
	clientWantsStream := detectAnthropicStream(anthropicBody)

	clientModel := strings.TrimSpace(gjson.GetBytes(anthropicBody, "model").String())

	translateStart := time.Now()
	// Translate Anthropic request → OpenAI format.
	openAIBody, err := translateAnthropicToOpenAI(anthropicBody, nil)
	if err != nil {
		return nil, fmt.Errorf("copilot messages: translate request: %w", err)
	}

	// Force stream=true for all upstream Copilot requests.
	// Copilot's non-streaming endpoint often hangs for 60s on Claude models,
	// causing context-canceled timeouts.  Streaming is much more reliable and
	// we can reassemble the chunks into a non-streaming Anthropic response
	// when the client requested stream=false.
	openAIBody = forceStreamTrue(openAIBody)
	// Inject stream_options.include_usage=true after forceStreamTrue has set stream=true.
	// This ensures Copilot appends a usage-summary SSE chunk so token counts are recorded.
	// Must be before the upstream request is built in forwardMessagesViaChatCompletions.
	openAIBody = ensureStreamIncludeUsage(openAIBody)

	openAIBody, _ = rewriteCopilotUpstreamModel(openAIBody, account, s.getCopilotPlatformModelMapping(ctx, account))

	// Same normalization as ForwardChatCompletions: GitHub Copilot often returns 400 when
	// the OpenAI-shaped body still has adjacent user/system messages. The Anthropic→OpenAI
	// translator merges most cases in sanitizeOpenAIMessages; this pass catches any
	// remaining edge cases after model rewrite (matches ericc-ch/copilot-api chat path).
	openAIBody = mergeConsecutiveSameRoleMessagesInOpenAIBody(openAIBody)
	openAIBody = s.clampCopilotUpstreamMaxTokens(ctx, openAIBody, account)

	upstreamSent := strings.TrimSpace(extractModelFromBody(openAIBody))
	translatedBodyBytes := len(openAIBody)

	// Determine which Copilot endpoint to use for this model.
	// Models like gpt-5.4 only support /responses; fall back to /chat/completions on error.
	supportedEndpoints, epErr := s.getSupportedEndpointsForModel(ctx, account, upstreamSent)
	if epErr != nil {
		slog.Warn("copilot messages: could not resolve supported endpoints, falling back to /chat/completions",
			"account_id", account.ID,
			"model", upstreamSent,
			"error", epErr)
	}
	useResponses := shouldUseResponsesEndpoint(supportedEndpoints)

	// Log / billing: preserve the client's Anthropic model id, not the Copilot wire id.
	model := clientModel
	if model == "" {
		model = extractModelFromBody(openAIBody)
	}
	AppendOpsSpan(c, OpsSpan{
		Name:        "translate.req",
		StartUnixMs: translateStart.UnixMilli(),
		DurationMs:  time.Since(translateStart).Milliseconds(),
		Status:      "ok",
	})

	// DEBUG: log the full translated request body to diagnose 400 Bad Request
	slog.Debug("copilot messages translated openai body",
		"account_id", account.ID,
		"model", model,
		"translated_body_bytes", translatedBodyBytes,
		"body", string(openAIBody))

	// Get Copilot API token.
	tokenStart := time.Now()
	token, err := s.tokenProvider.GetAccessToken(ctx, account)
	AppendOpsSpan(c, OpsSpan{
		Name:        "token.fetch",
		StartUnixMs: tokenStart.UnixMilli(),
		DurationMs:  time.Since(tokenStart).Milliseconds(),
		Status: func() string {
			if err != nil {
				return "error"
			}
			return "ok"
		}(),
		Attrs: map[string]any{"account_id": account.ID},
	})
	if err != nil {
		return nil, fmt.Errorf("copilot messages: auth: %w", err)
	}

	// When the model only supports /responses, translate the Anthropic request directly
	// to Responses API format and forward via that endpoint.
	// /responses is only available on the canonical CopilotAPIBase (not plan subdomains).
	if useResponses {
		return s.forwardMessagesViaResponses(ctx, c, account, token, anthropicBody, openAIBody, upstreamSent, model, startTime, clientWantsStream)
	}

	// Default path: /chat/completions.

	// Determine base URL.
	baseURL := copilot.CopilotAPIBase
	if customURL := strings.TrimSpace(account.GetCredential("base_url")); customURL != "" {
		baseURL = strings.TrimRight(customURL, "/")
	} else if planType := strings.TrimSpace(account.GetCredential("plan_type")); planType != "" {
		baseURL = copilot.ChatBaseURLForPlan(planType)
	}

	// Determine X-Initiator: "agent" when the conversation already has assistant
	// or tool turns (multi-turn / sub-agent call), "user" for the first turn.
	// This mirrors the TypeScript copilot-api reference implementation.
	initiator := copilotInitiator(openAIBody)
	// Session-level quota optimization for Anthropic Messages path.
	// Priority: X-Session-ID header > metadata.user_id in anthropicBody.
	if rawSK := sessionKeyFromHeader(c); rawSK != "" {
		sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
		if s.sessionCache.markAndCheckSeen(sk) {
			initiator = "agent"
		}
	} else if rawSK := extractSessionKeyFromAnthropicBody(anthropicBody); rawSK != "" {
		sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
		if s.sessionCache.markAndCheckSeen(sk) {
			initiator = "agent"
		}
	}

	// Detect whether the original Anthropic request contains any image blocks.
	// Copilot requires the Copilot-Vision-Request: true header to enable vision
	// capabilities; without it the model may reject or ignore image content.
	// We scan the original anthropicBody (before OpenAI translation) because
	// Anthropic uses {"type":"image"} blocks which map cleanly to this check.
	isVision := containsImageBlock(anthropicBody)

	// Build upstream request to Copilot /chat/completions.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(openAIBody))
	if err != nil {
		return nil, fmt.Errorf("copilot messages: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	for k, vals := range copilot.CopilotHeaders(initiator, isVision) {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	// Store the upstream request body (translated to OpenAI format) for ops anomaly capture.
	setOpsUpstreamRequestBody(c, openAIBody)

	upstreamStartMessages := time.Now()
	resp, err := s.httpClient.Do(req) //nolint:gosec // URL is from trusted Copilot API config
	upstreamDurationMsMessages := time.Since(upstreamStartMessages).Milliseconds()
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, upstreamDurationMsMessages)
	upstreamStatusMessages := "ok"
	if err != nil {
		upstreamStatusMessages = "error"
	}
	AppendOpsSpan(c, OpsSpan{
		Name:        "upstream.post",
		StartUnixMs: upstreamStartMessages.UnixMilli(),
		DurationMs:  upstreamDurationMsMessages,
		Status:      upstreamStatusMessages,
		Attrs: map[string]any{
			"account_id": account.ID,
			"endpoint":   "chat/completions",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("copilot messages: upstream request: %w", err)
	}

	slog.Debug("copilot messages upstream response",
		"account_id", account.ID,
		"model", model,
		"status", resp.StatusCode,
		"client_stream", clientWantsStream,
		"latency_ms", time.Since(startTime).Milliseconds())

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusBadRequest {
			slog.Warn("copilot messages upstream 400 (outgoing openai body snippet)",
				"account_id", account.ID,
				"model", model,
				"upstream_model", upstreamSent,
				"x_request_id", resp.Header.Get("x-request-id"),
				"openai_body_snip", truncateString(string(openAIBody), 2048))
		}
		return s.handleErrorResponse(c, resp, account)
	}

	if clientWantsStream {
		// Client wants SSE → stream Anthropic events directly.
		return s.handleMessagesStreamingResponse(c, resp, model, upstreamSent, startTime)
	}
	// Client wants non-streaming Anthropic JSON, but upstream is streaming.
	// Reassemble the SSE chunks into a single OpenAI response, then translate.
	return s.handleMessagesStreamToNonStreamingResponse(c, resp, model, upstreamSent, startTime)
}

// forwardMessagesViaResponses forwards a Claude Code /v1/messages request to the
// Copilot /responses endpoint (OpenAI Responses API).
//
// Used when the target model's supported_endpoints contains "/responses" but not
// "/chat/completions" (e.g. gpt-5.4, gpt-5.3-codex).
//
// Flow:
//  1. Translate original Anthropic body → Responses API request format
//  2. POST to CopilotAPIBase/responses (plan subdomains do not expose /responses)
//  3. Translate streaming Responses API events back to Anthropic SSE or JSON
func (s *CopilotGatewayService) forwardMessagesViaResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	token string,
	anthropicBody []byte,
	openAIBody []byte, // translated OpenAI body used to compute X-Initiator header
	upstreamModel string,
	clientModel string,
	startTime time.Time,
	clientWantsStream bool,
) (*CopilotForwardResult, error) {
	translateStartViaResp := time.Now()
	// Translate Anthropic → Responses API format.
	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(anthropicBody, &anthropicReq); err != nil {
		return nil, fmt.Errorf("copilot messages via responses: parse anthropic body: %w", err)
	}
	responsesReq, err := apicompat.AnthropicToResponses(&anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("copilot messages via responses: translate to responses: %w", err)
	}
	// Override model with the already-normalized upstream model id (post mapping).
	responsesReq.Model = upstreamModel
	// Always stream upstream for reliability (same strategy as chat/completions path).
	responsesReq.Stream = true

	responsesBody, err := json.Marshal(responsesReq)
	if err != nil {
		return nil, fmt.Errorf("copilot messages via responses: marshal responses body: %w", err)
	}
	AppendOpsSpan(c, OpsSpan{
		Name:        "translate.req",
		StartUnixMs: translateStartViaResp.UnixMilli(),
		DurationMs:  time.Since(translateStartViaResp).Milliseconds(),
		Status:      "ok",
	})

	slog.Debug("copilot messages via responses translated body",
		"account_id", account.ID,
		"model", upstreamModel,
		"body_bytes", len(responsesBody))

	// /responses is only available on the canonical base URL.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, copilot.CopilotAPIBase+"/responses", bytes.NewReader(responsesBody))
	if err != nil {
		return nil, fmt.Errorf("copilot messages via responses: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	// Use the translated OpenAI body to determine X-Initiator (agent vs user),
	// matching the same multi-turn detection logic as the /chat/completions path.
	initiator := copilotInitiator(openAIBody)
	// Session-level quota optimization for Anthropic→Responses bridge.
	if rawSK := sessionKeyFromHeader(c); rawSK != "" {
		sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
		if s.sessionCache.markAndCheckSeen(sk) {
			initiator = "agent"
		}
	} else if rawSK := extractSessionKeyFromAnthropicBody(anthropicBody); rawSK != "" {
		sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
		if s.sessionCache.markAndCheckSeen(sk) {
			initiator = "agent"
		}
	}
	for k, vals := range copilot.CopilotHeaders(initiator, containsImageBlock(anthropicBody)) {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	setOpsUpstreamRequestBody(c, responsesBody)

	upstreamStart := time.Now()
	resp, err := s.httpClient.Do(req) //nolint:gosec
	upstreamDurationMsViaResp := time.Since(upstreamStart).Milliseconds()
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, upstreamDurationMsViaResp)
	upstreamStatusViaResp := "ok"
	if err != nil {
		upstreamStatusViaResp = "error"
	}
	AppendOpsSpan(c, OpsSpan{
		Name:        "upstream.post",
		StartUnixMs: upstreamStart.UnixMilli(),
		DurationMs:  upstreamDurationMsViaResp,
		Status:      upstreamStatusViaResp,
		Attrs: map[string]any{
			"account_id": account.ID,
			"endpoint":   "responses",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("copilot messages via responses: upstream request: %w", err)
	}

	slog.Debug("copilot messages via responses upstream response",
		"account_id", account.ID,
		"model", upstreamModel,
		"status", resp.StatusCode,
		"latency_ms", time.Since(startTime).Milliseconds())

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusBadRequest {
			slog.Warn("copilot messages via responses upstream 400",
				"account_id", account.ID,
				"model", upstreamModel,
				"x_request_id", resp.Header.Get("x-request-id"),
				"body_snip", truncateString(string(responsesBody), 2048))
		}
		return s.handleErrorResponse(c, resp, account)
	}

	if clientWantsStream {
		return s.handleMessagesResponsesStreamingResponse(c, resp, clientModel, upstreamModel, startTime)
	}
	return s.handleMessagesResponsesStreamToNonStreamingResponse(c, resp, clientModel, upstreamModel, startTime)
}

// handleMessagesResponsesStreamingResponse translates a streaming Responses API response
// from Copilot back into Anthropic SSE events and writes them to the client.
func (s *CopilotGatewayService) handleMessagesResponsesStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	model string,
	upstreamModel string,
	startTime time.Time,
) (*CopilotForwardResult, error) {
	defer func() { _ = resp.Body.Close() }()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("copilot messages responses stream: writer does not support flushing")
	}

	state := apicompat.NewResponsesEventToAnthropicState()
	state.Model = model
	usage := &CopilotUsage{}
	var firstTokenMs *int

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	for scanner.Scan() {
		if err := c.Request.Context().Err(); err != nil {
			slog.Debug("copilot messages responses stream: client disconnected", "error", err)
			return &CopilotForwardResult{
				StatusCode: http.StatusOK, Model: model, UpstreamModel: upstreamModel,
				Usage: usage, Duration: time.Since(startTime), FirstTokenMs: firstTokenMs,
			}, nil
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}

		var event apicompat.ResponsesStreamEvent
		if err := json.Unmarshal([]byte(line[6:]), &event); err != nil {
			slog.Debug("copilot messages responses stream: skip unparseable event", "data", line[6:])
			continue
		}

		if firstTokenMs == nil {
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}

		// Capture usage from terminal events.
		if event.Response != nil && event.Response.Usage != nil {
			u := event.Response.Usage
			usage.PromptTokens = u.InputTokens
			usage.CompletionTokens = u.OutputTokens
			usage.TotalTokens = u.InputTokens + u.OutputTokens
		}

		events := apicompat.ResponsesEventToAnthropicEvents(&event, state)
		for _, evt := range events {
			sse, err := apicompat.ResponsesAnthropicEventToSSE(evt)
			if err != nil {
				continue
			}
			if _, writeErr := fmt.Fprint(c.Writer, sse); writeErr != nil {
				return &CopilotForwardResult{
					StatusCode: http.StatusOK, Model: model, UpstreamModel: upstreamModel,
					Usage: usage, Duration: time.Since(startTime), FirstTokenMs: firstTokenMs,
				}, nil
			}
		}
		if len(events) > 0 {
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("copilot messages responses stream: scanner error", "error", err)
		s.emitStreamErrorEvent(c.Writer, flusher, "overloaded_error", "Upstream connection interrupted, please retry.")
		return &CopilotForwardResult{
			StatusCode: http.StatusOK, Model: model, UpstreamModel: upstreamModel,
			Usage: usage, Duration: time.Since(startTime), FirstTokenMs: firstTokenMs,
		}, nil
	}

	// Send any remaining Anthropic events (message_stop, etc.).
	if finalEvents := apicompat.FinalizeResponsesAnthropicStream(state); len(finalEvents) > 0 {
		for _, evt := range finalEvents {
			sse, err := apicompat.ResponsesAnthropicEventToSSE(evt)
			if err != nil {
				continue
			}
			fmt.Fprint(c.Writer, sse) //nolint:errcheck
		}
		flusher.Flush()
	}

	return &CopilotForwardResult{
		StatusCode: http.StatusOK, Model: model, UpstreamModel: upstreamModel,
		Usage: usage, Duration: time.Since(startTime), FirstTokenMs: firstTokenMs,
	}, nil
}

// handleMessagesResponsesStreamToNonStreamingResponse reads a streaming Responses API
// response, collects the terminal event, translates it to Anthropic JSON, and writes
// the full non-streaming Anthropic response.
func (s *CopilotGatewayService) handleMessagesResponsesStreamToNonStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	model string,
	upstreamModel string,
	startTime time.Time,
) (*CopilotForwardResult, error) {
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	var finalResponse *apicompat.ResponsesResponse
	usage := &CopilotUsage{}
	var firstTokenMs *int

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}

		var event apicompat.ResponsesStreamEvent
		if err := json.Unmarshal([]byte(line[6:]), &event); err != nil {
			continue
		}

		if firstTokenMs == nil {
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}

		// The terminal event carries the full response object.
		if event.Response != nil &&
			(event.Type == "response.completed" || event.Type == "response.incomplete" || event.Type == "response.failed") {
			finalResponse = event.Response
			if event.Response.Usage != nil {
				u := event.Response.Usage
				usage.PromptTokens = u.InputTokens
				usage.CompletionTokens = u.OutputTokens
				usage.TotalTokens = u.InputTokens + u.OutputTokens
			}
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("copilot messages responses stream-to-nonstream: scanner error", "error", err)
	}

	if finalResponse == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "overloaded_error",
				"message": "Upstream stream ended without a terminal response event, please retry.",
			},
		})
		return &CopilotForwardResult{
			StatusCode: http.StatusServiceUnavailable, Model: model, UpstreamModel: upstreamModel,
			Usage: usage, Duration: time.Since(startTime), FirstTokenMs: firstTokenMs,
		}, nil
	}

	anthropicResp := apicompat.ResponsesToAnthropic(finalResponse, model)
	anthropicRespBody, err := json.Marshal(anthropicResp)
	if err != nil {
		return nil, fmt.Errorf("copilot messages via responses: marshal anthropic response: %w", err)
	}

	c.Data(http.StatusOK, "application/json", anthropicRespBody)
	return &CopilotForwardResult{
		StatusCode: http.StatusOK, Model: model, UpstreamModel: upstreamModel,
		Usage: usage, Duration: time.Since(startTime), FirstTokenMs: firstTokenMs,
	}, nil
}

func (s *CopilotGatewayService) handleMessagesStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	model string,
	upstreamModel string,
	startTime time.Time,
) (*CopilotForwardResult, error) {
	defer func() { _ = resp.Body.Close() }()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("copilot messages: response writer does not support flushing")
	}

	state := &copilotStreamState{
		toolCalls: make(map[int]copilotToolCallInfo),
	}
	usage := &CopilotUsage{}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	var firstTokenMs *int
	streamDone := false

	for scanner.Scan() {
		// Abort early if the client disconnected.
		if err := c.Request.Context().Err(); err != nil {
			slog.Debug("copilot messages stream: client disconnected", "error", err)
			return &CopilotForwardResult{
				StatusCode:    http.StatusOK,
				Model:         model,
				UpstreamModel: upstreamModel,
				Usage:         usage,
				Duration:      time.Since(startTime),
				FirstTokenMs:  firstTokenMs,
			}, nil
		}

		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			// Forward blank lines / non-data lines as-is to maintain SSE framing.
			fmt.Fprintf(c.Writer, "%s\n", line)
			flusher.Flush()
			continue
		}

		data := line[6:]
		if data == "[DONE]" {
			// Anthropic clients don't expect [DONE]; just stop.
			streamDone = true
			break
		}

		var chunk openAIChatStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			slog.Debug("copilot messages stream: skip unparseable chunk", "data", data)
			continue
		}

		// Record first token time on first parseable chunk.
		if firstTokenMs == nil {
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}

		// Accumulate usage.
		if chunk.Usage != nil {
			usage.PromptTokens = chunk.Usage.PromptTokens
			usage.CompletionTokens = chunk.Usage.CompletionTokens
			usage.TotalTokens = chunk.Usage.TotalTokens
		}

		// Translate chunk → Anthropic events.
		events := translateChunkToAnthropicEvents(&chunk, state)
		for _, evt := range events {
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", extractEventType(evt), evt)
			flusher.Flush()
		}
	}

	// Only treat scanner errors as failures when we never saw [DONE].
	// After a clean [DONE] the server closes the connection, which makes
	// scanner.Err() return an unexpected EOF even though the stream finished
	// successfully.  Emitting a spurious error event on a completed stream
	// would cause Claude Code to throw APIError on an otherwise-clean response.
	if !streamDone {
		if err := scanner.Err(); err != nil {
			slog.Warn("copilot messages stream scanner error", "error", err)
			// Send an Anthropic error event instead of synthesizing fake termination
			// events. The Anthropic SDK treats event: error as a terminal failure and
			// throws APIError, allowing Claude Code to detect the interruption and
			// retry automatically instead of getting stuck on incomplete tool JSON.
			s.emitStreamErrorEvent(c.Writer, flusher, "overloaded_error",
				"Upstream connection interrupted, please retry.")
		}
	}

	return &CopilotForwardResult{
		StatusCode:    http.StatusOK,
		Model:         model,
		UpstreamModel: upstreamModel,
		Usage:         usage,
		Duration:      time.Since(startTime),
		FirstTokenMs:  firstTokenMs,
	}, nil
}

// emitStreamErrorEvent sends an Anthropic error SSE event to the client.
// The Anthropic SDK interprets event: error as a terminal error and throws
// an APIError, allowing Claude Code to detect the failure and retry.
func (s *CopilotGatewayService) emitStreamErrorEvent(
	w io.Writer, flusher http.Flusher,
	errType string, message string,
) {
	errEvt, _ := json.Marshal(map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    errType,
			"message": message,
		},
	})
	fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(errEvt))
	flusher.Flush()
}

// handleMessagesStreamToNonStreamingResponse reads a streaming SSE response
// from the Copilot API, reassembles the chunks into a single OpenAI-format
// response, translates it to Anthropic format, and writes it as a normal
// (non-streaming) JSON response.
//
// This is used when the client sent stream=false but we forced stream=true
// upstream to avoid Copilot's non-streaming timeout issues.
func (s *CopilotGatewayService) handleMessagesStreamToNonStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	model string,
	upstreamModel string,
	startTime time.Time,
) (*CopilotForwardResult, error) {
	defer func() { _ = resp.Body.Close() }()

	// Accumulated state from all chunks.
	var (
		respID         string
		respModel      string
		contentBuilder strings.Builder
		finishReason   string
		toolCalls      = make(map[int]*openAIToolCall) // index → accumulated tool call
		usage          *openAIUsage
		firstTokenMs   *int
	)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	doneSeen := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]
		if data == "[DONE]" {
			doneSeen = true
			break
		}

		var chunk openAIChatStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if firstTokenMs == nil {
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}

		if respID == "" {
			respID = chunk.ID
		}
		if respModel == "" {
			respModel = chunk.Model
		}

		if chunk.Usage != nil {
			usage = chunk.Usage
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				_, _ = contentBuilder.WriteString(choice.Delta.Content)
			}
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
			// Accumulate tool calls by index.
			for _, tcd := range choice.Delta.ToolCalls {
				tc, ok := toolCalls[tcd.Index]
				if !ok {
					tc = &openAIToolCall{
						ID:   tcd.ID,
						Type: tcd.Type,
						Function: openAIFunctionCall{
							Name:      tcd.Function.Name,
							Arguments: tcd.Function.Arguments,
						},
					}
					toolCalls[tcd.Index] = tc
				} else {
					if tcd.ID != "" {
						tc.ID = tcd.ID
					}
					if tcd.Function.Name != "" {
						tc.Function.Name = tcd.Function.Name
					}
					tc.Function.Arguments += tcd.Function.Arguments
				}
			}
		}
	}

	// Only treat scanner errors as failures when we never saw [DONE].
	// After a clean [DONE] the server closes the connection, which causes the
	// scanner's underlying reader to return io.EOF — that manifests as
	// scanner.Err() == "unexpected EOF" even though the stream was fully
	// consumed.  We must not treat that as an error.
	if !doneSeen {
		scanErr := scanner.Err()

		// If the client's request context was cancelled (client disconnected or
		// its own timeout fired), there is no point writing a response — the
		// client is gone.  Return immediately without touching c.Writer.
		if c.Request.Context().Err() != nil || scanErr == context.Canceled {
			slog.Debug("copilot messages stream-to-nonstream: client disconnected, aborting",
				"scan_err", scanErr,
				"ctx_err", c.Request.Context().Err())
			return &CopilotForwardResult{
				StatusCode:    499, // Client Closed Request (nginx convention)
				Model:         model,
				UpstreamModel: upstreamModel,
				Duration:      time.Since(startTime),
				FirstTokenMs:  firstTokenMs,
			}, nil
		}

		if scanErr != nil {
			slog.Warn("copilot messages stream-to-nonstream scanner error", "error", scanErr)
		}

		// If we already accumulated content or tool calls before the stream was
		// cut (e.g. Copilot's 60-second SSE hard limit), salvage what we have
		// and return a best-effort response rather than a 529.  This avoids
		// an infinite retry loop when the model simply takes longer than 60s to
		// generate — the client gets a (possibly truncated) but usable response.
		hasContent := contentBuilder.Len() > 0 || len(toolCalls) > 0
		if hasContent {
			slog.Warn("copilot messages stream-to-nonstream: salvaging partial response after stream cut",
				"content_len", contentBuilder.Len(),
				"tool_calls", len(toolCalls),
				"scan_err", scanErr)
			// Fall through to the assembly block below — doneSeen stays false but
			// we treat the partial data as complete.  Set a synthetic finish_reason
			// so downstream translation produces a well-formed Anthropic response.
			if finishReason == "" {
				finishReason = "end_turn"
			}
		} else {
			// Nothing accumulated at all — we cannot salvage anything useful.
			// Return an Anthropic-format 529 error so Claude Code retries.
			c.JSON(529, gin.H{
				"type": "error",
				"error": gin.H{
					"type":    "overloaded_error",
					"message": "Upstream connection interrupted, please retry.",
				},
			})
			return &CopilotForwardResult{
				StatusCode:    529,
				Model:         model,
				UpstreamModel: upstreamModel,
				Duration:      time.Since(startTime),
				FirstTokenMs:  firstTokenMs,
			}, nil
		}
	}

	// Build the reconstructed OpenAI non-streaming response.
	msg := openAIMessage{
		Role:    "assistant",
		Content: contentBuilder.String(),
	}
	// Attach accumulated tool calls in index order.
	if len(toolCalls) > 0 {
		sorted := make([]openAIToolCall, 0, len(toolCalls))
		for i := 0; i < len(toolCalls); i++ {
			if tc, ok := toolCalls[i]; ok {
				sorted = append(sorted, *tc)
			}
		}
		msg.ToolCalls = sorted
		// If there are tool calls but no text, set content to nil (null)
		// to match the standard OpenAI response format.
		if contentBuilder.Len() == 0 {
			msg.Content = nil
		}
	}

	assembled := openAIChatResponse{
		ID:    respID,
		Model: respModel,
		Choices: []openAIChoice{
			{
				Index:        0,
				Message:      msg,
				FinishReason: finishReason,
			},
		},
		Usage: usage,
	}

	assembledBody, err := json.Marshal(assembled)
	if err != nil {
		return nil, fmt.Errorf("copilot messages: marshal assembled response: %w", err)
	}

	slog.Debug("copilot messages stream-to-nonstream assembled",
		"model", model,
		"content_len", contentBuilder.Len(),
		"tool_calls", len(toolCalls),
		"finish_reason", finishReason)

	// Parse usage for result tracking.
	copilotUsage := &CopilotUsage{}
	if usage != nil {
		copilotUsage.PromptTokens = usage.PromptTokens
		copilotUsage.CompletionTokens = usage.CompletionTokens
		copilotUsage.TotalTokens = usage.TotalTokens
	}

	// Translate assembled OpenAI response → Anthropic format.
	anthropicBody, err := translateOpenAIToAnthropic(assembledBody)
	if err != nil {
		slog.Warn("copilot messages: failed to translate assembled response, forwarding raw",
			"error", err, "model", model)
		c.Data(http.StatusOK, "application/json", assembledBody)
		return &CopilotForwardResult{
			StatusCode:    http.StatusOK,
			Model:         model,
			UpstreamModel: upstreamModel,
			Usage:         copilotUsage,
			Duration:      time.Since(startTime),
			FirstTokenMs:  firstTokenMs,
		}, nil
	}

	c.Data(http.StatusOK, "application/json", anthropicBody)
	return &CopilotForwardResult{
		StatusCode:    http.StatusOK,
		Model:         model,
		UpstreamModel: upstreamModel,
		Usage:         copilotUsage,
		Duration:      time.Since(startTime),
		FirstTokenMs:  firstTokenMs,
	}, nil
}

// detectAnthropicStream checks if an Anthropic request body has "stream": true.
func detectAnthropicStream(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	return req.Stream
}

// forceStreamTrue rewrites the JSON body so that "stream" is always true.
// This is used to force streaming mode for upstream Copilot requests, even
// when the client originally sent stream=false, because non-streaming requests
// to Copilot often hang until timeout on Claude models.
func forceStreamTrue(body []byte) []byte {
	result := gjson.GetBytes(body, "stream")
	if result.Exists() && result.Bool() {
		return body // already true
	}
	// Use sjson to set stream=true; fall back to original body on error.
	out, err := sjson.SetBytes(body, "stream", true)
	if err != nil {
		return body
	}
	return out
}

// ensureStreamIncludeUsage injects "stream_options":{"include_usage":true} when stream=true.
// This causes the Copilot API to append a usage-summary SSE chunk at the end of the stream,
// enabling accurate token count recording. No-op when stream is absent or false.
func ensureStreamIncludeUsage(body []byte) []byte {
	if !gjson.GetBytes(body, "stream").Bool() {
		return body
	}
	out, err := sjson.SetBytes(body, "stream_options.include_usage", true)
	if err != nil {
		slog.Warn("copilot: failed to inject stream_options.include_usage", "error", err)
		return body
	}
	return out
}

// isUsageOnlyChunk reports whether a decoded SSE data string is the trailing
// usage-summary chunk that Copilot appends when stream_options.include_usage=true.
// These chunks have a "usage" field but no non-empty "choices" array.
// Used to filter the chunk from the downstream stream when the client did not request it.
func isUsageOnlyChunk(data string) bool {
	if !gjson.Get(data, "usage").Exists() {
		return false
	}
	choices := gjson.Get(data, "choices")
	return !choices.Exists() || (choices.IsArray() && len(choices.Array()) == 0)
}

// extractEventType reads the "type" field from a JSON event object for the SSE
// event name (e.g. "message_start", "content_block_delta", …).
func extractEventType(jsonStr string) string {
	var e struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &e); err == nil && e.Type != "" {
		return e.Type
	}
	return "message"
}

// copilotInternalUserResponse is the raw response from the GitHub
// copilot_internal/user endpoint. Only the fields we need are decoded.
type copilotInternalUserResponse struct {
	// CopilotPlan is the plan type string returned by GitHub.
	CopilotPlan string `json:"copilot_plan"`

	// ChatEnabled indicates whether chat is available.
	ChatEnabled bool `json:"chat_enabled"`

	// Login is the GitHub username.
	Login string `json:"login"`

	// QuotaResetDate is the date the quota resets (top-level field).
	QuotaResetDate string `json:"quota_reset_date,omitempty"`

	// QuotaSnapshots is the rich quota data returned by GitHub (newer format).
	QuotaSnapshots *struct {
		Completions         *copilotQuotaSnapshotRaw `json:"completions"`
		Chat                *copilotQuotaSnapshotRaw `json:"chat"`
		PremiumInteractions *copilotQuotaSnapshotRaw `json:"premium_interactions"`
	} `json:"quota_snapshots"`
}

// copilotQuotaSnapshotRaw mirrors the quota_snapshots entries from GitHub API.
type copilotQuotaSnapshotRaw struct {
	Entitlement      int     `json:"entitlement"`
	OverageCount     int     `json:"overage_count"`
	OveragePermitted bool    `json:"overage_permitted"`
	PercentRemaining float64 `json:"percent_remaining"`
	Remaining        int     `json:"remaining"`
	Unlimited        bool    `json:"unlimited"`
}

// toQuotaDetail converts a raw snapshot into the canonical QuotaDetail.
func (r *copilotQuotaSnapshotRaw) toQuotaDetail() *copilot.QuotaDetail {
	if r == nil {
		return nil
	}
	return &copilot.QuotaDetail{
		Entitlement:      r.Entitlement,
		OveragePermitted: r.OveragePermitted,
		Used:             r.Entitlement - r.Remaining + r.OverageCount,
		Unlimited:        r.Unlimited,
		Remaining:        r.Remaining,
		OverageCount:     r.OverageCount,
		PercentRemaining: r.PercentRemaining,
	}
}

// FetchQuota fetches the Copilot quota and plan information for an account from
// the GitHub copilot_internal/user API endpoint.
func (s *CopilotGatewayService) FetchQuota(
	ctx context.Context,
	account *Account,
) (*copilot.CopilotQuotaInfo, error) {
	githubToken := account.GetCredential("github_token")
	if githubToken == "" {
		return nil, fmt.Errorf("copilot: no github_token configured for account %d", account.ID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/copilot_internal/user", nil)
	if err != nil {
		return nil, fmt.Errorf("copilot: build quota request: %w", err)
	}
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-GitHub-Api-Version", copilot.DefaultGitHubAPIVersion)

	resp, err := s.httpClient.Do(req) //nolint:gosec // URL is from trusted Copilot API config
	if err != nil {
		return nil, fmt.Errorf("copilot: quota request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("copilot: read quota response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Map upstream 401/403 to 422 (account credential issue, not a server error).
		// Other upstream errors become 502 (bad gateway).
		outCode := http.StatusBadGateway
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			outCode = http.StatusUnprocessableEntity
		}
		return nil, infraerrors.Newf(outCode, "COPILOT_UPSTREAM_ERROR",
			"copilot: quota HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw copilotInternalUserResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("copilot: parse quota response: %w", err)
	}

	// Map plan string to human-readable plan type.
	planType := planTypeFromString(raw.CopilotPlan)

	info := &copilot.CopilotQuotaInfo{
		Plan:           raw.CopilotPlan,
		PlanType:       planType,
		QuotaResetDate: raw.QuotaResetDate,
	}

	if s := raw.QuotaSnapshots; s != nil {
		info.Completions = s.Completions.toQuotaDetail()
		info.Chat = s.Chat.toQuotaDetail()
		info.PremiumInteractions = s.PremiumInteractions.toQuotaDetail()
	}

	return info, nil
}

// FetchAllCopilotQuotas fetches quota info for all active Copilot accounts
// concurrently. Results are returned in arbitrary order. Failures are
// recorded in the Error field of the corresponding summary entry.
func (s *CopilotGatewayService) FetchAllCopilotQuotas(
	ctx context.Context,
	adminSvc AdminService,
) ([]copilot.CopilotAccountQuotaSummary, error) {
	// Page through all Copilot accounts (large page size to minimise round-trips).
	const pageSize = 200
	var allAccounts []Account
	for page := 1; ; page++ {
		accounts, _, err := adminSvc.ListAccounts(ctx, page, pageSize, PlatformCopilot, "", StatusActive, "", 0)
		if err != nil {
			return nil, fmt.Errorf("copilot: list accounts: %w", err)
		}
		allAccounts = append(allAccounts, accounts...)
		if len(accounts) < pageSize {
			break
		}
	}

	results := make([]copilot.CopilotAccountQuotaSummary, len(allAccounts))
	var wg sync.WaitGroup
	for i, acc := range allAccounts {
		wg.Add(1)
		go func(idx int, a Account) {
			defer wg.Done()
			summary := copilot.CopilotAccountQuotaSummary{
				AccountID:   a.ID,
				AccountName: a.Name,
				Status:      string(a.Status),
				GitHubLogin: a.GetCredential("github_login"),
			}
			qi, err := s.FetchQuota(ctx, &a)
			if err != nil {
				summary.Error = err.Error()
			} else {
				summary.QuotaInfo = qi
			}
			results[idx] = summary
		}(i, acc)
	}
	wg.Wait()
	return results, nil
}

const (
	defaultCopilotMaxOutputTokens          = 8192
	copilotMaxOutputTokensCredentialKey    = "copilot_max_output_tokens"
	copilotMaxOutputTokensSanityUpperBound = 262144
)

// copilotModelUsesMaxOutputClamp is true for Copilot wire models where we may need to
// cap max_tokens (Sonnet/Opus). Haiku is left untouched.
func copilotModelUsesMaxOutputClamp(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return false
	}
	if strings.Contains(m, "haiku") {
		return false
	}
	return strings.Contains(m, "sonnet") || strings.Contains(m, "opus")
}

// effectiveCopilotMaxOutputTokensCap returns the ceiling for max_tokens and whether
// clamping applies. Three-layer priority:
//  1. 账号级 credentials.copilot_max_output_tokens（最高优先级）
//  2. 平台配置 CopilotPlatformConfig.max_output_tokens（按 plan_type 匹配）
//  3. 系统默认 defaultCopilotMaxOutputTokens（兜底）
//
// Special cases:
//   - absent at all layers → defaultCopilotMaxOutputTokens (8192), clamp=true
//   - account-level <= 0 (explicit 0) → do not clamp (may trigger upstream 400 again)
//   - > 0 → use that value (capped at copilotMaxOutputTokensSanityUpperBound)
func (s *CopilotGatewayService) effectiveCopilotMaxOutputTokensCap(ctx context.Context, account *Account) (cap int, clamp bool) {
	if account == nil || account.Credentials == nil {
		return s.platformOrSystemMaxOutputTokens(ctx, account)
	}
	raw, ok := account.Credentials[copilotMaxOutputTokensCredentialKey]
	if !ok || raw == nil {
		// 层 2：查平台配置
		return s.platformOrSystemMaxOutputTokens(ctx, account)
	}
	v := account.GetCredentialAsInt64(copilotMaxOutputTokensCredentialKey)
	if v <= 0 {
		return 0, false
	}
	if v > copilotMaxOutputTokensSanityUpperBound {
		v = copilotMaxOutputTokensSanityUpperBound
	}
	return int(v), true
}

// platformOrSystemMaxOutputTokens 先查平台配置，再回退到系统默认。
func (s *CopilotGatewayService) platformOrSystemMaxOutputTokens(ctx context.Context, account *Account) (int, bool) {
	if s.platformConfigSvc != nil && account != nil {
		planType := account.GetCredential("plan_type")
		if planType != "" {
			cfg, err := s.platformConfigSvc.GetByPlanType(ctx, planType)
			if err == nil && cfg != nil && cfg.MaxOutputTokens != nil {
				v := *cfg.MaxOutputTokens
				if v <= 0 {
					return 0, false
				}
				if v > copilotMaxOutputTokensSanityUpperBound {
					v = copilotMaxOutputTokensSanityUpperBound
				}
				return int(v), true
			}
		}
	}
	return defaultCopilotMaxOutputTokens, true
}

// clampCopilotUpstreamMaxTokens lowers max_tokens when the model is Sonnet/Opus and
// the account requests clamping (default 8192 unless overridden in credentials or platform config).
func (s *CopilotGatewayService) clampCopilotUpstreamMaxTokens(ctx context.Context, body []byte, account *Account) []byte {
	if len(body) == 0 {
		return body
	}
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if !copilotModelUsesMaxOutputClamp(model) {
		return body
	}
	mt := gjson.GetBytes(body, "max_tokens")
	if !mt.Exists() {
		return body
	}
	max := int(mt.Int())
	if max <= 0 {
		return body
	}
	capTok, doClamp := s.effectiveCopilotMaxOutputTokensCap(ctx, account)
	if !doClamp || max <= capTok {
		return body
	}
	out, err := sjson.SetBytes(body, "max_tokens", capTok)
	if err != nil {
		return body
	}
	slog.Debug("copilot: clamped max_tokens for upstream compatibility",
		"model", model, "requested_max_tokens", max, "clamped_to", capTok)
	return out
}

// mergeConsecutiveSameRoleMessagesInOpenAIBody parses an OpenAI Chat Completions
// request body, merges adjacent messages that share the same "user" or "system"
// role, and returns the re-serialised body.
//
// Claude Code in OpenAI-mode (via cc-switch) sends consecutive user messages
// (e.g. <available-deferred-tools> injection + actual user message).  The
// Copilot API rejects adjacent same-role messages with 400 Bad Request.
// This function normalises the body before forwarding, matching the behaviour
// already applied on the Anthropic path via mergeConsecutiveSameRoleMessages.
//
// If parsing fails the original body is returned unchanged so the upstream
// error (if any) is forwarded to the caller.
func mergeConsecutiveSameRoleMessagesInOpenAIBody(body []byte) []byte {
	var req struct {
		Messages []struct {
			Role       string          `json:"role"`
			Content    json.RawMessage `json:"content"`
			ToolCallID string          `json:"tool_call_id,omitempty"`
			ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
		} `json:"messages"`
	}

	// We want to preserve all other fields (model, stream, temperature …),
	// so unmarshal into a generic map and only rewrite the messages field.
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(body, &generic); err != nil {
		return body
	}

	msgsRaw, ok := generic["messages"]
	if !ok {
		return body
	}
	if err := json.Unmarshal(msgsRaw, &req.Messages); err != nil {
		return body
	}
	if len(req.Messages) == 0 {
		return body
	}

	// Build merged messages list.
	type msg struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content"`
		ToolCallID string          `json:"tool_call_id,omitempty"`
		ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
	}

	merged := make([]msg, 0, len(req.Messages))
	for _, m := range req.Messages {
		if len(merged) == 0 {
			merged = append(merged, msg{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID, ToolCalls: m.ToolCalls})
			continue
		}
		prev := &merged[len(merged)-1]

		// Only merge user/system; never merge assistant/tool messages, and
		// never merge a message that has tool_calls.
		canMerge := m.Role == prev.Role &&
			(m.Role == "user" || m.Role == "system") &&
			len(m.ToolCalls) == 0 &&
			len(prev.ToolCalls) == 0

		if !canMerge {
			merged = append(merged, msg{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID, ToolCalls: m.ToolCalls})
			continue
		}

		// Extract text from both sides and join.
		prevText := extractTextFromOpenAIContent(prev.Content)
		curText := extractTextFromOpenAIContent(m.Content)
		combined := prevText
		if curText != "" {
			if combined != "" {
				combined += "\n\n"
			}
			combined += curText
		}
		combinedJSON, err := json.Marshal(combined)
		if err != nil {
			merged = append(merged, msg{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID, ToolCalls: m.ToolCalls})
			continue
		}
		prev.Content = combinedJSON
	}

	mergedJSON, err := json.Marshal(merged)
	if err != nil {
		return body
	}
	generic["messages"] = mergedJSON

	out, err := json.Marshal(generic)
	if err != nil {
		return body
	}
	return out
}

// extractTextFromOpenAIContent extracts a plain string from an OpenAI message
// content field, which may be a JSON string or a JSON array of content parts.
func extractTextFromOpenAIContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try array of content parts: [{"type":"text","text":"..."}].
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var texts []string
		for _, p := range parts {
			if p.Type == "text" && p.Text != "" {
				texts = append(texts, p.Text)
			}
		}
		return strings.Join(texts, "\n\n")
	}
	return ""
}

// planTypeFromString returns a human-readable plan type label.
func planTypeFromString(plan string) string {
	switch plan {
	case "copilot_for_individuals", "copilot_individual":
		return "Individual"
	case "copilot_business":
		return "Business"
	case "copilot_enterprise":
		return "Enterprise"
	default:
		if plan != "" {
			return plan
		}
		return "Unknown"
	}
}

// StripUnsupportedContentPartsFromOpenAIBody removes content parts with
// unsupported types (e.g. "file") from user messages before forwarding to
// Copilot, which only accepts "text" and "image_url" part types.
//
// Returns the (possibly rewritten) body and a boolean indicating whether any
// file parts were found and stripped.  On parse failure the original body is
// returned with hasFile=false, preserving the existing upstream error path.
func StripUnsupportedContentPartsFromOpenAIBody(body []byte) ([]byte, bool) {
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(body, &generic); err != nil {
		return body, false
	}

	msgsRaw, ok := generic["messages"]
	if !ok {
		return body, false
	}

	var msgs []json.RawMessage
	if err := json.Unmarshal(msgsRaw, &msgs); err != nil {
		return body, false
	}

	hasFile := false
	rewritten := make([]json.RawMessage, 0, len(msgs))

	for _, rawMsg := range msgs {
		var m struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(rawMsg, &m); err != nil {
			rewritten = append(rewritten, rawMsg)
			continue
		}

		// Only filter array-form content; plain string content is fine as-is.
		var parts []json.RawMessage
		if err := json.Unmarshal(m.Content, &parts); err != nil {
			// Not an array — plain string content, keep unchanged.
			rewritten = append(rewritten, rawMsg)
			continue
		}

		filtered := make([]json.RawMessage, 0, len(parts))
		for _, p := range parts {
			var typed struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(p, &typed); err != nil {
				filtered = append(filtered, p)
				continue
			}
			if typed.Type == "file" {
				hasFile = true
				continue // drop this part
			}
			filtered = append(filtered, p)
		}

		if len(filtered) == len(parts) {
			// Nothing was stripped; keep the original raw message.
			rewritten = append(rewritten, rawMsg)
			continue
		}

		// Re-encode the message with the filtered content.
		newContent, err := json.Marshal(filtered)
		if err != nil {
			rewritten = append(rewritten, rawMsg)
			continue
		}

		// Preserve all other message fields by round-tripping through a generic map.
		var msgMap map[string]json.RawMessage
		if err := json.Unmarshal(rawMsg, &msgMap); err != nil {
			rewritten = append(rewritten, rawMsg)
			continue
		}
		msgMap["content"] = newContent
		newMsg, err := json.Marshal(msgMap)
		if err != nil {
			rewritten = append(rewritten, rawMsg)
			continue
		}
		rewritten = append(rewritten, newMsg)
	}

	if !hasFile {
		return body, false
	}

	newMsgs, err := json.Marshal(rewritten)
	if err != nil {
		return body, false
	}
	generic["messages"] = newMsgs

	out, err := json.Marshal(generic)
	if err != nil {
		return body, false
	}
	return out, true
}

// convertFilePartsToText rewrites OpenAI chat content parts of type="file" into
// type="text" parts so /chat/completions can still receive attachment context
// when the target model cannot be served via /responses.
//
// For text-like MIME types (text/*, application/json, etc.) the base64 payload
// is decoded and inlined directly (up to copilotChatFileTextPreviewMaxBytes).
// For binary types (PDF, images, etc.) a descriptive placeholder is inserted so
// the model at least knows a file was attached.
func convertFilePartsToText(body []byte) []byte {
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(body, &generic); err != nil {
		return body
	}

	msgsRaw, ok := generic["messages"]
	if !ok {
		return body
	}

	var msgs []json.RawMessage
	if err := json.Unmarshal(msgsRaw, &msgs); err != nil {
		return body
	}

	changed := false
	rewritten := make([]json.RawMessage, 0, len(msgs))

	for _, rawMsg := range msgs {
		var m struct {
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(rawMsg, &m); err != nil {
			rewritten = append(rewritten, rawMsg)
			continue
		}

		// Only array-form content can contain typed file parts.
		var parts []json.RawMessage
		if err := json.Unmarshal(m.Content, &parts); err != nil {
			rewritten = append(rewritten, rawMsg)
			continue
		}

		msgChanged := false
		converted := make([]json.RawMessage, 0, len(parts))
		for _, rawPart := range parts {
			var part copilotFilePart
			if err := json.Unmarshal(rawPart, &part); err != nil || part.Type != "file" || part.File == nil {
				converted = append(converted, rawPart)
				continue
			}

			textPart, err := json.Marshal(map[string]any{
				"type": "text",
				"text": describeChatFileForTextFallback(part.File),
			})
			if err != nil {
				converted = append(converted, rawPart)
				continue
			}
			converted = append(converted, textPart)
			msgChanged = true
			changed = true
		}

		if !msgChanged {
			rewritten = append(rewritten, rawMsg)
			continue
		}

		newContent, err := json.Marshal(converted)
		if err != nil {
			rewritten = append(rewritten, rawMsg)
			continue
		}

		var msgMap map[string]json.RawMessage
		if err := json.Unmarshal(rawMsg, &msgMap); err != nil {
			rewritten = append(rewritten, rawMsg)
			continue
		}
		msgMap["content"] = newContent
		newMsg, err := json.Marshal(msgMap)
		if err != nil {
			rewritten = append(rewritten, rawMsg)
			continue
		}
		rewritten = append(rewritten, newMsg)
	}

	if !changed {
		return body
	}

	newMsgs, err := json.Marshal(rewritten)
	if err != nil {
		return body
	}
	generic["messages"] = newMsgs
	out, err := json.Marshal(generic)
	if err != nil {
		return body
	}
	return out
}

// copilotFilePart is a minimal struct for parsing "file" typed content parts.
type copilotFilePart struct {
	Type string           `json:"type"`
	File *copilotFileData `json:"file,omitempty"`
}

// copilotFileData mirrors the ChatFile structure used for inline file attachments.
type copilotFileData struct {
	FileData string `json:"file_data,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// describeChatFileForTextFallback converts a file attachment into a plain-text
// representation suitable for /chat/completions:
//   - Text-like files (text/*, JSON, XML, …): base64-decoded content is inlined.
//   - Binary files (PDF, images, …): a descriptive placeholder is emitted so the
//     model knows a file was attached.
func describeChatFileForTextFallback(file *copilotFileData) string {
	filename := strings.TrimSpace(file.Filename)
	if filename == "" {
		filename = "unnamed file"
	}

	// file_id reference only (no inline data).
	if fileID := strings.TrimSpace(file.FileID); fileID != "" && strings.TrimSpace(file.FileData) == "" {
		return fmt.Sprintf("[Attached file: %s (file_id=%s), content unavailable - model cannot process files via this endpoint]", filename, fileID)
	}

	mimeType, encoded, ok := parseInlineBase64FileData(strings.TrimSpace(file.FileData))
	if !ok {
		if strings.TrimSpace(file.FileData) != "" {
			return fmt.Sprintf("[Attached file: %s, inline file payload omitted - model cannot process files via this endpoint]", filename)
		}
		return fmt.Sprintf("[Attached file: %s, content unavailable - model cannot process files via this endpoint]", filename)
	}

	// Text-like: try to inline the decoded content.
	if isTextLikeMIMEType(mimeType) {
		if text, ok := decodeInlineTextFile(encoded); ok {
			return fmt.Sprintf("[Attached file: %s (%s)]\n%s", filename, mimeType, text)
		}
	}

	// Binary (PDF, images, etc.): descriptive placeholder.
	return fmt.Sprintf("[Attached file: %s (%s), base64 content omitted - model cannot process binary files via this endpoint]", filename, mimeType)
}

// parseInlineBase64FileData parses a data URI of the form
// "data:<mime>;base64,<encoded>" and returns the MIME type and encoded payload.
func parseInlineBase64FileData(fileData string) (mimeType string, encoded string, ok bool) {
	if !strings.HasPrefix(fileData, "data:") {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(fileData, "data:"), ",", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	meta := strings.TrimSpace(parts[0])
	if !strings.HasSuffix(meta, ";base64") {
		return "", "", false
	}
	mimeType = strings.TrimSpace(strings.TrimSuffix(meta, ";base64"))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return mimeType, parts[1], true
}

// isTextLikeMIMEType reports whether mimeType represents human-readable text.
func isTextLikeMIMEType(mimeType string) bool {
	m := strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	if strings.HasPrefix(m, "text/") {
		return true
	}
	switch m {
	case "application/json",
		"application/xml",
		"application/javascript",
		"application/x-javascript",
		"application/x-ndjson",
		"application/yaml",
		"application/x-yaml",
		"application/toml",
		"application/x-toml":
		return true
	}
	return false
}

// decodeInlineTextFile base64-decodes an inline file payload and returns the
// content as a UTF-8 string (truncated to copilotChatFileTextPreviewMaxBytes).
// Returns ("", false) when the payload is binary or decoding fails.
func decodeInlineTextFile(encoded string) (string, bool) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			return "", false
		}
	}
	if len(decoded) == 0 || !utf8.Valid(decoded) {
		return "", false
	}

	truncated := false
	if len(decoded) > copilotChatFileTextPreviewMaxBytes {
		decoded = decoded[:copilotChatFileTextPreviewMaxBytes]
		// Trim to the last valid UTF-8 boundary.
		for len(decoded) > 0 && !utf8.Valid(decoded) {
			decoded = decoded[:len(decoded)-1]
		}
		truncated = true
	}

	text := strings.TrimSpace(string(decoded))
	if text == "" {
		return "", false
	}
	if truncated {
		text += "\n\n[truncated]"
	}
	return text, true
}
