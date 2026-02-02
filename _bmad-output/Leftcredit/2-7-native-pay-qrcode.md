# Story 2.7: Native支付二维码生成

Status: done

## Story

**作为** 普通用户（PC端）
**我希望** 看到支付二维码进行扫码支付
**以便** 在PC端使用微信扫码完成支付

## Acceptance Criteria

- [x] AC1: Native支付返回 code_url
- [x] AC2: 前端根据 code_url 生成二维码图片
- [x] AC3: 二维码尺寸适中（200x200像素）
- [x] AC4: 二维码下方显示「请使用微信扫码支付」提示
- [x] AC5: 二维码生成时间 < 1秒

## Tasks / Subtasks

- [x] Task 1: 创建 `QRCodeDisplay.vue` 组件
- [x] Task 2: 集成 qrcode.js 或 vue-qrcode
- [x] Task 3: 实现二维码展示样式

## Dev Notes

### 组件路径

前端组件路径：`src/components/user/recharge/QRCodeDisplay.vue`

### 二维码库

使用 qrcode.js（npm package: qrcode）生成二维码，已存在于项目依赖中

### UX 设计

参考 `_bmad-output/planning-artifacts/ux-design.md` 的支付中页面设计

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-2.7]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. **QRCodeDisplay 组件**：创建完整的二维码展示组件，支持：
   - 传入 `codeUrl` 生成二维码
   - 可配置尺寸 `size`（默认 200px）
   - 错误状态展示和重试功能
   - 生成成功/失败事件回调
2. **qrcode 库集成**：使用 `QRCode.toCanvas()` API 直接渲染到 canvas
3. **样式实现**：
   - 二维码使用白色背景圆角卡片包裹
   - 底部显示微信图标和扫码提示
   - 支持深色模式
4. **集成到支付页面**：在 `RechargePaymentView.vue` 中使用 QRCodeDisplay
5. **类型更新**：在 `RechargeOrder` 类型中添加 `qrcode_url` 和 `prepay_id` 字段
6. **单元测试**：12 个测试用例全部通过

### Code Review Notes

**Issues Found and Fixed:**
- MEDIUM: 添加 codeUrl 格式验证（验证微信支付 URL 前缀）
- MEDIUM: 添加重试次数限制（最多 3 次）和禁用状态
- LOW: 添加重试按钮 aria-label 无障碍属性
- 新增 2 个测试用例覆盖 URL 验证和重试限制

### File List

**新增文件:**
- `frontend/src/components/user/recharge/QRCodeDisplay.vue`
- `frontend/src/components/user/recharge/__tests__/QRCodeDisplay.spec.ts`

**修改文件:**
- `frontend/src/views/user/RechargePaymentView.vue` - 集成 QRCodeDisplay 组件
- `frontend/src/api/recharge.ts` - 添加 qrcode_url 和 prepay_id 类型
- `frontend/src/i18n/locales/zh.ts` - 添加二维码相关文案
- `frontend/src/i18n/locales/en.ts` - 添加二维码相关文案

## Change Log

- 2026-02-01: Story 2-7 开发完成，进入 Code Review 阶段
- 2026-02-01: Code Review 完成，修复 URL 验证和重试限制问题，Story 完成

