package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// --- Refund Flow ---

// getOrderProviderInstance looks up the provider instance that processed this order.
// Returns nil, nil for legacy orders without provider_instance_id.
func (s *PaymentService) getOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	if o.ProviderInstanceID == nil || *o.ProviderInstanceID == "" {
		return nil, nil
	}
	instID, err := strconv.ParseInt(*o.ProviderInstanceID, 10, 64)
	if err != nil {
		return nil, nil
	}
	return s.entClient.PaymentProviderInstance.Get(ctx, instID)
}

func (s *PaymentService) RequestRefund(ctx context.Context, oid, uid int64, reason string) error {
	o, err := s.validateRefundRequest(ctx, oid, uid)
	if err != nil {
		return err
	}
	u, err := s.userRepo.GetByID(ctx, o.UserID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if u.Balance < o.Amount {
		return infraerrors.BadRequest("BALANCE_NOT_ENOUGH", "refund amount exceeds balance")
	}
	nr := strings.TrimSpace(reason)
	now := time.Now()
	by := fmt.Sprintf("%d", uid)
	c, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(oid), paymentorder.UserIDEQ(uid), paymentorder.StatusEQ(OrderStatusCompleted), paymentorder.OrderTypeEQ(payment.OrderTypeBalance)).SetStatus(OrderStatusRefundRequested).SetRefundRequestedAt(now).SetRefundRequestReason(nr).SetRefundRequestedBy(by).SetRefundAmount(o.Amount).Save(ctx)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if c == 0 {
		return infraerrors.Conflict("CONFLICT", "order status changed")
	}
	s.writeAuditLog(ctx, oid, "REFUND_REQUESTED", fmt.Sprintf("user:%d", uid), map[string]any{"amount": o.Amount, "reason": nr})
	return nil
}

func (s *PaymentService) validateRefundRequest(ctx context.Context, oid, uid int64) (*dbent.PaymentOrder, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != uid {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission")
	}
	if o.OrderType != payment.OrderTypeBalance {
		return nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "only balance orders can request refund")
	}
	if o.Status != OrderStatusCompleted {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "only completed orders can request refund")
	}
	// Check provider instance allows user refund
	inst, err := s.getOrderProviderInstance(ctx, o)
	if err != nil || inst == nil {
		return nil, infraerrors.Forbidden("USER_REFUND_DISABLED", "refund is not available for this order")
	}
	if !inst.AllowUserRefund {
		return nil, infraerrors.Forbidden("USER_REFUND_DISABLED", "user refund is not enabled for this provider")
	}
	return o, nil
}

func (s *PaymentService) PrepareRefund(ctx context.Context, oid int64, amt float64, reason string, force, deduct bool) (*RefundPlan, *RefundResult, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	ok := []string{OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefundFailed, OrderStatusRefunding}
	if !psSliceContains(ok, o.Status) {
		return nil, nil, infraerrors.BadRequest("INVALID_STATUS", "order status does not allow refund")
	}
	// Check provider instance allows admin refund
	inst, instErr := s.getOrderProviderInstance(ctx, o)
	if instErr != nil {
		slog.Warn("refund: provider instance lookup failed", "orderID", oid, "error", instErr)
		return nil, nil, infraerrors.InternalServer("PROVIDER_LOOKUP_FAILED", "failed to look up payment provider for this order")
	}
	if inst == nil {
		// Legacy order without provider_instance_id — block refund
		return nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not available for this order")
	}
	if !inst.RefundEnabled {
		return nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not enabled for this provider")
	}
	if math.IsNaN(amt) || math.IsInf(amt, 0) {
		return nil, nil, infraerrors.BadRequest("INVALID_AMOUNT", "invalid refund amount")
	}
	if amt <= 0 {
		amt = o.Amount
	}
	if amt-o.Amount > amountToleranceCNY {
		return nil, nil, infraerrors.BadRequest("REFUND_AMOUNT_EXCEEDED", "refund amount exceeds recharge")
	}
	ga := calculateGatewayRefundAmount(o.Amount, o.PayAmount, amt)
	rr := strings.TrimSpace(reason)
	if rr == "" && o.RefundRequestReason != nil {
		rr = *o.RefundRequestReason
	}
	if rr == "" {
		rr = fmt.Sprintf("refund order:%d", o.ID)
	}
	p := &RefundPlan{OrderID: oid, Order: o, RefundAmount: amt, GatewayAmount: ga, Reason: rr, Force: force, DeductBalance: deduct, DeductionType: payment.DeductionTypeNone}
	if deduct {
		if er := s.prepDeduct(ctx, o, p, force); er != nil {
			return nil, er, nil
		}
	}
	return p, nil, nil
}

