# Story 4.4: 订单过期自动处理

Status: done

## Story

**作为** 系统
**我希望** 自动将超时未支付的订单标记为过期
**以便** 清理无效订单

## Acceptance Criteria

- [x] AC1: 定时任务每分钟执行一次
- [x] AC2: 扫描 `status = pending` 且 `expired_at < now()` 的订单
- [x] AC3: 批量更新状态为 `expired`
- [x] AC4: 每次最多处理100条
- [x] AC5: 记录过期订单数量日志

## Tasks / Subtasks

- [x] Task 1: 创建定时任务调度器
- [x] Task 2: 实现过期订单扫描逻辑
- [x] Task 3: 实现批量状态更新

## Dev Notes

### 定时任务

使用 cron 或 ticker 定时执行

### 批量更新

```sql
UPDATE recharge_orders SET status = 'expired'
WHERE status = 'pending' AND expired_at < NOW()
LIMIT 100
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-4.4]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. **Repository 接口扩展** (`backend/internal/service/recharge_order_service.go`)
   - 添加 `MarkExpiredOrders(ctx, limit)` 方法到接口

2. **Repository 实现** (`backend/internal/repository/recharge_order_repo.go`)
   - 实现 `MarkExpiredOrders` 方法
   - 使用先查询 ID 再批量更新的方式支持 LIMIT
   - 查找 status='pending' 且 expire_at < now() 的订单

3. **调度器服务** (`backend/internal/service/order_expire_scheduler.go`)
   - 创建 `OrderExpireScheduler` 调度器
   - 使用 ticker 每 60 秒执行一次
   - 启动时立即执行一次
   - 每次最多处理 100 条订单
   - 记录过期订单数量日志

4. **Wire 集成** (`backend/internal/service/wire.go`, `backend/cmd/server/wire.go`)
   - 添加 `ProvideOrderExpireScheduler` provider
   - 在 cleanup 中添加调度器停止逻辑

### Code Review Notes

代码审查发现并修复以下问题：
- 修复 limit 参数未被使用的问题，改用先查询 ID 再批量更新的方式

### File List

- `backend/internal/service/recharge_order_service.go` (modified)
- `backend/internal/repository/recharge_order_repo.go` (modified)
- `backend/internal/service/order_expire_scheduler.go` (new)
- `backend/internal/service/wire.go` (modified)
- `backend/cmd/server/wire.go` (modified)
- `backend/cmd/server/wire_gen.go` (regenerated)
