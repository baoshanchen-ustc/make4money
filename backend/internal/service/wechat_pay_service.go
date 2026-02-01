package service

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/jsapi"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// WeChatPayService 微信支付服务
type WeChatPayService struct {
	cfg            *config.Config
	client         *core.Client
	privateKey     *rsa.PrivateKey
	notifyHandler  *notify.Handler
	certDownloader *downloader.CertificateDownloader
	mu             sync.RWMutex
	initialized    bool
}

// NewWeChatPayService 创建微信支付服务
func NewWeChatPayService(cfg *config.Config) *WeChatPayService {
	svc := &WeChatPayService{
		cfg: cfg,
	}

	if cfg.WeChatPay.Enabled {
		if err := svc.initClient(); err != nil {
			log.Printf("[WeChatPay] Failed to initialize client: %v", err)
		}
	}

	return svc
}

// initClient 初始化微信支付客户端
func (s *WeChatPayService) initClient() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	privateKey, err := utils.LoadPrivateKeyWithPath(s.cfg.WeChatPay.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("load private key failed: %w", err)
	}
	s.privateKey = privateKey

	ctx := context.Background()

	// 创建微信支付客户端（自动下载和管理平台证书）
	client, err := core.NewClient(
		ctx,
		option.WithWechatPayAutoAuthCipher(
			s.cfg.WeChatPay.MchID,
			s.cfg.WeChatPay.CertSerialNo,
			privateKey,
			s.cfg.WeChatPay.APIv3Key,
		),
	)
	if err != nil {
		return fmt.Errorf("create wechat pay client failed: %w", err)
	}
	s.client = client

	// 初始化证书下载器（用于回调验签）
	certDownloader, err := downloader.NewCertificateDownloaderWithClient(
		ctx,
		client,
		s.cfg.WeChatPay.MchID,
	)
	if err != nil {
		return fmt.Errorf("create certificate downloader failed: %w", err)
	}
	s.certDownloader = certDownloader

	// 创建回调通知处理器
	s.notifyHandler = notify.NewNotifyHandler(
		s.cfg.WeChatPay.APIv3Key,
		verifiers.NewSHA256WithRSAVerifier(certDownloader),
	)

	s.initialized = true
	log.Printf("[WeChatPay] Client initialized successfully (with notify handler)")
	return nil
}

// IsEnabled 检查微信支付是否启用且已初始化
func (s *WeChatPayService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.WeChatPay.Enabled && s.initialized
}

// GetClient 获取微信支付客户端（线程安全）
func (s *WeChatPayService) GetClient() (*core.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return nil, fmt.Errorf("wechat pay client not initialized")
	}
	return s.client, nil
}

// GetPrivateKey 获取商户私钥（用于JSAPI签名）
func (s *WeChatPayService) GetPrivateKey() (*rsa.PrivateKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.privateKey == nil {
		return nil, fmt.Errorf("private key not loaded")
	}
	return s.privateKey, nil
}

// GetConfig 获取微信支付非敏感配置（只读）
// 仅返回可安全暴露的字段，APIv3Key/PrivateKeyPath 等敏感信息不返回
func (s *WeChatPayService) GetConfig() config.WeChatPayConfig {
	return config.WeChatPayConfig{
		Enabled:   s.cfg.WeChatPay.Enabled,
		AppID:     s.cfg.WeChatPay.AppID,
		MchID:     s.cfg.WeChatPay.MchID,
		NotifyURL: s.cfg.WeChatPay.NotifyURL,
	}
}

// RechargeConfig 充值配置（公开）
type RechargeConfig struct {
	MinAmount      float64   // 最小充值金额
	MaxAmount      float64   // 最大充值金额
	DefaultAmounts []float64 // 默认金额选项
}

// GetRechargeConfig 获取充值配置
// 后续 Story 1.3 实现动态配置后可从数据库加载
func (s *WeChatPayService) GetRechargeConfig() RechargeConfig {
	return RechargeConfig{
		MinAmount:      1.0,
		MaxAmount:      1000.0,
		DefaultAmounts: []float64{10, 50, 100, 200, 500},
	}
}

