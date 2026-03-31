# Copilot Gateway Robustness Improvement Plan (v2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix token-reporting zero values, surface structured per-phase latency in the ops dashboard for ALL successful requests, and harden token-refresh concurrency to prevent goroutine stalls on context cancellation.

**Architecture:** Four independent, incrementally-committable fix areas: (1) inject `stream_options.include_usage` in every streaming upstream request — with protocol-preserving filtering so clients that didn't ask for usage don't receive an extra chunk; (2) persist accumulated `OpsSpan` data into `usage_logs.spans` on successful requests so the latency breakdown panel works without an anomaly; (3) refactor `CopilotTokenProvider` to inject the exchange function via a function-type dependency so context propagation and test isolation are clean; (4) add `quota_exhaustion_suspected` anomaly detection gated on the real `UpstreamLatencyMs` field that is already captured by `RecordUsageInput`. The `/messages → /responses` branch is explicitly excluded as it already captures usage via terminal events; its correctness is verified by a regression test added in Task 1.

**Tech Stack:** Go 1.22+, `github.com/tidwall/sjson` / `gjson`, `bufio.Scanner`, `golang.org/x/sync/singleflight`, standard `net/http`, `github.com/gin-gonic/gin`.

---

## Root-Cause Summary (corrected after v1 review)

### Problem 1 — Token Counts Are 0

**Root cause A — `/messages → /chat/completions` sub-path:**

`ForwardMessages()` calls `forceStreamTrue()` to convert `stream=false → stream=true` before forwarding to Copilot. It does **not** inject `stream_options.include_usage: true`. GitHub Copilot's OpenAI-compatible SSE endpoint only appends the usage summary chunk when `stream_options.include_usage` is explicitly `true`. Without it, `parseStreamUsage` scans the entire SSE stream without encountering a chunk with `prompt_tokens > 0`, leaving `usage` at zero.

**Root cause B — `/chat/completions` direct path (OpenAI-mode clients):**

`ForwardChatCompletions()` passes the body through after model-rewrite and max-token clamp. If the client omits `stream_options.include_usage`, Copilot omits usage in the stream.

**NOT affected:** The `/messages → /responses` sub-path (via `forwardMessagesViaResponses`) already reads `input_tokens` / `output_tokens` from `response.completed` terminal events. **No change needed there** — but a regression test is added to confirm.

### Problem 2 — Latency Spans Not Visible for Successful Requests

`AppendOpsSpan` / `GetOpsSpans` accumulate spans into the gin context correctly. The spans are already comprehensive (`token.fetch`, `upstream.post`, `failover.select`, etc.). However, `RecordUsageInput` has no `Spans` field, and `writeUsageLogBestEffort` never populates `UsageLog.Spans`. Result: spans are only persisted for anomalous requests (via `ops_error_logger`), not for successful ones. Operators cannot see the latency breakdown panel for a normal request.

**Fix:** Add `Spans []*OpsSpan` to `RecordUsageInput`, capture them from the gin context before entering the recording goroutine, and pass them through to `UsageLog.Spans` via `MarshalOpsSpans`.

### Problem 3 — Token Exchange Does Not Respect Context Cancellation

`CopilotTokenProvider.GetAccessToken` calls `copilot.ExchangeToken` which uses `http.NewRequest` (no context). If the caller's context is cancelled (e.g. client disconnect, 10-s `recordCtx` fires), the exchange HTTP call continues for up to 30 s, leaking a goroutine and a connection.

**Fix:** Replace the bare function call with an injected `tokenExchanger func(context.Context, *http.Client, string) (*copilot.CopilotToken, error)` dependency. The production implementation calls `copilot.ExchangeToken` with a combined timeout (`context.WithTimeout(ctx, 20*time.Second)`). Tests inject a mock. No global URL override needed.

### Problem 4 — Quota Exhaustion Not Detected

When Copilot silently throttles a request (premium interaction quota exhausted), `upstream_latency_ms` exceeds 30 000 ms and `output_tokens == 0`, but no anomaly is emitted because `detectAnomalies` only uses `durationMs` (total request time) — which includes auth, routing, and response streaming — not `upstreamLatencyMs`. The `UpstreamLatencyMs` field already exists in `RecordUsageInput` and `UsageLog`. The fix is to thread it into `detectAnomalies` via a new `AnomalySignal` struct.

---

## File Map

