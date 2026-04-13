package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

const (
	SettingPaymentEnabled      = "payment_enabled"
	SettingMinRechargeAmount   = "MIN_RECHARGE_AMOUNT"
	SettingMaxRechargeAmount   = "MAX_RECHARGE_AMOUNT"
	SettingDailyRechargeLimit  = "DAILY_RECHARGE_LIMIT"
	SettingOrderTimeoutMinutes = "ORDER_TIMEOUT_MINUTES"
	SettingMaxPendingOrders    = "MAX_PENDING_ORDERS"
	SettingEnabledPaymentTypes = "ENABLED_PAYMENT_TYPES"
	SettingLoadBalanceStrategy = "LOAD_BALANCE_STRATEGY"
	SettingBalancePayDisabled  = "BALANCE_PAYMENT_DISABLED"
	SettingProductNamePrefix   = "PRODUCT_NAME_PREFIX"
	SettingProductNameSuffix   = "PRODUCT_NAME_SUFFIX"
	SettingHelpImageURL        = "PAYMENT_HELP_IMAGE_URL"
	SettingHelpText            = "PAYMENT_HELP_TEXT"
	SettingCancelRateLimitOn   = "CANCEL_RATE_LIMIT_ENABLED"
	SettingCancelRateLimitMax  = "CANCEL_RATE_LIMIT_MAX"
	SettingCancelWindowSize    = "CANCEL_RATE_LIMIT_WINDOW"
	SettingCancelWindowUnit    = "CANCEL_RATE_LIMIT_UNIT"
	SettingCancelWindowMode    = "CANCEL_RATE_LIMIT_WINDOW_MODE"
)

// Default values for payment configuration settings.
const (
	defaultOrderTimeoutMin  = 30
	defaultMaxPendingOrders = 3
)

// PaymentConfig holds the payment system configuration.
type PaymentConfig struct {
	Enabled              bool     `json:"enabled"`
	MinAmount            float64  `json:"min_amount"`
	MaxAmount            float64  `json:"max_amount"`
	DailyLimit           float64  `json:"daily_limit"`
	OrderTimeoutMin      int      `json:"order_timeout_minutes"`
	MaxPendingOrders     int      `json:"max_pending_orders"`
	EnabledTypes         []string `json:"enabled_payment_types"`
	BalanceDisabled      bool     `json:"balance_disabled"`
	LoadBalanceStrategy  string   `json:"load_balance_strategy"`
	ProductNamePrefix    string   `json:"product_name_prefix"`
	ProductNameSuffix    string   `json:"product_name_suffix"`
	HelpImageURL         string   `json:"help_image_url"`
	HelpText             string   `json:"help_text"`
	StripePublishableKey string   `json:"stripe_publishable_key,omitempty"`

	// Cancel rate limit settings
	CancelRateLimitEnabled bool   `json:"cancel_rate_limit_enabled"`
	CancelRateLimitMax     int    `json:"cancel_rate_limit_max"`
	CancelRateLimitWindow  int    `json:"cancel_rate_limit_window"`
	CancelRateLimitUnit    string `json:"cancel_rate_limit_unit"`
	CancelRateLimitMode    string `json:"cancel_rate_limit_window_mode"`
}

// UpdatePaymentConfigRequest contains fields to update payment configuration.
type UpdatePaymentConfigRequest struct {
	Enabled             *bool    `json:"enabled"`
	MinAmount           *float64 `json:"min_amount"`
	MaxAmount           *float64 `json:"max_amount"`
	DailyLimit          *float64 `json:"daily_limit"`
	OrderTimeoutMin     *int     `json:"order_timeout_minutes"`
	MaxPendingOrders    *int     `json:"max_pending_orders"`
	EnabledTypes        []string `json:"enabled_payment_types"`
	BalanceDisabled     *bool    `json:"balance_disabled"`
	LoadBalanceStrategy *string  `json:"load_balance_strategy"`
	ProductNamePrefix   *string  `json:"product_name_prefix"`
	ProductNameSuffix   *string  `json:"product_name_suffix"`
	HelpImageURL        *string  `json:"help_image_url"`
	HelpText            *string  `json:"help_text"`

	// Cancel rate limit settings
	CancelRateLimitEnabled *bool   `json:"cancel_rate_limit_enabled"`
	CancelRateLimitMax     *int    `json:"cancel_rate_limit_max"`
	CancelRateLimitWindow  *int    `json:"cancel_rate_limit_window"`
	CancelRateLimitUnit    *string `json:"cancel_rate_limit_unit"`
	CancelRateLimitMode    *string `json:"cancel_rate_limit_window_mode"`
}

