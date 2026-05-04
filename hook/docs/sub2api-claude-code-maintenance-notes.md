# sub2api × Claude Code 维护备忘

状态：v1（落地于 `review-sub2api-claude-plan-v5` 分支）
适用读者：维护 sub2api 仓库的工程师 / SRE。

> 用户视角的接入说明在 [`sub2api-claude-code-usage-guide.md`](./sub2api-claude-code-usage-guide.md)。本文档侧重**为什么这么实现**与**升级时怎么改**。

---

## 1. Header baseline 来源与更新边界

### 1.1 `claude.DefaultHeaders`（位置：`backend/internal/pkg/claude/constants.go`）

当前 baseline：

| Field | Value | 备注 |
|---|---|---|
| `User-Agent` | `claude-cli/2.1.92 (external, cli)` | 必须形如 `claude-cli/X.Y.Z (external, cli)` |
| `X-Stainless-Lang` | `js` | |
| `X-Stainless-Package-Version` | `0.70.0` | anthropic-sdk-typescript |
| `X-Stainless-OS` | `Linux` | |
| `X-Stainless-Arch` | `arm64` | |
| `X-Stainless-Runtime` | `node` | |
| `X-Stainless-Runtime-Version` | `v24.13.0` | |
| `X-Stainless-Retry-Count` | `0` | |
| `X-Stainless-Timeout` | `600` | |
| `X-App` | `cli` | 区别于 `x-client-app` |
| `Anthropic-Dangerous-Direct-Browser-Access` | `true` | 与当前 SDK 行为一致；**不要**改 |

`CLICurrentVersion` 必须与 `User-Agent` 中的版本号严格一致；`backend/internal/service/header_util.go` 顶部注释引用的抓包版本号应同步更新。

### 1.2 来源优先级（从高到低）

1. 真实 Claude Code CLI 抓包（首选）；
2. Parrot (`src/transform/cc_mimicry.py`) 维护的 `CLI_USER_AGENT`，更新滞后于真实 CLI；
3. 仓库内 migration 模板（如 `backend/migrations/129_seed_claude_code_template.sql` 写到的 `claude-cli/2.1.114`）— 这是**手工伪装模板**，不能单独作为 baseline 事实来源。

### 1.3 更新流程

1. 抓包确认；
2. 同时更新：
   - `backend/internal/pkg/claude/constants.go`（DefaultHeaders + CLICurrentVersion）；
   - `backend/internal/service/header_util.go` 文件头注释；
   - `backend/internal/pkg/claude/constants_test.go`（如果加新字段断言）；
3. 跑：
   ```bash
   cd backend
   go test ./internal/pkg/claude/... -count=1
   ```
4. 在 release notes 注明 baseline 变更原因。

---

## 2. 条件头白名单（Remote / Agent SDK / additional-protection）

### 2.1 涉及位置

- `backend/internal/service/gateway_service.go` 中 `allowedHeaders` map
- `backend/internal/service/header_util.go` 中 `headerWireCasing` + `headerWireOrder`

四个新增条件头：

```
x-claude-remote-container-id
x-claude-remote-session-id
x-client-app
x-anthropic-additional-protection
```

### 2.2 设计原则

- **只透传不合成**：客户端发送时透传到上游，缺失时**绝不**主动注入默认值；
- 全部小写 wire form（与真实 CLI 抓包一致）；
- `x-app` 与 `x-client-app` 是不同 header，互不覆盖；
- 默认 mimic debug 日志中 `x-claude-remote-container-id`、`x-claude-remote-session-id`、`x-anthropic-additional-protection` 通过 `safeHeaderValueForLog` 自动 hash；`x-client-app` 因为是非敏感的应用类型标识（cli / vscode / etc.），可原样保留。

### 2.3 添加新条件头时的清单

1. 加入 `allowedHeaders`；
2. 加入 `headerWireCasing`（设置正确的 wire form）；
3. 加入 `headerWireOrder`（debug log 顺序）；
4. 在 `claude_conditional_headers_test.go` 中扩 4 类测试（whitelist 包含、wire casing、wire order set、resolveWireCasing 还原）；
5. 评估是否敏感：敏感则在 `safeHeaderValueForLog` 加 hash 分支并在 `gateway_log_redaction_test.go` 加测试。

---

## 3. `x-client-request-id` 自动生成机制

### 3.1 位置

