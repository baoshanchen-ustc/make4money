package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestUnionFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		agg         float64
		limited     bool
		val         float64
		wantMin     bool
		wantAgg     float64
		wantLimited bool
	}{
		{"first non-zero value", 0, true, 5, true, 5, true},
		{"lower min replaces", 10, true, 3, true, 3, true},
		{"higher min does not replace", 3, true, 10, true, 3, true},
		{"higher max replaces", 10, true, 20, false, 20, true},
		{"lower max does not replace", 20, true, 10, false, 20, true},
		{"zero value makes unlimited", 5, true, 0, true, 5, false},
		{"already unlimited stays unlimited", 5, false, 10, true, 5, false},
		{"zero on first call", 0, true, 0, true, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotAgg, gotLimited := unionFloat(tt.agg, tt.limited, tt.val, tt.wantMin)
			if gotAgg != tt.wantAgg || gotLimited != tt.wantLimited {
				t.Fatalf("unionFloat(%v, %v, %v, %v) = (%v, %v), want (%v, %v)",
					tt.agg, tt.limited, tt.val, tt.wantMin,
					gotAgg, gotLimited, tt.wantAgg, tt.wantLimited)
			}
		})
	}
}

func makeInstance(id int64, providerKey, supportedTypes, limits string) *dbent.PaymentProviderInstance {
	return &dbent.PaymentProviderInstance{
		ID:             id,
		ProviderKey:    providerKey,
		SupportedTypes: supportedTypes,
		Limits:         limits,
		Enabled:        true,
	}
}

