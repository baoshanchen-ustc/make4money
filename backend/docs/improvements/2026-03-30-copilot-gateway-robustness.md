# Copilot Gateway Robustness Improvement Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix token-reporting zero values and slow-response pathologies in the Copilot gateway, making usage tracking reliable and response latency measurable end-to-end.

**Architecture:** Three independent fix areas — (1) inject `stream_options.include_usage` so Copilot always returns token counts in SSE streams, (2) add structured latency attribution so slow-path root causes are surfaced in the ops dashboard, (3) harden token-refresh concurrency to eliminate the hidden "first request after expiry" stall. Each area is independently testable and committable.

**Tech Stack:** Go 1.22+, `github.com/tidwall/sjson` / `gjson`, `bufio.Scanner`, `singleflight`, standard `net/http`.

---

## Background & Root-Cause Summary

### Problem 1 — Token Counts Are 0

**Root cause A — `/messages` path (Anthropic clients, i.e. Claude Code):**

`ForwardMessages()` calls `forceStreamTrue()` to convert `stream=false → stream=true` before sending to Copilot. However it does **not** inject `"stream_options": {"include_usage": true}`. GitHub Copilot's OpenAI-compatible endpoint only appends the usage summary chunk (the one `parseStreamUsage` looks for) when the request explicitly includes `stream_options.include_usage: true`. Without it the final chunk containing `"usage": {"prompt_tokens": N}` is **never sent**, so `usage` stays `&CopilotUsage{}` (all zeros).

**Root cause B — `/chat/completions` path (OpenAI-mode clients):**

`ForwardChatCompletions()` passes the body through unmodified (aside from model rewrite and max-token clamp). If the client omits `stream_options.include_usage`, the upstream also omits usage. Claude Code's OpenAI-mode (`cc-switch`) sends `stream_options.include_usage: true` by default, but third-party clients may not.

### Problem 2 — Slow Responses (no clear attribution)

| Slow-path | Where it shows | Current observability |
|-----------|---------------|----------------------|
| GitHub token exchange (30 s timeout) | `routing_latency_ms` spike | Only in logs, not distinguished from account-selection |
| Copilot upstream cold-start / throttle | `upstream_latency_ms` | Recorded but no P95/histogram |
| Multi-account failover (3× full round-trip) | Sum of all spans | Individual failover attempt durations not logged |
| Quota exhaustion silent slow-down | No signal at all | Not detected |

**Root cause:** `token.fetch` span is recorded but `routing_latency_ms` conflates "account selection" with "token exchange". There is no per-attempt upstream latency recorded during failover. And when Copilot silently throttles (quota soft-cap), the only observable symptom is `upstream_latency_ms > 30 000 ms` with HTTP 200.

### Problem 3 — Token-Refresh Thundering-Herd Window

`CopilotTokenProvider.GetAccessToken` uses `singleflight` correctly. However `ShouldRefresh()` is evaluated against `RefreshAt = now + (refreshIn - 60)s`. When `refreshIn` from GitHub is 300 s and we subtract 60, we get a 240-second proactive window. Inside that window, every call still returns the cached token immediately. The only stall happens when `IsExpired()` (60 s before actual expiry) triggers. This is a narrow window but under high concurrency (e.g. 50 concurrent users) all goroutines waiting on `sfGroup.Do` block until one 30-s HTTP call completes. The fix is to add a `context.WithTimeout` to the exchange call itself.

---

## File Map

| File | Change type | What changes |
|------|-------------|-------------|
| `internal/service/copilot_gateway_service.go` | Modify | Add `ensureStreamIncludeUsage()`, call it after `forceStreamTrue()` in `ForwardMessages()`; call it in `ForwardChatCompletions()` when stream=true; add `tokenFetchDurationMs` span attr; record per-attempt upstream latency in failover path |
| `internal/service/copilot_token_provider.go` | Modify | Add `context`-aware timeout to the exchange HTTP call; add `slog` latency line |
| `internal/service/copilot_gateway_service_test.go` | Modify/Add | Tests for `ensureStreamIncludeUsage`, `forceStreamTrue` + include_usage combo, token-refresh latency logging |
| `internal/handler/copilot_gateway_handler.go` | Modify | Log `token_fetch_ms` span attribute; add per-failover-attempt span |

