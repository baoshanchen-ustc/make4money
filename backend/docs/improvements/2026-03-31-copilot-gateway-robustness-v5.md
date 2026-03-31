# Copilot Gateway Robustness Improvement Plan (v5)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix token-reporting zero values for Copilot streaming paths, persist OpsSpans for successful Copilot requests, and harden token-refresh goroutine safety.

**Scope:**
- Token = 0 fix: `/messages → /chat/completions` and `/chat/completions` direct paths only.
- Spans persistence: Copilot handler goroutines only (ChatCompletions, Responses, Messages).
- `WriteAnomalyLog` external signature NOT changed.
- `detectAnomalies` signature changes to accept `AnomalySignal` — all 8 existing test calls updated in same commit.

**Architecture:** (1) `ensureStreamIncludeUsage` + `isUsageOnlyChunk` helpers; (2) inject `ensureStreamIncludeUsage` BEFORE `http.NewRequestWithContext` in `ForwardChatCompletions` (after L128, before L182) and `ForwardMessages` (after L990, before L992); update `handleStreamingResponse` with `forwardUsageChunk bool` — all 3 call sites updated; (3) add `Spans []*OpsSpan` to both `RecordUsageInput` (L7196) and `RecordUsageLongContextInput` (L7705); wire into both `UsageLog` construction points; (4) inject `tokenExchanger` into `CopilotTokenProvider`; add `ExchangeTokenWithContext` to `token.go`; (5) add `AnomalySignal` + `quota_exhaustion_suspected` logic; update `detectAnomalies` signature and all callers.

**Tech Stack:** Go 1.22+, `github.com/tidwall/sjson`/`gjson`, `bufio.Scanner`, `golang.org/x/sync/singleflight`, `net/http`, `gin-gonic/gin`.

---

## Verified Call-Site Counts (codebase audit)

| Interface | Current callers | Updated |
|-----------|----------------|---------|
| `handleStreamingResponse(c, resp, model, upstreamModel, startTime)` | ForwardChatCompletions:L236, ForwardResponses:L901, test:L388 | All 3 |
| `detectAnomalies(inputTokens, outputTokens int, durationMs int64, statusCode int, settings)` | 1 prod (anomaly_service.go:L213) + 8 tests (anomaly_service_test.go:L15,27,39,51,59,67,80,85) | All 9 |
| `WriteAnomalyLog(ctx, in, out, dur, status, input)` | 6 total | 0 — signature unchanged |
| `copilot.ExchangeToken` | 2 (copilot_token_provider.go, copilot_oauth_service.go) | 0 — new `ExchangeTokenWithContext` added |
| `RecordUsageInput` struct (L7196) | gateway_service.go | Add `Spans []*OpsSpan` |
| `RecordUsageLongContextInput` struct (L7705) | gateway_service.go | Add `Spans []*OpsSpan` |
| `UsageLog.Spans` | L7615 (RecordUsage), L7814 (RecordUsageWithLongContext) | Both |

---

## File Map

| File | What changes |
|------|-------------|
| `internal/service/copilot_gateway_service.go` | Add `ensureStreamIncludeUsage`, `isUsageOnlyChunk`; inject in `ForwardChatCompletions` after L128; inject in `ForwardMessages` after L990; update `handleStreamingResponse` sig + all 3 call sites |
| `internal/service/copilot_gateway_service_test.go` | Unit tests for helpers; integration tests for ForwardChatCompletions; update existing L388 test call |
| `internal/service/gateway_service.go` | Add `Spans []*OpsSpan` to `RecordUsageInput` and `RecordUsageLongContextInput`; assign `MarshalOpsSpans(input.Spans)` at both `UsageLog{}` construction points |
| `internal/handler/copilot_gateway_handler.go` | Capture `GetOpsSpans(c)` before each of 3 goroutines; pass as `Spans` in `RecordUsageInput` |
| `internal/service/copilot_token_provider.go` | Add `exchange tokenExchanger` field; add `newCopilotTokenProviderWithExchanger`; update `GetAccessToken` singleflight body |
| `internal/pkg/copilot/token.go` | Add `ExchangeTokenWithContext(ctx, httpClient, githubToken)` as new function; update `ExchangeToken` to delegate |
| `internal/service/anomaly_service.go` | Add `AnomalyQuotaExhaustionSuspected`; add `AnomalySignal` struct; update `detectAnomalies` signature; add `UpstreamLatencyMs *int` to `RequestLogInput`; update `WriteAnomalyLog` body |
| `internal/service/anomaly_service_test.go` | Update all 8 `detectAnomalies` calls to use `AnomalySignal{}` |

