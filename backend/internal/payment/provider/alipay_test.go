//go:build unit

package provider

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/smartwalle/alipay/v3"
)

func TestIsTradeNotExist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error returns false",
			err:  nil,
			want: false,
		},
		{
			name: "error containing ACQ.TRADE_NOT_EXIST returns true",
			err:  errors.New("alipay: sub_code=ACQ.TRADE_NOT_EXIST, sub_msg=交易不存在"),
			want: true,
		},
		{
			name: "error not containing the code returns false",
			err:  errors.New("alipay: sub_code=ACQ.SYSTEM_ERROR, sub_msg=系统错误"),
			want: false,
		},
		{
			name: "error with only partial match returns false",
			err:  errors.New("ACQ.TRADE_NOT"),
			want: false,
		},
		{
			name: "error with exact constant value returns true",
			err:  errors.New(alipayErrTradeNotExist),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isTradeNotExist(tt.err)
			if got != tt.want {
				t.Errorf("isTradeNotExist(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestNewAlipay(t *testing.T) {
	t.Parallel()

	validConfig := map[string]string{
		"appId":      "2021001234567890",
		"privateKey": "MIIEvQIBADANBgkqhkiG9w0BAQEFAASC...",
	}

	// helper to clone and override config fields
	withOverride := func(overrides map[string]string) map[string]string {
		cfg := make(map[string]string, len(validConfig))
		for k, v := range validConfig {
			cfg[k] = v
		}
		for k, v := range overrides {
			cfg[k] = v
		}
		return cfg
	}

	tests := []struct {
		name      string
		config    map[string]string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "valid config succeeds",
			config:  validConfig,
			wantErr: false,
		},
		{
			name:      "missing appId",
			config:    withOverride(map[string]string{"appId": ""}),
			wantErr:   true,
			errSubstr: "appId",
		},
		{
			name:      "missing privateKey",
			config:    withOverride(map[string]string{"privateKey": ""}),
			wantErr:   true,
			errSubstr: "privateKey",
		},
		{
			name:      "nil config map returns error for appId",
			config:    map[string]string{},
			wantErr:   true,
			errSubstr: "appId",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewAlipay("test-instance", tt.config)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("expected non-nil Alipay instance")
			}
			if got.instanceID != "test-instance" {
				t.Errorf("instanceID = %q, want %q", got.instanceID, "test-instance")
			}
		})
	}
}

