# Story 4.6: 前端订单状态轮询

Status: done

## Story

**作为** 前端应用
**我希望** 定期轮询订单状态
**以便** 在用户支付成功后及时跳转

## Acceptance Criteria

- [x] AC1: 支付中页面每3秒查询一次订单状态
- [x] AC2: 最多轮询40次（共2分钟）
- [x] AC3: 状态变为 `paid` 时跳转到成功页面
- [x] AC4: 状态变为 `failed` 或 `expired` 时跳转到失败页面
- [x] AC5: 轮询期间页面显示loading指示器
- [x] AC6: 页面离开时停止轮询

## Tasks / Subtasks

- [x] Task 1: 实现轮询逻辑（setInterval）
- [x] Task 2: 实现状态判断和页面跳转
- [x] Task 3: 实现组件卸载时清理定时器

## Dev Notes

### 轮询实现

```typescript
const pollInterval = setInterval(() => {
  // 查询订单状态
}, 3000);

onUnmounted(() => {
  clearInterval(pollInterval);
});
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-4.6]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Completion Notes List

1. 在 `recharge.ts` API 中添加 `getOrder` 方法
2. 在 `RechargePaymentView.vue` 中实现订单状态轮询:
   - 每 3 秒轮询一次订单状态
   - 最多轮询 40 次（2 分钟）
   - 状态变为 paid 时跳转到 RechargeSuccess 页面
   - 状态变为 failed/expired 时跳转到 RechargeFailed 页面
   - 轮询期间显示 loading 指示器
   - 组件卸载时清理定时器
3. 添加 `waitingPayment` 国际化文案（中英文）

### File List

- frontend/src/api/recharge.ts (修改)
- frontend/src/views/user/RechargePaymentView.vue (修改)
- frontend/src/i18n/locales/zh.ts (修改)
- frontend/src/i18n/locales/en.ts (修改)
