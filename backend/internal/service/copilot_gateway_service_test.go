package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/copilot"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// newRedirectingHTTPClient returns an *http.Client backed by the test server's
// TLS config whose transport rewrites every outbound request to point at srv,
// regardless of the original Host. Used to intercept calls to canonical Copilot
// URLs (e.g. api.githubcopilot.com) in unit tests.
func newRedirectingHTTPClient(srv *httptest.Server) *http.Client {
	srvURL, _ := url.Parse(srv.URL)
	base := srv.Client().Transport
	client := srv.Client()
	client.Transport = &redirectTransport{target: srvURL, base: base}
	return client
}

type redirectTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = t.target.Scheme
	cloned.URL.Host = t.target.Host
	cloned.Host = t.target.Host
	return t.base.RoundTrip(cloned)
}

func TestCopilotInitiator(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			"first user turn – no history",
			`{"messages":[{"role":"user","content":"hi"}]}`,
			"user",
		},
		{
			"multi-turn with assistant – agent call",
			`{"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"},{"role":"user","content":"more"}]}`,
			"agent",
		},
		{
			"tool result message – agent call",
			`{"messages":[{"role":"user","content":"hi"},{"role":"tool","content":"result"}]}`,
			"agent",
		},
		{
			"system + user only",
			`{"messages":[{"role":"system","content":"you are helpful"},{"role":"user","content":"hi"}]}`,
			"user",
		},
		{
			"invalid json defaults to user",
			`{invalid`,
			"user",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := copilotInitiator([]byte(tt.body))
			if got != tt.want {
				t.Errorf("copilotInitiator() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCopilotInitiatorFromResponsesBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			"first turn plain string input",
			`{"model":"gpt-4o","input":"hello"}`,
			"user",
		},
		{
			"first turn array with only user message",
			`{"model":"gpt-4o","input":[{"role":"user","content":"hello"}]}`,
			"user",
		},
		{
			"multi-turn with assistant message – agent call",
			`{"model":"gpt-4o","input":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"},{"role":"user","content":"more"}]}`,
			"agent",
		},
		{
			"function_call item – agent call",
			`{"model":"gpt-4o","input":[{"role":"user","content":"hi"},{"type":"function_call","name":"read_file","arguments":"{}"}]}`,
			"agent",
		},
		{
			"function_call_output item – agent call",
			`{"model":"gpt-4o","input":[{"role":"user","content":"hi"},{"type":"function_call_output","output":"result"}]}`,
			"agent",
		},
		{
			"empty input array",
			`{"model":"gpt-4o","input":[]}`,
			"user",
		},
		{
			"invalid json defaults to user",
			`{invalid`,
			"user",
		},
		{
			"missing input field defaults to user",
			`{"model":"gpt-4o"}`,
			"user",
		},
		{
			"continuation via previous_response_id – agent call",
			`{"model":"gpt-4o","previous_response_id":"resp_abc123","input":"continue the task"}`,
			"agent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := copilotInitiatorFromResponsesBody([]byte(tt.body))
			if got != tt.want {
				t.Errorf("copilotInitiatorFromResponsesBody() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCopilotModelUsesMaxOutputClamp(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"", false},
		{"claude-haiku-4-5-20251001", false},
		{"claude-sonnet-4.6", true},
		{"claude-opus-4.6", true},
		{"gpt-4", false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := copilotModelUsesMaxOutputClamp(tt.model); got != tt.want {
				t.Errorf("copilotModelUsesMaxOutputClamp(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestEffectiveCopilotMaxOutputTokensCap(t *testing.T) {
	cap, clamp := effectiveCopilotMaxOutputTokensCap(nil)
	if !clamp || cap != defaultCopilotMaxOutputTokens {
		t.Fatalf("nil account: cap=%d clamp=%v", cap, clamp)
	}
	acc := &Account{Credentials: map[string]any{"copilot_max_output_tokens": json.Number("16384")}}
	cap, clamp = effectiveCopilotMaxOutputTokensCap(acc)
	if !clamp || cap != 16384 {
		t.Fatalf("custom 16384: cap=%d clamp=%v", cap, clamp)
	}
	off := &Account{Credentials: map[string]any{"copilot_max_output_tokens": 0}}
	cap, clamp = effectiveCopilotMaxOutputTokensCap(off)
	if clamp || cap != 0 {
		t.Fatalf("explicit 0: cap=%d clamp=%v", cap, clamp)
	}
}

func TestClampCopilotUpstreamMaxTokens(t *testing.T) {
	raw := `{"model":"claude-sonnet-4.6","max_tokens":32000,"messages":[{"role":"user","content":"hi"}],"stream":true}`
	out := clampCopilotUpstreamMaxTokens([]byte(raw), nil)
	var parsed struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.MaxTokens != defaultCopilotMaxOutputTokens {
		t.Fatalf("max_tokens = %d, want %d", parsed.MaxTokens, defaultCopilotMaxOutputTokens)
	}

	custom := &Account{Credentials: map[string]any{"copilot_max_output_tokens": float64(4096)}}
	out2 := clampCopilotUpstreamMaxTokens([]byte(raw), custom)
	var p2 struct {
		MaxTokens int `json:"max_tokens"`
	}
	_ = json.Unmarshal(out2, &p2)
	if p2.MaxTokens != 4096 {
		t.Fatalf("custom cap max_tokens = %d, want 4096", p2.MaxTokens)
	}

	off := &Account{Credentials: map[string]any{"copilot_max_output_tokens": 0}}
	out3 := clampCopilotUpstreamMaxTokens([]byte(raw), off)
	var p3 struct {
		MaxTokens int `json:"max_tokens"`
	}
	_ = json.Unmarshal(out3, &p3)
	if p3.MaxTokens != 32000 {
		t.Fatalf("clamp off max_tokens = %d, want 32000", p3.MaxTokens)
	}

	unchanged := clampCopilotUpstreamMaxTokens([]byte(`{"model":"claude-haiku-4.5","max_tokens":32000,"stream":true}`), nil)
	var p4 struct {
		MaxTokens int `json:"max_tokens"`
	}
	_ = json.Unmarshal(unchanged, &p4)
	if p4.MaxTokens != 32000 {
		t.Fatalf("haiku max_tokens = %d, want 32000", p4.MaxTokens)
	}
}

func TestDetectStreamMode(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"stream true", `{"model":"gpt-4","stream":true}`, true},
		{"stream false", `{"model":"gpt-4","stream":false}`, false},
		{"no stream field", `{"model":"gpt-4"}`, false},
		{"stream string", `{"model":"gpt-4","stream":"true"}`, false},
		{"stream null", `{"model":"gpt-4","stream":null}`, false},
		{"invalid json", `{invalid`, false},
		{"empty body", ``, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectStreamMode([]byte(tt.body))
			if got != tt.want {
				t.Errorf("detectStreamMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCopilotGatewayService_ParseStreamUsage(t *testing.T) {
	svc := &CopilotGatewayService{}

	t.Run("valid usage", func(t *testing.T) {
		usage := &CopilotUsage{}
		data := `{"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`
		svc.parseStreamUsage(data, usage)

		if usage.PromptTokens != 10 {
			t.Errorf("PromptTokens = %d, want 10", usage.PromptTokens)
		}
		if usage.CompletionTokens != 20 {
			t.Errorf("CompletionTokens = %d, want 20", usage.CompletionTokens)
		}
		if usage.TotalTokens != 30 {
			t.Errorf("TotalTokens = %d, want 30", usage.TotalTokens)
		}
	})

	t.Run("no usage field", func(t *testing.T) {
		usage := &CopilotUsage{}
		data := `{"choices":[{"delta":{"content":"hi"}}]}`
		svc.parseStreamUsage(data, usage)

		if usage.TotalTokens != 0 {
			t.Errorf("TotalTokens = %d, want 0", usage.TotalTokens)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		usage := &CopilotUsage{}
		svc.parseStreamUsage("{invalid}", usage)

		if usage.TotalTokens != 0 {
			t.Errorf("TotalTokens = %d, want 0", usage.TotalTokens)
		}
	})

	t.Run("updates existing usage", func(t *testing.T) {
		usage := &CopilotUsage{PromptTokens: 5, CompletionTokens: 5, TotalTokens: 10}
		data := `{"usage":{"prompt_tokens":15,"completion_tokens":25,"total_tokens":40}}`
		svc.parseStreamUsage(data, usage)

		if usage.TotalTokens != 40 {
			t.Errorf("TotalTokens = %d, want 40 (should overwrite)", usage.TotalTokens)
		}
	})
}

func TestCopilotGatewayService_ParseNonStreamUsage(t *testing.T) {
	svc := &CopilotGatewayService{}

	t.Run("valid usage", func(t *testing.T) {
		body := []byte(`{"id":"chatcmpl-xxx","choices":[],"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}}`)
		usage := svc.parseNonStreamUsage(body)

		if usage.PromptTokens != 100 {
			t.Errorf("PromptTokens = %d, want 100", usage.PromptTokens)
		}
		if usage.CompletionTokens != 50 {
			t.Errorf("CompletionTokens = %d, want 50", usage.CompletionTokens)
		}
		if usage.TotalTokens != 150 {
			t.Errorf("TotalTokens = %d, want 150", usage.TotalTokens)
		}
	})

	t.Run("no usage", func(t *testing.T) {
		body := []byte(`{"id":"chatcmpl-xxx","choices":[]}`)
		usage := svc.parseNonStreamUsage(body)

		if usage.TotalTokens != 0 {
			t.Errorf("TotalTokens = %d, want 0", usage.TotalTokens)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		usage := svc.parseNonStreamUsage([]byte(`{invalid}`))
		if usage.TotalTokens != 0 {
			t.Errorf("TotalTokens = %d, want 0", usage.TotalTokens)
		}
	})
}

func TestCopilotGatewayService_HandleNonStreamingResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &CopilotGatewayService{}

	t.Run("success", func(t *testing.T) {
		respBody := `{"id":"chatcmpl-xxx","choices":[{"message":{"content":"hello"}}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`

		// Create mock upstream response
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"X-Request-Id": {"req-123"}},
			Body:       copilotStringReadCloser(respBody),
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		result, err := svc.handleNonStreamingResponse(c, resp, "gpt-4o", "gpt-4o", time.Now())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.StatusCode != http.StatusOK {
			t.Errorf("StatusCode = %d, want %d", result.StatusCode, http.StatusOK)
		}
		if result.Model != "gpt-4o" {
			t.Errorf("Model = %q, want %q", result.Model, "gpt-4o")
		}
		if result.Usage == nil {
			t.Fatal("Usage should not be nil")
		}
		if result.Usage.TotalTokens != 8 {
			t.Errorf("TotalTokens = %d, want 8", result.Usage.TotalTokens)
		}

		// Check response was forwarded to client
		if w.Code != http.StatusOK {
			t.Errorf("response code = %d, want %d", w.Code, http.StatusOK)
		}
		if !strings.Contains(w.Body.String(), "chatcmpl-xxx") {
			t.Errorf("response body should contain chatcmpl-xxx")
		}
	})
}

func TestCopilotGatewayService_HandleErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("401 invalidates token", func(t *testing.T) {
		provider := NewCopilotTokenProvider()
		// Pre-populate token cache
		provider.tokens[42] = nil // just to verify it gets deleted

		svc := &CopilotGatewayService{tokenProvider: provider}

		errBody := `{"error":{"message":"Unauthorized","type":"auth_error"}}`
		resp := &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       copilotStringReadCloser(errBody),
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		account := &Account{ID: 42, Platform: PlatformCopilot}
		result, err := svc.handleErrorResponse(c, resp, account)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.StatusCode != http.StatusUnauthorized {
			t.Errorf("StatusCode = %d, want %d", result.StatusCode, http.StatusUnauthorized)
		}

		// Verify token was invalidated
		provider.mu.RLock()
		_, exists := provider.tokens[42]
		provider.mu.RUnlock()
		if exists {
			t.Error("token should have been invalidated on 401")
		}
	})

	t.Run("429 signals failover without writing to client", func(t *testing.T) {
		svc := &CopilotGatewayService{tokenProvider: NewCopilotTokenProvider()}

		errBody := `{"error":{"message":"Rate limited"}}`
		resp := &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       copilotStringReadCloser(errBody),
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		account := &Account{ID: 1, Platform: PlatformCopilot}
		result, err := svc.handleErrorResponse(c, resp, account)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 429 must NOT be written to the client — handler loop needs to failover.
		if result.StatusCode != http.StatusTooManyRequests {
			t.Errorf("StatusCode = %d, want %d", result.StatusCode, http.StatusTooManyRequests)
		}
		// Default recorder code is 200 (nothing written).
		if w.Code != http.StatusOK {
			t.Errorf("response code = %d, want 200 (no write to client for 429)", w.Code)
		}
		if w.Body.Len() != 0 {
			t.Errorf("response body should be empty for 429, got: %s", w.Body.String())
		}
	})
}

func TestCopilotGatewayService_HandleStreamingResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &CopilotGatewayService{}

	t.Run("streams SSE lines", func(t *testing.T) {
		// Build a mock SSE response
		sseLines := []string{
			"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n",
			"\n",
			"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":2,\"total_tokens\":7}}\n",
			"\n",
			"data: [DONE]\n",
			"\n",
		}
		sseBody := strings.Join(sseLines, "")

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       copilotStringReadCloser(sseBody),
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

		result, err := svc.handleStreamingResponse(c, resp, "gpt-4o", "gpt-4o", time.Now(), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.StatusCode != http.StatusOK {
			t.Errorf("StatusCode = %d, want %d", result.StatusCode, http.StatusOK)
		}
		if result.Model != "gpt-4o" {
			t.Errorf("Model = %q, want %q", result.Model, "gpt-4o")
		}
		if result.Usage == nil {
			t.Fatal("Usage should not be nil")
		}
		if result.Usage.TotalTokens != 7 {
			t.Errorf("TotalTokens = %d, want 7", result.Usage.TotalTokens)
		}

		// Verify SSE content was forwarded
		body := w.Body.String()
		if !strings.Contains(body, "Hello") {
			t.Error("response should contain 'Hello'")
		}
		if !strings.Contains(body, "[DONE]") {
			t.Error("response should contain '[DONE]'")
		}
	})
}

func TestCopilotGatewayService_ListModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		modelsResp := `{"data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/models" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			auth := r.Header.Get("Authorization")
			if auth != "Bearer copilot-token-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, modelsResp)
		}))
		defer server.Close()

		provider := NewCopilotTokenProvider()
		svc := NewCopilotGatewayService(provider)

		// Pre-populate token so no exchange is needed
		tok := newCopilotTestToken("copilot-token-123")
		provider.mu.Lock()
		provider.tokens[1] = &tok
		provider.mu.Unlock()

		account := &Account{
			ID:       1,
			Platform: PlatformCopilot,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"github_token": "ghp_test",
				"base_url":     server.URL,
			},
		}

		body, err := svc.ListModels(t.Context(), account)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(string(body), "gpt-4o") {
			t.Errorf("response should contain model list, got: %s", string(body))
		}
	})

	t.Run("upstream error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":"internal"}`)
		}))
		defer server.Close()

		provider := NewCopilotTokenProvider()
		svc := NewCopilotGatewayService(provider)

		tok := newCopilotTestToken("tok")
		provider.mu.Lock()
		provider.tokens[2] = &tok
		provider.mu.Unlock()

		account := &Account{
			ID:       2,
			Platform: PlatformCopilot,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"github_token": "ghp_test",
				"base_url":     server.URL,
			},
		}

		_, err := svc.ListModels(t.Context(), account)
		if err == nil {
			t.Fatal("expected error for 500 response")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("error should mention status code, got: %v", err)
		}
	})
}

