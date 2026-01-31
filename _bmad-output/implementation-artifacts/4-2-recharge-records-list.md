# Story 4.2: 充值记录列表

Status: done

## Story

**作为** 普通用户
**我希望** 查看我的充值记录列表
**以便** 了解历史充值情况

## Acceptance Criteria

- [x] AC1: GET `/api/v1/recharge/orders` 接口
- [x] AC2: 分页展示，每页10条，按创建时间倒序
- [x] AC3: 支持筛选条件：状态（可选）、时间范围（可选）
- [x] AC4: 返回字段：order_no, amount, status, created_at, paid_at
- [x] AC5: 返回分页信息：total, page, page_size

## Tasks / Subtasks

- [x] Task 1: 创建充值记录列表 Handler
- [x] Task 2: 实现分页查询 Service 方法
- [x] Task 3: 实现筛选条件处理
- [x] Task 4: 创建前端充值记录页面

## Dev Notes

### 分页实现

分页使用 Offset + Limit

### 前端页面

前端页面路径：`src/views/user/RechargeRecordsView.vue`

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-4.2]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. **后端接口实现**: 新增 `GET /api/v1/recharge/orders` 接口，支持分页查询用户的充值订单列表。

2. **分页查询**: 在 `RechargeOrderRepository` 中添加 `ListByUserID` 方法，支持按用户 ID 查询订单，按创建时间倒序排列。

3. **筛选条件**: 支持以下筛选条件：
   - `status`: 订单状态（pending/paid/failed/expired/cancelled）
   - `start_time`: 开始时间（RFC3339 或 YYYY-MM-DD 格式）
   - `end_time`: 结束时间（RFC3339 或 YYYY-MM-DD 格式）

4. **前端页面**: 创建 `RechargeRecordsView.vue` 页面，包含：
   - 筛选器（状态、日期范围）
   - 订单列表（订单号、金额、状态、时间）
   - 分页控件
   - 空状态提示

5. **路由配置**: 添加 `/recharge/records` 路由

6. **国际化**: 添加中英文翻译

### File List

**Backend:**
- `backend/internal/handler/recharge/handler.go` - 添加 ListOrders handler
- `backend/internal/service/recharge_order_service.go` - 添加 ListRechargeOrdersRequest、ListRechargeOrdersResult、ListUserOrders
- `backend/internal/repository/recharge_order_repo.go` - 添加 ListByUserID 方法
- `backend/internal/server/routes/user.go` - 注册新路由 GET /api/v1/recharge/orders

**Frontend:**
- `frontend/src/views/user/RechargeRecordsView.vue` - 新增充值记录列表页面
- `frontend/src/api/recharge.ts` - 添加 ListOrdersRequest、ListOrdersResponse、listOrders API
- `frontend/src/router/index.ts` - 添加 RechargeRecords 路由
- `frontend/src/i18n/locales/zh.ts` - 添加 rechargeRecords 翻译
- `frontend/src/i18n/locales/en.ts` - 添加 rechargeRecords 翻译

### Change Log

- 2026-02-01: Story 4-2 实现完成，所有 AC 验证通过

## Senior Developer Review (AI)

**Date:** 2026-02-01
**Reviewer:** Claude Opus 4.5
**Outcome:** ✅ Approved

### Review Notes

1. **代码质量**: 后端实现遵循现有项目模式，分层清晰（Handler → Service → Repository）
2. **安全性**: 权限校验正确，只能查询自己的订单
3. **性能**: 分页实现合理，限制最大 pageSize 为 100
4. **前端**: 使用 Composition API，代码结构清晰，支持响应式筛选
5. **国际化**: 完整的中英文翻译

### No Issues Found

代码实现符合 AC 要求，无需修复。
