package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ForwardDeepSeek forwards an Anthropic /v1/messages request to a DeepSeek
// account's OpenAI Chat Completions endpoint with full bidirectional
// translation, mirroring the seamless behaviour provided for OpenAI
// (ForwardAsAnthropic) so that a conversation can switch between Claude,
// OpenAI and DeepSeek accounts without client-visible schema differences.
//
// Pipeline:
//
//	client request  : Anthropic /v1/messages body
//	    ↓ apicompat.AnthropicToResponses
//	Responses request
//	    ↓ apicompat.ResponsesRequestToChatCompletions
//	ChatCompletions request (model rewritten with DeepSeek mapping)
//	    ↓ HTTP POST {base}/v1/chat/completions, Bearer auth, stream=true
//	upstream SSE (ChatCompletions chunks)
//	    ↓ apicompat.ChatChunkToResponsesEvents
//	Responses SSE events
//	    ↓ apicompat.ResponsesEventToAnthropicEvents
//	Anthropic SSE events → client (or buffered into a JSON response when the
//	client requested stream=false).
func (s *GatewayService) ForwardDeepSeek(ctx context.Context, c *gin.Context, account *Account, parsed *ParsedRequest) (*ForwardResult, error) {
	if parsed == nil {
		return nil, fmt.Errorf("forward deepseek: empty request")
	}
	startTime := time.Now()

	body := StripEmptyTextBlocks(parsed.Body)

	resp, originalModel, billingModel, err := s.dispatchDeepSeekChatRequest(ctx, c, account, body, parsed)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	// Failover / error handling.
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))

		upstreamMsg := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))

		// Thinking-context retry: DeepSeek thinking-mode upstream rejects the
		// request when prior assistant turns lack reasoning_content / when the
		// echoed thinking blocks fail validation. Strip thinking from history
		// (FilterThinkingBlocksForRetry preserves the text by downgrading it to
		// text blocks) and try once more on the same account.
		if resp.StatusCode == http.StatusBadRequest && isDeepSeekThinkingContextError(respBody) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  resp.Header.Get("x-request-id"),
				Kind:               "deepseek_thinking_error",
				Message:            upstreamMsg,
				Detail:             truncateString(string(respBody), 2048),
			})
			filteredBody := FilterThinkingBlocksForRetry(body)
			if !bytes.Equal(filteredBody, body) {
				logger.LegacyPrintf("service.deepseek_gateway", "DeepSeek account %d: thinking-context 400, retrying with stripped thinking blocks", account.ID)
				retryResp, _, _, retryErr := s.dispatchDeepSeekChatRequest(ctx, c, account, filteredBody, parsed)
				if retryErr == nil && retryResp != nil {
					if retryResp.StatusCode < 400 {
						resp = retryResp
					} else {
						retryBody, _ := io.ReadAll(io.LimitReader(retryResp.Body, 2<<20))
						_ = retryResp.Body.Close()
						appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
							Platform:           account.Platform,
							AccountID:          account.ID,
							AccountName:        account.Name,
							UpstreamStatusCode: retryResp.StatusCode,
							UpstreamRequestID:  retryResp.Header.Get("x-request-id"),
							Kind:               "deepseek_thinking_retry",
							Message:            sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(retryBody)),
							Detail:             truncateString(string(retryBody), 2048),
						})
						resp = &http.Response{
							StatusCode: retryResp.StatusCode,
							Header:     retryResp.Header.Clone(),
							Body:       io.NopCloser(bytes.NewReader(retryBody)),
						}
					}
				}
			}
		}

		// Re-evaluate after potential retry.
		if resp.StatusCode >= 400 {
			respBody, _ = io.ReadAll(io.LimitReader(resp.Body, 2<<20))
			_ = resp.Body.Close()
			resp.Body = io.NopCloser(bytes.NewReader(respBody))
			upstreamMsg = sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))

			if s.shouldFailoverUpstreamError(resp.StatusCode) {
				appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
					Platform:           account.Platform,
					AccountID:          account.ID,
					AccountName:        account.Name,
					UpstreamStatusCode: resp.StatusCode,
					UpstreamRequestID:  resp.Header.Get("x-request-id"),
					Kind:               "failover",
					Message:            upstreamMsg,
					Detail:             truncateString(string(respBody), 2048),
				})
				if s.rateLimitService != nil {
					s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
				}
				return nil, &UpstreamFailoverError{
					StatusCode:             resp.StatusCode,
					ResponseBody:           respBody,
					RetryableOnSameAccount: account.IsPoolMode() && isPoolModeRetryableStatus(resp.StatusCode),
				}
			}
			// Non-failover: return Anthropic-formatted error straight to client.
			writeAnthropicError(c, resp.StatusCode, "upstream_error", upstreamMsg)
			return nil, fmt.Errorf("deepseek upstream error: status=%d msg=%s", resp.StatusCode, upstreamMsg)
		}
	}

	// Stream vs buffered path.
	var (
		usage            apicompat.AnthropicUsage
		firstTokenMs     *int
		clientDisconnect bool
	)
	if parsed.Stream {
		usage, firstTokenMs, clientDisconnect, err = s.handleDeepSeekStreamingResponse(resp, c, originalModel, startTime)
	} else {
		usage, err = s.handleDeepSeekBufferedResponse(resp, c, originalModel)
	}
	if err != nil {
		return nil, err
	}

	// UpstreamModel is for persistence/billing — use the stripped form so cost
	// lookups do not need to know about the "[thinking]" suffix convention.
	return &ForwardResult{
		RequestID:        resp.Header.Get("x-request-id"),
		Usage:            anthropicUsageToClaudeUsage(usage),
		Model:            originalModel,
		UpstreamModel:    billingModel,
		Stream:           parsed.Stream,
		Duration:         time.Since(startTime),
		FirstTokenMs:     firstTokenMs,
		ClientDisconnect: clientDisconnect,
	}, nil
}