---

## Task 1: `ensureStreamIncludeUsage` + `isUsageOnlyChunk` helpers

**Files:** `copilot_gateway_service.go`, `copilot_gateway_service_test.go`

- [ ] **Step 1.1: Write failing tests**

Add to `copilot_gateway_service_test.go`:

```go
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
```

- [ ] **Step 1.2: Run tests — confirm FAIL**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestEnsureStreamIncludeUsage|TestIsUsageOnlyChunk" -v
```

Expected: `FAIL` — undefined function.

- [ ] **Step 1.3: Implement helpers in `copilot_gateway_service.go`**

Add after `forceStreamTrue` function:

```go
// ensureStreamIncludeUsage injects "stream_options":{"include_usage":true} when stream=true.
// This causes the Copilot API to append a usage-summary SSE chunk, enabling accurate token
// count recording.  No-op when stream is absent or false.
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
// usage-summary chunk (has "usage" but no non-empty "choices").  Used to filter
// the chunk from the downstream stream when the client did not request it.
func isUsageOnlyChunk(data string) bool {
	if !gjson.Get(data, "usage").Exists() {
		return false
	}
	choices := gjson.Get(data, "choices")
	return !choices.Exists() || (choices.IsArray() && len(choices.Array()) == 0)
}
```

- [ ] **Step 1.4: Run tests — confirm PASS**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestEnsureStreamIncludeUsage|TestIsUsageOnlyChunk" -v
```

