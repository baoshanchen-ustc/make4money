package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// --- Order Creation ---

func (s *PaymentService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*CreateOrderResponse, error) {
	if req.OrderType == "" {
		req.OrderType = payment.OrderTypeBalance
	}
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get payment config: %w", err)
	}
	if !cfg.Enabled {
		return nil, infraerrors.Forbidden("PAYMENT_DISABLED", "payment system is disabled")
	}
	plan, err := s.validateOrderInput(ctx, req, cfg)
	if err != nil {
		return nil, err
	}
	if err := s.checkCancelRateLimit(ctx, req.UserID, cfg); err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.Status != payment.EntityStatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}
	orderAmount := req.Amount
	limitAmount := req.Amount
	if plan != nil {
		orderAmount = plan.Price
		limitAmount = plan.Price
	} else if req.OrderType == payment.OrderTypeBalance {
		orderAmount = calculateCreditedBalance(req.Amount, cfg.BalanceRechargeMultiplier)
	}
	feeRate := cfg.RechargeFeeRate
	payAmountStr := payment.CalculatePayAmount(limitAmount, feeRate)
	payAmount, _ := strconv.ParseFloat(payAmountStr, 64)
	selection, err := s.selectProviderInstance(ctx, req, cfg, payAmount)
	if err != nil {
		return nil, err
	}
	order, err := s.createOrderInTx(ctx, req, plan, cfg, orderAmount, limitAmount, feeRate, payAmount, selection)
	if err != nil {
		return nil, err
	}
	resp, err := s.invokeProvider(ctx, order, req, cfg, limitAmount, payAmountStr, payAmount, plan, selection)
	if err != nil {
		_, _ = s.entClient.PaymentOrder.UpdateOneID(order.ID).
			SetStatus(OrderStatusFailed).
			Save(ctx)
		return nil, err
	}
	return resp, nil
}

func (s *PaymentService) validateOrderInput(ctx context.Context, req CreateOrderRequest, cfg *PaymentConfig) (*dbent.SubscriptionPlan, error) {
	if !isPaymentTypeEnabled(cfg, req.PaymentType) {
		return nil, infraerrors.BadRequest("PAYMENT_TYPE_DISABLED", "payment method is not enabled").
			WithMetadata(map[string]string{"payment_type": req.PaymentType})
	}
	if req.OrderType == payment.OrderTypeBalance && cfg.BalanceDisabled {
		return nil, infraerrors.Forbidden("BALANCE_PAYMENT_DISABLED", "balance recharge has been disabled")
	}
	if req.OrderType == payment.OrderTypeSubscription {
		return s.validateSubOrder(ctx, req)
	}
	if math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) || req.Amount <= 0 {
		return nil, infraerrors.BadRequest("INVALID_AMOUNT", "amount must be a positive number")
	}
	if (cfg.MinAmount > 0 && req.Amount < cfg.MinAmount) || (cfg.MaxAmount > 0 && req.Amount > cfg.MaxAmount) {
		return nil, infraerrors.BadRequest("INVALID_AMOUNT", "amount out of range").
			WithMetadata(map[string]string{"min": fmt.Sprintf("%.2f", cfg.MinAmount), "max": fmt.Sprintf("%.2f", cfg.MaxAmount)})
	}
	return nil, nil
}

func (s *PaymentService) selectProviderInstance(ctx context.Context, req CreateOrderRequest, cfg *PaymentConfig, payAmount float64) (*payment.InstanceSelection, error) {
	s.EnsureProviders(ctx)
	sel, err := s.loadBalancer.SelectInstance(ctx, "", req.PaymentType, payment.Strategy(cfg.LoadBalanceStrategy), payAmount)
	if err != nil {
		return nil, infraerrors.ServiceUnavailable("PAYMENT_GATEWAY_ERROR", fmt.Sprintf("payment method (%s) is not configured", req.PaymentType))
	}
	if sel == nil {
		return nil, infraerrors.TooManyRequests("NO_AVAILABLE_INSTANCE", "no available payment instance")
	}
	return sel, nil
}