func TestPcAggregateMethodLimits(t *testing.T) {
	t.Parallel()

	t.Run("single instance with limits", func(t *testing.T) {
		t.Parallel()
		inst := makeInstance(1, "easypay", "alipay,wxpay",
			`{"alipay":{"singleMin":2,"singleMax":14},"wxpay":{"singleMin":1,"singleMax":12}}`)
		ml := pcAggregateMethodLimits("alipay", []*dbent.PaymentProviderInstance{inst})
		if ml.SingleMin != 2 || ml.SingleMax != 14 {
			t.Fatalf("alipay limits = min:%v max:%v, want min:2 max:14", ml.SingleMin, ml.SingleMax)
		}
	})

	t.Run("two instances union takes widest range", func(t *testing.T) {
		t.Parallel()
		inst1 := makeInstance(1, "easypay", "alipay,wxpay",
			`{"alipay":{"singleMin":5,"singleMax":100}}`)
		inst2 := makeInstance(2, "easypay", "alipay,wxpay",
			`{"alipay":{"singleMin":2,"singleMax":200}}`)
		ml := pcAggregateMethodLimits("alipay", []*dbent.PaymentProviderInstance{inst1, inst2})
		if ml.SingleMin != 2 {
			t.Fatalf("SingleMin = %v, want 2 (lowest floor)", ml.SingleMin)
		}
		if ml.SingleMax != 200 {
			t.Fatalf("SingleMax = %v, want 200 (highest ceiling)", ml.SingleMax)
		}
	})

	t.Run("one instance unlimited makes aggregate unlimited", func(t *testing.T) {
		t.Parallel()
		inst1 := makeInstance(1, "easypay", "wxpay",
			`{"wxpay":{"singleMin":3,"singleMax":10}}`)
		inst2 := makeInstance(2, "easypay", "wxpay", "") // no limits = unlimited
		ml := pcAggregateMethodLimits("wxpay", []*dbent.PaymentProviderInstance{inst1, inst2})
		if ml.SingleMin != 0 || ml.SingleMax != 0 {
			t.Fatalf("limits = min:%v max:%v, want min:0 max:0 (unlimited)", ml.SingleMin, ml.SingleMax)
		}
	})

	t.Run("one field unlimited others limited", func(t *testing.T) {
		t.Parallel()
		inst1 := makeInstance(1, "easypay", "alipay",
			`{"alipay":{"singleMin":5,"singleMax":100}}`)
		inst2 := makeInstance(2, "easypay", "alipay",
			`{"alipay":{"singleMin":3,"singleMax":0}}`) // singleMax=0 = unlimited
		ml := pcAggregateMethodLimits("alipay", []*dbent.PaymentProviderInstance{inst1, inst2})
		if ml.SingleMin != 3 {
			t.Fatalf("SingleMin = %v, want 3 (lowest floor)", ml.SingleMin)
		}
		if ml.SingleMax != 0 {
			t.Fatalf("SingleMax = %v, want 0 (unlimited)", ml.SingleMax)
		}
	})

	t.Run("empty instances returns zeros", func(t *testing.T) {
		t.Parallel()
		ml := pcAggregateMethodLimits("alipay", nil)
		if ml.SingleMin != 0 || ml.SingleMax != 0 || ml.DailyLimit != 0 {
			t.Fatalf("empty instances should return all zeros, got %+v", ml)
		}
	})

	t.Run("invalid JSON treated as unlimited", func(t *testing.T) {
		t.Parallel()
		inst := makeInstance(1, "easypay", "alipay", `{invalid json}`)
		ml := pcAggregateMethodLimits("alipay", []*dbent.PaymentProviderInstance{inst})
		if ml.SingleMin != 0 || ml.SingleMax != 0 {
			t.Fatalf("invalid JSON should be treated as unlimited, got %+v", ml)
		}
	})

	t.Run("type not in limits JSON treated as unlimited", func(t *testing.T) {
		t.Parallel()
		inst := makeInstance(1, "easypay", "alipay,wxpay",
			`{"wxpay":{"singleMin":1,"singleMax":10}}`) // only wxpay, no alipay
		ml := pcAggregateMethodLimits("alipay", []*dbent.PaymentProviderInstance{inst})
		if ml.SingleMin != 0 || ml.SingleMax != 0 {
			t.Fatalf("missing type should be treated as unlimited, got %+v", ml)
		}
	})

	t.Run("daily limit aggregation", func(t *testing.T) {
		t.Parallel()
		inst1 := makeInstance(1, "easypay", "alipay",
			`{"alipay":{"singleMin":1,"singleMax":100,"dailyLimit":500}}`)
		inst2 := makeInstance(2, "easypay", "alipay",
			`{"alipay":{"singleMin":2,"singleMax":200,"dailyLimit":1000}}`)
		ml := pcAggregateMethodLimits("alipay", []*dbent.PaymentProviderInstance{inst1, inst2})
		if ml.DailyLimit != 1000 {
			t.Fatalf("DailyLimit = %v, want 1000 (highest cap)", ml.DailyLimit)
		}
	})
}

