# Copilot Gateway Robustness Improvement Plan (v3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix token-reporting zero values for Copilot streaming paths, persist OpsSpans for successful Copilot requests so the latency breakdown panel works, and harden token-refresh goroutine safety.

**Scope (explicitly bounded):**
- Token = 0 fix covers **two Copilot paths only**: `/messages → /chat/completions` and `/chat/completions` direct. The `/messages → /responses` path already captures usage via terminal events and is verified by a regression test.
- Spans persistence covers **Copilot handler goroutines only** (ChatCompletions, Responses, Messages). Other handlers (OpenAI, Sora) are out of scope to keep the change surgical.
- `WriteAnomalyLog` signature is **NOT changed**. Quota-exhaustion detection is added via an internal `AnomalySignal` struct used only inside `anomaly_service.go`, with no external interface changes.

**Architecture:** Four independent, incrementally-committable areas: (1) `ensureStreamIncludeUsage` helper + `isUsageOnlyChunk` filter added to `copilot_gateway_service.go`; (2) `handleStreamingResponse` gains a `forwardUsageChunk bool` parameter — **all three call sites updated**; (3) `RecordUsageInput.Spans` field added + Copilot handler goroutines capture spans before launch; (4) `CopilotTokenProvider` uses injected `tokenExchanger` function with context propagation — `copilot.ExchangeTokenWithContext` added as new function, existing `copilot.ExchangeToken` preserved for backward compat.

**Tech Stack:** Go 1.22+, `github.com/tidwall/sjson` / `gjson`, `bufio.Scanner`, `golang.org/x/sync/singleflight`, standard `net/http`, `github.com/gin-gonic/gin`.

---

## Verified Call-Site Counts (from codebase audit)

| Interface | Current callers | Callers updated in this plan |
|-----------|----------------|------------------------------|
| `handleStreamingResponse(c, resp, model, upstreamModel, startTime)` | 2 production + 1 test (ForwardChatCompletions:L236, ForwardResponses:L901, test:L388) | All 3 |
| `WriteAnomalyLog(ctx, in, out, dur, status, input)` | 6 total (copilot:3, openai:2, sora:1) | 0 — signature NOT changed |
| `copilot.ExchangeToken(httpClient, githubToken)` | 1 (`copilot_token_provider.go:101`) + 1 (`copilot_oauth_service.go:170`) | 0 — signature NOT changed; new `ExchangeTokenWithContext` added |
| `RecordUsageInput` struct | gateway_service.go:7196 | Copilot handler goroutines only |
| `UsageLog.Spans` assignment | gateway_service.go:7615, 7814 | Both |

---

## File Map

| File | Change type | What changes |
|------|-------------|-------------|
| `internal/service/copilot_gateway_service.go` | Modify | Add `ensureStreamIncludeUsage`, `isUsageOnlyChunk`; update `handleStreamingResponse` signature with `forwardUsageChunk bool`; update `ForwardChatCompletions`, `ForwardResponses`, and existing test call; call `ensureStreamIncludeUsage` in `ForwardMessages` and `ForwardChatCompletions` |
| `internal/service/copilot_gateway_service_test.go` | Modify/Add | Tests for `ensureStreamIncludeUsage`, `isUsageOnlyChunk`, integration tests for both Copilot paths, `/responses` regression test, update existing test call to `handleStreamingResponse` |
| `internal/service/gateway_service.go` | Modify | Add `Spans []*OpsSpan` to `RecordUsageInput`; pass `MarshalOpsSpans(input.Spans)` to `UsageLog.Spans` at both construction points (L7615, L7814) |
| `internal/handler/copilot_gateway_handler.go` | Modify | Capture `GetOpsSpans(c)` before goroutine in all 3 handler loops; pass to `RecordUsageInput.Spans` |
| `internal/service/copilot_token_provider.go` | Modify | Replace direct `copilot.ExchangeToken` call with injected `tokenExchanger` func; use combined context timeout; add `newCopilotTokenProviderWithExchanger` constructor |
| `internal/pkg/copilot/token.go` | Modify | Add `ExchangeTokenWithContext(ctx, httpClient, githubToken)` — new function with context; keep existing `ExchangeToken` calling `ExchangeTokenWithContext(context.Background(), ...)` |
| `internal/service/anomaly_service.go` | Modify | Add `AnomalySignal` struct and `AnomalyQuotaExhaustionSuspected` const; update `detectAnomalies` to accept `AnomalySignal`; update internal `WriteAnomalyLog` body to build `AnomalySignal` from existing params — **external signature unchanged** |

---