func (s *PaymentService) validateSubOrder(ctx context.Context, req CreateOrderRequest) (*dbent.SubscriptionPlan, error) {
	if req.PlanID == 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "subscription order requires a plan")
	}
	plan, err := s.configService.GetPlan(ctx, req.PlanID)
	if err != nil || !plan.ForSale {
		return nil, infraerrors.NotFound("PLAN_NOT_AVAILABLE", "plan not found or not for sale")
	}
	group, err := s.groupRepo.GetByID(ctx, plan.GroupID)
	if err != nil || group.Status != payment.EntityStatusActive {
		return nil, infraerrors.NotFound("GROUP_NOT_FOUND", "subscription group is no longer available")
	}
	if !group.IsSubscriptionType() {
		return nil, infraerrors.BadRequest("GROUP_TYPE_MISMATCH", "group is not a subscription type")
	}
	return plan, nil
}

func (s *PaymentService) createOrderInTx(ctx context.Context, req CreateOrderRequest, plan *dbent.SubscriptionPlan, cfg *PaymentConfig, orderAmount, limitAmount, feeRate, payAmount float64, sel *payment.InstanceSelection) (*dbent.PaymentOrder, error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize order creation per user so pending-order and daily-limit checks
	// see a stable view even when the same user submits multiple requests at once.
	lockedUser, err := tx.User.Query().
		Where(user.IDEQ(req.UserID)).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock user for order creation: %w", err)
	}
	if lockedUser.Status != payment.EntityStatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}
	if err := s.checkPendingLimit(ctx, tx, req.UserID, cfg.MaxPendingOrders); err != nil {
		return nil, err
	}
	if err := s.checkDailyLimit(ctx, tx, req.UserID, limitAmount, cfg.DailyLimit); err != nil {
		return nil, err
	}
	if err := s.revalidateSelectedInstance(ctx, tx, req.PaymentType, payAmount, sel); err != nil {
		return nil, err
	}
	tm := cfg.OrderTimeoutMin
	if tm <= 0 {
		tm = defaultOrderTimeoutMin
	}
	exp := time.Now().Add(time.Duration(tm) * time.Minute)
	b := tx.PaymentOrder.Create().
		SetUserID(req.UserID).
		SetUserEmail(lockedUser.Email).
		SetUserName(lockedUser.Username).
		SetNillableUserNotes(psNilIfEmpty(lockedUser.Notes)).
		SetAmount(orderAmount).
		SetPayAmount(payAmount).
		SetFeeRate(feeRate).
		SetRechargeCode("").
		SetOutTradeNo(generateOutTradeNo()).
		SetPaymentType(req.PaymentType).
		SetPaymentTradeNo("").
		SetOrderType(req.OrderType).
		SetStatus(OrderStatusPending).
		SetExpiresAt(exp).
		SetClientIP(req.ClientIP).
		SetSrcHost(req.SrcHost).
		SetNillableProviderInstanceID(psNilIfEmpty(sel.InstanceID))
	if req.SrcURL != "" {
		b.SetSrcURL(req.SrcURL)
	}
	if plan != nil {
		b.SetPlanID(plan.ID).SetSubscriptionGroupID(plan.GroupID).SetSubscriptionDays(psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit))
	}
	order, err := b.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}
	code := fmt.Sprintf("PAY-%d-%d", order.ID, time.Now().UnixNano()%100000)
	order, err = tx.PaymentOrder.UpdateOneID(order.ID).SetRechargeCode(code).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("set recharge code: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit order transaction: %w", err)
	}
	return order, nil
}

