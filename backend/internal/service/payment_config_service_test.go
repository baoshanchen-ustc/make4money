package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type paymentConfigSettingRepoStub struct {
	data map[string]string
}

func (s *paymentConfigSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *paymentConfigSettingRepoStub) GetValue(context.Context, string) (string, error) {
	return "", nil
}

func (s *paymentConfigSettingRepoStub) Set(context.Context, string, string) error {
	return nil
}

func (s *paymentConfigSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	result := make(map[string]string, len(s.data))
	for key, value := range s.data {
		result[key] = value
	}
	return result, nil
}

func (s *paymentConfigSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	if s.data == nil {
		s.data = make(map[string]string)
	}
	for key, value := range settings {
		s.data[key] = value
	}
	return nil
}

func (s *paymentConfigSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	result := make(map[string]string, len(s.data))
	for key, value := range s.data {
		result[key] = value
	}
	return result, nil
}

func (s *paymentConfigSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(s.data, key)
	return nil
}

func TestPcParseFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		defaultVal float64
		expected   float64
	}{
		{"empty string returns default", "", 1.0, 1.0},
		{"valid float", "3.14", 0, 3.14},
		{"valid integer as float", "42", 0, 42.0},
		{"invalid string returns default", "notanumber", 9.99, 9.99},
		{"zero value", "0", 5.0, 0},
		{"negative value", "-10.5", 0, -10.5},
		{"very large value", "99999999.99", 0, 99999999.99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := pcParseFloat(tt.input, tt.defaultVal)
			if got != tt.expected {
				t.Fatalf("pcParseFloat(%q, %v) = %v, want %v", tt.input, tt.defaultVal, got, tt.expected)
			}
		})
	}
}

func TestPcParseInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		defaultVal int
		expected   int
	}{
		{"empty string returns default", "", 30, 30},
		{"valid int", "10", 0, 10},
		{"invalid string returns default", "abc", 5, 5},
		{"float string returns default", "3.14", 0, 0},
		{"zero value", "0", 99, 0},
		{"negative value", "-1", 0, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := pcParseInt(tt.input, tt.defaultVal)
			if got != tt.expected {
				t.Fatalf("pcParseInt(%q, %v) = %v, want %v", tt.input, tt.defaultVal, got, tt.expected)
			}
		})
	}
}

