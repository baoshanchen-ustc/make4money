# Copilot Session 级别 Premium 配额优化实现方案

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 通过进程内 session 级别缓存，将同一对话 session 内的所有后续请求（第 2 轮起）的 `X-Initiator` 从 `user`（Premium）改为 `agent`（Standard 免费），覆盖 Claude Code、Codex CLI、普通 API 等所有客户端类型。

**Architecture:** 在 `CopilotGatewayService` 结构体中新增一个带 TTL 的进程内 session 缓存（`sync.Mutex` + `map`，零依赖）。各转发函数在计算 `X-Initiator` 前先提取 session key：Anthropic Messages 路径从 `metadata.user_id` 解析出 `session_id`；ChatCompletions 路径从 OpenAI `user` 字段或 `X-Session-ID` header 提取；Responses 路径同样从 OpenAI `user` 字段或 `X-Session-ID` header 提取（`previous_response_id` 已通过 `copilotInitiatorFromResponsesBody` 独立处理续轮，session cache 与其叠加）。**Session cache key 必须包含 `account.ID` 维度**（格式 `"accountID:sessionKey"`），确保不同租户/API key 之间严格隔离。若 session key 在缓存中已存在（同一账号维度下），则直接返回 `"agent"`；若首次出现则写入缓存并走原有的 assistant/tool 检测逻辑。session 缓存 TTL 设为 2 小时，与 Claude Code 典型会话时长匹配。此方案与已有的 sub-agent system prompt 检测（`2026-04-12` 方案）完全正交，两者叠加后效果更好。

**Tech Stack:** Go 1.22+，标准库 `sync`/`time`/`encoding/json`，现有 `copilot_gateway_service.go`、`copilot_gateway_handler.go`，以及 `metadata_userid.go:ParseMetadataUserID`。

---

## 背景与根因

### 当前问题

经 `2026-04-12` 方案修复后，Claude Code sub-agent 的 system prompt 已经可以被检测到，从而将 sub-agent 首次请求从 `user`（Premium）改为 `agent`（免费）。

但用户还希望：
1. **非 CC 客户端**（普通 API 调用方）的多轮对话中，续轮也不该消耗 Premium
2. **同一 session 内所有请求**（无论客户端类型）只有第一轮算 Premium

### 根本改进点

GitHub Copilot 的 Premium 计费是 **per-request** 的——每次发送 `X-Initiator: user` 就消耗一次 Premium。

但从用户角度，一次"对话"（session）应该只算一次 Premium。当前逻辑把"有没有 assistant/tool 消息"作为代理判断，但：
- 首轮（无 assistant/tool）→ `user` → Premium ✓（合理）
- 后续轮（有 assistant/tool）→ `agent` ✓（已经正确）
- **Sub-agent 首轮**（无 assistant/tool，但 session 内）→ `user` → Premium ✗（不合理，是同一 session）
- **普通 API 续轮**（有 assistant/tool）→ `agent` ✓（已经正确）
- **普通 API 首轮（跨请求独立调用）**→ `user` → Premium ✓（合理）

关键洞察：**如果客户端传递了 session 标识，说明这些请求属于同一对话**。只要同一 session 内出现过任何 Premium 请求，后续所有请求都应该走免费配额。

### Session Key 来源

| 路径 | Session Key 来源 | 解析方式 |
|------|----------------|---------|
| Anthropic Messages | `body.metadata.user_id` | `ParseMetadataUserID().SessionID` |
| OpenAI ChatCompletions | `body.user` 字段 | `ParseMetadataUserID().SessionID`（CC 传递此值） |
| OpenAI Responses (Codex CLI) | `body.user` 字段或 `X-Session-ID` header | 同上；`previous_response_id` 通过 `copilotInitiatorFromResponsesBody` 独立处理，session cache 与其叠加 |
| 任意路径 | `X-Session-ID` header | 直接使用 header 值（客户端自定义） |

**Cache key 格式：** `fmt.Sprintf("%d:%s", account.ID, sessionKey)`
— 必须携带 `account.ID` 维度，防止不同租户之间因 session key 碰撞造成配额串用。

### 效果预期

| 请求类型 | 修改前 | 修改后 |
|---------|--------|--------|
| Session 首轮（任意客户端） | `user` → Premium | `user` → Premium ✓（不变） |
| Session 续轮（有 assistant/tool） | `agent` → 免费 | `agent` → 免费 ✓（不变） |
| **CC Sub-agent 首轮（同 session）** | `user` → Premium | `agent` → 免费 ✓（**修复**） |
| **普通 API 并发子任务（同 session）** | `user` → Premium | `agent` → 免费 ✓（**修复**） |
| **无 session 标识的独立请求** | `user` → Premium | `user` → Premium ✓（不变） |

