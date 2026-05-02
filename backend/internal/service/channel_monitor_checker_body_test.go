//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// swapMonitorHTTPClient 临时替换 monitorHTTPClient 为不带 SSRF 校验的普通 client，
// 让 httptest (127.0.0.1) 能连通。测试结束后恢复。
func swapMonitorHTTPClient(t *testing.T) {
	t.Helper()
	orig := monitorHTTPClient
	monitorHTTPClient = &http.Client{Timeout: 5 * time.Second}
	t.Cleanup(func() { monitorHTTPClient = orig })
}

// captureHandler 把每次收到的请求 body 和 headers 存起来，测试断言用。
type captureHandler struct {
	lastBody    map[string]any
	lastHeaders http.Header
	respondText string // 写到 Anthropic content[0].text 里（校验用）
	status      int
}

func (h *captureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.lastHeaders = r.Header.Clone()
	defer func() { _ = r.Body.Close() }()
	var parsed map[string]any
	_ = json.NewDecoder(r.Body).Decode(&parsed)
	h.lastBody = parsed

	if h.status == 0 {
		h.status = 200
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.status)
	// 构造 Anthropic 格式的响应：content[0].text = h.respondText
	_ = json.NewEncoder(w).Encode(map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": h.respondText},
		},
	})
}

func setupFakeAnthropic(t *testing.T, handler *captureHandler) string {
	t.Helper()
	swapMonitorHTTPClient(t)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv.URL
}

func setupFakeMonitorProvider(t *testing.T, handler http.Handler) string {
	t.Helper()
	swapMonitorHTTPClient(t)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv.URL
}

var monitorPromptQuestionRegex = regexp.MustCompile(`Q: (\d+) ([+-]) (\d+) = \?\s*A:\s*$`)

func expectedAnswerFromPrompt(prompt string) (string, error) {
	matches := monitorPromptQuestionRegex.FindStringSubmatch(prompt)
	if len(matches) != 4 {
		return "", fmt.Errorf("monitor prompt did not contain final arithmetic question: %q", prompt)
	}
	left, err := strconv.Atoi(matches[1])
	if err != nil {
		return "", err
	}
	right, err := strconv.Atoi(matches[3])
	if err != nil {
		return "", err
	}
	switch matches[2] {
	case "+":
		return strconv.Itoa(left + right), nil
	case "-":
		return strconv.Itoa(left - right), nil
	default:
		return "", fmt.Errorf("unsupported operator %q", matches[2])
	}
}

func anthropicPromptFromBody(body map[string]any) string {
	messages, _ := body["messages"].([]any)
	if len(messages) == 0 {
		return ""
	}
	message, _ := messages[0].(map[string]any)
	switch content := message["content"].(type) {
	case string:
		return content
	case []any:
		for _, part := range content {
			partMap, _ := part.(map[string]any)
			if text, _ := partMap["text"].(string); text != "" {
				return text
			}
		}
	}
	return ""
}

func geminiPromptFromBody(body map[string]any) string {
	contents, _ := body["contents"].([]any)
	if len(contents) == 0 {
		return ""
	}
	content, _ := contents[0].(map[string]any)
	parts, _ := content["parts"].([]any)
	if len(parts) == 0 {
		return ""
	}
	part, _ := parts[0].(map[string]any)
	text, _ := part["text"].(string)
	return text
}

func TestRunCheckForModel_OffMode_PreservesDefaultBody(t *testing.T) {
	h := &captureHandler{respondText: "the answer is 42"}
	endpoint := setupFakeAnthropic(t, h)

	// 跑一次 off 模式（opts=nil），确认默认 body 行为未变
	_ = runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", nil)

	if h.lastBody["model"] != "claude-x" {
		t.Errorf("default body should contain model=claude-x, got %v", h.lastBody["model"])
	}
	if _, ok := h.lastBody["messages"]; !ok {
		t.Error("default body should contain messages")
	}
	if h.lastBody["stream"] != nil {
		t.Errorf("default body should not enable streaming, got %v", h.lastBody["stream"])
	}
	if mt, ok := h.lastBody["max_tokens"].(float64); !ok || mt != monitorChallengeMaxTokens {
		t.Errorf("default body should use max_tokens=%d, got %v", monitorChallengeMaxTokens, h.lastBody["max_tokens"])
	}
	if h.lastHeaders.Get("x-api-key") != "sk-fake" {
		t.Errorf("expected adapter's x-api-key header, got %q", h.lastHeaders.Get("x-api-key"))
	}
}