// ValidateRechargeAmount 验证充值金额是否在允许范围内
// 返回 nil 表示验证通过，否则返回错误信息
func (s *WeChatPayService) ValidateRechargeAmount(amount float64) error {
	cfg := s.GetRechargeConfig()

	if amount < cfg.MinAmount {
		return fmt.Errorf("充值金额不能小于 %.2f 元", cfg.MinAmount)
	}
	if amount > cfg.MaxAmount {
		return fmt.Errorf("充值金额不能大于 %.2f 元", cfg.MaxAmount)
	}
	return nil
}

// PaymentChannel 支付渠道类型
type PaymentChannel string

const (
	WeChatPayChannelNative PaymentChannel = "native" // 扫码支付
	WeChatPayChannelJSAPI  PaymentChannel = "jsapi"  // 公众号/小程序支付
)

// WeChatPayRequest 微信支付请求参数
type WeChatPayRequest struct {
	AppID       string         // 应用ID
	MchID       string         // 商户号
	OutTradeNo  string         // 商户订单号
	AmountInFen int64          // 金额（分）
	Description string         // 商品描述
	NotifyURL   string         // 回调地址
	Channel     PaymentChannel // 支付渠道
	OpenID      string         // 用户OpenID（JSAPI必填）
	ExpireAt    time.Time      // 过期时间
}

// WeChatPayResult 微信支付下单结果
type WeChatPayResult struct {
	PrepayID  string // 预支付交易会话标识
	QRCodeURL string // 二维码链接（Native支付）
}

// CreateWeChatPayOrderRequest 创建微信支付订单请求
type CreateWeChatPayOrderRequest struct {
	OrderNo     string  // 商户订单号
	Amount      float64 // 金额（元）
	Description string  // 商品描述
	Channel     string  // 支付渠道：native/jsapi
	OpenID      string  // 用户OpenID（JSAPI必填）
}

// CreateWeChatPayOrderResult 创建微信支付订单结果
type CreateWeChatPayOrderResult struct {
	PrepayID  string // 预支付交易会话标识
	QRCodeURL string // 二维码链接（Native支付）
	// JSAPI 支付参数（仅 JSAPI 渠道返回，用于前端调起支付）
	JSAPIParams *JSAPIPaymentParams `json:"jsapi_params,omitempty"`
}

// JSAPIPaymentParams JSAPI 支付调起参数
// 前端使用这些参数调用 WeixinJSBridge.invoke('getBrandWCPayRequest', ...) 或 wx.chooseWXPay()
type JSAPIPaymentParams struct {
	AppID     string `json:"appId"`     // 公众号/小程序 AppID
	TimeStamp string `json:"timeStamp"` // 时间戳（秒级）
	NonceStr  string `json:"nonceStr"`  // 随机字符串
	Package   string `json:"package"`   // 订单详情扩展字符串，格式为 prepay_id=xxx
	SignType  string `json:"signType"`  // 签名类型，固定为 RSA
	PaySign   string `json:"paySign"`   // 签名值
}

// AmountToFen 将金额从元转换为分
// 使用 math.Round 避免浮点数精度问题
func AmountToFen(yuan float64) int64 {
	return int64(math.Round(yuan * 100))
}

// maskOpenID 脱敏 OpenID（只显示前6位和后4位）
func maskOpenID(openID string) string {
	if len(openID) <= 10 {
		return openID[:len(openID)/2] + "..."
	}
	return openID[:6] + "..." + openID[len(openID)-4:]
}