// ── OpenAI body merge ─────────────────────────────────────────────────────────

func TestMergeConsecutiveSameRoleMessagesInOpenAIBody(t *testing.T) {
	t.Run("consecutive user messages merged", func(t *testing.T) {
		body := `{"model":"claude-sonnet-4.6","stream":true,"messages":[
			{"role":"user","content":"<available-deferred-tools>\nAgent\n</available-deferred-tools>"},
			{"role":"user","content":"hello world"}
		]}`
		got := mergeConsecutiveSameRoleMessagesInOpenAIBody([]byte(body))

		var result struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(got, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if result.Model != "claude-sonnet-4.6" {
			t.Errorf("model = %q, want claude-sonnet-4.6 (other fields preserved)", result.Model)
		}
		userCount := 0
		for _, m := range result.Messages {
			if m.Role == "user" {
				userCount++
			}
		}
		if userCount != 1 {
			t.Errorf("expected 1 merged user message, got %d: %v", userCount, result.Messages)
		}
		if len(result.Messages) > 0 && !strings.Contains(result.Messages[0].Content, "available-deferred-tools") {
			t.Errorf("merged content missing first part: %q", result.Messages[0].Content)
		}
		if len(result.Messages) > 0 && !strings.Contains(result.Messages[0].Content, "hello world") {
			t.Errorf("merged content missing second part: %q", result.Messages[0].Content)
		}
	})

	t.Run("alternating roles not merged", func(t *testing.T) {
		body := `{"model":"gpt-4o","messages":[
			{"role":"user","content":"hi"},
			{"role":"assistant","content":"hello"},
			{"role":"user","content":"again"}
		]}`
		got := mergeConsecutiveSameRoleMessagesInOpenAIBody([]byte(body))

		var result struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(got, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(result.Messages) != 3 {
			t.Errorf("expected 3 messages (no merge), got %d", len(result.Messages))
		}
	})

	t.Run("content parts array merged as text", func(t *testing.T) {
		body := `{"model":"gpt-4o","messages":[
			{"role":"user","content":[{"type":"text","text":"part one"}]},
			{"role":"user","content":"part two"}
		]}`
		got := mergeConsecutiveSameRoleMessagesInOpenAIBody([]byte(body))

		var result struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(got, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(result.Messages) != 1 {
			t.Errorf("expected 1 merged message, got %d", len(result.Messages))
		}
		if !strings.Contains(result.Messages[0].Content, "part one") ||
			!strings.Contains(result.Messages[0].Content, "part two") {
			t.Errorf("merged content = %q, want both parts", result.Messages[0].Content)
		}
	})

	t.Run("invalid json returned unchanged", func(t *testing.T) {
		body := []byte(`{invalid json`)
		got := mergeConsecutiveSameRoleMessagesInOpenAIBody(body)
		if string(got) != string(body) {
			t.Errorf("expected original body returned on parse error")
		}
	})

	t.Run("other fields preserved", func(t *testing.T) {
		body := `{"model":"gpt-4o","stream":true,"temperature":0.7,"messages":[
			{"role":"user","content":"hi"}
		]}`
		got := mergeConsecutiveSameRoleMessagesInOpenAIBody([]byte(body))

		var result struct {
			Model       string  `json:"model"`
			Stream      bool    `json:"stream"`
			Temperature float64 `json:"temperature"`
		}
		if err := json.Unmarshal(got, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if result.Model != "gpt-4o" || !result.Stream || result.Temperature != 0.7 {
			t.Errorf("fields not preserved: %+v", result)
		}
	})
}

// ── helpers ──────────────────────────────────────────────────────────

// copilotStringReadCloser wraps a string as io.ReadCloser for http.Response.Body.
func copilotStringReadCloser(s string) *copilotTestReadCloser {
	return &copilotTestReadCloser{Reader: strings.NewReader(s)}
}

type copilotTestReadCloser struct {
	Reader *strings.Reader
}

func (rc *copilotTestReadCloser) Read(p []byte) (int, error) { return rc.Reader.Read(p) }
func (rc *copilotTestReadCloser) Close() error               { return nil }

// newCopilotTestToken returns a copilot.CopilotToken that won't expire during tests.
func newCopilotTestToken(token string) copilot.CopilotToken {
	return copilot.CopilotToken{
		Token:     token,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		RefreshAt: time.Now().Add(5 * time.Minute),
	}
}

// ── P1-B: containsImageBlock ──────────────────────────────────────────────────

func TestContainsImageBlock(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "no messages",
			body: `{"model":"claude-sonnet-4","messages":[]}`,
			want: false,
		},
		{
			name: "plain text message",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hello"}]}`,
			want: false,
		},
		{
			name: "text block only",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`,
			want: false,
		},
		{
			name: "single image block",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":[
				{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc="}}
			]}]}`,
			want: true,
		},
		{
			name: "text + image block",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":[
				{"type":"text","text":"look at this"},
				{"type":"image","source":{"type":"base64","media_type":"image/jpeg","data":"xyz="}}
			]}]}`,
			want: true,
		},
		{
			name: "image in second message",
			body: `{"model":"claude-sonnet-4","messages":[
				{"role":"user","content":"first"},
				{"role":"user","content":[
					{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc="}}
				]}
			]}`,
			want: true,
		},
		{
			name: "tool_result only — no direct image block",
			body: `{"model":"claude-sonnet-4","messages":[{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"call_1","content":"result text"}
			]}]}`,
			want: false,
		},
		{
			name: "invalid json",
			body: `not-json`,
			want: false,
		},
		{
			name: "multi-turn no image",
			body: `{"model":"claude-sonnet-4","messages":[
				{"role":"user","content":"hello"},
				{"role":"assistant","content":"hi there"},
				{"role":"user","content":"how are you?"}
			]}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsImageBlock([]byte(tt.body))
			if got != tt.want {
				t.Errorf("containsImageBlock() = %v, want %v\nbody: %s", got, tt.want, tt.body)
			}
		})
	}
}

