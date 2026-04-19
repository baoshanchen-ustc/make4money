// Package payment provides the core payment provider abstraction,
// registry, load balancing, and shared utilities for the payment subsystem.
package payment

import (
	"context"
	"strings"
)

// PaymentType represents a supported payment method.
type PaymentType = string

// Supported payment type constants.
const (
	TypeAlipay       PaymentType = "alipay"
	TypeWxpay        PaymentType = "wxpay"
	TypeAlipayDirect PaymentType = "alipay_direct" // legacy alias, normalized to alipay on read/write
	TypeWxpayDirect  PaymentType = "wxpay_direct"  // legacy alias, normalized to wxpay on read/write
	TypeStripe       PaymentType = "stripe"
	TypeCard         PaymentType = "card"
	TypeLink         PaymentType = "link"
	TypeEasyPay      PaymentType = "easypay"
)

// Order status constants shared across payment and service layers.
const (
	OrderStatusPending           = "PENDING"
	OrderStatusPaid              = "PAID"
	OrderStatusRecharging        = "RECHARGING"
	OrderStatusCompleted         = "COMPLETED"
	OrderStatusExpired           = "EXPIRED"
	OrderStatusCancelled         = "CANCELLED"
	OrderStatusFailed            = "FAILED"
	OrderStatusRefundRequested   = "REFUND_REQUESTED"
	OrderStatusRefunding         = "REFUNDING"
	OrderStatusPartiallyRefunded = "PARTIALLY_REFUNDED"
	OrderStatusRefunded          = "REFUNDED"
	OrderStatusRefundFailed      = "REFUND_FAILED"
)

// Order types distinguish balance recharges from subscription purchases.
const (
	OrderTypeBalance      = "balance"
	OrderTypeSubscription = "subscription"
)

// Entity statuses shared across users, groups, etc.
const (
	EntityStatusActive = "active"
)

// Deduction types for refund flow.
const (
	DeductionTypeBalance      = "balance"
	DeductionTypeSubscription = "subscription"
	DeductionTypeNone         = "none"
)

// Payment notification status values.
const (
	NotificationStatusSuccess = "success"
	NotificationStatusPaid    = "paid"
)

// Provider-level status constants returned by provider implementations
// to the service layer (lowercase, distinct from OrderStatus uppercase constants).
const (
	ProviderStatusPending  = "pending"
	ProviderStatusPaid     = "paid"
	ProviderStatusSuccess  = "success"
	ProviderStatusFailed   = "failed"
	ProviderStatusRefunded = "refunded"
)

// DefaultLoadBalanceStrategy is the default load-balancing strategy
// used when no strategy is configured.
const DefaultLoadBalanceStrategy = "round-robin"

// ConfigKeyPublishableKey is the config map key for Stripe's publishable key.
const ConfigKeyPublishableKey = "publishableKey"

// GetBasePaymentType extracts the base payment method from a composite key.
// For example, "alipay_direct" -> "alipay".
func GetBasePaymentType(t string) string {
	return string(NormalizeVisiblePaymentType(t))
}

// NormalizeStoredPaymentType normalizes legacy aliases while preserving provider-specific sub-methods.
func NormalizeStoredPaymentType(t string) PaymentType {
	switch strings.TrimSpace(t) {
	case string(TypeAlipayDirect):
		return TypeAlipay
	case string(TypeWxpayDirect):
		return TypeWxpay
	default:
		return PaymentType(strings.TrimSpace(t))
	}
}

// NormalizeVisiblePaymentType normalizes legacy aliases and Stripe sub-methods into user-facing capabilities.
func NormalizeVisiblePaymentType(t string) PaymentType {
	switch NormalizeStoredPaymentType(t) {
	case TypeCard, TypeLink, TypeStripe:
		return TypeStripe
	case TypeAlipay:
		return TypeAlipay
	case TypeWxpay:
		return TypeWxpay
	default:
		return NormalizeStoredPaymentType(t)
	}
}

// IsVisiblePaymentType reports whether the type is a user-facing capability.
func IsVisiblePaymentType(t string) bool {
	switch NormalizeVisiblePaymentType(t) {
	case TypeAlipay, TypeWxpay, TypeStripe:
		return true
	default:
		return false
	}
}

// VisiblePaymentTypeFilterValues expands a user-facing payment type into the stored
// variants that should match it in queries.
func VisiblePaymentTypeFilterValues(filter string) []string {
	switch NormalizeVisiblePaymentType(filter) {
	case TypeAlipay:
		return []string{string(TypeAlipay), string(TypeAlipayDirect)}
	case TypeWxpay:
		return []string{string(TypeWxpay), string(TypeWxpayDirect)}
	case TypeStripe:
		return []string{string(TypeStripe), string(TypeCard), string(TypeLink)}
	default:
		normalized := strings.TrimSpace(string(NormalizeStoredPaymentType(filter)))
		if normalized == "" {
			return nil
		}
		return []string{normalized}
	}
}