// dispatchDeepSeekChatRequest performs the Anthropic→Chat conversion and HTTP
// dispatch for ForwardDeepSeek. It returns the upstream response (which may
// carry a non-2xx status) plus the original model id and billing model id.
//
// The function does NOT consume the response body — callers are responsible
// for reading and closing it.
func (s *GatewayService) dispatchDeepSeekChatRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	anthropicBody []byte,
	parsed *ParsedRequest,
) (*http.Response, string, string, error) {
	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(anthropicBody, &anthropicReq); err != nil {
		return nil, "", "", fmt.Errorf("parse anthropic request: %w", err)
	}
	originalModel := anthropicReq.Model

	mappedModel, _ := account.ResolveMappedModel(originalModel)
	upstreamModel := mappedModel
	billingModel := StripDeepSeekThinkingSuffix(mappedModel)
	thinkingEnabled := IsDeepSeekThinkingModel(mappedModel)
	anthropicReq.Model = upstreamModel

	responsesReq, err := apicompat.AnthropicToResponses(&anthropicReq)
	if err != nil {
		return nil, originalModel, billingModel, fmt.Errorf("convert anthropic to responses: %w", err)
	}
	responsesReq.Model = upstreamModel
	responsesReq.Include = nil

	chatReq, err := apicompat.ResponsesRequestToChatCompletions(responsesReq)
	if err != nil {
		return nil, originalModel, billingModel, fmt.Errorf("convert responses to chat completions: %w", err)
	}
	chatReq.Model = upstreamModel
	chatReq.Stream = true
	chatReq.StreamOptions = &apicompat.ChatStreamOptions{IncludeUsage: true}
	applyDeepSeekThinkingMode(chatReq, thinkingEnabled)

	upstreamBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, originalModel, billingModel, fmt.Errorf("marshal chat completions request: %w", err)
	}

	setOpsUpstreamRequestBody(c, upstreamBody)

	token, tokenType, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, originalModel, billingModel, err
	}
	if tokenType != "apikey" {
		return nil, originalModel, billingModel, fmt.Errorf("deepseek forward requires apikey token, got: %s", tokenType)
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	targetURL, err := buildDeepSeekChatCompletionsURL(s, account)
	if err != nil {
		return nil, originalModel, billingModel, err
	}

	logger.L().Debug("deepseek forward: dispatching",
		zap.Int64("account_id", account.ID),
		zap.String("account_name", account.Name),
		zap.String("original_model", originalModel),
		zap.String("mapped_model", mappedModel),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("thinking", thinkingEnabled),
		zap.Bool("client_stream", parsed.Stream),
	)

	upstreamCtx, releaseUpstreamCtx := detachStreamUpstreamContext(ctx, parsed.Stream)
	// upstreamCtx is intentionally not cancelled here: the response body is
	// streamed to the client after this function returns. ForwardDeepSeek
	// calls release indirectly when the stream completes via context expiry.
	_ = releaseUpstreamCtx

	req, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(upstreamBody))
	if err != nil {
		return nil, originalModel, billingModel, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			UpstreamURL:        safeUpstreamURL(targetURL),
			Kind:               "request_error",
			Message:            safeErr,
		})
		writeAnthropicError(c, http.StatusBadGateway, "api_error", "Upstream request failed")
		return nil, originalModel, billingModel, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	return resp, originalModel, billingModel, nil
}

