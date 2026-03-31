# Copilot Gateway Robustness Improvement Plan (v4)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix token-reporting zero values for Copilot streaming paths, persist OpsSpans for successful Copilot requests so the latency breakdown panel works, and harden token-refresh goroutine safety.

**Scope (explicitly bounded):**
- Token = 0 fix covers **two Copilot paths**: `/messages → /chat/completions` and `/chat/completions` direct. The `/messages → /responses` path already captures usage via terminal events (verified by existing test) — it is **not** touched in Task 2.
- Spans persistence covers **Copilot handler goroutines only** (ChatCompletions, Responses, Messages). OpenAI and Sora handlers out of scope.
- `WriteAnomalyLog` signature is **NOT changed**. Quota-exhaustion detection uses a new `AnomalySignal` struct internal to `anomaly_service.go`, plus `UpstreamLatencyMs *int` added to `RequestLogInput` (zero-cost optional field).
- `detectAnomalies` **signature change is backward-compatible**: V4 switches the signature to accept `AnomalySignal` AND updates **all existing callers** in `anomaly_service_test.go` in the same task.

**Architecture:** Four independent, incrementally-committable areas: (1) `ensureStreamIncludeUsage` + `isUsageOnlyChunk` helpers; (2) `ensureStreamIncludeUsage` injected **before `http.NewRequestWithContext`** in `ForwardChatCompletions` (L182) and `ForwardMessages` (L981); `handleStreamingResponse` gains `forwardUsageChunk bool` — all 3 call sites updated; (3) `RecordUsageInput.Spans []*OpsSpan` added + both `UsageLog` construction points in `RecordUsage` (L7615) assign `Spans` — `RecordUsageWithLongContext` (L7814) **also receives the same `Spans` field** added to `RecordUsageLongContextInput`; (4) `CopilotTokenProvider` uses injected `tokenExchanger` func with context propagation.

**Tech Stack:** Go 1.22+, `github.com/tidwall/sjson` / `gjson`, `bufio.Scanner`, `golang.org/x/sync/singleflight`, standard `net/http`, `github.com/gin-gonic/gin`.

---

## Verified Call-Site Counts (from codebase audit)

| Interface | Current callers | Callers updated in this plan |
|-----------|----------------|------------------------------|
| `handleStreamingResponse(c, resp, model, upstreamModel, startTime)` | 2 production (ForwardChatCompletions:L236, ForwardResponses:L901) + 1 test (test:L388) | All 3 |
| `detectAnomalies(inputTokens, outputTokens int, durationMs int64, statusCode int, settings)` | 1 production (WriteAnomalyLog:L213) + 8 tests (anomaly_service_test.go L15,27,39,51,59,67,80,85) | All 9 updated in Task 6 |
| `WriteAnomalyLog(ctx, in, out, dur, status, input)` | 6 total (copilot:3, openai:2, sora:1) | 0 — signature NOT changed |
| `copilot.ExchangeToken(httpClient, githubToken)` | 1 (`copilot_token_provider.go`) + 1 (`copilot_oauth_service.go`) | 0 — signature NOT changed; new `ExchangeTokenWithContext` added |
| `RecordUsageInput` struct | gateway_service.go:7196 | Add `Spans []*OpsSpan` |
| `RecordUsageLongContextInput` struct | gateway_service.go:7705 | Add `Spans []*OpsSpan` |
| `UsageLog.Spans` assignment | Two construction points in RecordUsage (L7615) and RecordUsageWithLongContext (L7814) | Both |

---

## File Map