## Task 1: `ensureStreamIncludeUsage` + `isUsageOnlyChunk` helpers

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`
- Modify: `backend/internal/service/copilot_gateway_service_test.go`

### Step-by-step

- [ ] **Step 1.1: Write failing tests**

Add to `backend/internal/service/copilot_gateway_service_test.go`:

```go
func TestEnsureStreamIncludeUsage(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantVal string // expected gjson raw of stream_options.include_usage; "" = absent
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
```

- [ ] **Step 1.2: Run tests — confirm they fail**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestEnsureStreamIncludeUsage|TestIsUsageOnlyChunk" -v
```

Expected: `FAIL` — undefined function.

- [ ] **Step 1.3: Implement both helpers in `copilot_gateway_service.go`**

Add after `forceStreamTrue` (around line 1841):

```go
// ensureStreamIncludeUsage injects "stream_options":{"include_usage":true} into the
// request body when stream=true.  This causes the Copilot API to append a usage-summary
// SSE chunk at the end of the stream, enabling accurate token count recording.
// No-op when stream is absent or false; non-streaming responses include usage by default.
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
// usage-summary chunk that the Copilot API appends when stream_options.include_usage
// is true.  These chunks have a non-nil "usage" field but no content in "choices".
// Used to filter the chunk from the downstream stream when the original client
// request did not include stream_options.include_usage.
func isUsageOnlyChunk(data string) bool {
	if !gjson.Get(data, "usage").Exists() {
		return false
	}
	choices := gjson.Get(data, "choices")
	return !choices.Exists() || (choices.IsArray() && len(choices.Array()) == 0)
}
```

- [ ] **Step 1.4: Run tests — confirm they pass**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestEnsureStreamIncludeUsage|TestIsUsageOnlyChunk" -v
```

Expected: all 11 sub-cases `PASS`.

- [ ] **Step 1.5: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_gateway_service.go internal/service/copilot_gateway_service_test.go
git commit -m "Feature: 添加 ensureStreamIncludeUsage 和 isUsageOnlyChunk 辅助函数"
```

---

## Task 2: Update `handleStreamingResponse` signature and all 3 call sites

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`
- Modify: `backend/internal/service/copilot_gateway_service_test.go`

### Why

`handleStreamingResponse` is called from **3 places** (confirmed by audit):
1. `ForwardChatCompletions` (L236) — direct OpenAI proxy, client receives SSE verbatim
2. `ForwardResponses` (L901) — Copilot `/responses` endpoint, client receives SSE verbatim
3. Existing unit test (L388)

Only `ForwardChatCompletions` needs usage-chunk filtering (because on that path the client may be an OpenAI-mode client that didn't request usage). `ForwardResponses` is also a direct SSE path, so it gets the same treatment. The existing test must be updated to pass `forwardUsageChunk`.

### Step-by-step

- [ ] **Step 2.1: Write test for usage-chunk filtering in ForwardChatCompletions**

Add to `copilot_gateway_service_test.go`:

```go
func TestForwardChatCompletions_UsageChunkFilteredWhenClientDidNotRequest(t *testing.T) {
    var capturedBody []byte
    srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        capturedBody, _ = io.ReadAll(r.Body)
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
        // Copilot-injected usage-only chunk (present because include_usage was injected).
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

    // Upstream body must have include_usage injected (root-cause verification).
    if !gjson.GetBytes(capturedBody, "stream_options.include_usage").Bool() {
        t.Errorf("expected stream_options.include_usage=true in upstream body; got %s", capturedBody)
    }

    // Token counts must be non-zero.
    if result.Usage == nil || result.Usage.PromptTokens == 0 {
        t.Errorf("expected non-zero PromptTokens; got %+v", result.Usage)
    }

    // Client response must NOT contain the usage-only chunk.
    body := w.Body.String()
    if strings.Contains(body, `"prompt_tokens"`) {
        t.Errorf("usage-only chunk must be filtered from client response; got: %s", body)
    }
    if !strings.Contains(body, "hello") {
        t.Errorf("content chunk must be forwarded; got: %s", body)
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

    // Client explicitly requests include_usage.
    clientBody := []byte(`{"model":"gpt-4o","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`)
    result, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
    if err != nil {
        t.Fatalf("ForwardChatCompletions: %v", err)
    }

    if result.Usage == nil || result.Usage.PromptTokens == 0 {
        t.Errorf("expected non-zero PromptTokens; got %+v", result.Usage)
    }

    // Client SHOULD receive the usage chunk.
    body := w.Body.String()
    if !strings.Contains(body, `"prompt_tokens"`) {
        t.Errorf("usage chunk must be forwarded when client requested it; got: %s", body)
    }
}
```

- [ ] **Step 2.2: Run tests — confirm they fail**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestForwardChatCompletions_UsageChunk" -v
```

Expected: `FAIL`

- [ ] **Step 2.3: Update `handleStreamingResponse` signature**

In `copilot_gateway_service.go`, change the function signature:

```go
// handleStreamingResponse proxies SSE streaming from Copilot API to the client.
//
// forwardUsageChunk controls whether a Copilot-injected usage-summary chunk
// (added when stream_options.include_usage=true is set upstream) is forwarded
// to the downstream client.  Set to true only when the original client request
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
		if err := c.Request.Context().Err(); err != nil {
			slog.Debug("copilot stream: client disconnected", "error", err)
			return &CopilotForwardResult{
				StatusCode: http.StatusOK, Model: model, UpstreamModel: upstreamModel,
				Usage: usage, Duration: time.Since(startTime), FirstTokenMs: firstTokenMs,
			}, nil
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "data: ") {
			data := line[6:]
			if data == "[DONE]" {
				streamDone = true
				fmt.Fprintf(c.Writer, "%s\n", line)
				flusher.Flush()
				break
			}
			if firstTokenMs == nil {
				ms := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &ms
			}
			s.parseStreamUsage(data, usage)

			// Filter the upstream-injected usage-only chunk when the client did not
			// request it.  Still parsed above so token counts are always recorded.
			if !forwardUsageChunk && isUsageOnlyChunk(data) {
				continue
			}
		}

		fmt.Fprintf(c.Writer, "%s\n", line)
		flusher.Flush()
	}

	if !streamDone {
		if err := scanner.Err(); err != nil {
			slog.Warn("copilot stream scanner error", "error", err)
		}
	}

	return &CopilotForwardResult{
		StatusCode: http.StatusOK, Model: model, UpstreamModel: upstreamModel,
		Usage: usage, Duration: time.Since(startTime), FirstTokenMs: firstTokenMs,
	}, nil
}
```

- [ ] **Step 2.4: Update ForwardChatCompletions call site (L236)**

Before the streaming dispatch in `ForwardChatCompletions`, add:

```go
	// Remember whether the client wants a usage chunk in the stream,
	// BEFORE ensureStreamIncludeUsage may override stream_options.
	clientWantsUsageChunk := gjson.GetBytes(body, "stream_options.include_usage").Bool()

	body = ensureStreamIncludeUsage(body)
```

Change the call from:

```go
	return s.handleStreamingResponse(c, resp, model, upstreamSent, startTime)
```

to:

```go
	return s.handleStreamingResponse(c, resp, model, upstreamSent, startTime, clientWantsUsageChunk)
```

- [ ] **Step 2.5: Update ForwardResponses call site (L901)**

In `ForwardResponses`, before calling `handleStreamingResponse` (around line 901), add:

```go
	clientWantsUsageChunkResp := gjson.GetBytes(body, "stream_options.include_usage").Bool()
	body = ensureStreamIncludeUsage(body)
```

Change the call from:

```go
	result, fwdErr = s.handleStreamingResponse(c, resp, model, upstreamSent, startTime)
```

to:

```go
	result, fwdErr = s.handleStreamingResponse(c, resp, model, upstreamSent, startTime, clientWantsUsageChunkResp)
```

- [ ] **Step 2.6: Update the existing unit test call (line ~388)**

Find the existing test that directly calls `handleStreamingResponse` and add `false` as the last argument:

```go
result, err := svc.handleStreamingResponse(c, resp, "gpt-4o", "gpt-4o", startTime, false)
```

- [ ] **Step 2.7: Build verification**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

Expected: no compilation errors (all 3 call sites updated).

- [ ] **Step 2.8: Run tests**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestForwardChatCompletions_UsageChunk" -v
go test ./internal/service/... -timeout 120s
```

Expected: new tests pass, no regressions.

- [ ] **Step 2.9: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_gateway_service.go internal/service/copilot_gateway_service_test.go
git commit -m "Fix: ForwardChatCompletions/ForwardResponses 注入 include_usage 并过滤未请求的 usage chunk"
```

---

## Task 3: Wire `ensureStreamIncludeUsage` into `ForwardMessages` + `/responses` regression test

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`
- Modify: `backend/internal/service/copilot_gateway_service_test.go`

### Step-by-step

- [ ] **Step 3.1: Write integration test for ForwardMessages**

```go
func TestForwardMessages_UpstreamBodyHasStreamIncludeUsage(t *testing.T) {
    var capturedBody []byte
    srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        capturedBody, _ = io.ReadAll(r.Body)
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        // Minimal Anthropic SSE: two delta chunks + usage chunk + DONE.
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

    gin.SetMode(gin.TestMode)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

    anthropicBody := []byte(`{"model":"claude-sonnet-4-5","stream":false,"max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`)
    result, err := svc.ForwardMessages(context.Background(), c, account, anthropicBody)
    if err != nil {
        t.Fatalf("ForwardMessages: %v", err)
    }

    // Upstream body: stream=true (from forceStreamTrue) AND include_usage=true.
    if !gjson.GetBytes(capturedBody, "stream").Bool() {
        t.Errorf("expected stream=true in upstream body; got %s", capturedBody)
    }
    if !gjson.GetBytes(capturedBody, "stream_options.include_usage").Bool() {
        t.Errorf("expected stream_options.include_usage=true; got %s", capturedBody)
    }
    // Token counts must be non-zero.
    if result == nil || result.Usage == nil || result.Usage.PromptTokens == 0 {
        t.Errorf("expected non-zero PromptTokens; result=%+v", result)
    }
}
```

- [ ] **Step 3.2: Write /responses regression test (confirms path unaffected)**

```go
func TestForwardMessages_ViaResponses_UsageNonZeroWithoutIncludeUsage(t *testing.T) {
    // The /responses path reads usage from response.completed terminal event,
    // not from stream_options.include_usage.  This regression test ensures
    // that the /responses path still reports non-zero token counts.
    srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        // Minimal Responses API SSE stream.
        fmt.Fprint(w, "event: response.output_text.delta\n")
        fmt.Fprint(w, `data: {"type":"response.output_text.delta","delta":"hello"}`)
        fmt.Fprint(w, "\n\n")
        fmt.Fprint(w, "event: response.completed\n")
        fmt.Fprint(w, `data: {"type":"response.completed","response":{"id":"r1","usage":{"input_tokens":12,"output_tokens":4}}}`)
        fmt.Fprint(w, "\n\n")
    }))
    defer srv.Close()

    provider := NewCopilotTokenProvider()
    tok := newCopilotTestToken("copilot-token-resp")
    provider.tokens[1] = &tok

    svc := NewCopilotGatewayService(provider)
    svc.httpClient = srv.Client()

    // Account with base_url pointing to /responses endpoint.
    // The /responses routing is gated on supportedEndpoints returned by /models,
    // which we bypass by setting a test flag on the service.
    // Use an account with forceResponses=true via test helper.
    account := &Account{
        ID: 1, Platform: PlatformCopilot, Type: AccountTypeAPIKey,
        Credentials: map[string]any{
            "github_token": "ghp_test",
            "base_url":     srv.URL,
        },
        Extra: map[string]any{
            "_test_force_responses_endpoint": true,
        },
    }

    gin.SetMode(gin.TestMode)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

    anthropicBody := []byte(`{"model":"claude-sonnet-4-5","stream":true,"max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`)
    result, err := svc.ForwardMessages(context.Background(), c, account, anthropicBody)
    if err != nil {
        t.Fatalf("ForwardMessages via /responses: %v", err)
    }
    if result == nil || result.Usage == nil {
        t.Fatal("expected non-nil result with usage")
    }
    if result.Usage.PromptTokens == 0 {
        t.Errorf("expected non-zero PromptTokens on /responses path; got %+v", result.Usage)
    }
}
```

**Note:** If the `_test_force_responses_endpoint` Extra flag approach doesn't match existing test infrastructure, simplify to a direct `svc.forwardMessagesViaResponses(...)` call, which is already an exported method in the test package.

- [ ] **Step 3.3: Run tests — confirm they fail**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestForwardMessages_Upstream|TestForwardMessages_ViaResponses" -v
```