func TestEnsureStreamIncludeUsage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantVal string // gjson raw of stream_options.include_usage; "" = absent
	}{
		{"no stream_options, stream true",
			`{"model":"gpt-4o","stream":true}`, "true"},
		{"stream_options empty",
			`{"model":"gpt-4o","stream":true,"stream_options":{}}`, "true"},
		{"include_usage already true",
			`{"model":"gpt-4o","stream":true,"stream_options":{"include_usage":true}}`, "true"},
		{"include_usage false → set true",
			`{"model":"gpt-4o","stream":true,"stream_options":{"include_usage":false}}`, "true"},
		{"stream false → no injection",
			`{"model":"gpt-4o","stream":false}`, ""},
		{"stream absent → no injection",
			`{"model":"gpt-4o"}`, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ensureStreamIncludeUsage([]byte(tc.input))
			val := gjson.GetBytes(got, "stream_options.include_usage")
			if tc.wantVal == "" {
				if val.Exists() {
					t.Errorf("want absent, got %s", val.Raw)
				}
				return
			}
			if !val.Exists() || val.Raw != tc.wantVal {
				t.Errorf("want include_usage=%s, got %q; body=%s", tc.wantVal, val.Raw, got)
			}
		})
	}
}

func TestIsUsageOnlyChunk(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{"usage chunk, no choices",
			`{"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`, true},
		{"usage chunk, empty choices array",
			`{"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5}}`, true},
		{"content chunk with choices",
			`{"choices":[{"delta":{"content":"hi"}}]}`, false},
		{"content chunk with both choices and usage",
			`{"choices":[{"delta":{"content":"hi"}}],"usage":{"prompt_tokens":10}}`, false},
		{"empty object",
			`{}`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUsageOnlyChunk(tc.data); got != tc.want {
				t.Errorf("isUsageOnlyChunk(%q) = %v, want %v", tc.data, got, tc.want)
			}
		})
	}
}