// isDeepSeekThinkingContextError detects upstream 400 errors that indicate the
// previous turn's reasoning/thinking content failed validation. Observed
// variants from DeepSeek include:
//
//	"The `reasoning_content` in the thinking mode must be passed back to the API."
//	"The `content[].thinking` in the thinking mode must be passed back to the API."
func isDeepSeekThinkingContextError(respBody []byte) bool {
	msg := strings.ToLower(extractUpstreamErrorMessage(respBody))
	if msg == "" {
		return false
	}
	if !strings.Contains(msg, "thinking mode") && !strings.Contains(msg, "thinking_mode") {
		return false
	}
	return strings.Contains(msg, "reasoning_content") ||
		strings.Contains(msg, "content[].thinking") ||
		strings.Contains(msg, "passed back")
}

// handleDeepSeekStreamingResponse translates an upstream Chat Completions SSE
// stream into Anthropic SSE events written live to the client.
func (s *GatewayService) handleDeepSeekStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	startTime time.Time,
) (apicompat.AnthropicUsage, *int, bool, error) {
	requestID := resp.Header.Get("x-request-id")

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	chatState := apicompat.NewChatToResponsesEventState()
	chatState.Model = originalModel
	anthState := apicompat.NewResponsesEventToAnthropicState()
	anthState.Model = originalModel

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var (
		usage            apicompat.AnthropicUsage
		firstTokenMs     *int
		firstChunk       = true
		clientDisconnect bool
	)

	emit := func(events []apicompat.ResponsesStreamEvent) bool {
		for _, evt := range events {
			// Track usage.
			if (evt.Type == "response.completed" || evt.Type == "response.incomplete" || evt.Type == "response.failed") &&
				evt.Response != nil && evt.Response.Usage != nil {
				usage.InputTokens = evt.Response.Usage.InputTokens
				usage.OutputTokens = evt.Response.Usage.OutputTokens
				if evt.Response.Usage.InputTokensDetails != nil {
					usage.CacheReadInputTokens = evt.Response.Usage.InputTokensDetails.CachedTokens
				}
			}
			for _, anthEvt := range apicompat.ResponsesEventToAnthropicEvents(&evt, anthState) {
				sse, err := apicompat.ResponsesAnthropicEventToSSE(anthEvt)
				if err != nil {
					continue
				}
				if _, err := fmt.Fprint(c.Writer, sse); err != nil {
					logger.L().Info("deepseek stream: client disconnected",
						zap.String("request_id", requestID))
					return true
				}
			}
			c.Writer.Flush()
		}
		return false
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := line[6:]
		if payload == "[DONE]" {
			continue
		}
		if firstChunk {
			firstChunk = false
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}

		var chunk apicompat.ChatCompletionsChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			logger.L().Warn("deepseek stream: parse chunk failed",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
			continue
		}

		events := apicompat.ChatChunkToResponsesEvents(chunk, chatState)
		if emit(events) {
			clientDisconnect = true
			return usage, firstTokenMs, clientDisconnect, nil
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		logger.L().Warn("deepseek stream: read error",
			zap.Error(err),
			zap.String("request_id", requestID),
		)
	}

	// Emit deferred completion + termination events.
	if finalEvents := apicompat.FinalizeChatToResponsesStream(chatState); len(finalEvents) > 0 {
		emit(finalEvents)
	}
	if termEvents := apicompat.FinalizeResponsesAnthropicStream(anthState); len(termEvents) > 0 {
		for _, evt := range termEvents {
			sse, err := apicompat.ResponsesAnthropicEventToSSE(evt)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprint(c.Writer, sse); err != nil {
				break
			}
		}
		c.Writer.Flush()
	}
	return usage, firstTokenMs, clientDisconnect, nil
}

// handleDeepSeekBufferedResponse consumes the upstream SSE stream, assembles a
// complete ChatCompletionsResponse, then converts it to a non-streaming
// Anthropic response JSON written to the client.
func (s *GatewayService) handleDeepSeekBufferedResponse(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
) (apicompat.AnthropicUsage, error) {
	requestID := resp.Header.Get("x-request-id")

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	accumulator := newChatCompletionsAccumulator()
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := line[6:]
		if payload == "[DONE]" {
			continue
		}
		var chunk apicompat.ChatCompletionsChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			logger.L().Warn("deepseek buffered: parse chunk failed",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
			continue
		}
		accumulator.ingest(chunk)
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		logger.L().Warn("deepseek buffered: read error",
			zap.Error(err),
			zap.String("request_id", requestID),
		)
	}

	chatResp := accumulator.build()
	responsesResp := apicompat.ChatCompletionsResponseToResponses(chatResp, originalModel)
	anthropicResp := apicompat.ResponsesToAnthropic(responsesResp, originalModel)

	c.JSON(http.StatusOK, anthropicResp)
	return anthropicResp.Usage, nil
}

