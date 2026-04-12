# Copilot 平台配置 — Batch 3: Handler + Routes + Wire

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 `CopilotPlatformConfigHandler`（GET /admin/copilot/platform-config、PUT /admin/copilot/platform-config/:plan_type），注册路由，并手动更新 wire_gen.go 完成依赖注入。

**Architecture:** Handler 遵循 `model_pricing_handler.go` 模式（请求/响应 struct + ShouldBindJSON）。路由注册到 `admin.go` 现有 `/admin/copilot` 分组。wire_gen.go 为手动维护文件，按现有模式添加三行。

**Tech Stack:** Go · Gin · entgo.io/ent

**前置条件:** Batch 2 已完成。

**Spec:** `docs/superpowers/specs/2026-04-12-copilot-platform-config-design.md` Section 3。

---

### Task 6: Handler

**Files:**
- Create: `backend/internal/handler/admin/copilot_platform_config_handler.go`

- [ ] **Step 1: 创建 Handler 文件**

```go
// backend/internal/handler/admin/copilot_platform_config_handler.go
package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CopilotPlatformConfigHandler 处理 Copilot 平台配置的 HTTP 请求。
type CopilotPlatformConfigHandler struct {
	svc *service.CopilotPlatformConfigService
}

func NewCopilotPlatformConfigHandler(svc *service.CopilotPlatformConfigService) *CopilotPlatformConfigHandler {
	return &CopilotPlatformConfigHandler{svc: svc}
}

// copilotPlatformConfigResponse 是返回给前端的 JSON 结构。
// null 字段表示该 plan_type 未设置默认值。
type copilotPlatformConfigResponse struct {
	PlanType        string            `json:"plan_type"`
	MaxOutputTokens *int64            `json:"max_output_tokens"`
	MaxBodyKB       *int              `json:"max_body_kb"`
	ModelMapping    map[string]string `json:"model_mapping"`
	ModelWhitelist  []string          `json:"model_whitelist"`
}

// updateCopilotPlatformConfigRequest 是 PUT 请求体。
// 所有字段可选；omitempty 不使用，以便区分"未传"和"传 null"。
// 使用自定义 UnmarshalJSON 实现双层指针语义（见下方）。
// 简化处理：所有字段均会被写入，null 表示清除。
type updateCopilotPlatformConfigRequest struct {
	MaxOutputTokens *int64            `json:"max_output_tokens"`
	MaxBodyKB       *int              `json:"max_body_kb"`
	ModelMapping    map[string]string `json:"model_mapping"`
	ModelWhitelist  []string          `json:"model_whitelist"`
}

// List 处理 GET /admin/copilot/platform-config
// 返回全部 5 个 plan_type 的配置，按 plan_type 字母序排列。
func (h *CopilotPlatformConfigHandler) List(c *gin.Context) {
	entries, err := h.svc.GetAll(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]copilotPlatformConfigResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, entryToConfigResponse(e))
	}
	response.Success(c, out)
}

// Update 处理 PUT /admin/copilot/platform-config/:plan_type
func (h *CopilotPlatformConfigHandler) Update(c *gin.Context) {
	planType := c.Param("plan_type")
	if !isValidCopilotPlanType(planType) {
		response.BadRequest(c, "invalid plan_type, must be one of: individual_free, individual_pro, individual_pro_plus, business, enterprise")
		return
	}

	var req updateCopilotPlatformConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 所有字段均设为"已传入"：前端每次保存一张卡片，提交该 plan_type 的完整状态。
	patch := service.CopilotPlatformConfigPatch{
		MaxOutputTokens:    req.MaxOutputTokens,
		MaxBodyKB:          req.MaxBodyKB,
		ModelMapping:       req.ModelMapping,
		ModelWhitelist:     req.ModelWhitelist,
		SetMaxOutputTokens: true,
		SetMaxBodyKB:       true,
		SetModelMapping:    true,
		SetModelWhitelist:  true,
	}

	updated, err := h.svc.UpdateByPlanType(c.Request.Context(), planType, patch)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, entryToConfigResponse(*updated))
}

func entryToConfigResponse(e service.CopilotPlatformConfigEntry) copilotPlatformConfigResponse {
	// 确保 JSON 序列化时空切片/空 map 输出 [] / {} 而非 null
	mapping := e.ModelMapping
	if mapping == nil {
		mapping = map[string]string{}
	}
	whitelist := e.ModelWhitelist
	if whitelist == nil {
		whitelist = []string{}
	}
	return copilotPlatformConfigResponse{
		PlanType:        e.PlanType,
		MaxOutputTokens: e.MaxOutputTokens,
		MaxBodyKB:       e.MaxBodyKB,
		ModelMapping:    mapping,
		ModelWhitelist:  whitelist,
	}
}

func isValidCopilotPlanType(planType string) bool {
	for _, valid := range service.AllCopilotPlanTypes {
		if planType == valid {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: 编译检查**

```bash
cd backend && go build ./internal/handler/admin/...
```

Expected: 无编译错误。

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handler/admin/copilot_platform_config_handler.go
git commit -m "Feature: 新增 CopilotPlatformConfigHandler"
```

---

### Task 7: 路由注册

**Files:**
- Modify: `backend/internal/server/routes/admin.go`

- [ ] **Step 1: 在 admin.go 中注册 Copilot 平台配置路由**

在 `registerCopilotAnalyticsRoutes` 函数（第 591 行附近）的同一 `copilot` group 内，或在 `RegisterAdminRoutes` 中新增一行调用。

找到 `RegisterAdminRoutes` 函数的 `registerCopilotOAuthRoutes(admin, h)` 调用旁，添加：