// buildPaymentRequest 构建支付请求参数
func (s *WeChatPayService) buildPaymentRequest(orderNo string, amount float64, description, channel, openID string) *WeChatPayRequest {
	expireMinutes := s.cfg.WeChatPay.OrderExpireMinutes
	if expireMinutes <= 0 {
		expireMinutes = 30
	}

	ch := WeChatPayChannelNative
	if channel == "jsapi" {
		ch = WeChatPayChannelJSAPI
	}

	return &WeChatPayRequest{
		AppID:       s.cfg.WeChatPay.AppID,
		MchID:       s.cfg.WeChatPay.MchID,
		OutTradeNo:  orderNo,
		AmountInFen: AmountToFen(amount),
		Description: description,
		NotifyURL:   s.cfg.WeChatPay.NotifyURL,
		Channel:     ch,
		OpenID:      openID,
		ExpireAt:    time.Now().Add(time.Duration(expireMinutes) * time.Minute),
	}
}

// CreateOrder 创建微信支付订单
// 支持 Native（扫码支付）和 JSAPI（公众号支付）两种方式
// 超时时间设置为 30 秒
func (s *WeChatPayService) CreateOrder(ctx context.Context, req *CreateWeChatPayOrderRequest) (*CreateWeChatPayOrderResult, error) {
	// 检查是否已初始化
	if !s.IsEnabled() {
		return nil, fmt.Errorf("wechat pay is not enabled or not initialized")
	}

	// 获取客户端
	client, err := s.GetClient()
	if err != nil {
		log.Printf("[WeChatPay] Failed to get client: %v", err)
		return nil, fmt.Errorf("get wechat pay client: %w", err)
	}

	// 构建请求参数
	payReq := s.buildPaymentRequest(req.OrderNo, req.Amount, req.Description, req.Channel, req.OpenID)

	// 根据支付渠道调用不同的 API
	switch payReq.Channel {
	case WeChatPayChannelNative:
		return s.createNativeOrder(ctx, client, payReq)
	case WeChatPayChannelJSAPI:
		return s.createJSAPIOrder(ctx, client, payReq)
	default:
		return nil, fmt.Errorf("unsupported payment channel: %s", payReq.Channel)
	}
}

// createNativeOrder 创建 Native 扫码支付订单
func (s *WeChatPayService) createNativeOrder(ctx context.Context, client *core.Client, req *WeChatPayRequest) (*CreateWeChatPayOrderResult, error) {
	svc := native.NativeApiService{Client: client}

	// 构建 Native 支付请求
	nativeReq := native.PrepayRequest{
		Appid:       core.String(req.AppID),
		Mchid:       core.String(req.MchID),
		Description: core.String(req.Description),
		OutTradeNo:  core.String(req.OutTradeNo),
		TimeExpire:  core.Time(req.ExpireAt),
		NotifyUrl:   core.String(req.NotifyURL),
		Amount: &native.Amount{
			Total:    core.Int64(req.AmountInFen),
			Currency: core.String("CNY"),
		},
	}

	log.Printf("[WeChatPay] Creating Native order: order_no=%s, amount=%d fen, expire_at=%s",
		req.OutTradeNo, req.AmountInFen, req.ExpireAt.Format(time.RFC3339))

	// 调用微信支付 API
	resp, result, err := svc.Prepay(ctx, nativeReq)
	if err != nil {
		log.Printf("[WeChatPay] Native prepay failed: order_no=%s, error=%v", req.OutTradeNo, err)
		return nil, fmt.Errorf("native prepay: %w", err)
	}

	// 检查 HTTP 状态码
	if result.Response.StatusCode != 200 {
		log.Printf("[WeChatPay] Native prepay returned non-200: order_no=%s, status=%d",
			req.OutTradeNo, result.Response.StatusCode)
		return nil, fmt.Errorf("native prepay returned status %d", result.Response.StatusCode)
	}

	if resp.CodeUrl == nil {
		log.Printf("[WeChatPay] Native prepay returned nil code_url: order_no=%s", req.OutTradeNo)
		return nil, fmt.Errorf("native prepay returned nil code_url")
	}

	log.Printf("[WeChatPay] Native order created successfully: order_no=%s", req.OutTradeNo)

	return &CreateWeChatPayOrderResult{
		QRCodeURL: *resp.CodeUrl,
	}, nil
}