func (s *PaymentService) revalidateSelectedInstance(ctx context.Context, tx *dbent.Tx, paymentType string, payAmount float64, sel *payment.InstanceSelection) error {
	if sel == nil || sel.InstanceID == "" {
		return nil
	}
	instID, err := strconv.ParseInt(sel.InstanceID, 10, 64)
	if err != nil {
		return fmt.Errorf("parse selected instance id %q: %w", sel.InstanceID, err)
	}
	inst, err := tx.PaymentProviderInstance.Query().
		Where(paymentproviderinstance.IDEQ(instID)).
		ForUpdate().
		Only(ctx)
	if err != nil {
		return fmt.Errorf("lock selected payment instance: %w", err)
	}
	if !inst.Enabled || (!payment.InstanceSupportsType(inst.SupportedTypes, paymentType) && inst.ProviderKey != payment.GetBasePaymentType(paymentType)) {
		return infraerrors.TooManyRequests("NO_AVAILABLE_INSTANCE", "selected payment instance is no longer available")
	}
	usage, err := payment.ScanNullableSum(func(dest any) error {
		return tx.PaymentOrder.Query().
			Where(
				paymentorder.ProviderInstanceIDEQ(sel.InstanceID),
				paymentorder.StatusIn(OrderStatusPending, OrderStatusPaid, OrderStatusCompleted, OrderStatusRecharging),
				paymentorder.CreatedAtGTE(startOfDay(time.Now())),
			).
			Aggregate(dbent.Sum(paymentorder.FieldPayAmount)).
			Scan(ctx, dest)
	})
	if err != nil {
		return fmt.Errorf("query selected instance usage: %w", err)
	}
	cl := orderInstanceChannelLimits(inst, paymentType)
	if cl.SingleMin > 0 && payAmount < cl.SingleMin {
		return infraerrors.TooManyRequests("NO_AVAILABLE_INSTANCE", "selected payment instance no longer supports this amount")
	}
	if cl.SingleMax > 0 && payAmount > cl.SingleMax {
		return infraerrors.TooManyRequests("NO_AVAILABLE_INSTANCE", "selected payment instance no longer supports this amount")
	}
	if cl.DailyLimit > 0 && usage+payAmount > cl.DailyLimit {
		return infraerrors.TooManyRequests("NO_AVAILABLE_INSTANCE", "selected payment instance has exhausted its daily capacity")
	}
	return nil
}

func orderInstanceChannelLimits(inst *dbent.PaymentProviderInstance, paymentType string) payment.ChannelLimits {
	if inst == nil || inst.Limits == "" {
		return payment.ChannelLimits{}
	}
	var limits payment.InstanceLimits
	if err := json.Unmarshal([]byte(inst.Limits), &limits); err != nil {
		return payment.ChannelLimits{}
	}
	lookupKey := paymentType
	if inst.ProviderKey == payment.TypeStripe {
		lookupKey = payment.TypeStripe
	}
	if cl, ok := limits[lookupKey]; ok {
		return cl
	}
	return payment.ChannelLimits{}
}

func (s *PaymentService) checkPendingLimit(ctx context.Context, tx *dbent.Tx, userID int64, max int) error {
	if max <= 0 {
		max = defaultMaxPendingOrders
	}
	c, err := tx.PaymentOrder.Query().Where(paymentorder.UserIDEQ(userID), paymentorder.StatusEQ(OrderStatusPending)).Count(ctx)
	if err != nil {
		return fmt.Errorf("count pending orders: %w", err)
	}
	if c >= max {
		return infraerrors.TooManyRequests("TOO_MANY_PENDING", fmt.Sprintf("too many pending orders (max %d)", max)).
			WithMetadata(map[string]string{"max": strconv.Itoa(max)})
	}
	return nil
}

