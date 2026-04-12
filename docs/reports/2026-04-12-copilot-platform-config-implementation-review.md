# Copilot Platform Config 实现复审报告

## 基本信息
- 复审日期：2026-04-12
- 复审对象：`feature/copilot-platform-config`
- 复审位置：`.worktrees/feature/copilot-platform-config`
- 复审类型：实现后代码 review
- 复审方式：静态代码审查 + 本地构建/测试验证

## 复审结论（摘要）
- 功能主体已经落地，数据库、后端服务、路由、前端页面和编辑弹窗都已接通。
- 本次本地验证中，后端构建、后端测试、前端类型检查均通过。
- 但当前实现仍有 1 个高风险问题和 1 个中风险问题，建议先修复后再合并。

---

## Findings

### 1) HIGH: `/admin/accounts` ↔ `/admin/copilot/accounts` 路由切换时只刷新表格，不刷新统计列数据

#### 证据
- 路由切换监听里直接调用的是 `baseReload()`：
  - `frontend/src/views/admin/AccountsView.vue:654`
  - `frontend/src/views/admin/AccountsView.vue:658`
- 但当前页面真正完整的刷新逻辑在包装过的 `reload()` 中，它除了 `baseReload()` 之外，还会补拉：
  - `refreshTodayStatsBatch()`
  - `refreshCopilotQuotaBatch()`
  - 位置：`frontend/src/views/admin/AccountsView.vue:707`
  - 位置：`frontend/src/views/admin/AccountsView.vue:712`

#### 问题
这次为了解决同组件路由复用，新增了：
- `watch(() => route.meta.defaultPlatform, ...)`

方向是对的，但它绕过了页面原本的刷新封装，只做了列表数据重载，没有同步刷新依赖当前列表内容的统计列数据。

#### 风险
- 从 `/admin/accounts` 切到 `/admin/copilot/accounts` 时，账户列表会更新，但 `today_stats` / `copilot_quota` 可能仍停留在旧状态，或者对新列表显示为空。
- 用户会看到“列表是 Copilot 账号了，但统计列没更新”的不一致页面。

#### 建议
- route watch 里改调 `reload()`，不要直接调 `baseReload()`。
- 如果必须保留 `baseReload()`，则至少补齐和 `reload()` 一致的后续逻辑：
  - `resetAutoRefreshCache()`
  - `refreshTodayStatsBatch()`
  - `refreshCopilotQuotaBatch()`

---

### 2) MEDIUM: 平台配置 `PUT` 接口把“缺省字段”也当成清空处理，和文档契约不一致

#### 证据
- Handler 在绑定请求后，无条件把 4 个 `Set*` 标记都设为 `true`：
  - `backend/internal/handler/admin/copilot_platform_config_handler.go:58`
  - `backend/internal/handler/admin/copilot_platform_config_handler.go:64`
  - `backend/internal/handler/admin/copilot_platform_config_handler.go:69`
  - `backend/internal/handler/admin/copilot_platform_config_handler.go:72`
- 当前前端页面的确会发送完整 payload：
  - `frontend/src/views/admin/copilot/CopilotPlatformConfigView.vue:270`
  - `frontend/src/views/admin/copilot/CopilotPlatformConfigView.vue:275`

#### 问题
当前实现语义实际上是“整张卡片全量覆盖”。

这对当前前端页面没问题，但如果按设计文档/API 文案理解为：
- 字段可选
- 仅 `null` 表示清除

那么其他调用方如果只传一个字段，剩余未传字段会被误清空。

#### 风险
- 管理员用 curl 或后续别的客户端做部分更新时，容易把未传字段意外清掉。
- 实现语义和接口文档不一致，后续维护成本会上升。

#### 建议
- 二选一：
  1. 收敛文档，把该接口明确成“全量 replace 语义”，要求客户端始终传完整对象。
  2. 或修改 handler，区分“字段缺失”和“显式传 `null`”，让未传字段保持不变。

---

## 额外观察

### Worktree 不是干净状态
- 当前 worktree 仍有未提交修改：
  - `frontend/package-lock.json`
- 该文件包含 `axios` / `proxy-from-env` 的锁文件漂移，不在本次主 diff 列表里，但当前分支工作树并不干净。

这不是本次功能实现的核心 bug，但在准备合并前建议先确认它是否属于本功能的一部分，避免把无关依赖变更混入。

---

## 验证记录

### 已执行
1. 后端构建
```bash
go build ./...
```
结果：通过

2. 后端 service 测试
```bash
go test ./internal/service/...
```
结果：通过

3. 后端 handler 测试
```bash
go test ./internal/handler/...
```
结果：通过

4. 前端 TypeScript 类型检查
```bash
npm run typecheck
```
结果：通过

### 说明
- worktree 中不存在 `npm run type-check` 脚本，实际脚本名为 `typecheck`。

---

## 变更范围概览

本次复审覆盖的主要手写实现文件包括：
- `backend/internal/service/copilot_platform_config.go`
- `backend/internal/service/copilot_platform_config_service.go`
- `backend/internal/repository/copilot_platform_config_repo.go`
- `backend/internal/handler/admin/copilot_platform_config_handler.go`
- `backend/internal/service/copilot_gateway_service.go`
- `backend/internal/handler/copilot_gateway_handler.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/account.go`
- `frontend/src/api/admin/copilotPlatformConfig.ts`
- `frontend/src/router/index.ts`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/views/admin/copilot/CopilotPlatformConfigView.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/composables/useModelWhitelist.ts`

---

## 复审结论

### Recommendation
`REQUEST CHANGES`

### 原因
- 主功能已完成，且基本验证通过。
- 但当前仍有一个明确的前端路由切换刷新缺口，会造成页面局部数据不同步。
- 接口契约语义也还需要在实现或文档上统一。

### 建议的下一步
1. 修复 `AccountsView` route watch 中的刷新逻辑，确保统计列同步更新。
2. 明确 `PUT /admin/copilot/platform-config/:plan_type` 的语义是“全量覆盖”还是“部分更新”。
3. 清理或确认 `frontend/package-lock.json` 的未提交漂移。
