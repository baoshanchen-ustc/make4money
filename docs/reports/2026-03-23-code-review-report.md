# 代码审查报告（2026-03-23）

## 基本信息

- 审查日期：2026-03-23
- 审查范围：当前 `main` 分支工作区（`/Users/ziji/personal/github/sub2api`）
- 审查方式：静态审查 + 基线验证（后端 `go test/go vet`，前端 `typecheck/test`）

## 审查结论

- 发现 3 个需要关注的问题：
  - 高风险 1 个
  - 中风险 2 个
- 后端测试与静态检查基线通过，但前端测试基线当前不通过，存在系统性不稳定因素。

---

## Findings（按严重度排序）

### 1) 高风险：i18n 模块加载期直接访问 `localStorage`，在非浏览器或受限环境可直接崩溃

- 位置：
  - `frontend/src/i18n/index.ts:20`
  - `frontend/src/i18n/index.ts:35`
  - `frontend/src/i18n/index.ts:69`
- 问题描述：
  - `getDefaultLocale()` 在模块初始化时直接调用 `localStorage.getItem(...)`。
  - `setLocale()` 直接调用 `localStorage.setItem(...)`。
  - 缺少对 `localStorage` 可用性的守卫（如运行环境检查、异常降级）。
- 影响：
  - 测试环境中可直接导致模块导入失败。
  - 在隐私模式、受限 WebView 或未来 SSR/预渲染场景下存在启动期异常风险。
- 证据：
  - `npm run test:run` 报错：`TypeError: localStorage.getItem is not a function`
  - 触发栈包含 `src/i18n/index.ts:20`
- 建议修复：
  - 增加统一的安全存储访问层（例如 `safeStorage`）。
  - 在 `getDefaultLocale()` 与 `setLocale()` 中用 `try/catch` + 可用性判断降级到默认语言。

### 2) 中风险：前端测试基线不稳定，存在全局对象污染与测试初始化未接入

- 位置：
  - `frontend/vitest.config.ts:13`
  - `frontend/src/__tests__/setup.ts`
  - `frontend/src/components/admin/account/__tests__/AccountTestModal.spec.ts:97`
- 问题描述：
  - 已存在 `src/__tests__/setup.ts`，但 `vitest.config.ts` 未配置 `setupFiles`。
  - 某测试直接覆写 `globalThis.localStorage`，且未在 `afterEach` 恢复原对象。
  - 导致其他用例依赖的 `localStorage` 能力丢失，出现级联失败。
- 影响：
  - 前端测试结果对执行顺序和运行环境敏感，稳定性差。
  - CI 与本地可能出现“偶发红/绿”。
- 证据：
  - `npm run test:run` 结果：17 个失败文件，28 个失败测试。
  - 多个失败栈为 `localStorage.clear/getItem is not a function`。
- 建议修复：
  - 在 `vitest.config.ts` 添加 `setupFiles`，统一初始化测试环境。
  - 禁止在单测中直接替换全局 `localStorage`；改为局部 spy/mocking 并在 `afterEach` 完整恢复。
  - 对依赖 `localStorage/sessionStorage` 的模块增加无存储环境回退逻辑。

### 3) 中风险：API Key 鉴权 `skipBilling` 路径判断硬编码，存在路由行为不一致

- 位置：
  - `backend/internal/server/middleware/api_key_auth.go:130`
  - `backend/internal/server/routes/gateway.go:118`
- 问题描述：
  - `skipBilling` 当前仅在 `c.Request.URL.Path == "/v1/usage"` 时生效。
  - 但路由还存在 `/antigravity/v1/usage`，同样指向 `h.Gateway.Usage`。
  - 导致相同语义的 usage 查询接口在不同前缀下，计费拦截行为可能不一致。
- 影响：
  - 用户可能出现“一个 usage 入口可访问，另一个被余额/订阅限制拦截”的不一致体验。
  - 增加排障复杂度，易引发灰度/多平台接入问题。
- 建议修复：
  - 将 `skipBilling` 判断改为“路由语义匹配”而非硬编码字符串（如统一 helper，或路由层打标）。
  - 为 `/v1/usage` 与 `/antigravity/v1/usage` 增加一致性测试。

---

## 验证记录

### 后端

- 执行：`cd backend && go test ./...`
- 结果：通过

- 执行：`cd backend && go vet ./...`
- 结果：通过

### 前端

- 执行：`cd frontend && npm run typecheck`
- 结果：通过

- 执行：`cd frontend && npm run test:run`
- 结果：失败（17 files failed, 28 tests failed）
- 主要失败特征：`localStorage.getItem/clear/removeItem is not a function`

---

## 优先级建议

1. 先修复前端存储访问安全层与测试环境初始化（Finding 1 + 2）。
2. 再统一 usage 路由的 `skipBilling` 语义并补回归测试（Finding 3）。
3. 最后将上述规则沉淀为 lint/test 约束，避免后续回归。
