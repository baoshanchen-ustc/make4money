package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestNewWeChatPayServiceDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.WeChatPay.Enabled = false

	svc := NewWeChatPayService(cfg)
	if svc.IsEnabled() {
		t.Fatal("IsEnabled() should be false when disabled")
	}
}

func TestWeChatPayServiceGetClientNotInitialized(t *testing.T) {
	cfg := &config.Config{}
	cfg.WeChatPay.Enabled = false

	svc := NewWeChatPayService(cfg)
	_, err := svc.GetClient()
	if err == nil {
		t.Fatal("GetClient() should return error when not initialized")
	}
}

func TestWeChatPayServiceGetPrivateKeyNotLoaded(t *testing.T) {
	cfg := &config.Config{}
	cfg.WeChatPay.Enabled = false

	svc := NewWeChatPayService(cfg)
	_, err := svc.GetPrivateKey()
	if err == nil {
		t.Fatal("GetPrivateKey() should return error when not loaded")
	}
}

func TestWeChatPayServiceGetConfigSanitized(t *testing.T) {
	cfg := &config.Config{}
	cfg.WeChatPay.Enabled = true
	cfg.WeChatPay.AppID = "wx123"
	cfg.WeChatPay.MchID = "mch456"
	cfg.WeChatPay.APIv3Key = "secret-key-should-not-be-returned"
	cfg.WeChatPay.CertSerialNo = "serial-should-not-be-returned"
	cfg.WeChatPay.PrivateKeyPath = "/path/should-not-be-returned"
	cfg.WeChatPay.NotifyURL = "https://example.com/callback"

	// Don't call initClient() since it requires real key file
	svc := &WeChatPayService{cfg: cfg}

	result := svc.GetConfig()
	if result.AppID != "wx123" {
		t.Fatalf("GetConfig().AppID = %q, want wx123", result.AppID)
	}
	if result.MchID != "mch456" {
		t.Fatalf("GetConfig().MchID = %q, want mch456", result.MchID)
	}
	if result.NotifyURL != "https://example.com/callback" {
		t.Fatalf("GetConfig().NotifyURL = %q, want https://example.com/callback", result.NotifyURL)
	}
	// Sensitive fields must be empty (sanitized)
	if result.APIv3Key != "" {
		t.Fatal("GetConfig().APIv3Key should be empty (sanitized)")
	}
	if result.CertSerialNo != "" {
		t.Fatal("GetConfig().CertSerialNo should be empty (sanitized)")
	}
	if result.PrivateKeyPath != "" {
		t.Fatal("GetConfig().PrivateKeyPath should be empty (sanitized)")
	}
}

func TestWeChatPayServiceGetRechargeConfig(t *testing.T) {
	cfg := &config.Config{}
	svc := &WeChatPayService{cfg: cfg}

	rechargeCfg := svc.GetRechargeConfig()

	if rechargeCfg.MinAmount != 1.0 {
		t.Fatalf("GetRechargeConfig().MinAmount = %f, want 1.0", rechargeCfg.MinAmount)
	}
	if rechargeCfg.MaxAmount != 1000.0 {
		t.Fatalf("GetRechargeConfig().MaxAmount = %f, want 1000.0", rechargeCfg.MaxAmount)
	}
	if len(rechargeCfg.DefaultAmounts) != 5 {
		t.Fatalf("GetRechargeConfig().DefaultAmounts length = %d, want 5", len(rechargeCfg.DefaultAmounts))
	}
}

func TestWeChatPayServiceValidateRechargeAmount(t *testing.T) {
	cfg := &config.Config{}
	svc := &WeChatPayService{cfg: cfg}

	tests := []struct {
		name      string
		amount    float64
		expectErr bool
	}{
		{"valid amount 10", 10.0, false},
		{"valid amount 100", 100.0, false},
		{"valid amount min boundary", 1.0, false},
		{"valid amount max boundary", 1000.0, false},
		{"invalid amount too small", 0.5, true},
		{"invalid amount zero", 0, true},
		{"invalid amount too large", 1001.0, true},
		{"invalid amount negative", -10.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ValidateRechargeAmount(tt.amount)
			if tt.expectErr && err == nil {
				t.Errorf("ValidateRechargeAmount(%f) expected error, got nil", tt.amount)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("ValidateRechargeAmount(%f) unexpected error: %v", tt.amount, err)
			}
		})
	}
}