// ── Task 2: ForwardChatCompletions usage chunk filtering ─────────────────────

func TestForwardChatCompletions_UsageChunkFilteredWhenClientDidNotRequest(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		// Copilot-injected usage-only chunk.
		fmt.Fprint(w, "data: {\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":3,\"total_tokens\":11}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	provider := NewCopilotTokenProvider()
	tok := newCopilotTestToken("copilot-token-xyz")
	provider.tokens[1] = &tok

	svc := NewCopilotGatewayService(provider)
	svc.httpClient = srv.Client()

	account := &Account{
		ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"github_token": "ghp_test", "base_url": srv.URL},
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/copilot/v1/chat/completions", nil)

	// Client sends stream=true WITHOUT stream_options.include_usage.
	clientBody := []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	result, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
	if err != nil {
		t.Fatalf("ForwardChatCompletions: %v", err)
	}

	// Root-cause check: upstream must have include_usage injected.
	if !gjson.GetBytes(capturedBody, "stream_options.include_usage").Bool() {
		t.Errorf("want stream_options.include_usage=true in upstream body; got %s", capturedBody)
	}
	// Token counts must be non-zero (parsed from usage chunk internally).
	if result.Usage == nil || result.Usage.PromptTokens == 0 {
		t.Errorf("want non-zero PromptTokens; got %+v", result.Usage)
	}
	// Client must NOT receive the usage-only chunk.
	respBody := w.Body.String()
	if strings.Contains(respBody, `"prompt_tokens"`) {
		t.Errorf("usage-only chunk must be filtered from client response; got: %s", respBody)
	}
	if !strings.Contains(respBody, "hello") {
		t.Errorf("content chunk must be forwarded to client; got: %s", respBody)
	}
}

func TestForwardChatCompletions_UsageChunkForwardedWhenClientRequested(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":2,\"total_tokens\":7}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	provider := NewCopilotTokenProvider()
	tok := newCopilotTestToken("copilot-token-xyz2")
	provider.tokens[1] = &tok

	svc := NewCopilotGatewayService(provider)
	svc.httpClient = srv.Client()

	account := &Account{
		ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"github_token": "ghp_test", "base_url": srv.URL},
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/copilot/v1/chat/completions", nil)

	// Client explicitly requests include_usage — usage chunk must be forwarded.
	clientBody := []byte(`{"model":"gpt-4o","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`)
	result, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
	if err != nil {
		t.Fatalf("ForwardChatCompletions: %v", err)
	}
	if result.Usage == nil || result.Usage.PromptTokens == 0 {
		t.Errorf("want non-zero PromptTokens; got %+v", result.Usage)
	}
	// Client SHOULD receive the usage chunk.
	respBody := w.Body.String()
	if !strings.Contains(respBody, `"prompt_tokens"`) {
		t.Errorf("want usage chunk forwarded to client; got: %s", respBody)
	}
}

// ── Task 3: ForwardMessages injects stream_options.include_usage ─────────────

func TestForwardMessages_UpstreamBodyHasStreamIncludeUsage(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Minimal OpenAI SSE chat/completions stream.
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":3,\"total_tokens\":13}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	provider := NewCopilotTokenProvider()
	tok := newCopilotTestToken("copilot-token-abc")
	provider.tokens[1] = &tok

	svc := NewCopilotGatewayService(provider)
	svc.httpClient = srv.Client()

	account := &Account{
		ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"github_token": "ghp_test", "base_url": srv.URL},
	}

	// Force /chat/completions route for "gpt-4o" (no model mapping on this account).
	// Endpoint value must be "/chat/completions" (with leading slash) to match shouldUseResponsesEndpoint.
	svc.setModelEndpointsCache(account.ID, map[string][]string{
		"gpt-4o": {"/chat/completions"},
	}, false)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	anthropicBody := []byte(`{"model":"gpt-4o","stream":false,"max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`)
	result, err := svc.ForwardMessages(context.Background(), c, account, anthropicBody)
	if err != nil {
		t.Fatalf("ForwardMessages: %v", err)
	}

	// Upstream body: stream=true (from forceStreamTrue) AND include_usage=true.
	if !gjson.GetBytes(capturedBody, "stream").Bool() {
		t.Errorf("want stream=true in upstream body; got %s", capturedBody)
	}
	if !gjson.GetBytes(capturedBody, "stream_options.include_usage").Bool() {
		t.Errorf("want stream_options.include_usage=true in upstream body; got %s", capturedBody)
	}
	// Token counts must be non-zero.
	if result == nil || result.Usage == nil || result.Usage.PromptTokens == 0 {
		t.Errorf("want non-zero PromptTokens; result=%+v", result)
	}
}

