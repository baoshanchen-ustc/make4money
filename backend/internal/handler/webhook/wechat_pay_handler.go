package webhook

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// 请求体最大大小限制（1MB）
const maxRequestBodySize = 1 << 20

// WeChatPayWebhookHandler 微信支付回调处理器
type WeChatPayWebhookHandler struct {
	callbackRepo             *repository.PaymentCallbackRepository
	wechatPayService         *service.WeChatPayService
	paymentCallbackService   *service.PaymentCallbackService
	rechargeOrderService     *service.RechargeOrderService
	subscriptionOrderService *service.SubscriptionOrderService
}

// NewWeChatPayWebhookHandler 创建微信支付回调处理器
func NewWeChatPayWebhookHandler(
	callbackRepo *repository.PaymentCallbackRepository,
	wechatPayService *service.WeChatPayService,
	paymentCallbackService *service.PaymentCallbackService,
	rechargeOrderService *service.RechargeOrderService,
	subscriptionOrderService *service.SubscriptionOrderService,
) *WeChatPayWebhookHandler {
	return &WeChatPayWebhookHandler{
		callbackRepo:             callbackRepo,
		wechatPayService:         wechatPayService,
		paymentCallbackService:   paymentCallbackService,
		rechargeOrderService:     rechargeOrderService,
		subscriptionOrderService: subscriptionOrderService,
	}
}

