# 分销邀请系统 — 执行计划总览

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现完整的 2 层分销系统，包含注册邀请奖励、消费返佣、分销员角色与控制台，全后台可配置。

**Architecture:** 5 张新数据库表（ent schema + SQL migration）→ service 层（ReferralService + worker）→ handler 层（user/distributor/admin）→ 前端（用户邀请页 + 分销员控制台 + 管理配置页）。采用 outbox 事件模式保障奖励发放可靠性；快照策略保证配置变更不影响存量关系。

**Tech Stack:** Go/Ent ORM/Gin/PostgreSQL/Redis，前端 Vue 3 + TypeScript + Pinia

**Spec:** `docs/superpowers/specs/2026-04-11-referral-system-design.md`

---

## 批次划分

| 批次 | 文件 | 内容 | 依赖 |
|------|------|------|------|
| **Batch 1** | `batch1-db-schema.md` | Ent schema + SQL migration | 无 |
| **Batch 2** | `batch2-domain-constants.md` | domain 常量 + service 领域类型 | Batch 1 |
| **Batch 3** | `batch3-repository.md` | referral_repo + wire 注册 | Batch 2 |
| **Batch 4** | `batch4-referral-service.md` | ReferralService 核心逻辑（邀请码生成、注册关系建立、配置读取） | Batch 3 |
| **Batch 5** | `batch5-auth-integration.md` | 注册流程集成（auth_service 挂载分销逻辑） | Batch 4 |
| **Batch 6** | `batch6-worker.md` | outbox worker（注册奖励 + 消费返佣结算） | Batch 4 |
| **Batch 7** | `batch7-billing-integration.md` | gateway_service 计费后触发返佣 worker | Batch 6 |
| **Batch 8** | `batch8-admin-service.md` | AdminService 扩展（封禁联动、角色修改）+ setting 配置 | Batch 4 |
| **Batch 9** | `batch9-handlers-routes.md` | 全部 handler + 路由注册 + middleware | Batch 8 |
| **Batch 10** | `batch10-frontend.md` | 前端全部页面、API 文件、路由、i18n | Batch 9 |

**执行顺序必须严格按批次。** 每个批次产出可编译的代码；每批次完成后运行 `go build ./...` 验证。

---

## 关键文件速查

### 后端新增
```
backend/ent/schema/referral_code.go
backend/ent/schema/referral_relation.go
backend/ent/schema/referral_signup_reward_event.go
backend/ent/schema/referral_commission_event.go
backend/ent/schema/distributor_application.go
backend/migrations/087_referral_system.sql
backend/internal/service/referral_service.go
backend/internal/repository/referral_repo.go
backend/internal/service/referral_reward_worker.go
backend/internal/handler/user/referral_handler.go
backend/internal/handler/distributor/overview_handler.go
backend/internal/handler/admin/referral_handler.go
backend/internal/handler/admin/distributor_application_handler.go
backend/internal/server/middleware/distributor_only.go
```

### 后端修改
```
backend/internal/domain/constants.go           (+RoleDistributor)
backend/internal/service/auth_service.go       (+referral 注册逻辑)
backend/internal/service/gateway_service.go    (+计费后触发返佣)
backend/internal/service/setting_service.go    (+11 个 referral SettingKey)
backend/internal/service/admin_service.go      (+封禁联动 + 角色修改)
backend/internal/server/routes/admin.go        (+新路由)
backend/internal/server/routes/user.go         (+新路由)
backend/internal/repository/wire.go            (+repo provider)
backend/internal/handler/wire.go               (+handler provider)
```

### 前端新增
```
frontend/src/api/user/referral.ts
frontend/src/api/distributor/index.ts
frontend/src/api/admin/referral.ts
frontend/src/views/user/ReferralView.vue
frontend/src/views/distributor/DistributorLayout.vue
frontend/src/views/distributor/OverviewView.vue
frontend/src/views/distributor/InviteesView.vue
frontend/src/views/admin/ReferralSettingsView.vue
frontend/src/views/admin/ReferralRelationsView.vue
frontend/src/views/admin/DistributorApplicationsView.vue
```

### 前端修改
```
frontend/src/views/auth/RegisterView.vue       (+邀请码字段)
frontend/src/router/index.ts                   (+路由 + distributor 守卫)
frontend/src/i18n/locales/zh.ts
frontend/src/i18n/locales/en.ts
```