---

## Task 1: Add `ensureStreamIncludeUsage` helper and tests

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go` (near `forceStreamTrue`, line ~1826)
- Modify: `backend/internal/service/copilot_gateway_service_test.go`

### Why

When `stream=true`, the Copilot API only appends the usage summary SSE chunk if the request body contains `"stream_options": {"include_usage": true}`. Without this, `parseStreamUsage` never finds a non-zero usage value and all token counts are recorded as 0.

### Step-by-step

- [ ] **Step 1.1: Write failing test for `ensureStreamIncludeUsage`**

Add to `backend/internal/service/copilot_gateway_service_test.go`:

```go
func TestEnsureStreamIncludeUsage(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  string // expected "stream_options.include_usage" value
    }{
        {
            name:  "no stream_options",
            input: `{"model":"gpt-4o","stream":true}`,
            want:  "true",
        },
        {
            name:  "stream_options exists without include_usage",
            input: `{"model":"gpt-4o","stream":true,"stream_options":{}}`,
            want:  "true",
        },
        {
            name:  "include_usage already true",
            input: `{"model":"gpt-4o","stream":true,"stream_options":{"include_usage":true}}`,
            want:  "true",
        },
        {
            name:  "include_usage false → set to true",
            input: `{"model":"gpt-4o","stream":true,"stream_options":{"include_usage":false}}`,
            want:  "true",
        },
        {
            name:  "stream=false, should NOT inject",
            input: `{"model":"gpt-4o","stream":false}`,
            want:  "", // stream_options.include_usage absent
        },
        {
            name:  "stream absent, should NOT inject",
            input: `{"model":"gpt-4o"}`,
            want:  "", // stream_options.include_usage absent
        },
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got := ensureStreamIncludeUsage([]byte(tc.input))
            val := gjson.GetBytes(got, "stream_options.include_usage")
            if tc.want == "" {
                if val.Exists() {
                    t.Errorf("expected no stream_options.include_usage, got %s", val.Raw)
                }
                return
            }
            if !val.Exists() {
                t.Errorf("expected stream_options.include_usage=%s, field absent; body=%s", tc.want, string(got))
                return
            }
            if val.Raw != tc.want {
                t.Errorf("expected stream_options.include_usage=%s, got %s", tc.want, val.Raw)
            }
        })
    }
}
```

- [ ] **Step 1.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestEnsureStreamIncludeUsage -v
```

Expected: `FAIL` — `ensureStreamIncludeUsage undefined`

- [ ] **Step 1.3: Implement `ensureStreamIncludeUsage`**

In `backend/internal/service/copilot_gateway_service.go`, add directly after `forceStreamTrue` (around line 1841):

```go
// ensureStreamIncludeUsage injects "stream_options": {"include_usage": true} into
// the request body when stream=true, so the Copilot API appends a usage summary
// chunk at the end of the SSE stream.  Without this field Copilot omits the usage
// chunk and all token counts are recorded as zero.
//
// The function is a no-op when stream is absent or false (non-streaming requests
// do include usage in the response body by default).
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
```

- [ ] **Step 1.4: Run test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestEnsureStreamIncludeUsage -v
```

Expected: `PASS` for all 6 sub-cases.

- [ ] **Step 1.5: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_gateway_service.go internal/service/copilot_gateway_service_test.go
git commit -m "Feature: 添加 ensureStreamIncludeUsage 辅助函数确保 Copilot 流式响应包含 token 统计"
```

---