// WeChatPayResponse 微信支付回调响应格式
type WeChatPayResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// HandlePaymentNotify 处理微信支付回调通知
// POST /api/v1/webhook/wechat/payment
// 功能：
// 1. 接收微信支付回调
// 2. 验证签名（使用平台证书）
// 3. 验证时间戳（5分钟内，防重放攻击）
// 4. 解密回调数据（AEAD_AES_256_GCM）
// 5. 使用分布式锁处理订单状态更新和余额到账
// 6. 记录回调日志并更新处理结果
func (h *WeChatPayWebhookHandler) HandlePaymentNotify(c *gin.Context) {
	startTime := time.Now()
	ctx := c.Request.Context()

	// 读取请求体（限制大小防止内存耗尽攻击）
	bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, maxRequestBodySize))
	if err != nil {
		log.Printf("[WeChatPayWebhook] Failed to read request body: %v", err)
		h.respondFail(c, "读取请求体失败")
		return
	}
	bodyStr := string(bodyBytes)

	// 重置请求体以便 SDK 可以再次读取
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// 提取必要的请求头（仅记录微信支付相关头部）
	headers := extractWeChatPayHeaders(c.Request.Header)
	headersJSON, _ := json.Marshal(headers)

	log.Printf("[WeChatPayWebhook] Received callback: body_length=%d, timestamp=%s",
		len(bodyStr), headers["Wechatpay-Timestamp"])

	// 创建回调记录（初始状态）
	record := &repository.PaymentCallbackRecord{
		PaymentMethod:   "wechat_pay",
		RequestHeaders:  string(headersJSON),
		RequestBody:     bodyStr,
		SignatureValid:  false,
		ProcessStatus:   "received",
		ProcessMessage:  "回调已接收，验签中",
		ResponseCode:    "",
		ResponseMessage: "",
		ProcessTimeMs:   0,
	}

	callback, err := h.callbackRepo.Create(ctx, record)
	if err != nil {
		log.Printf("[WeChatPayWebhook] Failed to save callback record: %v", err)
		// 即使记录保存失败，仍继续处理验签
	}

	// 使用 WeChatPayService 验签和解密
	notifyResult := h.wechatPayService.ParsePaymentNotify(ctx, c.Request)

	// 更新回调记录
	processTimeMs := time.Since(startTime).Milliseconds()
	record.ProcessTimeMs = processTimeMs

	if notifyResult.IsValid() {
		// 验签成功，解密成功
		record.SignatureValid = true
		record.ProcessStatus = "verified"
		record.ProcessMessage = "签名验证通过，数据解密成功"

		// 从解密后的数据中提取订单信息
		if notifyResult.Transaction != nil {
			if notifyResult.Transaction.OutTradeNo != nil {
				orderNo := *notifyResult.Transaction.OutTradeNo
				record.OrderNo = &orderNo
			}
			if notifyResult.Transaction.TransactionId != nil {
				txID := *notifyResult.Transaction.TransactionId
				record.TransactionID = &txID
			}
		}

		log.Printf("[WeChatPayWebhook] Signature verified: callback_id=%d, order_no=%s, transaction_id=%s",
			getCallbackID(callback), safeStringPtr(record.OrderNo), safeStringPtr(record.TransactionID))

		// 根据订单号前缀分发到不同的处理服务
		orderNo := safeStringPtr(record.OrderNo)
		var processSuccess bool
		var processAlreadyPaid bool
		var processErrorMessage string

		if service.IsSubscriptionOrder(orderNo) {
			// 订阅订单：调用 SubscriptionOrderService 处理
			if h.subscriptionOrderService != nil {
				transactionID := ""
				if notifyResult.Transaction.TransactionId != nil {
					transactionID = *notifyResult.Transaction.TransactionId
				}
				amountInFen := int64(0)
				if notifyResult.Transaction.Amount != nil && notifyResult.Transaction.Amount.Total != nil {
					amountInFen = *notifyResult.Transaction.Amount.Total
				}
				subResult := h.subscriptionOrderService.ProcessPaymentSuccess(ctx, orderNo, transactionID, amountInFen)
				processSuccess = subResult.Success
				processAlreadyPaid = subResult.AlreadyPaid
				processErrorMessage = subResult.ErrorMessage
				log.Printf("[WeChatPayWebhook] Subscription order processed: order_no=%s, success=%v, already_paid=%v",
					orderNo, processSuccess, processAlreadyPaid)
			} else {
				processSuccess = false
				processErrorMessage = "订阅订单服务未配置"
				log.Printf("[WeChatPayWebhook] Subscription order service not configured")
			}
		} else {
			// 充值订单：调用 PaymentCallbackService 处理
			paymentResult := h.paymentCallbackService.ProcessPaymentCallback(ctx, notifyResult.Transaction)
			processSuccess = paymentResult.Success
			processAlreadyPaid = paymentResult.AlreadyPaid
			processErrorMessage = paymentResult.ErrorMessage
		}

		if processSuccess {
			if processAlreadyPaid {
				record.ProcessStatus = "already_paid"
				record.ProcessMessage = "订单已处理（幂等）"
			} else {
				record.ProcessStatus = "completed"
				record.ProcessMessage = "支付处理完成"
			}
			record.ResponseCode = "SUCCESS"
			record.ResponseMessage = ""
			log.Printf("[WeChatPayWebhook] Payment processed: callback_id=%d, order_no=%s, already_paid=%v",
				getCallbackID(callback), safeStringPtr(record.OrderNo), processAlreadyPaid)
		} else {
			record.ProcessStatus = "business_error"
			record.ProcessMessage = "业务处理失败: " + processErrorMessage
			record.ResponseCode = "FAIL"
			record.ResponseMessage = "业务处理失败"
			log.Printf("[WeChatPayWebhook] Payment processing failed: callback_id=%d, order_no=%s, error=%s",
				getCallbackID(callback), safeStringPtr(record.OrderNo), processErrorMessage)
		}
	} else {
		// 验签失败或解密失败
		record.SignatureValid = false
		record.ProcessStatus = "signature_invalid"
		record.ResponseCode = "FAIL"

		errMsg := "验证失败"
		if notifyResult.TimestampErr != nil {
			errMsg = "时间戳验证失败: " + notifyResult.TimestampErr.Error()
		} else if notifyResult.SignatureErr != nil {
			errMsg = "签名验证失败: " + notifyResult.SignatureErr.Error()
		} else if notifyResult.DecryptionErr != nil {
			errMsg = "数据解密失败: " + notifyResult.DecryptionErr.Error()
		}
		record.ProcessMessage = errMsg
		record.ResponseMessage = "签名验证失败"

		log.Printf("[WeChatPayWebhook] Signature verification failed: callback_id=%d, error=%s",
			getCallbackID(callback), errMsg)
	}

	// 更新回调记录
	record.ProcessTimeMs = time.Since(startTime).Milliseconds()
	if callback != nil {
		_, updateErr := h.callbackRepo.Update(ctx, callback.ID, record)
		if updateErr != nil {
			log.Printf("[WeChatPayWebhook] Failed to update callback record: %v", updateErr)
		}
	}

	log.Printf("[WeChatPayWebhook] Callback processed: id=%d, status=%s, process_time=%dms",
		getCallbackID(callback), record.ProcessStatus, record.ProcessTimeMs)

	// 根据处理结果返回响应
	if record.ResponseCode == "SUCCESS" {
		h.respondSuccess(c)
	} else {
		h.respondFail(c, record.ResponseMessage)
	}
}

// respondSuccess 返回成功响应
func (h *WeChatPayWebhookHandler) respondSuccess(c *gin.Context) {
	c.JSON(http.StatusOK, WeChatPayResponse{
		Code:    "SUCCESS",
		Message: "",
	})
}

// respondFail 返回失败响应
func (h *WeChatPayWebhookHandler) respondFail(c *gin.Context, message string) {
	c.JSON(http.StatusOK, WeChatPayResponse{
		Code:    "FAIL",
		Message: message,
	})
}

// extractWeChatPayHeaders 提取微信支付相关的请求头
// 只记录签名验证所需的头部，避免记录敏感信息
func extractWeChatPayHeaders(header http.Header) map[string]string {
	headers := make(map[string]string)

	// 微信支付签名验证所需的头部
	wechatPayHeaders := []string{
		"Wechatpay-Timestamp",
		"Wechatpay-Nonce",
		"Wechatpay-Signature",
		"Wechatpay-Signature-Type",
		"Wechatpay-Serial",
		"Content-Type",
	}

	for _, key := range wechatPayHeaders {
		if values := header.Get(key); values != "" {
			headers[key] = values
		}
	}

	return headers
}

