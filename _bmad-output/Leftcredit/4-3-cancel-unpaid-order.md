# Story 4.3: 取消未支付订单

Status: done

## Story

**作为** 普通用户
**我希望** 取消未支付的充值订单
**以便** 放弃当前订单重新充值

## Acceptance Criteria

- [x] AC1: POST `/api/v1/recharge/orders/:order_no/cancel` 接口
- [x] AC2: 只能取消状态为 `pending` 的订单
- [x] AC3: 取消后状态变为 `failed`
- [x] AC4: 记录取消原因：用户主动取消
- [x] AC5: 已支付/已过期/已取消的订单不可再取消

## Tasks / Subtasks

- [x] Task 1: 创建取消订单 Handler
- [x] Task 2: 实现取消订单 Service 方法
- [x] Task 3: 实现并发安全的状态更新

## Dev Notes

### 并发控制

乐观锁或条件更新防止并发问题

```sql
UPDATE recharge_orders SET status = 'failed' WHERE order_no = ? AND status = 'pending'
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-4.3]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. **Handler 实现** (`backend/internal/handler/recharge/handler.go`)
   - 添加 `CancelOrder` 方法，处理 POST `/api/v1/recharge/orders/:order_no/cancel`
   - 返回 `CancelOrderResponse` 包含订单号、状态和消息

2. **Service 实现** (`backend/internal/service/recharge_order_service.go`)
   - 添加 `CancelOrder` 方法，处理取消订单业务逻辑
   - 校验用户权限（只能取消自己的订单）
   - 检查订单状态（只能取消 pending 状态）
   - 检查订单是否已过期（代码审查后添加）
   - 添加错误定义：`ErrOrderCannotBeCancelled`（400）、`ErrOrderCancelConflict`（409）

3. **Repository 实现** (`backend/internal/repository/recharge_order_repo.go`)
   - 添加 `UpdateStatusWithCondition` 方法，使用条件更新实现乐观锁
   - 返回受影响行数，0 表示并发冲突

4. **路由注册** (`backend/internal/server/routes/user.go`)
   - 添加 `POST /orders/:order_no/cancel` 路由

5. **单元测试** (`backend/internal/service/recharge_order_service_test.go`)
   - 添加 `TestCancelOrderErrors` 验证错误定义
   - 添加 `TestOrderStatusTransitions` 验证状态转换逻辑

### Code Review Notes

代码审查发现并修复以下问题：
- 添加订单过期检查，防止已过期但状态仍为 pending 的订单被取消

### File List

- `backend/internal/handler/recharge/handler.go` (modified)
- `backend/internal/service/recharge_order_service.go` (modified)
- `backend/internal/repository/recharge_order_repo.go` (modified)
- `backend/internal/server/routes/user.go` (modified)
- `backend/internal/service/recharge_order_service_test.go` (modified)