func TestAlipayCreatePaymentBranches(t *testing.T) {
	req := payment.CreatePaymentRequest{
		OrderID:   "order-123",
		Amount:    "18.88",
		Subject:   "Test Order",
		NotifyURL: "https://override.example/notify",
		ReturnURL: "https://override.example/return",
	}
	provider := &Alipay{
		config: map[string]string{
			"notifyUrl": "https://config.example/notify",
			"returnUrl": "https://config.example/return",
		},
		client: &alipay.Client{},
	}

	origWap := alipayTradeWapPay
	origPreCreate := alipayTradePreCreate
	t.Cleanup(func() {
		alipayTradeWapPay = origWap
		alipayTradePreCreate = origPreCreate
	})

	tests := []struct {
		name         string
		req          payment.CreatePaymentRequest
		wantTradeNo  string
		wantPayURL   string
		wantQRCode   string
		wantWapCalls int
		wantPreCalls int
	}{
		{
			name: "mobile uses wap pay flow",
			req: func() payment.CreatePaymentRequest {
				mobileReq := req
				mobileReq.IsMobile = true
				return mobileReq
			}(),
			wantTradeNo:  req.OrderID,
			wantPayURL:   "https://pay.example/wap",
			wantQRCode:   "",
			wantWapCalls: 1,
			wantPreCalls: 0,
		},
		{
			name:         "desktop returns a precreate qr payload",
			req:          req,
			wantTradeNo:  req.OrderID,
			wantPayURL:   "",
			wantQRCode:   "https://qr.example/precreate",
			wantWapCalls: 0,
			wantPreCalls: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			wapCalls := 0
			preCreateCalls := 0

			alipayTradeWapPay = func(client *alipay.Client, param alipay.TradeWapPay) (*url.URL, error) {
				wapCalls++
				if client != provider.client {
					t.Fatalf("wap client mismatch")
				}
				if param.OutTradeNo != tt.req.OrderID {
					t.Fatalf("wap OutTradeNo = %q, want %q", param.OutTradeNo, tt.req.OrderID)
				}
				if param.TotalAmount != tt.req.Amount {
					t.Fatalf("wap TotalAmount = %q, want %q", param.TotalAmount, tt.req.Amount)
				}
				if param.Subject != tt.req.Subject {
					t.Fatalf("wap Subject = %q, want %q", param.Subject, tt.req.Subject)
				}
				if param.ProductCode != alipayProductCodeWapPay {
					t.Fatalf("wap ProductCode = %q, want %q", param.ProductCode, alipayProductCodeWapPay)
				}
				if param.NotifyURL != tt.req.NotifyURL {
					t.Fatalf("wap NotifyURL = %q, want %q", param.NotifyURL, tt.req.NotifyURL)
				}
				if param.ReturnURL != tt.req.ReturnURL {
					t.Fatalf("wap ReturnURL = %q, want %q", param.ReturnURL, tt.req.ReturnURL)
				}
				return url.Parse("https://pay.example/wap")
			}

			alipayTradePreCreate = func(ctx context.Context, client *alipay.Client, param alipay.TradePreCreate) (*alipay.TradePreCreateRsp, error) {
				preCreateCalls++
				if client != provider.client {
					t.Fatalf("precreate client mismatch")
				}
				if param.OutTradeNo != tt.req.OrderID {
					t.Fatalf("precreate OutTradeNo = %q, want %q", param.OutTradeNo, tt.req.OrderID)
				}
				if param.TotalAmount != tt.req.Amount {
					t.Fatalf("precreate TotalAmount = %q, want %q", param.TotalAmount, tt.req.Amount)
				}
				if param.Subject != tt.req.Subject {
					t.Fatalf("precreate Subject = %q, want %q", param.Subject, tt.req.Subject)
				}
				if param.ProductCode != alipayProductCodePreCreate {
					t.Fatalf("precreate ProductCode = %q, want %q", param.ProductCode, alipayProductCodePreCreate)
				}
				if param.NotifyURL != tt.req.NotifyURL {
					t.Fatalf("precreate NotifyURL = %q, want %q", param.NotifyURL, tt.req.NotifyURL)
				}
				return &alipay.TradePreCreateRsp{
					Error: alipay.Error{
						Code: alipay.CodeSuccess,
					},
					QRCode: "https://qr.example/precreate",
				}, nil
			}

			resp, err := provider.CreatePayment(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("CreatePayment() unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("CreatePayment() returned nil response")
			}
			if resp.TradeNo != tt.wantTradeNo {
				t.Fatalf("TradeNo = %q, want %q", resp.TradeNo, tt.wantTradeNo)
			}
			if resp.PayURL != tt.wantPayURL {
				t.Fatalf("PayURL = %q, want %q", resp.PayURL, tt.wantPayURL)
			}
			if resp.QRCode != tt.wantQRCode {
				t.Fatalf("QRCode = %q, want %q", resp.QRCode, tt.wantQRCode)
			}
			if wapCalls != tt.wantWapCalls {
				t.Fatalf("wap calls = %d, want %d", wapCalls, tt.wantWapCalls)
			}
			if preCreateCalls != tt.wantPreCalls {
				t.Fatalf("precreate calls = %d, want %d", preCreateCalls, tt.wantPreCalls)
			}
		})
	}
}

func TestParseAlipayAmount_UsesFirstValidFallbackField(t *testing.T) {
	t.Parallel()

	amount, err := parseAlipayAmount("", "18.88", "19.00")
	if err != nil {
		t.Fatalf("parseAlipayAmount() error = %v", err)
	}
	if amount != 18.88 {
		t.Fatalf("amount = %v, want 18.88", amount)
	}
}

func TestParseAlipayAmount_RejectsMissingAmounts(t *testing.T) {
	t.Parallel()

	_, err := parseAlipayAmount("", " ", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAlipayCreatePaymentDesktopPreCreateError(t *testing.T) {
	provider := &Alipay{
		config: map[string]string{
			"notifyUrl": "https://config.example/notify",
			"returnUrl": "https://config.example/return",
		},
		client: &alipay.Client{},
	}

	origPreCreate := alipayTradePreCreate
	t.Cleanup(func() {
		alipayTradePreCreate = origPreCreate
	})

	alipayTradePreCreate = func(ctx context.Context, client *alipay.Client, param alipay.TradePreCreate) (*alipay.TradePreCreateRsp, error) {
		return nil, errors.New("precreate failed")
	}

	_, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "order-456",
		Amount:  "9.99",
		Subject: "Desktop Order",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "TradePreCreate") {
		t.Fatalf("error = %q, want to contain %q", err.Error(), "TradePreCreate")
	}
}