| File | Change type | What changes |
|------|-------------|-------------|
| `internal/service/copilot_gateway_service.go` | Modify | Add `ensureStreamIncludeUsage`, `isUsageOnlyChunk`; inject `ensureStreamIncludeUsage` **before** `http.NewRequestWithContext` in `ForwardChatCompletions` and `ForwardMessages`; update `handleStreamingResponse` signature with `forwardUsageChunk bool`; update `ForwardChatCompletions` (L236), `ForwardResponses` (L901) call sites |
| `internal/service/copilot_gateway_service_test.go` | Modify/Add | Tests for `ensureStreamIncludeUsage`, `isUsageOnlyChunk`; integration tests for `/chat/completions` path; update existing `handleStreamingResponse` test call (L388) |
| `internal/service/gateway_service.go` | Modify | Add `Spans []*OpsSpan` to `RecordUsageInput` and `RecordUsageLongContextInput`; assign `MarshalOpsSpans(input.Spans)` to `UsageLog.Spans` at both construction points (L7615, L7814) |
| `internal/handler/copilot_gateway_handler.go` | Modify | Capture `GetOpsSpans(c)` before goroutine in all 3 handler loops; pass to `RecordUsageInput.Spans` |
| `internal/service/copilot_token_provider.go` | Modify | Replace direct `copilot.ExchangeToken` call with injected `tokenExchanger` func; use combined context timeout; add `newCopilotTokenProviderWithExchanger` constructor |
| `internal/pkg/copilot/token.go` | Modify | Add `ExchangeTokenWithContext(ctx, httpClient, githubToken)` — new function with context; update existing `ExchangeToken` to call it with `context.Background()` |
| `internal/service/anomaly_service.go` | Modify | Add `AnomalySignal` struct and `AnomalyQuotaExhaustionSuspected` const; change `detectAnomalies` signature to accept `AnomalySignal`; add `UpstreamLatencyMs *int` to `RequestLogInput`; update `WriteAnomalyLog` body to build `AnomalySignal` — external signature unchanged |
| `internal/service/anomaly_service_test.go` | Modify | Update all 8 existing `detectAnomalies(...)` calls to `detectAnomalies(AnomalySignal{...}, settings)` |

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

