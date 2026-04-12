# Copilot 平台配置 — Batch 4: 继承逻辑 + model_whitelist 账号选择

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `CopilotGatewayService`/`CopilotGatewayHandler` 中实现三层继承逻辑（账号级 → 平台配置 → 系统默认），并在 `isModelSupportedByAccount` Copilot 分支中加入 model_whitelist 检查。

**Architecture:**
- `CopilotGatewayService` 注入 `*CopilotPlatformConfigService`，通过 setter 注入（不破坏现有 wire）。
- 在 `effectiveCopilotMaxOutputTokensCap`（只读账号级）之后加 fallback 查平台配置。
- `CopilotGatewayHandler.checkCopilotBodySize` 的 fallback 链同理。
- `isModelSupportedByAccount` Copilot 分支：先查账号 model_whitelist，再查平台配置 model_whitelist，白名单非空则过滤。

**Tech Stack:** Go

**前置条件:** Batch 2（Service 已实现）、Batch 3（wire_gen.go 已更新）。

**Spec:** Section 2（继承逻辑）、Section 3（model_whitelist 账号选择）。

---

### Task 10: 注入 CopilotPlatformConfigService 到 CopilotGatewayService

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`
- Modify: `backend/cmd/server/wire_gen.go`

- [ ] **Step 1: 在 CopilotGatewayService struct 中添加字段（使用 setter 注入）**

找到 `type CopilotGatewayService struct {` 定义（顶部附近），在其字段列表末尾添加：

```go
platformConfigSvc *CopilotPlatformConfigService
```

找到 `NewCopilotGatewayService(...)` 构造函数（已存在），在函数体末尾、`return svc` 之前不做修改——使用 setter 注入代替。

在文件中添加一个 setter 方法（放在构造函数之后）：

```go
// SetPlatformConfigService 注入平台配置服务（供继承逻辑使用）。
func (s *CopilotGatewayService) SetPlatformConfigService(svc *CopilotPlatformConfigService) {
	s.platformConfigSvc = svc
}
```

- [ ] **Step 2: 在 wire_gen.go 中添加注入调用**

在 `wire_gen.go` 的 `copilotGatewayService` 构造行之后添加：

```go
copilotGatewayService.SetPlatformConfigService(copilotPlatformConfigService)
```

（`copilotPlatformConfigService` 变量已在 Batch 3 的 wire_gen.go 步骤中添加。）

- [ ] **Step 3: 编译检查**

```bash
cd backend && go build ./...
```

Expected: 无编译错误。

- [ ] **Step 4: Commit**

```bash
git add backend/internal/service/copilot_gateway_service.go \
        backend/cmd/server/wire_gen.go
git commit -m "Feature: CopilotGatewayService 注入 CopilotPlatformConfigService"
```

---

### Task 11: max_output_tokens 继承逻辑

**Files:**
- Modify: `backend/internal/service/copilot_gateway_service.go`
- Create: `backend/internal/service/copilot_gateway_service_platform_config_test.go`

- [ ] **Step 1: 写失败测试**

```go
// backend/internal/service/copilot_gateway_service_platform_config_test.go
package service

import (
	"context"
	"testing"
)

// stubCopilotPlatformConfigSvc 用于测试继承逻辑的 stub。
type stubCopilotPlatformConfigSvc struct {
	entries map[string]*CopilotPlatformConfigEntry
}

func (s *stubCopilotPlatformConfigSvc) GetByPlanType(ctx context.Context, planType string) (*CopilotPlatformConfigEntry, error) {
	if e, ok := s.entries[planType]; ok {
		return e, nil
	}
	return nil, ErrCopilotPlatformConfigNotFound
}

// TestEffectiveCopilotMaxOutputTokens_AccountLevelTakesPrecedence 验证账号级配置优先。
func TestEffectiveCopilotMaxOutputTokens_AccountLevelTakesPrecedence(t *testing.T) {
	svc := &CopilotGatewayService{}
	platformTokens := int64(4096)
	svc.platformConfigSvc = &CopilotPlatformConfigService{
		repo: &stubCopilotPlatformConfigRepo{
			entries: []CopilotPlatformConfigEntry{
				{PlanType: "individual_pro", MaxOutputTokens: &platformTokens},
			},
		},
	}
	account := &Account{
		Platform: PlatformCopilot,
		Credentials: map[string]any{
			"copilot_max_output_tokens": float64(16384),
			"plan_type":                 "individual_pro",
		},
	}
	cap, clamp := svc.effectiveCopilotMaxOutputTokensCap(context.Background(), account)
	if cap != 16384 {
		t.Errorf("expected account-level cap=16384, got %d", cap)
	}
	if !clamp {
		t.Error("expected clamp=true")
	}
}

// TestEffectiveCopilotMaxOutputTokens_FallsBackToPlatformConfig 验证账号未设置时继承平台配置。
func TestEffectiveCopilotMaxOutputTokens_FallsBackToPlatformConfig(t *testing.T) {
	svc := &CopilotGatewayService{}
	platformTokens := int64(4096)
	svc.platformConfigSvc = &CopilotPlatformConfigService{
		repo: &stubCopilotPlatformConfigRepo{
			entries: []CopilotPlatformConfigEntry{
				{PlanType: "business", MaxOutputTokens: &platformTokens},
			},
		},
	}
	account := &Account{
		Platform: PlatformCopilot,
		Credentials: map[string]any{
			"plan_type": "business",
			// copilot_max_output_tokens 未设置
		},
	}
	cap, clamp := svc.effectiveCopilotMaxOutputTokensCap(context.Background(), account)
	if cap != 4096 {
		t.Errorf("expected platform-config cap=4096, got %d", cap)
	}
	if !clamp {
		t.Error("expected clamp=true")
	}
}

// TestEffectiveCopilotMaxOutputTokens_FallsBackToSystemDefault 验证两者未设置时使用系统默认。
func TestEffectiveCopilotMaxOutputTokens_FallsBackToSystemDefault(t *testing.T) {
	svc := &CopilotGatewayService{}
	// platformConfigSvc 为 nil（未注入），模拟系统默认场景
	account := &Account{
		Platform:    PlatformCopilot,
		Credentials: map[string]any{},
	}
	cap, clamp := svc.effectiveCopilotMaxOutputTokensCap(context.Background(), account)
	if cap != defaultCopilotMaxOutputTokens {
		t.Errorf("expected system default cap=%d, got %d", defaultCopilotMaxOutputTokens, cap)
	}
	if !clamp {
		t.Error("expected clamp=true")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

```bash
cd backend && go test ./internal/service/ -run TestEffectiveCopilotMaxOutputTokens -v
```

Expected: FAIL — 方法签名不匹配（当前 `effectiveCopilotMaxOutputTokensCap` 不接受 context 和 svc）。

- [ ] **Step 3: 修改 effectiveCopilotMaxOutputTokensCap 函数签名以接受 context，加入中间层 fallback**

找到现有的 `effectiveCopilotMaxOutputTokensCap` 函数（位于 `copilot_gateway_service.go` 末尾附近）：

**旧代码：**
```go
func effectiveCopilotMaxOutputTokensCap(account *Account) (cap int, clamp bool) {
	if account == nil || account.Credentials == nil {
		return defaultCopilotMaxOutputTokens, true
	}
	raw, ok := account.Credentials[copilotMaxOutputTokensCredentialKey]
	if !ok || raw == nil {
		return defaultCopilotMaxOutputTokens, true
	}
	v := account.GetCredentialAsInt64(copilotMaxOutputTokensCredentialKey)
	if v <= 0 {
		return 0, false
	}
	if v > copilotMaxOutputTokensSanityUpperBound {
		v = copilotMaxOutputTokensSanityUpperBound
	}
	return int(v), true
}
```

**新代码（方法化到 CopilotGatewayService）：**
```go
func (s *CopilotGatewayService) effectiveCopilotMaxOutputTokensCap(ctx context.Context, account *Account) (cap int, clamp bool) {
	if account == nil || account.Credentials == nil {
		return s.platformOrSystemMaxOutputTokens(ctx, account)
	}
	raw, ok := account.Credentials[copilotMaxOutputTokensCredentialKey]
	if !ok || raw == nil {
		// 层 2：查平台配置
		return s.platformOrSystemMaxOutputTokens(ctx, account)
	}
	v := account.GetCredentialAsInt64(copilotMaxOutputTokensCredentialKey)
	if v <= 0 {
		return 0, false
	}
	if v > copilotMaxOutputTokensSanityUpperBound {
		v = copilotMaxOutputTokensSanityUpperBound
	}
	return int(v), true
}

// platformOrSystemMaxOutputTokens 先查平台配置，再回退到系统默认。
func (s *CopilotGatewayService) platformOrSystemMaxOutputTokens(ctx context.Context, account *Account) (int, bool) {
	if s.platformConfigSvc != nil && account != nil {
		planType := account.GetCredential("plan_type")
		if planType != "" {
			cfg, err := s.platformConfigSvc.GetByPlanType(ctx, planType)
			if err == nil && cfg != nil && cfg.MaxOutputTokens != nil {
				v := *cfg.MaxOutputTokens
				if v <= 0 {
					return 0, false
				}
				if v > copilotMaxOutputTokensSanityUpperBound {
					v = copilotMaxOutputTokensSanityUpperBound
				}
				return int(v), true
			}
		}
	}
	return defaultCopilotMaxOutputTokens, true
}
```

- [ ] **Step 4: 修复所有调用 effectiveCopilotMaxOutputTokensCap 的地方**

在 copilot_gateway_service.go 中搜索 `effectiveCopilotMaxOutputTokensCap(account)` 的调用（约有 2-3 处），全部改为 `s.effectiveCopilotMaxOutputTokensCap(ctx, account)`。确保 `ctx` 在调用处可用。

搜索命令确认调用数量：
```bash
grep -n "effectiveCopilotMaxOutputTokensCap" backend/internal/service/copilot_gateway_service.go
```

- [ ] **Step 5: 运行测试，确认通过**

```bash
cd backend && go test ./internal/service/ -run TestEffectiveCopilotMaxOutputTokens -v
```

Expected: 3 个测试全部 PASS。

- [ ] **Step 6: 编译检查**

```bash
cd backend && go build ./...
```

Expected: 无编译错误。

- [ ] **Step 7: Commit**

```bash
git add backend/internal/service/copilot_gateway_service.go \
        backend/internal/service/copilot_gateway_service_platform_config_test.go
git commit -m "Feature: max_output_tokens 加入平台配置继承逻辑（三层优先级）"
```

---

### Task 12: model_whitelist 账号选择逻辑

**Files:**
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/gateway_service_copilot_model_support_test.go`

背景：`isModelSupportedByAccount` 的 Copilot 分支当前是 `return true`（始终允许）。
现在加入：先查账号 `model_whitelist`，再查平台配置 `model_whitelist`，非空则过滤。

- [ ] **Step 1: 在 GatewayService 中注入 CopilotPlatformConfigService**

在 `gateway_service.go` 的 `GatewayService` struct 中找到字段定义，添加：

```go
copilotPlatformConfigSvc *CopilotPlatformConfigService
```

添加 setter 方法（在 struct 定义之后某处）：

```go
// SetCopilotPlatformConfigService 注入平台配置服务，供 whitelist 继承逻辑使用。
func (s *GatewayService) SetCopilotPlatformConfigService(svc *CopilotPlatformConfigService) {
	s.copilotPlatformConfigSvc = svc
}
```

- [ ] **Step 2: 在 wire_gen.go 中添加注入调用**

在 `gatewayService` 构造行之后（`gatewayService := service.NewGatewayService(...)` 那行的后面）添加：

```go
gatewayService.SetCopilotPlatformConfigService(copilotPlatformConfigService)
```

- [ ] **Step 3: 修改 isModelSupportedByAccount 的 Copilot 分支**

找到 `isModelSupportedByAccount` 函数中的 Copilot 分支（约第 3479 行）：

**旧代码：**
```go
// Copilot 账号的 model_mapping 仅用于转发时的名称重写（由 rewriteCopilotUpstreamModel 处理），
// 不应在账号选择阶段充当白名单过滤器。Copilot 账号支持所有模型，始终可调度。
if account.Platform == PlatformCopilot {
    return true
}
```

**新代码：**
```go
// Copilot 账号的 model_mapping 仅用于转发时的名称重写，不作为白名单。
// model_whitelist 才是白名单：账号级优先，平台级 fallback，均为空则允许所有模型。
if account.Platform == PlatformCopilot {
    return s.isCopilotModelInWhitelist(account, requestedModel)
}
```

在文件末尾（或该函数附近）添加辅助函数：

```go
// isCopilotModelInWhitelist 检查 requestedModel 是否在 Copilot 账号的白名单中。
// 优先级：账号级 model_whitelist > 平台配置 model_whitelist > 允许所有（return true）。
func (s *GatewayService) isCopilotModelInWhitelist(account *Account, requestedModel string) bool {
	if requestedModel == "" {
		return true
	}
	// 层 1：账号级 model_whitelist
	accountWhitelist := account.GetCopilotModelWhitelist()
	if len(accountWhitelist) > 0 {
		return containsString(accountWhitelist, requestedModel)
	}
	// 层 2：平台配置 model_whitelist（通过 plan_type 查询）
	if s.copilotPlatformConfigSvc != nil {
		planType := account.GetCredential("plan_type")
		if planType != "" {
			cfg, err := s.copilotPlatformConfigSvc.GetByPlanType(context.Background(), planType)
			if err == nil && cfg != nil && len(cfg.ModelWhitelist) > 0 {
				return containsString(cfg.ModelWhitelist, requestedModel)
			}
		}
	}
	// 层 3：无白名单，允许所有
	return true
}

// containsString 检查 slice 中是否包含 s（精确匹配）。
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: 在 Account 中添加 GetCopilotModelWhitelist 方法**

找到 `backend/internal/service/account.go`，在 `GetMaxBodyBytes()` 方法附近添加：

```go
// GetCopilotModelWhitelist 返回 Copilot 账号的模型白名单（从 credentials.model_whitelist 读取）。
// 返回 nil 表示未设置，返回空切片表示白名单为空（不允许任何模型）。
func (a *Account) GetCopilotModelWhitelist() []string {
	if a.Credentials == nil {
		return nil
	}
	raw, ok := a.Credentials["model_whitelist"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}
```

- [ ] **Step 5: 为现有测试添加 whitelist 用例**

在 `backend/internal/service/gateway_service_copilot_model_support_test.go` 末尾追加：

```go
// TestGatewayServiceIsModelSupportedByAccount_CopilotWhitelistFilters 验证
// 当账号设置了 model_whitelist 时，只有白名单内的模型被允许。
func TestGatewayServiceIsModelSupportedByAccount_CopilotWhitelistFilters(t *testing.T) {
	svc := &GatewayService{}
	account := &Account{
		Platform: PlatformCopilot,
		Credentials: map[string]any{
			"model_whitelist": []interface{}{"claude-sonnet-4.6", "gpt-4o"},
		},
	}
	allowed := []string{"claude-sonnet-4.6", "gpt-4o"}
	blocked := []string{"claude-opus-4.6", "claude-haiku-4.5"}
	for _, m := range allowed {
		if !svc.isModelSupportedByAccount(account, m) {
			t.Errorf("model %q should be allowed by whitelist", m)
		}
	}
	for _, m := range blocked {
		if svc.isModelSupportedByAccount(account, m) {
			t.Errorf("model %q should be blocked by whitelist", m)
		}
	}
}

// TestGatewayServiceIsModelSupportedByAccount_CopilotEmptyWhitelistAllowsAll 验证
// 当账号 model_whitelist 为空时允许所有模型。
func TestGatewayServiceIsModelSupportedByAccount_CopilotEmptyWhitelistAllowsAll(t *testing.T) {
	svc := &GatewayService{}
	account := &Account{
		Platform: PlatformCopilot,
		Credentials: map[string]any{
			// model_whitelist 未设置
		},
	}
	for _, m := range []string{"claude-sonnet-4.6", "gpt-4o", "claude-opus-4.6"} {
		if !svc.isModelSupportedByAccount(account, m) {
			t.Errorf("model %q should be allowed when whitelist is empty", m)
		}
	}
}
```

- [ ] **Step 6: 运行全部 Copilot model support 测试**

```bash
cd backend && go test ./internal/service/ -run TestGatewayServiceIsModelSupportedByAccount_Copilot -v
```

Expected: 4 个测试（含原有 2 个）全部 PASS。

- [ ] **Step 7: 全量编译检查**

```bash
cd backend && go build ./...
```

Expected: 无编译错误。

- [ ] **Step 8: Commit**

```bash
git add backend/internal/service/gateway_service.go \
        backend/internal/service/account.go \
        backend/internal/service/gateway_service_copilot_model_support_test.go \
        backend/cmd/server/wire_gen.go
git commit -m "Feature: Copilot model_whitelist 三层继承逻辑（账号级→平台配置→允许所有）"
```