- 主 helper：`backend/internal/service/claude_request_id.go`
- 调用点：`gateway_service.go` 中 `/v1/messages` 与 `/v1/messages/count_tokens` 两条主路径，在 X-Claude-Code-Session-Id 同步**之后**、debug snapshot **之前** 调用 `ensureClaudeFirstPartyRequestID`。

### 3.2 触发条件（必须全部满足）

1. `isFirstPartyAnthropicMessagesURL(targetURL)` 返回 true，要求：
   - `https` 协议；
   - host == `api.anthropic.com`（精确匹配，case-insensitive）；
   - path == `/v1/messages` 或 `/v1/messages/count_tokens`；
   - query 不影响判断；
2. 请求当前缺少 `x-client-request-id`（兼容 canonical / wire casing 的 lookup）；
3. 对应 token type 的开关开启：
   - OAuth / SetupToken：`gateway.claude_request_id.auto_generate_oauth`（默认 `true`）；
   - 其它（API key passthrough / bedrock / 未识别）：`gateway.claude_request_id.auto_generate_api_key_passthrough`（默认 `false`）。

### 3.3 拒绝场景（设计意图）

- `http://api.anthropic.com/...` — 协议降级，拒绝；
- `https://api.anthropic.com.evil/...` — host suffix attack，拒绝；
- `https://example.com/v1/messages` — 第三方域，拒绝；
- `https://api.anthropic.com/v1/models` — 路径不匹配，拒绝；
- 自定义 relay 域名 — 拒绝；
- 客户端已带 `x-client-request-id` — 不覆盖。

### 3.4 测试入口

```bash
cd backend
go test ./internal/service -run 'FirstPartyAnthropic|EnsureClaudeFirstPartyRequestID|ShouldAutoGenerateClaudeRequestID|ClaudeRequestID' -count=1
```

---

## 4. Beta policy preset 语义（`conservative` vs `claude_code_compat`）

### 4.1 位置

- DTO：`backend/internal/handler/dto/settings.go` `BetaPolicySettings.Preset`
- 内部 struct：`backend/internal/service/settings_view.go` `BetaPolicySettings.Preset`
- 服务读写：`backend/internal/service/setting_service.go` `GetBetaPolicySettings` / `SetBetaPolicySettings`
- 网关执行：`backend/internal/service/gateway_service.go` `evaluateBetaPolicy`
- compat allow-list：`backend/internal/service/gateway_service.go` `claudeCodeCompatAllowedBetas`
- 前端 UI：`frontend/src/views/admin/SettingsView.vue` 中 Beta Policy section
- i18n：`frontend/src/i18n/locales/{en,zh}.ts` `admin.settings.betaPolicy.preset*`

### 4.2 行为约定

- `""`（缺失）/ 未识别值：`Get` 时 fallback 到 `conservative`，**不回写 DB**；
- `Set` 时空值归一化为 `conservative`；未识别值返回错误；
- `conservative`（默认）：rules 列表完全按规则执行；
- `claude_code_compat`：当 `rule.BetaToken ∈ claudeCodeCompatAllowedBetas` 时**跳过** Filter / Block，让 token 透传给上游。

### 4.3 当前 allow-list

```
fast-mode-2026-02-01
context-1m-2025-08-07
```

**扩缩列表都是产品决策**：每加 / 删一项就是默认配额行为变更，必须：

1. 同步本文档；
2. 同步 i18n 中的 `presetCompatWarning` 文案；
3. 同步用户文档 `sub2api-claude-code-usage-guide.md` §13 FAQ；
4. 检查 `TestClaudeCodeCompatAllowedBetas_Sentinel`（防漂移测试）。

### 4.4 测试入口

```bash
cd backend
go test -tags unit ./internal/service -run 'BetaPolicy|EvaluateBetaPolicy|ClaudeCodeCompatAllowedBetas' -count=1
```

---

## 5. forwarding 类开关的真实语义

| 开关 | 等价语义 | 不等于… |
|---|---|---|
| `fingerprintUnification=false` | 不套用 sub2api 统一 header fingerprint，仅透传客户端原 header | 不是"完全不改写 metadata.user_id" |
| `metadataPassthrough=false` | 仍重写 body 中的 `metadata.user_id`（替换为 sub2api 自己的 session 形态） | 不是"100% 透明转发" |
| `session_id_masking_enabled=true` | 进一步**固定 / 伪装** session 段，属高级风险开关 | 不要默认开启 |

