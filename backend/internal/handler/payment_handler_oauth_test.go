package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	paymentcore "github.com/Wei-Shaw/sub2api/internal/payment"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type paymentHandlerSettingRepoStub struct {
	values map[string]string
}

func (s paymentHandlerSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (s paymentHandlerSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", service.ErrSettingNotFound
}

func (s paymentHandlerSettingRepoStub) Set(context.Context, string, string) error { return nil }

func (s paymentHandlerSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[key] = s.values[key]
	}
	return result, nil
}

func (s paymentHandlerSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (s paymentHandlerSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}

func (s paymentHandlerSettingRepoStub) Delete(context.Context, string) error { return nil }

type paymentHandlerUserRepoStub struct {
	service.UserRepository
	user *service.User
}

func (s paymentHandlerUserRepoStub) GetByID(context.Context, int64) (*service.User, error) {
	if s.user != nil {
		return s.user, nil
	}
	return &service.User{Status: paymentcore.EntityStatusActive}, nil
}

func TestPaymentHandlerCreateOrderReturnsOAuthRequiredForWeChatInApp(t *testing.T) {
	gin.SetMode(gin.TestMode)

	configSvc := service.NewPaymentConfigService(nil, paymentHandlerSettingRepoStub{values: map[string]string{
		service.SettingPaymentEnabled:            "true",
		service.SettingEnabledPaymentTypes:       paymentcore.TypeWxpay,
		service.SettingKeyWeChatLoginMPEnabled:   "true",
		service.SettingKeyWeChatLoginMPAppID:     "wx123456",
		service.SettingKeyWeChatLoginMPAppSecret: "wechat-secret",
	}}, nil)
	paymentSvc := service.NewPaymentService(nil, nil, nil, nil, nil, configSvc, paymentHandlerUserRepoStub{}, nil)
	paymentHandler := NewPaymentHandler(paymentSvc, nil, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := strings.NewReader(`{"amount":19.9,"payment_type":"wxpay","order_type":"balance"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment/orders", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 MicroMessenger/8.0.49")
	req.Header.Set("Referer", "https://app.example.com/purchase?plan=starter")
	c.Request = req
	c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 7, Concurrency: 1})

	paymentHandler.CreateOrder(c)
	require.Equal(t, http.StatusOK, rec.Code)

	var bodyResp struct {
		Code int `json:"code"`
		Data struct {
			ResultType  string  `json:"result_type"`
			PaymentType string  `json:"payment_type"`
			Amount      float64 `json:"amount"`
			OAuth       struct {
				AuthorizeURL string `json:"authorize_url"`
				AppID        string `json:"appid"`
				Scope        string `json:"scope"`
				RedirectURL  string `json:"redirect_url"`
			} `json:"oauth"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &bodyResp))
	require.Equal(t, 0, bodyResp.Code)
	require.Equal(t, string(paymentcore.CreatePaymentResultOAuthRequired), bodyResp.Data.ResultType)
	require.Equal(t, paymentcore.TypeWxpay, bodyResp.Data.PaymentType)
	require.Equal(t, 19.9, bodyResp.Data.Amount)
	require.Equal(t, "wx123456", bodyResp.Data.OAuth.AppID)
	require.Equal(t, "snsapi_base", bodyResp.Data.OAuth.Scope)
	require.Equal(t, "/auth/wechat/payment/callback", bodyResp.Data.OAuth.RedirectURL)

	startURL, err := url.Parse(bodyResp.Data.OAuth.AuthorizeURL)
	require.NoError(t, err)
	require.Empty(t, startURL.Scheme)
	require.Empty(t, startURL.Host)
	require.Equal(t, "/api/v1/auth/oauth/wechat/payment/start", startURL.Path)
	require.Equal(t, "wxpay", startURL.Query().Get("payment_type"))
	require.Equal(t, "19.9", startURL.Query().Get("amount"))
	require.Equal(t, "balance", startURL.Query().Get("order_type"))
	require.Equal(t, "/purchase?plan=starter", startURL.Query().Get("redirect"))
}

func TestPaymentHandlerCreateOrderReturnsPaymentDisabledBeforeOAuthPreview(t *testing.T) {
	gin.SetMode(gin.TestMode)

	configSvc := service.NewPaymentConfigService(nil, paymentHandlerSettingRepoStub{values: map[string]string{
		service.SettingPaymentEnabled:            "false",
		service.SettingEnabledPaymentTypes:       paymentcore.TypeWxpay,
		service.SettingKeyWeChatLoginMPEnabled:   "true",
		service.SettingKeyWeChatLoginMPAppID:     "wx123456",
		service.SettingKeyWeChatLoginMPAppSecret: "wechat-secret",
	}}, nil)
	paymentSvc := service.NewPaymentService(nil, nil, nil, nil, nil, configSvc, paymentHandlerUserRepoStub{}, nil)
	paymentHandler := NewPaymentHandler(paymentSvc, nil, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := strings.NewReader(`{"amount":19.9,"payment_type":"wxpay","order_type":"balance"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment/orders", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 MicroMessenger/8.0.49")
	req.Header.Set("Referer", "https://app.example.com/purchase?plan=starter")
	c.Request = req
	c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 7, Concurrency: 1})

	paymentHandler.CreateOrder(c)
	require.Equal(t, http.StatusForbidden, rec.Code)

	var bodyResp struct {
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &bodyResp))
	require.Equal(t, http.StatusForbidden, bodyResp.Code)
	require.Equal(t, "PAYMENT_DISABLED", bodyResp.Reason)
}