- [ ] **Step 1.5: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_gateway_service.go internal/service/copilot_gateway_service_test.go
git commit -m "Feature: 添加 ensureStreamIncludeUsage 和 isUsageOnlyChunk 辅助函数"
```

---

## Task 2: Inject `ensureStreamIncludeUsage` + update `handleStreamingResponse`

**Files:** `copilot_gateway_service.go`, `copilot_gateway_service_test.go`

### Exact insertion points (from source audit)

**`ForwardChatCompletions`:**
- Body transformations end at **L128**: `body = clampCopilotUpstreamMaxTokens(body, account)`
- `http.NewRequestWithContext` is at **L182**: `req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))`
- Insert `ensureStreamIncludeUsage` between L128 and the `AppendOpsSpan("translate.req")` call at ~L130.

**`ForwardMessages`:**
- `forceStreamTrue` at **L981**: `openAIBody = forceStreamTrue(openAIBody)`
- All body transforms end at **L990**: `openAIBody = clampCopilotUpstreamMaxTokens(openAIBody, account)`
- `upstreamSent` is computed at **L992**: `upstreamSent := strings.TrimSpace(extractModelFromBody(openAIBody))`
- Insert `ensureStreamIncludeUsage` after L990, before L992.

**`handleStreamingResponse` call sites:**
- `ForwardChatCompletions` L236
- `ForwardResponses` L901 (no `ensureStreamIncludeUsage` injection here — `/responses` uses `response.completed` for usage)
- existing test at L388

### Step-by-step

- [ ] **Step 2.1: Write integration tests**

```go
func TestForwardChatCompletions_UsageChunkFilteredWhenClientDidNotRequest(t *testing.T) {
    var capturedBody []byte
    srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        capturedBody, _ = io.ReadAll(r.Body)
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
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

    clientBody := []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
    result, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
    if err != nil {
        t.Fatalf("ForwardChatCompletions: %v", err)
    }

    // Root-cause check: upstream must have include_usage injected.
    if !gjson.GetBytes(capturedBody, "stream_options.include_usage").Bool() {
        t.Errorf("want stream_options.include_usage=true in upstream body; got %s", capturedBody)
    }
    // Token counts must be non-zero.
    if result.Usage == nil || result.Usage.PromptTokens == 0 {
        t.Errorf("want non-zero PromptTokens; got %+v", result.Usage)
    }
    // Client must NOT receive the usage-only chunk.
    body := w.Body.String()
    if strings.Contains(body, `"prompt_tokens"`) {
        t.Errorf("usage-only chunk must be filtered; got: %s", body)
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

    clientBody := []byte(`{"model":"gpt-4o","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`)
    result, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
    if err != nil {
        t.Fatalf("ForwardChatCompletions: %v", err)
    }
    if result.Usage == nil || result.Usage.PromptTokens == 0 {
        t.Errorf("want non-zero PromptTokens; got %+v", result.Usage)
    }
    // Client SHOULD receive the usage chunk.
    body := w.Body.String()
    if !strings.Contains(body, `"prompt_tokens"`) {
        t.Errorf("want usage chunk forwarded; got: %s", body)
    }
}
```

- [ ] **Step 2.2: Run tests — confirm FAIL**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestForwardChatCompletions_UsageChunk" -v
```

- [ ] **Step 2.3: Update `handleStreamingResponse` signature**

In `copilot_gateway_service.go`, change the function signature at L244:

```go
func (s *CopilotGatewayService) handleStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	model string,
	upstreamModel string,
	startTime time.Time,
	forwardUsageChunk bool,
) (*CopilotForwardResult, error) {
```

Inside the loop, after `s.parseStreamUsage(data, usage)`, add:

```go
		// Filter usage-only chunks injected upstream when the client did not request them.
		// Still parsed above so token counts are always recorded internally.
		if !forwardUsageChunk && isUsageOnlyChunk(data) {
			continue
		}
```

- [ ] **Step 2.4: Inject `ensureStreamIncludeUsage` in `ForwardChatCompletions` BEFORE request build**

In `ForwardChatCompletions`, after `body = clampCopilotUpstreamMaxTokens(body, account)` (L128) and before `AppendOpsSpan("translate.req")`:

```go
	// Remember whether the client originally requested usage in the stream,
	// before ensureStreamIncludeUsage may add it.
	clientWantsUsageChunk := gjson.GetBytes(body, "stream_options.include_usage").Bool()
	// Inject stream_options.include_usage=true so Copilot appends a usage-summary SSE chunk.
	// Must be called before http.NewRequestWithContext (L182) to be included in the request body.
	body = ensureStreamIncludeUsage(body)
```

Update the streaming call at L236:

```go
		return s.handleStreamingResponse(c, resp, model, upstreamSent, startTime, clientWantsUsageChunk)
```

Update the non-streaming call at L240 — `handleStreamingResponse` is not invoked there, no change needed.

- [ ] **Step 2.5: Update `ForwardResponses` call site (L901)**

```go
		// ForwardResponses: usage comes from response.completed terminal event, not include_usage.
		result, fwdErr = s.handleStreamingResponse(c, resp, model, upstreamSent, startTime, false)
```

- [ ] **Step 2.6: Update existing test at L388**

```go
result, err := svc.handleStreamingResponse(c, resp, "gpt-4o", "gpt-4o", time.Now(), false)
```

- [ ] **Step 2.7: Build check**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

Expected: no errors.

- [ ] **Step 2.8: Run tests**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestForwardChatCompletions_UsageChunk" -v
go test ./internal/service/... -timeout 120s
```

- [ ] **Step 2.9: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_gateway_service.go internal/service/copilot_gateway_service_test.go
git commit -m "Fix: ForwardChatCompletions 注入 stream_options.include_usage 并过滤未请求的 usage chunk"
```

---

## Task 3: Inject `ensureStreamIncludeUsage` into `ForwardMessages`

**Files:** `copilot_gateway_service.go`, `copilot_gateway_service_test.go`

### Exact insertion point

`ForwardMessages` at L990: `openAIBody = clampCopilotUpstreamMaxTokens(openAIBody, account)`
Insert AFTER L990, BEFORE L992: `upstreamSent := strings.TrimSpace(extractModelFromBody(openAIBody))`

The test uses `svc.setModelEndpointsCache(account.ID, map[string][]string{upstreamSent: {"/chat/completions"}}, false)` to force the `/chat/completions` branch. The key must match the **upstream model** after `rewriteCopilotUpstreamModel` — for test purposes, inject after checking what `rewriteCopilotUpstreamModel` does to the test model (default identity for unknown models). For safety, inject the cache AFTER the service is created with `"gpt-4o"` as the model in the request body (gpt-4o maps to itself since no account model_mapping is set).

### Step-by-step

- [ ] **Step 3.1: Write integration test**

```go
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
    // setModelEndpointsCache is a package-private method accessible in same-package tests.
    // Endpoint value must be "/chat/completions" (with leading slash) to match shouldUseResponsesEndpoint logic.
    svc.setModelEndpointsCache(account.ID, map[string][]string{
        "gpt-4o": {"/chat/completions"},
    }, false)

    gin.SetMode(gin.TestMode)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

    // Use a model that maps to gpt-4o upstream (or set the model directly as gpt-4o).
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
        t.Errorf("want stream_options.include_usage=true; got %s", capturedBody)
    }
    if result == nil || result.Usage == nil || result.Usage.PromptTokens == 0 {
        t.Errorf("want non-zero PromptTokens; result=%+v", result)
    }
}
```

- [ ] **Step 3.2: Run test — confirm FAIL**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestForwardMessages_UpstreamBodyHasStreamIncludeUsage" -v
```

- [ ] **Step 3.3: Inject in `ForwardMessages` after L990**

In `copilot_gateway_service.go`, change (around L990):

```go
	openAIBody = clampCopilotUpstreamMaxTokens(openAIBody, account)

	upstreamSent := strings.TrimSpace(extractModelFromBody(openAIBody))
```

to:

```go
	openAIBody = clampCopilotUpstreamMaxTokens(openAIBody, account)
	// Inject stream_options.include_usage=true after forceStreamTrue has set stream=true.
	// Must be before the upstream request is built in forwardMessagesViaChatCompletions.
	openAIBody = ensureStreamIncludeUsage(openAIBody)

	upstreamSent := strings.TrimSpace(extractModelFromBody(openAIBody))
```

- [ ] **Step 3.4: Run test — confirm PASS**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestForwardMessages_UpstreamBodyHasStreamIncludeUsage" -v
```

- [ ] **Step 3.5: Full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -timeout 120s
```

- [ ] **Step 3.6: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_gateway_service.go internal/service/copilot_gateway_service_test.go
git commit -m "Fix: ForwardMessages 注入 stream_options.include_usage 修复 token 计数为 0"
```

---

## Task 4: Persist OpsSpans for successful Copilot requests

**Files:** `gateway_service.go`, `copilot_gateway_handler.go`, new `copilot_spans_test.go`

### Key facts from source audit

- `RecordUsageInput.Result` is `*ForwardResult` (gateway_service.go:L7197).
- `ForwardResult.Usage` is `ClaudeUsage` (gateway_service.go:L486).
- `ClaudeUsage` has fields: `InputTokens int`, `OutputTokens int`, etc. (L473+).
- `openAIRecordUsageBestEffortLogRepoStub` (gateway_record_usage_test.go:L52) fields: `bestEffortErr error`, `createErr error`, `bestEffortCalls int`, `createCalls int`, `lastLog *UsageLog`, `lastCtxErr error`. **No `saveFn` field.**
- `newGatewayRecordUsageServiceForTest(usageRepo, userRepo, subRepo)` exists in gateway_record_usage_test.go:L17.
- `openAIRecordUsageUserRepoStub` and `openAIRecordUsageSubRepoStub` are defined in openai_gateway_record_usage_test.go.
- Test uses `//go:build unit` build tag.

### Step-by-step

- [ ] **Step 4.1: Write test**

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
    usageRepo := &openAIRecordUsageBestEffortLogRepoStub{}
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
            Usage:    ClaudeUsage{InputTokens: 10, OutputTokens: 5},
        },
        APIKey:  &APIKey{ID: 1001},
        User:    &User{ID: 2001},
        Account: &Account{ID: 3001, Platform: PlatformCopilot},
        Spans:   spans,
    })
    if err != nil {
        t.Fatalf("RecordUsage: %v", err)
    }
    if usageRepo.lastLog == nil {
        t.Fatal("UsageLog was not saved")
    }
    if usageRepo.lastLog.Spans == nil {
        t.Fatal("expected Spans non-nil in saved UsageLog")
    }
    if !strings.Contains(*usageRepo.lastLog.Spans, "token.fetch") {
        t.Errorf("expected token.fetch in spans; got %s", *usageRepo.lastLog.Spans)
    }
}
```

- [ ] **Step 4.2: Run test — confirm FAIL**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test -tags unit ./internal/service/... -run TestRecordUsage_CopilotSpansPersistedToUsageLog -v
```