文档中常被混淆，要求文案准确性高于简洁性。

---

## 6. 响应头 gateway prefix denylist

### 6.1 位置

- `backend/internal/util/responseheaders/responseheaders.go` 中 `gatewayTracePrefixes`
- 配置：`config.ResponseHeaderConfig.AllowGatewayTraceHeaders`（默认 `false`）

### 6.2 当前 denylist

```
x-litellm-
helicone-
x-portkey-
cf-aig-
x-kong-
x-bt-
```

### 6.3 关键设计

- **lower-case prefix match**，使用 `strings.HasPrefix`；
- 必须**先做** `strings.ToLower(strings.TrimSpace(key))`；
- 优先级**高于** `additional_allowed`：即使运营把 `x-litellm-model-id` 加入 `additional_allowed` 也会被拦下；
- **关闭 `cfg.Enabled` 时仍生效**：避免运营关掉 response header filter 后退化为"放行所有 gateway 痕迹"；
- `AllowGatewayTraceHeaders=true` 是**危险诊断 override**，仅限本地排障；不接 admin UI；
- `x-request-id` / `retry-after` / `x-ratelimit-*` 等业务必需头不受影响。

### 6.4 添加新前缀

1. 加入 `gatewayTracePrefixes`；
2. 在 `responseheaders_test.go` 的 `gatewayTraceProbes` 加测试样本；
3. 同步用户文档 §9。

### 6.5 测试入口

```bash
cd backend
go test ./internal/util/responseheaders/... -count=1
```

---

## 7. 调试与日志安全

### 7.1 默认日志（`buildClaudeMimicDebugLine`）

- `metadata.user_id` → `meta.user_id.hash=sha256:XXXXXXXX...`；
- `system` 内容 → 完整内容 hash + ≤80 字符截断预览；
- `authorization` / `x-api-key` → `Bearer [redacted]` / `[redacted]`；
- `cookie` / `set-cookie` → `[redacted]`；
- `x-claude-remote-container-id` / `x-claude-remote-session-id` / `x-anthropic-additional-protection` → hash form；
- `x-client-app` → 原文。

### 7.2 `logredact` 默认敏感 key

```
authorization_code, code, code_verifier, access_token, refresh_token,
id_token, client_secret, password, cookie, set-cookie, user_id
```

### 7.3 `SUB2API_DEBUG_GATEWAY_BODY` —— 危险本地开关

- 写入未脱敏的完整 request body 与 headers；
- 文件以明文落盘，**没有自动 redaction**；
- 仅限**本地短期排障**使用，禁止在共享 / 生产环境开启；
- 启用后必须由运维手工管理 / 删除产生的 log；
- 默认 mimic debug 已 hash 关键字段，绝大多数排障场景**不需要**开此开关。

### 7.4 测试入口

```bash
cd backend
go test ./internal/util/logredact/... -count=1
go test ./internal/service -run 'safeHeaderValueForLog|hashSummary|buildClaudeMimicDebugLine|LogRedaction|ClaudeMimicDebug|Redact' -count=1
```

---

## 8. 观测 / 指标边界（T8.5 follow-up）

本轮新增的三块行为（`x-client-request-id` 自动生成、beta preset、response header denylist）**故意没有**接入指标系统：

| 行为 | 没接指标的理由 |
|---|---|
| `ensureClaudeFirstPartyRequestID` | 行为简单且确定（first-party + 缺失 + 开关 → 生成）。指标对运营帮助有限；如需观测，建议从默认 mimic debug log 反向搜（已 hash） |
| `evaluateBetaPolicy` 的 preset 路径 | 已有现成的 beta filter set，运营关心的是"这个请求最终带了什么 beta"，不是"preset 跳过了几次" |
| `responseheaders` denylist | 出现命中 = 上游真的回了 gateway 痕迹头，是**异常事件**，更适合走错误日志而不是计数器 |

如果未来要补观测：

1. **labels 必须低基数**：禁用 account name / 完整 request id / 完整 `metadata.user_id` / 完整 header value；
2. 复用 `logredact` 与 `hashSummary` 的脱敏；
3. 观测点应明确文档化"指标含义 + 取值范围 + 容许的 cardinality"。

---

## 9. 配置项总览（本轮新增）

### 9.1 静态 config（`config.go`）

