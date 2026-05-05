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
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// ForwardUniversalChatCompletions dispatches to the appropriate upstream based
// on the selected account's platform. For OpenAI accounts it does a direct
// CC→OpenAI passthrough (no format conversion). For Anthropic/Antigravity/Gemini
// accounts it delegates to the existing Anthropic conversion chain.
func (s *GatewayService) ForwardUniversalChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	parsed *ParsedRequest,
) (*ForwardResult, error) {
	switch account.Platform {
	case PlatformOpenAI:
		return s.forwardAsChatCompletionsOpenAI(ctx, c, account, body, parsed)
	default:
		// anthropic, antigravity, gemini → existing Anthropic chain
		return s.ForwardAsChatCompletions(ctx, c, account, body, parsed)
	}
}

// ForwardUniversalResponses dispatches to the appropriate upstream based
// on the selected account's platform. For OpenAI accounts it does a direct
// Responses→OpenAI passthrough. For Anthropic/Antigravity/Gemini accounts
// it delegates to the existing ForwardAsResponses.
func (s *GatewayService) ForwardUniversalResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	parsed *ParsedRequest,
) (*ForwardResult, error) {
	switch account.Platform {
	case PlatformOpenAI:
		return s.forwardAsResponsesOpenAI(ctx, c, account, body, parsed)
	default:
		return s.ForwardAsResponses(ctx, c, account, body, parsed)
	}
}

// forwardAsChatCompletionsOpenAI forwards a Chat Completions request directly
// to an OpenAI-compatible upstream (no format conversion, CC body is already
// in OpenAI format). The response is streamed back to the client as-is.
func (s *GatewayService) forwardAsChatCompletionsOpenAI(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	parsed *ParsedRequest,
) (*ForwardResult, error) {
	startTime := time.Now()

	originalModel := gjson.GetBytes(body, "model").String()
	clientStream := gjson.GetBytes(body, "stream").Bool()
	includeUsage := gjson.GetBytes(body, "stream_options.include_usage").Bool()

	// Model mapping for API Key accounts
	mappedModel := originalModel
	if account.Type == AccountTypeAPIKey {
		if m := account.GetMappedModel(originalModel); m != "" {
			mappedModel = m
		}
	}
	if mappedModel != originalModel {
		body = replaceModelInBody(body, mappedModel)
	}

	// For OAuth accounts, convert Chat Completions body to Responses format
	// since the upstream endpoint is chatgpt.com/backend-api/codex/responses
	if account.Type == AccountTypeOAuth {
		var chatReq apicompat.ChatCompletionsRequest
		if err := json.Unmarshal(body, &chatReq); err == nil {
			if responsesReq, err := apicompat.ChatCompletionsToResponses(&chatReq); err == nil {
				if converted, err := json.Marshal(responsesReq); err == nil {
					body = converted
				}
			}
		}
		// Apply Codex OAuth transform (sets instructions, model normalization, etc.)
		var reqBody map[string]any
		if err := json.Unmarshal(body, &reqBody); err == nil {
			applyCodexOAuthTransform(reqBody, false, false)
			// Responses API requires instructions field
			if _, ok := reqBody["instructions"]; !ok {
				reqBody["instructions"] = ""
			}
			if transformed, err := json.Marshal(reqBody); err == nil {
				body = transformed
			}
		}
	}

	logger.L().Debug("universal forward openai: model mapping",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("mapped_model", mappedModel),
	)

	// Get access token
	token, tokenType, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	// Get proxy URL
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	// Build upstream request for OpenAI
	upstreamCtx, releaseUpstreamCtx := detachStreamUpstreamContext(ctx, clientStream)
	upstreamReq, err := s.buildOpenAIUpstreamRequest(upstreamCtx, c, account, body, token, tokenType, mappedModel, clientStream)
	releaseUpstreamCtx()
	if err != nil {
		return nil, fmt.Errorf("build openai upstream request: %w", err)
	}

	// Send request
	// OpenAI OAuth accounts go through ChatGPT internal API (chatgpt.com) which is
	// protected by Cloudflare WAF. TLS fingerprint emulation triggers WAF detection,
	// so we use plain Do() for OAuth accounts.
	var resp *http.Response
	if account.Type == AccountTypeOAuth {
		resp, err = s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	} else {
		resp, err = s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	}
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			Kind:               "request_error",
			Message:            safeErr,
		})
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Upstream request failed")
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	// Handle error response
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))

		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)

		if s.shouldFailoverUpstreamError(resp.StatusCode) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  resp.Header.Get("x-request-id"),
				Kind:               "failover",
				Message:            upstreamMsg,
			})
			if s.rateLimitService != nil {
				s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
			}
			return nil, &UpstreamFailoverError{
				StatusCode:   resp.StatusCode,
				ResponseBody: respBody,
			}
		}

		writeGatewayCCError(c, mapUpstreamStatusCode(resp.StatusCode), "server_error", upstreamMsg)
		return nil, fmt.Errorf("upstream error: %d %s", resp.StatusCode, upstreamMsg)
	}

	// Forward response to client
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}

	var result *ForwardResult
	var handleErr error
	if clientStream {
		result, handleErr = s.forwardOpenAIStreamingResponse(resp, c, originalModel, mappedModel, startTime, includeUsage)
	} else {
		result, handleErr = s.forwardOpenAIBufferedResponse(resp, c, originalModel, mappedModel, startTime)
	}
	return result, handleErr
}