## Task 2: Inject `ensureStreamIncludeUsage` before upstream request + update `handleStreamingResponse`

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`
- Modify: `backend/internal/service/copilot_gateway_service_test.go`

### Critical insight

`ForwardChatCompletions` builds the upstream request at **line 182** (`http.NewRequestWithContext`). The body must be modified **before** that line. The `setOpsUpstreamRequestBody(c, body)` call at L197 happens after the request is built but before `httpClient.Do` — it must also come after `ensureStreamIncludeUsage` so the stored body reflects what was actually sent.

`ForwardMessages` injects `ensureStreamIncludeUsage` after `forceStreamTrue` (L981) and before `http.NewRequestWithContext` (around L1050 where the chat/completions request is built in the `/chat/completions` branch of `getSupportedEndpointsForModel`).

`ForwardResponses` (L901) uses `handleStreamingResponse` too, but `/responses` parses usage from `response.completed` terminal events already — `ensureStreamIncludeUsage` is **NOT** injected there to avoid unnecessary upstream protocol changes.

### Step-by-step

- [ ] **Step 2.1: Write integration test for ForwardChatCompletions usage-chunk filtering**

Add to `copilot_gateway_service_test.go`:

```go
func TestForwardChatCompletions_UsageChunkFilteredWhenClientDidNotRequest(t *testing.T) {
    var capturedBody []byte
    srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        capturedBody, _ = io.ReadAll(r.Body)
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
        // Copilot-injected usage-only chunk (present because include_usage was injected upstream).
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

    // Root-cause verification: upstream body must have include_usage injected.
    if !gjson.GetBytes(capturedBody, "stream_options.include_usage").Bool() {
        t.Errorf("expected stream_options.include_usage=true in upstream body; got %s", capturedBody)
    }

    // Token counts must be non-zero (from usage chunk parsed internally).
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

    // Client explicitly requests include_usage — usage chunk must be forwarded.
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

In `copilot_gateway_service.go`, change the function signature from (L244):

```go
func (s *CopilotGatewayService) handleStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	model string,
	upstreamModel string,
	startTime time.Time,
) (*CopilotForwardResult, error) {
```

to:

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
```

Inside the `for scanner.Scan()` loop, in the `if strings.HasPrefix(line, "data: ")` block, after `s.parseStreamUsage(data, usage)`, add:

```go
		// Filter the upstream-injected usage-only chunk when the client did not
		// request it.  Still parsed above so token counts are always recorded.
		if !forwardUsageChunk && isUsageOnlyChunk(data) {
			continue
		}
```

- [ ] **Step 2.4: Inject `ensureStreamIncludeUsage` in `ForwardChatCompletions` BEFORE the request is built**

In `ForwardChatCompletions`, after `body = clampCopilotUpstreamMaxTokens(body, account)` (around L128) and before the `AppendOpsSpan("translate.req")` call, add:

```go
	// Remember whether the client originally requested usage in the stream,
	// before ensureStreamIncludeUsage potentially adds it.
	clientWantsUsageChunk := gjson.GetBytes(body, "stream_options.include_usage").Bool()
	// Inject stream_options.include_usage=true so Copilot appends a usage-summary
	// SSE chunk.  Must happen before http.NewRequestWithContext builds the request body.
	body = ensureStreamIncludeUsage(body)
```

Update the streaming dispatch call at L236 from:

```go
		return s.handleStreamingResponse(c, resp, model, upstreamSent, startTime)
```

to:

```go
		return s.handleStreamingResponse(c, resp, model, upstreamSent, startTime, clientWantsUsageChunk)
```

Update the non-streaming dispatch at L240 (no `forwardUsageChunk` needed but `handleStreamingResponse` is not called there — no change needed for non-streaming path).

- [ ] **Step 2.5: Update `ForwardResponses` call site (L901)**

`ForwardResponses` uses `handleStreamingResponse` but does NOT inject `ensureStreamIncludeUsage` (the `/responses` path already parses usage from `response.completed`). Only the signature update is needed:

Change the call at L901 from:

```go
		result, fwdErr = s.handleStreamingResponse(c, resp, model, upstreamSent, startTime)
```

to:

```go
		// ForwardResponses reads usage from response.completed terminal event, not from
		// stream_options.include_usage.  forwardUsageChunk=false: no usage SSE chunk expected.
		result, fwdErr = s.handleStreamingResponse(c, resp, model, upstreamSent, startTime, false)
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
git commit -m "Fix: ForwardChatCompletions 注入 stream_options.include_usage 并过滤未请求的 usage chunk"
```

---

## Task 3: Inject `ensureStreamIncludeUsage` into `ForwardMessages` (`/chat/completions` branch)

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`
- Modify: `backend/internal/service/copilot_gateway_service_test.go`

### Step-by-step

- [ ] **Step 3.1: Write integration test for ForwardMessages `/chat/completions` branch**

```go
func TestForwardMessages_UpstreamBodyHasStreamIncludeUsage(t *testing.T) {
    var capturedBody []byte
    srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        capturedBody, _ = io.ReadAll(r.Body)
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        // Minimal OpenAI SSE chat/completions stream (ForwardMessages translates Anthropic → OpenAI).
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

    // Inject a model-endpoints cache entry so ForwardMessages routes to /chat/completions
    // (not /responses) without an actual /models API call.
    svc.setModelEndpointsForTest("claude-sonnet-4-5", []string{"chat/completions"})

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

**Note:** If `svc.setModelEndpointsForTest` does not exist, check whether `CopilotGatewayService` has a `modelEndpointsCache` or similar field that can be set directly in tests. Alternatively, use an existing test helper that sets up the model endpoint routing. If neither exists, simplify by calling `svc.forwardChatCompletions(...)` directly (it is unexported but accessible in same-package tests) with a pre-built OpenAI body that already has `stream:true`.

- [ ] **Step 3.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestForwardMessages_UpstreamBodyHasStreamIncludeUsage" -v
```

Expected: `FAIL`

- [ ] **Step 3.3: Inject `ensureStreamIncludeUsage` in `ForwardMessages` after `forceStreamTrue`**

In `copilot_gateway_service.go`, find (around line 981):

```go
	openAIBody = forceStreamTrue(openAIBody)
```

Change to:

```go
	openAIBody = forceStreamTrue(openAIBody)
	// Ensure Copilot returns token-usage statistics in the final SSE chunk.
	// Must run after forceStreamTrue so stream=true is already set in the body.
	openAIBody = ensureStreamIncludeUsage(openAIBody)
```

- [ ] **Step 3.4: Run integration test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestForwardMessages_UpstreamBodyHasStreamIncludeUsage" -v
```

Expected: `PASS`

- [ ] **Step 3.5: Run full service tests**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -timeout 120s
```

Expected: all pass.

- [ ] **Step 3.6: Commit**

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
- Add: `backend/internal/service/copilot_spans_test.go`

### Why

`UsageLog.Spans *string` exists. `MarshalOpsSpans` exists. `RecordUsageInput` and `RecordUsageLongContextInput` both lack a `Spans` field. The two `UsageLog{}` construction points in `RecordUsage` (L7615) and `RecordUsageWithLongContext` (L7814) neither assign `Spans`. Adding the field and wiring it through makes the latency breakdown panel work.

**Important:** Both structs must receive the same `Spans []*OpsSpan` field to keep both construction points compilable. `RecordUsageWithLongContext` is used by Gemini handlers — adding the field is zero-cost for existing callers (nil by default).

### Step-by-step

- [ ] **Step 4.1: Write test for span persistence**

Create `backend/internal/service/copilot_spans_test.go`:

```go
//go:build unit

package service

import (
    "context"
    "strings"
    "testing"
    "time"
)

func TestRecordUsage_CopilotSpansPersistedToUsageLog(t *testing.T) {
    var savedLog *UsageLog
    usageRepo := &openAIRecordUsageBestEffortLogRepoStub{
        bestEffortErr: nil,
    }
    usageRepo.saveFn = func(_ context.Context, log *UsageLog) error {
        savedLog = log
        return nil
    }
    userRepo := &openAIRecordUsageUserRepoStub{}
    subRepo := &openAIRecordUsageSubRepoStub{}
    svc := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, subRepo)

    spans := []*OpsSpan{
        {Name: "token.fetch", StartUnixMs: 1000, DurationMs: 50, Status: "ok"},
        {Name: "upstream.post", StartUnixMs: 1050, DurationMs: 800, Status: "ok"},
    }

    _, _, err := svc.RecordUsage(context.Background(), &RecordUsageInput{
        Result: &ForwardResult{
            Model:    "gpt-4o",
            Duration: time.Second,
            Usage:    ForwardUsage{InputTokens: 10, OutputTokens: 5},
        },
        APIKey:  &APIKey{ID: 1001},
        User:    &User{ID: 2001},
        Account: &Account{ID: 3001, Platform: PlatformCopilot},
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

**Note before writing:** Read `openai_gateway_record_usage_test.go` lines 52–75 to verify exact field names of `openAIRecordUsageBestEffortLogRepoStub`. If it does not have a `saveFn` callback, replace with checking `usageRepo.lastLog`. Also verify `ForwardResult`, `ForwardUsage` type names from `gateway_service.go`.

- [ ] **Step 4.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test -tags unit ./internal/service/... -run TestRecordUsage_CopilotSpansPersistedToUsageLog -v
```

Expected: `FAIL` — `RecordUsageInput` has no `Spans` field.

- [ ] **Step 4.3: Add `Spans` to both input structs in `gateway_service.go`**

In `RecordUsageInput` (around L7219 after `Initiator string`), add:

```go
	// Spans holds per-phase timing events collected by AppendOpsSpan during the request.
	// When non-nil, serialised via MarshalOpsSpans and stored in usage_logs.spans.
	Spans []*OpsSpan
```

In `RecordUsageLongContextInput` (around L7730 after `Initiator string`), add:

```go
	// Spans holds per-phase timing events collected by AppendOpsSpan during the request.
	Spans []*OpsSpan
```

- [ ] **Step 4.4: Assign `Spans` in both `UsageLog` construction points**

**Point 1** — `RecordUsage` (around L7654, just before `CreatedAt: time.Now()`):

```go
		Spans:             MarshalOpsSpans(input.Spans),
		CreatedAt:         time.Now(),
```

**Point 2** — `RecordUsageWithLongContext` (around L7849, same pattern):

```go
		Spans:             MarshalOpsSpans(input.Spans),
		CreatedAt:         time.Now(),
```

- [ ] **Step 4.5: Capture spans before goroutine in Copilot handler**

In `copilot_gateway_handler.go`, for each of the 3 recording goroutines (ChatCompletions, Responses, Messages), add span capture before the goroutine launch:

```go
	// Capture span slice before entering goroutine.
	// gin.Context is not safe to use across goroutine boundaries; copy the slice header here.
	capturedSpans := make([]*service.OpsSpan, len(service.GetOpsSpans(c)))
	copy(capturedSpans, service.GetOpsSpans(c))
```

Then in the goroutine's `RecordUsage` call, add:

```go
		_, _, err := h.gatewayService.RecordUsage(recordCtx, &service.RecordUsageInput{
			// ... existing fields ...
			Spans: capturedSpans,
		})
```

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
- Add/Modify: `backend/internal/service/copilot_token_provider_test.go`

### Step-by-step

- [ ] **Step 5.1: Write tests**

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

    ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancel()

    errCh := make(chan error, 1)
    go func() {
        _, err := provider.GetAccessToken(ctx, &Account{
            ID:          1,
            Platform:    PlatformCopilot,
            Credentials: map[string]any{"github_token": "ghp_test"},
        })
        errCh <- err
    }()

    // Wait until server is blocking before we time out.
    select {
    case <-hanging:
        // server is now blocking; let the context expire
    case <-time.After(2 * time.Second):
        t.Fatal("server never received request")
    }

    select {
    case err := <-errCh:
        if err == nil {
            t.Fatal("expected error on context cancellation, got nil")
        }
    case <-time.After(2 * time.Second):
        t.Fatal("GetAccessToken did not return within 2s after context cancelled")
    }
}