| Key (viper) | 类型 | 默认 | 用途 |
|---|---|---|---|
| `gateway.claude_request_id.auto_generate_oauth` | bool | `true` | OAuth / SetupToken 自动生成 `x-client-request-id` |
| `gateway.claude_request_id.auto_generate_api_key_passthrough` | bool | `false` | API key passthrough 同上（默认关闭） |
| `security.response_headers.allow_gateway_trace_headers` | bool | `false` | 危险诊断 override，绕过 gateway prefix denylist；**不接 admin UI** |

### 9.2 DB settings

| Setting Key | 字段 | 默认 | 用途 |
|---|---|---|---|
| `SettingKeyBetaPolicySettings` | `Preset` | `conservative` | `conservative` / `claude_code_compat` |
| `SettingKeyBetaPolicySettings` | `Rules` | 既有规则 | 行为不变 |

### 9.3 Env binding

viper 已开启 env binding（`AutomaticEnv` + `_` 替代 `.`）。例如：

```bash
GATEWAY_CLAUDE_REQUEST_ID_AUTO_GENERATE_OAUTH=false
GATEWAY_CLAUDE_REQUEST_ID_AUTO_GENERATE_API_KEY_PASSTHROUGH=true
SECURITY_RESPONSE_HEADERS_ALLOW_GATEWAY_TRACE_HEADERS=true
```

---

## 10. 完整测试矩阵（一键回归）

### 10.1 后端

```bash
cd backend

# 既有回归
go test ./internal/service ./internal/handler -run 'ClaudeCode|GatewayBeta|Metadata|SessionIDMasking|Header|BetaPolicy' -count=1
go test ./internal/service -run 'AnthropicAPIKeyPassthrough|ResponseHeaders|Privacy|AntigravityPrivacy|OpenAIPrivacy' -count=1

# 本轮新增
go test ./internal/service -run 'FirstPartyAnthropic|ClaudeRequestID|ClaudeConditionalHeaders|ClaudeDefaults|LogRedaction|ClaudeMimicDebug|safeHeaderValueForLog|hashSummary|HeaderWire|AllowedHeaders|EnsureClaudeFirstPartyRequestID|ShouldAutoGenerateClaudeRequestID|isFirstPartyAnthropic|EvaluateBetaPolicy|ClaudeCodeCompatAllowedBetas' -count=1

# 需要 unit build tag 的 stub-repo 测试
go test -tags unit ./internal/service -run 'BetaPolicy|EvaluateBetaPolicy|ClaudeCodeCompatAllowedBetas' -count=1

# util / config / pkg
go test ./internal/util/responseheaders/... ./internal/util/logredact/... ./internal/pkg/claude/... ./internal/config -count=1
```

### 10.2 前端

```bash
cd frontend
pnpm test:run UseKeyModal.spec.ts
```

### 10.3 静态检查

```bash
# 默认 Claude Code 片段不应包含 3P OTEL
! rg -q 'CLAUDE_CODE_ENABLE_TELEMETRY|OTEL_' frontend/src/components/keys/UseKeyModal.vue

# 推荐 env 在 README 与 hook/docs 中保持一致
rg -n 'ANTHROPIC_AUTH_TOKEN|CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC|CLAUDE_CODE_ATTRIBUTION_HEADER' README*.md hook/docs

# 不应出现绝对化承诺（手工 review 即可，grep 用作提醒）
rg -in '完全隐藏代理|完全抑制所有 telemetry|100% 等价于真实 Claude Code' README*.md hook/docs
```

---

## 11. Release / Rollback 速查

### 11.1 本轮发布说明摘要

> 建议作为 release notes 主体的"本轮治理"段：

- **前端模板补 `CLAUDE_CODE_ATTRIBUTION_HEADER=0`**：UseKeyModal 的 Unix / CMD / PowerShell 三种 shell 片段都会包含该 env，向 `settings.json` 既有行为对齐。无需用户行动；旧片段也仍然有效，只是缺少 attribution off。
- **新增 4 个条件头透传**：Remote / Agent SDK 场景下的 `x-claude-remote-container-id` / `x-claude-remote-session-id` / `x-client-app` / `x-anthropic-additional-protection` 在客户端发送时透传；缺失不合成。
- **first-party `x-client-request-id` 自动生成由 OAuth path 默认开启 + first-party 检查保护**：自定义 relay / 第三方域 / API key passthrough 默认不会受影响。
- **响应头 gateway 痕迹 denylist 默认开启**：`x-litellm-*` / `helicone-*` / `x-portkey-*` / `cf-aig-*` / `x-kong-*` / `x-bt-*` 默认不会泄漏到客户端，即使运营在 `additional_allowed` 误放行；危险诊断 override 默认关闭。
- **Beta policy 引入 preset 维度**：默认 `conservative`（保留历史行为），可显式切换 `claude_code_compat` 让 fast-mode / context-1m beta 透传到上游 — 仅在配额可承受时启用。
- **debug log 默认 hash 敏感 fingerprint**：`metadata.user_id` / remote container/session 不再明文进默认日志；`SUB2API_DEBUG_GATEWAY_BODY` 重新标注为危险本地开关。
- **DefaultHeaders baseline 注释化**：记录 baseline 来源 + 更新策略，避免维护漂移。