// forwardAsResponsesOpenAI forwards a Responses API request directly
// to an OpenAI-compatible upstream (no format conversion, Responses body is
// already in OpenAI-compatible format). The response is streamed back as-is.
func (s *GatewayService) forwardAsResponsesOpenAI(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	parsed *ParsedRequest,
) (*ForwardResult, error) {
	startTime := time.Now()

	originalModel := gjson.GetBytes(body, "model").String()
	clientStream := gjson.GetBytes(body, "stream").Bool()

	// Model mapping for API Key accounts
	mappedModel := originalModel
	if account.Type == AccountTypeAPIKey {
		if m := account.GetMappedModel(originalModel); m != "" {
			mappedModel = m
		}
	}
	if mappedModel != originalModel {
		body = replaceModelInBody(body, mappedModel)
	}

	logger.L().Debug("universal forward openai responses: model mapping",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("mapped_model", mappedModel),
	)

	// Get access token
	token, tokenType, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	// Get proxy URL
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	// Build upstream request for OpenAI
	upstreamCtx, releaseUpstreamCtx := detachStreamUpstreamContext(ctx, clientStream)
	upstreamReq, err := s.buildOpenAIUpstreamRequest(upstreamCtx, c, account, body, token, tokenType, mappedModel, clientStream)
	releaseUpstreamCtx()
	if err != nil {
		return nil, fmt.Errorf("build openai upstream request: %w", err)
	}

	// Send request
	// OpenAI OAuth accounts go through ChatGPT internal API (chatgpt.com) which is
	// protected by Cloudflare WAF. TLS fingerprint emulation triggers WAF detection,
	// so we use plain Do() for OAuth accounts.
	var resp2 *http.Response
	if account.Type == AccountTypeOAuth {
		resp2, err = s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	} else {
		resp2, err = s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	}
	if err != nil {
		if resp2 != nil && resp2.Body != nil {
			_ = resp2.Body.Close()
		}
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			Kind:               "request_error",
			Message:            safeErr,
		})
		writeResponsesError(c, http.StatusBadGateway, "server_error", "Upstream request failed")
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp2.Body.Close() }()

	// Handle error response
	if resp2.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp2.Body, 2<<20))
		_ = resp2.Body.Close()
		resp2.Body = io.NopCloser(bytes.NewReader(respBody))

		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)

		if s.shouldFailoverUpstreamError(resp2.StatusCode) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp2.StatusCode,
				UpstreamRequestID:  resp2.Header.Get("x-request-id"),
				Kind:               "failover",
				Message:            upstreamMsg,
			})
			if s.rateLimitService != nil {
				s.rateLimitService.HandleUpstreamError(ctx, account, resp2.StatusCode, resp2.Header, respBody)
			}
			return nil, &UpstreamFailoverError{
				StatusCode:   resp2.StatusCode,
				ResponseBody: respBody,
			}
		}

		writeResponsesError(c, mapUpstreamStatusCode(resp2.StatusCode), "server_error", upstreamMsg)
		return nil, fmt.Errorf("upstream error: %d %s", resp2.StatusCode, upstreamMsg)
	}

	// Forward response to client
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp2.Header, s.responseHeaderFilter)
	}

	var result *ForwardResult
	var handleErr error
	if clientStream {
		result, handleErr = s.forwardOpenAIStreamingResponse(resp2, c, originalModel, mappedModel, startTime, false)
	} else {
		result, handleErr = s.forwardOpenAIBufferedResponse(resp2, c, originalModel, mappedModel, startTime)
	}
	return result, handleErr
}
func (s *GatewayService) buildOpenAIUpstreamRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	token, tokenType, model string,
	stream bool,
) (*http.Request, error) {
	var url string
	if account.Type == AccountTypeOAuth {
		// OAuth accounts use ChatGPT internal API, not api.openai.com.
		// Do NOT use GetOpenAIBaseURL() — it returns api.openai.com for all
		// OpenAI accounts, which sends the request to the wrong host.
		url = "https://chatgpt.com/backend-api/codex/responses"
	} else {
		// API Key accounts: use OpenAI Platform API or custom base URL
		customBase := account.GetOpenAIBaseURL()
		if customBase == "" {
			customBase = "https://api.openai.com"
		}
		url = customBase + "/v1/chat/completions"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "text/event-stream")

	// Auth — OpenAI always uses Bearer
	if token != "" {
		req.Header.Set("authorization", "Bearer "+token)
	}

	// OAuth: set ChatGPT required headers
	if account.Type == AccountTypeOAuth {
		req.Host = "chatgpt.com"
		if chatgptAccountID := account.GetChatGPTAccountID(); chatgptAccountID != "" {
			req.Header.Set("chatgpt-account-id", chatgptAccountID)
		}
		req.Header.Set("OpenAI-Beta", "responses=experimental")
		req.Header.Set("originator", "codex_cli_rs")
		req.Header.Set("version", "0.104.0")
		req.Header.Set("accept", "text/event-stream")
		if deviceID := account.GetOpenAIDeviceID(); deviceID != "" {
			req.Header.Set("oai-device-id", deviceID)
			req.Header.Set("Cookie", "oai-did="+deviceID)
		}
		if sessionID := account.GetOpenAISessionID(); sessionID != "" {
			req.Header.Set("oai-session-id", sessionID)
		}
		// Apply custom User-Agent if configured
		if ua := account.GetOpenAIUserAgent(); ua != "" {
			req.Header.Set("user-agent", ua)
		}
		// OAuth safety: default to Codex CLI UA if no custom UA set
		if req.Header.Get("user-agent") == "" {
			req.Header.Set("user-agent", "codex_cli_rs/0.104.0")
		}
	}

	// Copy relevant headers from the original request
	for _, h := range []string{"X-Request-ID", "Traceparent"} {
		if v := c.GetHeader(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	// Only copy User-Agent if not already set by OAuth branch
	if account.Type != AccountTypeOAuth || req.Header.Get("user-agent") == "" {
		if v := c.GetHeader("User-Agent"); v != "" {
			req.Header.Set("user-agent", v)
		}
	}

	return req, nil
}

// forwardOpenAIStreamingResponse streams the OpenAI SSE response to the client,
// converting Responses API SSE events to Chat Completions SSE format.
func (s *GatewayService) forwardOpenAIStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	mappedModel string,
	startTime time.Time,
	includeUsage bool,
) (*ForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	var usage ClaudeUsage
	var firstTokenMs *int
	firstChunk := true
	created := time.Now().Unix()
	chunkIndex := 0
	sawTerminal := false

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	resultWithUsage := func() *ForwardResult {
		return &ForwardResult{
			RequestID:     requestID,
			Usage:         usage,
			Model:         originalModel,
			UpstreamModel: mappedModel,
			Stream:        true,
			Duration:      time.Since(startTime),
			FirstTokenMs:  firstTokenMs,
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimPrefix(strings.TrimPrefix(line, "data: "), "data:")
		if data == "[DONE]" {
			sawTerminal = true
			break
		}

		eventType := gjson.Get(data, "type").String()

		switch eventType {
		case "response.output_text.delta":
			if firstChunk {
				firstChunk = false
				ms := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &ms
			}
			delta := gjson.Get(data, "delta").String()
			chunk := buildChatCompletionsChunk(delta, originalModel, created, chunkIndex)
			fmt.Fprint(c.Writer, chunk)
			chunkIndex++
			c.Writer.Flush()

		case "response.done", "response.completed":
			sawTerminal = true
			if pt := gjson.Get(data, "response.usage.input_tokens"); pt.Exists() {
				usage.InputTokens = int(pt.Int())
			}
			if ct := gjson.Get(data, "response.usage.output_tokens"); ct.Exists() {
				usage.OutputTokens = int(ct.Int())
			}
			if includeUsage {
				usageSSE := buildCCUsageSSE(usage, originalModel)
				fmt.Fprint(c.Writer, usageSSE) //nolint:errcheck
			}
			fmt.Fprint(c.Writer, "data: [DONE]\n\n") //nolint:errcheck
			c.Writer.Flush()
			return resultWithUsage(), nil

		case "error":
			errMsg := gjson.Get(data, "error.message").String()
			if errMsg == "" {
				errMsg = "Upstream error"
			}
			logger.L().Warn("openai streaming error",
				zap.String("request_id", requestID),
				zap.String("error", errMsg),
			)
			fmt.Fprint(c.Writer, "data: [DONE]\n\n") //nolint:errcheck
			c.Writer.Flush()
			return resultWithUsage(), nil
		}
	}

	if err := scanner.Err(); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logger.L().Warn("forward openai stream: read error",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
		}
	}

	if !sawTerminal {
		if includeUsage {
			usageSSE := buildCCUsageSSE(usage, originalModel)
			fmt.Fprint(c.Writer, usageSSE) //nolint:errcheck
		}
		fmt.Fprint(c.Writer, "data: [DONE]\n\n") //nolint:errcheck
	}
	c.Writer.Flush()

	return resultWithUsage(), nil
}

// forwardOpenAIBufferedResponse reads the full OpenAI response and writes it to the client.
// If the upstream returns Responses API format, it converts to Chat Completions format.
func (s *GatewayService) forwardOpenAIBufferedResponse(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	mappedModel string,
	startTime time.Time,
) (*ForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		writeGatewayCCError(c, http.StatusBadGateway, "server_error", "Failed to read upstream response")
		return nil, fmt.Errorf("read upstream response: %w", err)
	}

	// Check if this is a Responses API format response and convert if needed
	if isResponsesAPIFormat(respBody) {
		respBody = convertResponsesToChatCompletions(respBody, originalModel, mappedModel)
	} else if originalModel != mappedModel {
		// Already Chat Completions format, just replace model name
		respBody = bytes.ReplaceAll(respBody, []byte(`"model":"`+mappedModel+`"`), []byte(`"model":"`+originalModel+`"`))
	}

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", respBody)

	// Extract usage
	var usage ClaudeUsage
	usageData := gjson.GetBytes(respBody, "usage")
	if usageData.Exists() {
		if pt := usageData.Get("prompt_tokens"); pt.Exists() {
			usage.InputTokens = int(pt.Int())
		}
		if ct := usageData.Get("completion_tokens"); ct.Exists() {
			usage.OutputTokens = int(ct.Int())
		}
	}

	// Also check Responses API usage format if not already captured
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		if pt := gjson.GetBytes(respBody, "response.usage.input_tokens"); pt.Exists() {
			usage.InputTokens = int(pt.Int())
		}
		if ct := gjson.GetBytes(respBody, "response.usage.output_tokens"); ct.Exists() {
			usage.OutputTokens = int(ct.Int())
		}
	}

	return &ForwardResult{
		RequestID:     requestID,
		Usage:         usage,
		Model:         originalModel,
		UpstreamModel: mappedModel,
		Stream:        false,
		Duration:      time.Since(startTime),
	}, nil
}

