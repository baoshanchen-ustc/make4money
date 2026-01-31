package service

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestGenerateOrderNo(t *testing.T) {
	t.Run("format validation", func(t *testing.T) {
		orderNo := GenerateOrderNo()

		// 验证长度：RECH(4) + 时间戳(14) + 随机字符串(10) = 28
		if len(orderNo) != 28 {
			t.Errorf("expected order no length 28, got %d: %s", len(orderNo), orderNo)
		}

		// 验证前缀
		if !strings.HasPrefix(orderNo, "RECH") {
			t.Errorf("expected order no to start with 'RECH', got %s", orderNo)
		}

		// 验证格式：RECH + 14位数字 + 10位字母数字
		pattern := `^RECH\d{14}[a-zA-Z0-9]{10}$`
		matched, err := regexp.MatchString(pattern, orderNo)
		if err != nil {
			t.Fatalf("regex error: %v", err)
		}
		if !matched {
			t.Errorf("order no does not match expected pattern: %s", orderNo)
		}
	})

	t.Run("uniqueness", func(t *testing.T) {
		generated := make(map[string]bool)
		for i := 0; i < 100; i++ {
			orderNo := GenerateOrderNo()
			if generated[orderNo] {
				t.Errorf("duplicate order no generated: %s", orderNo)
			}
			generated[orderNo] = true
		}
	})

	t.Run("timestamp embedded", func(t *testing.T) {
		before := time.Now()
		orderNo := GenerateOrderNo()
		after := time.Now()

		// 提取时间戳部分（第5-18个字符）
		timestamp := orderNo[4:18]

		// 解析时间戳
		parsedTime, err := time.ParseInLocation("20060102150405", timestamp, time.Local)
		if err != nil {
			t.Fatalf("failed to parse timestamp: %v", err)
		}

		// 验证时间在 before 和 after 之间（±1秒的容差）
		if parsedTime.Before(before.Add(-time.Second)) || parsedTime.After(after.Add(time.Second)) {
			t.Errorf("timestamp %s not within expected range [%s, %s]",
				parsedTime.Format(time.RFC3339),
				before.Format(time.RFC3339),
				after.Format(time.RFC3339))
		}
	})
}

func TestGenerateRandomString(t *testing.T) {
	t.Run("correct length", func(t *testing.T) {
		for _, length := range []int{5, 10, 20, 50} {
			s := generateRandomString(length)
			if len(s) != length {
				t.Errorf("expected length %d, got %d", length, len(s))
			}
		}
	})

	t.Run("valid characters", func(t *testing.T) {
		s := generateRandomString(100)
		validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		for _, c := range s {
			if !strings.ContainsRune(validChars, c) {
				t.Errorf("invalid character found: %c", c)
			}
		}
	})

	t.Run("randomness", func(t *testing.T) {
		generated := make(map[string]bool)
		for i := 0; i < 100; i++ {
			s := generateRandomString(10)
			if generated[s] {
				t.Errorf("duplicate random string generated: %s", s)
			}
			generated[s] = true
		}
	})
}

func TestOrderStatusConstants(t *testing.T) {
	// 验证状态常量存在且唯一
	statuses := []string{
		OrderStatusPending,
		OrderStatusPaid,
		OrderStatusFailed,
		OrderStatusExpired,
		OrderStatusCancelled,
	}

	seen := make(map[string]bool)
	for _, status := range statuses {
		if status == "" {
			t.Error("empty status constant found")
		}
		if seen[status] {
			t.Errorf("duplicate status constant: %s", status)
		}
		seen[status] = true
	}
}

func TestPaymentMethodConstants(t *testing.T) {
	// 验证支付方式常量
	if PaymentMethodWeChatPay != "wechat_pay" {
		t.Errorf("expected PaymentMethodWeChatPay to be 'wechat_pay', got %s", PaymentMethodWeChatPay)
	}
	if PaymentMethodAlipay != "alipay" {
		t.Errorf("expected PaymentMethodAlipay to be 'alipay', got %s", PaymentMethodAlipay)
	}
}

func TestPaymentChannelConstants(t *testing.T) {
	// 验证支付渠道常量
	if PaymentChannelNative != "native" {
		t.Errorf("expected PaymentChannelNative to be 'native', got %s", PaymentChannelNative)
	}
	if PaymentChannelJSAPI != "jsapi" {
		t.Errorf("expected PaymentChannelJSAPI to be 'jsapi', got %s", PaymentChannelJSAPI)
	}
	if PaymentChannelH5 != "h5" {
		t.Errorf("expected PaymentChannelH5 to be 'h5', got %s", PaymentChannelH5)
	}
}
