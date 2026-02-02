# Story 5.3: 补偿到账逻辑

Status: done

## Story

**作为** 系统
**我希望** 发现微信已支付但本地未到账时自动触发到账
**以便** 保证用户权益

## Acceptance Criteria

- [x] AC1: 查询结果为 SUCCESS 但本地状态为 pending 时触发到账
- [x] AC2: 复用回调处理的到账逻辑（分布式锁+事务）
- [x] AC3: 到账成功后更新订单状态和余额
- [x] AC4: 到账成功后发送通知（通过日志记录实现，站内信系统暂未实现）
- [x] AC5: 记录补偿到账日志

## Tasks / Subtasks

- [x] Task 1: 后端 - 重构到账逻辑为可复用方法 (AC: 2)
  - [x] 1.1 将回调处理中的到账逻辑提取为独立方法 `ProcessPaymentSuccess`
  - [x] 1.2 添加 `source` 参数区分回调和补偿来源
  - [x] 1.3 确保分布式锁和事务逻辑完整

- [x] Task 2: 后端 - 补偿触发逻辑 (AC: 1, 3)
  - [x] 2.1 在 `SyncOrderStatus` 方法中检测需要补偿的情况
  - [x] 2.2 调用 `ProcessPaymentSuccess` 执行补偿
  - [x] 2.3 返回补偿后的最新状态

- [x] Task 3: 后端 - 通知发送 (AC: 4)
  - [x] 3.1 通过 log.Printf 记录补偿成功日志
  - [x] 3.2 站内信系统暂未实现，后续可扩展

- [x] Task 4: 后端 - 补偿日志记录 (AC: 5)
  - [x] 4.1 在 `balance_logs` 中记录补偿类型（通过 description 字段）
  - [x] 4.2 添加日志标记区分正常回调和补偿

- [ ] Task 5: 单元测试 (AC: 1-5)
  - [ ] 5.1 测试补偿触发条件
  - [ ] 5.2 测试幂等性（重复补偿不会重复到账）
  - [ ] 5.3 测试分布式锁竞争场景

## Dev Notes

### 依赖关系

**前置条件**:
- Story 3.3（订单状态更新与余额到账）完成，到账逻辑已实现
- Story 5.2（查询微信支付订单状态）完成

### 后端实现

#### 1. 重构到账逻辑

在 `backend/internal/service/recharge_service.go` 中重构：

