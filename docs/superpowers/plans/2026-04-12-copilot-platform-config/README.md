# Copilot 平台配置 — 实现计划索引

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 Copilot 平台配置功能：按 plan_type 统一设置参数默认值 + 菜单重组 + EditAccountModal 分离 whitelist/mapping。

**Architecture:** 三层继承优先级（账号级 → 平台配置 → 系统默认）。新增 `copilot_platform_configs` 表，REST API（GET/PUT），Vue 平台配置页（5 张卡片）。

**Tech Stack:** Go · Gin · entgo.io/ent · PostgreSQL · Vue 3 · TypeScript · vue-i18n

**Spec:** `docs/superpowers/specs/2026-04-12-copilot-platform-config-design.md`

---

## 计划文件

| Batch | 文件 | 内容 | 任务 |
|-------|------|------|------|
| 1 | [batch-1-db-ent.md](batch-1-db-ent.md) | DB 迁移 + Ent Schema | Task 1-2 |
| 2 | [batch-2-repo-service.md](batch-2-repo-service.md) | Repository + Service | Task 3-5 |
| 3 | [batch-3-handler-routes-wire.md](batch-3-handler-routes-wire.md) | Handler + Routes + Wire | Task 6-9 |
| 4 | [batch-4-inheritance-whitelist.md](batch-4-inheritance-whitelist.md) | 继承逻辑 + model_whitelist 账号选择 | Task 10-12 |
| 5 | [batch-5-frontend-routes-sidebar-api.md](batch-5-frontend-routes-sidebar-api.md) | 前端路由 + 侧边栏 + API + i18n | Task 13-16 |
| 6 | [batch-6-views-modal.md](batch-6-views-modal.md) | CopilotPlatformConfigView + EditAccountModal | Task 17-20 |

## 执行顺序

Batch 1 → 2 → 3 → 4（后端独立）  
Batch 5 → 6（前端，依赖后端 API 但可并行开发）

## 关键文件速查

**后端新增：**
- `backend/migrations/087_copilot_platform_configs.sql`
- `backend/ent/schema/copilot_platform_config.go`
- `backend/internal/service/copilot_platform_config.go`（类型 + 常量）
- `backend/internal/service/copilot_platform_config_service.go`（Service）
- `backend/internal/repository/copilot_platform_config_repo.go`（Repository）
- `backend/internal/handler/admin/copilot_platform_config_handler.go`（Handler）

**后端修改：**
- `backend/internal/handler/handler.go`（添加 CopilotPlatformConfig 字段）
- `backend/internal/handler/wire.go`（ProvideAdminHandlers）
- `backend/internal/repository/wire.go`（ProviderSet）
- `backend/cmd/server/wire_gen.go`（手动注入 3 行 + SetPlatformConfigService + SetCopilotPlatformConfigService）
- `backend/internal/server/routes/admin.go`（新增 registerCopilotPlatformConfigRoutes）
- `backend/internal/service/copilot_gateway_service.go`（继承逻辑 + setter）
- `backend/internal/service/gateway_service.go`（whitelist 逻辑 + setter）
- `backend/internal/service/account.go`（GetCopilotModelWhitelist）

**前端新增：**
- `frontend/src/api/admin/copilotPlatformConfig.ts`
- `frontend/src/views/admin/copilot/CopilotPlatformConfigView.vue`
- `frontend/src/views/admin/copilot/CopilotAccountListView.vue`

**前端修改：**
- `frontend/src/router/index.ts`（路由重组）
- `frontend/src/components/layout/AppSidebar.vue`（菜单重组）
- `frontend/src/components/account/EditAccountModal.vue`（whitelist 字段）
- `frontend/src/i18n/locales/zh.ts` + `en.ts`（新词条）