func (s *PaymentService) prepDeduct(ctx context.Context, o *dbent.PaymentOrder, p *RefundPlan, force bool) *RefundResult {
	if o.OrderType == payment.OrderTypeSubscription {
		p.DeductionType = payment.DeductionTypeSubscription
		if o.SubscriptionGroupID != nil && o.SubscriptionDays != nil {
			p.SubDaysToDeduct = *o.SubscriptionDays
			sub, err := s.subscriptionSvc.GetActiveSubscription(ctx, o.UserID, *o.SubscriptionGroupID)
			if err == nil && sub != nil {
				p.SubscriptionID = sub.ID
			} else if !force {
				return &RefundResult{Success: false, Warning: "cannot find active subscription for deduction, use force", RequireForce: true}
			}
		}
		return nil
	}
	u, err := s.userRepo.GetByID(ctx, o.UserID)
	if err != nil {
		if !force {
			return &RefundResult{Success: false, Warning: "cannot fetch user balance, use force", RequireForce: true}
		}
		return nil
	}
	p.DeductionType = payment.DeductionTypeBalance
	if u.Balance > 0 {
		p.BalanceToDeduct = math.Min(p.RefundAmount, u.Balance)
	}
	return nil
}

func (s *PaymentService) ExecuteRefund(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	c, err := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(p.OrderID), paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundRequested, OrderStatusRefundFailed, OrderStatusRefunding)).SetStatus(OrderStatusRefunding).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock: %w", err)
	}
	if c == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	if err := s.deductBalanceForRefund(ctx, p); err != nil {
		s.restoreStatus(ctx, p)
		return nil, err
	}
	if err := s.deductSubscriptionForRefund(ctx, p); err != nil {
		s.restoreStatus(ctx, p)
		return nil, err
	}
	resp, err := s.gwRefund(ctx, p)
	if err != nil {
		return s.handleGwFail(ctx, p, err)
	}
	return s.handleRefundResponse(ctx, p, resp)
}

func (s *PaymentService) gwRefund(ctx context.Context, p *RefundPlan) (*payment.RefundResponse, error) {
	// Use the exact provider instance that created this order, not a random one
	// from the registry. Each instance has its own merchant credentials.
	prov, err := s.getRefundProvider(ctx, p.Order)
	if err != nil {
		return nil, fmt.Errorf("get refund provider: %w", err)
	}
	tradeNo, err := s.resolveRefundTradeNo(ctx, prov, p.Order)
	if err != nil {
		return nil, err
	}
	queryRef := tradeNo
	if queryRef == "" {
		queryRef = p.Order.OutTradeNo
	}
	resp, err := prov.Refund(ctx, payment.RefundRequest{
		TradeNo: tradeNo,
		OrderID: p.Order.OutTradeNo,
		Amount:  strconv.FormatFloat(p.GatewayAmount, 'f', 2, 64),
		Reason:  p.Reason,
	})
	if err != nil {
		if resolved, reconciled, recErr := s.reconcileRefund(ctx, prov, queryRef); recErr == nil && resolved {
			return reconciled, nil
		}
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("refund provider returned nil response")
	}
	return resp, nil
}

// getRefundProvider uses the order-bound provider instance when preparing refunds.
func (s *PaymentService) getRefundProvider(ctx context.Context, o *dbent.PaymentOrder) (payment.Provider, error) {
	return s.getOrderProvider(ctx, o)
}

func (s *PaymentService) resolveRefundTradeNo(ctx context.Context, prov payment.Provider, order *dbent.PaymentOrder) (string, error) {
	if order == nil {
		return "", fmt.Errorf("nil order")
	}
	if order.PaymentTradeNo != "" {
		return order.PaymentTradeNo, nil
	}
	switch prov.ProviderKey() {
	case payment.TypeEasyPay, payment.TypeAlipay, payment.TypeWxpay:
		return "", nil
	case payment.TypeStripe:
		return "", fmt.Errorf("missing stripe payment trade number for order %d", order.ID)
	}
	resp, err := prov.QueryOrder(ctx, order.OutTradeNo)
	if err != nil {
		return "", fmt.Errorf("query upstream trade number: %w", err)
	}
	tradeNo := strings.TrimSpace(resp.TradeNo)
	if tradeNo == "" {
		return "", fmt.Errorf("missing upstream trade number for order %d", order.ID)
	}
	if prov.ProviderKey() == payment.TypeEasyPay && tradeNo == order.OutTradeNo {
		return "", nil
	}
	if _, saveErr := s.entClient.PaymentOrder.UpdateOneID(order.ID).SetPaymentTradeNo(tradeNo).Save(ctx); saveErr == nil {
		order.PaymentTradeNo = tradeNo
	}
	return tradeNo, nil
}