```go
// 在 RegisterAdminRoutes 函数体内，// Copilot OAuth 行之后添加：
// Copilot 平台配置
registerCopilotPlatformConfigRoutes(admin, h)
```

并在文件末尾添加新函数：

```go
func registerCopilotPlatformConfigRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	if h.Admin.CopilotPlatformConfig == nil {
		return
	}
	copilot := admin.Group("/copilot")
	{
		copilot.GET("/platform-config", h.Admin.CopilotPlatformConfig.List)
		copilot.PUT("/platform-config/:plan_type", h.Admin.CopilotPlatformConfig.Update)
	}
}
```

- [ ] **Step 2: 编译检查（此时 CopilotPlatformConfig 字段尚不存在于 AdminHandlers，会报错）**

先继续下一步。

---

### Task 8: 注册到 Handlers 结构体 + wire.go + wire_gen.go

**Files:**
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/repository/wire.go`
- Modify: `backend/cmd/server/wire_gen.go`

- [ ] **Step 1: 在 AdminHandlers 中添加字段**

修改 `backend/internal/handler/handler.go`，在 `AdminHandlers` struct 的 `ModelPricing` 字段之后添加：

```go
CopilotPlatformConfig *admin.CopilotPlatformConfigHandler
```

- [ ] **Step 2: 在 handler/wire.go 的 ProvideAdminHandlers 中添加参数**

修改 `backend/internal/handler/wire.go`：

在 `ProvideAdminHandlers` 函数签名中的 `modelPricingHandler *admin.ModelPricingHandler,` 之后添加参数：

```go
copilotPlatformConfigHandler *admin.CopilotPlatformConfigHandler,
```

在 `return &AdminHandlers{...}` 的 `ModelPricing: modelPricingHandler,` 之后添加：

```go
CopilotPlatformConfig: copilotPlatformConfigHandler,
```

在 `ProviderSet` 的 `admin.NewCopilotAnalyticsHandler,` 之后添加：

```go
admin.NewCopilotPlatformConfigHandler,
```

- [ ] **Step 3: 在 repository/wire.go 的 ProviderSet 中添加**

修改 `backend/internal/repository/wire.go`，在 `ProviderSet` 的 `// Encryptors` 注释之前添加：

```go
NewCopilotPlatformConfigRepository,
```

- [ ] **Step 4: 手动更新 wire_gen.go**

`backend/cmd/server/wire_gen.go` 是手动维护的生成文件。
在 `modelPricingHandler := admin.NewModelPricingHandler(modelPricingService)` 之后（约第 227 行），添加以下三行：

```go
copilotPlatformConfigRepository := repository.NewCopilotPlatformConfigRepository(client)
copilotPlatformConfigService := service.NewCopilotPlatformConfigService(copilotPlatformConfigRepository)
copilotPlatformConfigHandler := admin.NewCopilotPlatformConfigHandler(copilotPlatformConfigService)
```

然后找到 `adminHandlers := handler.ProvideAdminHandlers(...)` 这行（约第 230 行），在实参列表末尾（`modelPricingHandler` 之后）添加 `copilotPlatformConfigHandler`。

完整的 `ProvideAdminHandlers` 调用末尾应为：
```
..., copilotAnalyticsHandler, modelPricingHandler, copilotPlatformConfigHandler)
```

- [ ] **Step 5: 编译检查**

```bash
cd backend && go build ./...
```

Expected: 无编译错误。

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler/handler.go \
        backend/internal/handler/wire.go \
        backend/internal/repository/wire.go \
        backend/cmd/server/wire_gen.go \
        backend/internal/server/routes/admin.go
git commit -m "Feature: 注册 CopilotPlatformConfig 路由、Handler 和依赖注入"
```

---

### Task 9: Handler 集成测试（签名验证）

**Files:**
- Create: `backend/internal/handler/admin/copilot_platform_config_handler_test.go`

- [ ] **Step 1: 写测试**

```go
// backend/internal/handler/admin/copilot_platform_config_handler_test.go
package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// TestCopilotPlatformConfigHandler_SignatureCheck 验证 Handler 方法签名存在。
func TestCopilotPlatformConfigHandler_SignatureCheck(t *testing.T) {
	h := &CopilotPlatformConfigHandler{}
	_ = h.List
	_ = h.Update
}

// TestEntryToConfigResponse_NilToEmpty 验证 nil slice/map 被转换为空值而非 null。
func TestEntryToConfigResponse_NilToEmpty(t *testing.T) {
	e := service.CopilotPlatformConfigEntry{
		PlanType:       "individual_free",
		ModelMapping:   nil,
		ModelWhitelist: nil,
	}
	resp := entryToConfigResponse(e)
	if resp.ModelMapping == nil {
		t.Error("expected ModelMapping to be non-nil empty map, got nil")
	}
	if resp.ModelWhitelist == nil {
		t.Error("expected ModelWhitelist to be non-nil empty slice, got nil")
	}
}

// TestIsValidCopilotPlanType 验证合法和非法 plan_type 的判断。
func TestIsValidCopilotPlanType(t *testing.T) {
	valid := []string{"individual_free", "individual_pro", "individual_pro_plus", "business", "enterprise"}
	for _, pt := range valid {
		if !isValidCopilotPlanType(pt) {
			t.Errorf("expected %q to be valid", pt)
		}
	}
	if isValidCopilotPlanType("unknown") {
		t.Error("expected 'unknown' to be invalid")
	}
}
```

- [ ] **Step 2: 运行测试**

```bash
cd backend && go test ./internal/handler/admin/ -run TestCopilotPlatformConfig -v
```

Expected: 3 个测试全部 PASS。

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handler/admin/copilot_platform_config_handler_test.go
git commit -m "Feature: 新增 CopilotPlatformConfigHandler 测试"
```
