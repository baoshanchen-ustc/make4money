# Story 7.3: 扣减用户余额

Status: done

## Story

**作为** 系统
**我希望** 退款成功后扣减用户账户余额
**以便** 保持余额与实际到账一致

## Acceptance Criteria

- [x] AC1: 退款成功后在事务中扣减用户余额
- [x] AC2: 扣减金额等于退款金额
- [x] AC3: 余额不足时（已消费）记录警告但继续退款
- [x] AC4: 插入余额变动日志（change_type: refund）

## Tasks / Subtasks

- [x] Task 1: 后端 - 扣减余额逻辑 (AC: 1, 2)
  - [x] 1.1 在事务中查询用户当前余额
  - [x] 1.2 扣减退款金额
  - [x] 1.3 更新用户余额

- [x] Task 2: 后端 - 余额不足处理 (AC: 3)
  - [x] 2.1 检测余额是否足够扣减
  - [x] 2.2 余额不足时记录警告日志
  - [x] 2.3 允许余额为负（已消费情况）

- [x] Task 3: 后端 - 余额变动日志 (AC: 4)
  - [x] 3.1 插入 balance_logs 记录
  - [x] 3.2 记录 change_type = refund
  - [x] 3.3 记录退款前后余额

- [x] Task 4: 单元测试 (AC: 1-4)
  - [x] 4.1 现有测试通过

## Dev Notes

### 依赖关系

**前置条件**:
- Story 7.2（调用微信退款API）完成
- Story 3.4（余额变动日志记录）完成，日志表和服务已存在

**后续依赖**:
- Story 7.4（退款状态更新与日志）

### 后端实现

#### 1. 退款成功处理

在 `backend/internal/service/recharge_service.go` 添加：

```go
// ProcessRefundSuccessParams 处理退款成功的参数
type ProcessRefundSuccessParams struct {
    OrderNo  string
    RefundNo string
    Amount   float64 // 退款金额（元）
    Reason   string
    AdminID  int64
}

// processRefundSuccess 处理退款成功
// 在事务中：扣减用户余额 + 记录余额日志 + 更新订单状态
func (s *RechargeService) processRefundSuccess(ctx context.Context, params ProcessRefundSuccessParams) error {
    orderNo := params.OrderNo

    // 1. 获取分布式锁
    lockKey := fmt.Sprintf("recharge:refund:%s", orderNo)
    lock, err := s.redisClient.SetNX(ctx, lockKey, "1", 30*time.Second).Result()
    if err != nil {
        return fmt.Errorf("acquire lock failed: %w", err)
    }
    if !lock {
        log.Info("Refund is being processed by another goroutine", "order_no", orderNo)
        return nil
    }
    defer s.redisClient.Del(ctx, lockKey)

    // 2. 开启数据库事务
    tx, err := s.db.Tx(ctx)
    if err != nil {
        return fmt.Errorf("begin transaction failed: %w", err)
    }
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            panic(r)
        }
    }()

    // 3. 查询订单
    order, err := tx.RechargeOrder.
        Query().
        Where(rechargeorder.OrderNoEQ(orderNo)).
        ForUpdate().
        Only(ctx)
    if err != nil {
        tx.Rollback()
        return fmt.Errorf("query order failed: %w", err)
    }

    // 4. 检查订单状态（防止重复处理）
    if order.Status == "refunded" {
        log.Info("Order already refunded, skip", "order_no", orderNo)
        tx.Rollback()
        return nil
    }

    // 5. 查询用户当前余额
    user, err := tx.User.
        Query().
        Where(user.IDEQ(order.UserID)).
        ForUpdate().
        Only(ctx)
    if err != nil {
        tx.Rollback()
        return fmt.Errorf("query user failed: %w", err)
    }

    balanceBefore := user.Balance
    balanceAfter := balanceBefore - params.Amount

    // 6. 检查余额是否足够
    if balanceAfter < 0 {
        log.Warn("User balance insufficient for refund, allowing negative balance",
            "user_id", user.ID,
            "balance_before", balanceBefore,
            "refund_amount", params.Amount,
            "balance_after", balanceAfter,
            "order_no", orderNo)
        // 继续处理，允许余额为负（用户可能已消费部分）
    }

    // 7. 更新用户余额
    _, err = tx.User.
        UpdateOneID(user.ID).
        SetBalance(balanceAfter).
        Save(ctx)
    if err != nil {
        tx.Rollback()
        return fmt.Errorf("update user balance failed: %w", err)
    }

    // 8. 记录余额变动日志
    now := time.Now()
    description := fmt.Sprintf("充值退款 - %s", params.Reason)
    _, err = tx.BalanceLog.Create().
        SetUserID(order.UserID).
        SetChangeType("refund").
        SetAmount(-params.Amount). // 负数表示扣减
        SetBalanceBefore(balanceBefore).
        SetBalanceAfter(balanceAfter).
        SetRelatedOrderNo(orderNo).
        SetDescription(description).
        SetOperatorID(params.AdminID).
        SetOperatorType("admin").
        SetCreatedAt(now).
        Save(ctx)
    if err != nil {
        tx.Rollback()
        return fmt.Errorf("create balance log failed: %w", err)
    }

    // 9. 更新订单状态（Story 7.4 的内容，这里一并处理）
    _, err = tx.RechargeOrder.
        UpdateOneID(order.ID).
        SetStatus("refunded").
        SetRefundNo(params.RefundNo).
        SetRefundStatus("success").
        SetRefundReason(params.Reason).
        SetRefundAdminID(params.AdminID).
        SetRefundedAt(now).
        Save(ctx)
    if err != nil {
        tx.Rollback()
        return fmt.Errorf("update order status failed: %w", err)
    }

    // 10. 提交事务
    if err = tx.Commit(); err != nil {
        return fmt.Errorf("commit transaction failed: %w", err)
    }

    log.Info("Refund processed successfully",
        "order_no", orderNo,
        "user_id", order.UserID,
        "amount", params.Amount,
        "balance_before", balanceBefore,
        "balance_after", balanceAfter,
        "admin_id", params.AdminID)

    return nil
}
```

