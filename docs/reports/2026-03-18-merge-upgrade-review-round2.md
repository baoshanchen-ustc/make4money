# Merge 升级复审报告（Round 2）

## 基本信息
- 复审日期：2026-03-18
- 复审基线提交：`04d5a1f80117cf783ff8b13a0fabf554e2899178`
- 对应上一轮问题：`docs/reports/2026-03-18-merge-upgrade-review.md`
- 复审目标：验证上次提出的 3 个问题是否被正确修复，并确认未引入明显新回归

## 复审结论（摘要）
- 上一轮提出的 3 个问题均已落地修复，代码状态与修复说明一致。
- 本轮未发现新的阻断级问题。
- 前端 TypeScript 类型检查通过。
- 后端本地测试仍受 Go 版本限制（环境 `1.26.0`，项目要求 `>=1.26.1`）。

---

## 逐项复核结果

### 1) `embed_on.go` 高风险问题：已修复

#### 修复点
- `shouldBypassEmbeddedFrontend` 已新增：
  - `trimmed == "/chat/completions"`

#### 证据
- `backend/internal/web/embed_on.go:236`

#### 复核结论
- 该修复可避免 embed 模式下根路径 `POST /chat/completions` 被前端中间件误拦截，符合预期。

---

### 2) `KeysView.vue` Copilot usage 404 问题：已修复

#### 修复点
- 新增平台分支变量：
  - `const usageEnabledValue = platform === 'copilot' ? 'false' : 'true'`
- URL 参数使用：
  - `usageEnabled: usageEnabledValue`

#### 证据
- `frontend/src/views/user/KeysView.vue:1746`
- `frontend/src/views/user/KeysView.vue:1756`

#### 复核结论
- Copilot 导入不再强制开启 usage 拉取，可规避 `/copilot/v1/usage` 的 404 问题。

---

### 3) `EditAccountModal.vue` 重复逻辑问题：已修复

#### 修复点
- `apikey` 分支中重复的 `api_key/model_mapping/custom_error_codes` 处理块已删除。
- 当前仅保留一套完整处理逻辑（含 delete 清理和 pool_mode 处理）。

#### 证据
- 关键逻辑仅出现一次：
  - `frontend/src/components/account/EditAccountModal.vue:2902`
  - `frontend/src/components/account/EditAccountModal.vue:2914`
  - `frontend/src/components/account/EditAccountModal.vue:2935`

#### 复核结论
- 逻辑重复问题已清理，行为一致性风险显著降低。

---

## 变更文件核对
- `backend/internal/web/embed_on.go`
- `frontend/src/views/user/KeysView.vue`
- `frontend/src/components/account/EditAccountModal.vue`

与修复提交说明一致：`04d5a1f80117cf783ff8b13a0fabf554e2899178`。

---

## 验证记录

### 已执行
1. 前端类型检查
```bash
npm --prefix frontend run typecheck
```
结果：通过（无报错）

### 受环境限制未执行
1. 后端测试（示例）
```bash
go test ./internal/web -run Test -count=1
```
结果：失败，原因是本地 Go 版本为 `1.26.0`，而 `backend/go.mod` 要求 `>=1.26.1`。

---

## 给 Claude Code 的复查建议（可选）
1. 在其环境中补跑后端相关测试，至少覆盖 `internal/server/routes` 与 `internal/web` 路径。
2. 增加一个回归用例，验证 embed 模式下 `POST /chat/completions` 不会被前端中间件吞掉。
3. 对 `EditAccountModal` 的 `handleSubmit(apikey)` 增加最小单测，确保后续合并不再引入重复块。