### 11.2 回滚速查表

| 行为 | 关闭方式 | 影响范围 |
|---|---|---|
| `x-client-request-id` OAuth 自动生成 | `gateway.claude_request_id.auto_generate_oauth=false` | 仅 first-party Anthropic OAuth path 不再补 ID；上游可能略微更"非 CLI"。 |
| `x-client-request-id` API key passthrough 自动生成 | 默认就是关闭 | n/a |
| 响应头 gateway prefix denylist | `security.response_headers.allow_gateway_trace_headers=true`（仅诊断） | 客户端会看到 gateway 痕迹响应头，被 Claude Code gateway detection 检出。 |
| Beta policy `claude_code_compat` 预设 | admin UI 切回 `Conservative` | fast-mode / context-1m 重新被 filter；可能丢失部分新能力。 |
| 前端 `CLAUDE_CODE_ATTRIBUTION_HEADER=0` 模板 | 用户自行从 env / settings.json 移除；不影响功能 | 仅恢复为 attribution header 默认开启状态 |
| 4 个新条件头透传 | 改回 `allowedHeaders` 移除即可（不推荐） | 缺失这些条件头会让 Remote / Agent SDK 路径退化 |
| DefaultHeaders baseline | 编辑 `claude/constants.go` 改回旧值 | User-Agent / Stainless 字段回滚到旧版本 |

### 11.3 灰度顺序（推荐）

如果是从未上线本轮治理的环境推上线：

1. **先发**：前端模板 + 用户文档（A1 / A2）；
2. **再发**：条件头白名单 + 响应头 denylist（A3 / A5）；
3. **再发**：log redaction + DefaultHeaders 注释（A4 / B1）；
4. **再发**：first-party `x-client-request-id` 自动生成（B2，先观察一周再决定是否打开 API key passthrough 那个开关）；
5. **最后**：Beta policy compat preset（B3，先在 staging admin 灰度，再开放给生产 admin 选择）。

---

## 12. 与本计划相关的关键文件索引

| 主题 | 文件 |
|---|---|
| 计划与冻结决议 | `.omx/plans/sub2api-claude-code-telemetry-hardening-plan.md`, `.omx/plans/sub2api-claude-code-telemetry-hardening-freeze.md` |
| Header baseline | `backend/internal/pkg/claude/constants.go`, `backend/internal/service/header_util.go` |
| 条件头白名单 | `backend/internal/service/gateway_service.go` (`allowedHeaders`), `header_util.go` |
| 日志脱敏 | `backend/internal/service/gateway_service.go` (`safeHeaderValueForLog` / `buildClaudeMimicDebugLine`), `backend/internal/util/logredact/redact.go` |
| 响应头 denylist | `backend/internal/util/responseheaders/responseheaders.go` |
| 配置 | `backend/internal/config/config.go` |
| 设置 DTO / 服务 / handler | `backend/internal/handler/dto/settings.go`, `backend/internal/service/setting_service.go`, `backend/internal/service/settings_view.go`, `backend/internal/handler/admin/setting_handler.go` |
| Request-id helper | `backend/internal/service/claude_request_id.go` |
| 前端 UseKeyModal | `frontend/src/components/keys/UseKeyModal.vue` |
| 前端 Settings View | `frontend/src/views/admin/SettingsView.vue` |
| 前端 API 类型 | `frontend/src/api/admin/settings.ts` |
| i18n（zh / en） | `frontend/src/i18n/locales/zh.ts`, `frontend/src/i18n/locales/en.ts` |
| 用户文档 | `hook/docs/sub2api-claude-code-usage-guide.md` |
| 维护文档（本文件） | `hook/docs/sub2api-claude-code-maintenance-notes.md` |