// VisiblePaymentTypesForProvider returns the user-facing capabilities served by a provider instance.
func VisiblePaymentTypesForProvider(providerKey, supportedTypes string) []PaymentType {
	allowed := allowedVisibleTypesForProvider(providerKey)
	if len(allowed) == 0 {
		return nil
	}
	if providerKey == string(TypeStripe) {
		return []PaymentType{TypeStripe}
	}
	if strings.TrimSpace(supportedTypes) == "" {
		return append([]PaymentType(nil), allowed...)
	}

	allowedSet := make(map[PaymentType]struct{}, len(allowed))
	for _, t := range allowed {
		allowedSet[t] = struct{}{}
	}

	seen := make(map[PaymentType]struct{}, len(allowed))
	result := make([]PaymentType, 0, len(allowed))
	for _, raw := range strings.Split(supportedTypes, ",") {
		normalized := NormalizeVisiblePaymentType(raw)
		if _, ok := allowedSet[normalized]; !ok {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func allowedVisibleTypesForProvider(providerKey string) []PaymentType {
	switch providerKey {
	case string(TypeAlipay):
		return []PaymentType{TypeAlipay}
	case string(TypeWxpay):
		return []PaymentType{TypeWxpay}
	case string(TypeEasyPay):
		return []PaymentType{TypeAlipay, TypeWxpay}
	case string(TypeStripe):
		return []PaymentType{TypeStripe}
	default:
		return nil
	}
}

// CreatePaymentRequest holds the parameters for creating a new payment.
type CreatePaymentRequest struct {
	OrderID            string // Internal order ID
	Amount             string // Pay amount in CNY (formatted to 2 decimal places)
	PaymentType        string // e.g. "alipay", "wxpay", "stripe"
	Subject            string // Product description
	NotifyURL          string // Webhook callback URL
	ReturnURL          string // Browser redirect URL after payment
	OpenID             string // WeChat JSAPI payer OpenID when available
	ClientIP           string // Payer's IP address
	IsMobile           bool   // Whether the request comes from a mobile device
	InstanceSubMethods string // Comma-separated sub-methods from instance supported_types (for Stripe)
}

// CreatePaymentResultType describes the shape of the create-payment result.
type CreatePaymentResultType = string

const (
	CreatePaymentResultOrderCreated  CreatePaymentResultType = "order_created"
	CreatePaymentResultOAuthRequired CreatePaymentResultType = "oauth_required"
	CreatePaymentResultJSAPIReady    CreatePaymentResultType = "jsapi_ready"
)

// WechatOAuthInfo describes the next step when WeChat OAuth is required before payment.
type WechatOAuthInfo struct {
	AuthorizeURL string `json:"authorize_url,omitempty"`
	AppID        string `json:"appid,omitempty"`
	OpenID       string `json:"openid,omitempty"`
	Scope        string `json:"scope,omitempty"`
	State        string `json:"state,omitempty"`
	RedirectURL  string `json:"redirect_url,omitempty"`
}

// WechatJSAPIPayload contains the fields the frontend needs to invoke WeChat JSAPI payment.
type WechatJSAPIPayload struct {
	AppID     string `json:"appId,omitempty"`
	TimeStamp string `json:"timeStamp,omitempty"`
	NonceStr  string `json:"nonceStr,omitempty"`
	Package   string `json:"package,omitempty"`
	SignType  string `json:"signType,omitempty"`
	PaySign   string `json:"paySign,omitempty"`
}

// CreatePaymentResponse is returned after successfully initiating a payment.
type CreatePaymentResponse struct {
	TradeNo      string                  // Third-party transaction ID
	PayURL       string                  // H5 payment URL (alipay/wxpay)
	QRCode       string                  // QR code content for scanning
	ClientSecret string                  // Stripe PaymentIntent client secret
	ResultType   CreatePaymentResultType // Typed result contract for frontend flows
	OAuth        *WechatOAuthInfo        // WeChat OAuth bootstrap payload when required
	JSAPI        *WechatJSAPIPayload     // WeChat JSAPI invocation payload when ready
}

// QueryOrderResponse describes the payment status from the upstream provider.
type QueryOrderResponse struct {
	TradeNo string
	Status  string  // "pending", "paid", "failed", "refunded"
	Amount  float64 // Amount in CNY
	PaidAt  string  // RFC3339 timestamp or empty
}

// PaymentNotification is the parsed result of a webhook/notify callback.
type PaymentNotification struct {
	TradeNo string
	OrderID string
	Amount  float64
	Status  string // "success" or "failed"
	RawData string // Raw notification body for audit
}

// RefundRequest contains the parameters for requesting a refund.
type RefundRequest struct {
	TradeNo string
	OrderID string
	Amount  string // Refund amount formatted to 2 decimal places
	Reason  string
}

// RefundResponse is returned after a refund request.
type RefundResponse struct {
	RefundID string
	Status   string // "success", "pending", "failed"
}

// InstanceSelection holds the selected provider instance and its decrypted config.
type InstanceSelection struct {
	InstanceID     string
	ProviderKey    string // Provider key of the selected instance (e.g. "alipay", "easypay")
	Config         map[string]string
	SupportedTypes string // Comma-separated list of supported payment types from the instance
	PaymentMode    string // Payment display mode: "qrcode", "redirect", "popup"
}

// Provider defines the interface that all payment providers must implement.
type Provider interface {
	// Name returns a human-readable name for this provider.
	Name() string
	// ProviderKey returns the unique key identifying this provider type (e.g. "easypay").
	ProviderKey() string
	// SupportedTypes returns the list of payment types this provider handles.
	SupportedTypes() []PaymentType
	// CreatePayment initiates a payment and returns the upstream response.
	CreatePayment(ctx context.Context, req CreatePaymentRequest) (*CreatePaymentResponse, error)
	// QueryOrder queries the payment status of the given trade number.
	QueryOrder(ctx context.Context, tradeNo string) (*QueryOrderResponse, error)
	// VerifyNotification parses and verifies a webhook callback.
	// Returns nil for unrecognized or irrelevant events (caller should return 200).
	VerifyNotification(ctx context.Context, rawBody string, headers map[string]string) (*PaymentNotification, error)
	// Refund requests a refund from the upstream provider.
	Refund(ctx context.Context, req RefundRequest) (*RefundResponse, error)
}

// CancelableProvider extends Provider with the ability to cancel pending payments.
type CancelableProvider interface {
	Provider
	// CancelPayment cancels/expires a pending payment on the upstream platform.
	CancelPayment(ctx context.Context, tradeNo string) error
}