func TestRunCheckForModel_MergeMode_UserFieldsWinButDenyListProtects(t *testing.T) {
	h := &captureHandler{respondText: "the answer is 42"}
	endpoint := setupFakeAnthropic(t, h)

	opts := &CheckOptions{
		BodyOverrideMode: MonitorBodyOverrideModeMerge,
		BodyOverride: map[string]any{
			"system":     "You are Claude Code...",
			"max_tokens": float64(999),   // 应该覆盖默认 50
			"model":      "hacked-model", // 应该被黑名单挡住，保留原 model
			"messages":   []any{},        // 同上，被挡
		},
		ExtraHeaders: map[string]string{
			"User-Agent":     "claude-cli/1.0",
			"Content-Length": "999", // 黑名单
			"x-custom":       "ok",
		},
	}
	_ = runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", opts)

	if h.lastBody["system"] != "You are Claude Code..." {
		t.Errorf("merge mode should inject system, got %v", h.lastBody["system"])
	}
	// max_tokens 覆盖生效
	if mt, ok := h.lastBody["max_tokens"].(float64); !ok || mt != 999 {
		t.Errorf("merge mode should override max_tokens to 999, got %v", h.lastBody["max_tokens"])
	}
	// model 在黑名单 — 应该保留默认值
	if h.lastBody["model"] != "claude-x" {
		t.Errorf("model should be protected by deny list, got %v", h.lastBody["model"])
	}
	// messages 在黑名单 — 应该保留默认值（非空）
	msgs, _ := h.lastBody["messages"].([]any)
	if len(msgs) == 0 {
		t.Error("messages should be protected by deny list (kept default, non-empty)")
	}
	// header 合并
	if h.lastHeaders.Get("User-Agent") != "claude-cli/1.0" {
		t.Errorf("extra User-Agent should override, got %q", h.lastHeaders.Get("User-Agent"))
	}
	if h.lastHeaders.Get("x-custom") != "ok" {
		t.Errorf("extra custom header should be present, got %q", h.lastHeaders.Get("x-custom"))
	}
	// Content-Length 黑名单：会被 net/http 自动重算，但不应由用户的 "999" 决定。
	// 我们无法直接断言丢弃（http.Client 总会填上），只断言请求成功即可。
}

func TestRunCheckForModel_ReplaceMode_FullBodyUsedAndChallengeSkipped(t *testing.T) {
	// replace 模式下我们的 body 完全自定义，challenge 数学题不会出现在请求里，
	// 上游也不会回正确答案 — 但只要 2xx + 响应文本非空，就算 operational
	h := &captureHandler{respondText: "any non-empty text"}
	endpoint := setupFakeAnthropic(t, h)

	userBody := map[string]any{
		"model":      "user-forced-model",
		"messages":   []any{map[string]any{"role": "user", "content": "hi"}},
		"max_tokens": float64(10),
		"system":     "You are someone else",
	}
	opts := &CheckOptions{
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride:     userBody,
	}
	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", opts)

	// 请求 body = 用户提供的原样
	if h.lastBody["model"] != "user-forced-model" {
		t.Errorf("replace mode should use user's model, got %v", h.lastBody["model"])
	}
	if h.lastBody["system"] != "You are someone else" {
		t.Errorf("replace mode should use user's system, got %v", h.lastBody["system"])
	}
	// challenge 虽然没命中，但由于 replace 模式跳过 challenge 校验 + 响应非空 → operational
	if res.Status != MonitorStatusOperational {
		t.Errorf("replace mode with 2xx + non-empty text should be operational, got status=%s message=%q",
			res.Status, res.Message)
	}
}

func TestRunCheckForModel_ReplaceMode_EmptyResponseIsFailed(t *testing.T) {
	h := &captureHandler{respondText: ""} // 上游 200 但 content[0].text 为空
	endpoint := setupFakeAnthropic(t, h)

	opts := &CheckOptions{
		BodyOverrideMode: MonitorBodyOverrideModeReplace,
		BodyOverride:     map[string]any{"model": "x", "messages": []any{}},
	}
	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-x", opts)

	if res.Status != MonitorStatusFailed {
		t.Errorf("replace mode with empty text should be failed, got status=%s", res.Status)
	}
	if !strings.Contains(res.Message, "replace-mode") {
		t.Errorf("failure message should hint replace-mode, got %q", res.Message)
	}
}