Expected: `FAIL`

- [ ] **Step 3.4: Add `ensureStreamIncludeUsage` call in `ForwardMessages`**

In `copilot_gateway_service.go`, find (around line 981):

```go
	openAIBody = forceStreamTrue(openAIBody)
```

Change to:

```go
	openAIBody = forceStreamTrue(openAIBody)
	// Ensure Copilot returns token-usage statistics in the final SSE chunk.
	// Must run after forceStreamTrue so stream=true is visible to the helper.
	openAIBody = ensureStreamIncludeUsage(openAIBody)
```

- [ ] **Step 3.5: Run integration test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestForwardMessages_UpstreamBodyHasStreamIncludeUsage" -v
```

Expected: `PASS`

- [ ] **Step 3.6: Run full service tests**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -timeout 120s
```

Expected: all pass.

- [ ] **Step 3.7: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_gateway_service.go internal/service/copilot_gateway_service_test.go
git commit -m "Fix: ForwardMessages 注入 stream_options.include_usage 修复 token 计数为 0"
```

---

## Task 4: Persist OpsSpans for successful Copilot requests

**Files:**
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/handler/copilot_gateway_handler.go`

### Why

`UsageLog.Spans *string` exists. `MarshalOpsSpans` exists. But `RecordUsageInput` has no `Spans` field, and neither `UsageLog` construction point in `gateway_service.go` (L7615, L7814) assigns `Spans`. Adding the field and wiring it through makes the latency breakdown panel work for all successful Copilot requests.