func TestStripUnsupportedContentPartsFromOpenAIBody(t *testing.T) {
	t.Run("file parts stripped from user message array", func(t *testing.T) {
		// Cherry Studio 发送带 PDF 文件的请求，Copilot 不支持 type="file"，应被剥除。
		body := `{"model":"gpt-4o","stream":true,"messages":[
			{"role":"user","content":[
				{"type":"text","text":"请分析这个PDF"},
				{"type":"file","file":{"filename":"report.pdf","file_data":"data:application/pdf;base64,JVBERi0x"}}
			]}
		]}`

		got, hasFile := StripUnsupportedContentPartsFromOpenAIBody([]byte(body))

		if !hasFile {
			t.Error("expected hasFile=true when file parts are present")
		}

		var result struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(got, &result); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}

		require := func(cond bool, msg string) {
			t.Helper()
			if !cond {
				t.Error(msg)
			}
		}

		require(len(result.Messages) == 1, "should have 1 message")

		// Content should be an array with only the text part remaining.
		var parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(result.Messages[0].Content, &parts); err != nil {
			t.Fatalf("unmarshal content parts: %v", err)
		}
		require(len(parts) == 1, "should have 1 part after file stripped")
		require(parts[0].Type == "text", "remaining part should be text")
		require(parts[0].Text == "请分析这个PDF", "text content should be preserved")
	})

	t.Run("no file parts returns unchanged body with hasFile=false", func(t *testing.T) {
		body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`
		got, hasFile := StripUnsupportedContentPartsFromOpenAIBody([]byte(body))
		if hasFile {
			t.Error("expected hasFile=false when no file parts")
		}
		if string(got) != body {
			t.Errorf("body should be unchanged when no file parts: got %s", got)
		}
	})

	t.Run("message with only file parts gets empty content array", func(t *testing.T) {
		// 消息只有 file part 没有 text，剥除后 content 为空数组。
		body := `{"model":"gpt-4o","messages":[
			{"role":"user","content":[
				{"type":"file","file":{"filename":"doc.pdf","file_data":"data:application/pdf;base64,abc"}}
			]}
		]}`
		got, hasFile := StripUnsupportedContentPartsFromOpenAIBody([]byte(body))
		if !hasFile {
			t.Error("expected hasFile=true")
		}
		var result struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(got, &result); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var parts []json.RawMessage
		if err := json.Unmarshal(result.Messages[0].Content, &parts); err != nil {
			t.Fatalf("unmarshal parts: %v", err)
		}
		if len(parts) != 0 {
			t.Errorf("expected empty content array, got %d parts", len(parts))
		}
	})

	t.Run("invalid json returned unchanged with hasFile=false", func(t *testing.T) {
		body := []byte(`{invalid}`)
		got, hasFile := StripUnsupportedContentPartsFromOpenAIBody(body)
		if hasFile {
			t.Error("expected hasFile=false on parse error")
		}
		if string(got) != string(body) {
			t.Error("expected original body on parse error")
		}
	})

	t.Run("other fields preserved", func(t *testing.T) {
		body := `{"model":"gpt-4o","temperature":0.7,"stream":true,"messages":[
			{"role":"user","content":[
				{"type":"text","text":"hi"},
				{"type":"file","file":{"filename":"x.pdf","file_data":"data:application/pdf;base64,abc"}}
			]}
		]}`
		got, _ := StripUnsupportedContentPartsFromOpenAIBody([]byte(body))

		if !strings.Contains(string(got), `"temperature":0.7`) {
			t.Error("temperature field should be preserved")
		}
		if !strings.Contains(string(got), `"stream":true`) {
			t.Error("stream field should be preserved")
		}
	})
}

// ── ForwardChatCompletions: file parts routed via /responses ─────────────────

// TestForwardChatCompletions_FilePartsViaResponsesStreaming verifies that when a
// Chat Completions request contains file content parts, ForwardChatCompletions
// bridges through Copilot /responses and translates the response back to Chat
// Completions SSE, so the client receives the file content.
func TestForwardChatCompletions_FilePartsViaResponsesStreaming(t *testing.T) {
	var capturedPath string
	var capturedBody []byte

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"data":[{"id":"gpt-4o","supported_endpoints":["/responses"]}]}`)
			return
		}

		capturedPath = r.URL.Path
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Minimal Responses API SSE stream with text output.
		fmt.Fprint(w, `data: {"type":"response.created","response":{"id":"resp_abc","status":"in_progress","model":"gpt-4o","output":[]}}`, "\n\n")
		fmt.Fprint(w, `data: {"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"PDF 摘要内容"}`, "\n\n")
		fmt.Fprint(w, `data: {"type":"response.completed","response":{"id":"resp_abc","status":"completed","model":"gpt-4o","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"PDF 摘要内容"}]}],"usage":{"input_tokens":50,"output_tokens":20}}}`, "\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	provider := NewCopilotTokenProvider()
	tok := newCopilotTestToken("copilot-token-file")
	provider.tokens[1] = &tok

	svc := NewCopilotGatewayService(provider)
	// Redirect requests to api.githubcopilot.com → test server via custom transport.
	svc.httpClient = newRedirectingHTTPClient(srv)

	account := &Account{
		ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"github_token": "ghp_test"},
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/copilot/v1/chat/completions", nil)

	clientBody := []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":[{"type":"text","text":"总结这个PDF"},{"type":"file","file":{"filename":"report.pdf","file_data":"data:application/pdf;base64,JVBERi0x"}}]}]}`)
	result, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
	if err != nil {
		t.Fatalf("ForwardChatCompletions: %v", err)
	}

	// Upstream must have been called at /responses, not /chat/completions.
	if capturedPath != "/responses" {
		t.Fatalf("expected upstream path /responses, got %q", capturedPath)
	}

	// Upstream request must be Responses API format (has "input", not "messages").
	if !gjson.GetBytes(capturedBody, "input").Exists() {
		t.Fatalf("expected Responses API format with 'input' field; got: %s", capturedBody)
	}
	// File part must have been converted to input_file.
	inputContent := gjson.GetBytes(capturedBody, "input.0.content")
	if !inputContent.Exists() {
		t.Fatalf("expected input[0].content in upstream body; got: %s", capturedBody)
	}
	var foundInputFile bool
	for _, part := range inputContent.Array() {
		if part.Get("type").String() == "input_file" {
			foundInputFile = true
		}
	}
	if !foundInputFile {
		t.Fatalf("expected input_file part in upstream body; got content: %s", inputContent.Raw)
	}

	// Client response must be Chat Completions SSE.
	respBody := w.Body.String()
	if !strings.Contains(respBody, "PDF 摘要内容") {
		t.Errorf("expected response text in Chat Completions SSE; got: %s", respBody)
	}
	if !strings.Contains(respBody, "data:") {
		t.Errorf("expected SSE format with 'data:' prefix; got: %s", respBody)
	}
	if !strings.Contains(respBody, "[DONE]") {
		t.Errorf("expected [DONE] in SSE stream; got: %s", respBody)
	}

	// Usage must be captured.
	if result == nil || result.Usage == nil {
		t.Fatalf("expected non-nil usage; result=%+v", result)
	}
	if result.Usage.PromptTokens != 50 || result.Usage.CompletionTokens != 20 {
		t.Errorf("expected usage prompt=50 completion=20; got %+v", result.Usage)
	}
}

