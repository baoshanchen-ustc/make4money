# Story 4.9: 充值失败页面

Status: done

## Story

**作为** 普通用户
**我希望** 支付失败或过期后看到失败页面
**以便** 了解失败原因并可以重试

## Acceptance Criteria

- [x] AC1: 显示失败图标和「充值失败」标题
- [x] AC2: 显示失败原因（支付失败/订单过期/用户取消）
- [x] AC3: 提供「重新充值」按钮
- [x] AC4: 提供「查看充值记录」链接（改为返回首页按钮）

## Tasks / Subtasks

- [x] Task 1: 创建 `RechargeFailedView.vue` 页面
- [x] Task 2: 实现失败页面布局（符合 UX 设计）
- [x] Task 3: 根据订单状态显示不同失败原因

## Dev Notes

### 页面路径

前端页面路径：`src/views/user/RechargeFailedView.vue`

### 失败原因映射

- `failed` → 支付失败
- `expired` → 订单已过期
- `cancelled` → 已取消

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-4.9]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Completion Notes List

1. 创建了 `RechargeFailedView.vue` 页面组件，包含：
   - 红色失败图标（X）和圆形背景
   - 「充值失败」标题
   - 根据订单状态显示不同失败原因：
     - failed → 支付过程中出现问题
     - expired → 订单已过期
     - cancelled → 订单已取消
   - 订单信息区域（订单号、金额、状态）
   - 「返回首页」和「重新充值」两个按钮
2. 在 router/index.ts 中添加了 RechargeFailed 路由
3. 添加了 i18n 翻译文本：
   - zh: failedTitle, failedSubtitle, failedReasonPayment, failedReasonExpired, failedReasonCancelled, failedReasonUnknown, retryRecharge, statusCancelled
   - en: 对应英文翻译

### File List

- frontend/src/views/user/RechargeFailedView.vue (新增)
- frontend/src/router/index.ts (修改)
- frontend/src/i18n/locales/zh.ts (修改)
- frontend/src/i18n/locales/en.ts (修改)