func TestRunCheckForModel_AnthropicTextAfterThinkingBlockPassesChallenge(t *testing.T) {
	endpoint := setupFakeMonitorProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		answer, err := expectedAnswerFromPrompt(anthropicPromptFromBody(body))
		if err != nil {
			t.Fatalf("extract expected answer: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "thinking", "thinking": ""},
				{"type": "text", "text": answer},
			},
			"usage": map[string]any{"input_tokens": 562, "output_tokens": 57},
		})
	}))

	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-opus-4-6", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational when Anthropic text follows thinking block, got status=%s message=%q", res.Status, res.Message)
	}
}

func TestRunCheckForModel_AnthropicCompatibilityProbePassesClaudeCodeValidation(t *testing.T) {
	validator := NewClaudeCodeValidator()
	endpoint := setupFakeMonitorProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if !validator.Validate(r, body) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"type":    nil,
					"message": "This API is only for use in claude code",
				},
				"type": "error",
			})
			return
		}
		answer, err := expectedAnswerFromPrompt(anthropicPromptFromBody(body))
		if err != nil {
			t.Fatalf("extract expected answer: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": answer}},
		})
	}))

	opts := &CheckOptions{CompatibilityProbeEnabled: true}
	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-opus-4-7", opts)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational with Claude Code-like compatibility probe request, got status=%s message=%q", res.Status, res.Message)
	}
}

func TestRunCheckForModel_AnthropicCompatibilityProbeUsesClaudeCodeStreamingBody(t *testing.T) {
	endpoint := setupFakeMonitorProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["stream"] != true || body["temperature"] != float64(1) || body["max_tokens"].(float64) < 1024 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"expected Claude Code streaming probe body"}}`))
			return
		}
		messages, _ := body["messages"].([]any)
		message, _ := messages[0].(map[string]any)
		content, _ := message["content"].([]any)
		contentPart, _ := content[0].(map[string]any)
		cacheControl, _ := contentPart["cache_control"].(map[string]any)
		if _, hasTTL := cacheControl["ttl"]; hasTTL {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"unexpected cache_control ttl for API-key probe"}}`))
			return
		}
		answer, err := expectedAnswerFromPrompt(anthropicPromptFromBody(body))
		if err != nil {
			t.Fatalf("extract expected answer: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\n", answer)
		fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	}))

	opts := &CheckOptions{CompatibilityProbeEnabled: true}
	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-opus-4-6", opts)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational with Claude Code streaming body, got status=%s message=%q", res.Status, res.Message)
	}
}

func TestRunCheckForModel_AnthropicCompatibilityProbeOverridesCriticalClaudeHeaders(t *testing.T) {
	endpoint := setupFakeMonitorProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "claude-cli/") {
			t.Fatalf("compatibility probe should send Claude CLI User-Agent, got %q", got)
		}
		if got := r.Header.Get("X-App"); got != "cli" {
			t.Fatalf("compatibility probe should send X-App=cli, got %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("compatibility probe should send Accept=application/json, got %q", got)
		}
		if got := r.Header.Get("x-stainless-helper-method"); got != "stream" {
			t.Fatalf("compatibility probe should mark stream helper method, got %q", got)
		}
		if got := r.Header.Get("x-client-request-id"); got == "" {
			t.Fatal("compatibility probe should include x-client-request-id")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		answer, err := expectedAnswerFromPrompt(anthropicPromptFromBody(body))
		if err != nil {
			t.Fatalf("extract expected answer: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: content_block_delta\ndata: {\"delta\":{\"text\":%q}}\n\n", answer)
	}))

	opts := &CheckOptions{
		CompatibilityProbeEnabled: true,
		ExtraHeaders: map[string]string{
			"User-Agent": "Go-http-client/2.0",
			"X-App":      "",
		},
	}
	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-opus-4-7", opts)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational when compatibility probe repairs critical Claude headers, got status=%s message=%q", res.Status, res.Message)
	}
}