// TestForwardChatCompletions_FilePartsViaResponsesNonStreaming verifies the
// non-streaming path: file request is bridged via /responses and the response
// is returned as a single Chat Completions JSON object.
func TestForwardChatCompletions_FilePartsViaResponsesNonStreaming(t *testing.T) {
	var capturedPath string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"data":[{"id":"gpt-4o","supported_endpoints":["/responses"]}]}`)
			return
		}

		capturedPath = r.URL.Path
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `data: {"type":"response.completed","response":{"id":"resp_xyz","status":"completed","model":"gpt-4o","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"文件分析结果"}]}],"usage":{"input_tokens":30,"output_tokens":10}}}`, "\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	provider := NewCopilotTokenProvider()
	tok := newCopilotTestToken("copilot-token-nonstream")
	provider.tokens[1] = &tok

	svc := NewCopilotGatewayService(provider)
	svc.httpClient = newRedirectingHTTPClient(srv)

	account := &Account{
		ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"github_token": "ghp_test"},
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/copilot/v1/chat/completions", nil)

	// stream: false
	clientBody := []byte(`{"model":"gpt-4o","stream":false,"messages":[{"role":"user","content":[{"type":"text","text":"分析文件"},{"type":"file","file":{"filename":"doc.pdf","file_data":"data:application/pdf;base64,JVBERi0x"}}]}]}`)
	result, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
	if err != nil {
		t.Fatalf("ForwardChatCompletions: %v", err)
	}

	if capturedPath != "/responses" {
		t.Fatalf("expected upstream path /responses, got %q", capturedPath)
	}

	// Response must be valid Chat Completions JSON (not SSE).
	respBody := w.Body.Bytes()
	if w.Result().Header.Get("Content-Type") == "text/event-stream" {
		t.Fatal("expected JSON response for non-streaming request, got SSE content-type")
	}
	if !gjson.GetBytes(respBody, "choices.0.message.content").Exists() {
		t.Fatalf("expected choices[0].message.content in response; got: %s", respBody)
	}
	if got := gjson.GetBytes(respBody, "choices.0.message.content").String(); got != "文件分析结果" {
		t.Errorf("expected '文件分析结果', got %q; body: %s", got, respBody)
	}

	if result == nil || result.Usage == nil || result.Usage.PromptTokens != 30 {
		t.Errorf("expected usage prompt=30; result=%+v", result)
	}
}

// TestForwardChatCompletions_UnsupportedAPIForModel_FallbackToChatCompletions
// verifies that when the upstream returns HTTP 400 with
// error.code == "unsupported_api_for_model" (e.g. claude-opus-4.6 does not
// support the Responses API), ForwardChatCompletions falls back to the
// standard /chat/completions path using the stripped body (file parts removed).
func TestForwardChatCompletions_UnsupportedAPIForModel_FallbackToChatCompletions(t *testing.T) {
	var requestPaths []string
	var sawModelsRequest bool

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			sawModelsRequest = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"data":[{"id":"claude-opus-4.6","supported_endpoints":["/responses"]}]}`)
			return
		}

		requestPaths = append(requestPaths, r.URL.Path)
		_, _ = io.ReadAll(r.Body)

		if r.URL.Path == "/responses" {
			// Simulate Copilot rejecting the model for the Responses API.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"error":{"message":"model claude-opus-4.6 does not support Responses API.","code":"unsupported_api_for_model"}}`)
			return
		}
		// Fallback /chat/completions responds with a minimal streaming reply.
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"文档摘要\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: {\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":10}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	provider := NewCopilotTokenProvider()
	tok := newCopilotTestToken("copilot-token-opus-fallback")
	provider.tokens[1] = &tok

	svc := NewCopilotGatewayService(provider)
	svc.httpClient = newRedirectingHTTPClient(srv)

	account := &Account{
		ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"github_token": "ghp_test"},
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/copilot/v1/chat/completions", nil)

	// Request with a PDF file attachment using claude-opus-4.6.
	clientBody := []byte(`{"model":"claude-opus-4.6","stream":true,"messages":[{"role":"user","content":[{"type":"text","text":"总结这个PDF"},{"type":"file","file":{"filename":"report.pdf","file_data":"data:application/pdf;base64,JVBERi0x"}}]}]}`)
	result, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
	if err != nil {
		t.Fatalf("ForwardChatCompletions: %v", err)
	}

	if !sawModelsRequest {
		t.Fatal("expected /models probe before upstream path selection")
	}
	// Must have tried /responses first, then fallen back to /chat/completions.
	if len(requestPaths) < 2 {
		t.Fatalf("expected 2 upstream requests (/responses then /chat/completions), got: %v", requestPaths)
	}
	if requestPaths[0] != "/responses" {
		t.Errorf("first request should be /responses, got %q", requestPaths[0])
	}
	if requestPaths[1] != "/chat/completions" {
		t.Errorf("second request (fallback) should be /chat/completions, got %q", requestPaths[1])
	}

	// Client should receive a successful streaming response.
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d; body: %s", w.Code, w.Body.String())
	}
	respBody := w.Body.String()
	if !strings.Contains(respBody, "文档摘要") {
		t.Errorf("expected response text in SSE stream; got: %s", respBody)
	}
	if !strings.Contains(respBody, "[DONE]") {
		t.Errorf("expected [DONE] in SSE stream; got: %s", respBody)
	}

	if result == nil || result.Usage == nil {
		t.Fatalf("expected non-nil usage; result=%+v", result)
	}
	if result.Usage.PromptTokens != 100 || result.Usage.CompletionTokens != 10 {
		t.Errorf("expected usage prompt=100 completion=10; got %+v", result.Usage)
	}
}

// TestForwardChatCompletions_NonUnsupportedAPI400_NotFallback verifies that a
// 400 error with a different code (e.g. a validation error) is NOT treated as
// the unsupported-API fallback — it is forwarded to the client as-is.
func TestForwardChatCompletions_NonUnsupportedAPI400_NotFallback(t *testing.T) {
	var requestPaths []string
	var sawModelsRequest bool

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			sawModelsRequest = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"data":[{"id":"gpt-4o","supported_endpoints":["/responses"]}]}`)
			return
		}

		requestPaths = append(requestPaths, r.URL.Path)
		_, _ = io.ReadAll(r.Body)

		// /responses returns a different 400 error (not unsupported_api_for_model).
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"error":{"message":"invalid request format","code":"invalid_request_error"}}`)
	}))
	defer srv.Close()

	provider := NewCopilotTokenProvider()
	tok := newCopilotTestToken("copilot-token-400-nofallback")
	provider.tokens[1] = &tok

	svc := NewCopilotGatewayService(provider)
	svc.httpClient = newRedirectingHTTPClient(srv)

	account := &Account{
		ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"github_token": "ghp_test"},
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/copilot/v1/chat/completions", nil)

	clientBody := []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":[{"type":"text","text":"test"},{"type":"file","file":{"filename":"f.pdf","file_data":"data:application/pdf;base64,JVBERi0x"}}]}]}`)
	result, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
	if err != nil {
		t.Fatalf("ForwardChatCompletions: %v", err)
	}

	if !sawModelsRequest {
		t.Fatal("expected /models probe before upstream path selection")
	}
	// Only one upstream request — no fallback should happen.
	if len(requestPaths) != 1 {
		t.Errorf("expected exactly 1 upstream request (no fallback), got: %v", requestPaths)
	}
	if requestPaths[0] != "/responses" {
		t.Errorf("expected /responses, got %q", requestPaths[0])
	}

	// The 400 should be forwarded to the client.
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 forwarded to client, got %d", w.Code)
	}
	if result == nil || result.StatusCode != http.StatusBadRequest {
		t.Errorf("expected result.StatusCode=400, got %+v", result)
	}
}