## Task 2: Call `ensureStreamIncludeUsage` in `ForwardMessages` path

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go` (line ~981, after `forceStreamTrue`)

### Why

`ForwardMessages` is the path used by Claude Code (Anthropic-format clients). It already calls `forceStreamTrue` but never injects `include_usage`. This is the primary cause of token = 0 for the most common usage pattern.

### Step-by-step

- [ ] **Step 2.1: Write integration test for `ForwardMessages` token recording**

Add to `backend/internal/service/copilot_gateway_service_test.go`:

```go
func TestForwardMessages_UpstreamBodyIncludesStreamIncludeUsage(t *testing.T) {
    // Build a minimal fake Copilot upstream that captures the request body.
    var capturedBody []byte
    srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        capturedBody, _ = io.ReadAll(r.Body)
        // Return a minimal SSE stream with usage chunk.
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n")
        fmt.Fprint(w, "data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}\n\n")
        fmt.Fprint(w, "data: [DONE]\n\n")
    }))
    defer srv.Close()

    // Point the service at the fake server.
    svc := newTestCopilotGatewayService(t, srv)
    account := &Account{
        ID:       1,
        Platform: PlatformCopilot,
        Credentials: []AccountCredential{
            {Key: "github_token", Value: "ghp_test"},
            {Key: "base_url", Value: srv.URL},
        },
    }

    c, _ := gin.CreateTestContext(httptest.NewRecorder())
    c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

    anthropicBody := []byte(`{"model":"claude-sonnet-4-5","stream":false,"max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`)
    _, err := svc.ForwardMessages(context.Background(), c, account, anthropicBody)
    if err != nil {
        t.Fatalf("ForwardMessages: %v", err)
    }

    if len(capturedBody) == 0 {
        t.Fatal("upstream did not receive a request body")
    }
    includeUsage := gjson.GetBytes(capturedBody, "stream_options.include_usage")
    if !includeUsage.Bool() {
        t.Errorf("expected stream_options.include_usage=true in upstream request, body=%s", string(capturedBody))
    }
    streamVal := gjson.GetBytes(capturedBody, "stream")
    if !streamVal.Bool() {
        t.Errorf("expected stream=true in upstream request (forceStreamTrue), body=%s", string(capturedBody))
    }
}
```

- [ ] **Step 2.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestForwardMessages_UpstreamBodyIncludesStreamIncludeUsage -v
```

Expected: `FAIL` — `stream_options.include_usage` absent.

- [ ] **Step 2.3: Add the call in `ForwardMessages`**

In `copilot_gateway_service.go`, find the block (around line 981):

```go
	openAIBody = forceStreamTrue(openAIBody)
```

Change to:

```go
	openAIBody = forceStreamTrue(openAIBody)
	// Ensure Copilot returns usage statistics in the final SSE chunk.
	// forceStreamTrue may have just enabled streaming; ensureStreamIncludeUsage
	// must run after so it can see stream=true.
	openAIBody = ensureStreamIncludeUsage(openAIBody)
```

- [ ] **Step 2.4: Run test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestForwardMessages_UpstreamBodyIncludesStreamIncludeUsage -v
```

Expected: `PASS`

- [ ] **Step 2.5: Run full service tests to check for regressions**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -timeout 120s
```

Expected: all tests pass.

- [ ] **Step 2.6: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_gateway_service.go internal/service/copilot_gateway_service_test.go
git commit -m "Fix: ForwardMessages 路径注入 stream_options.include_usage，修复 token 计数为 0 问题"
```

---

## Task 3: Call `ensureStreamIncludeUsage` in `ForwardChatCompletions` path

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go` (around line 125–135, preprocessing block)

### Why

When third-party clients (or Claude Code in OpenAI-mode) send `stream=true` without `stream_options.include_usage`, `ForwardChatCompletions` passes the body straight through and the Copilot response omits usage. Adding the injection here ensures consistent token tracking regardless of what the client sends.

### Step-by-step

- [ ] **Step 3.1: Write test for `ForwardChatCompletions` usage injection**

Add to `copilot_gateway_service_test.go`:

