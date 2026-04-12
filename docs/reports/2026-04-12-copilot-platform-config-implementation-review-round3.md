# Copilot Platform Config 实现复审报告（Round 3）

## 基本信息
- 复审日期：2026-04-12
- 复审对象：当前 `main` 上的 Copilot 平台配置实现（`HEAD`）
- 参考基线：`bd5f078a`
- 对应上一轮报告：`docs/reports/2026-04-12-copilot-platform-config-implementation-review-round2.md`
- 复审类型：需求追加后的实现复审
- 复审方式：静态代码审查 + 本地验证

## 复审结论（摘要）
- 这次追加需求中的大部分内容已经落地：
  - 迁移预置了 44/30 条白名单
  - `max_output_tokens = 0` 语义已打通
  - Copilot 菜单已独立分组
  - route 复用刷新问题已修复
- 但当前实现仍有 1 个高风险问题和 1 个中风险问题。

---

## Findings

### 1) HIGH: 默认平台白名单使用 dot 形式 Claude 模型名，但 `/copilot/v1/models` 暴露的是 dash 形式，精确匹配会误拦合法请求

#### 证据
- 迁移里预置的 Claude 白名单是 dot 形式，例如：
  - `claude-opus-4.6`
  - `claude-sonnet-4.6`
  - `claude-sonnet-4.5`
  - `backend/migrations/087_copilot_platform_configs.sql:31`
  - `backend/migrations/087_copilot_platform_configs.sql:34`
- 前端 Copilot 白名单选择器也使用同样的 dot 形式：
  - `frontend/src/composables/useModelWhitelist.ts:234`
  - `frontend/src/composables/useModelWhitelist.ts:236`
- 但后端默认 `/models` 返回的 `copilot.DefaultModels` 中，Claude ID 是 dash 形式：
  - `backend/internal/pkg/copilot/types.go:158`
  - `backend/internal/pkg/copilot/types.go:171`
  - `backend/internal/pkg/copilot/types.go:172`
- `/copilot/v1/models` 就是直接把这组 `DefaultModels` 序列化返回：
  - `backend/internal/handler/copilot_gateway_handler.go:571`
  - `backend/internal/handler/copilot_gateway_handler.go:576`
- 调度阶段的 Copilot 白名单检查目前是**精确字符串匹配**，没有做 dash/dot 归一化：
  - `backend/internal/service/gateway_service.go:3538`
  - `backend/internal/service/gateway_service.go:3545`
  - `backend/internal/service/gateway_service.go:3552`
- 当前测试也只覆盖了 dot 形式白名单，没有覆盖 `/models` 返回的 dash 形式：
  - `backend/internal/service/gateway_service_copilot_model_support_test.go:54`
  - `backend/internal/service/gateway_service_copilot_model_support_test.go:59`

#### 问题
如果客户端按 `/copilot/v1/models` 返回值选择 Claude 模型，例如：
- `claude-sonnet-4-6`

而平台/账号白名单里存的是：
- `claude-sonnet-4.6`

那么当前 `containsString` 精确比较会直接判定不在白名单内，从而错误过滤掉本应允许的请求。

#### 风险
- 白名单一旦启用，使用 `/models` 返回结果的客户端可能会被误拦。
- 这会直接影响 Claude Code / Copilot 客户端这类依赖模型枚举结果的调用路径。

#### 建议
- 在 Copilot 白名单校验前统一做模型 ID 归一化，至少保证 dash/dot Claude 形式可互认。
- 一种直接做法：
  - 对 `requestedModel` 先跑 `copilot.NormalizeModelIDForCopilotUpstream`
  - 对 whitelist 条目也归一化到同一 canonical form 后再比较
- 同时补一组回归测试，覆盖：
  - whitelist 存 dot，request 用 dash
  - whitelist 存 dash，request 用 dot

---

### 2) MEDIUM: Business / Enterprise 预置白名单里 `text-embedding-ada-002b` 疑似拼写错误

#### 证据
- migration 在 business / enterprise 默认白名单里写的是：
  - `text-embedding-ada-002b`
  - `backend/migrations/087_copilot_platform_configs.sql:34`
  - `backend/migrations/087_copilot_platform_configs.sql:35`
- 同仓库前端 Copilot 模型超集里对应的是常见模型名：
  - `text-embedding-ada-002`
  - `frontend/src/composables/useModelWhitelist.ts:274`

#### 问题
`text-embedding-ada-002b` 看起来像误拼。若真实模型名应为 `text-embedding-ada-002`，那么 business / enterprise 默认白名单会把合法请求挡掉。

#### 风险
- Business / Enterprise 账户对该模型会默认不可用。
- 因为这是 migration 预置值，问题会直接落到新部署库中。

#### 建议
- 确认用户给的 business 可用模型原始列表。
- 如果原始需求不是 `002b`，应修正 migration 预置值，并补一条对应测试/校验。

---

## 验证记录

### 已执行
1. 后端测试
```bash
go test ./internal/service/... ./internal/handler/...
```
结果：通过

2. 前端 TypeScript 类型检查
```bash
npm --prefix frontend run typecheck
```
结果：通过

3. 数量核对
```bash
copilotModels: 44
individual_free whitelist: 44
business whitelist: 30
```
结果：数量与需求描述一致

---

## 复审结论

### Recommendation
`REQUEST CHANGES`

### 原因
- 预置模型数量、菜单分组、`0 = 不限制` 等主需求已经对齐。
- 但当前仍存在一个会误拦合法 Claude 模型请求的白名单 ID 格式兼容性问题。
- 此外 business / enterprise 默认名单里还有一个高概率拼写错误。

### 建议的下一步
1. 先统一 Copilot 白名单的模型 ID canonical form，修掉 dash/dot 互不识别问题。
2. 确认并修正 `text-embedding-ada-002b` 是否为 typo。
3. 补回归测试覆盖上述两类情况。