---

## 文件改动范围

| 文件 | 操作 | 说明 |
|------|------|------|
| `backend/internal/service/copilot_session_cache.go` | **新建** | session 缓存独立文件：`copilotSessionCache` 类型、TTL 清理逻辑、`extractSessionKey` 辅助函数 |
| `backend/internal/service/copilot_gateway_service.go` | 修改 | 在 `CopilotGatewayService` 结构体加 `sessionCache *copilotSessionCache` 字段；在 `NewCopilotGatewayService` 初始化；修改**五处**转发函数（含 `ForwardResponses`）接入 session cache；在所有转发函数返回路径填充 `result.Initiator` |
| `backend/internal/handler/copilot_gateway_handler.go` | 修改（analytics 层） | 三处 `capturedInitiator` 改为读取 `result.Initiator` 以保持 analytics 与上游一致 |
| `backend/internal/service/copilot_session_cache_test.go` | **新建** | session cache 单元测试 |
| `backend/internal/service/copilot_gateway_service_test.go` | 修改 | 新增 session 场景的集成测试用例 |

**不需要修改：**
- `metadata_userid.go`：已有 `ParseMetadataUserID()`，直接复用
- `claude_code_validator.go`：不涉及 session 逻辑
- `copilot_anthropic_translation.go`：不涉及 session 逻辑

---

## Task 0：新建 `copilot_session_cache.go`

**Files:**
- Create: `backend/internal/service/copilot_session_cache.go`
- Test: `backend/internal/service/copilot_session_cache_test.go`

### Step 0.1：写失败测试

新建 `backend/internal/service/copilot_session_cache_test.go`：

```go
//go:build unit

package service

import (
	"fmt"
	"testing"
	"time"
)

func TestCopilotSessionCache_FirstSeen_ReturnsPassthrough(t *testing.T) {
	c := newCopilotSessionCache(2 * time.Hour)
	// 首次见到 session key：不覆盖，返回 false（由调用方决定 initiator）
	if c.markAndCheckSeen("sess-abc") {
		t.Fatal("first call: expected false (not yet seen), got true")
	}
}

func TestCopilotSessionCache_SecondSeen_ReturnsAgent(t *testing.T) {
	c := newCopilotSessionCache(2 * time.Hour)
	c.markAndCheckSeen("sess-abc") // 首次
	// 第二次：session 已存在，应返回 true（调用方应使用 "agent"）
	if !c.markAndCheckSeen("sess-abc") {
		t.Fatal("second call: expected true (already seen), got false")
	}
}

func TestCopilotSessionCache_DifferentKeys_Independent(t *testing.T) {
	c := newCopilotSessionCache(2 * time.Hour)
	c.markAndCheckSeen("sess-aaa")
	// 不同 key 首次应该返回 false
	if c.markAndCheckSeen("sess-bbb") {
		t.Fatal("different key: expected false (first time), got true")
	}
}

func TestCopilotSessionCache_TTLExpiry(t *testing.T) {
	ttl := 50 * time.Millisecond
	c := newCopilotSessionCache(ttl)
	c.markAndCheckSeen("sess-ttl")
	// 等 TTL 过期
	time.Sleep(ttl + 20*time.Millisecond)
	c.evictExpired() // 手动触发清理
	// 过期后再访问，应视为全新 session
	if c.markAndCheckSeen("sess-ttl") {
		t.Fatal("after TTL: expected false (evicted), got true")
	}
}

// TestCopilotSessionCache_AccountIsolation verifies that two different accounts
// with the same raw session key do NOT share cache state.
func TestCopilotSessionCache_AccountIsolation(t *testing.T) {
	c := newCopilotSessionCache(2 * time.Hour)
	const rawKey = "shared-session-key"

	// Account 1, first time → cache miss
	k1 := fmt.Sprintf("%d:%s", int64(1), rawKey)
	if c.markAndCheckSeen(k1) {
		t.Fatal("account 1 first call: expected false")
	}
	// Account 2, same raw key, first time → still a miss (different namespace)
	k2 := fmt.Sprintf("%d:%s", int64(2), rawKey)
	if c.markAndCheckSeen(k2) {
		t.Fatal("account 2 first call: expected false (isolated from account 1)")
	}
	// Account 1, second time → cache hit
	if !c.markAndCheckSeen(k1) {
		t.Fatal("account 1 second call: expected true (cache hit)")
	}
}

func TestExtractSessionKeyFromOpenAIBody(t *testing.T) {
	// CC 风格的 user 字段（legacy metadata.user_id 格式，含 session_id）
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"user":"user_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2_account__session_12345678-1234-1234-1234-123456789abc"}`)
	key := extractSessionKeyFromOpenAIBody(body)
	const want = "12345678-1234-1234-1234-123456789abc"
	if key != want {
		t.Fatalf("got %q, want %q", key, want)
	}
}

