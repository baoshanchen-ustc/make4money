package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeResponsesRequestServiceTier(t *testing.T) {
	t.Parallel()

	req := &apicompat.ResponsesRequest{ServiceTier: " fast "}
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "priority", req.ServiceTier)

	req.ServiceTier = "flex"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "flex", req.ServiceTier)

	// OpenAI 官方合法 tier 应被透传保留。
	req.ServiceTier = "auto"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "auto", req.ServiceTier)

	req.ServiceTier = "default"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "default", req.ServiceTier)

	req.ServiceTier = "scale"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "scale", req.ServiceTier)

	// 真未知值仍被剥离。
	req.ServiceTier = "turbo"
	normalizeResponsesRequestServiceTier(req)
	require.Empty(t, req.ServiceTier)
}

func TestNormalizeResponsesBodyServiceTier(t *testing.T) {
	t.Parallel()

	body, tier, err := normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"fast"}`))
	require.NoError(t, err)
	require.Equal(t, "priority", tier)
	require.Equal(t, "priority", gjson.GetBytes(body, "service_tier").String())

	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"flex"}`))
	require.NoError(t, err)
	require.Equal(t, "flex", tier)
	require.Equal(t, "flex", gjson.GetBytes(body, "service_tier").String())

	// OpenAI 官方 tier 直接保留在 body 中（透传上游）。
	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"auto"}`))
	require.NoError(t, err)
	require.Equal(t, "auto", tier)
	require.Equal(t, "auto", gjson.GetBytes(body, "service_tier").String())

	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"default"}`))
	require.NoError(t, err)
	require.Equal(t, "default", tier)
	require.Equal(t, "default", gjson.GetBytes(body, "service_tier").String())

	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"scale"}`))
	require.NoError(t, err)
	require.Equal(t, "scale", tier)
	require.Equal(t, "scale", gjson.GetBytes(body, "service_tier").String())

	// 真未知值才会被删除。
	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"turbo"}`))
	require.NoError(t, err)
	require.Empty(t, tier)
	require.False(t, gjson.GetBytes(body, "service_tier").Exists())
}

func TestIsDeepSeekBaseURL(t *testing.T) {
	t.Parallel()

	require.True(t, isDeepSeekBaseURL("https://api.deepseek.com"))
	require.True(t, isDeepSeekBaseURL("https://api.deepseek.com/v1"))
	require.False(t, isDeepSeekBaseURL("https://api.openai.com"))
}

func TestBuildOpenAIChatCompletionsURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://api.deepseek.com/chat/completions", buildOpenAIChatCompletionsURL("https://api.deepseek.com"))
	require.Equal(t, "https://api.deepseek.com/v1/chat/completions", buildOpenAIChatCompletionsURL("https://api.deepseek.com/v1"))
	require.Equal(t, "https://api.deepseek.com/chat/completions", buildOpenAIChatCompletionsURL("https://api.deepseek.com/chat/completions"))
}

func TestOpenAIGatewayService_ForwardAsChatCompletions_DeepSeekUsesDirectChatPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.3","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1}}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_deepseek"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          1,
		Name:        "deepseek",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://api.deepseek.com",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "https://api.deepseek.com/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "gpt-5.3", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Contains(t, rec.Body.String(), "chat.completion.chunk")
	require.Contains(t, rec.Body.String(), "[DONE]")
}

func TestOpenAIGatewayService_Forward_DeepSeekUsesDirectResponsesJSONPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4-mini","input":"hello","stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_deepseek_resp"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl-1",
			"object":"chat.completion",
			"created":1,
			"model":"deepseek-chat",
			"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}
		}`)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          1,
		Name:        "deepseek",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://api.deepseek.com",
		},
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "https://api.deepseek.com/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "gpt-5.4-mini", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "system", gjson.GetBytes(upstream.lastBody, "messages.0.role").String())
	require.Equal(t, "hello", gjson.GetBytes(upstream.lastBody, "messages.1.content").String())
	require.Equal(t, "response", gjson.GetBytes(rec.Body.Bytes(), "object").String())
	require.Equal(t, "ok", gjson.Get(rec.Body.String(), "output.0.content.0.text").String())
}

func TestOpenAIGatewayService_Forward_DeepSeekUsesDirectResponsesStreamPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4-mini","input":"hello","stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_deepseek_stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          1,
		Name:        "deepseek",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://api.deepseek.com",
		},
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), "event: response.created")
	require.Contains(t, rec.Body.String(), "event: response.output_text.delta")
	require.Contains(t, rec.Body.String(), "event: response.completed")
	require.Contains(t, rec.Body.String(), "data: [DONE]")
}