// replaceModelInBody replaces the "model" field in a JSON body.
func replaceModelInBody(body []byte, model string) []byte {
	old := gjson.GetBytes(body, "model").String()
	if old == "" || old == model {
		return body
	}
	return bytes.ReplaceAll(body,
		[]byte(`"model":"`+old+`"`),
		[]byte(`"model":"`+model+`"`),
	)
}

// buildCCUsageSSE builds a Chat Completions usage-only SSE chunk.
func buildCCUsageSSE(usage ClaudeUsage, model string) string {
	return fmt.Sprintf(`data: {"id":"usage","object":"chat.completion.chunk","created":0,"model":"%s","choices":[],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d}}

`, model, usage.InputTokens, usage.OutputTokens, usage.InputTokens+usage.OutputTokens)
}

// buildChatCompletionsChunk builds a Chat Completions SSE chunk from text delta.
func buildChatCompletionsChunk(delta string, model string, created int64, index int) string {
	escaped, _ := json.Marshal(delta)
	return fmt.Sprintf(`data: {"id":"chatcmpl-universal-%d-%d","object":"chat.completion.chunk","created":%d,"model":"%s","choices":[{"index":%d,"delta":{"content":%s}}]}

`, index, index, created, model, index, string(escaped))
}

// isResponsesAPIFormat checks if a JSON body is in Responses API format.
func isResponsesAPIFormat(body []byte) bool {
	if t := gjson.GetBytes(body, "type"); t.Exists() && t.Str == "response.completed" {
		return true
	}
	if out := gjson.GetBytes(body, "response.output"); out.Exists() {
		return true
	}
	return false
}

// convertResponsesToChatCompletions converts a Responses API JSON response to Chat Completions format.
func convertResponsesToChatCompletions(body []byte, originalModel, mappedModel string) []byte {
	var textContent string
	outputItems := gjson.GetBytes(body, "response.output")
	if outputItems.Exists() && outputItems.IsArray() {
		for _, item := range outputItems.Array() {
			for _, content := range item.Get("content").Array() {
				if t := content.Get("text"); t.Exists() {
					textContent += t.String()
				}
			}
		}
	}

	respModel := originalModel

	inputTokens := int(gjson.GetBytes(body, "response.usage.input_tokens").Int())
	outputTokens := int(gjson.GetBytes(body, "response.usage.output_tokens").Int())
	totalTokens := inputTokens + outputTokens

	result := map[string]any{
		"id":      gjson.GetBytes(body, "response.id").String(),
		"object":  "chat.completion",
		"created": gjson.GetBytes(body, "response.created_at").Int(),
		"model":   respModel,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": textContent,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     inputTokens,
			"completion_tokens": outputTokens,
			"total_tokens":      totalTokens,
		},
	}

	b, _ := json.Marshal(result)
	return b
}