func TestExtractSessionKeyFromOpenAIBody_NoUser(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`)
	if key := extractSessionKeyFromOpenAIBody(body); key != "" {
		t.Fatalf("expected empty key, got %q", key)
	}
}

func TestExtractSessionKeyFromAnthropicBody(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hi"}],"metadata":{"user_id":"user_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2_account__session_12345678-1234-1234-1234-123456789abc"}}`)
	key := extractSessionKeyFromAnthropicBody(body)
	const want = "12345678-1234-1234-1234-123456789abc"
	if key != want {
		t.Fatalf("got %q, want %q", key, want)
	}
}
```

- [ ] **Step 0.1：写失败测试，保存文件**

### Step 0.2：运行测试，确认失败

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/ -run "TestCopilotSessionCache|TestExtractSessionKey" -tags unit -v 2>&1 | head -30
```

期望：编译失败（函数未定义）。

- [ ] **Step 0.2：确认失败**

### Step 0.3：实现 `copilot_session_cache.go`

新建 `backend/internal/service/copilot_session_cache.go`：

```go
package service

import (
	"encoding/json"
	"sync"
	"time"
)

// copilotSessionCache tracks which session keys have been seen.
//
// The first request within a session pays the Premium quota (X-Initiator: user);
// all subsequent requests in the same session are free (X-Initiator: agent).
//
// Implementation uses a sync.Map with per-entry expiry timestamps. A background
// goroutine is NOT started deliberately — eviction is lazy (on write) plus an
// explicit evictExpired() call that callers can invoke periodically. This keeps
// the cache self-contained and easy to test without goroutine leaks.
type copilotSessionCache struct {
	mu      sync.Mutex
	entries map[string]time.Time // key → expiry
	ttl     time.Duration
}

func newCopilotSessionCache(ttl time.Duration) *copilotSessionCache {
	return &copilotSessionCache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
	}
}

// markAndCheckSeen records a session key as seen and reports whether it was
// already present (true = already seen → caller should use "agent").
//
// On first call for a key: stores it with a TTL, returns false.
// On subsequent calls within TTL: refreshes TTL, returns true.
// After TTL expires and eviction: behaves like first call again.
func (c *copilotSessionCache) markAndCheckSeen(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if exp, ok := c.entries[key]; ok && now.Before(exp) {
		// Already seen and not expired — refresh TTL and signal "agent".
		c.entries[key] = now.Add(c.ttl)
		return true
	}
	// First time (or expired): record and signal "user" (caller decides).
	c.entries[key] = now.Add(c.ttl)
	return false
}

// evictExpired removes all entries whose TTL has elapsed. Call periodically
// to prevent unbounded memory growth (e.g. from a ticker in the service).
func (c *copilotSessionCache) evictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for k, exp := range c.entries {
		if now.After(exp) {
			delete(c.entries, k)
		}
	}
}

// size returns the number of live entries (for metrics/testing).
func (c *copilotSessionCache) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// --------------------------------------------------------------------------
// Session key extraction helpers
// --------------------------------------------------------------------------

// extractSessionKeyFromOpenAIBody tries to extract a session key from an
// OpenAI-format request body.
//
// Claude Code (and compatible clients) populate the OpenAI "user" field with
// the Anthropic metadata.user_id value, which contains a session UUID that is
// stable across all requests in the same conversation (including sub-agents).
//
// Returns "" if the body has no parseable session ID.
func extractSessionKeyFromOpenAIBody(body []byte) string {
	var req struct {
		User string `json:"user"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.User == "" {
		return ""
	}
	parsed := ParseMetadataUserID(req.User)
	if parsed == nil || parsed.SessionID == "" {
		return ""
	}
	return parsed.SessionID
}

