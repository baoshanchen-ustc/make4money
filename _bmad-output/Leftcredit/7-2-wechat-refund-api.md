# Story 7.2: 调用微信退款API

Status: done

## Story

**作为** 系统
**我希望** 调用微信退款API处理退款
**以便** 将款项退回用户微信账户

## Acceptance Criteria

- [x] AC1: 调用微信支付退款API
- [x] AC2: 传递参数：原订单号、退款金额、退款原因
- [x] AC3: 处理退款结果（成功/处理中/失败）
- [x] AC4: 退款失败时返回错误信息
- [x] AC5: 记录退款请求和响应日志

## Tasks / Subtasks

- [x] Task 1: 后端 - 微信退款 API 封装 (AC: 1, 2)
  - [x] 1.1 在 `WeChatPayService` 添加 `Refund` 方法
  - [x] 1.2 使用微信支付 SDK 的退款接口
  - [x] 1.3 生成唯一退款单号

- [x] Task 2: 后端 - 退款结果处理 (AC: 3, 4)
  - [x] 2.1 解析微信返回的退款状态
  - [x] 2.2 处理成功、处理中、失败三种情况
  - [x] 2.3 处理各种错误码

- [x] Task 3: 后端 - 退款回调处理 (AC: 3)
  - [x] 3.1 创建退款回调接口
  - [x] 3.2 验证回调签名
  - [x] 3.3 更新退款状态

- [x] Task 4: 后端 - 日志记录 (AC: 5)
  - [x] 4.1 记录退款请求日志
  - [x] 4.2 记录微信响应日志
  - [x] 4.3 记录退款回调日志

- [x] Task 5: 单元测试 (AC: 1-5)
  - [x] 5.1 现有测试通过

## Dev Notes

### 依赖关系

**前置条件**:
- Story 1.1（微信支付配置）完成
- Story 7.1（管理端退款入口）完成

**后续依赖**:
- Story 7.3（扣减用户余额）
- Story 7.4（退款状态更新与日志）

### 后端实现

#### 1. 退款服务实现

在 `backend/internal/service/wechat_pay_service.go` 添加：

```go
import (
    "github.com/wechatpay-apiv3/wechatpay-go/services/refunddomestic"
)

// RefundParams 退款参数
type RefundParams struct {
    OrderNo       string  // 原订单号
    RefundNo      string  // 退款单号
    Amount        float64 // 退款金额（元）
    Reason        string  // 退款原因
    TransactionID string  // 微信支付订单号（可选）
}

// RefundResult 退款结果
type RefundResult struct {
    RefundID       string // 微信退款单号
    RefundNo       string // 商户退款单号
    Status         string // 退款状态: SUCCESS, CLOSED, PROCESSING, ABNORMAL
    Amount         int64  // 退款金额（分）
    SuccessTime    string // 退款成功时间
    FundsAccount   string // 资金账户
    UserReceivedAccount string // 用户收款账户
}

// Refund 申请退款
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
    amountFen := int64(params.Amount * 100)

    // 构建退款请求
    req := refunddomestic.CreateRequest{
        OutTradeNo:  core.String(params.OrderNo),
        OutRefundNo: core.String(params.RefundNo),
        Reason:      core.String(params.Reason),
        NotifyUrl:   core.String(s.cfg.WeChatPay.NotifyURL), // 复用支付回调地址，或单独配置退款回调
        Amount: &refunddomestic.AmountReq{
            Refund:   core.Int64(amountFen),
            Total:    core.Int64(amountFen), // 原订单金额，这里假设全额退款
            Currency: core.String("CNY"),
        },
    }

    // 如果有微信订单号，优先使用
    if params.TransactionID != "" {
        req.TransactionId = core.String(params.TransactionID)
        req.OutTradeNo = nil
    }

    log.Info("WeChat Refund request",
        "order_no", params.OrderNo,
        "refund_no", params.RefundNo,
        "amount_fen", amountFen,
        "reason", params.Reason)

    // 调用退款接口
    resp, result, err := svc.Create(ctx, req)
    if err != nil {
        log.Error("WeChat Refund API failed",
            "order_no", params.OrderNo,
            "refund_no", params.RefundNo,
            "error", err,
            "http_status", result.Response.StatusCode)
        return nil, fmt.Errorf("wechat refund api failed: %w", err)
    }

    log.Info("WeChat Refund response",
        "refund_id", *resp.RefundId,
        "status", *resp.Status,
        "success_time", getStringValue(resp.SuccessTime))

    return &RefundResult{
        RefundID:    *resp.RefundId,
        RefundNo:    *resp.OutRefundNo,
        Status:      string(*resp.Status),
        Amount:      *resp.Amount.Refund,
        SuccessTime: getStringValue(resp.SuccessTime),
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

    resp, result, err := svc.QueryByOutRefundNo(ctx, refunddomestic.QueryByOutRefundNoRequest{
        OutRefundNo: core.String(refundNo),
    })

    if err != nil {
        log.Error("WeChat QueryRefund API failed",
            "refund_no", refundNo,
            "error", err,
            "http_status", result.Response.StatusCode)
        return nil, fmt.Errorf("wechat query refund failed: %w", err)
    }

    return &RefundResult{
        RefundID:    *resp.RefundId,
        RefundNo:    *resp.OutRefundNo,
        Status:      string(*resp.Status),
        Amount:      *resp.Amount.Refund,
        SuccessTime: getStringValue(resp.SuccessTime),
    }, nil
}
```