func TestParsePaymentConfig(t *testing.T) {
	t.Parallel()

	svc := &PaymentConfigService{}

	t.Run("empty vals uses defaults", func(t *testing.T) {
		t.Parallel()
		cfg := svc.parsePaymentConfig(map[string]string{})
		if cfg.Enabled {
			t.Fatal("expected Enabled=false by default")
		}
		if cfg.MinAmount != 1 {
			t.Fatalf("expected MinAmount=1, got %v", cfg.MinAmount)
		}
		if cfg.MaxAmount != 0 {
			t.Fatalf("expected MaxAmount=0 (no limit), got %v", cfg.MaxAmount)
		}
		if cfg.OrderTimeoutMin != 30 {
			t.Fatalf("expected OrderTimeoutMin=30, got %v", cfg.OrderTimeoutMin)
		}
		if cfg.MaxPendingOrders != 3 {
			t.Fatalf("expected MaxPendingOrders=3, got %v", cfg.MaxPendingOrders)
		}
		if cfg.LoadBalanceStrategy != payment.DefaultLoadBalanceStrategy {
			t.Fatalf("expected LoadBalanceStrategy=%s, got %q", payment.DefaultLoadBalanceStrategy, cfg.LoadBalanceStrategy)
		}
		if len(cfg.EnabledTypes) != 0 {
			t.Fatalf("expected empty EnabledTypes, got %v", cfg.EnabledTypes)
		}
	})

	t.Run("all values populated", func(t *testing.T) {
		t.Parallel()
		vals := map[string]string{
			SettingPaymentEnabled:      "true",
			SettingMinRechargeAmount:   "5.00",
			SettingMaxRechargeAmount:   "1000.00",
			SettingDailyRechargeLimit:  "5000.00",
			SettingOrderTimeoutMinutes: "15",
			SettingMaxPendingOrders:    "5",
			SettingEnabledPaymentTypes: "alipay,wxpay,stripe",
			SettingBalancePayDisabled:  "true",
			SettingLoadBalanceStrategy: "least_amount",
			SettingProductNamePrefix:   "PRE",
			SettingProductNameSuffix:   "SUF",
		}
		cfg := svc.parsePaymentConfig(vals)

		if !cfg.Enabled {
			t.Fatal("expected Enabled=true")
		}
		if cfg.MinAmount != 5 {
			t.Fatalf("MinAmount = %v, want 5", cfg.MinAmount)
		}
		if cfg.MaxAmount != 1000 {
			t.Fatalf("MaxAmount = %v, want 1000", cfg.MaxAmount)
		}
		if cfg.DailyLimit != 5000 {
			t.Fatalf("DailyLimit = %v, want 5000", cfg.DailyLimit)
		}
		if cfg.OrderTimeoutMin != 15 {
			t.Fatalf("OrderTimeoutMin = %v, want 15", cfg.OrderTimeoutMin)
		}
		if cfg.MaxPendingOrders != 5 {
			t.Fatalf("MaxPendingOrders = %v, want 5", cfg.MaxPendingOrders)
		}
		if len(cfg.EnabledTypes) != 3 {
			t.Fatalf("EnabledTypes len = %d, want 3", len(cfg.EnabledTypes))
		}
		if cfg.EnabledTypes[0] != "alipay" || cfg.EnabledTypes[1] != "wxpay" || cfg.EnabledTypes[2] != "stripe" {
			t.Fatalf("EnabledTypes = %v, want [alipay wxpay stripe]", cfg.EnabledTypes)
		}
		if !cfg.BalanceDisabled {
			t.Fatal("expected BalanceDisabled=true")
		}
		if cfg.LoadBalanceStrategy != "least_amount" {
			t.Fatalf("LoadBalanceStrategy = %q, want %q", cfg.LoadBalanceStrategy, "least_amount")
		}
		if cfg.ProductNamePrefix != "PRE" {
			t.Fatalf("ProductNamePrefix = %q, want %q", cfg.ProductNamePrefix, "PRE")
		}
		if cfg.ProductNameSuffix != "SUF" {
			t.Fatalf("ProductNameSuffix = %q, want %q", cfg.ProductNameSuffix, "SUF")
		}
	})

	t.Run("enabled types with spaces are trimmed", func(t *testing.T) {
		t.Parallel()
		vals := map[string]string{
			SettingEnabledPaymentTypes: " alipay , wxpay ",
		}
		cfg := svc.parsePaymentConfig(vals)
		if len(cfg.EnabledTypes) != 2 {
			t.Fatalf("EnabledTypes len = %d, want 2", len(cfg.EnabledTypes))
		}
		if cfg.EnabledTypes[0] != "alipay" || cfg.EnabledTypes[1] != "wxpay" {
			t.Fatalf("EnabledTypes = %v, want [alipay wxpay]", cfg.EnabledTypes)
		}
	})

	t.Run("empty enabled types string", func(t *testing.T) {
		t.Parallel()
		vals := map[string]string{
			SettingEnabledPaymentTypes: "",
		}
		cfg := svc.parsePaymentConfig(vals)
		if len(cfg.EnabledTypes) != 0 {
			t.Fatalf("expected empty EnabledTypes for empty string, got %v", cfg.EnabledTypes)
		}
	})
}

func TestGetBasePaymentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{payment.TypeEasyPay, payment.TypeEasyPay},
		{payment.TypeStripe, payment.TypeStripe},
		{payment.TypeCard, payment.TypeStripe},
		{payment.TypeLink, payment.TypeStripe},
		{payment.TypeAlipay, payment.TypeAlipay},
		{payment.TypeAlipayDirect, payment.TypeAlipay},
		{payment.TypeWxpay, payment.TypeWxpay},
		{payment.TypeWxpayDirect, payment.TypeWxpay},
		{"unknown", "unknown"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := payment.GetBasePaymentType(tt.input)
			if got != tt.expected {
				t.Fatalf("GetBasePaymentType(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMergePaymentConfigPatchPreservesOmittedFields(t *testing.T) {
	t.Parallel()

	current := &PaymentConfig{
		Enabled:                true,
		MinAmount:              10,
		MaxAmount:              200,
		DailyLimit:             500,
		OrderTimeoutMin:        15,
		MaxPendingOrders:       4,
		EnabledTypes:           []string{"alipay", "wxpay"},
		BalanceDisabled:        true,
		LoadBalanceStrategy:    string(payment.StrategyLeastAmount),
		ProductNamePrefix:      "PRE",
		ProductNameSuffix:      "SUF",
		HelpImageURL:           "https://example.com/help.png",
		HelpText:               "help",
		CancelRateLimitEnabled: true,
		CancelRateLimitMax:     5,
		CancelRateLimitWindow:  60,
		CancelRateLimitUnit:    "minute",
		CancelRateLimitMode:    "rolling",
	}

	newMin := 25.0
	req := UpdatePaymentConfigRequest{
		MinAmount: &newMin,
	}

	merged := mergePaymentConfigPatch(current, req)

	if merged.MinAmount != 25 {
		t.Fatalf("MinAmount = %v, want 25", merged.MinAmount)
	}
	if merged.MaxAmount != current.MaxAmount {
		t.Fatalf("MaxAmount = %v, want %v", merged.MaxAmount, current.MaxAmount)
	}
	if len(merged.EnabledTypes) != 2 || merged.EnabledTypes[0] != "alipay" || merged.EnabledTypes[1] != "wxpay" {
		t.Fatalf("EnabledTypes = %v, want [alipay wxpay]", merged.EnabledTypes)
	}
	if !merged.BalanceDisabled {
		t.Fatal("expected BalanceDisabled to be preserved")
	}
	if merged.HelpText != current.HelpText {
		t.Fatalf("HelpText = %q, want %q", merged.HelpText, current.HelpText)
	}
	if !merged.CancelRateLimitEnabled || merged.CancelRateLimitMax != 5 || merged.CancelRateLimitWindow != 60 {
		t.Fatalf("cancel rate limit settings were not preserved: %+v", merged)
	}
}

func TestMergePaymentConfigPatchAllowsClearingEnabledTypes(t *testing.T) {
	t.Parallel()

	current := &PaymentConfig{
		Enabled:      true,
		EnabledTypes: []string{"alipay", "wxpay"},
	}
	req := UpdatePaymentConfigRequest{
		EnabledTypes: []string{},
	}

	merged := mergePaymentConfigPatch(current, req)
	if len(merged.EnabledTypes) != 0 {
		t.Fatalf("EnabledTypes = %v, want empty slice", merged.EnabledTypes)
	}

	serialized := serializePaymentConfig(merged)
	if got := serialized[SettingEnabledPaymentTypes]; got != "" {
		t.Fatalf("serialized enabled types = %q, want empty string", got)
	}
}

func TestIsPaymentTypeEnabled(t *testing.T) {
	t.Parallel()

	t.Run("normalized allowlist matches direct types", func(t *testing.T) {
		t.Parallel()

		cfg := &PaymentConfig{
			EnabledTypes: []string{"alipay", "stripe"},
		}

		if !isPaymentTypeEnabled(cfg, "alipay") {
			t.Fatal("expected alipay to be enabled")
		}
		if !isPaymentTypeEnabled(cfg, " Stripe ") {
			t.Fatal("expected stripe to be enabled with case/space normalization")
		}
		if isPaymentTypeEnabled(cfg, "wxpay") {
			t.Fatal("expected wxpay to be disabled")
		}
		if isPaymentTypeEnabled(cfg, "") {
			t.Fatal("expected empty payment type to be disabled")
		}
	})

	t.Run("empty allowlist permits configured payment methods", func(t *testing.T) {
		t.Parallel()

		cfg := &PaymentConfig{}
		if !isPaymentTypeEnabled(cfg, "alipay") {
			t.Fatal("expected empty allowlist to permit alipay")
		}
		if !isPaymentTypeEnabled(cfg, "stripe") {
			t.Fatal("expected empty allowlist to permit stripe")
		}
	})

	t.Run("easypay allowlist enables alipay and wxpay subtypes", func(t *testing.T) {
		t.Parallel()

		cfg := &PaymentConfig{EnabledTypes: []string{" EasyPay "}}
		if !isPaymentTypeEnabled(cfg, "alipay") {
			t.Fatal("expected easypay to enable alipay")
		}
		if !isPaymentTypeEnabled(cfg, "wxpay_direct") {
			t.Fatal("expected easypay to enable wxpay_direct")
		}
		if isPaymentTypeEnabled(cfg, "stripe") {
			t.Fatal("expected stripe to remain disabled")
		}
	})
}

func TestBuildUpdateMapDoesNotMaterializeOmittedDefaults(t *testing.T) {
	t.Parallel()

	svc := &PaymentConfigService{}
	minAmount := 25.0
	updates, err := svc.BuildUpdateMap(context.Background(), UpdatePaymentConfigRequest{
		MinAmount: &minAmount,
	})
	if err != nil {
		t.Fatalf("BuildUpdateMap returned error: %v", err)
	}
	if got := updates[SettingMinRechargeAmount]; got != "25.00" {
		t.Fatalf("min amount update = %q, want 25.00", got)
	}
	if _, ok := updates[SettingLoadBalanceStrategy]; ok {
		t.Fatal("omitted load balance strategy should not be materialized")
	}
	if _, ok := updates[SettingOrderTimeoutMinutes]; ok {
		t.Fatal("omitted order timeout should not be materialized")
	}
}

func TestBuildUpdateMapRejectsInvalidBalanceRechargeMultiplier(t *testing.T) {
	t.Parallel()

	svc := &PaymentConfigService{}
	invalid := 0.0

	_, err := svc.BuildUpdateMap(context.Background(), UpdatePaymentConfigRequest{
		BalanceRechargeMultiplier: &invalid,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if got := infraerrors.Code(err); got != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", got, http.StatusBadRequest)
	}
	if got := infraerrors.Reason(err); got != "INVALID_BALANCE_RECHARGE_MULTIPLIER" {
		t.Fatalf("reason = %q, want %q", got, "INVALID_BALANCE_RECHARGE_MULTIPLIER")
	}
	if got := infraerrors.Message(err); got != "balance recharge multiplier must be greater than 0" {
		t.Fatalf("message = %q, want %q", got, "balance recharge multiplier must be greater than 0")
	}
}

func TestBuildUpdateMapRejectsInvalidRechargeFeeRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   float64
		reason  string
		message string
	}{
		{
			name:    "out of range",
			value:   100.01,
			reason:  "INVALID_RECHARGE_FEE_RATE",
			message: "recharge fee rate must be between 0 and 100",
		},
		{
			name:    "too many decimals",
			value:   1.234,
			reason:  "INVALID_RECHARGE_FEE_RATE",
			message: "recharge fee rate allows at most 2 decimal places",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := &PaymentConfigService{}
			_, err := svc.BuildUpdateMap(context.Background(), UpdatePaymentConfigRequest{
				RechargeFeeRate: &tt.value,
			})
			if err == nil {
				t.Fatal("expected validation error")
			}
			if got := infraerrors.Code(err); got != http.StatusBadRequest {
				t.Fatalf("status code = %d, want %d", got, http.StatusBadRequest)
			}
			if got := infraerrors.Reason(err); got != tt.reason {
				t.Fatalf("reason = %q, want %q", got, tt.reason)
			}
			if got := infraerrors.Message(err); got != tt.message {
				t.Fatalf("message = %q, want %q", got, tt.message)
			}
		})
	}
}