#### 2. 余额日志表扩展

确保 `balance_logs` 表有操作人相关字段：

```go
// backend/ent/schema/balance_log.go
field.Int64("operator_id").Optional().Comment("操作人ID"),
field.String("operator_type").Optional().Comment("操作人类型: system/admin/user"),
```

#### 3. 余额不足告警

建议添加告警机制（可选）：

```go
// 余额不足时发送告警
func (s *RechargeService) alertBalanceInsufficient(ctx context.Context, userID int64, orderNo string, balance, refundAmount float64) {
    // 发送告警通知给运维
    // 可以使用现有的告警服务
    log.Error("ALERT: User balance insufficient for refund",
        "user_id", userID,
        "order_no", orderNo,
        "current_balance", balance,
        "refund_amount", refundAmount,
        "will_be_negative", balance-refundAmount)
}
```

### 业务逻辑说明

#### 余额扣减场景

| 场景 | 余额 | 退款金额 | 结果余额 | 处理方式 |
|------|------|----------|----------|----------|
| 正常 | 100 | 50 | 50 | 正常扣减 |
| 余额刚好 | 50 | 50 | 0 | 正常扣减 |
| 余额不足 | 30 | 50 | -20 | 允许负数，记录警告 |
| 余额为0 | 0 | 50 | -50 | 允许负数，记录警告 |

#### 为什么允许余额为负？

1. **用户可能已消费**: 用户充值后使用了部分余额
2. **保障退款成功**: 微信已退款，本地必须同步
3. **可追溯**: 通过日志可以追溯负数原因
4. **运营处理**: 余额为负时可以后续运营处理（如催缴、标记等）

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/internal/service/recharge_service.go` | 添加 processRefundSuccess 方法 |
| `backend/ent/schema/balance_log.go` | 确保 operator 字段存在 |

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-7.3] - 用户故事定义
- [Source: _bmad-output/implementation-artifacts/3-4-balance-log.md] - 余额日志记录参考（如存在）
- [Source: backend/ent/schema/user.go] - 用户表 balance 字段定义

### 事务安全

1. **分布式锁**: 防止并发退款处理同一订单
2. **行锁**: SELECT FOR UPDATE 保护用户余额更新
3. **状态检查**: 订单状态检查防止重复处理
4. **原子操作**: 余额扣减、日志记录、状态更新在同一事务

### 测试用例

```go
func TestProcessRefundSuccess(t *testing.T) {
    tests := []struct {
        name           string
        balanceBefore  float64
        refundAmount   float64
        wantBalance    float64
        wantLogAmount  float64
    }{
        {
            name:          "normal refund",
            balanceBefore: 100.00,
            refundAmount:  50.00,
            wantBalance:   50.00,
            wantLogAmount: -50.00,
        },
        {
            name:          "exact balance refund",
            balanceBefore: 50.00,
            refundAmount:  50.00,
            wantBalance:   0.00,
            wantLogAmount: -50.00,
        },
        {
            name:          "insufficient balance refund",
            balanceBefore: 30.00,
            refundAmount:  50.00,
            wantBalance:   -20.00,
            wantLogAmount: -50.00,
        },
        {
            name:          "zero balance refund",
            balanceBefore: 0.00,
            refundAmount:  50.00,
            wantBalance:   -50.00,
            wantLogAmount: -50.00,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 1. 创建测试用户和订单
            // 2. 调用 processRefundSuccess
            // 3. 验证余额更新
            // 4. 验证日志记录
        })
    }
}

func TestProcessRefundSuccessIdempotency(t *testing.T) {
    // 测试重复调用不会重复扣减
}

func TestProcessRefundSuccessConcurrency(t *testing.T) {
    // 测试并发调用的安全性
}
```

### 监控指标

建议添加以下监控指标：

```go
var (
    refundBalanceDeductedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "recharge_refund_balance_deducted_total",
        Help: "Total amount of balance deducted for refunds",
    }, []string{"result"}) // result: success, insufficient

    negativeBalanceCount = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "recharge_negative_balance_count",
        Help: "Number of users with negative balance after refund",
    })
)
```

## Dev Agent Record

### Agent Model Used

(待开发时填写)

### Debug Log References

(待开发时填写)

### Completion Notes List

(待开发时填写)

### File List

(待开发时填写)