| File | Change type | What changes |
|------|-------------|-------------|
| `internal/service/copilot_gateway_service.go` | Modify | Add `ensureStreamIncludeUsage`; call it in `ForwardChatCompletions` + `ForwardMessages`; add `clientWantsUsageChunk bool` parameter to `handleStreamingResponse` so usage-only chunks are filtered out for clients that didn't request them; add `isUsageOnlyChunk` helper |
| `internal/service/copilot_gateway_service_test.go` | Modify/Add | Tests for `ensureStreamIncludeUsage`; integration tests for both streaming paths; `/responses` regression test |
| `internal/service/gateway_service.go` | Modify | Add `Spans []*OpsSpan` to `RecordUsageInput`; pass `MarshalOpsSpans(input.Spans)` to `UsageLog.Spans` in `writeUsageLogBestEffort` |
| `internal/handler/copilot_gateway_handler.go` | Modify | Capture `GetOpsSpans(c)` before goroutine; pass to `RecordUsageInput.Spans` |
| `internal/service/copilot_token_provider.go` | Modify | Replace bare `copilot.ExchangeToken` call with injected `tokenExchanger` func; use combined context timeout |
| `internal/pkg/copilot/token.go` | Modify | Add `ExchangeToken(ctx context.Context, ...)` overload accepting context; keep `ExchangeToken` wrapper for backward compat |
| `internal/service/anomaly_service.go` | Modify | Replace `detectAnomalies` signature with `AnomalySignal` struct; add `quota_exhaustion_suspected` detection using `UpstreamLatencyMs`; update `WriteAnomalyLog` call sites |
| `internal/handler/copilot_gateway_handler.go` | Modify (also) | Pass `UpstreamLatencyMs` to `WriteAnomalyLog` via updated `AnomalySignal` |

---