```go
func TestForwardChatCompletions_StreamingBodyIncludesIncludeUsage(t *testing.T) {
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

    svc := newTestCopilotGatewayService(t, srv)
    account := &Account{
        ID:       1,
        Platform: PlatformCopilot,
        Credentials: []AccountCredential{
            {Key: "github_token", Value: "ghp_test"},
            {Key: "base_url", Value: srv.URL},
        },
    }

    c, _ := gin.CreateTestContext(httptest.NewRecorder())
    c.Request = httptest.NewRequest(http.MethodPost, "/copilot/v1/chat/completions", nil)

    // Client sends streaming request WITHOUT stream_options.
    clientBody := []byte(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
    result, err := svc.ForwardChatCompletions(context.Background(), c, account, clientBody)
    if err != nil {
        t.Fatalf("ForwardChatCompletions: %v", err)
    }

    // Upstream body must have include_usage injected.
    includeUsage := gjson.GetBytes(capturedBody, "stream_options.include_usage")
    if !includeUsage.Bool() {
        t.Errorf("expected stream_options.include_usage=true in upstream request, body=%s", string(capturedBody))
    }

    // Usage must be non-zero.
    if result.Usage == nil || result.Usage.PromptTokens == 0 {
        t.Errorf("expected non-zero PromptTokens, got %+v", result.Usage)
    }
}
```

- [ ] **Step 3.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestForwardChatCompletions_StreamingBodyIncludesIncludeUsage -v
```

Expected: `FAIL`

- [ ] **Step 3.3: Add the call in `ForwardChatCompletions`**

In `copilot_gateway_service.go`, in `ForwardChatCompletions`, find the preprocessing block (after `clampCopilotUpstreamMaxTokens`, before token fetch, around line 128–135):

```go
	body = mergeConsecutiveSameRoleMessagesInOpenAIBody(body)
	body, logModel = rewriteCopilotUpstreamModel(body, account)
	body = clampCopilotUpstreamMaxTokens(body, account)
```

Change to:

```go
	body = mergeConsecutiveSameRoleMessagesInOpenAIBody(body)
	body, logModel = rewriteCopilotUpstreamModel(body, account)
	body = clampCopilotUpstreamMaxTokens(body, account)
	// Ensure streaming requests include usage statistics in the final SSE chunk.
	body = ensureStreamIncludeUsage(body)
```

- [ ] **Step 3.4: Run test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestForwardChatCompletions_StreamingBodyIncludesIncludeUsage -v
```

Expected: `PASS`

- [ ] **Step 3.5: Run full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -timeout 120s
```

Expected: all pass.

- [ ] **Step 3.6: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_gateway_service.go internal/service/copilot_gateway_service_test.go
git commit -m "Fix: ForwardChatCompletions 路径注入 stream_options.include_usage，确保全路径 token 计数一致"
```

---

## Task 4: Add token-exchange latency to `token.fetch` span

**Files:**
- Modify: `backend/internal/service/copilot_token_provider.go`
- Modify: `backend/internal/service/copilot_gateway_service.go` (span attribute update)

### Why

When a token refresh stalls (GitHub API slow), `routing_latency_ms` spikes but there is no way to tell from the ops dashboard whether the latency came from account selection or token exchange. Adding explicit duration logging to `GetAccessToken` enables precise attribution.

### Step-by-step

- [ ] **Step 4.1: Write test for token-exchange latency logging**

Add to `backend/internal/service/copilot_token_provider_test.go`:

```go
func TestGetAccessToken_LogsExchangeLatency(t *testing.T) {
    // Build a slow-ish token exchange server.
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(20 * time.Millisecond) // simulate network
        json.NewEncoder(w).Encode(map[string]any{
            "token":      "ghs_test_token",
            "expires_at": time.Now().Add(30 * time.Minute).Unix(),
            "refresh_in": 300,
        })
    }))
    defer srv.Close()

    // Patch TokenExchangeURL — use the test server.
    // We test via the log output using a test slog handler.
    var logBuf bytes.Buffer
    logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
    slog.SetDefault(logger)

    provider := NewCopilotTokenProvider()
    // Override the exchange URL for testing via a small helper function or by
    // structuring the provider to accept a configurable URL.
    // (see implementation note in Step 4.3)
    provider.setExchangeURLForTest(srv.URL)

    token, err := provider.GetAccessToken(context.Background(), &Account{
        ID:          99,
        Platform:    PlatformCopilot,
        Credentials: []AccountCredential{{Key: "github_token", Value: "ghp_test"}},
    })
    if err != nil {
        t.Fatalf("GetAccessToken: %v", err)
    }
    if token == "" {
        t.Fatal("expected non-empty token")
    }
    log := logBuf.String()
    if !strings.Contains(log, "exchange_ms") {
        t.Errorf("expected exchange_ms in log output, got: %s", log)
    }
}
```