func TestRunCheckForModel_AnthropicClientNotAllowedFallsBackToCompatibilityProbe(t *testing.T) {
	var attempts int
	endpoint := setupFakeMonitorProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		defer func() { _ = r.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if attempts == 1 {
			if body["stream"] != nil {
				t.Fatalf("first attempt should use default non-compat body, got stream=%v", body["stream"])
			}
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"type":    nil,
					"message": "This API is only for use in claude code, continue to use it outside claude code can cause your account banned",
				},
				"type": "error",
			})
			return
		}

		if body["stream"] != true {
			t.Fatalf("fallback attempt should use compatibility streaming body, got stream=%v", body["stream"])
		}
		if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "claude-cli/") {
			t.Fatalf("fallback should send Claude CLI User-Agent, got %q", got)
		}
		answer, err := expectedAnswerFromPrompt(anthropicPromptFromBody(body))
		if err != nil {
			t.Fatalf("extract expected answer: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: content_block_delta\ndata: {\"delta\":{\"text\":%q}}\n\n", answer)
	}))

	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-opus-4-6", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational after Claude Code rejection fallback, got status=%s message=%q", res.Status, res.Message)
	}
	if attempts != 2 {
		t.Fatalf("expected exactly 2 attempts, got %d", attempts)
	}
}

func TestRunCheckForModel_AnthropicCompatibilityProbeRetriesClaudeCodeRejectionWithFreshFingerprint(t *testing.T) {
	var attempts int
	var firstRequestID string
	var firstUserID string
	endpoint := setupFakeMonitorProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		defer func() { _ = r.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["stream"] != true {
			t.Fatalf("compatibility attempt should use streaming body, got stream=%v", body["stream"])
		}
		metadata, _ := body["metadata"].(map[string]any)
		userID, _ := metadata["user_id"].(string)
		requestID := r.Header.Get("x-client-request-id")
		if userID == "" || requestID == "" {
			t.Fatalf("compatibility attempt should include user_id and request id, user_id=%q request_id=%q", userID, requestID)
		}

		if attempts == 1 {
			firstUserID = userID
			firstRequestID = requestID
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"type":    nil,
					"message": "This API is only for use in claude code, continue to use it outside claude code can cause your account banned",
				},
				"type": "error",
			})
			return
		}

		if userID == firstUserID {
			t.Fatal("retry should regenerate metadata.user_id")
		}
		if requestID == firstRequestID {
			t.Fatal("retry should regenerate x-client-request-id")
		}
		answer, err := expectedAnswerFromPrompt(anthropicPromptFromBody(body))
		if err != nil {
			t.Fatalf("extract expected answer: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: content_block_delta\ndata: {\"delta\":{\"text\":%q}}\n\n", answer)
	}))

	opts := &CheckOptions{CompatibilityProbeEnabled: true}
	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-opus-4-6", opts)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational after retrying Claude Code rejection with fresh fingerprint, got status=%s message=%q", res.Status, res.Message)
	}
	if attempts != 2 {
		t.Fatalf("expected exactly 2 attempts, got %d", attempts)
	}
}

