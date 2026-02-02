# Story 4.7: 支付中页面倒计时

Status: done

## Story

**作为** 普通用户
**我希望** 看到订单过期倒计时
**以便** 了解剩余支付时间

## Acceptance Criteria

- [x] AC1: 显示格式：XX分XX秒
- [x] AC2: 每秒更新一次
- [x] AC3: 倒计时归零时跳转到失败页面
- [x] AC4: 倒计时与后端过期时间同步

## Tasks / Subtasks

- [x] Task 1: 创建 `OrderCountdown.vue` 组件
- [x] Task 2: 实现倒计时计算逻辑
- [x] Task 3: 实现倒计时归零跳转

## Dev Notes

### 组件路径

前端组件路径：`src/components/user/recharge/OrderCountdown.vue`

### 计算逻辑

根据 expired_at 计算剩余时间

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-4.7]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Completion Notes List

1. 创建了 `OrderCountdown.vue` 组件，包含以下功能：
   - Props: `expireAt` (ISO 格式过期时间字符串)
   - Emits: `expired` 事件（倒计时归零时触发）
   - 倒计时显示格式：MM:SS
   - 每秒更新一次
   - 颜色变化：剩余时间 < 60秒变红，< 5分钟变橙
   - 组件卸载时清理定时器
2. 在 `RechargePaymentView.vue` 中集成倒计时组件：
   - 仅在订单状态为 pending 且有 expire_at 时显示
   - 监听 expired 事件，触发时跳转到失败页面
3. 添加了 i18n 翻译文本：
   - zh: remainingTime, countdown
   - en: remainingTime, countdown

### File List

- frontend/src/components/user/recharge/OrderCountdown.vue (新增)
- frontend/src/views/user/RechargePaymentView.vue (修改)
- frontend/src/i18n/locales/zh.ts (修改)
- frontend/src/i18n/locales/en.ts (修改)