func TestPcGroupByPaymentType(t *testing.T) {
	t.Parallel()

	t.Run("stripe instance maps all types to stripe group", func(t *testing.T) {
		t.Parallel()
		stripe := makeInstance(1, payment.TypeStripe, "card,alipay,link,wxpay", "")
		easypay := makeInstance(2, payment.TypeEasyPay, "alipay,wxpay", "")

		groups := pcGroupByPaymentType([]*dbent.PaymentProviderInstance{stripe, easypay})

		// Stripe instance should only be in "stripe" group
		if len(groups[payment.TypeStripe]) != 1 || groups[payment.TypeStripe][0].ID != 1 {
			t.Fatalf("stripe group should contain only stripe instance, got %v", groups[payment.TypeStripe])
		}
		// alipay group should only contain easypay, NOT stripe
		if len(groups[payment.TypeAlipay]) != 1 || groups[payment.TypeAlipay][0].ID != 2 {
			t.Fatalf("alipay group should contain only easypay instance, got %v", groups[payment.TypeAlipay])
		}
		// wxpay group should only contain easypay, NOT stripe
		if len(groups[payment.TypeWxpay]) != 1 || groups[payment.TypeWxpay][0].ID != 2 {
			t.Fatalf("wxpay group should contain only easypay instance, got %v", groups[payment.TypeWxpay])
		}
	})

	t.Run("multiple easypay instances in same groups", func(t *testing.T) {
		t.Parallel()
		ep1 := makeInstance(1, payment.TypeEasyPay, "alipay,wxpay", "")
		ep2 := makeInstance(2, payment.TypeEasyPay, "alipay,wxpay", "")

		groups := pcGroupByPaymentType([]*dbent.PaymentProviderInstance{ep1, ep2})

		if len(groups[payment.TypeAlipay]) != 2 {
			t.Fatalf("alipay group should have 2 instances, got %d", len(groups[payment.TypeAlipay]))
		}
		if len(groups[payment.TypeWxpay]) != 2 {
			t.Fatalf("wxpay group should have 2 instances, got %d", len(groups[payment.TypeWxpay]))
		}
	})

	t.Run("stripe with no supported types still in stripe group", func(t *testing.T) {
		t.Parallel()
		stripe := makeInstance(1, payment.TypeStripe, "", "")

		groups := pcGroupByPaymentType([]*dbent.PaymentProviderInstance{stripe})

		if len(groups[payment.TypeStripe]) != 1 {
			t.Fatalf("stripe with empty types should still be in stripe group, got %v", groups)
		}
	})

	t.Run("legacy direct types are grouped under visible capabilities", func(t *testing.T) {
		t.Parallel()
		easypay := makeInstance(1, payment.TypeEasyPay, "alipay_direct,wxpay_direct", "")

		groups := pcGroupByPaymentType([]*dbent.PaymentProviderInstance{easypay})

		if len(groups[payment.TypeAlipay]) != 1 || groups[payment.TypeAlipay][0].ID != 1 {
			t.Fatalf("alipay group should contain easypay legacy type, got %v", groups[payment.TypeAlipay])
		}
		if len(groups[payment.TypeWxpay]) != 1 || groups[payment.TypeWxpay][0].ID != 1 {
			t.Fatalf("wxpay group should contain easypay legacy type, got %v", groups[payment.TypeWxpay])
		}
		if _, ok := groups[payment.TypeAlipayDirect]; ok {
			t.Fatalf("legacy alipay_direct group should not be exposed: %v", groups)
		}
		if _, ok := groups[payment.TypeWxpayDirect]; ok {
			t.Fatalf("legacy wxpay_direct group should not be exposed: %v", groups)
		}
	})
}

func TestPcComputeGlobalRange(t *testing.T) {
	t.Parallel()

	t.Run("all methods have limits", func(t *testing.T) {
		t.Parallel()
		methods := map[string]MethodLimits{
			"alipay": {SingleMin: 2, SingleMax: 14},
			"wxpay":  {SingleMin: 1, SingleMax: 12},
			"stripe": {SingleMin: 5, SingleMax: 100},
		}
		gMin, gMax := pcComputeGlobalRange(methods)
		if gMin != 1 {
			t.Fatalf("global min = %v, want 1 (lowest floor)", gMin)
		}
		if gMax != 100 {
			t.Fatalf("global max = %v, want 100 (highest ceiling)", gMax)
		}
	})

	t.Run("one method unlimited makes global unlimited", func(t *testing.T) {
		t.Parallel()
		methods := map[string]MethodLimits{
			"alipay": {SingleMin: 2, SingleMax: 14},
			"stripe": {SingleMin: 0, SingleMax: 0}, // unlimited
		}
		gMin, gMax := pcComputeGlobalRange(methods)
		if gMin != 0 {
			t.Fatalf("global min = %v, want 0 (unlimited)", gMin)
		}
		if gMax != 0 {
			t.Fatalf("global max = %v, want 0 (unlimited)", gMax)
		}
	})

	t.Run("empty methods returns zeros", func(t *testing.T) {
		t.Parallel()
		gMin, gMax := pcComputeGlobalRange(map[string]MethodLimits{})
		if gMin != 0 || gMax != 0 {
			t.Fatalf("empty methods should return (0, 0), got (%v, %v)", gMin, gMax)
		}
	})

	t.Run("only min unlimited", func(t *testing.T) {
		t.Parallel()
		methods := map[string]MethodLimits{
			"alipay": {SingleMin: 0, SingleMax: 100},
			"wxpay":  {SingleMin: 5, SingleMax: 50},
		}
		gMin, gMax := pcComputeGlobalRange(methods)
		if gMin != 0 {
			t.Fatalf("global min = %v, want 0 (unlimited)", gMin)
		}
		if gMax != 100 {
			t.Fatalf("global max = %v, want 100", gMax)
		}
	})
}

