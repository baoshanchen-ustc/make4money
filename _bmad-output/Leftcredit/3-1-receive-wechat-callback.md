# Story 3.1: 接收微信支付回调

Status: done

## Story

**作为** 系统
**我希望** 接收微信支付平台的异步回调通知
**以便** 获知用户支付结果

## Acceptance Criteria

- [x] AC1: 提供 POST `/api/v1/webhook/wechat/payment` 回调接口
- [x] AC2: 回调接口无需JWT认证
- [x] AC3: 记录所有回调请求到 `payment_callbacks` 表
- [x] AC4: 记录内容：请求头、请求体、接收时间
- [x] AC5: 响应格式符合微信支付规范

## Tasks / Subtasks

- [x] Task 1: 创建 `backend/ent/schema/payment_callback.go` Schema
- [x] Task 2: 创建回调 Handler
- [x] Task 3: 注册公开路由（无需认证）
- [x] Task 4: 实现回调日志记录

## Dev Notes

### 数据库表

参考 `_bmad-output/planning-artifacts/epics.md#数据库需求` 的 payment_callbacks 表设计

### 响应格式

微信支付要求的响应格式：
- 成功: `{"code": "SUCCESS", "message": ""}`
- 失败: `{"code": "FAIL", "message": "失败原因"}`

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.1]
- [微信支付回调文档](https://pay.weixin.qq.com/wiki/doc/apiv3/apis/chapter3_1_5.shtml)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. **Schema 创建**：创建 `payment_callback.go` Schema，包含以下字段：
   - order_no: 订单号（可选，从回调解析）
   - payment_method: 支付方式
   - transaction_id: 支付平台订单号
   - request_headers: 请求头（JSONB）
   - request_body: 请求体
   - signature_valid: 签名验证结果
   - process_status: 处理状态
   - process_message: 处理结果描述
   - response_code/response_message: 响应信息
   - process_time_ms: 处理耗时
2. **Repository 层**：创建 `PaymentCallbackRepository`，支持 Create/Update/GetByID/ListByOrderNo
3. **Handler 层**：创建 `WeChatPayWebhookHandler`，实现 `HandlePaymentNotify`
4. **路由注册**：在 `/api/v1/webhook/wechat/payment` 注册公开路由（无需认证）
5. **Wire 更新**：更新 repository 和 handler 的 Wire ProviderSet
6. **单元测试**：添加响应格式和请求处理测试

### Code Review Notes

**Issues Found and Fixed:**
- MEDIUM: 添加请求体大小限制（1MB），防止内存耗尽攻击
- LOW: 只记录微信支付相关的请求头，避免记录敏感信息

### File List

**新增文件:**
- `backend/ent/schema/payment_callback.go`
- `backend/internal/repository/payment_callback_repo.go`
- `backend/internal/handler/webhook/wechat_pay_handler.go`
- `backend/internal/handler/webhook/wechat_pay_handler_test.go`
- `backend/internal/server/routes/webhook.go`

**修改文件:**
- `backend/internal/repository/wire.go` - 添加 PaymentCallbackRepository
- `backend/internal/handler/wire.go` - 添加 WeChatPayWebhookHandler
- `backend/internal/handler/handler.go` - 添加 WeChatPayWebhook 字段
- `backend/internal/server/router.go` - 注册 webhook 路由
- `backend/cmd/server/wire_gen.go` - 重新生成

## Change Log

- 2026-02-01: Story 3-1 开发完成
- 2026-02-01: Code Review 完成，修复安全问题，Story 完成

