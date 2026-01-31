package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/redis/go-redis/v9"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
)

// PaymentCallbackService 支付回调业务处理服务
// 负责处理支付成功后的业务逻辑：
// - 分布式锁防止重复处理
// - 订单状态检查和更新
// - 金额验证
// - 用户余额增加
// - 余额变动日志记录
type PaymentCallbackService struct {
	entClient       *dbent.Client
	redisClient     *redis.Client
	orderRepo       RechargeOrderRepository
	userRepo        UserRepository
	balanceLogRepo  BalanceLogRepository
}

// NewPaymentCallbackService 创建支付回调业务处理服务
func NewPaymentCallbackService(
	entClient *dbent.Client,
	redisClient *redis.Client,
	orderRepo RechargeOrderRepository,
	userRepo UserRepository,
	balanceLogRepo BalanceLogRepository,
) *PaymentCallbackService {
	return &PaymentCallbackService{
		entClient:      entClient,
		redisClient:    redisClient,
		orderRepo:      orderRepo,
		userRepo:       userRepo,
		balanceLogRepo: balanceLogRepo,
	}
}

// Redis 分布式锁相关常量
const (
	callbackLockKeyPrefix = "recharge:callback:"
	callbackLockTTL       = 30 * time.Second
)

// ProcessPaymentResult 处理支付结果
type ProcessPaymentResult struct {
	Success        bool   // 是否处理成功
	AlreadyPaid    bool   // 订单是否已经支付过（幂等处理）
	OrderNo        string // 订单号
	TransactionID  string // 微信交易ID
	Amount         float64 // 订单金额
	UserID         int64  // 用户ID
	ErrorMessage   string // 错误信息
}

// ProcessPaymentCallback 处理支付回调
// 整个流程：
// 1. 获取分布式锁（防止重复处理）
// 2. 检查订单状态（幂等处理）
// 3. 验证回调金额
// 4. 在事务中更新订单状态和用户余额
func (s *PaymentCallbackService) ProcessPaymentCallback(
	ctx context.Context,
	transaction *payments.Transaction,
) *ProcessPaymentResult {
	result := &ProcessPaymentResult{}

	// 提取必要信息
	if transaction == nil || transaction.OutTradeNo == nil {
		result.ErrorMessage = "transaction or out_trade_no is nil"
		return result
	}
	orderNo := *transaction.OutTradeNo
	result.OrderNo = orderNo

	if transaction.TransactionId != nil {
		result.TransactionID = *transaction.TransactionId
	}

	// 检查交易状态是否为成功
	if transaction.TradeState == nil || *transaction.TradeState != "SUCCESS" {
		tradeState := ""
		if transaction.TradeState != nil {
			tradeState = *transaction.TradeState
		}
		result.ErrorMessage = fmt.Sprintf("trade_state is not SUCCESS: %s", tradeState)
		return result
	}

	// 1. 获取分布式锁
	lockKey := callbackLockKeyPrefix + orderNo
	locked, err := s.redisClient.SetNX(ctx, lockKey, 1, callbackLockTTL).Result()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("acquire lock failed: %v", err)
		return result
	}
	if !locked {
		// 其他进程正在处理，直接返回成功（幂等）
		result.Success = true
		result.AlreadyPaid = true
		log.Printf("[PaymentCallback] Order already being processed: order_no=%s", orderNo)
		return result
	}
	// 确保释放锁
	defer func() {
		if err := s.redisClient.Del(ctx, lockKey).Err(); err != nil {
			log.Printf("[PaymentCallback] Failed to release lock: order_no=%s, error=%v", orderNo, err)
		}
	}()

	// 2. 查询订单
	order, err := s.orderRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("get order failed: %v", err)
		return result
	}
	result.Amount = order.Amount
	result.UserID = order.UserID

	// 检查订单状态
	if order.Status != OrderStatusPending {
		// 订单已处理过（幂等）
		result.Success = true
		result.AlreadyPaid = true
		log.Printf("[PaymentCallback] Order already processed: order_no=%s, status=%s", orderNo, order.Status)
		return result
	}

	// 3. 验证金额（以分为单位）
	if transaction.Amount == nil || transaction.Amount.Total == nil {
		result.ErrorMessage = "transaction amount is nil"
		return result
	}
	callbackAmountInFen := *transaction.Amount.Total
	orderAmountInFen := int64(math.Round(order.Amount * 100))
	if callbackAmountInFen != orderAmountInFen {
		result.ErrorMessage = fmt.Sprintf("amount mismatch: callback=%d, order=%d", callbackAmountInFen, orderAmountInFen)
		log.Printf("[PaymentCallback] Amount mismatch: order_no=%s, callback_amount=%d, order_amount=%d",
			orderNo, callbackAmountInFen, orderAmountInFen)
		return result
	}

	// 4. 在事务中更新订单状态和用户余额
	err = s.processPaymentInTransaction(ctx, order, result.TransactionID)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("transaction failed: %v", err)
		return result
	}

	result.Success = true
	log.Printf("[PaymentCallback] Payment processed successfully: order_no=%s, user_id=%d, amount=%.2f",
		orderNo, order.UserID, order.Amount)
	return result
}

// processPaymentInTransaction 在事务中处理支付
func (s *PaymentCallbackService) processPaymentInTransaction(
	ctx context.Context,
	order *RechargeOrder,
	transactionID string,
) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("start transaction: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()

	// 使用事务上下文
	txCtx := dbent.NewTxContext(ctx, tx)

	// 4.1 查询用户当前余额（用于日志记录）
	user, err := s.userRepo.GetByID(txCtx, order.UserID)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("get user: %w", err)
	}
	balanceBefore := user.Balance

	// 4.2 更新订单状态
	now := time.Now()
	order.Status = OrderStatusPaid
	order.WeChatTransactionID = &transactionID
	order.PaidAt = &now

	if err := s.orderRepo.Update(txCtx, order); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update order: %w", err)
	}

	// 4.3 增加用户余额
	if err := s.userRepo.UpdateBalance(txCtx, order.UserID, order.Amount); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update user balance: %w", err)
	}

	// 4.4 插入余额变动日志
	balanceAfter := balanceBefore + order.Amount
	balanceLog := &BalanceLog{
		UserID:         order.UserID,
		ChangeType:     BalanceChangeTypeRecharge,
		Amount:         order.Amount,
		BalanceBefore:  balanceBefore,
		BalanceAfter:   balanceAfter,
		RelatedOrderNo: &order.OrderNo,
		Description:    fmt.Sprintf("充值 %.2f 元", order.Amount),
		OperatorID:     0,
		OperatorType:   OperatorTypeSystem,
	}
	if err := s.balanceLogRepo.Create(txCtx, balanceLog); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("create balance log: %w", err)
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
