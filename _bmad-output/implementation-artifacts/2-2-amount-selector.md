# Story 2.2: 充值金额选择器

Status: done

## Story

**作为** 普通用户
**我希望** 通过快捷按钮或自定义输入选择充值金额
**以便** 方便快速地选择常用金额或精确输入

## Acceptance Criteria

- [x] AC1: 显示快捷金额按钮（从配置获取，如：10、50、100、200、500元）
- [x] AC2: 点击快捷按钮选中对应金额，按钮高亮显示
- [x] AC3: 支持自定义金额输入框
- [x] AC4: 输入自定义金额时取消快捷按钮选中状态
- [x] AC5: 金额输入只允许数字和小数点，最多2位小数
- [x] AC6: 显示充值金额范围提示（如：最小1元，最大1000元）

## Tasks / Subtasks

- [x] Task 1: 创建 `AmountSelector.vue` 组件
- [x] Task 2: 实现快捷金额按钮
- [x] Task 3: 实现自定义金额输入
- [x] Task 4: 实现输入校验（正则表达式）

## Dev Notes

### 组件路径

前端组件路径：`src/components/user/recharge/AmountSelector.vue`

### 双向绑定

使用 v-model 双向绑定金额

### UX 设计

参考 `_bmad-output/planning-artifacts/ux-design.md` 的金额选择器设计

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-2.2]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. **Task 1 完成**: 创建 `AmountSelector.vue` 组件基础结构，使用 Composition API + TypeScript
2. **Task 2 完成**: 实现快捷金额按钮组，支持从 props 接收配置的默认金额列表，按钮选中高亮
3. **Task 3 完成**: 实现自定义金额输入框，支持 v-model 双向绑定，输入时取消快捷按钮选中
4. **Task 4 完成**: 实现金额输入校验：
   - 只允许数字和小数点
   - 最多两位小数
   - 失焦时验证最小/最大金额范围
   - 显示友好的错误提示
5. 集成到 `RechargeView.vue`，从 rechargeStore 获取配置
6. 添加中英文国际化支持
7. 编写完整的单元测试（20 个测试用例）
8. 修复 vitest.config.ts 兼容性问题

### Code Review 修复

- [x] H1: 修复 aria-label 硬编码中文问题，改用 i18n 翻译
- [x] M2: 增强测试断言，添加更严格的边缘用例测试
- [x] L1: 移除空的 beforeEach

### File List

**新增文件:**
- `frontend/src/components/user/recharge/AmountSelector.vue` - 金额选择器组件
- `frontend/src/components/user/recharge/__tests__/AmountSelector.spec.ts` - 组件单元测试

**修改文件:**
- `frontend/src/views/user/RechargeView.vue` - 集成 AmountSelector 组件
- `frontend/src/i18n/locales/zh.ts` - 添加充值金额相关中文文案
- `frontend/src/i18n/locales/en.ts` - 添加充值金额相关英文文案
- `frontend/vitest.config.ts` - 修复与 vite.config.ts 回调形式的兼容性问题

## Change Log

- 2026-02-01: Story 实现完成，所有 AC 满足，测试通过
- 2026-02-01: Code Review 完成，修复 aria-label 国际化和测试增强
