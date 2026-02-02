# Story 2.1: 充值页面与余额展示

Status: done

## Story

**作为** 普通用户
**我希望** 进入充值页面时看到当前账户余额
**以便** 了解当前余额并决定充值金额

## Acceptance Criteria

- [x] AC1: 充值页面顶部显示当前账户余额（格式：¥XX.XX）
- [x] AC2: 余额从用户信息接口获取
- [x] AC3: 余额显示实时刷新（进入页面时重新获取）
- [x] AC4: 页面加载时显示loading状态

## Tasks / Subtasks

- [x] Task 1: 创建 `RechargeView.vue` 页面
- [x] Task 2: 实现余额展示组件
- [x] Task 3: 调用用户信息接口获取余额
- [x] Task 4: 实现页面加载状态
- [x] Task 5: 添加 i18n 翻译

## Dev Notes

### 接口复用

复用 `authStore.refreshUser()` 方法获取余额（用户表已有 balance 字段）

### 页面路径

前端页面路径：`src/views/user/RechargeView.vue`

### UX 设计

参考 `_bmad-output/planning-artifacts/ux-design.md` 的充值首页设计，实现了：
- 橙色渐变余额卡片
- 钱包图标（白色半透明背景）
- 余额标签和数值（¥XX.XX 格式）
- 加载状态 spinner
- 充值表单区域占位（后续 Story 实现）

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-2.1]
- [Source: _bmad-output/planning-artifacts/ux-design.md] - UX 设计规范

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Completion Notes List

- Task 1: 重构 `RechargeView.vue`，添加完整页面结构
- Task 2: 实现余额展示卡片组件，包含钱包图标、余额标签、余额数值
- Task 3: 使用 `authStore.refreshUser()` 在 onMounted 时获取最新用户数据
- Task 4: 添加 loading 状态和 spinner 动画
- Task 5: 在 zh.ts 和 en.ts 中添加 `recharge.currentBalance` 和 `recharge.subtitle` 翻译

### File List

**前端修改**:
- `frontend/src/views/user/RechargeView.vue` (重构) - 充值页面，添加余额展示卡片和加载状态
- `frontend/src/i18n/locales/zh.ts` (修改) - 添加 currentBalance、subtitle 翻译
- `frontend/src/i18n/locales/en.ts` (修改) - 添加 currentBalance、subtitle 翻译

**配置修改**:
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (修改) - 更新状态为 in-progress -> done

## Change Log

- 2026-02-01: 实现 Story 2.1 - 充值页面与余额展示。创建带余额卡片和加载状态的充值页面。