func TestGetAccessToken_LogsExchangeLatency(t *testing.T) {
    var logBuf bytes.Buffer
    logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
    origLogger := slog.Default()
    slog.SetDefault(logger)
    t.Cleanup(func() { slog.SetDefault(origLogger) })

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

In `internal/pkg/copilot/token.go`, **add** a new function (do not change existing `ExchangeToken`). First read the file to identify existing header constants (`DefaultEditorVersion`, `DefaultEditorPluginVersion`, `DefaultUserAgent`, `DefaultGitHubAPIVersion`) and the response struct name (`TokenExchangeResponse`). Then add:

```go
// ExchangeTokenWithContext exchanges a GitHub personal access token for a short-lived
// Copilot API token.  ctx controls the HTTP exchange — cancelling it aborts the call.
// Existing callers should migrate to this function; ExchangeToken delegates here.
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
	return buildCopilotToken(tokenResp), nil
}
```

**Note:** Read `token.go` first. If the existing `ExchangeToken` function has inline token-building logic, extract it to a `buildCopilotToken(r TokenExchangeResponse) *CopilotToken` helper shared by both functions to avoid duplication. If a `buildCopilotToken` already exists, use it. If `TokenExchangeResponse` has different field names, match them exactly.

Then update the existing `ExchangeToken`:

```go
// ExchangeToken is a convenience wrapper for ExchangeTokenWithContext using
// context.Background().  Existing callers (copilot_oauth_service, etc.) are
// unaffected; new callers should prefer ExchangeTokenWithContext.
func ExchangeToken(httpClient *http.Client, githubToken string) (*CopilotToken, error) {
	return ExchangeTokenWithContext(context.Background(), httpClient, githubToken)
}
```

- [ ] **Step 5.4: Add `tokenExchanger` type and update `CopilotTokenProvider`**

In `copilot_token_provider.go`:

```go
// tokenExchanger is the function type for exchanging a GitHub token for a Copilot token.
// Injected at construction time; production uses copilot.ExchangeTokenWithContext.
type tokenExchanger func(ctx context.Context, httpClient *http.Client, githubToken string) (*copilot.CopilotToken, error)
```

Update `CopilotTokenProvider` struct to add the `exchange` field:

```go
type CopilotTokenProvider struct {
	httpClient *http.Client
	exchange   tokenExchanger

	mu     sync.RWMutex
	tokens map[int64]*copilot.CopilotToken

	sfGroup singleflight.Group
}
```

Update `NewCopilotTokenProvider` to use the new constructor:

```go
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

In the singleflight body inside `GetAccessToken`, replace the existing `copilot.ExchangeToken` call with:

```go
		exchangeStart := time.Now()
		// Use a combined context: respect caller cancellation but cap at 20 s
		// to avoid holding the goroutine forever on a slow GitHub API.
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

**Note:** Read `copilot_token_provider.go` before making changes to understand the exact singleflight body structure and variable names. The `fallbackToken` variable may have a different name in the actual code.

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

## Task 6: `quota_exhaustion_suspected` anomaly — update `detectAnomalies` signature + all callers

**Files:**
- Modify: `backend/internal/service/anomaly_service.go`
- Modify: `backend/internal/service/anomaly_service_test.go` — **update all 8 existing test calls**
- Modify: `backend/internal/handler/copilot_gateway_handler.go`

### Why

The `detectAnomalies` signature change from 5 positional params to `AnomalySignal` struct is purely internal (package-private function). However, it has **8 existing call sites in `anomaly_service_test.go`** that must all be updated in the same commit to keep the build green.

The external `WriteAnomalyLog` signature (6 callers across 3 files) is NOT changed. It constructs `AnomalySignal` internally from its existing params + the new `RequestLogInput.UpstreamLatencyMs` optional field.

### Step-by-step

- [ ] **Step 6.1: Write test for quota-exhaustion detection**

Add to `backend/internal/service/anomaly_service_test.go`:

```go
func TestDetectAnomalies_QuotaExhaustionSuspected(t *testing.T) {
    settings := &AnomalySettings{
        SlowRequestThresholdMs: 20000,
        TimeoutThresholdMs:     60000,
        DetectZeroToken:        true,
    }
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
            "upstream 10s, zero output → not quota",
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

- [ ] **Step 6.3: Add `AnomalySignal`, `AnomalyQuotaExhaustionSuspected`, update `detectAnomalies`, update `WriteAnomalyLog`, add `UpstreamLatencyMs` to `RequestLogInput`**

In `anomaly_service.go`:

Add new constant (next to existing `AnomalyError`):

```go
AnomalyQuotaExhaustionSuspected AnomalyType = "quota_exhaustion_suspected"
```

Add `AnomalySignal` struct (after the constants block):

```go
// AnomalySignal bundles all observable signals for anomaly classification.
// It is an internal type used only within anomaly_service.go.
// Constructed by WriteAnomalyLog from its positional parameters plus
// optional fields from RequestLogInput.
type AnomalySignal struct {
	InputTokens  int
	OutputTokens int
	DurationMs   int64
	// UpstreamLatencyMs is the time (ms) from sending the upstream request to the
	// first response byte.  Nil when not available (e.g. non-Copilot paths).
	UpstreamLatencyMs *int
	StatusCode        int
}
```

Replace the `detectAnomalies` function signature and body:

```go
// detectAnomalies is a pure function that computes which anomaly types apply for a
// completed request.  It accepts an AnomalySignal so new signal fields can be added
// without changing the external WriteAnomalyLog call sites.
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

Update `WriteAnomalyLog` body to build `AnomalySignal` (external signature unchanged):

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
	// ... rest of function unchanged (logInput construction, repo call, etc.) ...
```

Add `UpstreamLatencyMs *int` to `RequestLogInput` (the struct in the same file or wherever it's defined — check with `grep -n "RequestLogInput" ./internal/service/*.go`):

```go
// UpstreamLatencyMs, when non-nil, is used for quota-exhaustion anomaly detection.
// Callers that don't have this data leave it nil.
UpstreamLatencyMs *int
```

- [ ] **Step 6.4: Update all 8 existing `detectAnomalies` calls in `anomaly_service_test.go`**

The 8 call sites currently use the old 5-parameter signature. Each must be rewritten to pass an `AnomalySignal{}`:

| Old call | Replacement |
|----------|-------------|
| `detectAnomalies(0, 0, 5000, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:0, OutputTokens:0, DurationMs:5000, StatusCode:200}, settings)` |
| `detectAnomalies(100, 200, 25000, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:100, OutputTokens:200, DurationMs:25000, StatusCode:200}, settings)` |
| `detectAnomalies(0, 0, 70000, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:0, OutputTokens:0, DurationMs:70000, StatusCode:200}, settings)` |
| `detectAnomalies(100, 200, 1000, 500, settings)` | `detectAnomalies(AnomalySignal{InputTokens:100, OutputTokens:200, DurationMs:1000, StatusCode:500}, settings)` |
| `detectAnomalies(100, 200, 5000, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:100, OutputTokens:200, DurationMs:5000, StatusCode:200}, settings)` |
| `detectAnomalies(0, 0, 5000, 200, settings)` (ZeroTokenDisabled) | `detectAnomalies(AnomalySignal{InputTokens:0, OutputTokens:0, DurationMs:5000, StatusCode:200}, settings)` |
| `detectAnomalies(100, 200, 20000, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:100, OutputTokens:200, DurationMs:20000, StatusCode:200}, settings)` |
| `detectAnomalies(100, 200, 20001, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:100, OutputTokens:200, DurationMs:20001, StatusCode:200}, settings)` |

Make all 8 replacements in `anomaly_service_test.go`.

- [ ] **Step 6.5: Build verification after all changes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

Expected: no compilation errors.

- [ ] **Step 6.6: Run anomaly tests**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestDetectAnomalies" -v
```

Expected: all pass.

- [ ] **Step 6.7: Wire `UpstreamLatencyMs` in Copilot handler `WriteAnomalyLog` calls**

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
        UpstreamLatencyMs:    upstreamLatencyMsVal, // *int from getContextLatencyMsPtr
    },
)
```

OpenAI and Sora handlers continue passing `RequestLogInput` without `UpstreamLatencyMs` — the new pointer field zero-values to `nil`, no change needed.

- [ ] **Step 6.8: Run full test suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go test ./... -timeout 120s
```

Expected: no errors, all pass.

- [ ] **Step 6.9: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/anomaly_service.go internal/service/anomaly_service_test.go \
        internal/handler/copilot_gateway_handler.go
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
- [x] `ensureStreamIncludeUsage` injected BEFORE `http.NewRequestWithContext` (root cause verified by `capturedBody` assertion) → Tasks 2, 3
- [x] Protocol preservation (usage chunk filtered for clients that didn't ask) → Task 2
- [x] `handleStreamingResponse` all 3 call sites updated → Task 2
- [x] Spans persisted for successful Copilot requests → Task 4
- [x] Both `RecordUsageInput` AND `RecordUsageLongContextInput` receive `Spans` → Task 4
- [x] Context-aware token exchange → Task 5
- [x] `copilot.ExchangeToken` backward-compatible (copilot_oauth_service unaffected) → Task 5
- [x] `WriteAnomalyLog` external signature unchanged (0 callers to update) → Task 6
- [x] `detectAnomalies` signature change → all 8 existing test calls updated in same task → Task 6
- [x] Quota exhaustion detection using real `UpstreamLatencyMs` → Task 6

**Type consistency:**
- `ensureStreamIncludeUsage(body []byte) []byte` — defined Task 1, used Tasks 2, 3.
- `isUsageOnlyChunk(data string) bool` — defined Task 1, used Task 2.
- `handleStreamingResponse(..., forwardUsageChunk bool)` — updated Task 2, ForwardResponses:L901 updated same task.
- `tokenExchanger func(context.Context, *http.Client, string) (*copilot.CopilotToken, error)` — Task 5.
- `AnomalySignal.UpstreamLatencyMs *int` — matches type from `getContextLatencyMsPtr` return value (`*int`).
- `AnomalyQuotaExhaustionSuspected AnomalyType` — Task 6.
- `RequestLogInput.UpstreamLatencyMs *int` — Task 6.
- `RecordUsageInput.Spans []*OpsSpan` AND `RecordUsageLongContextInput.Spans []*OpsSpan` — Task 4.

**Risk summary:**
| Area | Risk level | Mitigation |
|------|-----------|-----------|
| Injecting `include_usage` upstream | Low | Standard OpenAI field, Copilot documented support; only present when stream=true |
| Usage-only chunk filtering | Low | `isUsageOnlyChunk` detects by structure, not position; content chunks with both choices+usage are never filtered |
| `handleStreamingResponse` sig change | Low | 3 call sites all updated in same task; compiler catches misses |
| `detectAnomalies` sig change | Low | All 9 callers (1 prod + 8 tests) updated in same commit |
| `ExchangeToken` backward compat | None | Old function delegates to new one |
| `WriteAnomalyLog` unchanged | None | No callers updated |
| `RequestLogInput.UpstreamLatencyMs` | None | New optional `*int` field; existing callers compile with nil zero-value |
| Both `RecordUsage*Input` structs updated | None | Additive field change; no existing callers broken |
