# Admin User View — 设计文档

**日期**：2026-04-09  
**状态**：已确认，待实现

---

## 背景

管理员希望能在管理后台直接查看任意用户登录后所看到的完整页面内容，包括 Dashboard、API Keys、Usage、Subscriptions、Profile 等所有板块，并保留操作能力（可代替用户执行操作）。

---

## 方案

方案 A：独立路由 + 用户选择器 + 全板块 Tab + 保留操作能力。

---

## 路由

| 路径 | 说明 |
|------|------|
| `/admin/user-view` | 用户视图主页，用户未选时显示引导提示 |
| `/admin/user-view/:userId` | 查看指定用户的完整视图，支持直链分享/刷新恢复 |

两条路由均需 `requiresAuth: true` + `requiresAdmin: true`。

---

## 页面结构

```
┌─────────────────────────────────────────────────────────┐
│ Admin AppLayout（侧边栏 + Header）                        │
│  ┌──────────────────────────────────────────────────┐   │
│  │ [黄色 Banner] 管理员预览模式 — user@example.com   │   │
│  │               (ID: 42)          [切换用户 ▾]      │   │
│  ├──────────────────────────────────────────────────┤   │
│  │ Tab: Dashboard | API Keys | Usage | Subs | Profile│   │
│  ├──────────────────────────────────────────────────┤   │
│  │                                                  │   │
│  │  （对应 Tab 的用户组件，注入 userId 后渲染）         │   │
│  │                                                  │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### 顶部 Banner（常驻）

黄色警示样式，包含：
- 「管理员预览模式」标注
- 当前查看用户的邮箱和 ID
- 用户搜索下拉框（防抖 300ms，搜索邮箱/ID，复用现有 `adminUsageAPI.searchUsers`）
- 切换用户按钮

未选中用户时，Banner 区显示引导提示「请选择一个用户」。

### Tab 切换区

| Tab | 复用组件/内容 | 数据来源 |
|-----|------------|---------|
| Dashboard | `UserDashboardStats`、`UserDashboardCharts`、`UserDashboardRecentUsage`、`UserDashboardQuickActions` | 新增 admin 代理端点 |
| API Keys | `KeysView` 内组件（表格 + 操作） | 现有 `GET /admin/users/:id/api-keys` |
| Usage | `UsageView` 内组件 | 现有 `GET /admin/usage?user_id=:id` |
| Subscriptions | `SubscriptionsView` 内组件 | 现有或新增 admin subscriptions API |
| Profile | `ProfileView` 内组件 | 现有 `GET /admin/users/:id` |

### 操作能力

保留所有操作按钮（新建 Key、删除、修改密码等）。操作时调用 admin 代理 API，确保权限通过管理员身份鉴权。

---

## 后端改动

### 新增端点

| 端点 | 说明 |
|------|------|
| `GET /api/v1/admin/users/:id/dashboard/stats` | 返回与 `/usage/dashboard/stats` 相同结构，按指定 user_id 查询 |
| `GET /api/v1/admin/users/:id/dashboard/trend` | 代理 dashboard 趋势图数据 |
| `GET /api/v1/admin/users/:id/dashboard/models` | 代理 dashboard 模型统计数据 |

实现方式：在现有 admin user handler 中扩展，复用 `usageService.GetUserDashboardStats(ctx, userID)` 等方法，加上现有 `adminAuth` 中间件。

### 已有端点（可直接复用）

- `GET /api/v1/admin/usage?user_id=:id` — 使用记录
- `GET /api/v1/admin/users/:id` — 用户信息
- `GET /api/v1/admin/usage/search-users?q=xxx` — 用户搜索

---

## 前端改动

### 新增文件

```
frontend/src/
├── views/admin/
│   └── UserViewView.vue              # 页面容器（路由入口）
├── components/admin/user-view/
│   ├── UserViewBanner.vue            # 顶部 Banner + 用户选择器
│   └── UserViewTabs.vue              # Tab 切换 + 内容渲染
└── api/admin/
    └── userView.ts                   # admin 代理 API 封装（dashboard stats/trend/models）
```

### 修改文件

| 文件 | 改动说明 |
|------|---------|
| `router/index.ts` | 新增 `/admin/user-view` 和 `/admin/user-view/:userId` 两条路由 |
| `components/layout/AppSidebar.vue` | 侧边栏 Users 菜单项下方新增「User View」入口 |
| `views/admin/UsersView.vue` | 用户列表每行操作列新增「查看视图」快捷按钮，跳转到 `/admin/user-view/:id` |
| 用户组件（Dashboard/Keys/Usage/Subs/Profile） | 支持接收可选 `userId` prop；prop 存在时调用 admin API，否则调用原用户 API |

### 组件解耦策略

现有用户组件从 `authStore` 获取 userId。改造方式：接收可选 `userId?: number` prop，prop 存在时走 admin API 路径，否则保持现有逻辑不变。

---

## 侧边栏位置

在 admin 侧边导航 **Users** 菜单项下方添加「User View」入口，图标使用 `EyeIcon`。

---

## URL 设计

- 选中用户后 URL 更新为 `/admin/user-view/:userId`
- 页面刷新后根据 URL 参数自动恢复用户选择状态
- 支持直接分享链接给其他管理员

---

## 不在范围内

- JWT 代入 / impersonation 机制（安全风险高，非必要）
- 管理员代操作的审计日志（可后续迭代）
- Tab 切换动画（使用现有默认行为）
