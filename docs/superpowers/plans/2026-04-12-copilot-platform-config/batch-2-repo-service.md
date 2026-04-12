# Copilot 平台配置 — Batch 2: Repository + Service

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 `CopilotPlatformConfigRepo`（数据库读写）和 `CopilotPlatformConfigService`（业务逻辑），包含 GetAll、GetByPlanType、UpdateByPlanType 三个方法。

**Architecture:** 遵循项目现有模式（参考 `model_pricing_repo.go` / `model_pricing_service.go`）。Service 层暴露接口供 Handler 和 GatewayService 调用；Repository 层负责 Ent ORM 操作。

**Tech Stack:** Go · entgo.io/ent

**前置条件:** Batch 1 已完成（`CopilotPlatformConfig` Ent schema 已生成）。

**Spec:** `docs/superpowers/specs/2026-04-12-copilot-platform-config-design.md` Section 2、Section 3。

---

### Task 3: Service 层类型定义

**Files:**
- Create: `backend/internal/service/copilot_platform_config.go`

- [ ] **Step 1: 创建 service 类型 + 错误变量文件**

```go
// backend/internal/service/copilot_platform_config.go
package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ErrCopilotPlatformConfigNotFound 是指定 plan_type 记录不存在时返回的错误，映射到 HTTP 404。
var ErrCopilotPlatformConfigNotFound = infraerrors.NotFound(
	"COPILOT_PLATFORM_CONFIG_NOT_FOUND",
	"copilot platform config not found",
)

// CopilotPlanType 枚举 Copilot plan 类型。
type CopilotPlanType = string

const (
	CopilotPlanIndividualFree    CopilotPlanType = "individual_free"
	CopilotPlanIndividualPro     CopilotPlanType = "individual_pro"
	CopilotPlanIndividualProPlus CopilotPlanType = "individual_pro_plus"
	CopilotPlanBusiness          CopilotPlanType = "business"
	CopilotPlanEnterprise        CopilotPlanType = "enterprise"
)

// AllCopilotPlanTypes 返回所有合法 plan_type，按展示顺序排列。
var AllCopilotPlanTypes = []CopilotPlanType{
	CopilotPlanIndividualFree,
	CopilotPlanIndividualPro,
	CopilotPlanIndividualProPlus,
	CopilotPlanBusiness,
	CopilotPlanEnterprise,
}

// CopilotPlatformConfigEntry 对应 copilot_platform_configs 表的一行。
// 所有配置字段均为指针类型，nil 表示"未设置，继承系统默认"。
type CopilotPlatformConfigEntry struct {
	ID               int64
	PlanType         string
	MaxOutputTokens  *int64
	MaxBodyKB        *int
	ModelMapping     map[string]string // nil = 未设置
	ModelWhitelist   []string          // nil = 未设置
}

// CopilotPlatformConfigRepository 是平台配置的存储接口。
type CopilotPlatformConfigRepository interface {
	// GetAll 返回所有 plan_type 的配置（固定 5 行）。
	GetAll(ctx context.Context) ([]CopilotPlatformConfigEntry, error)
	// GetByPlanType 按 plan_type 查询单条记录。
	GetByPlanType(ctx context.Context, planType string) (*CopilotPlatformConfigEntry, error)
	// Upsert 更新指定 plan_type 的配置（行始终存在，只做 UPDATE）。
	Upsert(ctx context.Context, planType string, patch CopilotPlatformConfigPatch) (*CopilotPlatformConfigEntry, error)
}

// CopilotPlatformConfigPatch 描述一次 PUT 操作中要更新的字段。
// 使用双指针语义：外层 nil 表示"此次不修改该字段"（前端未传），
// 内层 nil（*int64(nil)）表示"清除该字段"（前端传 null）。
// 对于本功能，PUT 请求体中所有字段均会被写入（允许清除），
// 所以 Patch 中只用单层指针，nil = 清除。
type CopilotPlatformConfigPatch struct {
	MaxOutputTokens *int64
	MaxBodyKB       *int
	ModelMapping    map[string]string // nil = 清除
	ModelWhitelist  []string          // nil = 清除（注意：空切片 [] 与 nil 不同，空切片=空白名单）
	// 使用 bool 标记哪些字段被显式传入（区分"未传"和"传 null"）
	SetMaxOutputTokens bool
	SetMaxBodyKB       bool
	SetModelMapping    bool
	SetModelWhitelist  bool
}
```

- [ ] **Step 2: 编译检查**

```bash
cd backend && go build ./internal/service/...
```

Expected: 无编译错误。

- [ ] **Step 3: Commit**

```bash
git add backend/internal/service/copilot_platform_config.go
git commit -m "Feature: 新增 CopilotPlatformConfig service 类型定义"
```

