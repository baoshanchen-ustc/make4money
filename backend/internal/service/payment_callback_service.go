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
// - 汇率转换和用户余额增加
// - 余额变动日志记录
// - 异步发送充值成功通知
type PaymentCallbackService struct {
	entClient      *dbent.Client
	redisClient    *redis.Client
	orderRepo      RechargeOrderRepository
	userRepo       UserRepository
	balanceLogRepo BalanceLogRepository
	emailService   *EmailService
	settingService *SettingService
}

// NewPaymentCallbackService 创建支付回调业务处理服务
func NewPaymentCallbackService(
	entClient *dbent.Client,
	redisClient *redis.Client,
	orderRepo RechargeOrderRepository,
	userRepo UserRepository,
	balanceLogRepo BalanceLogRepository,
	emailService *EmailService,
	settingService *SettingService,
) *PaymentCallbackService {
	return &PaymentCallbackService{
		entClient:      entClient,
		redisClient:    redisClient,
		orderRepo:      orderRepo,
		userRepo:       userRepo,
		balanceLogRepo: balanceLogRepo,
		emailService:   emailService,
		settingService: settingService,
	}
}

// Redis 分布式锁相关常量
const (
	callbackLockKeyPrefix = "recharge:callback:"
	callbackLockTTL       = 30 * time.Second
)

// PaymentSource 支付来源
type PaymentSource string

const (
	PaymentSourceCallback   PaymentSource = "callback"    // 微信回调
	PaymentSourceCompensate PaymentSource = "compensate"  // 定时任务补偿
	PaymentSourceManualSync PaymentSource = "manual_sync" // 用户手动同步
)

// ProcessPaymentSuccessParams 处理支付成功的参数
type ProcessPaymentSuccessParams struct {
	OrderNo       string        // 订单号
	TransactionID string        // 微信支付订单号
	AmountInFen   int64         // 实际支付金额（分），0 表示不验证
	Source        PaymentSource // 支付来源
}

// ProcessPaymentResult 处理支付结果
type ProcessPaymentResult struct {
	Success        bool    // 是否处理成功
	AlreadyPaid    bool    // 订单是否已经支付过（幂等处理）
	OrderNo        string  // 订单号
	TransactionID  string  // 微信交易ID
	Amount         float64 // 订单金额（CNY）
	CreditedAmount float64 // 实际到账额度（汇率转换后）
	ExchangeRate   float64 // 使用的汇率
	UserID         int64   // 用户ID
	ErrorMessage   string  // 错误信息
}

// ProcessPaymentCallback 处理支付回调
// 整个流程：
// 1. 获取分布式锁（防止重复处理）
// 2. 检查订单状态（幂等处理）
// 3. 验证回调金额
// 4. 获取汇率并计算到账额度
// 5. 在事务中更新订单状态和用户余额
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

	// 4. 获取汇率配置并计算到账额度
	exchangeRate, creditedAmount := s.getExchangeRateAndCalculateCredit(ctx, order.Amount)
	result.CreditedAmount = creditedAmount
	result.ExchangeRate = exchangeRate

	// 5. 构建描述
	description := s.buildDescriptionWithRate(order.Amount, creditedAmount, exchangeRate, PaymentSourceCallback)

	// 6. 在事务中更新订单状态和用户余额
	err = s.processPaymentInTransactionWithDesc(ctx, order, result.TransactionID, description, creditedAmount, exchangeRate)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("transaction failed: %v", err)
		return result
	}

	result.Success = true
	log.Printf("[PaymentCallback] Payment processed successfully: order_no=%s, user_id=%d, amount=%.2f, credited=%.2f, rate=%.4f",
		orderNo, order.UserID, order.Amount, creditedAmount, exchangeRate)

	// 7. 异步发送充值成功通知
	go s.sendRechargeSuccessNotification(result)

	return result
}

