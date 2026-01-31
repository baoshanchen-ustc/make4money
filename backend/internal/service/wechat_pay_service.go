package service

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

// WeChatPayService 微信支付服务
type WeChatPayService struct {
	cfg         *config.Config
	client      *core.Client
	privateKey  *rsa.PrivateKey
	mu          sync.RWMutex
	initialized bool
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
	s.initialized = true
	log.Printf("[WeChatPay] Client initialized successfully")
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
