# Story 2.3: 充值金额范围验证

Status: done

## Story

**作为** 系统
**我希望** 验证用户输入的充值金额在允许范围内
**以便** 防止无效或异常金额的订单

## Acceptance Criteria

- [x] AC1: 前端验证：金额 ≥ min_amount 且 ≤ max_amount
- [x] AC2: 后端验证：金额范围校验
- [x] AC3: 金额不在范围内时显示错误提示
- [x] AC4: 提交按钮在金额无效时禁用
- [x] AC5: 错误提示明确说明允许范围

## Tasks / Subtasks

- [x] Task 1: 实现前端金额验证逻辑
- [x] Task 2: 实现后端金额验证逻辑
- [x] Task 3: 实现错误提示展示

## Dev Notes

### 前端验证

- `RechargeView.vue` 中添加 `isAmountValid` computed 属性
- 提交按钮根据 `isAmountValid` 禁用/启用
- 按钮文案动态显示选择金额或默认提示

### 后端验证

- `WeChatPayService.ValidateRechargeAmount()` 方法验证金额范围
- `RechargeHandler.ValidateAmount()` API 端点供前端调用（可选）
- 后续 Story 2-5 创建订单时会调用此验证

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-2.3]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. **Task 1 完成**: 前端金额验证
   - 添加 `isAmountValid` computed 验证金额范围
   - 实现提交按钮禁用逻辑
   - 添加动态按钮文案（显示金额或默认提示）
   - 添加 submitting 状态和 loading 动画

2. **Task 2 完成**: 后端金额验证
   - `WeChatPayService.GetRechargeConfig()` 获取充值配置
   - `WeChatPayService.ValidateRechargeAmount()` 验证金额范围
   - `RechargeHandler.ValidateAmount()` API 端点（可选调用）
   - 更新 `GetConfig()` 使用 service 方法获取配置

3. **Task 3 完成**: 错误提示
   - 复用 Story 2-2 的 AmountSelector 组件错误提示
   - 添加 i18n 支持的提交按钮文案

### Code Review 修复

- [x] M1: 移除未使用的 amountSelectorRef

### File List

**修改文件:**
- `frontend/src/views/user/RechargeView.vue` - 添加金额验证和提交按钮
- `frontend/src/i18n/locales/zh.ts` - 添加提交按钮相关文案
- `frontend/src/i18n/locales/en.ts` - 添加提交按钮相关英文文案
- `backend/internal/service/wechat_pay_service.go` - 添加金额验证方法
- `backend/internal/service/wechat_pay_service_test.go` - 添加验证测试
- `backend/internal/handler/recharge/handler.go` - 添加 ValidateAmount API

## Change Log

- 2026-02-01: Story 实现完成，所有 AC 满足，前后端测试通过
- 2026-02-01: Code Review 完成，移除未使用变量