// createJSAPIOrder 创建 JSAPI 公众号支付订单
// 使用 PrepayWithRequestPayment 方法直接返回前端调起支付所需的所有参数
func (s *WeChatPayService) createJSAPIOrder(ctx context.Context, client *core.Client, req *WeChatPayRequest) (*CreateWeChatPayOrderResult, error) {
	// JSAPI 支付需要 OpenID
	if req.OpenID == "" {
		return nil, fmt.Errorf("openid is required for JSAPI payment")
	}

	svc := jsapi.JsapiApiService{Client: client}

	// 构建 JSAPI 支付请求
	jsapiReq := jsapi.PrepayRequest{
		Appid:       core.String(req.AppID),
		Mchid:       core.String(req.MchID),
		Description: core.String(req.Description),
		OutTradeNo:  core.String(req.OutTradeNo),
		TimeExpire:  core.Time(req.ExpireAt),
		NotifyUrl:   core.String(req.NotifyURL),
		Amount: &jsapi.Amount{
			Total:    core.Int64(req.AmountInFen),
			Currency: core.String("CNY"),
		},
		Payer: &jsapi.Payer{
			Openid: core.String(req.OpenID),
		},
	}

	log.Printf("[WeChatPay] Creating JSAPI order: order_no=%s, amount=%d fen, openid=%s..., expire_at=%s",
		req.OutTradeNo, req.AmountInFen, maskOpenID(req.OpenID), req.ExpireAt.Format(time.RFC3339))

	// 使用 PrepayWithRequestPayment 直接获取前端调起支付所需的签名参数
	resp, result, err := svc.PrepayWithRequestPayment(ctx, jsapiReq)
	if err != nil {
		log.Printf("[WeChatPay] JSAPI prepay failed: order_no=%s, error=%v", req.OutTradeNo, err)
		return nil, fmt.Errorf("jsapi prepay: %w", err)
	}

	// 检查 HTTP 状态码
	if result.Response.StatusCode != 200 {
		log.Printf("[WeChatPay] JSAPI prepay returned non-200: order_no=%s, status=%d",
			req.OutTradeNo, result.Response.StatusCode)
		return nil, fmt.Errorf("jsapi prepay returned status %d", result.Response.StatusCode)
	}

	if resp.PrepayId == nil {
		log.Printf("[WeChatPay] JSAPI prepay returned nil prepay_id: order_no=%s", req.OutTradeNo)
		return nil, fmt.Errorf("jsapi prepay returned nil prepay_id")
	}

	log.Printf("[WeChatPay] JSAPI order created successfully: order_no=%s, prepay_id=%s",
		req.OutTradeNo, *resp.PrepayId)

	return &CreateWeChatPayOrderResult{
		PrepayID: *resp.PrepayId,
		JSAPIParams: &JSAPIPaymentParams{
			AppID:     safeString(resp.Appid),
			TimeStamp: safeString(resp.TimeStamp),
			NonceStr:  safeString(resp.NonceStr),
			Package:   safeString(resp.Package),
			SignType:  safeString(resp.SignType),
			PaySign:   safeString(resp.PaySign),
		},
	}, nil
}

// NotifyMaxTimestampAgeSeconds 回调请求时间戳最大允许偏差（5分钟，防重放攻击）
const NotifyMaxTimestampAgeSeconds = 300

// PaymentNotifyResult 支付回调解析结果
type PaymentNotifyResult struct {
	Transaction   *payments.Transaction // 解密后的交易数据
	SignatureErr  error                 // 签名验证错误（nil表示验证通过）
	TimestampErr  error                 // 时间戳验证错误（nil表示验证通过）
	DecryptionErr error                 // 解密错误（nil表示解密成功）
}