func TestForwardChatCompletions_NoFilePartsUsesDirectPath(t *testing.T) {
	var capturedPath string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: {\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":1}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	provider := NewCopilotTokenProvider()
	tok := newCopilotTestToken("copilot-token-direct")
	provider.tokens[1] = &tok

	svc := NewCopilotGatewayService(provider)
	svc.httpClient = srv.Client()

	account := &Account{
		ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"github_token": "ghp_test", "base_url": srv.URL},
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/copilot/v1/chat/completions", nil)

	// Plain text only, no file parts.
	clientBody := []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hello"}]}`)
	_, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
	if err != nil {
		t.Fatalf("ForwardChatCompletions: %v", err)
	}

	if capturedPath != "/chat/completions" {
		t.Fatalf("expected upstream path /chat/completions for no-file request, got %q", capturedPath)
	}
}

// =============================================================================
// X-Initiator header billing guard tests
//
// These tests lock the Copilot billing behavior at the HTTP layer.
// X-Initiator controls which quota bucket each request draws from:
//   - "user"  → Premium Interactions (paid, counts against monthly limit)
//   - "agent" → Standard quota (free, for multi-turn / tool-call sub-requests)
//
// Any code change that breaks these tests risks silently burning the upstream
// account's Premium quota on every tool-call sub-request in a Codex/CC task.
// =============================================================================

// TestXInitiatorHeader_ChatCompletions verifies that ForwardChatCompletions sets
// the correct X-Initiator header on the upstream request:
//   - First turn (messages has only system/user roles) → "user"
//   - Multi-turn (messages contains an assistant message) → "agent"
//   - Tool result (messages contains a tool role) → "agent"
func TestXInitiatorHeader_ChatCompletions(t *testing.T) {
	cases := []struct {
		name          string
		body          string
		wantInitiator string
	}{
		{
			name:          "first turn – user only → Premium quota",
			body:          `{"model":"gpt-4o","stream":false,"messages":[{"role":"user","content":"hello"}]}`,
			wantInitiator: "user",
		},
		{
			name:          "multi-turn with assistant → Standard quota (free)",
			body:          `{"model":"gpt-4o","stream":false,"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"},{"role":"user","content":"more"}]}`,
			wantInitiator: "agent",
		},
		{
			name:          "tool result sub-request → Standard quota (free)",
			body:          `{"model":"gpt-4o","stream":false,"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"ok","tool_calls":[{"id":"c1","type":"function","function":{"name":"f","arguments":"{}"}}]},{"role":"tool","tool_call_id":"c1","content":"result"}]}`,
			wantInitiator: "agent",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedInitiator string
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedInitiator = r.Header.Get("X-Initiator")
				_, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"},\"finish_reason\":\"stop\"}]}\n\n")
				fmt.Fprint(w, "data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\n")
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer srv.Close()

			provider := NewCopilotTokenProvider()
			tok := newCopilotTestToken("tok-chat-" + tc.name)
			provider.tokens[1] = &tok

			svc := NewCopilotGatewayService(provider)
			svc.httpClient = newRedirectingHTTPClient(srv)

			// Use account with custom base_url pointing to test server so the
			// direct /chat/completions path is used (no /models probe needed).
			account := &Account{
				ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"github_token": "ghp_test", "base_url": srv.URL},
			}

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/copilot/v1/chat/completions", nil)

			_, err := svc.ForwardChatCompletions(context.Background(), c, account, []byte(tc.body))
			if err != nil {
				t.Fatalf("ForwardChatCompletions: %v", err)
			}

			if capturedInitiator != tc.wantInitiator {
				t.Errorf("X-Initiator = %q, want %q\n(wrong value burns upstream Premium quota on every sub-request)", capturedInitiator, tc.wantInitiator)
			}
		})
	}
}

// TestXInitiatorHeader_ResponsesEndpoint verifies that ForwardResponses sets the
// correct X-Initiator header for the OpenAI Responses API (used by Codex CLI).
// This is the path most likely to be abused: a Codex task fires dozens of
// tool-call sub-requests — each must be "agent", not "user".
func TestXInitiatorHeader_ResponsesEndpoint(t *testing.T) {
	cases := []struct {
		name          string
		body          string
		wantInitiator string
	}{
		{
			name:          "first turn plain string → Premium quota",
			body:          `{"model":"gpt-4o","input":"hello"}`,
			wantInitiator: "user",
		},
		{
			name:          "first turn array user only → Premium quota",
			body:          `{"model":"gpt-4o","input":[{"role":"user","content":"hello"}]}`,
			wantInitiator: "user",
		},
		{
			name:          "multi-turn with assistant → Standard quota (free)",
			body:          `{"model":"gpt-4o","input":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"},{"role":"user","content":"more"}]}`,
			wantInitiator: "agent",
		},
		{
			name:          "function_call item → Standard quota (free)",
			body:          `{"model":"gpt-4o","input":[{"role":"user","content":"hi"},{"type":"function_call","name":"read_file","arguments":"{}","call_id":"c1"}]}`,
			wantInitiator: "agent",
		},
		{
			name:          "function_call_output item → Standard quota (free)",
			body:          `{"model":"gpt-4o","input":[{"role":"user","content":"hi"},{"type":"function_call_output","call_id":"c1","output":"file contents"}]}`,
			wantInitiator: "agent",
		},
		{
			name:          "continuation via previous_response_id → Standard quota (free)",
			body:          `{"model":"gpt-4o","previous_response_id":"resp_abc123","input":"continue the task"}`,
			wantInitiator: "agent",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedInitiator string
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedInitiator = r.Header.Get("X-Initiator")
				_, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `data: {"type":"response.completed","response":{"id":"r1","status":"completed","model":"gpt-4o","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":10,"output_tokens":5}}}`, "\n\n")
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer srv.Close()

			provider := NewCopilotTokenProvider()
			tok := newCopilotTestToken("tok-responses-" + tc.name)
			provider.tokens[1] = &tok

			svc := NewCopilotGatewayService(provider)
			svc.httpClient = newRedirectingHTTPClient(srv)

			account := &Account{
				ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"github_token": "ghp_test"},
			}

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/copilot/v1/responses", nil)

			_, err := svc.ForwardResponses(context.Background(), c, account, []byte(tc.body))
			if err != nil {
				t.Fatalf("ForwardResponses: %v", err)
			}

			if capturedInitiator != tc.wantInitiator {
				t.Errorf("X-Initiator = %q, want %q\n(wrong value burns upstream Premium quota on every sub-request)", capturedInitiator, tc.wantInitiator)
			}
		})
	}
}