// extractSessionKeyFromAnthropicBody tries to extract a session key from an
// Anthropic-format request body (metadata.user_id).
//
// Returns "" if the body has no parseable session ID.
func extractSessionKeyFromAnthropicBody(body []byte) string {
	var req struct {
		Metadata struct {
			UserID string `json:"user_id"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.Metadata.UserID == "" {
		return ""
	}
	parsed := ParseMetadataUserID(req.Metadata.UserID)
	if parsed == nil || parsed.SessionID == "" {
		return ""
	}
	return parsed.SessionID
}
```

- [ ] **Step 0.3：实现 session cache，保存文件**

### Step 0.4：运行测试，确认通过

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/ -run "TestCopilotSessionCache|TestExtractSessionKey" -tags unit -v
```

期望：所有用例 PASS。

- [ ] **Step 0.4：确认测试通过**

### Step 0.5：提交

```bash
cd /Users/ziji/personal/github/sub2api
git add backend/internal/service/copilot_session_cache.go \
        backend/internal/service/copilot_session_cache_test.go
# 按仓库提交协议提交（类型: Feature，中文描述）
```

- [ ] **Step 0.5：提交**

---

## Task 1：将 session cache 注入 `CopilotGatewayService`，并启动定期清理

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`（`CopilotGatewayService` struct + `NewCopilotGatewayService`）

### Step 1.1：在 struct 加字段

找到 `CopilotGatewayService` 结构体（约第 35 行），在末尾加一行：

```go
type CopilotGatewayService struct {
    tokenProvider *CopilotTokenProvider
    httpClient    *http.Client

    // modelEndpointsCacheMu protects modelEndpointsCache.
    modelEndpointsCacheMu sync.RWMutex
    // modelEndpointsCache maps accountID → per-model supported_endpoints, cached from /models.
    modelEndpointsCache map[int64]*copilotModelEndpointsCacheEntry

    // platformConfigSvc 用于读取账号级平台配置（继承等逻辑）。
    platformConfigSvc *CopilotPlatformConfigService

    // sessionCache tracks per-session Premium quota: first request pays user quota,
    // subsequent requests in the same session use free agent quota.
    sessionCache *copilotSessionCache
}
```

- [ ] **Step 1.1：添加 sessionCache 字段**

### Step 1.2：在 `NewCopilotGatewayService` 初始化并启动定期清理

找到 `NewCopilotGatewayService`（约第 66 行），在 `return` 前新增初始化，并在 return 后（实际是在 return 语句里）初始化：

```go
func NewCopilotGatewayService(
    tokenProvider *CopilotTokenProvider,
) *CopilotGatewayService {
    // ... （现有 transport 构造，不变）

    svc := &CopilotGatewayService{
        tokenProvider: tokenProvider,
        httpClient: &http.Client{
            Timeout:   5 * time.Minute,
            Transport: transport,
        },
        modelEndpointsCache: make(map[int64]*copilotModelEndpointsCacheEntry),
        sessionCache:        newCopilotSessionCache(2 * time.Hour),
    }

    // Start background eviction: runs every 10 minutes to prevent unbounded
    // memory growth from expired session entries.
    // The goroutine leaks only when the process exits — acceptable for a
    // long-lived service singleton.
    go func() {
        ticker := time.NewTicker(10 * time.Minute)
        defer ticker.Stop()
        for range ticker.C {
            svc.sessionCache.evictExpired()
        }
    }()

    return svc
}
```

> **注意：** 现有 `return &CopilotGatewayService{...}` 改为先赋值给 `svc`，再启动 goroutine，最后 `return svc`。

- [ ] **Step 1.2：初始化 sessionCache + 启动清理 goroutine**

### Step 1.3：确认编译通过

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./internal/service/
```

期望：无编译错误。

- [ ] **Step 1.3：确认编译通过**

### Step 1.4：提交

```bash
git add backend/internal/service/copilot_gateway_service.go
# 按仓库提交协议提交（类型: Feature，中文描述）
```

- [ ] **Step 1.4：提交**

---

## Task 2：在五个转发函数内接入 session cache

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`（五处转发函数中的 initiator 计算）

五处位置：
- `forwardChatCompletionsDirect`（约第 280 行）：OpenAI body，读 `user` 字段 + `X-Session-ID` header
- `forwardChatCompletionsViaResponses`（约第 436 行）：同上
- `forwardChatCompletionsViaMessages`（约第 576 行）：同上
- `ForwardResponses`（约第 1848 行）：使用 `copilotInitiatorFromResponsesBody(body)` 作为基础 initiator，再叠加 session cache；从 `user` 字段或 `X-Session-ID` 提取 session key
- `ForwardMessages`（约第 2062 行）：Anthropic body，读 `metadata.user_id` + `X-Session-ID` header

### Step 2.1：新增 `sessionKeyFromContext` 辅助函数

在 `copilot_session_cache.go` 末尾追加（复用 `c *gin.Context`）：

```go
// sessionKeyFromHeader returns the X-Session-ID header value, trimmed.
// Returns "" if absent or empty.
func sessionKeyFromHeader(c *gin.Context) string {
    return strings.TrimSpace(c.GetHeader("X-Session-ID"))
}
```

并在文件头 import 中加入：

```go
import (
    "encoding/json"
    "strings"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
)
```

- [ ] **Step 2.1：追加 sessionKeyFromHeader，更新 import**

### Step 2.2：修改 ChatCompletions 三条路径（第 280、436、576 行）

**替换模式**（三处完全相同）：

```go
// 修改前：
initiator := copilotInitiator(body)

// 修改后：
initiator := copilotInitiator(body)
// Session-level quota optimization: if this request belongs to a known
// session (scoped to this account), override to "agent" regardless of
// message history.
// Priority: X-Session-ID header > metadata.user_id in body.user field.
// Cache key is namespaced by account.ID to prevent cross-tenant pollution.
if rawSK := sessionKeyFromHeader(c); rawSK != "" {
    sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
    if s.sessionCache.markAndCheckSeen(sk) {
        initiator = "agent"
    }
} else if rawSK := extractSessionKeyFromOpenAIBody(body); rawSK != "" {
    sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
    if s.sessionCache.markAndCheckSeen(sk) {
        initiator = "agent"
    }
}
```

> **逻辑说明：**
> - cache key = `fmt.Sprintf("%d:%s", account.ID, rawSessionKey)` — 必须包含 account.ID，防止跨租户配额串用
> - `markAndCheckSeen` 首次调用返回 false（保留原有 initiator，通常是 `"user"`）
> - 后续调用返回 true（覆盖为 `"agent"`，走免费配额）
> - Header 优先于 body 中的 user 字段（方便非 CC 客户端显式指定 session）

- [ ] **Step 2.2：修改三处 ChatCompletions initiator 计算**

### Step 2.3：修改 ForwardResponses 路径（约第 1848 行）

`ForwardResponses` 当前直接使用 `copilotInitiatorFromResponsesBody(body)` 设置 `X-Initiator`，不经过 session cache。需要在此之前插入 session 覆盖逻辑：

```go
// 修改前（位于 ForwardResponses 第 1848 行）：
for k, vals := range copilot.CopilotHeaders(copilotInitiatorFromResponsesBody(body), false) {

// 修改后：
initiatorResp := copilotInitiatorFromResponsesBody(body)
// Session-level quota optimization for Responses path (Codex CLI).
// previous_response_id already causes copilotInitiatorFromResponsesBody to return
// "agent" for chained responses. Session cache further handles concurrent sub-tasks
// within the same session.
if rawSK := sessionKeyFromHeader(c); rawSK != "" {
    sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
    if s.sessionCache.markAndCheckSeen(sk) {
        initiatorResp = "agent"
    }
} else if rawSK := extractSessionKeyFromOpenAIBody(body); rawSK != "" {
    sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
    if s.sessionCache.markAndCheckSeen(sk) {
        initiatorResp = "agent"
    }
}
for k, vals := range copilot.CopilotHeaders(initiatorResp, false) {
```

同时在 `ForwardResponses` 函数末尾找到 `return result, nil`，补填 `Initiator` 字段：

```go
result.ReasoningEffort = reasoningEffort
result.Initiator = initiatorResp  // 补填，供 handler analytics 使用
return result, nil
```

> **注意：** `initiatorResp` 变量需要在函数顶部声明（`var initiatorResp string`），或者直接在 for 循环前赋值后使用，确保 early return 路径（`handleErrorResponse`）也可访问。最简单的做法是把 session 覆盖代码块放在 `copilotInitiatorFromResponsesBody(body)` 调用和 for 循环之间。

- [ ] **Step 2.3：修改 ForwardResponses initiator 计算，补填 result.Initiator**

### Step 2.4：修改 ForwardMessages 路径（约第 2062 行）

```go
// 修改前：
initiator := copilotInitiator(openAIBody)

// 修改后：
initiator := copilotInitiator(openAIBody)
// Session-level quota optimization for Anthropic Messages path.
// Priority: X-Session-ID header > metadata.user_id in anthropicBody.
if rawSK := sessionKeyFromHeader(c); rawSK != "" {
    sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
    if s.sessionCache.markAndCheckSeen(sk) {
        initiator = "agent"
    }
} else if rawSK := extractSessionKeyFromAnthropicBody(anthropicBody); rawSK != "" {
    sk := fmt.Sprintf("%d:%s", account.ID, rawSK)
    if s.sessionCache.markAndCheckSeen(sk) {
        initiator = "agent"
    }
}
```

同样的修改也应用于 `forwardMessagesViaResponses`（约第 2199 行）。

- [ ] **Step 2.4：修改两处 Messages initiator 计算**

### Step 2.5：确认编译通过

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./internal/service/
```

期望：无编译错误。

- [ ] **Step 2.5：确认编译通过**

### Step 2.6：提交

```bash
git add backend/internal/service/copilot_gateway_service.go \
        backend/internal/service/copilot_session_cache.go
# 按仓库提交协议提交（类型: Feature，中文描述）
```

- [ ] **Step 2.6：提交**

---

## Task 3：让 `CopilotForwardResult` 携带实际 Initiator，统一 analytics 口径

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`（`CopilotForwardResult` struct + 四个转发函数的 return 语句）
- Modify: `backend/internal/handler/copilot_gateway_handler.go`（三处 `capturedInitiator` 计算）

**背景：** handler 层独立调用 `service.CopilotInitiatorFromBody(body)` 计算 analytics 用的 initiator，但这个计算不知道 session cache 的存在，可能与 service 层发往上游的真实 `X-Initiator` 不一致。解决方案：在 `CopilotForwardResult` 加 `Initiator` 字段，让 service 层回传实际值，handler 直接使用。

### Step 3.1：在 `CopilotForwardResult` 加 `Initiator` 字段

找到 `CopilotForwardResult` 结构体（约第 106 行），加一行：

```go
type CopilotForwardResult struct {
    StatusCode    int
    Model         string
    UpstreamModel string
    Usage         *CopilotUsage
    Duration      time.Duration
    FirstTokenMs  *int
    ReasoningEffort *string
    // Initiator is the actual X-Initiator value sent to the Copilot upstream.
    // Populated by all forward functions so the handler can record it in analytics
    // without re-computing (and potentially diverging from) the upstream value.
    Initiator string
}
```

- [ ] **Step 3.1：添加 Initiator 字段**

### Step 3.2：在五个转发函数的返回路径上填充 `result.Initiator`

每个转发函数在本地变量 `initiator`（或 `initiatorResp`）确定后，找到该函数返回 `*CopilotForwardResult` 的所有路径，将 `Initiator` 字段赋值。

五个函数：`forwardChatCompletionsDirect`、`forwardChatCompletionsViaResponses`、`forwardChatCompletionsViaMessages`、`ForwardResponses`（已在 Step 2.3 处理）、`ForwardMessages` + `forwardMessagesViaResponses`。

具体做法：以 `forwardChatCompletionsDirect` 为例，找到现有 `return &CopilotForwardResult{...}` 的所有点，加入 `Initiator: initiator`：

```go
// 错误路径（早返回）：
return &CopilotForwardResult{StatusCode: resp.StatusCode, Initiator: initiator}, nil

// 正常路径（`handleStreamingResponse` / `handleNonStreamingResponse` 返回的 result）：
// 这些路径里 result 是 handleXxx 函数返回的，需要在拿到 result 后补填：
if result != nil {
    result.Initiator = initiator
}
return result, err
```

> **提示：** 使用 `grep -n "return.*CopilotForwardResult\|return result\b" copilot_gateway_service.go` 找到所有返回点。
> **注意：** `ForwardResponses` 的 `result.Initiator` 已在 Step 2.3 处填充，此步骤检查其余四个函数。

- [ ] **Step 3.2：五个函数的返回路径补填 Initiator**

### Step 3.3：更新 handler 三处 capturedInitiator 计算

打开 `backend/internal/handler/copilot_gateway_handler.go`，搜索 `capturedInitiator`，找到三处：

**第 370 行（ChatCompletions）：**
```go
// 修改前：
capturedInitiator := service.CopilotInitiatorFromBody(body)

// 修改后：
capturedInitiator := result.Initiator
```

**第 815 行（Responses）：**
```go
// 修改前：
// 注意：此处原代码调用 service.CopilotInitiatorFromBody(body)（内部用 copilotInitiator），
// 但 ForwardResponses 实际使用 copilotInitiatorFromResponsesBody — 两者逻辑不同。
// 改为读 result.Initiator 同时修正这一不一致。
capturedInitiatorResp := service.CopilotInitiatorFromBody(body)

// 修改后：
capturedInitiatorResp := result.Initiator
```

**第 1243 行（Messages）：**
```go
// 修改前：
capturedInitiatorMsg := service.CopilotInitiatorFromBody(body)

// 修改后：
capturedInitiatorMsg := result.Initiator
```

> **注意：** 三处都在 `capturedResult := result` 之后，`result.Initiator` 此时可安全访问（result 已确认非 nil）。

- [ ] **Step 3.3：更新三处 capturedInitiator**

### Step 3.4：确认编译通过

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

期望：无编译错误。

- [ ] **Step 3.4：确认编译通过**

### Step 3.5：提交

```bash
git add backend/internal/service/copilot_gateway_service.go \
        backend/internal/handler/copilot_gateway_handler.go
# 按仓库提交协议提交（类型: Refactor，中文描述）
```

- [ ] **Step 3.5：提交**

---

## Task 4：集成测试 — session cache 场景

**Files:**
- Test: `backend/internal/service/copilot_gateway_service_test.go`

新增两组测试：

### Step 4.1：写 TestCopilotInitiator_SessionCache_ChatCompletions

在 `TestXInitiatorHeader_ChatCompletions` 所在区域之后追加新函数：

```go
// TestCopilotInitiator_SessionCache tests that the second request within the
// same session uses agent quota even when it has no assistant/tool messages.
func TestCopilotInitiator_SessionCache_ChatCompletions(t *testing.T) {
    // Build a minimal CopilotGatewayService with a fresh session cache.
    svc := &CopilotGatewayService{
        sessionCache: newCopilotSessionCache(2 * time.Hour),
    }

    const accountID = int64(42)
    // A session key embedded in the OpenAI "user" field (legacy metadata format).
    const sessionUser = "user_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2_account__session_12345678-1234-1234-1234-123456789abc"

    firstTurnBody := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"user":"` + sessionUser + `"}`)
    secondTurnBody := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"follow-up"}],"user":"` + sessionUser + `"}`)

    // applySessionCache mimics the session override logic from the service.
    applySessionCache := func(c *gin.Context, body []byte) string {
        initiator := copilotInitiator(body)
        if rawSK := sessionKeyFromHeader(c); rawSK != "" {
            sk := fmt.Sprintf("%d:%s", accountID, rawSK)
            if svc.sessionCache.markAndCheckSeen(sk) {
                initiator = "agent"
            }
        } else if rawSK := extractSessionKeyFromOpenAIBody(body); rawSK != "" {
            sk := fmt.Sprintf("%d:%s", accountID, rawSK)
            if svc.sessionCache.markAndCheckSeen(sk) {
                initiator = "agent"
            }
        }
        return initiator
    }

    // Simulate first turn: session cache miss → original copilotInitiator result ("user").
    w1 := httptest.NewRecorder()
    c1, _ := gin.CreateTestContext(w1)
    c1.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
    if got := applySessionCache(c1, firstTurnBody); got != "user" {
        t.Fatalf("first turn: want user, got %s", got)
    }

    // Simulate second turn (same session, no assistant message): cache hit → "agent".
    w2 := httptest.NewRecorder()
    c2, _ := gin.CreateTestContext(w2)
    c2.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
    if got := applySessionCache(c2, secondTurnBody); got != "agent" {
        t.Fatalf("second turn (same session): want agent, got %s", got)
    }
}
```