// MethodLimits holds per-payment-type limits.
type MethodLimits struct {
	PaymentType    string  `json:"payment_type"`
	FeeRate        float64 `json:"fee_rate"`
	DailyLimit     float64 `json:"daily_limit"`
	DailyUsed      float64 `json:"daily_used"`
	DailyRemaining float64 `json:"daily_remaining"`
	SingleMin      float64 `json:"single_min"`
	SingleMax      float64 `json:"single_max"`
	Available      bool    `json:"available"`
}

// MethodLimitsResponse is the full response for the user-facing /limits API.
// It includes per-method limits and the global widest range (union of all methods).
type MethodLimitsResponse struct {
	Methods   map[string]MethodLimits `json:"methods"`
	GlobalMin float64                 `json:"global_min"` // 0 = no minimum
	GlobalMax float64                 `json:"global_max"` // 0 = no maximum
}

type CreateProviderInstanceRequest struct {
	ProviderKey    string            `json:"provider_key"`
	Name           string            `json:"name"`
	Config         map[string]string `json:"config"`
	SupportedTypes []string          `json:"supported_types"`
	Enabled        bool              `json:"enabled"`
	PaymentMode    string            `json:"payment_mode"`
	SortOrder      int               `json:"sort_order"`
	Limits         string            `json:"limits"`
	RefundEnabled  bool              `json:"refund_enabled"`
}