func (s *PaymentService) handleGwFail(ctx context.Context, p *RefundPlan, gErr error) (*RefundResult, error) {
	if s.RollbackRefund(ctx, p, gErr) {
		s.restoreStatus(ctx, p)
		s.writeAuditLog(ctx, p.OrderID, "REFUND_GATEWAY_FAILED", "admin", map[string]any{"detail": psErrMsg(gErr)})
		return &RefundResult{Success: false, Warning: "gateway failed: " + psErrMsg(gErr) + ", rolled back"}, nil
	}
	now := time.Now()
	_, _ = s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(OrderStatusRefundFailed).SetFailedAt(now).SetFailedReason(psErrMsg(gErr)).Save(ctx)
	s.writeAuditLog(ctx, p.OrderID, "REFUND_FAILED", "admin", map[string]any{"detail": psErrMsg(gErr)})
	return nil, infraerrors.InternalServer("REFUND_FAILED", psErrMsg(gErr))
}

func (s *PaymentService) markRefundOk(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	fs := OrderStatusRefunded
	if p.RefundAmount < p.Order.Amount {
		fs = OrderStatusPartiallyRefunded
	}
	now := time.Now()
	_, err := s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(fs).SetRefundAmount(p.RefundAmount).SetRefundReason(p.Reason).SetRefundAt(now).SetForceRefund(p.Force).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark refund: %w", err)
	}
	s.writeAuditLog(ctx, p.OrderID, "REFUND_SUCCESS", "admin", map[string]any{"refundAmount": p.RefundAmount, "reason": p.Reason, "balanceDeducted": p.BalanceToDeduct, "force": p.Force})
	return &RefundResult{Success: true, BalanceDeducted: p.BalanceToDeduct, SubDaysDeducted: p.SubDaysToDeduct}, nil
}

func (s *PaymentService) RollbackRefund(ctx context.Context, p *RefundPlan, gErr error) bool {
	if p.DeductionType == payment.DeductionTypeBalance && p.BalanceToDeduct > 0 {
		if err := s.userRepo.UpdateBalance(ctx, p.Order.UserID, p.BalanceToDeduct); err != nil {
			slog.Error("[CRITICAL] rollback failed", "orderID", p.OrderID, "amount", p.BalanceToDeduct, "error", err)
			s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "balanceDeducted": p.BalanceToDeduct})
			return false
		}
	}
	if p.DeductionType == payment.DeductionTypeSubscription && p.SubDaysToDeduct > 0 && p.SubscriptionID > 0 {
		if p.SubRevoked {
			if p.SubSnapshot == nil {
				err := fmt.Errorf("missing revoked subscription snapshot")
				slog.Error("[CRITICAL] subscription rollback failed", "orderID", p.OrderID, "subID", p.SubscriptionID, "error", err)
				s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "subDaysDeducted": p.SubDaysToDeduct})
				return false
			}
			if _, err := s.subscriptionSvc.RestoreSubscription(ctx, p.SubSnapshot); err != nil {
				slog.Error("[CRITICAL] subscription rollback failed", "orderID", p.OrderID, "subID", p.SubscriptionID, "days", p.SubDaysToDeduct, "error", err)
				s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "subDaysDeducted": p.SubDaysToDeduct})
				return false
			}
		} else {
			if _, err := s.subscriptionSvc.ExtendSubscription(ctx, p.SubscriptionID, p.SubDaysToDeduct); err != nil {
				slog.Error("[CRITICAL] subscription rollback failed", "orderID", p.OrderID, "subID", p.SubscriptionID, "days", p.SubDaysToDeduct, "error", err)
				s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "subDaysDeducted": p.SubDaysToDeduct})
				return false
			}
		}
	}
	return true
}

func cloneUserSubscription(sub *UserSubscription) *UserSubscription {
	if sub == nil {
		return nil
	}
	copy := *sub
	return &copy
}

