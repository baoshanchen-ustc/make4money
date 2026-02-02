# Story 2.4: 支付方式选择与订单发起

Status: done

## Story

**作为** 普通用户
**我希望** 选择支付方式并发起充值订单
**以便** 开始支付流程

## Acceptance Criteria

- [x] AC1: 显示支付方式选择器（当前仅支持微信支付）
- [x] AC2: 微信支付图标和名称显示
- [x] AC3: 点击「立即充值」按钮发起订单
- [x] AC4: 按钮点击后显示loading状态，防止重复提交
- [x] AC5: 订单创建成功后跳转到支付页面
- [x] AC6: 订单创建失败显示错误提示

## Tasks / Subtasks

- [x] Task 1: 创建 `PaymentMethodSelector.vue` 组件
- [x] Task 2: 实现订单创建 API 调用
- [x] Task 3: 实现提交状态管理
- [x] Task 4: 实现页面跳转逻辑

## Dev Notes

### 组件路径

前端组件路径：`src/components/user/recharge/PaymentMethodSelector.vue`

### API 调用

调用 POST `/api/v1/recharge/orders` 接口

### API 客户端

API客户端：`src/api/recharge.ts`

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-2.4]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. **Task 1 完成**: 创建 PaymentMethodSelector 组件
   - 支持微信支付和支付宝（支付宝暂未启用）
   - 自定义图标组件（SVG）
   - 选中状态样式切换
   - 禁用状态显示"即将上线"标签
   - v-model 双向绑定支持

2. **Task 2 完成**: 实现订单创建 API 调用
   - 更新 `recharge.ts` 添加 `createOrder` 方法
   - 定义 `CreateOrderRequest` 和 `RechargeOrder` 类型
   - 调用 POST `/api/v1/recharge/orders` 接口

3. **Task 3 完成**: 实现提交状态管理
   - `submitting` 状态控制按钮禁用
   - `errorMessage` 显示错误提示
   - `canSubmit` 综合验证（金额+支付方式）
   - 提交时显示 loading 动画

4. **Task 4 完成**: 实现页面跳转逻辑
   - 添加 `/recharge/payment/:orderNo` 路由
   - 创建 `RechargePaymentView.vue` 支付页面（占位）
   - 订单创建成功后跳转到支付页面

### Code Review 修复

- [x] L1: `selectMethod` 添加防御性检查，确保只有启用的支付方式才会触发事件

### File List

**新建文件:**
- `frontend/src/components/user/recharge/PaymentMethodSelector.vue` - 支付方式选择器组件
- `frontend/src/components/user/recharge/__tests__/PaymentMethodSelector.spec.ts` - 组件测试（14个测试用例）
- `frontend/src/views/user/RechargePaymentView.vue` - 支付页面（占位实现）

**修改文件:**
- `frontend/src/views/user/RechargeView.vue` - 集成支付方式选择和订单创建
- `frontend/src/api/recharge.ts` - 添加订单创建 API
- `frontend/src/router/index.ts` - 添加支付页面路由
- `frontend/src/i18n/locales/zh.ts` - 添加支付相关中文文案
- `frontend/src/i18n/locales/en.ts` - 添加支付相关英文文案

## Change Log

- 2026-02-01: Story 实现完成，所有 AC 满足，前端测试通过（75个测试用例）
- 2026-02-01: Code Review 完成，添加防御性检查
