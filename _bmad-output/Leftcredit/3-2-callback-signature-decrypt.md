# Story 3.2: 回调签名验证与数据解密

Status: done

## Story

**作为** 系统
**我希望** 验证回调签名并解密回调数据
**以便** 确保回调来自微信支付平台且数据未被篡改

## Acceptance Criteria

- [x] AC1: 使用微信支付平台证书验证签名
- [x] AC2: 验证请求头中的 Wechatpay-Timestamp, Wechatpay-Nonce, Wechatpay-Signature
- [x] AC3: 拒绝超过5分钟的请求（防重放攻击）
- [x] AC4: 使用 APIv3 密钥解密回调数据（AEAD_AES_256_GCM）
- [x] AC5: 签名验证失败时返回 FAIL 并记录日志
- [x] AC6: 更新 payment_callbacks 表的 signature_valid 字段

## Tasks / Subtasks

- [x] Task 1: 使用微信支付 SDK 验签方法
- [x] Task 2: 实现时间戳检查（5分钟内）
- [x] Task 3: 实现回调数据解密
- [x] Task 4: 更新回调日志的验签结果

## Dev Notes

### 微信支付 SDK 验签

使用 `github.com/wechatpay-apiv3/wechatpay-go/core/notify`

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.2]
- [Source: docs/微信支付Go-SDK集成指南.md]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. 使用微信支付官方 SDK `notify.Handler` 实现签名验证和数据解密
2. 实现 `ValidateTimestamp` 函数进行时间戳验证（5分钟内）
3. 使用 `ParseNotifyRequest` 方法同时完成签名验证和 AEAD_AES_256_GCM 解密
4. 创建 `PaymentNotifyResult` 结构体封装验证结果
5. 更新 `WeChatPayWebhookHandler` 调用验签服务并更新回调日志

### File List

- `backend/internal/service/wechat_pay_service.go` - 添加验签和解密功能
- `backend/internal/handler/webhook/wechat_pay_handler.go` - 集成验签服务
- `backend/internal/handler/webhook/wechat_pay_handler_test.go` - 更新测试
- `backend/cmd/server/wire_gen.go` - 更新依赖注入代码
