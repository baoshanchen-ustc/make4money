package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// OrderCompensationScheduler 订单补偿定时任务
// 定时扫描可能遗漏的 pending 订单，查询微信支付状态，自动补偿到账
type OrderCompensationScheduler struct {
	cfg                    *config.Config
	orderRepo              RechargeOrderRepository
	wechatPayService       *WeChatPayService
	paymentCallbackService *PaymentCallbackService
	stopCh                 chan struct{}
	wg                     sync.WaitGroup
}

// NewOrderCompensationScheduler 创建订单补偿调度器
func NewOrderCompensationScheduler(
	cfg *config.Config,
	orderRepo RechargeOrderRepository,
	wechatPayService *WeChatPayService,
	paymentCallbackService *PaymentCallbackService,
) *OrderCompensationScheduler {
	return &OrderCompensationScheduler{
		cfg:                    cfg,
		orderRepo:              orderRepo,
		wechatPayService:       wechatPayService,
		paymentCallbackService: paymentCallbackService,
		stopCh:                 make(chan struct{}),
	}
}

// Start 启动调度器
func (s *OrderCompensationScheduler) Start() {
	// 检查是否启用
	if !s.cfg.WeChatPay.Enabled || !s.cfg.WeChatPay.CompensationEnabled {
		log.Printf("[OrderCompensationScheduler] Disabled (wechat_pay.enabled=%v, compensation_enabled=%v)",
			s.cfg.WeChatPay.Enabled, s.cfg.WeChatPay.CompensationEnabled)
		return
	}

	s.wg.Add(1)
	go s.run()
	log.Printf("[OrderCompensationScheduler] Started (interval: %dm, threshold: %dm, batch: %d, concurrency: %d)",
		s.cfg.WeChatPay.CompensationIntervalMins,
		s.cfg.WeChatPay.CompensationThresholdMins,
		s.cfg.WeChatPay.CompensationBatchSize,
		s.cfg.WeChatPay.CompensationConcurrency)
}

// Stop 停止调度器
func (s *OrderCompensationScheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	log.Printf("[OrderCompensationScheduler] Stopped")
}

func (s *OrderCompensationScheduler) run() {
	defer s.wg.Done()

	interval := time.Duration(s.cfg.WeChatPay.CompensationIntervalMins) * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 启动时立即执行一次
	s.processCompensation()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processCompensation()
		}
	}
}

// processCompensation 执行一次补偿扫描
func (s *OrderCompensationScheduler) processCompensation() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	startTime := time.Now()
	log.Printf("[OrderCompensationScheduler] Job started")

	// 计算阈值时间：只处理创建时间超过 threshold 分钟的订单
	thresholdTime := time.Now().Add(-time.Duration(s.cfg.WeChatPay.CompensationThresholdMins) * time.Minute)

	// 获取待补偿订单
	orders, err := s.orderRepo.GetPendingOrdersForCompensation(ctx, thresholdTime, s.cfg.WeChatPay.CompensationBatchSize)
	if err != nil {
		log.Printf("[OrderCompensationScheduler] Error getting pending orders: %v", err)
		return
	}

	if len(orders) == 0 {
		log.Printf("[OrderCompensationScheduler] No pending orders to compensate")
		return
	}

	log.Printf("[OrderCompensationScheduler] Found %d pending orders for compensation", len(orders))

	// 使用信号量控制并发
	concurrency := s.cfg.WeChatPay.CompensationConcurrency
	if concurrency <= 0 {
		concurrency = 5
	}
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	var successCount, failCount, skippedCount int
	var mu sync.Mutex

	for _, order := range orders {
		wg.Add(1)
		sem <- struct{}{} // 获取信号量

		go func(orderNo string) {
			defer wg.Done()
			defer func() { <-sem }() // 释放信号量

			result := s.processOrder(ctx, orderNo)
			mu.Lock()
			switch result {
			case "success":
				successCount++
			case "skipped":
				skippedCount++
			default:
				failCount++
			}
			mu.Unlock()
		}(order.OrderNo)
	}

	wg.Wait()

	duration := time.Since(startTime)
	log.Printf("[OrderCompensationScheduler] Job completed: total=%d, success=%d, skipped=%d, failed=%d, duration=%v",
		len(orders), successCount, skippedCount, failCount, duration)
}

// processOrder 处理单个订单
// 返回值：success=补偿成功, skipped=无需补偿, failed=处理失败
func (s *OrderCompensationScheduler) processOrder(ctx context.Context, orderNo string) string {
	// 检查微信支付服务是否可用
	if s.wechatPayService == nil || !s.wechatPayService.IsEnabled() {
		return "failed"
	}

	// 查询微信支付状态
	wechatResult, err := s.wechatPayService.QueryOrder(ctx, orderNo)
	if err != nil {
		log.Printf("[OrderCompensationScheduler] Failed to query WeChat order: order_no=%s, error=%v", orderNo, err)
		return "failed"
	}

	log.Printf("[OrderCompensationScheduler] WeChat order status: order_no=%s, status=%s", orderNo, wechatResult.TradeState)

	switch wechatResult.TradeState {
	case "SUCCESS":
		// 微信已支付，触发补偿到账
		result := s.paymentCallbackService.ProcessPaymentSuccess(ctx, ProcessPaymentSuccessParams{
			OrderNo:       orderNo,
			TransactionID: wechatResult.TransactionID,
			AmountInFen:   0, // 补偿时不验证金额
			Source:        PaymentSourceCompensate,
		})

		if result.Success {
			if result.AlreadyPaid {
				log.Printf("[OrderCompensationScheduler] Order already paid: order_no=%s", orderNo)
				return "skipped"
			}
			log.Printf("[OrderCompensationScheduler] Order compensated successfully: order_no=%s", orderNo)
			return "success"
		}
		log.Printf("[OrderCompensationScheduler] Failed to compensate order: order_no=%s, error=%s", orderNo, result.ErrorMessage)
		return "failed"

	case "CLOSED":
		// 微信订单已关闭，更新本地状态为 expired
		if err := s.orderRepo.MarkOrderExpired(ctx, orderNo); err != nil {
			log.Printf("[OrderCompensationScheduler] Failed to mark order expired: order_no=%s, error=%v", orderNo, err)
		} else {
			log.Printf("[OrderCompensationScheduler] Marked order as expired (WeChat CLOSED): order_no=%s", orderNo)
		}
		return "skipped"

	case "NOTPAY", "USERPAYING":
		// 未支付或支付中，跳过
		return "skipped"

	case "PAYERROR":
		// 支付失败，标记为 failed
		if err := s.orderRepo.MarkOrderFailed(ctx, orderNo); err != nil {
			log.Printf("[OrderCompensationScheduler] Failed to mark order failed: order_no=%s, error=%v", orderNo, err)
		} else {
			log.Printf("[OrderCompensationScheduler] Marked order as failed (WeChat PAYERROR): order_no=%s", orderNo)
		}
		return "skipped"

	default:
		// 未知状态，跳过
		log.Printf("[OrderCompensationScheduler] Unknown WeChat status: order_no=%s, status=%s", orderNo, wechatResult.TradeState)
		return "skipped"
	}
}