**Scope note:** Only Copilot handler goroutines are updated in this task. OpenAI and Sora handler goroutines are left for a follow-up task to keep this change minimal.

### Step-by-step

- [ ] **Step 4.1: Write test for span persistence**

This test uses the `//go:build unit` pattern from the existing codebase. Create `backend/internal/service/copilot_spans_test.go`:

```go
//go:build unit

package service

import (
    "context"
    "testing"
)

func TestRecordUsage_CopilotSpansPersistedToUsageLog(t *testing.T) {
    var savedLog *UsageLog
    usageRepo := &stubUsageLogRepo{
        saveFn: func(_ context.Context, log *UsageLog) error {
            savedLog = log
            return nil
        },
    }
    // Use the existing unit-test constructor pattern from gateway_record_usage_test.go.
    svc := newGatewayRecordUsageServiceForTest(usageRepo, &stubUsageUserRepo{}, &stubUsageSubRepo{})

    spans := []*OpsSpan{
        {Name: "token.fetch", StartUnixMs: 1000, DurationMs: 50, Status: "ok"},
        {Name: "upstream.post", StartUnixMs: 1050, DurationMs: 800, Status: "ok"},
    }

    _, _, err := svc.RecordUsage(context.Background(), &RecordUsageInput{
        Result:  &ForwardResult{Model: "gpt-4o", Duration: 1 * 1e9}, // 1s in nanoseconds
        APIKey:  unitTestAPIKey(),
        User:    unitTestUser(),
        Account: unitTestAccount(PlatformCopilot),
        Spans:   spans,
    })
    if err != nil {
        t.Fatalf("RecordUsage: %v", err)
    }
    if savedLog == nil {
        t.Fatal("UsageLog was not saved")
    }
    if savedLog.Spans == nil {
        t.Fatal("expected Spans non-nil in saved UsageLog")
    }
    if !strings.Contains(*savedLog.Spans, "token.fetch") {
        t.Errorf("expected token.fetch in spans; got %s", *savedLog.Spans)
    }
}
```

**Note:** Replace `stubUsageLogRepo`, `newGatewayRecordUsageServiceForTest`, `unitTestAPIKey`, `unitTestUser`, `unitTestAccount` with the actual names from `gateway_record_usage_test.go`. Look them up before writing rather than guessing.