func TestFilterMethodLimitsByEnabledTypes(t *testing.T) {
	t.Parallel()

	methods := map[string]MethodLimits{
		payment.TypeAlipay: {PaymentType: payment.TypeAlipay},
		payment.TypeWxpay:  {PaymentType: payment.TypeWxpay},
		payment.TypeStripe: {PaymentType: payment.TypeStripe},
	}

	t.Run("empty allowlist keeps all methods", func(t *testing.T) {
		t.Parallel()

		filtered := filterMethodLimitsByEnabledTypes(methods, &PaymentConfig{})
		if len(filtered) != 3 {
			t.Fatalf("len(filtered) = %d, want 3", len(filtered))
		}
	})

	t.Run("visible allowlist keeps matching payment methods", func(t *testing.T) {
		t.Parallel()

		filtered := filterMethodLimitsByEnabledTypes(methods, &PaymentConfig{
			EnabledTypes: []string{payment.TypeAlipay, payment.TypeWxpayDirect},
		})
		if len(filtered) != 2 {
			t.Fatalf("len(filtered) = %d, want 2", len(filtered))
		}
		if _, ok := filtered[payment.TypeAlipay]; !ok {
			t.Fatal("expected alipay to be preserved")
		}
		if _, ok := filtered[payment.TypeWxpay]; !ok {
			t.Fatal("expected wxpay to be preserved")
		}
		if _, ok := filtered[payment.TypeStripe]; ok {
			t.Fatal("expected stripe to be filtered out")
		}
	})
}

func TestComputeMethodUsageSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("unlimited instance reports available", func(t *testing.T) {
		t.Parallel()

		inst := makeInstance(1, payment.TypeEasyPay, payment.TypeAlipay, "")
		snapshot := computeMethodUsageSnapshot(payment.TypeAlipay, []*dbent.PaymentProviderInstance{inst}, map[int64]float64{1: 999})
		if !snapshot.available {
			t.Fatal("expected unlimited instance to be available")
		}
		if snapshot.dailyUsed != 0 || snapshot.dailyRemaining != 0 {
			t.Fatalf("unexpected unlimited snapshot values: %+v", snapshot)
		}
	})

	t.Run("exhausted limited instance reports unavailable", func(t *testing.T) {
		t.Parallel()

		inst := makeInstance(1, payment.TypeEasyPay, payment.TypeAlipay,
			`{"alipay":{"dailyLimit":100}}`)
		snapshot := computeMethodUsageSnapshot(payment.TypeAlipay, []*dbent.PaymentProviderInstance{inst}, map[int64]float64{1: 100})
		if snapshot.available {
			t.Fatal("expected exhausted instance to be unavailable")
		}
		if snapshot.dailyRemaining != 0 {
			t.Fatalf("dailyRemaining = %v, want 0", snapshot.dailyRemaining)
		}
	})

	t.Run("selects instance with best remaining capacity", func(t *testing.T) {
		t.Parallel()

		inst1 := makeInstance(1, payment.TypeEasyPay, payment.TypeAlipay,
			`{"alipay":{"dailyLimit":100}}`)
		inst2 := makeInstance(2, payment.TypeEasyPay, payment.TypeAlipay,
			`{"alipay":{"dailyLimit":120}}`)
		snapshot := computeMethodUsageSnapshot(payment.TypeAlipay,
			[]*dbent.PaymentProviderInstance{inst1, inst2},
			map[int64]float64{1: 80, 2: 30},
		)
		if !snapshot.available {
			t.Fatal("expected at least one instance to remain available")
		}
		if snapshot.dailyUsed != 30 {
			t.Fatalf("dailyUsed = %v, want 30", snapshot.dailyUsed)
		}
		if snapshot.dailyRemaining != 90 {
			t.Fatalf("dailyRemaining = %v, want 90", snapshot.dailyRemaining)
		}
	})
}