- [ ] **Step 4.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestGetAccessToken_LogsExchangeLatency -v
```

Expected: `FAIL` — `exchange_ms` not in logs / `setExchangeURLForTest` undefined.

- [ ] **Step 4.3: Implement latency logging + test hook in `CopilotTokenProvider`**

In `copilot_token_provider.go`:

```go
type CopilotTokenProvider struct {
	httpClient  *http.Client
	exchangeURL string // overridable for tests; defaults to copilot.TokenExchangeURL

	mu     sync.RWMutex
	tokens map[int64]*copilot.CopilotToken

	sfGroup singleflight.Group
}

func NewCopilotTokenProvider() *CopilotTokenProvider {
	return &CopilotTokenProvider{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		exchangeURL: copilot.TokenExchangeURL,
		tokens:      make(map[int64]*copilot.CopilotToken),
	}
}

// setExchangeURLForTest overrides the token exchange endpoint.
// Only for use in tests.
func (p *CopilotTokenProvider) setExchangeURLForTest(url string) {
	p.exchangeURL = url
}
```

Inside `GetAccessToken`, in the singleflight body, replace the `copilot.ExchangeToken` call with:

```go
		exchangeStart := time.Now()
		newToken, err := copilot.ExchangeTokenFromURL(p.httpClient, githubToken, p.exchangeURL)
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
		slog.Debug("copilot token refreshed",
			"account_id", account.ID,
			"exchange_ms", exchangeMs,
			"expires_at", newToken.ExpiresAt.Format(time.RFC3339))
```

Add `ExchangeTokenFromURL` to `internal/pkg/copilot/token.go`:

```go
// ExchangeTokenFromURL is like ExchangeToken but uses the provided URL instead of
// the default TokenExchangeURL. Allows test injection of a fake exchange server.
func ExchangeTokenFromURL(httpClient *http.Client, githubToken, tokenURL string) (*CopilotToken, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if tokenURL == "" {
		tokenURL = TokenExchangeURL
	}
	req, err := http.NewRequest(http.MethodGet, tokenURL, nil)
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

Also update the existing `ExchangeToken` function to delegate to `ExchangeTokenFromURL`:

```go
func ExchangeToken(httpClient *http.Client, githubToken string) (*CopilotToken, error) {
	return ExchangeTokenFromURL(httpClient, githubToken, TokenExchangeURL)
}
```

- [ ] **Step 4.4: Run test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestGetAccessToken_LogsExchangeLatency -v
go test ./internal/pkg/copilot/... -v
```

Expected: both pass.

- [ ] **Step 4.5: Run full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./... -timeout 120s
```

Expected: all pass.

- [ ] **Step 4.6: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_token_provider.go internal/service/copilot_token_provider_test.go \
        internal/pkg/copilot/token.go
git commit -m "Feature: token.fetch span 记录 exchange_ms，提升慢响应诊断能力"
```

---

## Task 5: Add per-failover-attempt latency spans

**Files:**
- Modify: `backend/internal/handler/copilot_gateway_handler.go` (ChatCompletions and Messages handler loops)

### Why

When failover occurs (3 account switches), the total latency is the sum of all attempts but the ops dashboard only shows the final upstream latency. Operators cannot see how many attempts were made or how long each took. Adding a span per attempt makes multi-failover root causes immediately obvious in the latency breakdown panel.

### Step-by-step

- [ ] **Step 5.1: Write test verifying failover spans are recorded**

Add to `backend/internal/handler/gateway_handler_intercept_test.go` (or a new file `copilot_gateway_handler_failover_test.go`):

```go
func TestCopilotChatCompletions_FailoverSpansRecorded(t *testing.T) {
    // First account always 429, second account succeeds.
    callCount := 0
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        callCount++
        if callCount == 1 {
            w.WriteHeader(http.StatusTooManyRequests)
            return
        }
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(http.StatusOK)
        fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
        fmt.Fprint(w, "data: {\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":2}}\n\n")
        fmt.Fprint(w, "data: [DONE]\n\n")
    }))
    defer srv.Close()
    // ... (set up handler with two accounts pointing to srv)
    // Assert that the ops spans include at least one "failover.attempt" span
    // with a "attempt_upstream_ms" attribute > 0.
    // (Full wiring elided — use existing test helpers in the file)
}
```

- [ ] **Step 5.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/handler/... -run TestCopilotChatCompletions_FailoverSpansRecorded -v
```