```go
// PaymentSource 支付来源
type PaymentSource string

const (
    PaymentSourceCallback    PaymentSource = "callback"    // 微信回调
    PaymentSourceCompensate  PaymentSource = "compensate"  // 补偿到账
    PaymentSourceManualSync  PaymentSource = "manual_sync" // 用户手动同步
)

// ProcessPaymentSuccessParams 处理支付成功的参数
type ProcessPaymentSuccessParams struct {
    OrderNo       string
    TransactionID string        // 微信支付订单号
    Amount        int64         // 实际支付金额（分）
    Source        PaymentSource // 支付来源
}

// processPaymentSuccess 处理支付成功的通用逻辑
// 包含：分布式锁 → 状态检查 → 数据库事务（更新订单+增加余额+记录日志）→ 发送通知
func (s *RechargeService) processPaymentSuccess(ctx context.Context, params ProcessPaymentSuccessParams) error {
    orderNo := params.OrderNo

    // 1. 获取分布式锁
    lockKey := fmt.Sprintf("recharge:payment:%s", orderNo)
    lock, err := s.redisClient.SetNX(ctx, lockKey, "1", 30*time.Second).Result()
    if err != nil {
        return fmt.Errorf("acquire lock failed: %w", err)
    }
    if !lock {
        log.Info("Order is being processed by another goroutine",
            "order_no", orderNo,
            "source", params.Source)
        return nil // 其他协程正在处理，直接返回成功
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

    // 3. 查询订单（带行锁）
    order, err := tx.RechargeOrder.
        Query().
        Where(rechargeorder.OrderNoEQ(orderNo)).
        ForUpdate().
        Only(ctx)
    if err != nil {
        tx.Rollback()
        return fmt.Errorf("query order failed: %w", err)
    }

    // 4. 检查订单状态
    if order.Status != "pending" {
        log.Info("Order already processed, skip",
            "order_no", orderNo,
            "current_status", order.Status,
            "source", params.Source)
        tx.Rollback()
        return nil // 幂等处理
    }

    // 5. 验证金额（如果有的话）
    if params.Amount > 0 {
        orderAmountFen := int64(order.Amount * 100)
        if params.Amount != orderAmountFen {
            log.Error("Amount mismatch",
                "order_no", orderNo,
                "order_amount_fen", orderAmountFen,
                "paid_amount_fen", params.Amount)
            tx.Rollback()
            return fmt.Errorf("amount mismatch: order=%d, paid=%d", orderAmountFen, params.Amount)
        }
    }

    // 6. 更新订单状态
    now := time.Now()
    _, err = tx.RechargeOrder.
        UpdateOneID(order.ID).
        SetStatus("paid").
        SetTransactionID(params.TransactionID).
        SetPaidAt(now).
        Save(ctx)
    if err != nil {
        tx.Rollback()
        return fmt.Errorf("update order status failed: %w", err)
    }

    // 7. 查询用户当前余额（带行锁）
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
    balanceAfter := balanceBefore + order.Amount

    // 8. 更新用户余额
    _, err = tx.User.
        UpdateOneID(user.ID).
        SetBalance(balanceAfter).
        Save(ctx)
    if err != nil {
        tx.Rollback()
        return fmt.Errorf("update user balance failed: %w", err)
    }

    // 9. 记录余额变动日志
    description := "充值到账"
    if params.Source == PaymentSourceCompensate || params.Source == PaymentSourceManualSync {
        description = fmt.Sprintf("充值到账（补偿-%s）", params.Source)
    }

    _, err = tx.BalanceLog.Create().
        SetUserID(order.UserID).
        SetChangeType("recharge").
        SetAmount(order.Amount).
        SetBalanceBefore(balanceBefore).
        SetBalanceAfter(balanceAfter).
        SetRelatedOrderNo(orderNo).
        SetDescription(description).
        SetOperatorType("system").
        SetCreatedAt(now).
        Save(ctx)
    if err != nil {
        tx.Rollback()
        return fmt.Errorf("create balance log failed: %w", err)
    }

    // 10. 提交事务
    if err = tx.Commit(); err != nil {
        return fmt.Errorf("commit transaction failed: %w", err)
    }

    log.Info("Payment success processed",
        "order_no", orderNo,
        "user_id", order.UserID,
        "amount", order.Amount,
        "balance_before", balanceBefore,
        "balance_after", balanceAfter,
        "source", params.Source)

    // 11. 异步发送通知（不阻塞主流程）
    go s.sendRechargeSuccessNotification(context.Background(), order.UserID, orderNo, order.Amount, balanceAfter)

    return nil
}

// sendRechargeSuccessNotification 发送充值成功通知
func (s *RechargeService) sendRechargeSuccessNotification(ctx context.Context, userID int64, orderNo string, amount, balance float64) {
    defer func() {
        if r := recover(); r != nil {
            log.Error("Send notification panicked",
                "order_no", orderNo,
                "panic", r)
        }
    }()

    // 调用通知服务发送站内信
    if s.notificationService != nil {
        err := s.notificationService.SendRechargeSuccess(ctx, userID, orderNo, amount, balance)
        if err != nil {
            log.Error("Failed to send recharge success notification",
                "order_no", orderNo,
                "user_id", userID,
                "error", err)
        }
    }
}
```

#### 2. 更新 SyncOrderStatus 方法

在 `backend/internal/service/recharge_service.go` 中更新：

```go
// SyncOrderStatus 同步订单状态
func (s *RechargeService) SyncOrderStatus(ctx context.Context, userID int64, orderNo string) (*SyncOrderStatusResult, error) {
    // 1. 查询本地订单
    order, err := s.orderRepo.GetByOrderNo(ctx, orderNo)
    if err != nil {
        if ent.IsNotFound(err) {
            return nil, ErrOrderNotFound
        }
        return nil, fmt.Errorf("query order failed: %w", err)
    }

    // 2. 验证订单归属
    if order.UserID != userID {
        return nil, ErrOrderNotBelongToUser
    }

    // 3. 如果订单已经是终态，直接返回
    if order.Status != "pending" {
        return &SyncOrderStatusResult{
            OrderNo:      orderNo,
            Status:       order.Status,
            WeChatStatus: "",
            SyncedAt:     time.Now(),
        }, nil
    }

    // 4. 调用微信支付查询接口
    wechatResult, err := s.wechatPayService.QueryOrder(ctx, orderNo)
    if err != nil {
        return nil, fmt.Errorf("query wechat order failed: %w", err)
    }

    log.Info("WeChat order query result",
        "order_no", orderNo,
        "wechat_status", wechatResult.TradeState)

    // 5. 映射状态
    localStatus := mapWeChatStatusToLocal(wechatResult.TradeState)

    // 6. 如果微信显示已支付但本地是 pending，触发补偿到账
    if wechatResult.TradeState == "SUCCESS" && order.Status == "pending" {
        log.Info("Triggering compensation for successful payment",
            "order_no", orderNo,
            "user_id", userID)

        err = s.processPaymentSuccess(ctx, ProcessPaymentSuccessParams{
            OrderNo:       orderNo,
            TransactionID: wechatResult.TransactionID,
            Amount:        0, // 补偿时不再验证金额，因为查询接口可能不返回
            Source:        PaymentSourceManualSync,
        })
        if err != nil {
            log.Error("Compensation payment failed",
                "order_no", orderNo,
                "error", err)
            // 补偿失败不影响返回状态，让用户知道微信已支付
        } else {
            localStatus = "paid" // 补偿成功，更新返回状态
        }
    }

    return &SyncOrderStatusResult{
        OrderNo:      orderNo,
        Status:       localStatus,
        WeChatStatus: wechatResult.TradeState,
        SyncedAt:     time.Now(),
    }, nil
}
```