func TestSelectRequestedMethodLimits(t *testing.T) {
	t.Parallel()

	methods := map[string]MethodLimits{
		payment.TypeAlipay: {PaymentType: payment.TypeAlipay, Available: true},
		payment.TypeStripe: {PaymentType: payment.TypeStripe, Available: true},
	}

	got := selectRequestedMethodLimits(methods, []string{" alipay_direct ", "card", "wxpay"})
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	if got[0].PaymentType != payment.TypeAlipay {
		t.Fatalf("got[0].PaymentType = %q, want %q", got[0].PaymentType, payment.TypeAlipay)
	}
	if got[1].PaymentType != payment.TypeStripe {
		t.Fatalf("got[1].PaymentType = %q, want %q", got[1].PaymentType, payment.TypeStripe)
	}
	if got[2].PaymentType != payment.TypeWxpay {
		t.Fatalf("got[2].PaymentType = %q, want %q", got[2].PaymentType, payment.TypeWxpay)
	}
}

func TestLoadMethodUsageMap_CountsYesterdayPendingAndTodayPaidOrders(t *testing.T) {
	t.Parallel()

	client := newPaymentOrderEntClient(t)
	ctx := context.Background()
	now := time.Now().UTC()
	yesterday := now.Add(-24 * time.Hour)
	user := createPaymentOrderUserForTest(t, client, now, "usage@example.com")
	inst := createPaymentProviderInstanceForTest(t, client, payment.TypeAlipay, payment.TypeAlipay, `{"alipay":{"dailyLimit":300}}`)
	instanceID := strconv.FormatInt(int64(inst.ID), 10)

	_, err := client.PaymentOrder.Create().
		SetUserID(int64(user.ID)).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(40).
		SetPayAmount(40).
		SetFeeRate(0).
		SetRechargeCode("PAY-USAGE-1").
		SetOutTradeNo("trade-usage-1").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusPending).
		SetProviderInstanceID(instanceID).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		SetCreatedAt(yesterday).
		Save(ctx)
	if err != nil {
		t.Fatalf("create pending order: %v", err)
	}

	paidAt := now
	_, err = client.PaymentOrder.Create().
		SetUserID(int64(user.ID)).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(50).
		SetPayAmount(50).
		SetFeeRate(0).
		SetRechargeCode("PAY-USAGE-2").
		SetOutTradeNo("trade-usage-2").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("gateway-2").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusPaid).
		SetProviderInstanceID(instanceID).
		SetPaidAt(paidAt).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		SetCreatedAt(yesterday).
		Save(ctx)
	if err != nil {
		t.Fatalf("create paid order: %v", err)
	}

	svc := NewPaymentConfigService(client, paymentOrderResultSettingRepoStub{}, nil)
	usageMap, err := svc.loadMethodUsageMap(ctx, []*dbent.PaymentProviderInstance{inst})
	if err != nil {
		t.Fatalf("loadMethodUsageMap() error = %v", err)
	}
	if usageMap[int64(inst.ID)] != 90 {
		t.Fatalf("usageMap[%d] = %v, want 90", inst.ID, usageMap[int64(inst.ID)])
	}
}