// IsValid 检查回调是否完全有效（签名+时间戳+解密都通过）
func (r *PaymentNotifyResult) IsValid() bool {
	return r.SignatureErr == nil && r.TimestampErr == nil && r.DecryptionErr == nil && r.Transaction != nil
}

// Error 返回第一个错误
func (r *PaymentNotifyResult) Error() error {
	if r.SignatureErr != nil {
		return r.SignatureErr
	}
	if r.TimestampErr != nil {
		return r.TimestampErr
	}
	if r.DecryptionErr != nil {
		return r.DecryptionErr
	}
	return nil
}

// ValidateTimestamp 验证回调请求的时间戳是否在允许范围内（5分钟）
// 防止重放攻击
func ValidateTimestamp(timestampStr string) error {
	if timestampStr == "" {
		return fmt.Errorf("timestamp is empty")
	}

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}

	now := time.Now().Unix()
	diff := now - timestamp
	if diff < 0 {
		diff = -diff
	}

	if diff > NotifyMaxTimestampAgeSeconds {
		return fmt.Errorf("timestamp expired: diff=%d seconds (max=%d)", diff, NotifyMaxTimestampAgeSeconds)
	}

	return nil
}

// ParsePaymentNotify 解析并验证支付回调通知
// 返回解析结果，包含签名验证、时间戳验证、解密结果
// 调用方可以根据结果决定如何响应微信支付
func (s *WeChatPayService) ParsePaymentNotify(ctx context.Context, request *http.Request) *PaymentNotifyResult {
	result := &PaymentNotifyResult{}

	// 检查服务是否已初始化
	if !s.IsEnabled() {
		result.SignatureErr = fmt.Errorf("wechat pay service not initialized")
		return result
	}

	// 1. 验证时间戳（防重放攻击）
	timestamp := request.Header.Get("Wechatpay-Timestamp")
	if err := ValidateTimestamp(timestamp); err != nil {
		result.TimestampErr = err
		log.Printf("[WeChatPay] Notify timestamp validation failed: %v", err)
		// 时间戳验证失败仍继续处理签名，以便记录完整信息
	}

	// 2. 使用 SDK 验签并解密
	s.mu.RLock()
	handler := s.notifyHandler
	s.mu.RUnlock()

	if handler == nil {
		result.SignatureErr = fmt.Errorf("notify handler not initialized")
		return result
	}

	transaction := &payments.Transaction{}
	notifyReq, err := handler.ParseNotifyRequest(ctx, request, transaction)
	if err != nil {
		// SDK 的 ParseNotifyRequest 会同时验证签名和解密
		// 根据错误类型区分是签名错误还是解密错误
		result.SignatureErr = fmt.Errorf("signature verification or decryption failed: %w", err)
		log.Printf("[WeChatPay] Notify parse failed: %v", err)
		return result
	}

	// 3. 验证解析结果
	if notifyReq == nil {
		result.DecryptionErr = fmt.Errorf("parsed notify request is nil")
		return result
	}

	// 4. 设置解密后的交易数据
	result.Transaction = transaction
	log.Printf("[WeChatPay] Notify parsed successfully: out_trade_no=%s, transaction_id=%s, trade_state=%s",
		safeString(transaction.OutTradeNo),
		safeString(transaction.TransactionId),
		safeString(transaction.TradeState))

	return result
}

// GetAPIv3Key 获取 APIv3 密钥（仅内部使用，用于测试）
func (s *WeChatPayService) GetAPIv3Key() string {
	return s.cfg.WeChatPay.APIv3Key
}

// safeString 安全获取字符串指针的值
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// safeTimeString 安全获取时间指针的字符串值
func safeTimeString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// safeFundsAccountString 安全获取资金账户类型的字符串值
func safeFundsAccountString(f *refunddomestic.FundsAccount) string {
	if f == nil {
		return ""
	}
	return string(*f)
}

// WeChatQueryOrderResult 微信订单查询结果
type WeChatQueryOrderResult struct {
	TradeState     string // SUCCESS, REFUND, NOTPAY, CLOSED, PAYERROR, USERPAYING
	TransactionID  string // 微信支付订单号
	TradeStateDesc string // 状态描述
}