- [ ] **Step 4.1：写 session cache 集成测试（ChatCompletions）**

### Step 4.2：写 TestCopilotInitiator_SessionCache_XSessionIDHeader 和跨账号隔离测试

```go
// TestCopilotInitiator_SessionCache_XSessionIDHeader verifies that any client
// can use X-Session-ID to opt into session-level quota saving.
func TestCopilotInitiator_SessionCache_XSessionIDHeader(t *testing.T) {
    svc := &CopilotGatewayService{
        sessionCache: newCopilotSessionCache(2 * time.Hour),
    }

    const accountID = int64(7)
    noAssistantBody := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`)

    newReqWithSession := func() *gin.Context {
        w := httptest.NewRecorder()
        c, _ := gin.CreateTestContext(w)
        c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
        c.Request.Header.Set("X-Session-ID", "custom-session-xyz")
        return c
    }

    applySession := func(c *gin.Context, body []byte) string {
        initiator := copilotInitiator(body)
        if rawSK := sessionKeyFromHeader(c); rawSK != "" {
            sk := fmt.Sprintf("%d:%s", accountID, rawSK)
            if svc.sessionCache.markAndCheckSeen(sk) {
                initiator = "agent"
            }
        }
        return initiator
    }

    c1 := newReqWithSession()
    if got := applySession(c1, noAssistantBody); got != "user" {
        t.Fatalf("first turn: want user, got %s", got)
    }

    c2 := newReqWithSession()
    if got := applySession(c2, noAssistantBody); got != "agent" {
        t.Fatalf("second turn: want agent, got %s", got)
    }
}