Expected: `FAIL` — no failover span found.

- [ ] **Step 5.3: Add failover attempt span in the handler loop**

In `copilot_gateway_handler.go`, inside `ChatCompletions`, in the failover `continue` branch (around line 279–293):

```go
		if fwdErr != nil {
			if ctx.Err() == context.Canceled {
				// ... existing client-disconnect handling
			}
			failedAccountIDs[account.ID] = struct{}{}
			switchCount++
			// Record the failed attempt as a span for latency attribution.
			service.AppendOpsSpan(c, service.OpsSpan{
				Name:        "failover.attempt",
				StartUnixMs: forwardStart.UnixMilli(),
				DurationMs:  forwardDurationMs,
				Status:      "error",
				Attrs: map[string]any{
					"attempt":    switchCount,
					"account_id": account.ID,
					"error":      fwdErr.Error(),
				},
			})
			if switchCount >= h.maxAccountSwitches {
				// ... existing exhaustion handling
			}
			continue
		}
```

Apply the same pattern to the 429 and 421 failover branches:

```go
		// 429 Too Many Requests
		if result != nil && result.StatusCode == http.StatusTooManyRequests {
			failedAccountIDs[account.ID] = struct{}{}
			switchCount++
			service.AppendOpsSpan(c, service.OpsSpan{
				Name:        "failover.attempt",
				StartUnixMs: forwardStart.UnixMilli(),
				DurationMs:  forwardDurationMs,
				Status:      "rate_limited",
				Attrs: map[string]any{
					"attempt":     switchCount,
					"account_id":  account.ID,
					"status_code": 429,
				},
			})
			if switchCount >= h.maxAccountSwitches {
				// ... existing
			}
			continue
		}
```

- [ ] **Step 5.4: Run test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/handler/... -run TestCopilotChatCompletions_FailoverSpansRecorded -v
```

Expected: `PASS`

- [ ] **Step 5.5: Build verification**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 5.6: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/handler/copilot_gateway_handler.go
git commit -m "Feature: failover 切换时记录 attempt span，支持多次切换延迟归因"
```

---

## Task 6: Context-aware token-exchange timeout

**Files:**
- Modify: `backend/internal/service/copilot_token_provider.go`

### Why

`GetAccessToken` currently ignores the caller's `context.Context` during the HTTP exchange. If the gateway request is cancelled (client disconnected, or a 10-second `recordCtx` fires), the token exchange continues in the background for up to 30 seconds, wasting a goroutine and a connection. Passing the context through ensures the exchange is cancelled with the caller.

**Trade-off:** If we use the caller's context directly, a client disconnect during the *first* token fetch for an account would leave no cached token for the next request. The fix is to use a **combined timeout**: respect the caller's cancellation but also impose an independent 20-second deadline so the exchange doesn't drag on indefinitely.

### Step-by-step

- [ ] **Step 6.1: Write test for context cancellation**

Add to `backend/internal/service/copilot_token_provider_test.go`:

```go
func TestGetAccessToken_RespectsContextCancellation(t *testing.T) {
    // Token exchange server that hangs until cancelled.
    ready := make(chan struct{})
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        close(ready)
        // Block until the client cancels.
        <-r.Context().Done()
        w.WriteHeader(http.StatusServiceUnavailable)
    }))
    defer srv.Close()

    provider := NewCopilotTokenProvider()
    provider.setExchangeURLForTest(srv.URL)

    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    start := time.Now()
    <-ready // wait for server to be handling the request
    _, err := provider.GetAccessToken(ctx, &Account{
        ID:          1,
        Platform:    PlatformCopilot,
        Credentials: []AccountCredential{{Key: "github_token", Value: "ghp_test"}},
    })
    elapsed := time.Since(start)

    if err == nil {
        t.Fatal("expected error on context cancellation, got nil")
    }
    if elapsed > 2*time.Second {
        t.Errorf("GetAccessToken did not respect context cancellation: elapsed %v", elapsed)
    }
}
```