func TestRunCheckForModel_RetriesTransientHTTP502(t *testing.T) {
	var attempts int
	endpoint := setupFakeMonitorProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		defer func() { _ = r.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"title":"Error 502: Bad gateway"}`))
			return
		}

		answer, err := expectedAnswerFromPrompt(anthropicPromptFromBody(body))
		if err != nil {
			t.Fatalf("extract expected answer: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": answer}},
		})
	}))

	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, endpoint, "sk-fake", "claude-opus-4-6", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational after retrying transient 502, got status=%s message=%q", res.Status, res.Message)
	}
	if attempts != 2 {
		t.Fatalf("expected exactly 2 attempts, got %d", attempts)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRunCheckForModel_RetriesResponseHeaderTimeout(t *testing.T) {
	orig := monitorHTTPClient
	var attempts int
	monitorHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("http2: timeout awaiting response headers")
		}

		defer func() { _ = req.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		answer, err := expectedAnswerFromPrompt(anthropicPromptFromBody(body))
		if err != nil {
			t.Fatalf("extract expected answer: %v", err)
		}
		respBody := fmt.Sprintf(`{"content":[{"type":"text","text":%q}]}`, answer)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(respBody)),
			Request:    req,
		}, nil
	})}
	t.Cleanup(func() { monitorHTTPClient = orig })

	res := runCheckForModel(context.Background(), MonitorProviderAnthropic, "https://example.test", "sk-fake", "claude-opus-4-6", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational after retrying response header timeout, got status=%s message=%q", res.Status, res.Message)
	}
	if attempts != 2 {
		t.Fatalf("expected exactly 2 attempts, got %d", attempts)
	}
}

func TestRunCheckForModel_GeminiTextAfterThoughtBlockPassesChallenge(t *testing.T) {
	endpoint := setupFakeMonitorProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		answer, err := expectedAnswerFromPrompt(geminiPromptFromBody(body))
		if err != nil {
			t.Fatalf("extract expected answer: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"thought": true, "text": ""},
							{"text": answer},
						},
					},
				},
			},
		})
	}))

	res := runCheckForModel(context.Background(), MonitorProviderGemini, endpoint, "AIza-fake", "gemini-3.1-pro-preview", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational when Gemini text follows thought block, got status=%s message=%q", res.Status, res.Message)
	}
}

func TestRunCheckForModel_GeminiDefaultKeepsSmallChallengeBudget(t *testing.T) {
	var observedMaxOutputTokens float64
	endpoint := setupFakeMonitorProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		generationConfig, _ := body["generationConfig"].(map[string]any)
		observedMaxOutputTokens, _ = generationConfig["maxOutputTokens"].(float64)
		answer, err := expectedAnswerFromPrompt(geminiPromptFromBody(body))
		if err != nil {
			t.Fatalf("extract expected answer: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":%q}]},\"finishReason\":\"STOP\"}]}\n\n", answer)
	}))

	res := runCheckForModel(context.Background(), MonitorProviderGemini, endpoint, "AIza-fake", "gemini-3.1-pro-preview", nil)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational with default Gemini probe, got status=%s message=%q", res.Status, res.Message)
	}
	if observedMaxOutputTokens != monitorChallengeMaxTokens {
		t.Fatalf("default Gemini probe should keep maxOutputTokens=%d, got %v", monitorChallengeMaxTokens, observedMaxOutputTokens)
	}
}

func TestRunCheckForModel_GeminiSSEStreamPassesChallenge(t *testing.T) {
	endpoint := setupFakeMonitorProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ":streamGenerateContent") || r.URL.Query().Get("alt") != "sse" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"expected streamGenerateContent SSE endpoint"}}`))
			return
		}
		defer func() { _ = r.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		answer, err := expectedAnswerFromPrompt(geminiPromptFromBody(body))
		if err != nil {
			t.Fatalf("extract expected answer: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", `{"candidates":[{"content":{"parts":[{"thought":true,"text":"ignored thinking"}]}}]}`)
		fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":%q}]}}]}\n\n", answer)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))

	opts := &CheckOptions{CompatibilityProbeEnabled: true}
	res := runCheckForModel(context.Background(), MonitorProviderGemini, endpoint, "AIza-fake", "gemini-3.1-pro-preview", opts)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational with Gemini SSE response, got status=%s message=%q", res.Status, res.Message)
	}
}

func TestRunCheckForModel_GeminiAllowsEnoughOutputTokensForThinkingModels(t *testing.T) {
	endpoint := setupFakeMonitorProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		generationConfig, _ := body["generationConfig"].(map[string]any)
		maxOutputTokens, _ := generationConfig["maxOutputTokens"].(float64)
		w.Header().Set("Content-Type", "text/event-stream")
		if maxOutputTokens < 256 {
			fmt.Fprintf(w, "data: %s\n\n", `{"candidates":[{"content":{"parts":[{"thought":true,"text":"thinking used the token budget"}]},"finishReason":"MAX_TOKENS"}]}`)
			return
		}

		answer, err := expectedAnswerFromPrompt(geminiPromptFromBody(body))
		if err != nil {
			t.Fatalf("extract expected answer: %v", err)
		}
		fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":%q}]},\"finishReason\":\"STOP\"}]}\n\n", answer)
	}))

	opts := &CheckOptions{CompatibilityProbeEnabled: true}
	res := runCheckForModel(context.Background(), MonitorProviderGemini, endpoint, "AIza-fake", "gemini-3.1-pro-preview", opts)

	if res.Status != MonitorStatusOperational {
		t.Fatalf("expected operational when Gemini has enough output tokens for thinking, got status=%s message=%q", res.Status, res.Message)
	}
}