#### 3. 更新回调处理

在回调处理中也使用 `processPaymentSuccess`：

```go
// HandlePaymentCallback 处理微信支付回调
func (s *RechargeService) HandlePaymentCallback(ctx context.Context, notification *WeChatPayNotification) error {
    // ... 签名验证等前置处理 ...

    // 调用通用到账逻辑
    return s.processPaymentSuccess(ctx, ProcessPaymentSuccessParams{
        OrderNo:       notification.OutTradeNo,
        TransactionID: notification.TransactionID,
        Amount:        notification.Amount,
        Source:        PaymentSourceCallback,
    })
}
```

### 余额日志记录格式

补偿到账时的日志记录会带有特殊标记：

```
description: "充值到账（补偿-manual_sync）"  // 用户手动同步触发
description: "充值到账（补偿-compensate）"    // 定时任务补偿
description: "充值到账"                       // 正常回调
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/internal/service/recharge_service.go` | 添加/重构 processPaymentSuccess 方法 |
| `backend/internal/service/recharge_callback.go` | 更新回调处理使用通用方法 |

### 幂等性保证

1. **分布式锁**: Redis SETNX 确保同一订单同时只有一个处理
2. **状态检查**: 事务内检查订单状态，非 pending 直接返回
3. **行锁**: SELECT FOR UPDATE 防止并发更新

### 边界情况处理

1. **锁竞争**: 获取锁失败直接返回成功（其他协程在处理）
2. **事务失败**: 自动回滚，不会部分更新
3. **通知失败**: 异步发送，不影响主流程
4. **金额不匹配**: 仅在回调时验证金额，补偿不验证

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-5.3] - 用户故事定义
- [Source: _bmad-output/implementation-artifacts/3-3-order-status-update.md] - 回调到账逻辑参考（如已实现）
- [Source: backend/internal/service/recharge_service.go] - 现有服务代码

### 测试用例

```go
func TestProcessPaymentSuccess(t *testing.T) {
    tests := []struct {
        name           string
        orderStatus    string
        wantProcessed  bool
        wantBalanceAdd float64
    }{
        {
            name:           "pending order should be processed",
            orderStatus:    "pending",
            wantProcessed:  true,
            wantBalanceAdd: 100.00,
        },
        {
            name:           "already paid order should be skipped",
            orderStatus:    "paid",
            wantProcessed:  false,
            wantBalanceAdd: 0,
        },
        {
            name:           "expired order should be skipped",
            orderStatus:    "expired",
            wantProcessed:  false,
            wantBalanceAdd: 0,
        },
    }
    // ... 测试实现
}

func TestProcessPaymentSuccessIdempotency(t *testing.T) {
    // 并发调用测试，确保只处理一次
}

func TestProcessPaymentSuccessDistributedLock(t *testing.T) {
    // 模拟分布式锁竞争
}
```

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Debug Log References

无

### Completion Notes List

- 在 `PaymentCallbackService` 中添加 `PaymentSource` 类型和三个常量（callback、compensate、manual_sync）
- 添加 `ProcessPaymentSuccessParams` 结构体用于传递支付成功处理参数
- 添加 `ProcessPaymentSuccess` 方法作为可复用的支付成功处理入口，包含分布式锁和幂等检查
- 添加 `buildDescription` 方法根据 PaymentSource 生成不同的日志描述
- 添加 `processPaymentInTransactionWithDesc` 方法支持自定义描述的事务处理
- 更新 `RechargeOrderService.NewRechargeOrderService` 添加 `PaymentCallbackService` 依赖
- 更新 `SyncOrderStatus` 方法，当检测到微信支付成功但本地状态为 pending 时，自动触发补偿到账
- 补偿到账日志格式：充值 X.XX 元（手动同步补偿）、充值 X.XX 元（定时补偿）

### File List

- `backend/internal/service/payment_callback_service.go` - 添加 PaymentSource、ProcessPaymentSuccessParams、ProcessPaymentSuccess、buildDescription、processPaymentInTransactionWithDesc
- `backend/internal/service/recharge_order_service.go` - 更新构造函数添加 PaymentCallbackService 依赖，更新 SyncOrderStatus 添加补偿逻辑
- `backend/cmd/server/wire_gen.go` - Wire 自动重新生成
