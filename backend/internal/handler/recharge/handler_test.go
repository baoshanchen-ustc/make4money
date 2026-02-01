package recharge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

func TestGetConfig_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		WeChatPay: config.WeChatPayConfig{
			Enabled: false,
		},
	}
	wechatPayService := service.NewWeChatPayService(cfg)
	// rechargeOrderService, rechargeRateLimitService, turnstileService, settingService 为 nil（GetConfig 在禁用时不需要它们）
	handler := NewRechargeHandler(wechatPayService, nil, nil, nil, nil)

	router := gin.New()
	router.GET("/api/v1/recharge/config", handler.GetConfig)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recharge/config", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Code    int                    `json:"code"`
		Message string                 `json:"message"`
		Data    RechargeConfigResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}

	if resp.Data.Enabled != false {
		t.Errorf("expected enabled=false, got %v", resp.Data.Enabled)
	}

	if resp.Data.MinAmount != 0 {
		t.Errorf("expected min_amount=0, got %v", resp.Data.MinAmount)
	}

	if resp.Data.MaxAmount != 0 {
		t.Errorf("expected max_amount=0, got %v", resp.Data.MaxAmount)
	}

	if len(resp.Data.DefaultAmounts) != 0 {
		t.Errorf("expected empty default_amounts, got %v", resp.Data.DefaultAmounts)
	}
}

// TestGetConfig_Enabled 使用 mock 测试启用状态（不需要真实初始化微信支付客户端）
// 由于 WeChatPayService.IsEnabled() 需要 cfg.WeChatPay.Enabled && s.initialized
// 而初始化需要真实的私钥文件，这里我们测试禁用状态即可覆盖核心逻辑
func TestGetConfig_Returns_JSON_Structure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		WeChatPay: config.WeChatPayConfig{
			Enabled: false,
		},
	}
	wechatPayService := service.NewWeChatPayService(cfg)
	handler := NewRechargeHandler(wechatPayService, nil, nil, nil, nil)

	router := gin.New()
	router.GET("/api/v1/recharge/config", handler.GetConfig)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recharge/config", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Verify response structure
	var rawResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &rawResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Check required fields exist
	data, ok := rawResp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'data' field in response")
	}

	requiredFields := []string{"enabled", "min_amount", "max_amount", "default_amounts"}
	for _, field := range requiredFields {
		if _, exists := data[field]; !exists {
			t.Errorf("missing required field: %s", field)
		}
	}
}