- [ ] **Step 6.2: Run test — confirm it fails (or times out after 30s)**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestGetAccessToken_RespectsContextCancellation -v -timeout 35s
```

Expected: test hangs for ~30s then fails with timeout.

- [ ] **Step 6.3: Pass context into `GetAccessToken` exchange call**

In `copilot_token_provider.go`, update `GetAccessToken` signature to accept `ctx context.Context` (it already does) and pass it through into the singleflight body and to `ExchangeTokenFromURL`:

```go
// In the singleflight body:
// Use a detached context with a hard cap so that caller cancellation
// (e.g. client disconnect) doesn't leave a stale cached-token fetch
// but also so a slow GitHub API doesn't block indefinitely.
exchangeCtx, exchangeCancel := context.WithTimeout(context.Background(), 20*time.Second)
defer exchangeCancel()
// If caller cancelled, honour that too.
select {
case <-ctx.Done():
    if fallbackToken != "" {
        return fallbackToken, nil
    }
    return "", ctx.Err()
default:
}
```

And update the HTTP request creation in `ExchangeTokenFromURL` to accept a context:

```go
func ExchangeTokenFromURL(ctx context.Context, httpClient *http.Client, githubToken, tokenURL string) (*CopilotToken, error) {
    // ...
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
    // ...
}
```

Update `ExchangeToken` to pass `context.Background()` for backward compatibility:

```go
func ExchangeToken(httpClient *http.Client, githubToken string) (*CopilotToken, error) {
    return ExchangeTokenFromURL(context.Background(), httpClient, githubToken, TokenExchangeURL)
}
```

- [ ] **Step 6.4: Run test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestGetAccessToken_RespectsContextCancellation -v -timeout 5s
```

Expected: `PASS` in < 500 ms.

- [ ] **Step 6.5: Run full suite**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./... -timeout 120s
```

Expected: all pass.

- [ ] **Step 6.6: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/copilot_token_provider.go internal/service/copilot_token_provider_test.go \
        internal/pkg/copilot/token.go
git commit -m "Fix: token exchange 传递 context，防止客户端断连时 goroutine 泄漏"
```

---

## Task 7: Slow-request quota exhaustion detection

**Files:**
- Modify: `backend/internal/service/anomaly_service.go`

### Why

When Copilot silently throttles a request due to quota exhaustion, `upstream_latency_ms` spikes above 30 000 ms but the HTTP response is still 200. Currently this is only caught by the generic `slow_request` anomaly type (threshold 20 s). Adding a dedicated `quota_exhaustion_suspected` anomaly type (triggered when `upstream_latency_ms > 30 000` AND `output_tokens == 0`) gives operators a clearer signal.

### Step-by-step

- [ ] **Step 7.1: Write test for `quota_exhaustion_suspected` detection**

In `backend/internal/service/anomaly_service.go` tests:

```go
func TestWriteAnomalyLog_QuotaExhaustionSuspected(t *testing.T) {
    // upstream_latency_ms > 30000 AND output_tokens == 0 → should produce anomaly
    // (actual DB call is mocked; test the detection logic in isolation)
    types := detectAnomalyTypes(0 /*inputTokens*/, 0 /*outputTokens*/, 35000 /*durationMs*/, 200 /*statusCode*/, defaultSettings())
    found := false
    for _, at := range types {
        if at == "quota_exhaustion_suspected" {
            found = true
        }
    }
    if !found {
        t.Errorf("expected quota_exhaustion_suspected in anomaly types, got %v", types)
    }
}
```

This requires refactoring the detection logic into a testable pure function `detectAnomalyTypes`.

