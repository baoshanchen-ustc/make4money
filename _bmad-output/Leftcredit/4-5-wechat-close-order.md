# Story 4.5: 微信关闭订单API调用

Status: done

## Story

**作为** 系统
**我希望** 订单过期时调用微信关闭订单API
**以便** 防止用户在过期后仍能支付成功

## Acceptance Criteria

- [x] AC1: 订单标记为expired后调用微信"关闭订单"API
- [x] AC2: API调用失败时记录错误日志但不阻塞过期处理
- [x] AC3: 使用goroutine异步调用，不阻塞主流程
- [x] AC4: 记录关闭结果到订单备注字段

## Tasks / Subtasks

- [x] Task 1: 实现微信关闭订单 Service 方法
- [x] Task 2: 在过期处理中异步调用关单
- [x] Task 3: 实现关单结果记录

## Dev Notes

### 微信SDK

调用微信支付SDK的关单方法

### 异步处理

使用 goroutine 异步处理避免超时

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-4.5]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. **微信关闭订单方法** (`backend/internal/service/wechat_pay_service.go`)
   - 添加 `CloseOrder(ctx, orderNo)` 方法
   - 使用 native.NativeApiService 调用微信关单 API
   - 记录关单结果日志

2. **Repository 接口扩展** (`backend/internal/service/recharge_order_service.go`)
   - 修改 `MarkExpiredOrders` 返回过期订单号列表
   - 添加 `AppendNotes(ctx, orderNo, notes)` 方法用于追加备注

3. **Repository 实现** (`backend/internal/repository/recharge_order_repo.go`)
   - 实现 `MarkExpiredOrders` 返回订单号列表
   - 实现 `AppendNotes` 追加订单备注

4. **调度器集成** (`backend/internal/service/order_expire_scheduler.go`)
   - 添加 `wechatPayService` 依赖
   - 订单过期后异步调用 `closeWeChatOrder`
   - 记录关单成功/失败到订单备注

5. **Wire 更新** (`backend/internal/service/wire.go`)
   - 更新 `ProvideOrderExpireScheduler` 添加 `WeChatPayService` 依赖

### Code Review Notes

代码审查通过，有一个低优先级建议：
- 可以考虑使用信号量限制并发 goroutine 数量（后续优化）

### File List

- `backend/internal/service/wechat_pay_service.go` (modified)
- `backend/internal/service/recharge_order_service.go` (modified)
- `backend/internal/repository/recharge_order_repo.go` (modified)
- `backend/internal/service/order_expire_scheduler.go` (modified)
- `backend/internal/service/wire.go` (modified)
- `backend/cmd/server/wire_gen.go` (regenerated)