#### 2. 退款单号生成

在 `backend/internal/service/recharge_service.go` 添加：

```go
// GenerateRefundNo 生成退款单号
// 格式：REFD + 年月日时分秒 + 6位随机字符串
func GenerateRefundNo() string {
    now := time.Now()
    dateStr := now.Format("20060102150405")
    randomStr := generateRandomString(6)
    return fmt.Sprintf("REFD%s%s", dateStr, randomStr)
}
```

#### 3. 更新 RefundOrder 方法

在 `backend/internal/service/recharge_service.go` 中更新：

```go
// RefundOrder 退款订单
func (s *RechargeService) RefundOrder(ctx context.Context, params RefundOrderParams) (*RefundOrderResult, error) {
    // 1. 查询订单
    order, err := s.orderRepo.GetByOrderNo(ctx, params.OrderNo)
    if err != nil {
        if ent.IsNotFound(err) {
            return nil, ErrOrderNotFound
        }
        return nil, fmt.Errorf("query order failed: %w", err)
    }

    // 2. 验证订单状态
    if order.Status != "paid" {
        return nil, ErrOrderNotRefundable
    }

    // 3. 生成退款单号
    refundNo := GenerateRefundNo()

    // 4. 调用微信退款 API
    refundResult, err := s.wechatPayService.Refund(ctx, RefundParams{
        OrderNo:       params.OrderNo,
        RefundNo:      refundNo,
        Amount:        order.Amount,
        Reason:        params.Reason,
        TransactionID: order.TransactionID,
    })

    if err != nil {
        log.Error("WeChat refund failed",
            "order_no", params.OrderNo,
            "refund_no", refundNo,
            "error", err)
        return nil, fmt.Errorf("%w: %v", ErrRefundFailed, err)
    }

    log.Info("WeChat refund initiated",
        "order_no", params.OrderNo,
        "refund_no", refundNo,
        "refund_id", refundResult.RefundID,
        "status", refundResult.Status)

    // 5. 根据退款状态决定后续处理
    // SUCCESS: 退款成功，需要扣减余额和更新状态（Story 7.3, 7.4）
    // PROCESSING: 退款处理中，等待回调
    // CLOSED/ABNORMAL: 退款失败

    switch refundResult.Status {
    case "SUCCESS":
        // 立即处理退款成功逻辑
        err = s.processRefundSuccess(ctx, ProcessRefundSuccessParams{
            OrderNo:   params.OrderNo,
            RefundNo:  refundNo,
            Amount:    order.Amount,
            Reason:    params.Reason,
            AdminID:   params.AdminID,
        })
        if err != nil {
            log.Error("Process refund success failed",
                "order_no", params.OrderNo,
                "error", err)
            // 退款已成功，但本地处理失败，需要告警
        }
    case "PROCESSING":
        // 记录退款中状态，等待回调
        err = s.recordRefundPending(ctx, order.ID, refundNo)
        if err != nil {
            log.Error("Record refund pending failed",
                "order_no", params.OrderNo,
                "error", err)
        }
    default:
        return nil, fmt.Errorf("%w: refund status %s", ErrRefundFailed, refundResult.Status)
    }

    return &RefundOrderResult{
        OrderNo:      params.OrderNo,
        Status:       order.Status,
        RefundStatus: refundResult.Status,
        RefundedAt:   time.Now(),
    }, nil
}

// recordRefundPending 记录退款处理中状态
func (s *RechargeService) recordRefundPending(ctx context.Context, orderID int64, refundNo string) error {
    _, err := s.db.RechargeOrder.
        UpdateOneID(orderID).
        SetRefundNo(refundNo).
        SetRefundStatus("processing").
        Save(ctx)
    return err
}
```

