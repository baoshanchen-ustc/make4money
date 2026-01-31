# Story 1.6: 充值菜单显示控制

Status: done

## Story

**作为** 普通用户
**我希望** 只有在充值功能启用时才看到充值入口
**以便** 避免看到无法使用的功能

## Acceptance Criteria

- [x] AC1: 前端根据 `wechat_pay.enabled` 控制菜单显隐（简化为单一开关）
- [x] AC2: GET `/api/v1/recharge/config` 接口返回 `enabled` 字段
- [x] AC3: 启用时侧边栏显示「余额充值」菜单项
- [x] AC4: 禁用时隐藏菜单项，直接访问路由跳转到首页

## Tasks / Subtasks

- [x] Task 1: 后端 - 充值配置公开接口 (AC: 2)
  - [x] 1.1 创建 `backend/internal/handler/recharge/handler.go`（RechargeHandler 结构体）
  - [x] 1.2 实现 GET `/api/v1/recharge/config` 接口
  - [x] 1.3 返回 enabled、min_amount、max_amount、default_amounts
  - [x] 1.4 注册路由到 `backend/internal/server/routes/auth.go`

- [x] Task 2: 后端 - Wire 依赖注入配置 (AC: 2)
  - [x] 2.1 在 `backend/internal/handler/wire.go` 注册 RechargeHandler
  - [x] 2.2 在 Handlers 结构体中添加 Recharge 字段
  - [x] 2.3 确保依赖注入链完整

- [x] Task 3: 前端 - API 客户端 (AC: 1)
  - [x] 3.1 创建 `frontend/src/api/recharge.ts`
  - [x] 3.2 定义 RechargeConfig 接口和 getConfig 方法

- [x] Task 4: 前端 - Pinia Store (AC: 1)
  - [x] 4.1 创建 `frontend/src/stores/recharge.ts`
  - [x] 4.2 实现 `fetchConfig()` 方法和 `isEnabled` getter
  - [x] 4.3 在 `frontend/src/stores/index.ts` 导出

- [x] Task 5: 前端 - 菜单控制 (AC: 3)
  - [x] 5.1 在 AppSidebar.vue 中条件渲染充值菜单项
  - [x] 5.2 使用 recharge store 的 isEnabled 状态

- [x] Task 6: 前端 - 路由定义与守卫 (AC: 4)
  - [x] 6.1 在 router/index.ts 添加充值相关路由
  - [x] 6.2 实现充值路由守卫，禁用时跳转到首页

- [x] Task 7: 前端 - i18n 国际化 (AC: 3)
  - [x] 7.1 添加中文翻译 `nav.recharge`
  - [x] 7.2 添加英文翻译 `nav.recharge`

- [x] Task 8: 单元测试 (AC: 1-4)
  - [x] 8.1 后端：测试 GetConfig 接口返回正确结构
  - [x] 8.2 后端：测试 enabled 状态正确反映 WeChatPayService

## Dev Notes

### 依赖关系

**前置条件**:
- Story 1.1（WeChatPayService 及 IsEnabled 方法已实现）✅

本 Story 实现前端与后端的联动，确保充值功能的可见性由配置统一控制。

### 实现说明

由于 Story 1.3（SettingService.GetRechargeSettings）尚未实现，本实现采用以下简化策略：

1. **enabled 状态**: 直接从 `WeChatPayService.IsEnabled()` 获取
2. **充值金额配置**: 使用硬编码默认值（min_amount=1, max_amount=1000, default_amounts=[10,50,100,200,500]）
3. **后续扩展**: Story 1.3 实现后，可轻松替换为数据库配置

### 设计决策

1. **公开接口**: `/api/v1/recharge/config` 无需认证，因为菜单显隐在用户登录前就需要决定
2. **LocalStorage 缓存**: 避免页面刷新时菜单闪烁
3. **优雅降级**: API 失败时使用缓存值，而非强制隐藏
4. **路由守卫**: 双重保护，即使用户手动访问 URL 也会被拦截
5. **简易模式兼容**: 充值菜单项在简易模式下隐藏（`hideInSimpleMode: true`）

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

无调试问题。

### Completion Notes List

- Task 1: 创建 `handler/recharge/handler.go`，实现 `GetConfig` 接口，返回 enabled/min_amount/max_amount/default_amounts
- Task 2: 在 `handler/wire.go` 添加 recharge import，更新 Handlers 结构体和 ProvideHandlers 函数，注册 NewRechargeHandler
- Task 3: 创建 `frontend/src/api/recharge.ts`，定义 RechargeConfig 接口和 getConfig 方法
- Task 4: 创建 `frontend/src/stores/recharge.ts`，实现带 localStorage 缓存的 isEnabled 状态管理
- Task 5: 修改 `AppSidebar.vue`，添加 WalletIcon 图标，在 userNavItems 和 personalNavItems 中条件渲染充值菜单
- Task 6: 修改 `router/index.ts`，添加 /recharge 路由和 requiresRecharge 路由守卫
- Task 7: 在 zh.ts 和 en.ts 中添加 nav.recharge 和 recharge.* 翻译
- Task 8: 创建 `handler/recharge/handler_test.go`，添加 2 个测试用例验证接口返回结构

### File List

**后端修改**:
- `backend/internal/handler/recharge/handler.go` (新建) - 充值配置接口处理器
- `backend/internal/handler/recharge/handler_test.go` (新建) - 单元测试（2个用例）
- `backend/internal/handler/handler.go` (修改) - 添加 recharge import 和 Recharge 字段
- `backend/internal/handler/wire.go` (修改) - 添加 recharge import、更新 ProvideHandlers、注册 NewRechargeHandler
- `backend/internal/server/routes/auth.go` (修改) - 注册 /api/v1/recharge/config 路由
- `backend/cmd/server/wire_gen.go` (自动生成) - Wire 代码重新生成

**前端修改**:
- `frontend/src/api/recharge.ts` (新建) - API 客户端
- `frontend/src/api/index.ts` (修改) - 导出 rechargeAPI
- `frontend/src/stores/recharge.ts` (新建) - Pinia Store
- `frontend/src/stores/index.ts` (修改) - 导出 useRechargeStore
- `frontend/src/components/layout/AppSidebar.vue` (修改) - 添加充值菜单项条件渲染
- `frontend/src/router/index.ts` (修改) - 添加 /recharge 路由和守卫
- `frontend/src/router/meta.d.ts` (修改) - 添加 requiresRecharge 类型定义
- `frontend/src/views/user/RechargeView.vue` (新建) - 充值页面占位符
- `frontend/src/i18n/locales/zh.ts` (修改) - 添加中文翻译
- `frontend/src/i18n/locales/en.ts` (修改) - 添加英文翻译

**配置修改**:
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (修改) - 更新状态为 in-progress

## Change Log

- 2026-02-01: 实现 Story 1.6 - 充值菜单显示控制。创建后端公开接口、前端 API 客户端、Pinia Store、侧边栏菜单条件渲染、路由守卫、i18n 翻译、单元测试。
