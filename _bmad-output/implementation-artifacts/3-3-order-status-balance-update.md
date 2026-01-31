# Story 3.3: 订单状态更新与余额到账

Status: done

## Story

**作为** 系统
**我希望** 回调验证通过后更新订单状态并增加用户余额
**以便** 完成充值流程

## Acceptance Criteria

- [x] AC1: 使用Redis分布式锁（key: recharge:callback:{order_no}，过期30秒）
- [x] AC2: 检查订单状态是否为 pending，非pending直接返回SUCCESS
- [x] AC3: 验证回调金额与订单金额一致
- [x] AC4: 在同一数据库事务中：更新订单状态、记录transaction_id、增加用户余额、插入余额变动日志
- [x] AC5: 事务成功后返回 SUCCESS
- [x] AC6: 事务失败时回滚并返回 FAIL

## Tasks / Subtasks

- [x] Task 1: 实现 Redis 分布式锁
- [x] Task 2: 实现订单状态检查
- [x] Task 3: 实现金额验证
- [x] Task 4: 实现数据库事务（Ent Tx）
- [x] Task 5: 实现用户余额更新（行锁）

## Dev Notes

### Redis 分布式锁

使用 SETNX + 过期时间（30秒）

### 行锁

使用 Ent 事务自动加锁

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.3]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. 创建 `PaymentCallbackService` 处理支付回调业务逻辑
2. 使用 Redis SETNX 实现分布式锁，防止重复处理
3. 实现幂等处理：检查订单状态，已支付订单直接返回成功
4. 使用 Ent 事务确保订单状态更新和用户余额更新的原子性
5. 验证回调金额与订单金额一致（以分为单位比较）
6. 更新 Handler 集成 PaymentCallbackService

### File List

- `backend/internal/service/payment_callback_service.go` - 新增支付回调业务处理服务
- `backend/internal/service/wire.go` - 注册新服务
- `backend/internal/handler/webhook/wechat_pay_handler.go` - 集成支付处理服务
- `backend/cmd/server/wire_gen.go` - 更新依赖注入代码