// chatCompletionsAccumulator assembles a full ChatCompletionsResponse from a
// sequence of streaming chunks.
type chatCompletionsAccumulator struct {
	id           string
	model        string
	contentText  strings.Builder
	reasoning    strings.Builder
	finishReason string
	usage        *apicompat.ChatUsage
	toolCalls    map[int]*apicompat.ChatToolCall
	toolCallIdx  []int // ordered insertion
}

func newChatCompletionsAccumulator() *chatCompletionsAccumulator {
	return &chatCompletionsAccumulator{toolCalls: make(map[int]*apicompat.ChatToolCall)}
}

func (a *chatCompletionsAccumulator) ingest(chunk apicompat.ChatCompletionsChunk) {
	if chunk.ID != "" && a.id == "" {
		a.id = chunk.ID
	}
	if chunk.Model != "" && a.model == "" {
		a.model = chunk.Model
	}
	if chunk.Usage != nil {
		a.usage = chunk.Usage
	}
	for _, choice := range chunk.Choices {
		delta := choice.Delta
		if delta.Content != nil && *delta.Content != "" {
			a.contentText.WriteString(*delta.Content)
		}
		if delta.ReasoningContent != nil && *delta.ReasoningContent != "" {
			a.reasoning.WriteString(*delta.ReasoningContent)
		}
		for _, tc := range delta.ToolCalls {
			idx := 0
			if tc.Index != nil {
				idx = *tc.Index
			}
			existing, ok := a.toolCalls[idx]
			if !ok {
				existing = &apicompat.ChatToolCall{Type: "function"}
				a.toolCalls[idx] = existing
				a.toolCallIdx = append(a.toolCallIdx, idx)
			}
			if tc.ID != "" {
				existing.ID = tc.ID
			}
			if tc.Type != "" {
				existing.Type = tc.Type
			}
			if tc.Function.Name != "" {
				existing.Function.Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				existing.Function.Arguments += tc.Function.Arguments
			}
		}
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			a.finishReason = *choice.FinishReason
		}
	}
}

func (a *chatCompletionsAccumulator) build() *apicompat.ChatCompletionsResponse {
	msg := apicompat.ChatMessage{Role: "assistant"}
	if text := a.contentText.String(); text != "" {
		msg.Content, _ = json.Marshal(text)
	}
	if reasoning := a.reasoning.String(); reasoning != "" {
		msg.ReasoningContent = reasoning
	}
	for _, idx := range a.toolCallIdx {
		tc := a.toolCalls[idx]
		if tc == nil {
			continue
		}
		msg.ToolCalls = append(msg.ToolCalls, *tc)
	}
	out := &apicompat.ChatCompletionsResponse{
		ID:    a.id,
		Model: a.model,
		Choices: []apicompat.ChatChoice{{
			Index:        0,
			Message:      msg,
			FinishReason: a.finishReason,
		}},
	}
	if a.usage != nil {
		out.Usage = a.usage
	}
	return out
}

// applyDeepSeekThinkingMode is the single seam for injecting a provider-
// specific thinking toggle into a ChatCompletions request. DeepSeek V4
// currently selects thinking via the "[thinking]" model id suffix, which we
// keep on chatReq.Model already — so this hook is a no-op today. If the V4
// API later exposes a dedicated body flag (e.g. "thinking": true or
// "reasoning": {...}), wire it here in one place.
func applyDeepSeekThinkingMode(req *apicompat.ChatCompletionsRequest, enabled bool) {
	_ = req
	_ = enabled
}

func buildDeepSeekChatCompletionsURL(s *GatewayService, account *Account) (string, error) {
	baseURL := account.GetDeepSeekBaseURL()
	if baseURL == "" {
		return "", fmt.Errorf("deepseek account %d has no base_url", account.ID)
	}
	validated, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	base := strings.TrimRight(validated, "/")
	// Tolerate users configuring base_url with the /anthropic subpath from the
	// legacy passthrough path; strip it so we land on /v1/chat/completions.
	base = strings.TrimSuffix(base, "/anthropic")
	return base + "/v1/chat/completions", nil
}

func anthropicUsageToClaudeUsage(u apicompat.AnthropicUsage) ClaudeUsage {
	return ClaudeUsage{
		InputTokens:              u.InputTokens,
		OutputTokens:             u.OutputTokens,
		CacheCreationInputTokens: u.CacheCreationInputTokens,
		CacheReadInputTokens:     u.CacheReadInputTokens,
	}
}

// deepseekTranslationEnabled gates whether ForwardDeepSeek is used over the
// legacy Anthropic passthrough. Set SUB2API_DEEPSEEK_TRANSLATION=0 to fall
// back to the old path. Default: enabled.
func deepseekTranslationEnabled() bool {
	v := strings.TrimSpace(os.Getenv("SUB2API_DEEPSEEK_TRANSLATION"))
	if v == "" {
		return true
	}
	switch strings.ToLower(v) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}
