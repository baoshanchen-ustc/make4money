//go:build unit

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestEasyPayCreatePayment_AlipayMobileFallsBackToQRCodeAsPayURL(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":1,"trade_no":"trade-1","payurl":"","payurl2":"","qrcode":"https://qr.alipay.example/pay"}`))
	}))
	defer srv.Close()

	provider, err := NewEasyPay("inst-1", map[string]string{
		"pid":       "10001",
		"pkey":      "secret",
		"apiBase":   srv.URL,
		"notifyUrl": "https://merchant.example/notify",
		"returnUrl": "https://merchant.example/return",
	})
	if err != nil {
		t.Fatalf("NewEasyPay() error = %v", err)
	}

	resp, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "order-1",
		Amount:      "18.88",
		PaymentType: payment.TypeAlipay,
		Subject:     "test-order",
		IsMobile:    true,
		ClientIP:    "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if resp.PayURL != "https://qr.alipay.example/pay" {
		t.Fatalf("pay_url = %q, want fallback qr url", resp.PayURL)
	}
	if resp.QRCode != "https://qr.alipay.example/pay" {
		t.Fatalf("qr_code = %q, want original qr url", resp.QRCode)
	}
}