func (s *PaymentService) deductBalanceForRefund(ctx context.Context, p *RefundPlan) error {
	if p == nil || p.Order == nil {
		return fmt.Errorf("nil refund plan")
	}
	if !p.DeductBalance || p.DeductionType != payment.DeductionTypeBalance || p.BalanceToDeduct <= 0 {
		return nil
	}
	if s.hasAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED") {
		slog.Warn("skipping balance deduction on retry (previous rollback failed)", "orderID", p.OrderID)
		p.BalanceToDeduct = 0
		return nil
	}
	if err := s.userRepo.DeductBalance(ctx, p.Order.UserID, p.BalanceToDeduct); err != nil {
		return fmt.Errorf("deduct refund balance: %w", err)
	}
	return nil
}

func (s *PaymentService) hasAuditLog(ctx context.Context, oid int64, action string) bool {
	count, err := s.entClient.PaymentAuditLog.Query().
		Where(
			paymentauditlog.OrderIDEQ(strconv.FormatInt(oid, 10)),
			paymentauditlog.ActionEQ(strings.TrimSpace(action)),
		).
		Count(ctx)
	if err != nil {
		return false
	}
	return count > 0
}

func (s *PaymentService) deductSubscriptionForRefund(ctx context.Context, p *RefundPlan) error {
	if p == nil || p.Order == nil {
		return fmt.Errorf("nil refund plan")
	}
	if !p.DeductBalance || p.DeductionType != payment.DeductionTypeSubscription || p.SubDaysToDeduct <= 0 || p.SubscriptionID <= 0 {
		return nil
	}
	sub, err := s.subscriptionSvc.GetByID(ctx, p.SubscriptionID)
	if err != nil {
		return fmt.Errorf("load subscription for refund deduction: %w", err)
	}
	p.SubSnapshot = cloneUserSubscription(sub)

	now := time.Now()
	if !sub.ExpiresAt.AddDate(0, 0, -p.SubDaysToDeduct).After(now) {
		if err := s.subscriptionSvc.RevokeSubscription(ctx, p.SubscriptionID); err != nil {
			return fmt.Errorf("revoke subscription for refund: %w", err)
		}
		p.SubRevoked = true
		return nil
	}
	if _, err := s.subscriptionSvc.ExtendSubscription(ctx, p.SubscriptionID, -p.SubDaysToDeduct); err != nil {
		return fmt.Errorf("shorten subscription for refund: %w", err)
	}
	return nil
}

func (s *PaymentService) handleRefundResponse(ctx context.Context, p *RefundPlan, resp *payment.RefundResponse) (*RefundResult, error) {
	if resp == nil {
		return nil, fmt.Errorf("refund response is nil")
	}
	switch resp.Status {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded:
		return s.markRefundOk(ctx, p)
	case payment.ProviderStatusPending:
		s.writeAuditLog(ctx, p.OrderID, "REFUND_PENDING", "admin", map[string]any{
			"refundAmount": p.RefundAmount,
			"reason":       p.Reason,
			"refundID":     resp.RefundID,
		})
		return &RefundResult{
			Success:         false,
			Warning:         "refund is pending upstream reconciliation",
			BalanceDeducted: p.BalanceToDeduct,
			SubDaysDeducted: p.SubDaysToDeduct,
		}, nil
	default:
		return s.handleGwFail(ctx, p, fmt.Errorf("refund status %s", resp.Status))
	}
}

func (s *PaymentService) reconcileRefund(ctx context.Context, prov payment.Provider, queryRef string) (bool, *payment.RefundResponse, error) {
	if prov == nil || strings.TrimSpace(queryRef) == "" {
		return false, nil, nil
	}
	resp, err := prov.QueryOrder(ctx, queryRef)
	if err != nil {
		return false, nil, err
	}
	if resp == nil {
		return false, nil, nil
	}
	switch resp.Status {
	case payment.ProviderStatusRefunded:
		return true, &payment.RefundResponse{RefundID: queryRef, Status: payment.ProviderStatusRefunded}, nil
	case payment.ProviderStatusSuccess:
		return true, &payment.RefundResponse{RefundID: queryRef, Status: payment.ProviderStatusSuccess}, nil
	default:
		return false, nil, nil
	}
}

func (s *PaymentService) restoreStatus(ctx context.Context, p *RefundPlan) {
	rs := OrderStatusCompleted
	if p.Order.Status == OrderStatusRefundRequested {
		rs = OrderStatusRefundRequested
	}
	_, _ = s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetStatus(rs).Save(ctx)
}