// TestCopilotInitiator_SessionCache_AccountIsolation verifies that two different
// accounts sharing the same X-Session-ID value do NOT pollute each other's cache.
func TestCopilotInitiator_SessionCache_AccountIsolation(t *testing.T) {
    svc := &CopilotGatewayService{
        sessionCache: newCopilotSessionCache(2 * time.Hour),
    }

    noAssistantBody := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`)

    makeReq := func() *gin.Context {
        w := httptest.NewRecorder()
        c, _ := gin.CreateTestContext(w)
        c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
        c.Request.Header.Set("X-Session-ID", "same-session-id-for-both")
        return c
    }

    apply := func(c *gin.Context, body []byte, accountID int64) string {
        initiator := copilotInitiator(body)
        if rawSK := sessionKeyFromHeader(c); rawSK != "" {
            sk := fmt.Sprintf("%d:%s", accountID, rawSK)
            if svc.sessionCache.markAndCheckSeen(sk) {
                initiator = "agent"
            }
        }
        return initiator
    }

    // Account 1, first request → cache miss → "user"
    if got := apply(makeReq(), noAssistantBody, 1); got != "user" {
        t.Fatalf("account1 first: want user, got %s", got)
    }
    // Account 2, same session ID, first request → still cache miss (isolated) → "user"
    if got := apply(makeReq(), noAssistantBody, 2); got != "user" {
        t.Fatalf("account2 first: want user (isolated from account1), got %s", got)
    }
    // Account 1, second request → cache hit → "agent"
    if got := apply(makeReq(), noAssistantBody, 1); got != "agent" {
        t.Fatalf("account1 second: want agent, got %s", got)
    }
}
```

- [ ] **Step 4.2：写 X-Session-ID header 测试和跨账号隔离测试**

### Step 4.3：运行新增测试，确认通过

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/ -run "TestCopilotInitiator_SessionCache|TestCopilotSessionCache|TestExtractSessionKey" -tags unit -v
```

期望：所有用例 PASS，包括 `TestCopilotSessionCache_AccountIsolation` 和 `TestCopilotInitiator_SessionCache_AccountIsolation`。

- [ ] **Step 4.3：确认测试通过**

### Step 4.4：运行全量相关测试，确认无回归

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/ -run "TestXInitiatorHeader|TestCopilotInitiator" -tags unit -timeout 120s -count=1
```

期望：无 FAIL。

- [ ] **Step 4.4：确认无回归**

### Step 4.5：提交

```bash
git add backend/internal/service/copilot_gateway_service_test.go \
        backend/internal/service/copilot_session_cache_test.go
# 按仓库提交协议提交（类型: Feature，中文描述）
```

- [ ] **Step 4.5：提交**

---

## Task 5：全量构建验证

### Step 5.1：全量构建

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

期望：无编译错误。

- [ ] **Step 5.1：全量构建通过**

### Step 5.2：运行全量 service + handler 测试

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/ ./internal/handler/ -tags unit -timeout 120s -count=1 2>&1 | tail -20
```

期望：`ok` 行，无 FAIL。

- [ ] **Step 5.2：全量测试通过**

### Step 5.3：提交

如有遗漏文件，按仓库提交协议最终提交。

- [ ] **Step 5.3：最终提交**

---

## 设计边界说明

### session cache 的局限性

| 场景 | 行为 | 说明 |
|------|------|------|
| 进程重启 | cache 清空，所有 session 视为首次 | 可接受：重启后第一轮 Premium，之后恢复免费 |
| 多实例部署 | 各实例独立 cache，跨实例首次仍消耗 Premium | 可接受：session 通常粘连同一实例 |
| 无 session 标识的请求 | 原有 assistant/tool 逻辑不变 | 完全兼容现有行为 |
| TTL 过期后续轮 | 视为新 session，消耗一次 Premium | 极少见（2小时内无活动才会触发） |

### 与 2026-04-12 方案的关系

两个方案完全正交，叠加后效果更好：
- **2026-04-12 方案**：CC sub-agent 首轮通过 system prompt 检测 → `agent`（即使 session cache 未命中）
- **本方案**：session 内第 2 轮起全部 → `agent`（无论 system prompt 内容）

两者都作用于 service 层的 `initiator` 计算，不会互相覆盖（sub-agent 首轮如果 session cache 未命中，仍可通过 system prompt 检测命中）。

### `X-Session-ID` header 规范

- 值：任意非空字符串，推荐使用 UUID 格式
- 作用域：**Cache key 会自动附加 `account.ID` 前缀**，因此同名 session 在不同账号之间完全隔离，不会相互污染配额
- 生命周期：由客户端维护，推荐与用户对话生命周期绑定
- 示例：`X-Session-ID: 550e8400-e29b-41d4-a716-446655440000`

### Responses API 路径（Codex CLI）的覆盖范围

`ForwardResponses` 的 `X-Initiator` 原先通过 `copilotInitiatorFromResponsesBody` 确定，该函数在 `previous_response_id` 非空时已返回 `"agent"`（已有链式续轮处理）。Session cache 在此之上再叠加：
- 如果请求携带 `X-Session-ID` 或 `user` 字段，且 session 已见过 → 覆盖为 `"agent"`
- 无 session 标识时，仍回退到 `copilotInitiatorFromResponsesBody` 的逻辑（`previous_response_id` 检测）
- `result.Initiator` 由 `ForwardResponses` 显式填充，handler analytics 通过 `result.Initiator` 读取，与实际上游 header 保持一致