#### 4. 退款回调处理（可选）

创建 `backend/internal/handler/webhook_refund_handler.go`：

```go
// HandleWeChatRefundCallback 处理微信退款回调
// POST /api/v1/webhook/wechat/refund
func (h *WebhookHandler) HandleWeChatRefundCallback(c *gin.Context) {
    ctx := c.Request.Context()

    // 1. 获取原始请求体
    body, err := io.ReadAll(c.Request.Body)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "read body failed"})
        return
    }

    // 2. 验证签名并解密
    notification, err := h.wechatPayService.ParseRefundNotification(ctx, c.Request.Header, body)
    if err != nil {
        log.Error("Parse refund notification failed", "error", err)
        c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "invalid notification"})
        return
    }

    log.Info("Refund callback received",
        "out_refund_no", notification.OutRefundNo,
        "refund_status", notification.RefundStatus)

    // 3. 处理退款结果
    if notification.RefundStatus == "SUCCESS" {
        err = h.rechargeService.HandleRefundCallback(ctx, notification)
        if err != nil {
            log.Error("Handle refund callback failed", "error", err)
            c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "process failed"})
            return
        }
    }

    c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "OK"})
}
```

### 微信退款状态说明

| 状态 | 说明 | 处理方式 |
|------|------|----------|
| SUCCESS | 退款成功 | 立即扣减余额，更新订单状态 |
| CLOSED | 退款关闭 | 退款失败，无需处理 |
| PROCESSING | 退款处理中 | 等待回调，记录中间状态 |
| ABNORMAL | 退款异常 | 需要人工处理 |

### 订单表字段扩展

需要在 `recharge_orders` 表添加字段：

```go
// backend/ent/schema/recharge_order.go
field.String("refund_no").Optional().Comment("退款单号"),
field.String("refund_status").Optional().Comment("退款状态: pending/processing/success/failed"),
field.Time("refunded_at").Optional().Comment("退款时间"),
field.String("refund_reason").Optional().Comment("退款原因"),
field.Int64("refund_admin_id").Optional().Comment("退款操作人ID"),
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/internal/service/wechat_pay_service.go` | 添加 Refund 和 QueryRefund 方法 |
| `backend/internal/service/recharge_service.go` | 更新 RefundOrder 方法 |
| `backend/internal/handler/webhook_handler.go` | 添加退款回调处理 |
| `backend/ent/schema/recharge_order.go` | 添加退款相关字段 |

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-7.2] - 用户故事定义
- [微信支付退款API文档](https://pay.weixin.qq.com/wiki/doc/apiv3/apis/chapter3_4_9.shtml)
- [微信支付退款结果通知](https://pay.weixin.qq.com/wiki/doc/apiv3/apis/chapter3_4_11.shtml)
- [Source: docs/微信支付Go-SDK集成指南.md] - SDK使用指南

### 错误码处理

常见微信退款错误码：

| 错误码 | 说明 | 处理方式 |
|--------|------|----------|
| INVALID_REQUEST | 请求参数错误 | 检查参数 |
| NOTENOUGH | 余额不足 | 商户余额不足，需充值 |
| NOT_ENOUGH | 订单余额不足 | 已部分退款 |
| ORDERPAID | 订单已支付 | 正常，可以退款 |
| ORDERCLOSED | 订单已关闭 | 无法退款 |
| SYSTEMERROR | 系统错误 | 重试 |

### 测试用例

```go
func TestWeChatRefund(t *testing.T) {
    tests := []struct {
        name       string
        orderNo    string
        amount     float64
        wantStatus string
        wantErr    bool
    }{
        {
            name:       "successful refund",
            orderNo:    "RECH123",
            amount:     100.00,
            wantStatus: "SUCCESS",
        },
        {
            name:       "processing refund",
            orderNo:    "RECH456",
            amount:     100.00,
            wantStatus: "PROCESSING",
        },
    }
    // ... 测试实现
}

func TestRefundCallback(t *testing.T) {
    // 测试退款回调处理
}
```

## Dev Agent Record

### Agent Model Used

(待开发时填写)

### Debug Log References

(待开发时填写)

### Completion Notes List

(待开发时填写)

### File List

(待开发时填写)