// QueryOrder 查询微信支付订单状态
// 使用商户订单号查询订单在微信侧的真实状态
func (s *WeChatPayService) QueryOrder(ctx context.Context, orderNo string) (*WeChatQueryOrderResult, error) {
	// 检查是否已初始化
	if !s.IsEnabled() {
		return nil, fmt.Errorf("wechat pay is not enabled or not initialized")
	}

	// 获取客户端
	client, err := s.GetClient()
	if err != nil {
		log.Printf("[WeChatPay] Failed to get client for query order: %v", err)
		return nil, fmt.Errorf("get wechat pay client: %w", err)
	}

	svc := native.NativeApiService{Client: client}

	// 使用商户订单号查询
	resp, result, err := svc.QueryOrderByOutTradeNo(ctx, native.QueryOrderByOutTradeNoRequest{
		OutTradeNo: core.String(orderNo),
		Mchid:      core.String(s.cfg.WeChatPay.MchID),
	})

	if err != nil {
		log.Printf("[WeChatPay] Query order failed: order_no=%s, error=%v", orderNo, err)
		return nil, fmt.Errorf("query order: %w", err)
	}

	// 检查 HTTP 状态码
	if result.Response.StatusCode != 200 {
		log.Printf("[WeChatPay] Query order returned non-200: order_no=%s, status=%d",
			orderNo, result.Response.StatusCode)
		return nil, fmt.Errorf("query order returned status %d", result.Response.StatusCode)
	}

	if resp.TradeState == nil {
		log.Printf("[WeChatPay] Query order returned nil trade_state: order_no=%s", orderNo)
		return nil, fmt.Errorf("query order returned nil trade_state")
	}

	log.Printf("[WeChatPay] Query order result: order_no=%s, trade_state=%s, transaction_id=%s",
		orderNo, *resp.TradeState, safeString(resp.TransactionId))

	return &WeChatQueryOrderResult{
		TradeState:     *resp.TradeState,
		TransactionID:  safeString(resp.TransactionId),
		TradeStateDesc: safeString(resp.TradeStateDesc),
	}, nil
}

// CloseOrder 关闭微信支付订单
// 调用微信支付 API 关闭订单，防止已过期订单仍被支付
// 返回 nil 表示关闭成功，返回 error 表示关闭失败
func (s *WeChatPayService) CloseOrder(ctx context.Context, orderNo string) error {
	// 检查是否已初始化
	if !s.IsEnabled() {
		return fmt.Errorf("wechat pay is not enabled or not initialized")
	}

	// 获取客户端
	client, err := s.GetClient()
	if err != nil {
		log.Printf("[WeChatPay] Failed to get client for close order: %v", err)
		return fmt.Errorf("get wechat pay client: %w", err)
	}

	svc := native.NativeApiService{Client: client}

	closeReq := native.CloseOrderRequest{
		OutTradeNo: core.String(orderNo),
		Mchid:      core.String(s.cfg.WeChatPay.MchID),
	}

	log.Printf("[WeChatPay] Closing order: order_no=%s", orderNo)

	result, err := svc.CloseOrder(ctx, closeReq)
	if err != nil {
		log.Printf("[WeChatPay] Close order failed: order_no=%s, error=%v", orderNo, err)
		return fmt.Errorf("close order: %w", err)
	}

	// 检查 HTTP 状态码（204 表示成功）
	if result.Response.StatusCode != 204 && result.Response.StatusCode != 200 {
		log.Printf("[WeChatPay] Close order returned unexpected status: order_no=%s, status=%d",
			orderNo, result.Response.StatusCode)
		return fmt.Errorf("close order returned status %d", result.Response.StatusCode)
	}

	log.Printf("[WeChatPay] Order closed successfully: order_no=%s", orderNo)
	return nil
}

