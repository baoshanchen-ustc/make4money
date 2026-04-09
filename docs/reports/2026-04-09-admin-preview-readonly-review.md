# Admin Preview Read-Only Review (2026-04-09)

## Scope
- frontend/src/views/user/KeysView.vue
- frontend/src/views/user/ProfileView.vue
- frontend/src/views/user/DashboardView.vue
- frontend/src/views/user/UsageView.vue

## Reviewer Summary
- 4/4 originally reported regressions were retested. ProfileView、DashboardView、UsageView 的管理员预览路径行为正确，均保持只读。
- KeysView 仍然允许管理员在预览用户时触发 `keysAPI.*` 的写操作：空态 CTA 和 Modal 没有屏蔽，导致管理员自己账号被误改，风险依旧存在。

## Findings

### 1. 管理员预览仍能在空态中新建 key（严重级别：高）
- 在 `adminUserId` 存在时，列表空态继续渲染 `EmptyState`，其 `@action` 仍然是 `showCreateModal = true`（`frontend/src/views/user/KeysView.vue:647-653`）。
- 同一文件中，创建/编辑/删除/重置的 Dialog 与实现逻辑没有任何禁用条件（`frontend/src/views/user/KeysView.vue:672-862` 及多个 handler），所以管理员浏览无 key 的用户时，仍可点击空态按钮打开表单，并向 `keysAPI.*` 发起写操作，修改到管理员自己的 key。
- 需要在管理员预览分支中整体隐藏或禁用所有写操作入口，包括空态 CTA、快捷键、模态框触发逻辑以及后端调用，确保 admin preview 100% 只读。

## Regression Retest 状态
| 缺陷 | 结果 | 说明 |
| --- | --- | --- |
| KeysView 管理员预览误调用 keysAPI | ❌ 未通过 | 见 Finding 1 |
| ProfileView 管理员预览仍渲染编辑表单 | ✅ 通过 | admin 分支只保留 StatCard/InfoCard（`frontend/src/views/user/ProfileView.vue:19-37`） |
| DashboardView 管理员预览余额与 simple mode 显示错误 | ✅ 通过 | `loadStats` 并发 `adminUsersAPI.getById`，把被预览用户 `balance` 与 `run_mode` 传入 `UserDashboardStats`（`frontend/src/views/user/DashboardView.vue:15-41`） |
| UsageView CSV 导出读取管理员自己日志 | ✅ 通过 | 导出分支根据 `adminUserId` 切换到 `adminUsageAPI.list({ user_id })`（`frontend/src/views/user/UsageView.vue:1159-1174`） |

## Verification
- 未执行自动化测试；仅做静态代码审查加逻辑推演。

## Recommendation
**状态：请求修改。** 请先在 KeysView 管理员分支彻底屏蔽所有写操作，再回归四个场景后重新提审。
