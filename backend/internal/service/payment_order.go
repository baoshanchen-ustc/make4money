package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/url"
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
	selection, order, oauthResp, err := s.createOrderWithSelectionRetry(
		ctx,
		req,
		cfg,
		plan,
		orderAmount,
		limitAmount,
		feeRate,
		payAmount,
		func(sel *payment.InstanceSelection) (*CreateOrderResponse, error) {
			return s.maybeBuildWeChatOAuthRequiredResponseForSelection(ctx, req, orderAmount, payAmount, feeRate, sel)
		},
	)
	if err != nil {
		return nil, err
	}
	if oauthResp != nil {
		return oauthResp, nil
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
	if !psIsEnabledPaymentType(req.PaymentType, cfg.EnabledTypes) {
		return nil, infraerrors.Forbidden("PAYMENT_TYPE_DISABLED", "payment method is disabled")
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

func psIsEnabledPaymentType(requested string, enabledTypes []string) bool {
	normalizedRequested := string(payment.NormalizeVisiblePaymentType(requested))
	if !payment.IsVisiblePaymentType(normalizedRequested) {
		return false
	}
	if len(enabledTypes) == 0 {
		return true
	}
	for _, enabledType := range enabledTypes {
		if string(payment.NormalizeVisiblePaymentType(enabledType)) == normalizedRequested {
			return true
		}
	}
	return false
}

func preferredProviderKeyForCreateOrder(req CreateOrderRequest) string {
	if !req.IsMobile {
		return ""
	}
	if payment.NormalizeVisiblePaymentType(req.PaymentType) == payment.TypeAlipay {
		return payment.TypeAlipay
	}
	return ""
}

func (s *PaymentService) selectProviderInstance(ctx context.Context, req CreateOrderRequest, cfg *PaymentConfig, payAmount float64, excludedInstanceIDs []string) (*payment.InstanceSelection, error) {
	s.EnsureProviders(ctx)
	preferredProviderKey := preferredProviderKeyForCreateOrder(req)
	if preferredProviderKey != "" {
		sel, err := s.loadBalancer.SelectInstanceExcept(ctx, preferredProviderKey, req.PaymentType, payment.Strategy(cfg.LoadBalanceStrategy), payAmount, excludedInstanceIDs)
		if err == nil && sel != nil {
			return sel, nil
		}
	}
	sel, err := s.loadBalancer.SelectInstanceExcept(ctx, "", req.PaymentType, payment.Strategy(cfg.LoadBalanceStrategy), payAmount, excludedInstanceIDs)
	if err != nil {
		return nil, infraerrors.ServiceUnavailable("PAYMENT_GATEWAY_ERROR", fmt.Sprintf("payment method (%s) is not configured", req.PaymentType))
	}
	if sel == nil {
		return nil, infraerrors.TooManyRequests("NO_AVAILABLE_INSTANCE", "no available payment instance")
	}
	return sel, nil
}

func (s *PaymentService) createOrderWithSelectionRetry(
	ctx context.Context,
	req CreateOrderRequest,
	cfg *PaymentConfig,
	plan *dbent.SubscriptionPlan,
	orderAmount, limitAmount, feeRate, payAmount float64,
	prepare func(sel *payment.InstanceSelection) (*CreateOrderResponse, error),
) (*payment.InstanceSelection, *dbent.PaymentOrder, *CreateOrderResponse, error) {
	if s.loadBalancer == nil || s.entClient == nil || s.registry == nil {
		oauthResp, err := prepare(nil)
		if err != nil || oauthResp != nil {
			return nil, nil, oauthResp, err
		}
		return nil, nil, nil, infraerrors.ServiceUnavailable("PAYMENT_GATEWAY_ERROR", fmt.Sprintf("payment method (%s) is not configured", req.PaymentType))
	}
	return psCreateOrderWithSelectionRetry(
		func(excludedInstanceIDs []string) (*payment.InstanceSelection, *CreateOrderResponse, error) {
			selection, err := s.selectProviderInstance(ctx, req, cfg, payAmount, excludedInstanceIDs)
			if err != nil {
				return nil, nil, err
			}
			oauthResp, err := prepare(selection)
			if err != nil {
				return nil, nil, err
			}
			return selection, oauthResp, nil
		},
		func(sel *payment.InstanceSelection) (*dbent.PaymentOrder, error) {
			return s.createOrderInTx(ctx, req, plan, cfg, orderAmount, limitAmount, feeRate, payAmount, sel)
		},
	)
}

func psCreateOrderWithSelectionRetry(
	selectAndPrepare func(excludedInstanceIDs []string) (*payment.InstanceSelection, *CreateOrderResponse, error),
	create func(sel *payment.InstanceSelection) (*dbent.PaymentOrder, error),
) (*payment.InstanceSelection, *dbent.PaymentOrder, *CreateOrderResponse, error) {
	const maxSelectionAttempts = 3

	excludedInstanceIDs := make([]string, 0, maxSelectionAttempts-1)
	for attempt := 0; attempt < maxSelectionAttempts; attempt++ {
		selection, oauthResp, err := selectAndPrepare(excludedInstanceIDs)
		if err != nil {
			return nil, nil, nil, err
		}
		if oauthResp != nil {
			return selection, nil, oauthResp, nil
		}
		order, err := create(selection)
		if err == nil {
			return selection, order, nil, nil
		}
		if infraerrors.Reason(err) != "NO_AVAILABLE_INSTANCE" || selection == nil || strings.TrimSpace(selection.InstanceID) == "" {
			return nil, nil, nil, err
		}
		excludedInstanceIDs = append(excludedInstanceIDs, selection.InstanceID)
	}
	return nil, nil, nil, infraerrors.TooManyRequests("NO_AVAILABLE_INSTANCE", "no available payment instance")
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
	lockedUser, err := tx.User.Query().
		Where(user.IDEQ(req.UserID)).
		ForUpdate().
		Only(ctx)
	if err != nil && psIsSQLiteForUpdateUnsupported(err) {
		lockedUser, err = tx.User.Query().
			Where(user.IDEQ(req.UserID)).
			Only(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("lock user for order creation: %w", err)
	}
	if lockedUser.Status != payment.EntityStatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}
	if err := s.checkPendingLimit(ctx, tx, req.UserID, cfg.MaxPendingOrders); err != nil {
		return nil, err
	}
	if err := s.checkDailyLimit(ctx, tx, req.UserID, psDailyLimitAmount(req.OrderType, limitAmount, payAmount), cfg.DailyLimit); err != nil {
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
		used += psDailyLimitAmount(o.OrderType, o.Amount, o.PayAmount)
	}
	if used+amount > limit {
		return infraerrors.TooManyRequests("DAILY_LIMIT_EXCEEDED", fmt.Sprintf("daily recharge limit reached, remaining: %.2f", math.Max(0, limit-used)))
	}
	return nil
}

func psDailyLimitAmount(orderType string, amount, payAmount float64) float64 {
	if orderType == payment.OrderTypeBalance {
		return payAmount
	}
	return amount
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
	if err != nil && psIsSQLiteForUpdateUnsupported(err) {
		inst, err = tx.PaymentProviderInstance.Query().
			Where(paymentproviderinstance.IDEQ(instID)).
			Only(ctx)
	}
	if err != nil {
		return fmt.Errorf("lock selected payment instance: %w", err)
	}
	if !inst.Enabled || (!payment.InstanceSupportsType(inst.SupportedTypes, paymentType) && inst.ProviderKey != payment.GetBasePaymentType(paymentType)) {
		return infraerrors.TooManyRequests("NO_AVAILABLE_INSTANCE", "selected payment instance is no longer available")
	}
	orders, err := tx.PaymentOrder.Query().
		Where(
			paymentorder.ProviderInstanceIDEQ(sel.InstanceID),
			paymentorder.StatusIn(OrderStatusPending, OrderStatusPaid, OrderStatusCompleted, OrderStatusRecharging),
		).
		All(ctx)
	if err != nil {
		return fmt.Errorf("query selected instance usage: %w", err)
	}
	todayStart := psStartOfDayUTC(time.Now())
	var usage float64
	for _, order := range orders {
		if !psOrderCountsForProviderDailyUsage(order, todayStart) {
			continue
		}
		usage += order.PayAmount
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
	lookupKey := string(payment.NormalizeVisiblePaymentType(paymentType))
	if inst.ProviderKey == payment.TypeStripe {
		lookupKey = payment.TypeStripe
	}
	if cl, ok := limits[lookupKey]; ok {
		return cl
	}
	return payment.ChannelLimits{}
}

func psIsSQLiteForUpdateUnsupported(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOR UPDATE/SHARE not supported in SQLite")
}

func psOrderCountsForProviderDailyUsage(order *dbent.PaymentOrder, todayStart time.Time) bool {
	if order == nil {
		return false
	}
	if order.Status == OrderStatusPending {
		return true
	}
	if order.Status == OrderStatusPaid || order.Status == OrderStatusCompleted || order.Status == OrderStatusRecharging {
		return order.PaidAt != nil && !order.PaidAt.Before(todayStart)
	}
	return false
}

func (s *PaymentService) invokeProvider(ctx context.Context, order *dbent.PaymentOrder, req CreateOrderRequest, cfg *PaymentConfig, limitAmount float64, payAmountStr string, payAmount float64, plan *dbent.SubscriptionPlan, sel *payment.InstanceSelection) (*CreateOrderResponse, error) {
	prov, err := provider.CreateProvider(sel.ProviderKey, sel.InstanceID, sel.Config)
	if err != nil {
		return nil, infraerrors.ServiceUnavailable("PAYMENT_GATEWAY_ERROR", "payment method is temporarily unavailable")
	}
	subject := s.buildPaymentSubject(plan, limitAmount, cfg)
	outTradeNo := order.OutTradeNo
	pr, err := prov.CreatePayment(ctx, payment.CreatePaymentRequest{
		OrderID:            outTradeNo,
		Amount:             payAmountStr,
		PaymentType:        req.PaymentType,
		Subject:            subject,
		ReturnURL:          req.SrcURL,
		OpenID:             req.OpenID,
		ClientIP:           req.ClientIP,
		IsMobile:           req.IsMobile,
		InstanceSubMethods: sel.SupportedTypes,
	})
	if err != nil {
		slog.Error("[PaymentService] CreatePayment failed", "provider", sel.ProviderKey, "instance", sel.InstanceID, "error", err)
		return nil, classifyCreatePaymentError(req, sel.ProviderKey, err)
	}
	_, err = s.entClient.PaymentOrder.UpdateOneID(order.ID).SetNillablePaymentTradeNo(psNilIfEmpty(pr.TradeNo)).SetNillablePayURL(psNilIfEmpty(pr.PayURL)).SetNillableQrCode(psNilIfEmpty(pr.QRCode)).SetNillableProviderInstanceID(psNilIfEmpty(sel.InstanceID)).Save(ctx)
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
	resultType := pr.ResultType
	if resultType == "" {
		resultType = payment.CreatePaymentResultOrderCreated
	}
	return buildCreateOrderResponse(order, req, payAmount, sel, pr, resultType), nil
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

func (s *PaymentService) maybeBuildWeChatOAuthRequiredResponse(ctx context.Context, req CreateOrderRequest, amount, payAmount, feeRate float64) (*CreateOrderResponse, error) {
	return s.maybeBuildWeChatOAuthRequiredResponseForSelection(ctx, req, amount, payAmount, feeRate, nil)
}

func (s *PaymentService) maybeBuildWeChatOAuthRequiredResponseForSelection(ctx context.Context, req CreateOrderRequest, amount, payAmount, feeRate float64, sel *payment.InstanceSelection) (*CreateOrderResponse, error) {
	if sel != nil && sel.ProviderKey != "" && sel.ProviderKey != payment.TypeWxpay {
		return nil, nil
	}
	if strings.TrimSpace(req.OpenID) != "" || !req.IsWeChatBrowser || payment.GetBasePaymentType(req.PaymentType) != payment.TypeWxpay {
		return nil, nil
	}
	return s.buildWeChatOAuthRequiredResponse(ctx, req, amount, payAmount, feeRate)
}

func (s *PaymentService) buildWeChatOAuthRequiredResponse(ctx context.Context, req CreateOrderRequest, amount, payAmount, feeRate float64) (*CreateOrderResponse, error) {
	if s == nil || s.configService == nil || s.configService.settingRepo == nil {
		return nil, infraerrors.ServiceUnavailable(
			"WECHAT_PAYMENT_MP_NOT_CONFIGURED",
			"wechat in-app payment requires a configured WeChat MP OAuth credential",
		)
	}

	settings, err := s.configService.settingRepo.GetMultiple(ctx, []string{
		SettingKeyWeChatLoginMPEnabled,
		SettingKeyWeChatLoginMPAppID,
		SettingKeyWeChatLoginMPAppSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("get wechat payment oauth config: %w", err)
	}

	if settings[SettingKeyWeChatLoginMPEnabled] != "true" {
		return nil, infraerrors.ServiceUnavailable(
			"WECHAT_PAYMENT_MP_NOT_CONFIGURED",
			"wechat in-app payment requires an enabled WeChat MP OAuth credential",
		)
	}
	appID := strings.TrimSpace(settings[SettingKeyWeChatLoginMPAppID])
	appSecret := strings.TrimSpace(settings[SettingKeyWeChatLoginMPAppSecret])
	if appID == "" || appSecret == "" {
		return nil, infraerrors.ServiceUnavailable(
			"WECHAT_PAYMENT_MP_NOT_CONFIGURED",
			"wechat in-app payment requires a complete WeChat MP OAuth credential",
		)
	}

	authorizeURL, err := buildWeChatPaymentOAuthStartURL(req, "snsapi_base")
	if err != nil {
		return nil, err
	}

	return &CreateOrderResponse{
		Amount:      amount,
		PayAmount:   payAmount,
		FeeRate:     feeRate,
		ResultType:  payment.CreatePaymentResultOAuthRequired,
		PaymentType: req.PaymentType,
		OAuth: &payment.WechatOAuthInfo{
			AuthorizeURL: authorizeURL,
			AppID:        appID,
			Scope:        "snsapi_base",
			RedirectURL:  "/auth/wechat/payment/callback",
		},
	}, nil
}

func classifyCreatePaymentError(req CreateOrderRequest, providerKey string, err error) error {
	if err == nil {
		return nil
	}
	if providerKey == payment.TypeWxpay &&
		payment.GetBasePaymentType(req.PaymentType) == payment.TypeWxpay &&
		strings.Contains(err.Error(), "wxpay h5 payments are not authorized for this merchant") {
		return infraerrors.ServiceUnavailable(
			"WECHAT_H5_NOT_AUTHORIZED",
			"wechat h5 payment is not available for this merchant",
		).WithMetadata(map[string]string{
			"action": "open_in_wechat_or_scan_qr",
		})
	}
	return infraerrors.ServiceUnavailable("PAYMENT_GATEWAY_ERROR", fmt.Sprintf("payment gateway error: %s", err.Error()))
}

func buildCreateOrderResponse(order *dbent.PaymentOrder, req CreateOrderRequest, payAmount float64, sel *payment.InstanceSelection, pr *payment.CreatePaymentResponse, resultType payment.CreatePaymentResultType) *CreateOrderResponse {
	return &CreateOrderResponse{
		OrderID:      order.ID,
		Amount:       order.Amount,
		PayAmount:    payAmount,
		FeeRate:      order.FeeRate,
		Status:       OrderStatusPending,
		ResultType:   resultType,
		PaymentType:  req.PaymentType,
		OutTradeNo:   order.OutTradeNo,
		PayURL:       pr.PayURL,
		QRCode:       pr.QRCode,
		ClientSecret: pr.ClientSecret,
		OAuth:        pr.OAuth,
		JSAPI:        pr.JSAPI,
		JSAPIPayload: pr.JSAPI,
		ExpiresAt:    order.ExpiresAt,
		PaymentMode:  sel.PaymentMode,
	}
}

func buildWeChatPaymentOAuthStartURL(req CreateOrderRequest, scope string) (string, error) {
	u, err := url.Parse("/api/v1/auth/oauth/wechat/payment/start")
	if err != nil {
		return "", fmt.Errorf("build wechat payment oauth start url: %w", err)
	}
	q := u.Query()
	q.Set("payment_type", strings.TrimSpace(req.PaymentType))
	if req.Amount > 0 {
		q.Set("amount", strconv.FormatFloat(req.Amount, 'f', -1, 64))
	}
	if orderType := strings.TrimSpace(req.OrderType); orderType != "" {
		q.Set("order_type", orderType)
	}
	if req.PlanID > 0 {
		q.Set("plan_id", strconv.FormatInt(req.PlanID, 10))
	}
	if scope = strings.TrimSpace(scope); scope != "" {
		q.Set("scope", scope)
	}
	if redirectTo := paymentRedirectPathFromURL(req.SrcURL); redirectTo != "" {
		q.Set("redirect", redirectTo)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func paymentRedirectPathFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "/purchase"
	}
	if strings.HasPrefix(rawURL, "/") && !strings.HasPrefix(rawURL, "//") {
		return normalizePaymentRedirectPath(rawURL)
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "/purchase"
	}
	path := strings.TrimSpace(u.EscapedPath())
	if path == "" {
		path = strings.TrimSpace(u.Path)
	}
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "/purchase"
	}
	if strings.TrimSpace(u.RawQuery) != "" {
		path += "?" + u.RawQuery
	}
	return normalizePaymentRedirectPath(path)
}

func normalizePaymentRedirectPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/purchase"
	}
	if path == "/payment" {
		return "/purchase"
	}
	if strings.HasPrefix(path, "/payment?") {
		return "/purchase" + strings.TrimPrefix(path, "/payment")
	}
	return path
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

func (s *PaymentService) GetUserOrders(ctx context.Context, userID int64, p OrderListParams) ([]*dbent.PaymentOrder, int, error) {
	q := s.entClient.PaymentOrder.Query().Where(paymentorder.UserIDEQ(userID))
	if p.Status != "" {
		q = q.Where(paymentorder.StatusEQ(p.Status))
	}
	if p.OrderType != "" {
		q = q.Where(paymentorder.OrderTypeEQ(p.OrderType))
	}
	if p.PaymentType != "" {
		if types := psPaymentTypeFilterValues(p.PaymentType); len(types) > 0 {
			q = q.Where(paymentorder.PaymentTypeIn(types...))
		}
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
		if types := psPaymentTypeFilterValues(p.PaymentType); len(types) > 0 {
			q = q.Where(paymentorder.PaymentTypeIn(types...))
		}
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