func TestUpdatePaymentConfigUsesSharedSerializedPatchFlow(t *testing.T) {
	t.Parallel()

	repo := &paymentConfigSettingRepoStub{data: map[string]string{}}
	repo.data[SettingLoadBalanceStrategy] = string(payment.StrategyLeastAmount)
	repo.data[SettingEnabledPaymentTypes] = "alipay"

	svc := &PaymentConfigService{settingRepo: repo}
	loadBalanceStrategy := ""
	balanceMultiplier := 2.5
	req := UpdatePaymentConfigRequest{
		LoadBalanceStrategy:       &loadBalanceStrategy,
		EnabledTypes:              []string{" Stripe ", "card"},
		BalanceRechargeMultiplier: &balanceMultiplier,
	}

	expected, err := svc.BuildUpdateMap(context.Background(), req)
	if err != nil {
		t.Fatalf("BuildUpdateMap returned error: %v", err)
	}

	if err := svc.UpdatePaymentConfig(context.Background(), req); err != nil {
		t.Fatalf("UpdatePaymentConfig returned error: %v", err)
	}

	if len(repo.data) < len(expected) {
		t.Fatalf("repo stored %d settings, expected at least %d", len(repo.data), len(expected))
	}
	for key, want := range expected {
		if got := repo.data[key]; got != want {
			t.Fatalf("stored value for %s = %q, want %q", key, got, want)
		}
	}
	if _, ok := expected[SettingOrderTimeoutMinutes]; ok {
		t.Fatal("shared flow should not materialize omitted order timeout")
	}
}
