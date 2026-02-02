# Story 2.6: 微信支付订单创建

Status: done

## Story

**作为** 系统
**我希望** 调用微信支付API创建支付订单
**以便** 获取支付参数供用户完成支付

## Acceptance Criteria

- [x] AC1: 调用微信支付下单API（Native或JSAPI）
- [x] AC2: 传递参数：商户订单号、金额（分）、商品描述、notify_url
- [x] AC3: 保存返回的 prepay_id
- [x] AC4: API调用超时设置为30秒
- [x] AC5: API调用失败时记录详细错误日志
- [x] AC6: 返回给前端：订单号、支付参数

## Tasks / Subtasks

- [x] Task 1: 实现 `WeChatPayService.CreateOrder()` 方法
- [x] Task 2: 实现金额转换（元 → 分）
- [x] Task 3: 实现 Native 支付下单
- [x] Task 4: 实现 JSAPI 支付下单
- [x] Task 5: 实现错误处理和日志记录

## Dev Notes

### 微信支付SDK

使用微信支付Go SDK：`github.com/wechatpay-apiv3/wechatpay-go`

### 金额转换

金额转换：元 → 分（乘100）

```go
amountInCents := int64(amount * 100)
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-2.6]
- [Source: docs/微信支付Go-SDK集成指南.md]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. **CreateOrder 方法**：在 `WeChatPayService` 中实现了 `CreateOrder()` 方法，支持 Native 和 JSAPI 两种支付方式
2. **金额转换**：实现 `AmountToFen()` 函数，使用 `math.Round` 避免浮点数精度问题
3. **Native 支付**：实现 `createNativeOrder()` 方法，调用微信 Native API 获取二维码链接
4. **JSAPI 支付**：实现 `createJSAPIOrder()` 方法，调用微信 JSAPI API 获取 prepay_id
5. **错误处理**：所有 API 调用失败都记录详细日志，包括订单号、错误信息
6. **超时设置**：微信支付 SDK 默认使用 30 秒超时
7. **类型定义**：新增 `PaymentChannel`、`WeChatPayRequest`、`WeChatPayResult` 等类型
8. **单元测试**：添加金额转换测试、请求构建测试
9. **Schema 更新**：添加 `prepay_id` 字段到 `recharge_orders` 表
10. **Service 层**：添加 `UpdatePaymentResult` 方法保存支付结果

### Code Review Notes

**Issues Found and Fixed:**
- HIGH: 添加 `prepay_id` 字段到 schema，实现保存微信支付结果到数据库
- MEDIUM: 脱敏 JSAPI 日志中的 OpenID，避免隐私泄露

### File List

**新增字段:**
- `backend/ent/schema/recharge_order.go` - 添加 `prepay_id` 字段

**修改文件:**
- `backend/internal/service/wechat_pay_service.go` - 添加 CreateOrder 方法及相关类型，添加 maskOpenID 函数
- `backend/internal/service/wechat_pay_service_test.go` - 添加单元测试
- `backend/internal/service/recharge_order_service.go` - 添加 PrepayID 字段和 UpdatePaymentResult 方法
- `backend/internal/repository/recharge_order_repo.go` - 支持 prepay_id 字段更新

**Ent 生成文件（自动）:**
- `backend/ent/rechargeorder*.go` - Ent 自动生成

## Change Log

- 2026-02-01: 完成 Story 2.6 实现，所有 AC 验证通过
- 2026-02-01: Code Review 完成，修复 prepay_id 保存和日志脱敏问题