// RefundParams 退款请求参数
type RefundParams struct {
	OrderNo       string  // 原订单号
	RefundNo      string  // 退款单号
	Amount        float64 // 退款金额（元）
	TotalAmount   float64 // 原订单金额（元）
	Reason        string  // 退款原因
	TransactionID string  // 微信支付订单号（可选，优先使用）
}

// RefundResult 退款结果
type RefundResult struct {
	RefundID            string // 微信退款单号
	RefundNo            string // 商户退款单号
	Status              string // 退款状态: SUCCESS, CLOSED, PROCESSING, ABNORMAL
	Amount              int64  // 退款金额（分）
	SuccessTime         string // 退款成功时间
	FundsAccount        string // 资金账户
	UserReceivedAccount string // 用户收款账户
}

// Refund 申请退款
// 调用微信支付退款 API
func (s *WeChatPayService) Refund(ctx context.Context, params RefundParams) (*RefundResult, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("wechat pay is not enabled")
	}

	client, err := s.GetClient()
	if err != nil {
		return nil, fmt.Errorf("get wechat pay client failed: %w", err)
	}

	// 创建退款服务
	svc := refunddomestic.RefundsApiService{Client: client}

	// 金额转换为分
	refundAmountFen := AmountToFen(params.Amount)
	totalAmountFen := AmountToFen(params.TotalAmount)

	// 构建退款请求
	req := refunddomestic.CreateRequest{
		OutTradeNo:  core.String(params.OrderNo),
		OutRefundNo: core.String(params.RefundNo),
		Reason:      core.String(params.Reason),
		Amount: &refunddomestic.AmountReq{
			Refund:   core.Int64(refundAmountFen),
			Total:    core.Int64(totalAmountFen),
			Currency: core.String("CNY"),
		},
	}

	// 如果配置了退款回调地址，设置 NotifyUrl
	if s.cfg.WeChatPay.RefundNotifyURL != "" {
		req.NotifyUrl = core.String(s.cfg.WeChatPay.RefundNotifyURL)
	} else if s.cfg.WeChatPay.NotifyURL != "" {
		// 复用支付回调地址
		req.NotifyUrl = core.String(s.cfg.WeChatPay.NotifyURL)
	}

	// 如果有微信订单号，优先使用（更精确）
	if params.TransactionID != "" {
		req.TransactionId = core.String(params.TransactionID)
		req.OutTradeNo = nil
	}

	log.Printf("[WeChatPay] Refund request: order_no=%s, refund_no=%s, amount=%d fen, reason=%s",
		params.OrderNo, params.RefundNo, refundAmountFen, params.Reason)

	// 调用退款接口
	resp, result, err := svc.Create(ctx, req)
	if err != nil {
		statusCode := 0
		if result != nil && result.Response != nil {
			statusCode = result.Response.StatusCode
		}
		log.Printf("[WeChatPay] Refund API failed: order_no=%s, refund_no=%s, error=%v, http_status=%d",
			params.OrderNo, params.RefundNo, err, statusCode)
		return nil, fmt.Errorf("wechat refund api failed: %w", err)
	}

	log.Printf("[WeChatPay] Refund response: refund_id=%s, status=%s, success_time=%s",
		safeString(resp.RefundId), string(*resp.Status), safeTimeString(resp.SuccessTime))

	return &RefundResult{
		RefundID:            safeString(resp.RefundId),
		RefundNo:            safeString(resp.OutRefundNo),
		Status:              string(*resp.Status),
		Amount:              *resp.Amount.Refund,
		SuccessTime:         safeTimeString(resp.SuccessTime),
		FundsAccount:        safeFundsAccountString(resp.FundsAccount),
		UserReceivedAccount: safeString(resp.UserReceivedAccount),
	}, nil
}