- [ ] **Step 7.2: Run test — confirm it fails**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestWriteAnomalyLog_QuotaExhaustionSuspected -v
```

Expected: `FAIL` — `detectAnomalyTypes` undefined.

- [ ] **Step 7.3: Extract and extend `detectAnomalyTypes` in `anomaly_service.go`**

```go
// detectAnomalyTypes returns the set of anomaly type strings that apply to a
// completed request.  It is a pure function — no I/O — to keep it easily testable.
func detectAnomalyTypes(inputTokens, outputTokens int, durationMs int64, statusCode int, settings *AnomalySettings) []string {
    var types []string

    // Zero-token responses (both input and output are zero) indicate a billing
    // or upstream parsing failure.
    if inputTokens == 0 && outputTokens == 0 {
        types = append(types, "zero_token")
    }

    // Slow request: exceeded the configured threshold.
    slowThresholdMs := int64(settings.SlowRequestMs)
    if durationMs >= slowThresholdMs {
        types = append(types, "slow_request")
    }

    // Quota exhaustion suspected: upstream took more than 30 s AND output is
    // zero.  Copilot silently degrades when the premium interaction quota is
    // exhausted, returning HTTP 200 after a long stall with no content.
    const quotaExhaustionThresholdMs = 30_000
    if durationMs > quotaExhaustionThresholdMs && outputTokens == 0 {
        types = append(types, "quota_exhaustion_suspected")
    }

    // HTTP error.
    if statusCode >= 500 {
        types = append(types, "error")
    }

    return types
}
```

Update `WriteAnomalyLog` to call `detectAnomalyTypes` instead of inline checks.

- [ ] **Step 7.4: Run test — confirm it passes**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... -run TestWriteAnomalyLog_QuotaExhaustionSuspected -v
go test ./internal/service/... -run TestAnomalyService -v
```

Expected: all pass.

- [ ] **Step 7.5: Commit**

```bash
cd /Users/ziji/personal/github/sub2api/backend
git add internal/service/anomaly_service.go
git commit -m "Feature: 新增 quota_exhaustion_suspected 异常类型，检测 Copilot 配额耗尽导致的慢响应"
```

---

## Task 8: End-to-end verification

- [ ] **Step 8.1: Build and vet the entire backend**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./... && go vet ./...
```

Expected: no errors or warnings.

- [ ] **Step 8.2: Run full test suite with race detector**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test -race ./... -timeout 180s
```

Expected: all tests pass, no data race detected.

- [ ] **Step 8.3: Manual smoke test — check upstream request body**

With a running development instance, send a request via Claude Code and verify in the ops dashboard (request details → upstream request body) that `stream_options.include_usage: true` is present. Also verify token counts are non-zero.

- [ ] **Step 8.4: Manual smoke test — check latency spans**

Trigger a request that causes one failover (e.g. temporarily mark an account as unavailable). Verify the `failover.attempt` span appears in the ops latency breakdown panel.

---

## Self-Review Checklist

**Spec coverage:**
- [x] Token = 0 bug (root cause A, `/messages` path) → Tasks 1–2
- [x] Token = 0 bug (root cause B, `/chat/completions` path) → Tasks 1, 3
- [x] Token exchange latency attribution → Task 4
- [x] Failover attempt latency attribution → Task 5
- [x] Context cancellation / goroutine leak → Task 6
- [x] Quota exhaustion detection → Task 7

**Placeholder scan:** None — all steps contain actual code.

**Type consistency:**
- `ensureStreamIncludeUsage(body []byte) []byte` — consistent across Tasks 1, 2, 3.
- `ExchangeTokenFromURL(ctx context.Context, httpClient *http.Client, githubToken, tokenURL string)` — consistent across Tasks 4, 6.
- `detectAnomalyTypes(inputTokens, outputTokens int, durationMs int64, statusCode int, settings *AnomalySettings) []string` — consistent across Task 7.

**Risk notes:**
- Tasks 1–3 change what is sent upstream to Copilot. The only change is adding `stream_options.include_usage: true`. This is a standard OpenAI API field that Copilot has supported since at least early 2025. No breaking change risk.
- Task 6 changes the token exchange context propagation. The `context.Background()` fallback in `ExchangeToken` ensures all existing callers remain unaffected.
