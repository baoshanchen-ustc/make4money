# Story 3.4: 余额变动日志记录

Status: done

## Story

**作为** 系统
**我希望** 记录每笔余额变动的详细日志
**以便** 支持审计追溯和问题排查

## Acceptance Criteria

- [x] AC1: 插入 balance_logs 表记录
- [x] AC2: 记录字段：user_id, change_type(recharge), amount, balance_before, balance_after
- [x] AC3: 记录 related_order_no 关联订单号
- [x] AC4: 记录 operator_type(system) 和 description
- [x] AC5: 日志表只允许插入，不允许修改删除（应用层控制）

## Tasks / Subtasks

- [x] Task 1: 创建 `backend/ent/schema/balance_log.go` Schema
- [x] Task 2: 实现余额日志服务
- [x] Task 3: 在事务中插入日志

## Dev Notes

### 数据库表

参考 `_bmad-output/planning-artifacts/epics.md#数据库需求` 的 balance_logs 表设计

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.4]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Completion Notes List

1. 创建了 `balance_log.go` Ent Schema，包含所有必要字段
2. 在 `user.go` Schema 中添加了 `balance_logs` 反向边
3. 创建了 `balance_log_service.go` 定义模型和 Repository 接口
4. 创建了 `balance_log_repo.go` 实现 Repository
5. 在 `repository/wire.go` 中注册了 NewBalanceLogRepository
6. 修改了 `payment_callback_service.go`:
   - 添加 BalanceLogRepository 依赖
   - 在事务中先查询用户当前余额
   - 更新余额后插入余额变动日志
7. Repository 只实现了 Create 方法（只允许插入，不允许修改删除）

### File List

- backend/ent/schema/balance_log.go (新增)
- backend/ent/schema/user.go (修改 - 添加 balance_logs edge)
- backend/internal/service/balance_log_service.go (新增)
- backend/internal/repository/balance_log_repo.go (新增)
- backend/internal/repository/wire.go (修改)
- backend/internal/service/payment_callback_service.go (修改)
- backend/cmd/server/wire_gen.go (自动生成)