- [ ] **Step 4.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test -tags unit ./internal/service/... -run TestRecordUsage_CopilotSpansPersistedToUsageLog -v
```

Expected: `FAIL` — `RecordUsageInput` has no `Spans` field.

- [ ] **Step 4.3: Add `Spans` to `RecordUsageInput`**

In `gateway_service.go`, in the `RecordUsageInput` struct, add after the existing latency fields:

```go
// Spans holds per-phase timing events collected by AppendOpsSpan during the request.
// When non-nil, serialised via MarshalOpsSpans and stored in usage_logs.spans.
Spans []*OpsSpan
```

- [ ] **Step 4.4: Assign Spans in both UsageLog construction points**

**Point 1** (around L7615 in `writeUsageLogBestEffort`):

```go
usageLog := &UsageLog{
    // ... existing fields ...
    Spans: MarshalOpsSpans(input.Spans),
}
```

**Point 2** (around L7814, the second construction site):

```go
usageLog := &UsageLog{
    // ... existing fields ...
    Spans: MarshalOpsSpans(input.Spans),
}
```

- [ ] **Step 4.5: Capture spans before goroutine in Copilot handler**

In `copilot_gateway_handler.go`, in the `ChatCompletions` recording goroutine setup, after the existing `capturedUpstreamReqBody, capturedUpstreamRespBody` capture:

```go
// Capture a shallow copy of the span slice before entering the goroutine.
// gin.Context is not safe across goroutines; the slice header is copied here
// and MarshalOpsSpans will serialise the element pointers which are immutable
// after this point.
capturedSpans := make([]*service.OpsSpan, len(service.GetOpsSpans(c)))
copy(capturedSpans, service.GetOpsSpans(c))
```

Add `Spans: capturedSpans` to the `RecordUsageInput` in the `go func()`:

```go
requestID, usageLogID, err := h.gatewayService.RecordUsage(recordCtx, &service.RecordUsageInput{
    // ... existing fields ...
    Spans: capturedSpans,
})
```

Apply the same `capturedSpans` capture + pass pattern to the **Responses** and **Messages** handler goroutines in the same file.

- [ ] **Step 4.6: Run test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test -tags unit ./internal/service/... -run TestRecordUsage_CopilotSpansPersistedToUsageLog -v
```

Expected: `PASS`

- [ ] **Step 4.7: Build and run full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go test -tags unit ./... -timeout 120s && go test ./... -timeout 120s
```

Expected: no errors.

- [ ] **Step 4.8: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/gateway_service.go internal/handler/copilot_gateway_handler.go \
        internal/service/copilot_spans_test.go
git commit -m "Feature: Copilot 成功请求 OpsSpans 持久化到 usage_logs，ops 看板延迟分解可见"
```

---

## Task 5: Context-aware token exchange via injected `tokenExchanger`

**Files:**
- Modify: `backend/internal/pkg/copilot/token.go`
- Modify: `backend/internal/service/copilot_token_provider.go`
- Add: `backend/internal/service/copilot_token_provider_test.go` (new or existing)

### Why

`copilot.ExchangeToken` currently uses `http.NewRequest` (no context). A slow GitHub API holds the goroutine for up to 30 s even after the caller cancels. The fix: add `ExchangeTokenWithContext(ctx, ...)` as a new function; keep the existing `ExchangeToken` calling it with `context.Background()` so `copilot_oauth_service.go` (the other caller) needs zero changes. Inject a `tokenExchanger` function dependency into `CopilotTokenProvider` for clean test isolation.

### Step-by-step

- [ ] **Step 5.1: Write test for context cancellation**

Add to `backend/internal/service/copilot_token_provider_test.go`:

```go
func TestGetAccessToken_RespectsContextCancellation(t *testing.T) {
    hanging := make(chan struct{})
    fakeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        close(hanging) // signal: server received the request
        <-r.Context().Done()
        w.WriteHeader(http.StatusServiceUnavailable)
    }))
    defer fakeSrv.Close()

    mockExchanger := func(ctx context.Context, client *http.Client, token string) (*copilot.CopilotToken, error) {
        req, err := http.NewRequestWithContext(ctx, http.MethodGet, fakeSrv.URL, nil)
        if err != nil {
            return nil, err
        }
        _, err = client.Do(req) //nolint:gosec
        return nil, errors.New("exchange failed: " + err.Error())
    }

    provider := newCopilotTokenProviderWithExchanger(mockExchanger)

    ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
    defer cancel()

    // Launch GetAccessToken in goroutine so we can wait for the server to hang first.
    errCh := make(chan error, 1)
    go func() {
        _, err := provider.GetAccessToken(ctx, &Account{
            ID:          1,
            Platform:    PlatformCopilot,
            Credentials: map[string]any{"github_token": "ghp_test"},
        })
        errCh <- err
    }()

    // Wait until server is blocking.
    <-hanging

    start := time.Now()
    err := <-errCh
    elapsed := time.Since(start)

    if err == nil {
        t.Fatal("expected error on context cancellation, got nil")
    }
    if elapsed > 2*time.Second {
        t.Errorf("GetAccessToken did not respect cancellation: elapsed %v", elapsed)
    }
}

func TestGetAccessToken_LogsExchangeLatency(t *testing.T) {
    origLogger := slog.Default()
    var logBuf bytes.Buffer
    slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
    t.Cleanup(func() { slog.SetDefault(origLogger) }) // restore original

    mockExchanger := func(ctx context.Context, client *http.Client, token string) (*copilot.CopilotToken, error) {
        time.Sleep(20 * time.Millisecond)
        return &copilot.CopilotToken{
            Token:     "ghs_test_token",
            ExpiresAt: time.Now().Add(30 * time.Minute),
            RefreshAt: time.Now().Add(5 * time.Minute),
        }, nil
    }
    provider := newCopilotTokenProviderWithExchanger(mockExchanger)

    tok, err := provider.GetAccessToken(context.Background(), &Account{
        ID:          99,
        Platform:    PlatformCopilot,
        Credentials: map[string]any{"github_token": "ghp_test"},
    })
    if err != nil || tok == "" {
        t.Fatalf("GetAccessToken: err=%v tok=%q", err, tok)
    }
    if !strings.Contains(logBuf.String(), "exchange_ms") {
        t.Errorf("expected exchange_ms in log; got: %s", logBuf.String())
    }
}
```