// ProcessPaymentSuccess 处理支付成功的通用方法
// 可被回调处理和补偿逻辑复用
// 包含：分布式锁 → 订单查询 → 状态检查 → 金额验证（可选）→ 汇率转换 → 事务处理
func (s *PaymentCallbackService) ProcessPaymentSuccess(
	ctx context.Context,
	params ProcessPaymentSuccessParams,
) *ProcessPaymentResult {
	result := &ProcessPaymentResult{
		OrderNo: params.OrderNo,
	}

	// 1. 获取分布式锁
	lockKey := callbackLockKeyPrefix + params.OrderNo
	locked, err := s.redisClient.SetNX(ctx, lockKey, 1, callbackLockTTL).Result()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("acquire lock failed: %v", err)
		return result
	}
	if !locked {
		// 其他进程正在处理，直接返回成功（幂等）
		result.Success = true
		result.AlreadyPaid = true
		log.Printf("[PaymentCallback] Order already being processed: order_no=%s, source=%s",
			params.OrderNo, params.Source)
		return result
	}
	// 确保释放锁
	defer func() {
		if err := s.redisClient.Del(ctx, lockKey).Err(); err != nil {
			log.Printf("[PaymentCallback] Failed to release lock: order_no=%s, error=%v",
				params.OrderNo, err)
		}
	}()

	// 2. 查询订单
	order, err := s.orderRepo.GetByOrderNo(ctx, params.OrderNo)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("get order failed: %v", err)
		return result
	}
	result.Amount = order.Amount
	result.UserID = order.UserID
	result.TransactionID = params.TransactionID

	// 3. 检查订单状态
	if order.Status != OrderStatusPending {
		// 订单已处理过（幂等）
		result.Success = true
		result.AlreadyPaid = true
		log.Printf("[PaymentCallback] Order already processed: order_no=%s, status=%s, source=%s",
			params.OrderNo, order.Status, params.Source)
		return result
	}

	// 4. 验证金额（仅当 AmountInFen > 0 时验证）
	if params.AmountInFen > 0 {
		orderAmountInFen := int64(math.Round(order.Amount * 100))
		if params.AmountInFen != orderAmountInFen {
			result.ErrorMessage = fmt.Sprintf("amount mismatch: callback=%d, order=%d",
				params.AmountInFen, orderAmountInFen)
			log.Printf("[PaymentCallback] Amount mismatch: order_no=%s, callback_amount=%d, order_amount=%d, source=%s",
				params.OrderNo, params.AmountInFen, orderAmountInFen, params.Source)
			return result
		}
	}

	// 5. 获取汇率配置并计算到账额度
	exchangeRate, creditedAmount := s.getExchangeRateAndCalculateCredit(ctx, order.Amount)
	result.CreditedAmount = creditedAmount
	result.ExchangeRate = exchangeRate

	// 6. 构建描述（包含汇率信息）
	description := s.buildDescriptionWithRate(order.Amount, creditedAmount, exchangeRate, params.Source)

	// 7. 在事务中更新订单状态和用户余额
	err = s.processPaymentInTransactionWithDesc(ctx, order, params.TransactionID, description, creditedAmount, exchangeRate)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("transaction failed: %v", err)
		return result
	}

	result.Success = true
	log.Printf("[PaymentCallback] Payment processed successfully: order_no=%s, user_id=%d, amount=%.2f, credited=%.2f, rate=%.4f, source=%s",
		params.OrderNo, order.UserID, order.Amount, creditedAmount, exchangeRate, params.Source)

	// 8. 异步发送充值成功通知
	go s.sendRechargeSuccessNotification(result)

	return result
}

// getExchangeRateAndCalculateCredit 获取汇率并计算到账额度
// 返回：汇率、到账额度
func (s *PaymentCallbackService) getExchangeRateAndCalculateCredit(ctx context.Context, amount float64) (exchangeRate, creditedAmount float64) {
	exchangeRate = DefaultRechargeExchangeRate
	if s.settingService != nil {
		settings, err := s.settingService.GetRechargeSettings(ctx)
		if err != nil {
			log.Printf("[PaymentCallback] Failed to get recharge settings, using default rate: %v", err)
		} else if settings.ExchangeRate > 0 {
			exchangeRate = settings.ExchangeRate
		}
	}
	// 计算到账额度：支付金额 / 汇率，四舍五入保留两位小数
	creditedAmount = math.Round(amount/exchangeRate*100) / 100
	return
}

// buildDescriptionWithRate 根据支付来源和汇率构建日志描述
func (s *PaymentCallbackService) buildDescriptionWithRate(payAmount, creditedAmount, exchangeRate float64, source PaymentSource) string {
	var sourceDesc string
	switch source {
	case PaymentSourceCompensate:
		sourceDesc = "（定时补偿）"
	case PaymentSourceManualSync:
		sourceDesc = "（手动同步补偿）"
	default:
		sourceDesc = ""
	}

	// 如果汇率为 1:1，使用简化描述
	if exchangeRate == 1.0 {
		return fmt.Sprintf("充值 $%.2f%s", creditedAmount, sourceDesc)
	}

	// 非 1:1 汇率，显示转换信息
	return fmt.Sprintf("充值 ¥%.2f → $%.2f（汇率 %.4f:1）%s", payAmount, creditedAmount, exchangeRate, sourceDesc)
}