type UpdateProviderInstanceRequest struct {
	Name           *string           `json:"name"`
	Config         map[string]string `json:"config"`
	SupportedTypes []string          `json:"supported_types"`
	Enabled        *bool             `json:"enabled"`
	PaymentMode    *string           `json:"payment_mode"`
	SortOrder      *int              `json:"sort_order"`
	Limits         *string           `json:"limits"`
	RefundEnabled  *bool             `json:"refund_enabled"`
}
type CreatePlanRequest struct {
	GroupID       int64    `json:"group_id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Price         float64  `json:"price"`
	OriginalPrice *float64 `json:"original_price"`
	ValidityDays  int      `json:"validity_days"`
	ValidityUnit  string   `json:"validity_unit"`
	Features      string   `json:"features"`
	ProductName   string   `json:"product_name"`
	ForSale       bool     `json:"for_sale"`
	SortOrder     int      `json:"sort_order"`
}

type UpdatePlanRequest struct {
	GroupID       *int64   `json:"group_id"`
	Name          *string  `json:"name"`
	Description   *string  `json:"description"`
	Price         *float64 `json:"price"`
	OriginalPrice *float64 `json:"original_price"`
	ValidityDays  *int     `json:"validity_days"`
	ValidityUnit  *string  `json:"validity_unit"`
	Features      *string  `json:"features"`
	ProductName   *string  `json:"product_name"`
	ForSale       *bool    `json:"for_sale"`
	SortOrder     *int     `json:"sort_order"`
}

// PaymentConfigService manages payment configuration and CRUD for
// provider instances, channels, and subscription plans.
type PaymentConfigService struct {
	entClient     *dbent.Client
	settingRepo   SettingRepository
	encryptionKey []byte
	updateMu      sync.Mutex
}

// NewPaymentConfigService creates a new PaymentConfigService.
func NewPaymentConfigService(entClient *dbent.Client, settingRepo SettingRepository, encryptionKey []byte) *PaymentConfigService {
	return &PaymentConfigService{entClient: entClient, settingRepo: settingRepo, encryptionKey: encryptionKey}
}

// BuildUpdateMap converts a patch-style payment config request into the exact
// raw setting keys that should be written, preserving omitted keys as-is.
func (s *PaymentConfigService) BuildUpdateMap(ctx context.Context, req UpdatePaymentConfigRequest) (map[string]string, error) {
	_ = ctx
	return buildPaymentUpdateMap(req), nil
}

// IsPaymentEnabled returns whether the payment system is enabled.
func (s *PaymentConfigService) IsPaymentEnabled(ctx context.Context) bool {
	val, err := s.settingRepo.GetValue(ctx, SettingPaymentEnabled)
	if err != nil {
		return false
	}
	return val == "true"
}

// GetPaymentConfig returns the full payment configuration.
func (s *PaymentConfigService) GetPaymentConfig(ctx context.Context) (*PaymentConfig, error) {
	keys := paymentConfigSettingKeys()
	vals, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get payment config settings: %w", err)
	}
	cfg := s.parsePaymentConfig(vals)
	// Load Stripe publishable key from the first enabled Stripe provider instance
	cfg.StripePublishableKey = s.getStripePublishableKey(ctx)
	return cfg, nil
}

func (s *PaymentConfigService) parsePaymentConfig(vals map[string]string) *PaymentConfig {
	cfg := &PaymentConfig{
		Enabled:             vals[SettingPaymentEnabled] == "true",
		MinAmount:           pcParseFloat(vals[SettingMinRechargeAmount], 1),
		MaxAmount:           pcParseFloat(vals[SettingMaxRechargeAmount], 0),
		DailyLimit:          pcParseFloat(vals[SettingDailyRechargeLimit], 0),
		OrderTimeoutMin:     pcParseInt(vals[SettingOrderTimeoutMinutes], defaultOrderTimeoutMin),
		MaxPendingOrders:    pcParseInt(vals[SettingMaxPendingOrders], defaultMaxPendingOrders),
		BalanceDisabled:     vals[SettingBalancePayDisabled] == "true",
		LoadBalanceStrategy: vals[SettingLoadBalanceStrategy],
		ProductNamePrefix:   vals[SettingProductNamePrefix],
		ProductNameSuffix:   vals[SettingProductNameSuffix],
		HelpImageURL:        vals[SettingHelpImageURL],
		HelpText:            vals[SettingHelpText],

		CancelRateLimitEnabled: vals[SettingCancelRateLimitOn] == "true",
		CancelRateLimitMax:     pcParseInt(vals[SettingCancelRateLimitMax], 10),
		CancelRateLimitWindow:  pcParseInt(vals[SettingCancelWindowSize], 1),
		CancelRateLimitUnit:    vals[SettingCancelWindowUnit],
		CancelRateLimitMode:    vals[SettingCancelWindowMode],
	}
	if cfg.LoadBalanceStrategy == "" {
		cfg.LoadBalanceStrategy = payment.DefaultLoadBalanceStrategy
	}
	if raw := vals[SettingEnabledPaymentTypes]; raw != "" {
		for _, t := range strings.Split(raw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				cfg.EnabledTypes = append(cfg.EnabledTypes, t)
			}
		}
	}
	cfg.EnabledTypes = normalizeEnabledPaymentTypes(cfg.EnabledTypes)
	return cfg
}

// getStripePublishableKey finds the publishable key from the first enabled Stripe provider instance.
func (s *PaymentConfigService) getStripePublishableKey(ctx context.Context) string {
	instances, err := s.entClient.PaymentProviderInstance.Query().
		Where(
			paymentproviderinstance.EnabledEQ(true),
			paymentproviderinstance.ProviderKeyEQ(payment.TypeStripe),
		).Limit(1).All(ctx)
	if err != nil || len(instances) == 0 {
		return ""
	}
	cfg, err := s.decryptConfig(instances[0].Config)
	if err != nil || cfg == nil {
		return ""
	}
	return cfg[payment.ConfigKeyPublishableKey]
}

// UpdatePaymentConfig updates the payment configuration settings.
// NOTE: This function exceeds 30 lines because each field requires an independent
// nil-check before serialisation — this is inherent to patch-style update patterns
// and cannot be meaningfully decomposed without introducing unnecessary abstraction.
func (s *PaymentConfigService) UpdatePaymentConfig(ctx context.Context, req UpdatePaymentConfigRequest) error {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()

	if s.entClient != nil {
		return s.updatePaymentConfigTx(ctx, req)
	}

	m, err := s.BuildUpdateMap(ctx, req)
	if err != nil {
		return err
	}
	return s.settingRepo.SetMultiple(ctx, m)
}

func (s *PaymentConfigService) updatePaymentConfigTx(ctx context.Context, req UpdatePaymentConfigRequest) error {
	updates := buildPaymentUpdateMap(req)
	if len(updates) == 0 {
		return nil
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin payment config transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	builders := make([]*dbent.SettingCreate, 0, len(updates))
	for key, value := range updates {
		builders = append(builders, tx.Setting.Create().
			SetKey(key).
			SetValue(value).
			SetUpdatedAt(now))
	}
	if len(builders) > 0 {
		if err := tx.Setting.CreateBulk(builders...).
			OnConflictColumns(setting.FieldKey).
			UpdateNewValues().
			Exec(ctx); err != nil {
			return fmt.Errorf("upsert payment settings: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit payment config transaction: %w", err)
	}
	return nil
}

func mergePaymentConfigPatch(current *PaymentConfig, req UpdatePaymentConfigRequest) *PaymentConfig {
	if current == nil {
		current = (&PaymentConfigService{}).parsePaymentConfig(map[string]string{})
	}
	merged := *current

	if req.Enabled != nil {
		merged.Enabled = *req.Enabled
	}
	if req.MinAmount != nil {
		merged.MinAmount = *req.MinAmount
	}
	if req.MaxAmount != nil {
		merged.MaxAmount = *req.MaxAmount
	}
	if req.DailyLimit != nil {
		merged.DailyLimit = *req.DailyLimit
	}
	if req.OrderTimeoutMin != nil {
		merged.OrderTimeoutMin = *req.OrderTimeoutMin
	}
	if req.MaxPendingOrders != nil {
		merged.MaxPendingOrders = *req.MaxPendingOrders
	}
	if req.EnabledTypes != nil {
		merged.EnabledTypes = normalizeEnabledPaymentTypes(req.EnabledTypes)
	}
	if req.BalanceDisabled != nil {
		merged.BalanceDisabled = *req.BalanceDisabled
	}
	if req.LoadBalanceStrategy != nil {
		merged.LoadBalanceStrategy = *req.LoadBalanceStrategy
	}
	if req.ProductNamePrefix != nil {
		merged.ProductNamePrefix = *req.ProductNamePrefix
	}
	if req.ProductNameSuffix != nil {
		merged.ProductNameSuffix = *req.ProductNameSuffix
	}
	if req.HelpImageURL != nil {
		merged.HelpImageURL = *req.HelpImageURL
	}
	if req.HelpText != nil {
		merged.HelpText = *req.HelpText
	}
	if req.CancelRateLimitEnabled != nil {
		merged.CancelRateLimitEnabled = *req.CancelRateLimitEnabled
	}
	if req.CancelRateLimitMax != nil {
		merged.CancelRateLimitMax = *req.CancelRateLimitMax
	}
	if req.CancelRateLimitWindow != nil {
		merged.CancelRateLimitWindow = *req.CancelRateLimitWindow
	}
	if req.CancelRateLimitUnit != nil {
		merged.CancelRateLimitUnit = *req.CancelRateLimitUnit
	}
	if req.CancelRateLimitMode != nil {
		merged.CancelRateLimitMode = *req.CancelRateLimitMode
	}

	return &merged
}

func serializePaymentConfig(cfg *PaymentConfig) map[string]string {
	if cfg == nil {
		cfg = (&PaymentConfigService{}).parsePaymentConfig(map[string]string{})
	}
	enabledTypes := normalizeEnabledPaymentTypes(cfg.EnabledTypes)
	return map[string]string{
		SettingPaymentEnabled:      strconv.FormatBool(cfg.Enabled),
		SettingMinRechargeAmount:   formatOptionalPositiveFloat(cfg.MinAmount),
		SettingMaxRechargeAmount:   formatOptionalPositiveFloat(cfg.MaxAmount),
		SettingDailyRechargeLimit:  formatOptionalPositiveFloat(cfg.DailyLimit),
		SettingOrderTimeoutMinutes: formatOptionalPositiveInt(cfg.OrderTimeoutMin),
		SettingMaxPendingOrders:    formatOptionalPositiveInt(cfg.MaxPendingOrders),
		SettingEnabledPaymentTypes: joinTypes(enabledTypes),
		SettingBalancePayDisabled:  strconv.FormatBool(cfg.BalanceDisabled),
		SettingLoadBalanceStrategy: nonEmptyOrDefault(cfg.LoadBalanceStrategy, payment.DefaultLoadBalanceStrategy),
		SettingProductNamePrefix:   cfg.ProductNamePrefix,
		SettingProductNameSuffix:   cfg.ProductNameSuffix,
		SettingHelpImageURL:        cfg.HelpImageURL,
		SettingHelpText:            cfg.HelpText,
		SettingCancelRateLimitOn:   strconv.FormatBool(cfg.CancelRateLimitEnabled),
		SettingCancelRateLimitMax:  formatOptionalPositiveInt(cfg.CancelRateLimitMax),
		SettingCancelWindowSize:    formatOptionalPositiveInt(cfg.CancelRateLimitWindow),
		SettingCancelWindowUnit:    cfg.CancelRateLimitUnit,
		SettingCancelWindowMode:    cfg.CancelRateLimitMode,
	}
}

func buildPaymentUpdateMap(req UpdatePaymentConfigRequest) map[string]string {
	updates := make(map[string]string)
	if req.Enabled != nil {
		updates[SettingPaymentEnabled] = strconv.FormatBool(*req.Enabled)
	}
	if req.MinAmount != nil {
		updates[SettingMinRechargeAmount] = formatPositiveFloat(req.MinAmount)
	}
	if req.MaxAmount != nil {
		updates[SettingMaxRechargeAmount] = formatPositiveFloat(req.MaxAmount)
	}
	if req.DailyLimit != nil {
		updates[SettingDailyRechargeLimit] = formatPositiveFloat(req.DailyLimit)
	}
	if req.OrderTimeoutMin != nil {
		updates[SettingOrderTimeoutMinutes] = formatPositiveInt(req.OrderTimeoutMin)
	}
	if req.MaxPendingOrders != nil {
		updates[SettingMaxPendingOrders] = formatPositiveInt(req.MaxPendingOrders)
	}
	if req.EnabledTypes != nil {
		updates[SettingEnabledPaymentTypes] = joinTypes(normalizeEnabledPaymentTypes(req.EnabledTypes))
	}
	if req.BalanceDisabled != nil {
		updates[SettingBalancePayDisabled] = strconv.FormatBool(*req.BalanceDisabled)
	}
	if req.LoadBalanceStrategy != nil {
		updates[SettingLoadBalanceStrategy] = nonEmptyOrDefault(*req.LoadBalanceStrategy, payment.DefaultLoadBalanceStrategy)
	}
	if req.ProductNamePrefix != nil {
		updates[SettingProductNamePrefix] = *req.ProductNamePrefix
	}
	if req.ProductNameSuffix != nil {
		updates[SettingProductNameSuffix] = *req.ProductNameSuffix
	}
	if req.HelpImageURL != nil {
		updates[SettingHelpImageURL] = *req.HelpImageURL
	}
	if req.HelpText != nil {
		updates[SettingHelpText] = *req.HelpText
	}
	if req.CancelRateLimitEnabled != nil {
		updates[SettingCancelRateLimitOn] = strconv.FormatBool(*req.CancelRateLimitEnabled)
	}
	if req.CancelRateLimitMax != nil {
		updates[SettingCancelRateLimitMax] = formatPositiveInt(req.CancelRateLimitMax)
	}
	if req.CancelRateLimitWindow != nil {
		updates[SettingCancelWindowSize] = formatPositiveInt(req.CancelRateLimitWindow)
	}
	if req.CancelRateLimitUnit != nil {
		updates[SettingCancelWindowUnit] = *req.CancelRateLimitUnit
	}
	if req.CancelRateLimitMode != nil {
		updates[SettingCancelWindowMode] = *req.CancelRateLimitMode
	}
	return updates
}

func formatOptionalPositiveFloat(v float64) string {
	if v <= 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func formatPositiveFloat(v *float64) string {
	if v == nil || *v <= 0 {
		return "" // empty → parsePaymentConfig uses default
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

func formatOptionalPositiveInt(v int) string {
	if v <= 0 {
		return ""
	}
	return strconv.Itoa(v)
}

func formatPositiveInt(v *int) string {
	if v == nil || *v <= 0 {
		return ""
	}
	return strconv.Itoa(*v)
}

func nonEmptyOrDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func splitTypes(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func joinTypes(types []string) string {
	return strings.Join(types, ",")
}

func normalizeEnabledPaymentType(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}
	return payment.GetBasePaymentType(trimmed)
}

func normalizeEnabledPaymentTypes(types []string) []string {
	if len(types) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(types))
	seen := make(map[string]struct{}, len(types))
	for _, raw := range types {
		t := normalizeEnabledPaymentType(raw)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		normalized = append(normalized, t)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func pcParseFloat(s string, defaultVal float64) float64 {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultVal
	}
	return v
}

func paymentConfigSettingKeys() []string {
	return []string{
		SettingPaymentEnabled, SettingMinRechargeAmount, SettingMaxRechargeAmount,
		SettingDailyRechargeLimit, SettingOrderTimeoutMinutes, SettingMaxPendingOrders,
		SettingEnabledPaymentTypes, SettingBalancePayDisabled, SettingLoadBalanceStrategy,
		SettingProductNamePrefix, SettingProductNameSuffix,
		SettingHelpImageURL, SettingHelpText,
		SettingCancelRateLimitOn, SettingCancelRateLimitMax,
		SettingCancelWindowSize, SettingCancelWindowUnit, SettingCancelWindowMode,
	}
}

func pcParseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