func (s *PaymentService) checkDailyLimit(ctx context.Context, tx *dbent.Tx, userID int64, amount, limit float64) error {
	if limit <= 0 {
		return nil
	}
	ts := psStartOfDayUTC(time.Now())
	orders, err := tx.PaymentOrder.Query().
		Where(
			paymentorder.UserIDEQ(userID),
			paymentorder.Or(
				paymentorder.And(
					paymentorder.StatusEQ(OrderStatusPending),
					paymentorder.CreatedAtGTE(ts),
				),
				paymentorder.And(
					paymentorder.StatusIn(OrderStatusPaid, OrderStatusRecharging, OrderStatusCompleted),
					paymentorder.PaidAtGTE(ts),
				),
			),
		).
		All(ctx)
	if err != nil {
		return fmt.Errorf("query daily usage: %w", err)
	}
	var used float64
	for _, o := range orders {
		if o.OrderType == payment.OrderTypeBalance {
			used += o.PayAmount
			continue
		}
		used += o.Amount
	}
	if used+amount > limit {
		return infraerrors.TooManyRequests("DAILY_LIMIT_EXCEEDED", fmt.Sprintf("daily recharge limit reached, remaining: %.2f", math.Max(0, limit-used)))
	}
	return nil
}

func (s *PaymentService) invokeProvider(ctx context.Context, order *dbent.PaymentOrder, req CreateOrderRequest, cfg *PaymentConfig, limitAmount float64, payAmountStr string, payAmount float64, plan *dbent.SubscriptionPlan, sel *payment.InstanceSelection) (*CreateOrderResponse, error) {
	providerKey := sel.ProviderKey
	prov, err := provider.CreateProvider(providerKey, sel.InstanceID, sel.Config)
	if err != nil {
		return nil, infraerrors.ServiceUnavailable("PAYMENT_GATEWAY_ERROR", "payment method is temporarily unavailable")
	}
	subject := s.buildPaymentSubject(plan, limitAmount, cfg)
	outTradeNo := order.OutTradeNo
	pr, err := prov.CreatePayment(ctx, payment.CreatePaymentRequest{OrderID: outTradeNo, Amount: payAmountStr, PaymentType: req.PaymentType, Subject: subject, ClientIP: req.ClientIP, IsMobile: req.IsMobile, InstanceSubMethods: sel.SupportedTypes})
	if err != nil {
		slog.Error("[PaymentService] CreatePayment failed", "provider", providerKey, "instance", sel.InstanceID, "error", err)
		return nil, infraerrors.ServiceUnavailable("PAYMENT_GATEWAY_ERROR", fmt.Sprintf("payment gateway error: %s", err.Error()))
	}
	_, err = s.entClient.PaymentOrder.UpdateOneID(order.ID).SetNillablePaymentTradeNo(psNilIfEmpty(pr.TradeNo)).SetNillablePayURL(psNilIfEmpty(pr.PayURL)).SetNillableQrCode(psNilIfEmpty(pr.QRCode)).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update order with payment details: %w", err)
	}
	s.writeAuditLog(ctx, order.ID, "ORDER_CREATED", fmt.Sprintf("user:%d", req.UserID), map[string]any{
		"paymentAmount":  req.Amount,
		"creditedAmount": order.Amount,
		"payAmount":      order.PayAmount,
		"paymentType":    req.PaymentType,
		"orderType":      req.OrderType,
	})
	resp := &CreateOrderResponse{
		OrderID:      order.ID,
		Amount:       order.Amount,
		PayAmount:    payAmount,
		FeeRate:      order.FeeRate,
		Status:       OrderStatusPending,
		PaymentType:  req.PaymentType,
		PayURL:       pr.PayURL,
		QRCode:       pr.QRCode,
		ClientSecret: pr.ClientSecret,
		ExpiresAt:    order.ExpiresAt,
		PaymentMode:  sel.PaymentMode,
	}
	if providerKey == payment.TypeStripe {
		resp.StripePublishableKey = sel.Config[payment.ConfigKeyPublishableKey]
	}
	return resp, nil
}

func (s *PaymentService) buildPaymentSubject(plan *dbent.SubscriptionPlan, limitAmount float64, cfg *PaymentConfig) string {
	if plan != nil {
		if plan.ProductName != "" {
			return plan.ProductName
		}
		return "Sub2API Subscription " + plan.Name
	}
	amountStr := strconv.FormatFloat(limitAmount, 'f', 2, 64)
	pf := strings.TrimSpace(cfg.ProductNamePrefix)
	sf := strings.TrimSpace(cfg.ProductNameSuffix)
	if pf != "" || sf != "" {
		return strings.TrimSpace(pf + " " + amountStr + " " + sf)
	}
	return "Sub2API " + amountStr + " CNY"
}