Expected: `FAIL` — `RecordUsageInput` has no `Spans` field.

- [ ] **Step 4.3: Add `Spans` to both input structs in `gateway_service.go`**

In `RecordUsageInput` (after `Initiator string` at ~L7219):

```go
	// Spans holds per-phase timing events collected by AppendOpsSpan during the request.
	// Serialised via MarshalOpsSpans and stored in usage_logs.spans when non-nil.
	Spans []*OpsSpan
```

In `RecordUsageLongContextInput` (after `Initiator string` at ~L7730):

```go
	// Spans holds per-phase timing events (same semantics as RecordUsageInput.Spans).
	Spans []*OpsSpan
```

- [ ] **Step 4.4: Assign `Spans` in both `UsageLog{}` construction points**

**Point 1** — `RecordUsage` (L7615 construction block, add before `CreatedAt`):

```go
		Spans:         MarshalOpsSpans(input.Spans),
		CreatedAt:     time.Now(),
```

**Point 2** — `RecordUsageWithLongContext` (L7814 construction block, same addition):

```go
		Spans:         MarshalOpsSpans(input.Spans),
		CreatedAt:     time.Now(),
```

- [ ] **Step 4.5: Capture spans before goroutines in `copilot_gateway_handler.go`**

For each of the 3 recording goroutines (ChatCompletions at ~L343, Responses at ~L779, Messages at ~L1206), add span capture before the `go func()` launch:

```go
	capturedSpans := make([]*service.OpsSpan, len(service.GetOpsSpans(c)))
	copy(capturedSpans, service.GetOpsSpans(c))
```

Add `Spans: capturedSpans` in the `RecordUsageInput{}` inside each goroutine.

- [ ] **Step 4.6: Run test — confirm PASS**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test -tags unit ./internal/service/... -run TestRecordUsage_CopilotSpansPersistedToUsageLog -v
```

- [ ] **Step 4.7: Full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go test -tags unit ./... -timeout 120s && go test ./... -timeout 120s
```

- [ ] **Step 4.8: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/gateway_service.go internal/handler/copilot_gateway_handler.go \
        internal/service/copilot_spans_test.go
git commit -m "Feature: Copilot 成功请求 OpsSpans 持久化到 usage_logs"
```

---

## Task 5: Context-aware token exchange

**Files:** `internal/pkg/copilot/token.go`, `copilot_token_provider.go`, `copilot_token_provider_test.go`

### Key facts from source audit

- `token.go` (L15): `ExchangeToken(httpClient *http.Client, githubToken string) (*CopilotToken, error)` — uses `http.NewRequest` (no context).
- Header constants in `token.go` L27+: `DefaultEditorVersion`, `DefaultEditorPluginVersion`, `DefaultUserAgent`, `DefaultGitHubAPIVersion`.
- `TokenExchangeURL` and `TokenExchangeResponse` are defined in `types.go` in the same package.
- `TokenExchangeResponse` fields include `Token string`, `ErrorMessage string`, `ExpiresAt int64`, `RefreshIn int64`.
- The token-building logic (L61–82 of `token.go`) is inline; no `buildCopilotToken` helper exists.
- `CopilotTokenProvider` struct (copilot_token_provider.go:L34): `httpClient`, `mu`, `tokens`, `sfGroup` — no `exchange` field yet.
- singleflight body uses variable `githubToken` (L65).

### Step-by-step

- [ ] **Step 5.1: Write tests**

Add to `copilot_token_provider_test.go`:

```go
func TestGetAccessToken_RespectsContextCancellation(t *testing.T) {
    hanging := make(chan struct{})
    fakeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        close(hanging) // signal: received request
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
        return nil, fmt.Errorf("exchange failed: %w", err)
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

    select {
    case <-hanging:
        // server is now hanging; let the context expire
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

- [ ] **Step 5.2: Run tests — confirm FAIL**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestGetAccessToken_Respects|TestGetAccessToken_Logs" -v -timeout 10s
```

- [ ] **Step 5.3: Add `ExchangeTokenWithContext` to `token.go`**

In `internal/pkg/copilot/token.go`, add after the existing `ExchangeToken` function.
**The existing `ExchangeToken` body is extracted verbatim into the new function**, replacing `http.NewRequest` with `http.NewRequestWithContext(ctx, ...)`. Then `ExchangeToken` is updated to delegate:

```go
// ExchangeTokenWithContext exchanges a GitHub personal access token for a short-lived
// Copilot API token.  ctx controls the HTTP request — cancelling it aborts the exchange.
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

Update `ExchangeToken` to delegate:

```go
// ExchangeToken is a backward-compatible wrapper that calls ExchangeTokenWithContext
// with context.Background().  Existing callers are unaffected.
func ExchangeToken(httpClient *http.Client, githubToken string) (*CopilotToken, error) {
	return ExchangeTokenWithContext(context.Background(), httpClient, githubToken)
}
```

**Note:** Add `"context"` to the import block since `token.go` does not currently import it.

- [ ] **Step 5.4: Update `CopilotTokenProvider` in `copilot_token_provider.go`**

Add the `tokenExchanger` type and `exchange` field:

```go
// tokenExchanger is the function signature for exchanging a GitHub token for a Copilot token.
type tokenExchanger func(ctx context.Context, httpClient *http.Client, githubToken string) (*copilot.CopilotToken, error)

type CopilotTokenProvider struct {
	httpClient *http.Client
	exchange   tokenExchanger   // add this field

	mu      sync.RWMutex
	tokens  map[int64]*copilot.CopilotToken
	sfGroup singleflight.Group
}
```

Update `NewCopilotTokenProvider`:

```go
func NewCopilotTokenProvider() *CopilotTokenProvider {
	return newCopilotTokenProviderWithExchanger(copilot.ExchangeTokenWithContext)
}

func newCopilotTokenProviderWithExchanger(ex tokenExchanger) *CopilotTokenProvider {
	return &CopilotTokenProvider{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		exchange:   ex,
		tokens:     make(map[int64]*copilot.CopilotToken),
	}
}
```

In the singleflight body, replace the `copilot.ExchangeToken(...)` call with:

```go
		exchangeStart := time.Now()
		exchangeCtx, exchangeCancel := context.WithTimeout(ctx, 20*time.Second)
		defer exchangeCancel()

		newToken, err := p.exchange(exchangeCtx, p.httpClient, githubToken)
		exchangeMs := time.Since(exchangeStart).Milliseconds()
		if err != nil {
			slog.Error("copilot token exchange failed",
				"account_id", account.ID,
				"exchange_ms", exchangeMs,
				"error", err)
			// (keep any existing fallback logic unchanged)
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

**Important:** Read the actual singleflight closure body in `copilot_token_provider.go` before editing to preserve existing fallback-token logic (if any). Only replace the `copilot.ExchangeToken` call; keep surrounding logic unchanged.

- [ ] **Step 5.5: Run tests**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestGetAccessToken_Respects|TestGetAccessToken_Logs" -v -timeout 5s
go test ./internal/pkg/copilot/... -v
```

- [ ] **Step 5.6: Full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go test ./... -timeout 120s
```

- [ ] **Step 5.7: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_token_provider.go internal/service/copilot_token_provider_test.go \
        internal/pkg/copilot/token.go
git commit -m "Fix: CopilotTokenProvider 注入 exchanger 依赖，token exchange 传递 context"
```

---

## Task 6: `quota_exhaustion_suspected` anomaly + update `detectAnomalies` + all callers

**Files:** `anomaly_service.go`, `anomaly_service_test.go`, `copilot_gateway_handler.go`

### Key facts from source audit

- `detectAnomalies` current signature (anomaly_service.go:L176):
  `func detectAnomalies(inputTokens, outputTokens int, durationMs int64, statusCode int, settings *AnomalySettings) []AnomalyType`
- 8 existing test calls in `anomaly_service_test.go` at lines: 15, 27, 39, 51, 59, 67, 80, 85.
- `RequestLogInput` is defined in `anomaly_service.go` (L53).
- `WriteAnomalyLog` internal call to `detectAnomalies` at L213 (in WriteAnomalyLog body).
- `AnomalyType` constants: `AnomalyZeroToken`, `AnomalySlowRequest`, `AnomalyTimeout`, `AnomalyError`.

### Step-by-step

- [ ] **Step 6.1: Write test**

Add to `anomaly_service_test.go`:

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
        {"upstream > 30s, zero output → quota suspected",
            AnomalySignal{OutputTokens: 0, DurationMs: 35000, UpstreamLatencyMs: intPtr(32000), StatusCode: 200}, true},
        {"upstream > 30s, output non-zero → not quota",
            AnomalySignal{OutputTokens: 100, DurationMs: 35000, UpstreamLatencyMs: intPtr(32000), StatusCode: 200}, false},
        {"upstream nil → not quota",
            AnomalySignal{OutputTokens: 0, DurationMs: 35000, UpstreamLatencyMs: nil, StatusCode: 200}, false},
        {"upstream 10s, zero output → not quota",
            AnomalySignal{OutputTokens: 0, DurationMs: 12000, UpstreamLatencyMs: intPtr(10000), StatusCode: 200}, false},
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

- [ ] **Step 6.2: Run test — confirm FAIL**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestDetectAnomalies_QuotaExhaustionSuspected -v
```

- [ ] **Step 6.3: Update `anomaly_service.go`**

Add constant (next to `AnomalyError`):

```go
AnomalyQuotaExhaustionSuspected AnomalyType = "quota_exhaustion_suspected"
```

Add `AnomalySignal` struct after the constants block:

```go
// AnomalySignal bundles observable signals for anomaly classification.
// Internal to anomaly_service.go; constructed by WriteAnomalyLog.
type AnomalySignal struct {
	InputTokens       int
	OutputTokens      int
	DurationMs        int64
	UpstreamLatencyMs *int // nil when not available
	StatusCode        int
}
```

Replace `detectAnomalies` function:

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

	const quotaUpstreamThresholdMs = 30_000
	if sig.UpstreamLatencyMs != nil &&
		int64(*sig.UpstreamLatencyMs) > quotaUpstreamThresholdMs &&
		sig.OutputTokens == 0 {
		types = append(types, AnomalyQuotaExhaustionSuspected)
	}

	if sig.StatusCode >= 500 {
		types = append(types, AnomalyError)
	}

	return types
}
```

Add `UpstreamLatencyMs *int` to `RequestLogInput` struct (at L53, after existing fields):

```go
// UpstreamLatencyMs, when non-nil, enables quota-exhaustion detection.
// Callers without this data leave it nil.
UpstreamLatencyMs *int
```

Update `WriteAnomalyLog` body (L213 call to `detectAnomalies`):

```go
	sig := AnomalySignal{
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		DurationMs:        durationMs,
		UpstreamLatencyMs: input.UpstreamLatencyMs,
		StatusCode:        statusCode,
	}
	anomalies := detectAnomalies(sig, settings)
```

- [ ] **Step 6.4: Update all 8 existing `detectAnomalies` calls in `anomaly_service_test.go`**

Replace each call using the table below. `UpstreamLatencyMs` is omitted (zero-value nil) for all existing tests:

| Line | Old call | New call |
|------|----------|----------|
| 15 | `detectAnomalies(0, 0, 5000, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:0, OutputTokens:0, DurationMs:5000, StatusCode:200}, settings)` |
| 27 | `detectAnomalies(100, 200, 25000, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:100, OutputTokens:200, DurationMs:25000, StatusCode:200}, settings)` |
| 39 | `detectAnomalies(0, 0, 70000, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:0, OutputTokens:0, DurationMs:70000, StatusCode:200}, settings)` |
| 51 | `detectAnomalies(100, 200, 1000, 500, settings)` | `detectAnomalies(AnomalySignal{InputTokens:100, OutputTokens:200, DurationMs:1000, StatusCode:500}, settings)` |
| 59 | `detectAnomalies(100, 200, 5000, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:100, OutputTokens:200, DurationMs:5000, StatusCode:200}, settings)` |
| 67 | `detectAnomalies(0, 0, 5000, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:0, OutputTokens:0, DurationMs:5000, StatusCode:200}, settings)` |
| 80 | `detectAnomalies(100, 200, 20000, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:100, OutputTokens:200, DurationMs:20000, StatusCode:200}, settings)` |
| 85 | `detectAnomalies(100, 200, 20001, 200, settings)` | `detectAnomalies(AnomalySignal{InputTokens:100, OutputTokens:200, DurationMs:20001, StatusCode:200}, settings)` |

- [ ] **Step 6.5: Build check**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

- [ ] **Step 6.6: Run anomaly tests**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run "TestDetectAnomalies" -v
```

Expected: all pass including new quota test.

- [ ] **Step 6.7: Wire `UpstreamLatencyMs` in Copilot handler**

In `copilot_gateway_handler.go`, at each of the 3 `WriteAnomalyLog` call sites (ChatCompletions ~L398, Responses ~L834, Messages ~L1236), add `UpstreamLatencyMs` to the `RequestLogInput`:

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
        UpstreamLatencyMs:    upstreamLatencyMsVal, // *int, captured before goroutine
    },
)
```

OpenAI and Sora handlers omit `UpstreamLatencyMs` — zero-value `nil`, no compilation change.

- [ ] **Step 6.8: Full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go test ./... -timeout 120s
```

- [ ] **Step 6.9: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/anomaly_service.go internal/service/anomaly_service_test.go \
        internal/handler/copilot_gateway_handler.go
git commit -m "Feature: 新增 quota_exhaustion_suspected 异常，基于 upstream_latency_ms 检测 Copilot 配额耗尽"
```

---

## Task 7: Final verification

- [ ] **Step 7.1: Full build + vet + race**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go vet ./... && go test -race ./... -timeout 180s
```

- [ ] **Step 7.2: Unit-tagged tests**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test -tags unit -race ./... -timeout 180s
```

---

## Self-Review Checklist

**Spec coverage:**
- [x] Token = 0 (`/messages → /chat/completions`) → Tasks 1, 3
- [x] Token = 0 (`/chat/completions` direct) → Tasks 1, 2
- [x] `ensureStreamIncludeUsage` injected BEFORE `http.NewRequestWithContext` (verified via `capturedBody` assertion) → Tasks 2, 3
- [x] Protocol preservation (usage chunk filtered) → Task 2
- [x] `handleStreamingResponse` all 3 call sites updated → Task 2
- [x] Spans persisted for successful Copilot requests → Task 4
- [x] Both `RecordUsageInput` AND `RecordUsageLongContextInput` receive `Spans` → Task 4
- [x] Context-aware token exchange → Task 5
- [x] `copilot.ExchangeToken` backward-compatible → Task 5
- [x] `WriteAnomalyLog` external signature unchanged → Task 6
- [x] `detectAnomalies` signature change — all 8 existing test calls updated in same commit → Task 6
- [x] Quota exhaustion detection → Task 6

**Type consistency (verified against source):**
- `ensureStreamIncludeUsage(body []byte) []byte` — same package, gjson+sjson available.
- `isUsageOnlyChunk(data string) bool` — same package.
- `handleStreamingResponse(c, resp, model, upstreamModel, startTime, forwardUsageChunk bool)` — 3 call sites updated.
- `tokenExchanger func(context.Context, *http.Client, string) (*copilot.CopilotToken, error)` — matches `ExchangeTokenWithContext` signature.
- `AnomalySignal.UpstreamLatencyMs *int` — matches `getContextLatencyMsPtr` return type `*int`.
- `RequestLogInput.UpstreamLatencyMs *int` — in `anomaly_service.go` (L53 area).
- `RecordUsageInput.Spans []*OpsSpan` and `RecordUsageLongContextInput.Spans []*OpsSpan` — additive.
- Test stub `openAIRecordUsageBestEffortLogRepoStub.lastLog *UsageLog` — used to check `usageRepo.lastLog.Spans`.
- Test `ForwardResult.Usage ClaudeUsage{InputTokens: 10, OutputTokens: 5}` — matches `ClaudeUsage` struct (L473).
- `svc.setModelEndpointsCache(accountID, map[string][]string{"gpt-4o": {"/chat/completions"}}, false)` — package-private, accessible in same-package tests; endpoint string `"/chat/completions"` matches `shouldUseResponsesEndpoint` comparison at L784.

**Risk summary:**
| Area | Risk | Mitigation |
|------|------|------------|
| `include_usage` injection | Low | Standard field; only when stream=true |
| Usage-only chunk filtering | Low | Structure-based detection |
| `handleStreamingResponse` sig | Low | All 3 call sites updated; compiler validates |
| `detectAnomalies` sig | Low | All 9 callers (1 prod + 8 tests) updated in same commit |
| `ExchangeToken` compat | None | Old function delegates to new |
| `WriteAnomalyLog` unchanged | None | 0 callers modified |
| `RecordUsage*Input.Spans` | None | Additive field; existing callers compile with nil |
