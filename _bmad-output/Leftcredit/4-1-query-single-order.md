# Story 4.1: 查询单个订单详情

Status: done

## Story

**作为** 普通用户
**我希望** 查询某个充值订单的详细状态
**以便** 了解订单的支付进度

## Acceptance Criteria

- [x] AC1: GET `/api/v1/recharge/orders/:order_no` 接口
- [x] AC2: 返回字段：order_no, amount, status, payment_method, created_at, paid_at, expired_at
- [x] AC3: 只能查询自己的订单
- [x] AC4: 订单不存在时返回404

## Tasks / Subtasks

- [x] Task 1: 创建订单查询 Handler
- [x] Task 2: 实现订单详情 Service 方法
- [x] Task 3: 添加用户权限校验（user_id 匹配）
- [x] Task 4: 定义订单详情 DTO 响应结构

## Dev Notes

### 权限校验

查询时校验 user_id 匹配，使用 Ent 的 `Where` 条件

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-4.1]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Completion Notes List

1. 在 `recharge_order_service.go` 中添加 `GetUserOrder` 方法，带用户权限校验
2. 在 `handler.go` 中添加 `GetOrder` Handler 和 `OrderDetailResponse` DTO
3. 在 `routes/user.go` 中注册 GET `/recharge/orders/:order_no` 路由
4. 权限校验通过比对 order.UserID 与当前登录用户 ID 实现
5. 订单不存在或不属于该用户时返回 ErrRechargeOrderNotFound (404)

### File List

- backend/internal/service/recharge_order_service.go (修改)
- backend/internal/handler/recharge/handler.go (修改)
- backend/internal/server/routes/user.go (修改)