---

### Task 4: Repository 实现

**Files:**
- Create: `backend/internal/repository/copilot_platform_config_repo.go`

- [ ] **Step 1: 创建 Repository 实现文件**

```go
// backend/internal/repository/copilot_platform_config_repo.go
package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/copilotplatformconfig"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type copilotPlatformConfigRepository struct {
	client *dbent.Client
}

// NewCopilotPlatformConfigRepository 创建平台配置仓储。
func NewCopilotPlatformConfigRepository(client *dbent.Client) service.CopilotPlatformConfigRepository {
	return &copilotPlatformConfigRepository{client: client}
}

func (r *copilotPlatformConfigRepository) GetAll(ctx context.Context) ([]service.CopilotPlatformConfigEntry, error) {
	rows, err := r.client.CopilotPlatformConfig.Query().
		Order(dbent.Asc(copilotplatformconfig.FieldPlanType)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.CopilotPlatformConfigEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entToServiceConfig(row))
	}
	return out, nil
}

func (r *copilotPlatformConfigRepository) GetByPlanType(ctx context.Context, planType string) (*service.CopilotPlatformConfigEntry, error) {
	row, err := r.client.CopilotPlatformConfig.Query().
		Where(copilotplatformconfig.PlanType(planType)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrCopilotPlatformConfigNotFound
		}
		return nil, err
	}
	e := entToServiceConfig(row)
	return &e, nil
}

func (r *copilotPlatformConfigRepository) Upsert(ctx context.Context, planType string, patch service.CopilotPlatformConfigPatch) (*service.CopilotPlatformConfigEntry, error) {
	// 先查出现有行（行由迁移预插入，始终存在）
	existing, err := r.client.CopilotPlatformConfig.Query().
		Where(copilotplatformconfig.PlanType(planType)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrCopilotPlatformConfigNotFound
		}
		return nil, err
	}

	updater := r.client.CopilotPlatformConfig.UpdateOne(existing)

	if patch.SetMaxOutputTokens {
		if patch.MaxOutputTokens == nil {
			updater.ClearMaxOutputTokens()
		} else {
			updater.SetMaxOutputTokens(*patch.MaxOutputTokens)
		}
	}
	if patch.SetMaxBodyKB {
		if patch.MaxBodyKB == nil {
			updater.ClearMaxBodyKb()
		} else {
			updater.SetMaxBodyKb(*patch.MaxBodyKB)
		}
	}
	if patch.SetModelMapping {
		if patch.ModelMapping == nil {
			updater.ClearModelMapping()
		} else {
			updater.SetModelMapping(patch.ModelMapping)
		}
	}
	if patch.SetModelWhitelist {
		if patch.ModelWhitelist == nil {
			updater.ClearModelWhitelist()
		} else {
			updater.SetModelWhitelist(patch.ModelWhitelist)
		}
	}

	row, err := updater.Save(ctx)
	if err != nil {
		return nil, err
	}
	e := entToServiceConfig(row)
	return &e, nil
}

func entToServiceConfig(row *dbent.CopilotPlatformConfig) service.CopilotPlatformConfigEntry {
	return service.CopilotPlatformConfigEntry{
		ID:              row.ID,
		PlanType:        row.PlanType,
		MaxOutputTokens: row.MaxOutputTokens,
		MaxBodyKB:       row.MaxBodyKb,
		ModelMapping:    row.ModelMapping,
		ModelWhitelist:  row.ModelWhitelist,
	}
}
```

- [ ] **Step 2: 编译检查**

```bash
cd backend && go build ./internal/repository/...
```

Expected: 无编译错误。

- [ ] **Step 3: Commit**

```bash
git add backend/internal/repository/copilot_platform_config_repo.go
git commit -m "Feature: 新增 CopilotPlatformConfig Repository 实现"
```

---

### Task 5: Service 实现

**Files:**
- Create: `backend/internal/service/copilot_platform_config_service.go`
- Create: `backend/internal/service/copilot_platform_config_service_test.go`

- [ ] **Step 1: 写失败测试**