func TestGetAvailableMethodLimits_AppliesGlobalFeeRate(t *testing.T) {
	t.Parallel()

	client := newPaymentOrderEntClient(t)
	ctx := context.Background()
	createPaymentProviderInstanceForTest(t, client, payment.TypeAlipay, payment.TypeAlipay, `{"alipay":{"dailyLimit":300}}`)

	svc := NewPaymentConfigService(client, paymentOrderResultSettingRepoStub{values: map[string]string{
		SettingRechargeFeeRate: "0.03",
	}}, nil)
	resp, err := svc.GetAvailableMethodLimits(ctx)
	if err != nil {
		t.Fatalf("GetAvailableMethodLimits() error = %v", err)
	}
	if resp.Methods[payment.TypeAlipay].FeeRate != 0.03 {
		t.Fatalf("alipay fee_rate = %v, want 0.03", resp.Methods[payment.TypeAlipay].FeeRate)
	}
}

func TestPcInstanceTypeLimits(t *testing.T) {
	t.Parallel()

	t.Run("empty limits string returns false", func(t *testing.T) {
		t.Parallel()
		inst := makeInstance(1, "easypay", "alipay", "")
		_, ok := pcInstanceTypeLimits(inst, "alipay")
		if ok {
			t.Fatal("expected ok=false for empty limits")
		}
	})

	t.Run("type found returns correct values", func(t *testing.T) {
		t.Parallel()
		inst := makeInstance(1, "easypay", "alipay",
			`{"alipay":{"singleMin":2,"singleMax":14,"dailyLimit":500}}`)
		cl, ok := pcInstanceTypeLimits(inst, "alipay")
		if !ok {
			t.Fatal("expected ok=true")
		}
		if cl.SingleMin != 2 || cl.SingleMax != 14 || cl.DailyLimit != 500 {
			t.Fatalf("limits = %+v, want min:2 max:14 daily:500", cl)
		}
	})

	t.Run("type not found returns false", func(t *testing.T) {
		t.Parallel()
		inst := makeInstance(1, "easypay", "alipay",
			`{"wxpay":{"singleMin":1}}`)
		_, ok := pcInstanceTypeLimits(inst, "alipay")
		if ok {
			t.Fatal("expected ok=false for missing type")
		}
	})

	t.Run("invalid JSON returns false", func(t *testing.T) {
		t.Parallel()
		inst := makeInstance(1, "easypay", "alipay", `{bad json}`)
		_, ok := pcInstanceTypeLimits(inst, "alipay")
		if ok {
			t.Fatal("expected ok=false for invalid JSON")
		}
	})

	t.Run("legacy direct key falls back to visible capability", func(t *testing.T) {
		t.Parallel()
		inst := makeInstance(1, "easypay", "wxpay",
			`{"wxpay_direct":{"singleMin":6,"singleMax":66}}`)
		cl, ok := pcInstanceTypeLimits(inst, "wxpay")
		if !ok {
			t.Fatal("expected ok=true for legacy direct key")
		}
		if cl.SingleMin != 6 || cl.SingleMax != 66 {
			t.Fatalf("limits = %+v, want min:6 max:66", cl)
		}
	})

	t.Run("stripe always reads stripe key", func(t *testing.T) {
		t.Parallel()
		inst := makeInstance(1, "stripe", "card,link",
			`{"stripe":{"singleMin":9,"singleMax":99},"wxpay":{"singleMin":1,"singleMax":10}}`)
		cl, ok := pcInstanceTypeLimits(inst, "wxpay")
		if !ok {
			t.Fatal("expected ok=true for stripe key lookup")
		}
		if cl.SingleMin != 9 || cl.SingleMax != 99 {
			t.Fatalf("limits = %+v, want min:9 max:99", cl)
		}
	})
}