// getCallbackID 安全获取回调记录 ID
func getCallbackID(callback *ent.PaymentCallback) int64 {
	if callback == nil {
		return 0
	}
	return callback.ID
}

// safeStringPtr 安全获取字符串指针的值
func safeStringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// HandleRefundNotify 处理微信退款回调通知
// POST /api/v1/webhook/wechat/refund
// 功能：
// 1. 接收微信退款回调
// 2. 验证签名
// 3. 解密回调数据
// 4. 更新订单退款状态
func (h *WeChatPayWebhookHandler) HandleRefundNotify(c *gin.Context) {
	startTime := time.Now()
	ctx := c.Request.Context()

	// 读取请求体
	bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, maxRequestBodySize))
	if err != nil {
		log.Printf("[WeChatPayWebhook] Refund: Failed to read request body: %v", err)
		h.respondFail(c, "读取请求体失败")
		return
	}
	bodyStr := string(bodyBytes)

	// 重置请求体
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// 提取请求头
	headers := extractWeChatPayHeaders(c.Request.Header)
	headersJSON, _ := json.Marshal(headers)

	log.Printf("[WeChatPayWebhook] Refund: Received callback: body_length=%d, timestamp=%s",
		len(bodyStr), headers["Wechatpay-Timestamp"])

	// 创建回调记录
	record := &repository.PaymentCallbackRecord{
		PaymentMethod:   "wechat_pay_refund",
		RequestHeaders:  string(headersJSON),
		RequestBody:     bodyStr,
		SignatureValid:  false,
		ProcessStatus:   "received",
		ProcessMessage:  "退款回调已接收，验签中",
		ResponseCode:    "",
		ResponseMessage: "",
		ProcessTimeMs:   0,
	}

	callback, err := h.callbackRepo.Create(ctx, record)
	if err != nil {
		log.Printf("[WeChatPayWebhook] Refund: Failed to save callback record: %v", err)
	}

	// 解析退款回调
	notification, err := h.wechatPayService.ParseRefundNotification(ctx, c.Request)
	if err != nil {
		record.ProcessStatus = "signature_invalid"
		record.ProcessMessage = "退款回调验证失败: " + err.Error()
		record.ResponseCode = "FAIL"
		record.ResponseMessage = "签名验证失败"
		log.Printf("[WeChatPayWebhook] Refund: Verification failed: %v", err)

		// 更新记录
		record.ProcessTimeMs = time.Since(startTime).Milliseconds()
		if callback != nil {
			_, _ = h.callbackRepo.Update(ctx, callback.ID, record)
		}
		h.respondFail(c, "签名验证失败")
		return
	}

	// 验签成功
	record.SignatureValid = true
	record.ProcessStatus = "verified"
	orderNo := notification.OutTradeNo
	refundNo := notification.OutRefundNo
	record.OrderNo = &orderNo
	transactionID := notification.TransactionID
	record.TransactionID = &transactionID

	log.Printf("[WeChatPayWebhook] Refund: Verified: order_no=%s, refund_no=%s, status=%s",
		orderNo, refundNo, notification.RefundStatus)

	// 处理退款结果
	if h.rechargeOrderService != nil {
		err = h.rechargeOrderService.HandleRefundCallback(ctx, notification)
		if err != nil {
			record.ProcessStatus = "business_error"
			record.ProcessMessage = "退款业务处理失败: " + err.Error()
			record.ResponseCode = "FAIL"
			record.ResponseMessage = "业务处理失败"
			log.Printf("[WeChatPayWebhook] Refund: Business processing failed: %v", err)
		} else {
			record.ProcessStatus = "completed"
			record.ProcessMessage = "退款处理完成"
			record.ResponseCode = "SUCCESS"
			record.ResponseMessage = ""
			log.Printf("[WeChatPayWebhook] Refund: Completed: order_no=%s, refund_no=%s", orderNo, refundNo)
		}
	} else {
		record.ProcessStatus = "completed"
		record.ProcessMessage = "退款回调接收成功（无业务服务）"
		record.ResponseCode = "SUCCESS"
		log.Printf("[WeChatPayWebhook] Refund: No recharge service configured")
	}

	// 更新记录
	record.ProcessTimeMs = time.Since(startTime).Milliseconds()
	if callback != nil {
		_, _ = h.callbackRepo.Update(ctx, callback.ID, record)
	}

	log.Printf("[WeChatPayWebhook] Refund: Callback processed: id=%d, status=%s, process_time=%dms",
		getCallbackID(callback), record.ProcessStatus, record.ProcessTimeMs)

	// 返回响应
	if record.ResponseCode == "SUCCESS" {
		h.respondSuccess(c)
	} else {
		h.respondFail(c, record.ResponseMessage)
	}
}