```go
// backend/internal/service/copilot_platform_config_service_test.go
package service

import (
	"context"
	"testing"
)

// stubCopilotPlatformConfigRepo 是 CopilotPlatformConfigRepository 的最小 stub。
type stubCopilotPlatformConfigRepo struct {
	entries []CopilotPlatformConfigEntry
	upsertFn func(planType string, patch CopilotPlatformConfigPatch) (*CopilotPlatformConfigEntry, error)
}

func (s *stubCopilotPlatformConfigRepo) GetAll(ctx context.Context) ([]CopilotPlatformConfigEntry, error) {
	return s.entries, nil
}

func (s *stubCopilotPlatformConfigRepo) GetByPlanType(ctx context.Context, planType string) (*CopilotPlatformConfigEntry, error) {
	for _, e := range s.entries {
		if e.PlanType == planType {
			return &e, nil
		}
	}
	return nil, ErrCopilotPlatformConfigNotFound
}

func (s *stubCopilotPlatformConfigRepo) Upsert(ctx context.Context, planType string, patch CopilotPlatformConfigPatch) (*CopilotPlatformConfigEntry, error) {
	return s.upsertFn(planType, patch)
}

func TestCopilotPlatformConfigService_GetAll(t *testing.T) {
	maxTokens := int64(8192)
	repo := &stubCopilotPlatformConfigRepo{
		entries: []CopilotPlatformConfigEntry{
			{PlanType: "individual_free"},
			{PlanType: "individual_pro", MaxOutputTokens: &maxTokens},
		},
	}
	svc := NewCopilotPlatformConfigService(repo)
	results, err := svc.GetAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[1].MaxOutputTokens == nil || *results[1].MaxOutputTokens != 8192 {
		t.Fatalf("expected MaxOutputTokens=8192 for individual_pro")
	}
}

func TestCopilotPlatformConfigService_GetByPlanType_NotFound(t *testing.T) {
	repo := &stubCopilotPlatformConfigRepo{entries: nil}
	svc := NewCopilotPlatformConfigService(repo)
	_, err := svc.GetByPlanType(context.Background(), "nonexistent")
	if err != ErrCopilotPlatformConfigNotFound {
		t.Fatalf("expected ErrCopilotPlatformConfigNotFound, got %v", err)
	}
}

func TestCopilotPlatformConfigService_UpdateByPlanType(t *testing.T) {
	called := false
	repo := &stubCopilotPlatformConfigRepo{
		upsertFn: func(planType string, patch CopilotPlatformConfigPatch) (*CopilotPlatformConfigEntry, error) {
			called = true
			if planType != "business" {
				t.Errorf("expected planType=business, got %s", planType)
			}
			maxTokens := int64(8192)
			return &CopilotPlatformConfigEntry{PlanType: planType, MaxOutputTokens: &maxTokens}, nil
		},
	}
	svc := NewCopilotPlatformConfigService(repo)
	maxTokens := int64(8192)
	result, err := svc.UpdateByPlanType(context.Background(), "business", CopilotPlatformConfigPatch{
		MaxOutputTokens:    &maxTokens,
		SetMaxOutputTokens: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("repo.Upsert was not called")
	}
	if result.MaxOutputTokens == nil || *result.MaxOutputTokens != 8192 {
		t.Fatalf("expected MaxOutputTokens=8192 in result")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

```bash
cd backend && go test ./internal/service/ -run TestCopilotPlatformConfigService -v
```

Expected: FAIL — `NewCopilotPlatformConfigService` 未定义。

- [ ] **Step 3: 实现 Service**

```go
// backend/internal/service/copilot_platform_config_service.go
package service

import "context"

// CopilotPlatformConfigService 管理 Copilot 平台级参数配置。
type CopilotPlatformConfigService struct {
	repo CopilotPlatformConfigRepository
}

func NewCopilotPlatformConfigService(repo CopilotPlatformConfigRepository) *CopilotPlatformConfigService {
	return &CopilotPlatformConfigService{repo: repo}
}

// GetAll 返回全部 5 个 plan_type 的配置。
func (s *CopilotPlatformConfigService) GetAll(ctx context.Context) ([]CopilotPlatformConfigEntry, error) {
	return s.repo.GetAll(ctx)
}

// GetByPlanType 返回指定 plan_type 的配置。
// 供 CopilotGatewayService 的继承逻辑调用。
func (s *CopilotPlatformConfigService) GetByPlanType(ctx context.Context, planType string) (*CopilotPlatformConfigEntry, error) {
	return s.repo.GetByPlanType(ctx, planType)
}

// UpdateByPlanType 更新指定 plan_type 的配置。
func (s *CopilotPlatformConfigService) UpdateByPlanType(ctx context.Context, planType string, patch CopilotPlatformConfigPatch) (*CopilotPlatformConfigEntry, error) {
	return s.repo.Upsert(ctx, planType, patch)
}
```

- [ ] **Step 4: 运行测试，确认通过**

```bash
cd backend && go test ./internal/service/ -run TestCopilotPlatformConfigService -v
```

Expected: 3 个测试全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/copilot_platform_config_service.go backend/internal/service/copilot_platform_config_service_test.go
git commit -m "Feature: 新增 CopilotPlatformConfigService 实现与测试"
```