func isPaymentTypeEnabled(cfg *PaymentConfig, paymentType string) bool {
	want := normalizeEnabledPaymentType(paymentType)
	if want == "" {
		return false
	}
	if cfg == nil || len(cfg.EnabledTypes) == 0 {
		return true
	}
	for _, enabled := range normalizeEnabledPaymentTypes(cfg.EnabledTypes) {
		if enabled == payment.TypeEasyPay && (want == payment.TypeAlipay || want == payment.TypeWxpay || want == payment.TypeEasyPay) {
			return true
		}
		if enabled == want {
			return true
		}
	}
	return false
}

// --- Order Queries ---

func (s *PaymentService) GetOrder(ctx context.Context, orderID, userID int64) (*dbent.PaymentOrder, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != userID {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission for this order")
	}
	return o, nil
}

func (s *PaymentService) GetOrderByID(ctx context.Context, orderID int64) (*dbent.PaymentOrder, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	return o, nil
}

func (s *PaymentService) GetStripePublishableKeyForOrder(ctx context.Context, o *dbent.PaymentOrder) string {
	if o == nil || s.expectedProviderKeyForOrder(o) != payment.TypeStripe {
		return ""
	}
	prov, err := s.getOrderProvider(ctx, o)
	if err != nil {
		return ""
	}
	keyProvider, ok := prov.(stripePublishableKeyProvider)
	if !ok {
		return ""
	}
	return keyProvider.GetPublishableKey()
}

func (s *PaymentService) GetUserOrders(ctx context.Context, userID int64, p OrderListParams) ([]*dbent.PaymentOrder, int, error) {
	q := s.entClient.PaymentOrder.Query().Where(paymentorder.UserIDEQ(userID))
	if p.Status != "" {
		q = q.Where(paymentorder.StatusEQ(p.Status))
	}
	if p.OrderType != "" {
		q = q.Where(paymentorder.OrderTypeEQ(p.OrderType))
	}
	if p.PaymentType != "" {
		q = q.Where(paymentorder.PaymentTypeEQ(p.PaymentType))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count user orders: %w", err)
	}
	ps, pg := applyPagination(p.PageSize, p.Page)
	orders, err := q.Order(dbent.Desc(paymentorder.FieldCreatedAt)).Limit(ps).Offset((pg - 1) * ps).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("query user orders: %w", err)
	}
	return orders, total, nil
}

// AdminListOrders returns a paginated list of orders. If userID > 0, filters by user.
func (s *PaymentService) AdminListOrders(ctx context.Context, userID int64, p OrderListParams) ([]*dbent.PaymentOrder, int, error) {
	q := s.entClient.PaymentOrder.Query()
	if userID > 0 {
		q = q.Where(paymentorder.UserIDEQ(userID))
	}
	if p.Status != "" {
		q = q.Where(paymentorder.StatusEQ(p.Status))
	}
	if p.OrderType != "" {
		q = q.Where(paymentorder.OrderTypeEQ(p.OrderType))
	}
	if p.PaymentType != "" {
		q = q.Where(paymentorder.PaymentTypeEQ(p.PaymentType))
	}
	if p.Keyword != "" {
		q = q.Where(paymentorder.Or(
			paymentorder.OutTradeNoContainsFold(p.Keyword),
			paymentorder.UserEmailContainsFold(p.Keyword),
			paymentorder.UserNameContainsFold(p.Keyword),
		))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count admin orders: %w", err)
	}
	ps, pg := applyPagination(p.PageSize, p.Page)
	orders, err := q.Order(dbent.Desc(paymentorder.FieldCreatedAt)).Limit(ps).Offset((pg - 1) * ps).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("query admin orders: %w", err)
	}
	return orders, total, nil
}