// QueryRefund 查询退款状态
func (s *WeChatPayService) QueryRefund(ctx context.Context, refundNo string) (*RefundResult, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("wechat pay is not enabled")
	}

	client, err := s.GetClient()
	if err != nil {
		return nil, fmt.Errorf("get wechat pay client failed: %w", err)
	}

	svc := refunddomestic.RefundsApiService{Client: client}

	log.Printf("[WeChatPay] Query refund: refund_no=%s", refundNo)

	resp, result, err := svc.QueryByOutRefundNo(ctx, refunddomestic.QueryByOutRefundNoRequest{
		OutRefundNo: core.String(refundNo),
	})

	if err != nil {
		statusCode := 0
		if result != nil && result.Response != nil {
			statusCode = result.Response.StatusCode
		}
		log.Printf("[WeChatPay] Query refund API failed: refund_no=%s, error=%v, http_status=%d",
			refundNo, err, statusCode)
		return nil, fmt.Errorf("wechat query refund failed: %w", err)
	}

	log.Printf("[WeChatPay] Query refund response: refund_id=%s, status=%s",
		safeString(resp.RefundId), string(*resp.Status))

	return &RefundResult{
		RefundID:            safeString(resp.RefundId),
		RefundNo:            safeString(resp.OutRefundNo),
		Status:              string(*resp.Status),
		Amount:              *resp.Amount.Refund,
		SuccessTime:         safeTimeString(resp.SuccessTime),
		FundsAccount:        safeFundsAccountString(resp.FundsAccount),
		UserReceivedAccount: safeString(resp.UserReceivedAccount),
	}, nil
}

// RefundNotification 退款回调通知数据
type RefundNotification struct {
	OutTradeNo          string // 商户订单号
	OutRefundNo         string // 商户退款单号
	TransactionID       string // 微信支付订单号
	RefundID            string // 微信退款单号
	RefundStatus        string // 退款状态: SUCCESS, CLOSED, ABNORMAL
	SuccessTime         string // 退款成功时间
	Amount              int64  // 退款金额（分）
	UserReceivedAccount string // 用户收款账户
}

// ParseRefundNotification 解析退款回调通知
// 验证签名并解密回调数据
func (s *WeChatPayService) ParseRefundNotification(ctx context.Context, request *http.Request) (*RefundNotification, error) {
	// 检查服务是否已初始化
	if !s.IsEnabled() {
		return nil, fmt.Errorf("wechat pay service not initialized")
	}

	// 验证时间戳（防重放攻击）
	timestamp := request.Header.Get("Wechatpay-Timestamp")
	if err := ValidateTimestamp(timestamp); err != nil {
		log.Printf("[WeChatPay] Refund notify timestamp validation failed: %v", err)
		return nil, fmt.Errorf("timestamp validation failed: %w", err)
	}

	// 使用 SDK 验签并解密
	s.mu.RLock()
	handler := s.notifyHandler
	s.mu.RUnlock()

	if handler == nil {
		return nil, fmt.Errorf("notify handler not initialized")
	}

	// 微信退款回调的数据结构
	refundData := &refunddomestic.Refund{}
	_, err := handler.ParseNotifyRequest(ctx, request, refundData)
	if err != nil {
		log.Printf("[WeChatPay] Refund notify parse failed: %v", err)
		return nil, fmt.Errorf("parse refund notification failed: %w", err)
	}

	log.Printf("[WeChatPay] Refund notify parsed: out_trade_no=%s, out_refund_no=%s, status=%s",
		safeString(refundData.OutTradeNo), safeString(refundData.OutRefundNo), string(*refundData.Status))

	return &RefundNotification{
		OutTradeNo:          safeString(refundData.OutTradeNo),
		OutRefundNo:         safeString(refundData.OutRefundNo),
		TransactionID:       safeString(refundData.TransactionId),
		RefundID:            safeString(refundData.RefundId),
		RefundStatus:        string(*refundData.Status),
		SuccessTime:         safeTimeString(refundData.SuccessTime),
		Amount:              *refundData.Amount.Refund,
		UserReceivedAccount: safeString(refundData.UserReceivedAccount),
	}, nil
}