// TestXInitiatorHeader_MessagesEndpoint verifies that ForwardMessages (Anthropic
// protocol, used by Claude Code) sets the correct X-Initiator header.
// Anthropic tool_use/tool_result blocks must translate to "agent" after the
// OpenAI translation layer — this test pins that end-to-end behavior.
func TestXInitiatorHeader_MessagesEndpoint(t *testing.T) {
	cases := []struct {
		name          string
		body          string
		wantInitiator string
	}{
		{
			name: "first turn user only → Premium quota",
			body: `{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}`,
			wantInitiator: "user",
		},
		{
			name: "multi-turn with assistant → Standard quota (free)",
			body: `{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"},{"role":"user","content":"more"}]}`,
			wantInitiator: "agent",
		},
		{
			name: "tool_use + tool_result (Anthropic format) → Standard quota (free)",
			body: `{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":"read file"},{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"read_file","input":{"path":"/tmp/a"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"file contents"}]}]}`,
			wantInitiator: "agent",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var capturedInitiator string
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/models" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = fmt.Fprint(w, `{"data":[{"id":"claude-sonnet-4-5","supported_endpoints":["/chat/completions"]}]}`)
					return
				}
				capturedInitiator = r.Header.Get("X-Initiator")
				_, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n")
				fmt.Fprint(w, "data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":3}}\n\n")
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer srv.Close()

			provider := NewCopilotTokenProvider()
			tok := newCopilotTestToken("tok-messages-" + tc.name)
			provider.tokens[1] = &tok

			svc := NewCopilotGatewayService(provider)
			svc.httpClient = newRedirectingHTTPClient(srv)

			account := &Account{
				ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"github_token": "ghp_test"},
			}

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/copilot/v1/messages", nil)
			c.Request.Header.Set("Accept", "text/event-stream")

			_, err := svc.ForwardMessages(context.Background(), c, account, []byte(tc.body))
			if err != nil {
				t.Fatalf("ForwardMessages: %v", err)
			}

			if capturedInitiator != tc.wantInitiator {
				t.Errorf("X-Initiator = %q, want %q\n(wrong value burns upstream Premium quota on every sub-request)", capturedInitiator, tc.wantInitiator)
			}
		})
	}
}

// TestParseModelEndpointsFromModelsResponse_NormalizedKeyAlias verifies that
// parseModelEndpointsFromModelsResponse indexes each Claude model under both its
// raw dash-separated ID (as returned by Copilot /models) and the normalized
// dot-separated form (as produced by NormalizeModelIDForCopilotUpstream).
//
// This is the regression test for the bug where claude-opus-4.6 file attachments
// were silently downgraded to text because the /models cache stored the entry
// under "claude-opus-4-6" but the lookup used "claude-opus-4.6".
func TestParseModelEndpointsFromModelsResponse_NormalizedKeyAlias(t *testing.T) {
	body := []byte(`{"data":[
		{"id":"claude-opus-4-6","supported_endpoints":["/chat/completions","/responses"]},
		{"id":"claude-sonnet-4-6","supported_endpoints":["/chat/completions","/responses"]},
		{"id":"gpt-4o","supported_endpoints":["/chat/completions","/responses"]}
	]}`)

	m := parseModelEndpointsFromModelsResponse(body)

	cases := []struct {
		key  string
		want []string
	}{
		// Raw dash form (what /models returns)
		{"claude-opus-4-6", []string{"/chat/completions", "/responses"}},
		// Normalized dot form (what NormalizeModelIDForCopilotUpstream returns)
		{"claude-opus-4.6", []string{"/chat/completions", "/responses"}},
		{"claude-sonnet-4-6", []string{"/chat/completions", "/responses"}},
		{"claude-sonnet-4.6", []string{"/chat/completions", "/responses"}},
		// Non-Claude model: no normalization needed, only raw key exists
		{"gpt-4o", []string{"/chat/completions", "/responses"}},
	}

	for _, tc := range cases {
		eps, ok := m[tc.key]
		if !ok {
			t.Errorf("key %q not found in map; available keys: %v", tc.key, modelEndpointMapKeys(m))
			continue
		}
		if len(eps) != len(tc.want) {
			t.Errorf("key %q: got endpoints %v, want %v", tc.key, eps, tc.want)
			continue
		}
		for i, ep := range eps {
			if ep != tc.want[i] {
				t.Errorf("key %q: endpoint[%d] = %q, want %q", tc.key, i, ep, tc.want[i])
			}
		}
	}
}

// TestForwardChatCompletions_FilePartsClaudeOpusDashID verifies that when a
// Chat Completions request with a file attachment targets claude-opus-4.6 and
// the Copilot /models response uses the dash-separated ID "claude-opus-4-6",
// the request is still routed through /responses (not downgraded to text).
// This is the regression test for the ID mismatch bug.
func TestForwardChatCompletions_FilePartsClaudeOpusDashID(t *testing.T) {
	var capturedPath string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Copilot returns dash-separated IDs; this is what triggers the bug.
			_, _ = fmt.Fprint(w, `{"data":[{"id":"claude-opus-4-6","supported_endpoints":["/chat/completions","/responses"]}]}`)
			return
		}

		capturedPath = r.URL.Path
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `data: {"type":"response.completed","response":{"id":"resp_1","status":"completed","model":"claude-opus-4-6","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"PDF content read successfully"}]}],"usage":{"input_tokens":40,"output_tokens":15}}}`, "\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	provider := NewCopilotTokenProvider()
	tok := newCopilotTestToken("copilot-token-opus")
	provider.tokens[1] = &tok

	svc := NewCopilotGatewayService(provider)
	svc.httpClient = newRedirectingHTTPClient(srv)

	account := &Account{
		ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"github_token": "ghp_test"},
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/copilot/v1/chat/completions", nil)

	// Client sends dot-separated model ID (as Cherry Studio does).
	clientBody := []byte(`{"model":"claude-opus-4.6","stream":false,"messages":[{"role":"user","content":[{"type":"text","text":"总结这个PDF"},{"type":"file","file":{"filename":"doc.pdf","file_data":"data:application/pdf;base64,JVBERi0x"}}]}]}`)
	_, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
	if err != nil {
		t.Fatalf("ForwardChatCompletions: %v", err)
	}

	// Must have been routed to /responses, not /chat/completions.
	if capturedPath != "/responses" {
		t.Fatalf("expected upstream path /responses, got %q — file attachment was downgraded to text (ID mismatch bug)", capturedPath)
	}

	// Response must contain the actual file-read content.
	if !strings.Contains(w.Body.String(), "PDF content read successfully") {
		t.Errorf("expected file content in response; got: %s", w.Body.String())
	}
}

// modelEndpointMapKeys returns the keys of a map[string][]string for use in test error messages.
func modelEndpointMapKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestStaticSupportedEndpoints verifies the fallback static endpoint knowledge
// used when the live /models fetch fails.
func TestStaticSupportedEndpoints(t *testing.T) {
	cases := []struct {
		modelID      string
		wantResponses bool
	}{
		{"claude-opus-4.6", true},
		{"claude-opus-4-6", true},
		{"claude-sonnet-4.6", true},
		{"claude-sonnet-4-6", true},
		{"claude-haiku-4.5", true},
		{"claude-haiku-4-5", true},
		{"claude-sonnet-4", true},
		{"claude-opus-4", true},
		// Claude 3.x and non-Claude models should NOT get /responses from static list.
		{"claude-3.5-sonnet", false},
		{"gpt-4o", false},
		{"gemini-2.0-flash-001", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.modelID, func(t *testing.T) {
			eps := staticSupportedEndpoints(tc.modelID)
			hasResponses := false
			for _, ep := range eps {
				if ep == "/responses" {
					hasResponses = true
					break
				}
			}
			if hasResponses != tc.wantResponses {
				t.Errorf("staticSupportedEndpoints(%q): hasResponses=%v, want %v (got %v)", tc.modelID, hasResponses, tc.wantResponses, eps)
			}
		})
	}
}