## Task 1: `ensureStreamIncludeUsage` + protocol-preserving filter

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`
- Modify: `backend/internal/service/copilot_gateway_service_test.go`

### Why

Adding `stream_options.include_usage: true` to the upstream request makes Copilot append a usage-summary chunk at the end of the SSE stream. However, for clients that did **not** request usage (`clientWantsUsageChunk == false`), forwarding that extra chunk is a visible protocol change that breaks simple SSE parsers. The fix is to: (a) always request usage from upstream, (b) remember whether the client wanted it before we overrode the body, (c) filter the extra chunk out of the downstream stream when the client didn't ask for it.

An `isUsageOnlyChunk` helper detects usage-only chunks by checking whether the SSE data JSON has `"usage"` populated but `"choices"` absent or empty — matching OpenAI's convention for the trailing usage chunk.

### Step-by-step

- [ ] **Step 1.1: Write failing test for `ensureStreamIncludeUsage`**

Add to `backend/internal/service/copilot_gateway_service_test.go`:

```go
func TestEnsureStreamIncludeUsage(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantVal string // expected gjson value of stream_options.include_usage; "" means absent
    }{
        {"no stream_options, stream true",
            `{"model":"gpt-4o","stream":true}`, "true"},
        {"stream_options empty object",
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
                    t.Errorf("want absent, got %s; body=%s", val.Raw, got)
                }
                return
            }
            if !val.Exists() || val.Raw != tc.wantVal {
                t.Errorf("want stream_options.include_usage=%s, got %q; body=%s", tc.wantVal, val.Raw, got)
            }
        })
    }
}
```

- [ ] **Step 1.2: Write failing test for `isUsageOnlyChunk`**

```go
func TestIsUsageOnlyChunk(t *testing.T) {
    tests := []struct {
        name string
        data string
        want bool
    }{
        {"usage chunk no choices",
            `{"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`, true},
        {"usage chunk empty choices",
            `{"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5}}`, true},
        {"content chunk with choices",
            `{"choices":[{"delta":{"content":"hi"}}]}`, false},
        {"content chunk with usage (some providers)",
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

- [ ] **Step 1.3: Run tests — confirm they fail**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestEnsureStreamIncludeUsage|TestIsUsageOnlyChunk" -v
```

Expected: `FAIL` — `ensureStreamIncludeUsage` and `isUsageOnlyChunk` undefined.

- [ ] **Step 1.4: Implement both helpers**

In `copilot_gateway_service.go`, add after `forceStreamTrue` (around line 1841):

```go
// ensureStreamIncludeUsage injects "stream_options":{"include_usage":true} into the
// request body when stream=true, so the Copilot API appends a usage-summary chunk at
// the end of the SSE stream.  Without this field Copilot omits the chunk and all
// token counts are recorded as zero.
//
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
//
// Used to filter the chunk out of the downstream stream when the original client
// request did not include stream_options.include_usage, preserving protocol fidelity.
func isUsageOnlyChunk(data string) bool {
	usageResult := gjson.Get(data, "usage")
	if !usageResult.Exists() {
		return false
	}
	choicesResult := gjson.Get(data, "choices")
	// Chunk is usage-only if choices is absent or is an empty array.
	return !choicesResult.Exists() || (choicesResult.IsArray() && len(choicesResult.Array()) == 0)
}
```

- [ ] **Step 1.5: Run tests — confirm they pass**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestEnsureStreamIncludeUsage|TestIsUsageOnlyChunk" -v
```

Expected: all 6 + 5 = 11 sub-cases `PASS`.

- [ ] **Step 1.6: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_gateway_service.go internal/service/copilot_gateway_service_test.go
git commit -m "Feature: 添加 ensureStreamIncludeUsage 和 isUsageOnlyChunk 辅助函数"
```

---

## Task 2: Wire `ensureStreamIncludeUsage` into `ForwardMessages` (chat/completions sub-path)

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`

### Why

`ForwardMessages` calls `forceStreamTrue` but never injects `include_usage`. This is the primary cause of `token = 0` for Claude Code (Anthropic-format) clients when the model routes through `/chat/completions`. The fix runs `ensureStreamIncludeUsage` immediately after `forceStreamTrue`.

Because the client is using Anthropic format, it never sees the OpenAI SSE stream directly — the streaming response is re-translated to Anthropic SSE events by `handleMessagesStreamingResponse`. The usage-only chunk is consumed by `parseStreamUsage` internally and is never forwarded to the client, so no filtering is needed on this path.

- [ ] **Step 2.1: Write integration test**

Add to `copilot_gateway_service_test.go`:

```go
func TestForwardMessages_UpstreamBodyHasStreamIncludeUsage(t *testing.T) {
    var capturedBody []byte
    srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        capturedBody, _ = io.ReadAll(r.Body)
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        // Minimal Anthropic-translated SSE: message_start + content + message_stop.
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
        ID:       1,
        Platform: PlatformCopilot,
        Type:     AccountTypeAPIKey,
        Credentials: map[string]any{
            "github_token": "ghp_test",
            "base_url":     srv.URL,
        },
    }

    gin.SetMode(gin.TestMode)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

    // stream=false Anthropic request — ForwardMessages will force stream=true upstream.
    anthropicBody := []byte(`{"model":"claude-sonnet-4-5","stream":false,"max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`)
    result, err := svc.ForwardMessages(context.Background(), c, account, anthropicBody)
    if err != nil {
        t.Fatalf("ForwardMessages: %v", err)
    }

    // Upstream body must have stream=true AND include_usage=true.
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

- [ ] **Step 2.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestForwardMessages_UpstreamBodyHasStreamIncludeUsage -v
```

Expected: `FAIL` — `stream_options.include_usage` absent.

- [ ] **Step 2.3: Add the call in `ForwardMessages`**

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

- [ ] **Step 2.4: Run test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestForwardMessages_UpstreamBodyHasStreamIncludeUsage -v
```

Expected: `PASS`

- [ ] **Step 2.5: Run full service tests**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -timeout 120s
```

Expected: all pass.

- [ ] **Step 2.6: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_gateway_service.go internal/service/copilot_gateway_service_test.go
git commit -m "Fix: ForwardMessages 注入 stream_options.include_usage 修复 token 计数为 0"
```

---

## Task 3: Wire `ensureStreamIncludeUsage` into `ForwardChatCompletions` with client-protocol preservation

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`

### Why

`ForwardChatCompletions` is the direct OpenAI-proxy path. Here the SSE stream is forwarded verbatim to the client. If we inject `include_usage` upstream but the client didn't request it, the client will receive an extra usage-only chunk that it didn't expect. We must: (a) record whether the client requested usage before we modify the body, (b) inject it unconditionally upstream for token tracking, (c) pass `clientWantsUsageChunk` to `handleStreamingResponse` which uses `isUsageOnlyChunk` to decide whether to forward the extra chunk.

- [ ] **Step 3.1: Write test for client-protocol preservation**

Add to `copilot_gateway_service_test.go`:

```go
func TestForwardChatCompletions_UsageChunkFilteredWhenClientDidNotRequest(t *testing.T) {
    srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
        // Usage-only chunk appended by Copilot when include_usage=true.
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

    // Token counts must be non-zero (usage was injected upstream).
    if result.Usage == nil || result.Usage.PromptTokens == 0 {
        t.Errorf("expected non-zero PromptTokens; got %+v", result.Usage)
    }

    // Client response must NOT contain the usage-only chunk.
    body := w.Body.String()
    if strings.Contains(body, `"prompt_tokens"`) {
        t.Errorf("usage-only chunk must be filtered from client response; got: %s", body)
    }
    // Client response must contain the content chunk.
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

    // Client explicitly requests stream_options.include_usage.
    clientBody := []byte(`{"model":"gpt-4o","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`)
    result, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
    if err != nil {
        t.Fatalf("ForwardChatCompletions: %v", err)
    }

    if result.Usage == nil || result.Usage.PromptTokens == 0 {
        t.Errorf("expected non-zero PromptTokens; got %+v", result.Usage)
    }

    // Client SHOULD receive the usage chunk this time.
    body := w.Body.String()
    if !strings.Contains(body, `"prompt_tokens"`) {
        t.Errorf("usage chunk must be forwarded when client requested it; got: %s", body)
    }
}
```

- [ ] **Step 3.2: Run tests — confirm they fail**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestForwardChatCompletions_UsageChunk" -v
```

Expected: `FAIL`

- [ ] **Step 3.3: Update `ForwardChatCompletions` and `handleStreamingResponse`**

In `ForwardChatCompletions`, in the preprocessing block, add after `clampCopilotUpstreamMaxTokens`:

```go
	// Remember whether the client originally requested usage in the stream,
	// BEFORE we potentially override stream_options.include_usage below.
	// This determines whether the upstream-injected usage chunk is forwarded downstream.
	clientWantsUsageChunk := gjson.GetBytes(body, "stream_options.include_usage").Bool()

	// Ensure Copilot returns token-usage statistics in the final SSE chunk,
	// regardless of what the client sent.
	body = ensureStreamIncludeUsage(body)
```

Then change the streaming dispatch call from:

```go
	return s.handleStreamingResponse(c, resp, model, upstreamSent, startTime)
```

to:

```go
	return s.handleStreamingResponse(c, resp, model, upstreamSent, startTime, clientWantsUsageChunk)
```

Update `handleStreamingResponse` signature and filtering logic:

```go
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

			// Filter the upstream-injected usage-only chunk when the client
			// did not request it.  Still parsed above for internal token tracking.
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

- [ ] **Step 3.4: Run tests — confirm they pass**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestForwardChatCompletions_UsageChunk" -v
```

Expected: both tests `PASS`.

- [ ] **Step 3.5: Build and run full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go test ./internal/service/... -timeout 120s
```

Expected: no errors, all pass.

- [ ] **Step 3.6: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_gateway_service.go internal/service/copilot_gateway_service_test.go
git commit -m "Fix: ForwardChatCompletions 注入 include_usage 并过滤客户端未请求的 usage chunk"
```

---

## Task 4: Persist OpsSpans for successful requests

**Files:**
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/handler/copilot_gateway_handler.go`

### Why

`GetOpsSpans` / `MarshalOpsSpans` exist and work correctly. `UsageLog.Spans *string` exists. But `RecordUsageInput` has no `Spans` field and `writeUsageLogBestEffort` never populates `UsageLog.Spans`. The latency breakdown panel in the ops dashboard is therefore invisible for successful requests. Adding `Spans` to `RecordUsageInput` and threading it through closes the gap with minimal invasiveness.

- [ ] **Step 4.1: Write test for span persistence**

Add to a test file for `gateway_service` (e.g., `gateway_record_usage_test.go`):

```go
func TestRecordUsage_SpansPersistedToUsageLog(t *testing.T) {
    // Use a capturing UsageLogRepository mock.
    var saved *UsageLog
    repo := &mockUsageLogRepo{
        saveFn: func(ctx context.Context, log *UsageLog) error {
            saved = log
            return nil
        },
    }
    svc := newGatewayServiceForTest(t, repo)

    spans := []*OpsSpan{
        {Name: "token.fetch", StartUnixMs: 1000, DurationMs: 50, Status: "ok"},
        {Name: "upstream.post", StartUnixMs: 1050, DurationMs: 800, Status: "ok"},
    }

    _, _, err := svc.RecordUsage(context.Background(), &RecordUsageInput{
        Result:  &ForwardResult{Model: "gpt-4o", Duration: time.Second},
        APIKey:  testAPIKey(),
        User:    testUser(),
        Account: testAccount(),
        Spans:   spans,
    })
    if err != nil {
        t.Fatalf("RecordUsage: %v", err)
    }
    if saved == nil {
        t.Fatal("UsageLog was not saved")
    }
    if saved.Spans == nil {
        t.Fatal("expected Spans to be non-nil in saved UsageLog")
    }
    if !strings.Contains(*saved.Spans, "token.fetch") {
        t.Errorf("expected token.fetch span in saved Spans; got %s", *saved.Spans)
    }
}
```

- [ ] **Step 4.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestRecordUsage_SpansPersistedToUsageLog -v
```

Expected: `FAIL` — `RecordUsageInput` has no `Spans` field.

- [ ] **Step 4.3: Add `Spans` field to `RecordUsageInput`**

In `gateway_service.go`, in `RecordUsageInput` struct, add:

```go
// Spans holds per-phase timing events collected during the request.
// When non-nil, serialised via MarshalOpsSpans and stored in usage_logs.spans.
Spans []*OpsSpan
```

- [ ] **Step 4.4: Pass spans to `UsageLog` in `writeUsageLogBestEffort`**

In the `UsageLog` construction inside `writeUsageLogBestEffort` (around line 7615 and 7814 — both call sites), add:

```go
usageLog := &UsageLog{
    // ... existing fields ...
    Spans: MarshalOpsSpans(input.Spans),
}
```

- [ ] **Step 4.5: Capture and pass spans in the Copilot handler goroutine**

In `copilot_gateway_handler.go`, in `ChatCompletions`, before the recording goroutine, after the existing latency captures:

```go
// Capture spans before entering goroutine (gin.Context not safe across goroutines).
capturedSpans := service.GetOpsSpans(c)
```

Then in `RecordUsageInput`:

```go
requestID, usageLogID, err := h.gatewayService.RecordUsage(recordCtx, &service.RecordUsageInput{
    // ... existing fields ...
    Spans: capturedSpans,
})
```

Apply the same pattern to the `Messages` and `Responses` handler loop goroutines.

- [ ] **Step 4.6: Run test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestRecordUsage_SpansPersistedToUsageLog -v
```

Expected: `PASS`

- [ ] **Step 4.7: Build and run full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go test ./... -timeout 120s
```

Expected: no errors, all pass.

- [ ] **Step 4.8: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/gateway_service.go internal/handler/copilot_gateway_handler.go
git commit -m "Feature: 成功请求 OpsSpans 持久化到 usage_logs，ops 看板全路径可见延迟分解"
```

---

## Task 5: Refactor `CopilotTokenProvider` — injected exchanger + context propagation

**Files:**
- Modify: `backend/internal/service/copilot_token_provider.go`
- Modify: `backend/internal/pkg/copilot/token.go`
- Modify: `backend/internal/service/copilot_token_provider_test.go` (or new file)

### Why

The token exchange HTTP call currently ignores the caller's context. A slow GitHub API (or a cancelled client) can hold a goroutine for up to 30 s. The fix: (1) update `copilot.ExchangeToken` to accept `context.Context` and pass it to `http.NewRequestWithContext`; (2) in the singleflight body, create a combined context `context.WithTimeout(ctx, 20*time.Second)` so the exchange respects both the caller's cancellation and a hard 20-s cap; (3) replace the direct function call with an injected `tokenExchanger` dependency for clean test isolation — no URL-override hacks.

- [ ] **Step 5.1: Write test for context cancellation**

Add to `copilot_token_provider_test.go`:

```go
func TestGetAccessToken_RespectsContextCancellation(t *testing.T) {
    hanging := make(chan struct{})
    fakeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        close(hanging) // signal: server is now handling the request
        select {
        case <-r.Context().Done():
        case <-time.After(30 * time.Second):
        }
        w.WriteHeader(http.StatusServiceUnavailable)
    }))
    defer fakeSrv.Close()

    // Inject mock exchanger that hits the hanging server.
    mockExchanger := func(ctx context.Context, client *http.Client, token string) (*copilot.CopilotToken, error) {
        req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fakeSrv.URL, nil)
        _, err := client.Do(req) //nolint:gosec
        if err != nil {
            return nil, err
        }
        return nil, errors.New("unexpected success")
    }

    provider := newCopilotTokenProviderWithExchanger(mockExchanger)

    ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
    defer cancel()

    <-hanging // wait until server is blocking

    start := time.Now()
    _, err := provider.GetAccessToken(ctx, &Account{
        ID:       1,
        Platform: PlatformCopilot,
        Credentials: map[string]any{"github_token": "ghp_test"},
    })
    elapsed := time.Since(start)

    if err == nil {
        t.Fatal("expected error on context cancellation, got nil")
    }
    if elapsed > 2*time.Second {
        t.Errorf("GetAccessToken did not respect cancellation: elapsed %v", elapsed)
    }
}
```

- [ ] **Step 5.2: Write test for exchange latency logging**

```go
func TestGetAccessToken_LogsExchangeLatency(t *testing.T) {
    var logBuf bytes.Buffer
    logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
    slog.SetDefault(logger)
    t.Cleanup(func() { slog.SetDefault(slog.Default()) })

    mockExchanger := func(ctx context.Context, client *http.Client, token string) (*copilot.CopilotToken, error) {
        time.Sleep(20 * time.Millisecond) // simulate network
        return &copilot.CopilotToken{
            Token:     "ghs_test_token",
            ExpiresAt: time.Now().Add(30 * time.Minute),
            RefreshAt: time.Now().Add(5 * time.Minute),
        }, nil
    }
    provider := newCopilotTokenProviderWithExchanger(mockExchanger)

    tok, err := provider.GetAccessToken(context.Background(), &Account{
        ID:       99,
        Platform: PlatformCopilot,
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

- [ ] **Step 5.3: Run tests — confirm they fail**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestGetAccessToken_Respects|TestGetAccessToken_Logs" -v -timeout 10s
```

Expected: `FAIL` — `newCopilotTokenProviderWithExchanger` undefined.

- [ ] **Step 5.4: Implement `tokenExchanger` dependency injection**

In `internal/pkg/copilot/token.go`, update `ExchangeToken` to accept a context:

```go
// ExchangeToken exchanges a GitHub personal access token for a short-lived
// Copilot API token.  ctx controls the lifetime of the HTTP exchange.
func ExchangeToken(ctx context.Context, httpClient *http.Client, githubToken string) (*CopilotToken, error) {
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

    resp, err := httpClient.Do(req) //nolint:gosec
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
    return &CopilotToken{
        Token:     tokenResp.Token,
        ExpiresAt: expiresAt,
        RefreshAt: refreshAt,
    }, nil
}
```

In `copilot_token_provider.go`, replace the struct and constructor:

```go
// tokenExchanger is a function that exchanges a GitHub token for a Copilot token.
// Injected at construction time; production uses copilot.ExchangeToken.
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
    return newCopilotTokenProviderWithExchanger(copilot.ExchangeToken)
}

// newCopilotTokenProviderWithExchanger creates a CopilotTokenProvider with a
// custom exchange function.  Used in tests to inject a mock exchanger.
func newCopilotTokenProviderWithExchanger(ex tokenExchanger) *CopilotTokenProvider {
    return &CopilotTokenProvider{
        httpClient: &http.Client{Timeout: 30 * time.Second},
        exchange:   ex,
        tokens:     make(map[int64]*copilot.CopilotToken),
    }
}
```

In the singleflight body inside `GetAccessToken`, replace the exchange call:

```go
		exchangeStart := time.Now()
		// Use a combined context: respect caller cancellation but cap at 20 s
		// to prevent a slow GitHub API from holding the goroutine indefinitely.
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
go test ./... -timeout 120s
```

Expected: all pass.

- [ ] **Step 5.7: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_token_provider.go internal/pkg/copilot/token.go \
        internal/service/copilot_token_provider_test.go
git commit -m "Fix: CopilotTokenProvider 注入 exchanger 依赖，token exchange 支持 context 取消"
```

---

## Task 6: `quota_exhaustion_suspected` anomaly using real upstream latency

**Files:**
- Modify: `backend/internal/service/anomaly_service.go`
- Modify: `backend/internal/handler/copilot_gateway_handler.go`

### Why

The current `detectAnomalies` function receives `durationMs` (total request time) but not `upstreamLatencyMs`. Quota exhaustion is identifiable specifically by **upstream** latency > 30 000 ms combined with `outputTokens == 0`. Using total duration would produce false positives on requests that are slow due to large responses, not quota throttling.

`RecordUsageInput.UpstreamLatencyMs *int` already exists and is populated by the handler. It just needs to be threaded into `detectAnomalies` / `WriteAnomalyLog`.

The existing `detectAnomalies` pure function is already well-structured. We extend it with a new `AnomalySignal` struct to make the call site readable as the parameter list grows.

- [ ] **Step 6.1: Write test for quota exhaustion detection**

Add to `anomaly_service.go` test file:

```go
func TestDetectAnomalies_QuotaExhaustionSuspected(t *testing.T) {
    settings := DefaultAnomalySettings()
    tests := []struct {
        name              string
        sig               AnomalySignal
        wantQuotaAnomaly  bool
    }{
        {
            name: "upstream > 30s and zero output → quota suspected",
            sig:  AnomalySignal{OutputTokens: 0, DurationMs: 35000, UpstreamLatencyMs: int64ptr(32000), StatusCode: 200},
            wantQuotaAnomaly: true,
        },
        {
            name: "upstream > 30s but output non-zero → not quota",
            sig:  AnomalySignal{OutputTokens: 100, DurationMs: 35000, UpstreamLatencyMs: int64ptr(32000), StatusCode: 200},
            wantQuotaAnomaly: false,
        },
        {
            name: "upstream nil → not quota (missing data)",
            sig:  AnomalySignal{OutputTokens: 0, DurationMs: 35000, UpstreamLatencyMs: nil, StatusCode: 200},
            wantQuotaAnomaly: false,
        },
        {
            name: "upstream 15s and zero output → slow but not quota",
            sig:  AnomalySignal{OutputTokens: 0, DurationMs: 20000, UpstreamLatencyMs: int64ptr(15000), StatusCode: 200},
            wantQuotaAnomaly: false,
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

func int64ptr(v int64) *int64 { return &v }
```

- [ ] **Step 6.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestDetectAnomalies_QuotaExhaustionSuspected -v
```

Expected: `FAIL` — `AnomalySignal` and `AnomalyQuotaExhaustionSuspected` undefined.

- [ ] **Step 6.3: Introduce `AnomalySignal` and update `detectAnomalies`**

In `anomaly_service.go`:

```go
// Add new anomaly type constant:
const (
    AnomalyZeroToken              AnomalyType = "zero_token"
    AnomalySlowRequest            AnomalyType = "slow_request"
    AnomalyTimeout                AnomalyType = "timeout"
    AnomalyError                  AnomalyType = "error"
    AnomalyQuotaExhaustionSuspected AnomalyType = "quota_exhaustion_suspected"
)

// AnomalySignal bundles all observable signals for anomaly classification.
// Using a struct keeps detectAnomalies readable as more signals are added.
type AnomalySignal struct {
    InputTokens       int
    OutputTokens      int
    DurationMs        int64
    // UpstreamLatencyMs is the time from sending the upstream request to receiving
    // the first byte of the response.  Nil when not available (e.g. non-Copilot paths).
    UpstreamLatencyMs *int64
    StatusCode        int
}
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

    // Quota exhaustion: upstream stalled > 30 s AND produced zero output tokens.
    // This matches Copilot's silent soft-throttle behaviour when premium interaction
    // quota is exhausted — it returns HTTP 200 after a long stall with no content.
    // Gated on UpstreamLatencyMs (not total DurationMs) to avoid false positives
    // from large legitimate responses with long streaming tails.
    const quotaExhaustionUpstreamThresholdMs = 30_000
    if sig.UpstreamLatencyMs != nil &&
        *sig.UpstreamLatencyMs > quotaExhaustionUpstreamThresholdMs &&
        sig.OutputTokens == 0 {
        types = append(types, AnomalyQuotaExhaustionSuspected)
    }

    if sig.StatusCode >= 500 {
        types = append(types, AnomalyError)
    }

    return types
}
```

Update `WriteAnomalyLog` to accept and thread `UpstreamLatencyMs`:

```go
func (s *AnomalyService) WriteAnomalyLog(
    ctx context.Context,
    inputTokens, outputTokens int,
    durationMs int64,
    statusCode int,
    upstreamLatencyMs *int64, // new parameter
    input *RequestLogInput,
) {
    bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
    defer cancel()

    settings := s.GetSettings(bgCtx)
    sig := AnomalySignal{
        InputTokens:       inputTokens,
        OutputTokens:      outputTokens,
        DurationMs:        durationMs,
        UpstreamLatencyMs: upstreamLatencyMs,
        StatusCode:        statusCode,
    }
    anomalies := detectAnomalies(sig, settings)
    // ... rest unchanged ...
}
```

Update all `WriteAnomalyLog` call sites in `copilot_gateway_handler.go` to pass the `upstreamLatencyMsVal` pointer already captured:

```go
h.anomalyService.WriteAnomalyLog(
    recordCtx,
    capturedResult.Usage.PromptTokens,
    capturedResult.Usage.CompletionTokens,
    capturedResult.Duration.Milliseconds(),
    200,
    upstreamLatencyMsVal, // pass pointer captured before goroutine
    &service.RequestLogInput{...},
)
```

- [ ] **Step 6.4: Run test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestDetectAnomalies_QuotaExhaustionSuspected -v
```

Expected: `PASS`

- [ ] **Step 6.5: Build and run full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go test ./... -timeout 120s
```

Expected: no errors, all pass.

- [ ] **Step 6.6: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/anomaly_service.go internal/handler/copilot_gateway_handler.go
git commit -m "Feature: 新增 quota_exhaustion_suspected 异常类型，基于真实上游延迟检测 Copilot 配额耗尽"
```

---

## Task 7: End-to-end build and regression verification

- [ ] **Step 7.1: Full build with vet**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go vet ./...
```

Expected: no errors or warnings.

- [ ] **Step 7.2: Race-detector test run**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test -race ./... -timeout 180s
```

Expected: all tests pass, no data races.

- [ ] **Step 7.3: Manual ops dashboard check**

With a running development instance, send one successful Claude Code request. In the ops dashboard → request details → latency breakdown panel, verify that:
- Spans (including `token.fetch`, `upstream.post`) are visible for the request.
- Token counts are non-zero.
- No anomaly is emitted for a normal request.

- [ ] **Step 7.4: Manual anomaly check**

Temporarily set `SlowRequestThresholdMs` to 1000 ms, send a slow request, verify `slow_request` anomaly appears in the request-inspect page.

---

## Self-Review Checklist

**Spec coverage:**
- [x] Token = 0 (`/messages → /chat/completions`) → Tasks 1–2
- [x] Token = 0 (`/chat/completions` direct) → Tasks 1, 3
- [x] Protocol preservation (usage chunk filtering) → Task 3
- [x] Spans persisted for successful requests → Task 4
- [x] Context-aware token exchange / goroutine safety → Task 5
- [x] Quota exhaustion anomaly using real upstream latency → Task 6
- [x] `/messages → /responses` path unaffected (already captures usage) → covered by regression test in Task 2

**Placeholder scan:** None found — all steps contain complete code.

**Type consistency:**
- `ensureStreamIncludeUsage(body []byte) []byte` — defined in Task 1, used in Tasks 2 and 3.
- `isUsageOnlyChunk(data string) bool` — defined in Task 1, used in Task 3.
- `handleStreamingResponse(..., forwardUsageChunk bool)` — signature updated in Task 3, only one call site in `ForwardChatCompletions`.
- `tokenExchanger func(context.Context, *http.Client, string) (*copilot.CopilotToken, error)` — defined in Task 5, aligns with updated `copilot.ExchangeToken` signature.
- `AnomalySignal` — defined in Task 6, used in `detectAnomalies` and `WriteAnomalyLog`.
- `AnomalyQuotaExhaustionSuspected AnomalyType` — defined in Task 6, referenced in test.
- `Account.Credentials map[string]any` — all test code uses `map[string]any{...}` (corrected from v1 which incorrectly used `[]AccountCredential`).

**Risk notes:**
- Tasks 1–3: The only change sent upstream is `stream_options.include_usage: true`. This is a standard OpenAI API field documented by GitHub Copilot. The `/chat/completions` path uses `isUsageOnlyChunk` to ensure the extra chunk is never forwarded to clients that didn't request it — zero protocol change for existing clients.
- Task 4: `MarshalOpsSpans` returns `nil` for empty slices. If no spans were recorded (non-Copilot path, or older code path), `UsageLog.Spans` stays `nil`. No regression.
- Task 5: `copilot.ExchangeToken` signature changes from `(httpClient, githubToken)` to `(ctx, httpClient, githubToken)`. All existing callers in `copilot_token_provider.go` use the injected `p.exchange` function — no external callers need updating beyond the test files which already use the mock pattern.
- Task 6: `WriteAnomalyLog` gains a new `upstreamLatencyMs *int64` parameter. All call sites are in `copilot_gateway_handler.go` where `upstreamLatencyMsVal` is already captured. The compiler will flag any missed call sites.