- [ ] **Step 5.2: Run tests — confirm they fail**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestGetAccessToken_Respects|TestGetAccessToken_Logs" -v -timeout 10s
```

Expected: `FAIL` — `newCopilotTokenProviderWithExchanger` undefined.

- [ ] **Step 5.3: Add `ExchangeTokenWithContext` to `token.go`**

In `internal/pkg/copilot/token.go`, **add** (do not change existing `ExchangeToken`):

```go
// ExchangeTokenWithContext exchanges a GitHub personal access token for a short-lived
// Copilot API token.  ctx controls the HTTP exchange — cancelling it aborts the call.
func ExchangeTokenWithContext(ctx context.Context, httpClient *http.Client, githubToken string) (*CopilotToken, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, TokenExchangeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("copilot token exchange: build request: %w", err)
	}

	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("editor-version", DefaultEditorVersion)
	req.Header.Set("editor-plugin-version", DefaultEditorPluginVersion)
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("x-github-api-version", DefaultGitHubAPIVersion)
	req.Header.Set("x-vscode-user-agent-library-version", "electron-fetch")

	resp, err := httpClient.Do(req) //nolint:gosec // URL is a trusted constant
	if err != nil {
		return nil, fmt.Errorf("copilot token exchange: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("copilot token exchange: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("copilot token exchange: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenExchangeResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("copilot token exchange: parse response: %w", err)
	}
	if tokenResp.Token == "" {
		errMsg := tokenResp.ErrorMessage
		if errMsg == "" {
			errMsg = "empty token in response"
		}
		return nil, fmt.Errorf("copilot token exchange: %s", errMsg)
	}

	now := time.Now()
	expiresAt := now.Add(30 * time.Minute)
	if tokenResp.ExpiresAt > 0 {
		expiresAt = time.Unix(tokenResp.ExpiresAt, 0)
	}
	refreshIn := tokenResp.RefreshIn
	if refreshIn <= 0 {
		refreshIn = int64(time.Until(expiresAt).Seconds()) - 60
	}
	if refreshIn < 30 {
		refreshIn = 30
	}
	refreshAt := now.Add(time.Duration(refreshIn-60) * time.Second)
	return &CopilotToken{Token: tokenResp.Token, ExpiresAt: expiresAt, RefreshAt: refreshAt}, nil
}
```

Update the existing `ExchangeToken` to delegate (preserves all existing callers):

```go
// ExchangeToken is a convenience wrapper for ExchangeTokenWithContext using
// context.Background().  Existing callers (e.g. copilot_oauth_service) are
// unaffected; new callers should prefer ExchangeTokenWithContext.
func ExchangeToken(httpClient *http.Client, githubToken string) (*CopilotToken, error) {
	return ExchangeTokenWithContext(context.Background(), httpClient, githubToken)
}
```

- [ ] **Step 5.4: Add `tokenExchanger` type and update `CopilotTokenProvider`**

In `copilot_token_provider.go`:

```go
// tokenExchanger exchanges a GitHub token for a Copilot token.
// Injected at construction time; production uses copilot.ExchangeTokenWithContext.
type tokenExchanger func(ctx context.Context, httpClient *http.Client, githubToken string) (*copilot.CopilotToken, error)

type CopilotTokenProvider struct {
	httpClient *http.Client
	exchange   tokenExchanger

	mu     sync.RWMutex
	tokens map[int64]*copilot.CopilotToken

	sfGroup singleflight.Group
}

// NewCopilotTokenProvider creates a production CopilotTokenProvider.
func NewCopilotTokenProvider() *CopilotTokenProvider {
	return newCopilotTokenProviderWithExchanger(copilot.ExchangeTokenWithContext)
}

// newCopilotTokenProviderWithExchanger creates a CopilotTokenProvider with a
// custom exchange function.  Intended for tests only.
func newCopilotTokenProviderWithExchanger(ex tokenExchanger) *CopilotTokenProvider {
	return &CopilotTokenProvider{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		exchange:   ex,
		tokens:     make(map[int64]*copilot.CopilotToken),
	}
}
```

In the singleflight body inside `GetAccessToken`, replace the `copilot.ExchangeToken` call:

```go
		exchangeStart := time.Now()
		// Use a combined context: respect caller cancellation but cap at 20 s.
		exchangeCtx, exchangeCancel := context.WithTimeout(ctx, 20*time.Second)
		defer exchangeCancel()

		newToken, err := p.exchange(exchangeCtx, p.httpClient, githubToken)
		exchangeMs := time.Since(exchangeStart).Milliseconds()
		if err != nil {
			slog.Error("copilot token exchange failed",
				"account_id", account.ID,
				"exchange_ms", exchangeMs,
				"error", err)
			if fallbackToken != "" {
				return fallbackToken, nil
			}
			return "", fmt.Errorf("copilot token exchange: %w", err)
		}

		p.mu.Lock()
		p.tokens[account.ID] = newToken
		p.mu.Unlock()

		slog.Debug("copilot token refreshed",
			"account_id", account.ID,
			"exchange_ms", exchangeMs,
			"expires_at", newToken.ExpiresAt.Format(time.RFC3339))

		return newToken.Token, nil
```

- [ ] **Step 5.5: Run tests — confirm they pass**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestGetAccessToken_Respects|TestGetAccessToken_Logs" -v -timeout 5s
go test ./internal/pkg/copilot/... -v
```

Expected: all pass.

- [ ] **Step 5.6: Run full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go test ./... -timeout 120s
```

Expected: all pass.

- [ ] **Step 5.7: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_token_provider.go internal/service/copilot_token_provider_test.go \
        internal/pkg/copilot/token.go
git commit -m "Fix: CopilotTokenProvider 注入 exchanger 依赖，token exchange 传递 context"
```

---

## Task 6: `quota_exhaustion_suspected` anomaly — internal only, no external signature change

**Files:**
- Modify: `backend/internal/service/anomaly_service.go`
- Modify: `backend/internal/handler/copilot_gateway_handler.go`

### Why

The `WriteAnomalyLog` external signature has **6 callers across 3 files**. Changing it would require touching OpenAI and Sora handlers. Instead: (1) add `AnomalySignal` struct internally for use by `detectAnomalies`; (2) keep `WriteAnomalyLog` signature identical; (3) the only new data needed — `upstreamLatencyMs` — is extracted from `RecordUsageInput.UpstreamLatencyMs` which is already an `*int` captured by the handler.

The solution: Copilot handler passes `upstreamLatencyMsVal` (type `*int`) as part of the new `RequestLogInput.UpstreamLatencyMs` field (adding this one field to `RequestLogInput` has zero risk — it's only read by `detectAnomalies` internally). The OpenAI/Sora handlers pass `nil` for this field — no change needed.

### Step-by-step

- [ ] **Step 6.1: Write test for quota-exhaustion detection**

Add to `backend/internal/service/anomaly_service_test.go` (or a new file, without build tag):

```go
func TestDetectAnomalies_QuotaExhaustionSuspected(t *testing.T) {
    settings := DefaultAnomalySettings()
    intPtr := func(v int) *int { return &v }

    tests := []struct {
        name             string
        sig              AnomalySignal
        wantQuotaAnomaly bool
    }{
        {
            "upstream > 30s, zero output → quota suspected",
            AnomalySignal{OutputTokens: 0, DurationMs: 35000, UpstreamLatencyMs: intPtr(32000), StatusCode: 200},
            true,
        },
        {
            "upstream > 30s, output non-zero → not quota",
            AnomalySignal{OutputTokens: 100, DurationMs: 35000, UpstreamLatencyMs: intPtr(32000), StatusCode: 200},
            false,
        },
        {
            "upstream nil → not quota (data unavailable)",
            AnomalySignal{OutputTokens: 0, DurationMs: 35000, UpstreamLatencyMs: nil, StatusCode: 200},
            false,
        },
        {
            "upstream 10s, zero output → not quota (just slow/zero-token)",
            AnomalySignal{OutputTokens: 0, DurationMs: 12000, UpstreamLatencyMs: intPtr(10000), StatusCode: 200},
            false,
        },
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            types := detectAnomalies(tc.sig, settings)
            found := false
            for _, at := range types {
                if at == AnomalyQuotaExhaustionSuspected {
                    found = true
                }
            }
            if found != tc.wantQuotaAnomaly {
                t.Errorf("quota_exhaustion_suspected=%v, want %v; types=%v", found, tc.wantQuotaAnomaly, types)
            }
        })
    }
}
```

- [ ] **Step 6.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestDetectAnomalies_QuotaExhaustionSuspected -v
```

Expected: `FAIL` — `AnomalySignal` and `AnomalyQuotaExhaustionSuspected` undefined.

- [ ] **Step 6.3: Add `AnomalySignal`, update `detectAnomalies`, add `UpstreamLatencyMs` to `RequestLogInput`**

In `anomaly_service.go`:

Add new constant:

```go
AnomalyQuotaExhaustionSuspected AnomalyType = "quota_exhaustion_suspected"
```

Add `AnomalySignal` struct:

```go
// AnomalySignal bundles all observable signals for anomaly classification.
// Kept internal to anomaly_service.go; constructed by WriteAnomalyLog from
// existing parameters plus any new optional fields in RequestLogInput.
type AnomalySignal struct {
	InputTokens       int
	OutputTokens      int
	DurationMs        int64
	// UpstreamLatencyMs is the time (ms) from sending the upstream request to the
	// first response byte.  Nil when not available (e.g. non-Copilot paths).
	UpstreamLatencyMs *int
	StatusCode        int
}
```

Add `UpstreamLatencyMs *int` to `RequestLogInput` (zero-cost for existing callers):

```go
// UpstreamLatencyMs, when non-nil, is used for quota-exhaustion anomaly detection.
// Callers that don't have this data leave it nil.
UpstreamLatencyMs *int
```

Update `detectAnomalies` to accept `AnomalySignal`:

```go
func detectAnomalies(sig AnomalySignal, settings *AnomalySettings) []AnomalyType {
	var types []AnomalyType

	if settings.DetectZeroToken && sig.InputTokens == 0 && sig.OutputTokens == 0 {
		types = append(types, AnomalyZeroToken)
	}

	if sig.DurationMs > settings.TimeoutThresholdMs {
		types = append(types, AnomalyTimeout)
	} else if sig.DurationMs > settings.SlowRequestThresholdMs {
		types = append(types, AnomalySlowRequest)
	}

	// Quota exhaustion: upstream latency > 30 s AND zero output tokens.
	// Copilot silently throttles quota-exhausted requests: HTTP 200 after a long
	// stall, no content.  Gated on UpstreamLatencyMs (not total DurationMs) to
	// avoid false positives from large but legitimate slow responses.
	const quotaExhaustionUpstreamThresholdMs = 30_000
	if sig.UpstreamLatencyMs != nil &&
		int64(*sig.UpstreamLatencyMs) > quotaExhaustionUpstreamThresholdMs &&
		sig.OutputTokens == 0 {
		types = append(types, AnomalyQuotaExhaustionSuspected)
	}

	if sig.StatusCode >= 500 {
		types = append(types, AnomalyError)
	}

	return types
}
```

Update `WriteAnomalyLog` body to build `AnomalySignal` from existing params — **external signature unchanged**:

```go
func (s *AnomalyService) WriteAnomalyLog(
	ctx context.Context,
	inputTokens, outputTokens int,
	durationMs int64,
	statusCode int,
	input *RequestLogInput,
) {
	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()

	settings := s.GetSettings(bgCtx)

	// Build signal struct from call-site params + optional RequestLogInput fields.
	sig := AnomalySignal{
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		DurationMs:        durationMs,
		UpstreamLatencyMs: input.UpstreamLatencyMs,
		StatusCode:        statusCode,
	}
	anomalies := detectAnomalies(sig, settings)
	if len(anomalies) == 0 {
		return
	}
	// ... rest of function unchanged ...
```

- [ ] **Step 6.4: Wire `UpstreamLatencyMs` in Copilot handler `WriteAnomalyLog` calls**

In `copilot_gateway_handler.go`, at each of the 3 `WriteAnomalyLog` call sites, add `UpstreamLatencyMs` to the `RequestLogInput`:

```go
h.anomalyService.WriteAnomalyLog(
    recordCtx,
    capturedResult.Usage.PromptTokens,
    capturedResult.Usage.CompletionTokens,
    capturedResult.Duration.Milliseconds(),
    200,
    &service.RequestLogInput{
        RequestID:            requestID,
        UsageLogID:           usageLogIDPtr,
        UserID:               &userID,
        APIKeyID:             &apiKeyID,
        AccountID:            &accountID,
        GroupID:              apiKey.GroupID,
        RequestBody:          capturedReqBody,
        UpstreamRequestBody:  capturedUpstreamReqBody,
        UpstreamResponseBody: capturedUpstreamRespBody,
        UpstreamLatencyMs:    upstreamLatencyMsVal, // *int, already captured before goroutine
    },
)
```

OpenAI and Sora handlers continue passing `RequestLogInput` without `UpstreamLatencyMs` — they compile fine as Go zero-values `nil` for the new pointer field.

- [ ] **Step 6.5: Run test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestDetectAnomalies_QuotaExhaustionSuspected -v
```

Expected: `PASS`

- [ ] **Step 6.6: Build and run full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go test ./... -timeout 120s
```

Expected: no errors, all pass.

- [ ] **Step 6.7: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/anomaly_service.go internal/handler/copilot_gateway_handler.go
git commit -m "Feature: 新增 quota_exhaustion_suspected 异常，基于 upstream_latency_ms 检测 Copilot 配额耗尽"
```

---

## Task 7: Final verification

- [ ] **Step 7.1: Full build with vet and race detector**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go vet ./... && go test -race ./... -timeout 180s
```

Expected: no errors, no data races.

- [ ] **Step 7.2: Build tags coverage**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test -tags unit -race ./... -timeout 180s
```

Expected: unit-tagged tests also pass.

---

## Self-Review Checklist

**Spec coverage:**
- [x] Token = 0 (`/messages → /chat/completions`) → Tasks 1, 3
- [x] Token = 0 (`/chat/completions` direct) → Tasks 1, 2
- [x] Protocol preservation (usage chunk filtered for clients that didn't ask) → Task 2
- [x] `handleStreamingResponse` all 3 call sites updated → Task 2
- [x] Spans persisted for successful Copilot requests → Task 4
- [x] Context-aware token exchange → Task 5
- [x] `copilot.ExchangeToken` backward-compatible (copilot_oauth_service unaffected) → Task 5
- [x] `WriteAnomalyLog` external signature unchanged (0 callers to update) → Task 6
- [x] Quota exhaustion detection using real `UpstreamLatencyMs` → Task 6
- [x] `/messages → /responses` path regression test → Task 3

**Type consistency:**
- `ensureStreamIncludeUsage(body []byte) []byte` — defined Task 1, used Tasks 2, 3.
- `isUsageOnlyChunk(data string) bool` — defined Task 1, used Task 2.
- `handleStreamingResponse(..., forwardUsageChunk bool)` — updated Task 2, 3 call sites updated same task.
- `tokenExchanger func(context.Context, *http.Client, string) (*copilot.CopilotToken, error)` — Task 5.
- `AnomalySignal.UpstreamLatencyMs *int` — matches type from `getContextLatencyMsPtr` return value.
- `AnomalyQuotaExhaustionSuspected AnomalyType` — Task 6.
- `RequestLogInput.UpstreamLatencyMs *int` — Task 6.

**Risk summary:**
| Area | Risk level | Mitigation |
|------|-----------|-----------|
| Injecting `include_usage` upstream | Low | Standard OpenAI field, Copilot documented support; only present when stream=true |
| Usage-only chunk filtering | Low | `isUsageOnlyChunk` detects by structure, not position; content chunks with both choices+usage are never filtered |
| `handleStreamingResponse` sig change | Low | 3 call sites all updated in same task; compiler catches misses |
| `ExchangeToken` backward compat | None | Old function kept, delegates to new one |
| `WriteAnomalyLog` unchanged | None | No callers updated |
| `RequestLogInput.UpstreamLatencyMs` | None | New optional `*int` field; existing callers compile with nil zero-value |
