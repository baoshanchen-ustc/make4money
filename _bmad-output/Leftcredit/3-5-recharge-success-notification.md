# Story 3.5: 充值成功通知

Status: done

## Story

**作为** 普通用户
**我希望** 充值成功后收到通知
**以便** 及时了解充值结果

## Acceptance Criteria

- [x] AC1: 支付成功后异步发送通知（邮件形式）
- [x] AC2: 通知内容：充值金额、订单号、到账时间、当前余额
- [x] AC3: 使用goroutine异步发送，不阻塞回调响应
- [x] AC4: 发送失败时记录错误日志但不影响主流程

## Tasks / Subtasks

- [x] Task 1: 创建通知内容模板（HTML邮件模板）
- [x] Task 2: 实现异步发送逻辑（goroutine + 独立context）
- [x] Task 3: 调用现有邮件服务

## Dev Notes

### 实现说明

由于项目目前没有站内信基础设施，本 Story 采用邮件通知作为替代方案：
- 复用现有的 `EmailService.SendEmail()` 方法
- 仅当用户配置了邮箱且邮件服务可用时发送
- 使用 HTML 模板构建美观的邮件内容

### 异步发送

使用 goroutine 异步处理，创建独立的 context（10秒超时），避免：
1. 阻塞微信回调响应
2. 原 context 取消影响通知发送

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.5]
- backend/internal/service/email_service.go (现有邮件服务)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Completion Notes List

1. **PaymentCallbackService 依赖更新**：添加 `emailService *EmailService` 依赖
2. **异步通知方法**：新增 `sendRechargeSuccessNotification()` 方法
3. **邮件模板**：新增 `buildRechargeSuccessEmailBody()` 方法，生成 HTML 邮件
4. **Wire 更新**：重新生成 wire_gen.go 以注入 EmailService
5. **错误处理**：通知失败仅记录日志，不影响主流程

### File List

- backend/internal/service/payment_callback_service.go (修改)
- backend/cmd/server/wire_gen.go (重新生成)