// processPaymentInTransactionWithDesc 在事务中处理支付（支持自定义描述和汇率转换）
func (s *PaymentCallbackService) processPaymentInTransactionWithDesc(
	ctx context.Context,
	order *RechargeOrder,
	transactionID string,
	description string,
	creditedAmount float64,
	exchangeRate float64,
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

	// 查询用户当前余额（用于日志记录）
	user, err := s.userRepo.GetByID(txCtx, order.UserID)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("get user: %w", err)
	}
	balanceBefore := user.Balance

	// 更新订单状态（包含汇率信息）
	now := time.Now()
	order.Status = OrderStatusPaid
	order.WeChatTransactionID = &transactionID
	order.PaidAt = &now
	order.CreditedAmount = &creditedAmount
	order.ExchangeRateUsed = &exchangeRate

	if err := s.orderRepo.Update(txCtx, order); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update order: %w", err)
	}

	// 增加用户余额（使用汇率转换后的额度）
	if err := s.userRepo.UpdateBalance(txCtx, order.UserID, creditedAmount); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update user balance: %w", err)
	}

	// 插入余额变动日志
	balanceAfter := balanceBefore + creditedAmount
	balanceLog := &BalanceLog{
		UserID:         order.UserID,
		ChangeType:     BalanceChangeTypeRecharge,
		Amount:         creditedAmount,
		BalanceBefore:  balanceBefore,
		BalanceAfter:   balanceAfter,
		RelatedOrderNo: &order.OrderNo,
		Description:    description,
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

// sendRechargeSuccessNotification 异步发送充值成功通知
// 通过邮件通知用户充值结果，不阻塞回调响应
func (s *PaymentCallbackService) sendRechargeSuccessNotification(result *ProcessPaymentResult) {
	if s.emailService == nil {
		log.Printf("[PaymentCallback] Email service not available, skip notification: order_no=%s",
			result.OrderNo)
		return
	}

	// 使用新的 context（原 context 可能已超时或取消）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 获取用户信息
	user, err := s.userRepo.GetByID(ctx, result.UserID)
	if err != nil {
		log.Printf("[PaymentCallback] Failed to get user for notification: order_no=%s, user_id=%d, error=%v",
			result.OrderNo, result.UserID, err)
		return
	}

	// 检查用户是否有邮箱
	if user.Email == "" {
		log.Printf("[PaymentCallback] User has no email, skip notification: order_no=%s, user_id=%d",
			result.OrderNo, result.UserID)
		return
	}

	// 构建邮件内容
	subject := "充值成功通知"
	body := s.buildRechargeSuccessEmailBody(result, user.Balance)

	// 发送邮件（失败只记录日志）
	if err := s.emailService.SendEmail(ctx, user.Email, subject, body); err != nil {
		log.Printf("[PaymentCallback] Failed to send notification email: order_no=%s, email=%s, error=%v",
			result.OrderNo, user.Email, err)
		return
	}

	log.Printf("[PaymentCallback] Notification email sent: order_no=%s, email=%s",
		result.OrderNo, user.Email)
}

// buildRechargeSuccessEmailBody 构建充值成功邮件内容
func (s *PaymentCallbackService) buildRechargeSuccessEmailBody(result *ProcessPaymentResult, currentBalance float64) string {
	paidAt := time.Now().Format("2006-01-02 15:04:05")

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4CAF50; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background-color: #f9f9f9; }
        .info-row { padding: 10px 0; border-bottom: 1px solid #eee; }
        .label { color: #666; }
        .value { font-weight: bold; }
        .amount { color: #4CAF50; font-size: 24px; }
        .footer { padding: 20px; text-align: center; color: #888; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>💰 充值成功</h2>
        </div>
        <div class="content">
            <div class="info-row">
                <span class="label">充值金额：</span>
                <span class="value amount">¥%.2f</span>
            </div>
            <div class="info-row">
                <span class="label">订单号：</span>
                <span class="value">%s</span>
            </div>
            <div class="info-row">
                <span class="label">到账时间：</span>
                <span class="value">%s</span>
            </div>
            <div class="info-row">
                <span class="label">当前余额：</span>
                <span class="value">¥%.2f</span>
            </div>
        </div>
        <div class="footer">
            <p>感谢您的支持！如有问题，请联系客服。</p>
        </div>
    </div>
</body>
</html>
`, result.Amount, result.OrderNo, paidAt, currentBalance)
}
