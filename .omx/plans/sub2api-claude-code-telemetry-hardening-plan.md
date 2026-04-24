# sub2api Claude Code 遥测与转发链路治理计划

状态：Draft v5（二次审核优化版）  
日期：2026-04-24  
输入依据：`hook/docs/claude-code-telemetry-comparison-cliproxyapi-newapi-sub2api.md`

---

## 1. 需求摘要

基于现有系统复核报告，sub2api 的目标不是继续向 **最强伪装器** 演进，而是把当前已经有优势的这条链路做扎实：

1. 继续保持 **客户端侧 nonessential traffic 抑制** 的领先优势；
2. 补齐现代 Claude Code 普通 CLI 场景下的关键兼容项；
3. 把当前已有但分散的隐私、转发、响应头、兼容性能力，整理成可维护、可验证、可文档化的交付面；
4. 避免为追求“更像 Claude Code”而引入过多 synthetic rewrite。

本计划聚焦 **sub2api**，不包含 CLIProxyAPI / new-api 的实现改动。

二次 Review 结论：方案方向合理，优先级也基本正确；需要补强的不是“更深伪装”，而是 **配置开关 / 迁移 / 响应头 denylist 防绕过 / 用户显式 OTEL 边界 / 灰度回滚 / 现有入口文档同步 / 子任务边界与文件锁 / 每项 DoD** 这几类产品化细节。

本轮二次审核补充的关键点：

- 增加执行前置门禁：配置命名 freeze、文件 owner freeze、测试包路径确认后再并行；
- 明确所有 issue / 子代理任务必须回报 `Summary / Files changed / Tests / Rollback / Known risks`；
- 明确新增配置使用现有 snake_case / mapstructure 风格，不使用 Go 风格配置名暴露给用户；
- 明确 response header denylist 是 lower-case prefix 级别安全边界，不能用 exact-match `force_remove` 替代；
- 明确 Claude Code gateway detection 除响应头外，还会检查部分已知 `ANTHROPIC_BASE_URL` provider-owned host suffix（当前源码确认的是 Databricks suffix 等），响应头过滤不能掩盖这类 host 命中；
- 将 README 多语言同步定位为“短入口 + 链接详细文档”，避免三份 README 长文漂移；
- 强化 T2/T3/T7/T8 共享 `gateway_service.go`、T4/T5 共享 `config.go`、T5/T8 共享 settings API/UI 的串行/锁文件要求。

---

## 2. 目标与非目标

### 2.1 本轮目标

- 将 sub2api 的 Claude Code 使用方式沉淀成一套**默认正确**的推荐路径；
- 提升普通 Claude Code CLI 请求在 sub2api 上的**现代 header / metadata / request-id 对齐度**；
- 补齐当前报告中已确认的几个缺口：
  - shell 片段未显式下发 `CLAUDE_CODE_ATTRIBUTION_HEADER=0`
  - 默认内置白名单未覆盖 Remote / Agent SDK 条件头
  - `claude.DefaultHeaders` 基线过旧
  - 默认缺失 `x-client-request-id` 自动生成能力
  - beta policy 缺少“真实 Claude Code 兼容”视角
  - 文档仍不足以解释 `ANTHROPIC_AUTH_TOKEN` 与 `ANTHROPIC_API_KEY` 的差异
- 明确 `DISABLE_TELEMETRY` 与 `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` 的关系，避免用户误以为标准模板漏配；
- 明确用户显式开启的 3P OTEL（`CLAUDE_CODE_ENABLE_TELEMETRY` + `OTEL_*`）不属于 Anthropic 侧 telemetry 优化范围，标准模板不得主动开启；
- 明确 `fingerprintUnification`、`metadataPassthrough`、`session_id_masking_enabled` 三个开关的真实语义；
- 把响应头 gateway 痕迹过滤、BigQuery metrics cache 边界、调试日志脱敏要求加入测试 / 文档闭环；
- 把新增行为设计成可灰度、可回退的配置项，避免一次性改变所有账号 / 所有上游路径。

### 2.2 非目标

- 不追求全面复制 CLIProxyAPI 那种全链路强塑形；
- 不引入默认 fake `metadata.user_id` 新策略；
- 不把 TLS 指纹改成默认全局强开；
- 不以破坏现有 Anthropic API key passthrough 为代价去统一 OAuth 路径；
- 不在代理端强行删除 / 改写用户已有的 attribution、billing、CCH 类 header；本轮只增强**推荐用法模板**和**受控默认补齐**；
- 不默认额外下发 `DISABLE_TELEMETRY` 作为必选 env；标准模板继续以更强的 `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` 为主；
- 不默认设置 `CLAUDE_CODE_ENABLE_TELEMETRY`、`OTEL_*` 或任何用户 3P telemetry 相关变量；
- 不处理 Claude Code Remote / Agent SDK 的所有高级场景，只补齐**默认内置逻辑**的兼容底座。

---

## 3. 验收标准

### 3.1 产品 /行为验收

1. 使用前端生成的 shell / CMD / PowerShell 片段时，用户能直接获得：
   - `ANTHROPIC_BASE_URL`
   - `ANTHROPIC_AUTH_TOKEN`
   - `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`
   - `CLAUDE_CODE_ATTRIBUTION_HEADER=0`
2. OAuth / SetupToken 路径在普通 Claude Code CLI 请求下，默认内置逻辑可透传或补齐以下关键 header：
   - `X-Claude-Code-Session-Id`
   - `x-client-request-id`（限 first-party Anthropic 上游、且请求缺失时）
   - `x-claude-remote-container-id`
   - `x-claude-remote-session-id`
   - `x-client-app`
   - `x-anthropic-additional-protection`
3. `claude.DefaultHeaders` 更新后，不再使用 `claude-cli/2.1.22` / `0.70.0` 这类明显过旧基线；
4. 保留当前 `ANTHROPIC_AUTH_TOKEN` 推荐路径，并有明确文档说明为何不建议用户随意切换到 `ANTHROPIC_API_KEY`；
5. beta policy 至少提供一种“Claude Code 兼容优先”的预设或显式说明，不再让用户只能被动接受默认过滤；
6. 变更后不破坏：
   - Anthropic OAuth 主路径
   - Anthropic API key passthrough
   - `count_tokens`
   - 响应头白名单过滤
   - Antigravity / OpenAI privacy 现有逻辑
7. 文档中必须明确：
   - `DISABLE_TELEMETRY` 是 no-telemetry；`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` 是 essential-traffic，覆盖范围更宽；
   - `/api/event_logging/batch` 是兜底，不是 Datadog / BigQuery metrics 的代理侧主防线；
   - BigQuery metrics 有两条固定直连 Anthropic 的 endpoint：
     - export：`https://api.anthropic.com/api/claude_code/metrics`
     - opt-out 探测：`https://api.anthropic.com/api/claude_code/organizations/metrics_enabled`
   - `metricsStatusCache.enabled=true`、本机残留 `ANTHROPIC_API_KEY` / `apiKeyHelper` / keychain API key 时，BigQuery metrics 边界需要重新评估；
   - 用户显式开启 `CLAUDE_CODE_ENABLE_TELEMETRY` / `OTEL_*` 时，会走用户自配 3P OpenTelemetry 链路，不应被描述为 sub2api 可完全抑制的 Anthropic 侧遥测；
   - `fingerprintUnification=false` 不等于 `metadata.user_id` 透传，`metadataPassthrough=false` 仍会重写 `metadata.user_id`；
   - `session_id_masking_enabled` 会进一步固定 / 伪装 session 段，属于更高风险开关。
   - Claude Code gateway detection 同时会看响应头前缀与部分已知 `ANTHROPIC_BASE_URL` provider-owned host suffix；响应头过滤只能减少 gateway header 痕迹，不能掩盖已知 host suffix 命中。
8. 所有新增默认行为需要有明确回滚入口：
   - `x-client-request-id` 自动生成可关闭；
   - beta policy 兼容预设可退回保守策略；
   - response header gateway denylist 如支持管理员 override，必须是显式危险开关而不是 `additional_allowed` 误放行。

### 3.2 测试验收

1. 新增或更新单测，覆盖：
   - UseKeyModal 输出
   - 条件头白名单透传
   - wire casing 恢复
   - first-party `x-client-request-id` 自动生成逻辑
   - beta policy 兼容预设 / 过滤行为
   - 响应头 gateway 痕迹过滤
   - Anthropic API key passthrough 的请求头 / 响应头边界
   - debug / diagnostic 日志脱敏
   - response header `additional_allowed` 不应意外放行已知 gateway 痕迹前缀，或必须通过显式危险开关才能放行
   - 新增配置项默认值、迁移兼容、API contract
   - 配置名 / DTO 字段名和现有 snake_case / mapstructure 风格一致，不出现对外 Go 风格字段名
2. 至少通过以下回归：

```bash
go test ./internal/service ./internal/handler -run 'ClaudeCode|GatewayBeta|Metadata|SessionIDMasking|Header|BetaPolicy'
```

```bash
go test ./internal/service -run 'AnthropicAPIKeyPassthrough|ResponseHeaders|Privacy|AntigravityPrivacy|OpenAIPrivacy'
```

3. 如新增专门测试文件，需补充：

```bash
go test ./internal/service -run 'ClaudeRequestID|ClaudeConditionalHeaders|ClaudeDefaults'
```

前端片段测试单独执行：

```bash
cd frontend
pnpm test:run -- UseKeyModal
```

说明：上述命令是推荐回归子集；执行前应以当前仓库实际 package / test file 为准校正路径。例如响应头过滤当前核心包是 `backend/internal/util/responseheaders`，如果 `./internal/util/...` 覆盖过宽或命中无关失败，应拆成更精确的 package 命令并记录原因。

---

## 4. 实施原则

1. **优先减少误差，不优先增加伪装深度**。  
2. **优先补默认路径**，不把复杂场景强塞进所有路径。  
3. **先补文档与推荐配置，再补可选增强能力**。  
4. **所有改动都要在 `/v1/messages` 与 `/v1/messages/count_tokens` 两条主路径上对齐验证**。  
5. **保持 API key passthrough 的 auth-only 设计边界**。  
6. **所有自动生成行为必须可解释、可灰度、可回退**：条件头白名单可以默认启用；主动生成 `x-client-request-id`、兼容 beta policy 这类行为需要有清晰边界或开关。
7. **日志默认安全**：新增 debug / observability 代码不得打印 token、`Authorization`、`x-api-key`、cookie、完整 request body、完整 `metadata.user_id` 或其它用户内容；必须复用 / 补齐现有 redaction 机制。
8. **默认安全优先于管理员误配置**：`security.response_headers.additional_allowed` 不应轻易绕过 Claude Code gateway 痕迹过滤；如确需放行，必须用显式命名的 diagnostic override，并在 UI / 文档标红。
9. **配置变更先落 schema / defaults / tests，再接 UI**：新增开关必须同步覆盖 config default、DTO/API contract、前端表单、持久化、回归测试和文档。
10. **先冻结命名，再并行执行**：P0-06 必须先确定配置 key、DTO 字段、UI label、回滚语义和文件 owner；T4/T7/T8 不应各自发明不同配置名。
11. **优先用短 README 指向长文档**：README / README_CN / README_JA 只保留标准片段、风险摘要和链接；复杂解释集中在 `hook/docs/*`，避免多语言长文维护漂移。

---

## 5. 实施步骤

## 阶段 A：P0 快速落地项（1~2 天）

### A1. 统一前端生成的 Claude Code 环境变量模板

**目标**：把 `CLAUDE_CODE_ATTRIBUTION_HEADER=0` 从 `settings.json` 示例扩展到 shell/CMD/PowerShell 片段，减少用户手工漏配。

**涉及文件**：

- `frontend/src/components/keys/UseKeyModal.vue:443-475`
- `README.md`
- `README_CN.md`
- `README_JA.md`
- 其它包含 `ANTHROPIC_AUTH_TOKEN` / Claude Code 配置片段的文档

**动作**：

1. 更新 Unix / CMD / PowerShell 片段；
2. 保持 `settings.json` 示例不变；
3. 若有 i18n 提示文案，补一句“Claude Code 推荐使用 auth token + nonessential traffic off + attribution off”。
4. 不在代理转发层强行删除客户端已有 attribution / billing / CCH 相关 header；该项只修正用户侧推荐模板，避免误伤真实 Claude Code 或 API key passthrough。
5. 不把 `DISABLE_TELEMETRY` 作为默认必选 env 塞进模板；如需提及，放到文档的“可选说明”中，解释它弱于 `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`。
6. 同步更新 README / 多语言 README / hook docs 中的 Claude Code 配置示例，避免用户绕过前端复制到旧片段。
7. 用 `rg -n 'ANTHROPIC_AUTH_TOKEN|CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC|CLAUDE_CODE_ATTRIBUTION_HEADER'` 做一次全仓配置片段扫尾。

**完成定义**：

- 三种片段与 `settings.json` 输出保持一致；
- 不影响已有 `ANTHROPIC_AUTH_TOKEN` 推荐路径。
- 生成片段不引入代理端 header rewrite 语义变化。
- README 与前端生成器中的推荐 env 不再互相矛盾。

---

### A2. 文档化推荐配置与风险边界

**目标**：把“为什么推荐 `ANTHROPIC_AUTH_TOKEN`，为什么不建议用户无说明切到 `ANTHROPIC_API_KEY`”写成显式文档。

**涉及文件**：

- 建议新增：`hook/docs/sub2api-claude-code-usage-guide.md`
- 建议同步：`README.md`、`README_CN.md`、`README_JA.md`
- 参考输入：`hook/docs/claude-code-telemetry-comparison-cliproxyapi-newapi-sub2api.md`

**动作**：

1. 写明标准用法；
2. 写明 `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` 的作用范围；
3. 写明 `CLAUDE_CODE_ATTRIBUTION_HEADER=0` 的作用；
4. 写明 `ANTHROPIC_AUTH_TOKEN` vs `ANTHROPIC_API_KEY` 的边界；
5. 写明 `DISABLE_TELEMETRY` vs `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC`：
   - `DISABLE_TELEMETRY` → no-telemetry；
   - `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` → essential-traffic，范围更大；
   - 标准模板不额外要求 `DISABLE_TELEMETRY`，不是漏配；
6. 写明 `/api/event_logging/batch` 只是兜底，不拦 Datadog / BigQuery metrics；
7. 写明 BigQuery metrics 的 cache fast-path 边界：
   - metrics export endpoint：`https://api.anthropic.com/api/claude_code/metrics`；
   - metrics opt-out endpoint：`https://api.anthropic.com/api/claude_code/organizations/metrics_enabled`；
   - `metricsStatusCache.enabled=true` 时可能绕过首次网络 opt-out 探测；
   - 用户本机若仍有 `ANTHROPIC_API_KEY`、`apiKeyHelper`、keychain/API key 来源，或手工改用 `ANTHROPIC_API_KEY`，需要重新评估；
   - sub2api 标准模板使用 `ANTHROPIC_AUTH_TOKEN`，不应被 `getAuthHeaders()` 当作 `x-api-key`；
8. 写明 forwarding 开关语义：
   - `fingerprintUnification=false` 只表示不套用统一 header fingerprint；
   - `metadataPassthrough=false` 仍会重写 `metadata.user_id`；
   - `session_id_masking_enabled` 会固定 / 伪装 session 段，应作为高级风险开关解释；
9. 写明响应头白名单过滤的意义：Claude Code 会把 `x-litellm-*`、`helicone-*`、`x-portkey-*`、`cf-aig-*`、`x-kong-*`、`x-bt-*` 等 gateway 痕迹纳入自身 gateway detection。
10. 写明 gateway host detection 边界：
    - Claude Code 还会检查部分已知 `ANTHROPIC_BASE_URL` provider-owned host suffix（当前源码确认的 Databricks suffix 等），不是任意自定义域名都会被 host suffix 规则命中；
    - 响应头过滤只能减少响应头 gateway 痕迹，不能把一个已命中 host suffix 的 base URL 变成“未命中”；
    - sub2api 文档不要承诺“完全隐藏代理”，只承诺降低可控响应头泄漏和默认遥测触发面。
11. 写明用户显式 3P OTEL 边界：
    - `CLAUDE_CODE_ENABLE_TELEMETRY` + `OTEL_*` 是用户主动配置的 OpenTelemetry 链路；
    - sub2api 标准模板不设置这些变量；
    - 如果用户自己开启，应按用户组织的 OTEL 目标评估，而不是归入 Anthropic 侧 1P telemetry。
12. 写明 `security.response_headers.additional_allowed` 风险：
    - 不建议允许已知 gateway 痕迹头；
    - 如新增 A5 的危险 override，必须说明开启后会影响 Claude Code gateway detection。
13. README / 多语言 README 同步时只放短版：
    - 标准 env 片段；
    - “为什么推荐 auth token”的一句话摘要；
    - 指向 `hook/docs/sub2api-claude-code-usage-guide.md` 的详细说明链接。

**完成定义**：

- 新文档可直接发给用户使用；
- 报告中的关键结论有落地说明，不再只停留在分析层。
- 管理员不会把 `ANTHROPIC_API_KEY`、`DISABLE_TELEMETRY`、`metadataPassthrough`、event sink 的边界误解成等价替代项。

---

### A3. 把缺失的条件头纳入默认内置白名单与 wire casing

**目标**：补齐当前报告里已经确认的默认缺口。

**涉及文件**：

- `backend/internal/service/gateway_service.go:351-374`
- `backend/internal/service/header_util.go:13-44`

**动作**：

将以下头加入默认内置支持：

- `x-claude-remote-container-id`
- `x-claude-remote-session-id`
- `x-client-app`
- `x-anthropic-additional-protection`

同时：

1. 在 `allowedHeaders` 中加入 lowercase key；
2. 在 `headerWireCasing` 中定义 wire casing；按当前 Claude Code 源码，这四个条件头均应保持 lowercase wire form：
   - `x-claude-remote-container-id`
   - `x-claude-remote-session-id`
   - `x-client-app`
   - `x-anthropic-additional-protection`
3. 补 `headerWireOrder` 的 debug 输出顺序；建议放在 `X-Claude-Code-Session-Id` 附近，便于和真实请求对比；
4. 确认 `/v1/messages` 与 `/v1/messages/count_tokens` 都能经过相同白名单。
5. 这些头只做“入站存在则透传”，不要合成默认值；其中 remote container/session 可能是稳定标识，默认日志不得输出原文。

**完成定义**：

- 默认内置逻辑不再无故丢掉这些真实条件头；
- 不影响现有普通 CLI 请求。

---

### A4. 调试日志与诊断输出安全基线

**目标**：避免为了复核 Claude Code 转发形态而在日志中引入新的敏感信息泄漏面。

**涉及文件**：

- `backend/internal/service/gateway_service.go:196-216`
- `backend/internal/service/gateway_service.go:247-317`
- `backend/internal/service/gateway_service.go:8875-8931`
- `backend/internal/util/logredact/*`（如需要复用）

**动作**：

1. 新增或修改 debug / observability 日志时，必须默认脱敏：
   - `Authorization`
   - `x-api-key`
   - `cookie`
   - OAuth / API token
   - 完整 `metadata.user_id`
   - 完整 request body / message content；
2. `x-client-request-id` 可以完整输出或 hash 输出；`metadata.user_id` 建议只输出 hash / session hash 前 8 位；
3. 现有 `SUB2API_DEBUG_GATEWAY_BODY` 属于显式危险调试开关，应在维护文档中标注“仅本地临时排障使用，不得在共享 / 生产环境开启”；
4. 如果后续要长期保留 full-body debug 文件，需增加 redaction 或改成默认只写摘要。

**完成定义**：

- 新增测试确认敏感 header 在日志中不会原样出现；
- 维护文档明确 full-body debug 的风险和关闭方式。

---

### A5. 响应头 gateway 痕迹过滤防绕过

**目标**：把“默认白名单能挡住 gateway 痕迹头”升级成更稳定的安全边界，避免管理员通过 `additional_allowed` 无意中把 Claude Code 会检测的 gateway 头重新放出去。

**涉及文件**：

- `backend/internal/util/responseheaders/responseheaders.go`
- `backend/internal/config/config.go`
- `backend/internal/config/config_test.go`
- `backend/internal/service/*response_headers*_test.go`

**动作**：

1. 增加已知 gateway response header 前缀 denylist：
   - `x-litellm-`
   - `helicone-`
   - `x-portkey-`
   - `cf-aig-`
   - `x-kong-`
   - `x-bt-`
2. denylist 应在 `additional_allowed` 之后仍然生效，避免误配置绕过；
3. 如果确实需要诊断透传这些头，设计显式危险开关，例如：
   - `security.response_headers.allow_gateway_trace_headers=false`
   - 默认 `false`
   - 该开关建议优先作为静态 config / env 级别的诊断开关，不强制接入 admin UI；若接入 UI，必须标红“开启后可能被 Claude Code gateway detection 识别”
4. denylist 必须按 lower-case prefix 判断：
   - 对 header key 先 `strings.ToLower(strings.TrimSpace(key))`；
   - 用 `strings.HasPrefix()` 判断；
   - 不要用 `force_remove` 的 exact-match 语义替代 prefix denylist；
   - 混合大小写 header 也必须被拦截。
5. `force_remove` 继续保留，但不要求管理员手工配置这些前缀；
6. 覆盖 Anthropic OAuth、API key passthrough、count_tokens、streaming / non-streaming 路径；
7. 保留 `x-request-id`、rate-limit、`retry-after` 等当前业务必需响应头。
8. 不新增“允许所有响应头”的旁路；`security.response_headers.enabled=false` 仍应保持默认白名单语义，而不是退化成全量 copy。

**完成定义**：

- 即使管理员把 `x-litellm-model-id` 之类加入 `additional_allowed`，默认仍不会透传；
- 除非显式开启危险 diagnostic override；
- 测试覆盖默认过滤、大小写混淆、`additional_allowed` 误放行、显式 override 四种情况。

---

### A6. 新增配置项、迁移与回滚策略先行

**目标**：避免 B2 / B3 这类行为型变更缺少统一开关和回滚路径。

**涉及文件**：

- `backend/internal/config/config.go`
- `backend/internal/handler/dto/settings.go`
- `backend/internal/handler/admin/setting_handler.go`
- `backend/internal/service/domain_constants.go`
- `backend/internal/service/setting_service.go`
- `backend/migrations/*`（如新增 DB setting / 默认值 seed）
- `frontend/src/api/admin/settings.ts`
- `frontend/src/views/admin/SettingsView.vue`
- 相关 API contract / settings 测试

**动作**：

0. 先冻结配置命名与存储边界，避免 T4/T7/T8 分叉：
   - 对外配置统一使用 snake_case / mapstructure 风格；
   - 不把 `Gateway.ClaudeRequestIDAutogenerateForAPIKeyPassthrough` 这类 Go 风格名称写进文档、UI 或 API；
   - 区分“静态 config/env”与“DB settings/admin UI”：
     - response header dangerous override 属安全诊断旁路，优先放静态 `security.response_headers.*`，不默认进入 admin UI；
     - request-id / beta policy 属运行时产品策略，可进入 DB settings / admin UI；
   - 如果采用嵌套结构，推荐：
     - `gateway.claude_request_id.auto_generate_oauth=true`
     - `gateway.claude_request_id.auto_generate_api_key_passthrough=false`
   - 如果为兼容现有 flat setting 风格而不嵌套，也必须在 P0-06 一次性 freeze，后续任务不得另起名字。
1. 为 `x-client-request-id` 自动生成设计明确开关：
   - OAuth / SetupToken normal path 可默认开启；
   - API key passthrough 默认关闭；
   - 自定义 base URL 永远以最终 URL helper 判断为准。
2. 为 beta policy 预设设计清晰存储结构：
   - 不要只在 UI 写死；
   - 现有 `beta_policy_settings` 是 DB setting；如扩展 preset，建议兼容旧 JSON：缺 `preset` 时按 `conservative` 解释，保留旧 `rules`；
   - 推荐 DTO 形态示例：`{ "preset": "conservative|claude_code_compat", "rules": [...] }`；
   - 后端要能独立返回当前策略；
   - 支持从旧 `beta_policy_settings` 平滑迁移或兼容读取。
3. 为 response header gateway denylist 设计危险 override，默认关闭；
4. 所有新增配置项必须具备：
   - 默认值；
   - 明确的 setting key / config key / 环境变量覆盖策略；
   - API contract；
   - 前端表单状态；
   - 保存 / 回显；
   - 回滚说明。
5. 对已有部署采取 fail-safe：
   - 无配置时走安全默认；
   - 配置解析失败时 fail-closed 或退回保守策略；
   - 不因为缺字段导致启动失败。
6. P0-06 的 UI 工作允许分层：
   - P0 必须完成后端默认值、API contract、持久化 / 回显和回滚方式；
   - 如果 UI 表单改动与 T8 冲突，可先交付后端配置 + 文档化配置方式，UI 合并到 T8 或 P2；
   - 但危险 override 不应只有“隐形配置”，至少需要管理员文档和 release note 标红。

**完成定义**：

- 新配置项在空库、旧库、已有 settings 三种情况下行为一致可解释；
- 管理员可以一键退回旧行为；
- API contract 测试覆盖新增字段。

---

## 阶段 B：P1 兼容性增强项（2~4 天）

### B1. 更新 `claude.DefaultHeaders` 静态基线

**目标**：减少明显过期的 header 基线，避免 mimic / fallback 过老。

**涉及文件**：

- `backend/internal/pkg/claude/constants.go:47-62`
- `backend/internal/service/gateway_service.go:5750-5765`
- `backend/internal/service/gateway_service.go:6080-6100`

**动作**：

1. 将 `claude.DefaultHeaders` 更新到与当前复核报告一致的较新基线；
2. 保持 `Anthropic-Dangerous-Direct-Browser-Access` 的默认处理逻辑与当前 SDK 认知一致；
3. 不盲目追最新未验证版本，更新时必须记录基线来源：
   - 真实 Claude Code 还原源码；
   - 本地抓包 / debug snapshot；
   - 现有 `header_util.go` 注释中的抓包版本；
   - migration / 监控模板中的版本串只能作为参考，不能单独作为事实来源；
4. 当前已知参考点需要在注释或维护文档中对齐说明：
   - `header_util.go` 注释：`claude-cli/2.1.81`
   - `backend/migrations/129_seed_claude_code_template.sql` 中监控模板写到 `claude-cli/2.1.114`，但它是“手工伪装模板”，只能作参考；
   - `claude.DefaultHeaders` 当前仍为 `claude-cli/2.1.22`、SDK `0.70.0`
5. 优先选择“已验证稳定”的 baseline，并配合已有 min/max Claude Code version 管理能力，避免版本串过新或过旧；
6. 若担心未来继续老化，可补一个集中注释：
   - 基线来源
   - 更新频率
   - 允许通过配置覆盖的字段

**完成定义**：

- 默认 fallback 不再使用明显过旧版本串；
- 不破坏 `applyClaudeOAuthHeaderDefaults()` 与 `applyClaudeCodeMimicHeaders()` 现有行为。
- 维护者能从代码注释或维护文档追溯 header baseline 的来源和更新边界。

---

### B2. 为 first-party 上游补可控的 `x-client-request-id` 自动生成

**目标**：让 sub2api 在“请求缺失时”更接近真实 first-party Claude Code。

**涉及文件**：

- `backend/internal/service/gateway_service.go:5583-5660`
- `backend/internal/service/gateway_service.go:8590-8660`
- 可能新增辅助函数：`backend/internal/service/gateway_service.go` 或拆到独立 helper

**动作**：

1. 新增精确 helper，例如：

```go
func isFirstPartyAnthropicMessagesURL(targetURL string) bool
```

2. helper 必须基于最终 `targetURL` 解析，而不是只看 account platform：
   - `scheme == "https"`；
   - `host == "api.anthropic.com"`；
   - `path == "/v1/messages"` 或 `path == "/v1/messages/count_tokens"`；
   - query 中的 `beta=true`、`proxy=...` 不影响 path 判断；
3. 仅在以下条件全部满足时生成：
   - `isFirstPartyAnthropicMessagesURL(targetURL) == true`；
   - 请求当前缺少 `x-client-request-id`（需用 `getHeaderRaw()`，兼容 canonical / wire casing）；
   - 路径为 `/v1/messages` 或 `/v1/messages/count_tokens`；
   - 当前路径属于 OAuth / SetupToken normal path，或显式开启了对应 feature flag；
4. 默认对 OAuth / SetupToken normal path 启用；
5. Anthropic API key passthrough 第一版建议只透传、不自动生成；若要启用，必须通过独立开关，例如：
   - `gateway.claude_request_id.auto_generate_api_key_passthrough=false`
   - 默认 `false`
6. 自定义中继 `buildCustomRelayURL()` 指向第三方域时不得生成；即使 account 是 Anthropic 类型，也必须以最终 URL host/path 为准；
7. `custom_base_url=https://api.anthropic.com` 可视为 first-party，但仍必须通过 helper 判定；
8. 生成值使用 UUID，写入 wire casing `x-client-request-id`。
9. 生成时机应放在白名单透传、fingerprint 应用、mimic headers、beta policy 处理之后、真正发出请求之前；这样判断的是最终有效 header，避免先生成又被后续逻辑覆盖或重复。
10. 代理端生成的 request id 只用于上游请求形态与服务端排障，不等价于 Claude Code 客户端自己生成的 ID；文档不要承诺用户本地 Claude Code 日志一定能看到该 ID。
11. 如需观测，只记录“是否生成 / 跳过原因 / hash 前缀”，不要把该 ID 与完整 `metadata.user_id`、完整 body 绑定输出到默认日志。

**完成定义**：

- first-party Anthropic 普通路径在缺失 header 时可自动补齐；
- 非 first-party 上游不受影响。
- API key passthrough 的 auth-only / transparent 边界不会被默认生成逻辑打破。

---

### B3. 提供“Claude Code 兼容优先”的 beta policy 预设

**目标**：把默认过滤策略从“隐式平台判断”变成“显式产品策略”。

**涉及文件**：

- `backend/internal/service/settings_view.go:383-397`
- `backend/internal/service/gateway_service.go:5841-5928`
- `backend/internal/service/setting_service.go`
- `backend/internal/handler/dto/settings.go`
- `backend/internal/handler/admin/setting_handler.go`
- `frontend/src/api/admin/settings.ts`
- `frontend/src/views/admin/SettingsView.vue`

**动作**：

1. 设计至少两档策略：
   - 保守默认（当前逻辑）
   - Claude Code 兼容优先
2. 明确哪些 beta 会在兼容模式下放行：
   - `fast-mode-2026-02-01`
   - `context-1m-2025-08-07`
3. 在 UI 或文档中提示风险：成本、配额、兼容性。
4. 不把“兼容优先”作为无提示默认值；默认策略继续保守，兼容策略必须显式开启或显式选择。
5. 兼容策略需要和 min/max Claude Code version 文档联动：如果管理员锁定较旧 CLI 版本，不应默认放行较新版本才会使用的 beta。

**完成定义**：

- 管理员可以显式选择，而不是只能接受隐式过滤；
- 文档能解释为什么默认值与兼容值不同。

---

## 阶段 C：P2 稳定化与验证闭环（2~3 天）

### C1. 扩充测试矩阵

**目标**：把这次报告里的关键边界固化成测试，避免回归。

**建议新增 / 更新测试点**：

1. `UseKeyModal` 输出包含四个关键 env；
2. 新条件头能透传；
3. `headerWireCasing` 对新增条件头生效；
4. first-party `x-client-request-id` 缺失时自动生成；
5. 非 first-party 上游不生成 `x-client-request-id`；
6. beta policy 两档预设行为不同；
7. API key passthrough 不被新逻辑误伤；
8. `count_tokens` 路径与 `/v1/messages` 逻辑一致。
9. `isFirstPartyAnthropicMessagesURL()` 覆盖：
   - `https://api.anthropic.com/v1/messages?beta=true` → true
   - `https://api.anthropic.com/v1/messages/count_tokens?beta=true` → true
   - `http://api.anthropic.com/...` → false
   - `https://api.anthropic.com.evil/...` → false
   - `https://example.com/v1/messages` → false
   - 自定义 relay URL 到第三方域 → false
10. 响应头白名单过滤不会透传 gateway 痕迹：
    - `x-litellm-*`
    - `helicone-*`
    - `x-portkey-*`
    - `cf-aig-*`
    - `x-kong-*`
    - `x-bt-*`
11. Anthropic API key passthrough 也必须验证响应头过滤仍生效；
12. `additional_allowed` 即使包含已知 gateway header，默认也不能透传；只有显式危险 override 开启时才允许；
    - 混合大小写，如 `X-LiteLLM-Model-Id` / `CF-AIG-Trace-Id`，也必须被 lower-case prefix denylist 拦截；
    - `force_remove` exact match 不能作为 prefix denylist 的唯一实现。
13. debug / diagnostic 日志：
    - `Authorization`、`x-api-key` 不出现明文；
    - 完整 `metadata.user_id` 不进入默认日志；
    - remote container/session 条件头不进入默认日志原文；
    - full-body debug 只在显式 `SUB2API_DEBUG_GATEWAY_BODY` 下启用；
14. frontend 已有 Vitest 基建，优先补组件 / 纯函数测试；如果抽 `generateAnthropicFiles()` 难度更低，可以先把 env snippet 生成逻辑抽成纯函数并单测，再由组件调用。
15. 配置 / API contract：
    - 新增 request-id 自动生成开关有默认值；
    - beta policy preset 能保存 / 回显 / 兼容旧配置；
    - response header gateway dangerous override 默认关闭；
    - 旧 settings 数据缺字段时不 panic、不改变安全默认。
16. 用户显式 OTEL 文档测试 / 静态检查：
    - 默认生成片段中不得出现 `CLAUDE_CODE_ENABLE_TELEMETRY`；
    - 默认生成片段中不得出现 `OTEL_`；
    - 文档必须把 3P OTEL 与 Anthropic 侧 telemetry 分开描述。
17. README / 多语言 README 静态检查：
    - 出现 `ANTHROPIC_AUTH_TOKEN` 的 Claude Code 示例，应同时说明或包含 `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`；
    - 推荐片段应包含 `CLAUDE_CODE_ATTRIBUTION_HEADER=0`，除非该段明确不是 Claude Code 用法。
18. 文档边界静态检查：
    - 使用指南必须说明部分已知 `ANTHROPIC_BASE_URL` provider-owned host suffix 也可能参与 Claude Code gateway detection；
    - 不得出现“完全隐藏代理 / 完全抑制所有遥测”的绝对化承诺。

**涉及文件**：

- `backend/internal/service/*_test.go`
- `frontend/src/components/keys/*` 测试文件，或新增 env snippet builder 纯函数测试

**完成定义**：

- 关键路径均有自动化回归；
- 文末的回归命令可作为 CI 子集运行。
- 前端 env snippet 至少有 Vitest / snapshot / 纯函数单测中的一种自动化覆盖。
- 新增配置项具备默认值、API contract、旧配置兼容和回滚测试。

---

### C2. 把报告沉淀为用户文档 + 维护文档

**目标**：避免未来同样问题还要重新从源码考古。

**建议产出两类文档**：

1. **用户文档**：怎么正确接 Claude Code 到 sub2api；
2. **维护文档**：
   - 哪些头是普通 CLI 基线；
   - 哪些是条件头；
   - `x-client-request-id` 何时自动生成；
   - beta policy 为什么这么设计；
   - 为什么推荐 `ANTHROPIC_AUTH_TOKEN`。
   - header baseline 来源、抓包版本、更新频率；
   - `DISABLE_TELEMETRY` 与 `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` 的优先级；
   - BigQuery metrics cache fast-path 和本机 API key 残留风险；
   - `fingerprintUnification` / `metadataPassthrough` / `session_id_masking_enabled` 的差异；
   - response header filter 与 Claude Code gateway detection 的对应关系；
   - `SUB2API_DEBUG_GATEWAY_BODY` / mimic debug 的安全使用边界。

**建议路径**：

- 用户文档：`hook/docs/sub2api-claude-code-usage-guide.md`
- 维护文档：`hook/docs/sub2api-claude-code-maintenance-notes.md`

**完成定义**：

- 下次升级 Claude Code 或修复兼容性时，维护者有稳定基线可参考。

---

### C3. 灰度、回滚与观测清单

**目标**：把新增行为从“代码改完”变成可安全上线的产品能力。

**动作**：

1. 灰度顺序：
   - 先上线文档 / 模板 / 白名单；
   - 再上线 response header denylist；
   - 再开启 OAuth normal path 的 `x-client-request-id` 自动生成；
   - 最后开放 beta policy 兼容预设。
2. 回滚开关：
   - `x-client-request-id` 自动生成可关闭；
   - beta policy 可退回保守预设；
   - gateway response header dangerous override 默认关闭，出现兼容问题时优先用临时诊断而非永久放开；
   - `claude.DefaultHeaders` 更新后保留配置覆盖入口。
3. 观测指标 / 日志只记录摘要：
   - request-id 自动生成次数；
   - 非 first-party 跳过次数；
   - beta policy 被过滤 / 放行的 token 名称；
   - response header gateway denylist 命中次数；
   - 不记录 token、完整 body、完整 `metadata.user_id`。
   - 指标 label 必须低基数：不要把 account name、完整 request id、完整 header value、完整 model variant 放进 label。
4. 发布说明：
   - 标注 shell 片段新增 `CLAUDE_CODE_ATTRIBUTION_HEADER=0`；
   - 标注 `ANTHROPIC_AUTH_TOKEN` 与本机 API key 残留边界；
   - 标注 beta policy 新预设不会自动切换既有配置，除非管理员选择。

**完成定义**：

- 每个行为型改动都有关闭方式；
- 出现上游兼容问题时可以快速定位并回退；
- 观测不引入新的敏感信息泄漏面。

---

## 6. 风险与缓解

### 风险 1：新增条件头导致第三方上游兼容性下降

**缓解**：

- 仅加入默认白名单，不强制生成；
- `x-client-request-id` 仅 first-party Anthropic 上游自动生成；
- 新逻辑必须覆盖自定义 base URL 回归测试。
- 条件头白名单和主动生成行为分开实现：白名单默认启用，生成逻辑用 first-party helper / 开关保护。

### 风险 2：更新 `claude.DefaultHeaders` 后影响旧环境稳定性

**缓解**：

- 先更新到保守的新基线，不一步追最新所有字段；
- 保持配置覆盖入口可用；
- 通过版本边界控制功能兜底。
- 在注释 / 维护文档中记录 baseline 来源，避免后续维护者把“最新版本串”直接硬编码进去。

### 风险 3：beta 兼容预设带来成本 / 配额变化

**缓解**：

- 默认值继续保守；
- 兼容模式显式开启；
- 文档注明适用场景。

### 风险 4：为了更像 Claude Code，引入过多 synthetic 改写

**缓解**：

- 本计划只做“默认路径补齐”和“条件头补白名单”；
- 不扩大 fake identity / 深度 body rewrite 范围；
- 不把 TLS 指纹改成默认全局开启。

### 风险 5：`x-client-request-id` 被误加到第三方上游或 passthrough 路径

**缓解**：

- 使用 `isFirstPartyAnthropicMessagesURL(targetURL)` 精确判断最终 URL；
- 不用 account 类型、platform、模型名推断 first-party；
- API key passthrough 第一版默认不自动生成，除非管理员显式开启；
- 覆盖 third-party custom base URL、custom relay、count_tokens 的单测。

### 风险 6：遥测文档让用户误以为 sink 可以拦所有流量

**缓解**：

- 文档明确 `/api/event_logging/batch` 是兜底；
- 明确 Datadog 固定直连 Datadog intake，BigQuery metrics 是独立 endpoint；
- 标准路径强调 `ANTHROPIC_AUTH_TOKEN + CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`；
- 明确本机残留 Anthropic API key / cache fast-path 会改变 BigQuery metrics 边界。

### 风险 7：调试日志泄漏 token、metadata 或用户正文

**缓解**：

- 新增日志必须经过 redaction；
- 默认诊断日志只输出摘要 / hash；
- full-body debug 只作为显式本地排障工具，并在文档中标注风险；
- 增加日志脱敏单测。

### 风险 8：UseKeyModal 组件耦合较重导致 A1 难以自动验收

**缓解**：

- 优先抽出 env snippet builder 纯函数；
- 使用现有 Vitest 补组件测试 / snapshot；
- 如果组件挂载成本过高，先测纯函数并保留最小人工验证清单和静态 grep 检查。

### 风险 9：管理员通过 `additional_allowed` 误放行 gateway 痕迹头

**缓解**：

- 增加内置 gateway prefix denylist，优先级高于 `additional_allowed`；
- 如必须放行，使用显式危险 override；
- UI / 文档标注开启后可能被 Claude Code gateway detection 记录；
- 测试覆盖误配置和 override 两种情况。

### 风险 10：新增配置项无迁移 / 无回滚，导致旧部署行为不可预测

**缓解**：

- 所有新增设置提供默认值和旧数据兼容；
- API contract 测试覆盖缺字段情况；
- 后端解析失败时退回保守 / 安全默认；
- 发布说明列出每个新行为的回滚开关。

### 风险 11：用户显式开启 3P OTEL 后误以为 sub2api 会阻断

**缓解**：

- 标准模板不设置 `CLAUDE_CODE_ENABLE_TELEMETRY` / `OTEL_*`；
- 文档把用户 3P OTEL 与 Anthropic 侧 telemetry 分开；
- 静态检查确认默认片段不包含这些变量。

### 风险 12：代理生成的 `x-client-request-id` 被误解为客户端可见关联 ID

**缓解**：

- 文档明确：sub2api 自动生成的是上游请求 header，Claude Code 客户端本地日志不一定知道该值；
- 默认日志只记录生成事件和短 hash，不输出完整 ID 与用户内容绑定；
- 如果未来要把该 ID 回传给客户端，必须单独设计响应头 / 错误体兼容策略，并验证不会引入新的 fingerprint 或信息泄漏。

### 风险 13：只改前端生成器，README / 文档仍保留旧配置

**缓解**：

- A1 必须同步更新 README / 多语言 README / hook docs；
- 增加静态 grep 检查，发现 Claude Code 推荐片段缺 `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` 或 `CLAUDE_CODE_ATTRIBUTION_HEADER=0` 时失败；
- 发布说明提示用户从旧片段迁移到新推荐配置。

### 风险 14：响应头过滤被误解为可以消除所有 gateway detection

**缓解**：

- 文档明确 Claude Code 还会检查部分已知 `ANTHROPIC_BASE_URL` provider-owned host suffix（当前确认 Databricks suffix 等），不要泛化成任意自定义域名都会命中；
- response header denylist 只承诺清理可控响应头痕迹；
- 如果部署域名本身命中已知 host suffix，不能通过 A5 消除该命中；
- 不在营销 / README 中写“完全隐藏代理”类绝对化表述。

### 风险 15：多子代理并行导致配置命名或热点文件冲突

**缓解**：

- 执行前先完成 T-1：冻结配置 key、DTO 字段、文件 owner、测试命令；
- `gateway_service.go` 同一时间只允许一个 owner 修改，其他任务通过 helper 文件或 patch 建议交付；
- settings DTO / handler / `SettingsView.vue` 由 T5/T8 约定唯一 owner；
- 每个任务完成回报必须列出 changed files、tests、rollback、follow-ups，便于主控集成。

---

## 7. 验证步骤

### 7.1 代码级验证

执行：

```bash
cd backend
go test ./internal/service ./internal/handler -run 'ClaudeCode|GatewayBeta|Metadata|SessionIDMasking|Header|BetaPolicy'
go test ./internal/service -run 'AnthropicAPIKeyPassthrough|ResponseHeaders|Privacy|AntigravityPrivacy|OpenAIPrivacy'
```

如新增测试：

```bash
cd backend
go test ./internal/service -run 'ClaudeRequestID|ClaudeConditionalHeaders|ClaudeDefaults'
```

建议同时补充以下定向测试 / 静态检查：

```bash
cd backend
go test ./internal/service -run 'FirstPartyAnthropic|ResponseHeaders|AnthropicAPIKeyPassthrough|LogRedaction'
go test ./internal/config ./internal/server -run 'ResponseHeaders|Settings|APIContract'
```

```bash
rg -n 'CLAUDE_CODE_ATTRIBUTION_HEADER|CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC|ANTHROPIC_AUTH_TOKEN' ../frontend/src/components/keys
rg -n 'ANTHROPIC_AUTH_TOKEN|CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC|CLAUDE_CODE_ATTRIBUTION_HEADER' ../README.md ../README_CN.md ../README_JA.md ../hook/docs
if rg -n 'CLAUDE_CODE_ENABLE_TELEMETRY|OTEL_' ../frontend/src/components/keys; then
  echo "unexpected telemetry env in default Claude Code snippet" >&2
  exit 1
fi
```

```bash
cd frontend
pnpm test:run -- UseKeyModal
```

### 7.2 手工验证

1. 用前端生成 Claude Code 配置，确认 shell/CMD/PowerShell 与 `settings.json` 都包含期望 env；
2. 使用普通 Claude Code CLI 发请求，检查：
   - `x-app`
   - `User-Agent`
   - `X-Claude-Code-Session-Id`
   - `x-client-request-id`（仅 first-party Anthropic）
3. 开启 Remote / Agent SDK 相关 env，确认条件头不会在代理侧丢失；
4. 分别验证：
   - OAuth 主路径
   - API key passthrough
   - `/v1/messages/count_tokens`
5. 通过日志或抓包检查响应头白名单过滤是否仍正常工作。
6. 用本地测试上游返回 gateway 痕迹头，确认客户端响应中不会出现：
   - `x-litellm-model-id`
   - `helicone-id`
   - `x-portkey-trace-id`
   - `cf-aig-*`
   - `x-kong-*`
   - `x-bt-*`
7. 分别验证：
   - first-party `https://api.anthropic.com/v1/messages` 缺失 `x-client-request-id` 时会补；
   - custom base URL 指向第三方域时不会补；
   - API key passthrough 默认不会因 B2 改动而多出新自动生成 header；
8. 开启 debug 日志时确认 token 已脱敏；不开启 `SUB2API_DEBUG_GATEWAY_BODY` 时不写完整 body。

---

## 8. 建议排期

### 方案：一周内完成的最小闭环

**Day 1**
- A1 用法模板统一
- A2 用户文档初稿
- A4 日志 / debug 安全基线检查
- A6 新增配置项 / 回滚策略草案

**Day 2**
- A3 条件头白名单 + wire casing
- A5 响应头 gateway denylist 防绕过
- 对应测试

**Day 3-4**
- B1 更新 `claude.DefaultHeaders`
- B2 `x-client-request-id` 自动生成
- 对应测试

**Day 5**
- B3 beta policy 兼容预设
- C1 回归测试补齐

**Day 6-7**
- C2 维护文档
- C3 灰度 / 回滚 / 观测清单
- 手工回归与灰度说明

---

## 9. 建议执行顺序

如果只做最值得落地的一版，推荐顺序如下：

1. **A1 + A2**：先把用户入口修正到默认正确；
2. **A4 + A6**：先立住日志安全和配置回滚边界，避免后续验证时扩大泄漏面；
3. **A3 + A5**：补默认内置条件头支持，同时加固响应头 gateway 痕迹过滤；
4. **B1**：更新静态 header 基线；
5. **B2**：补 first-party `x-client-request-id` 自动生成；
6. **B3**：把 beta 兼容策略显式化；
7. **C1 + C2 + C3**：收口成测试、文档、灰度回滚闭环。

---

## 10. 完成后的交付物

本计划完成后，仓库内应至少新增或更新：

1. `frontend/src/components/keys/UseKeyModal.vue`
2. `backend/internal/service/gateway_service.go`
3. `backend/internal/service/header_util.go`
4. `backend/internal/pkg/claude/constants.go`
5. `backend/internal/util/responseheaders/responseheaders.go`
6. `backend/internal/config/config.go`
7. `backend/internal/service/settings_view.go`（如做 beta 兼容预设）
8. `backend/internal/handler/dto/settings.go` / `backend/internal/handler/admin/setting_handler.go`（如新增设置项）
9. `frontend/src/views/admin/SettingsView.vue` / `frontend/src/api/admin/settings.ts`（如新增 UI 设置）
10. 相关测试文件
11. `hook/docs/sub2api-claude-code-usage-guide.md`
12. `hook/docs/sub2api-claude-code-maintenance-notes.md`
13. 可能新增的 helper / 测试：
   - `isFirstPartyAnthropicMessagesURL()` 或等价 helper
   - `*_claude_request_id_test.go`
   - `*_response_headers_test.go`
   - `*_log_redaction_test.go`
14. 新增 / 更新 `UseKeyModal` 或 env snippet builder 的 Vitest 单测 / snapshot。
15. 发布 / 回滚说明片段。

---

## 11. P0 / P1 / P2 Issue 列表

> 说明：本节把前文 A/B/C 阶段拆成可建 issue 的粒度。每个 issue 都应独立有验收标准、测试命令和回滚说明；如果多个 Codex / 子代理并行执行，必须按“文件归属”避免写同一批文件。

### 11.0 全局 Issue 执行约束 / DoD

每个 issue 完成时必须回报以下固定字段，缺一项不算 ready for merge：

- **Summary**：改了什么，是否改变默认行为；
- **Files changed**：实际修改文件列表；
- **Config / migration**：新增或变更的配置 key、默认值、迁移 / 兼容策略；
- **Tests**：执行过的命令与结果；如果未执行，说明原因；
- **Rollback**：如何退回旧行为或关闭新行为；
- **Known risks / follow-ups**：剩余风险、后续 issue、人工验证点。

并行前必须先完成：

- 配置命名 freeze：request-id、response header dangerous override、beta policy preset；
- 文件 owner freeze：`gateway_service.go`、settings DTO / handler、`SettingsView.vue`、usage guide；
- 测试包路径确认：例如 response header util 当前包路径是 `backend/internal/util/responseheaders`，命令应以实际 package 为准；
- 文档事实边界确认：不承诺“完全隐藏代理 / 完全抑制所有遥测”。

### 11.1 P0 Issues：默认安全与推荐路径修正

| Issue ID | 标题 | 范围 | 主要文件 | 验收要点 | 依赖 |
|---|---|---|---|---|---|
| P0-00 | 执行前置门禁：配置命名与文件 owner freeze | 编排 / 任务治理 | 本计划、issue tracker | 冻结配置 key、DTO 字段、文件 owner、测试包路径、回报格式；明确 T2/T3/T7/T8 不并行写 `gateway_service.go` | 无 |
| P0-01 | Claude Code 用法模板补齐 attribution off | 前端生成 shell / CMD / PowerShell / settings 片段 | `frontend/src/components/keys/UseKeyModal.vue`、相关测试 | 四类片段均包含 `ANTHROPIC_AUTH_TOKEN`、`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`、`CLAUDE_CODE_ATTRIBUTION_HEADER=0`；不加入 `DISABLE_TELEMETRY` 默认项 | 无 |
| P0-02 | 用户文档 / README 风险边界同步 | 用户可读文档、多语言 README | `hook/docs/sub2api-claude-code-usage-guide.md`、`README*.md` | 解释 auth token、nonessential、attribution、event sink、Datadog、BigQuery cache、本机 API key 残留、3P OTEL、已知 gateway host suffix；README 只放短入口并链接长文档 | P0-01 |
| P0-03 | Remote / Agent SDK 条件头白名单与 wire casing | 请求头透传底座 | `backend/internal/service/gateway_service.go`、`backend/internal/service/header_util.go`、测试 | 四个条件头只透传不合成；messages / count_tokens 一致；wire casing 为 lowercase | 无 |
| P0-04 | debug / diagnostic 日志脱敏基线 | 日志安全 | `backend/internal/service/gateway_service.go`、`backend/internal/util/logredact/*`、测试 | token、cookie、完整 body、完整 `metadata.user_id`、remote session/container 不进默认日志；`SUB2API_DEBUG_GATEWAY_BODY` 文档化为危险本地开关 | 无 |
| P0-05 | 响应头 gateway 痕迹 denylist 防绕过 | 响应头过滤 | `backend/internal/util/responseheaders/responseheaders.go`、测试；config 字段由 P0-06 owner 合入 | `additional_allowed` 也不能默认放行 `x-litellm-*` 等；危险 override 默认关闭；OAuth / passthrough / count_tokens / streaming 全覆盖 | P0-00；危险 override schema 依赖 P0-06 |
| P0-06 | 新配置项、迁移、回滚策略底座 | 静态 config + DB settings/API contract/前端设置入口 | `backend/internal/config/config.go`、`setting_service.go`、settings DTO / handler、`SettingsView.vue` | 新行为都有默认值、回显/保存策略、旧配置兼容、回滚路径；缺字段不 panic；安全诊断开关默认不进 UI | P0-00；P0-05 / P1-02 / P1-03 需要对齐 |

### 11.2 P1 Issues：Claude Code 兼容性增强

| Issue ID | 标题 | 范围 | 主要文件 | 验收要点 | 依赖 |
|---|---|---|---|---|---|
| P1-01 | 更新 `claude.DefaultHeaders` 稳定基线 | 静态 fallback / mimic header | `backend/internal/pkg/claude/constants.go`、相关测试 / 维护文档 | 不再使用 `claude-cli/2.1.22` / SDK `0.70.0`；记录来源；不盲追未验证最新版 | P0-04 文档化日志安全后更易验证 |
| P1-02 | first-party `x-client-request-id` 自动生成 | OAuth / SetupToken normal path | `gateway_service.go`、可能新增 helper、配置 / 测试 | 基于最终 URL helper 判断 first-party；OAuth normal path 缺失时按 frozen 开关生成；第三方 / custom relay 不生成；API key passthrough 默认不生成 | P0-06 |
| P1-03 | Claude Code 兼容优先 beta policy 预设 | beta policy 后端 / UI / 文档 | `settings_view.go`、`gateway_service.go`、settings API / UI | 保守默认不变；兼容预设显式开启；`fast-mode` / `context-1m` 行为可测试；旧设置兼容 | P0-06 |
| P1-04 | 行为型变更观测摘要 | 低敏指标 / debug 摘要 | metrics / logger 相关文件 | 只记录低基数、低敏摘要；不记录完整 request id、token、body、metadata | P0-04、P0-05、P1-02、P1-03 |

### 11.3 P2 Issues：测试、文档、灰度闭环

| Issue ID | 标题 | 范围 | 主要文件 | 验收要点 | 依赖 |
|---|---|---|---|---|---|
| P2-01 | 自动化测试矩阵补齐 | 后端 / 前端测试 | `backend/internal/service/*_test.go`、`frontend/src/**.test.*` | 覆盖 UseKeyModal、条件头、request-id、beta policy、response header denylist、passthrough、log redaction、旧配置兼容 | P0/P1 实现 |
| P2-02 | 维护文档沉淀 | 维护者文档 | `hook/docs/sub2api-claude-code-maintenance-notes.md` | header baseline、条件头、request-id helper、beta policy、metadata 开关、response header filter、debug 安全边界 | P0/P1 实现 |
| P2-03 | 灰度 / 回滚 / 发布说明 | 上线闭环 | release note / docs / plan | 每个行为型改动有关闭方式；发布说明写明新增 env、auth token 边界、beta policy 不自动切换 | P0/P1 实现 |
| P2-04 | 端到端验证与最终收口 | 测试命令、手工验证 | 无固定文件，必要时补 QA 文档 | 后端指定 go test、前端 Vitest、静态 grep、手工三路径验证通过 | P2-01~P2-03 |

---

## 12. 逐项实施 Checklist

### 12.0 P0-00：执行前置门禁

- [ ] 冻结 request-id 配置命名：
  - [ ] OAuth normal path 默认值；
  - [ ] API key passthrough 默认值；
  - [ ] 环境变量 / config key / DTO 字段名。
- [ ] 冻结 response header dangerous override 命名，默认必须关闭。
- [ ] 冻结 beta policy preset DTO / DB setting 兼容策略：
  - [ ] 旧 `beta_policy_settings` 缺 `preset` 时按 `conservative` 解释；
  - [ ] 旧 `rules` 保留，不被迁移清空。
- [ ] 指定热点文件 owner：
  - [ ] `backend/internal/service/gateway_service.go`
  - [ ] settings DTO / handler / service
  - [ ] `frontend/src/views/admin/SettingsView.vue`
  - [ ] `hook/docs/sub2api-claude-code-usage-guide.md`
- [ ] 确认测试命令中的 package 路径存在；如 package 名不匹配，以实际路径替换。
- [ ] 建立每个任务的完成回报模板：`Summary / Files changed / Config / Tests / Rollback / Known risks`。

### 12.1 P0-01：Claude Code 用法模板补齐

- [ ] 确认 `UseKeyModal.vue` 当前 Unix / CMD / PowerShell 片段只缺 `CLAUDE_CODE_ATTRIBUTION_HEADER=0`。
- [ ] 在三种 shell 片段中加入 attribution off：
  - [ ] Unix：`export CLAUDE_CODE_ATTRIBUTION_HEADER=0`
  - [ ] CMD：`set CLAUDE_CODE_ATTRIBUTION_HEADER=0`
  - [ ] PowerShell：`$env:CLAUDE_CODE_ATTRIBUTION_HEADER=0`
- [ ] 保持 `settings.json` 示例已有 env 不退化。
- [ ] 不新增默认 `DISABLE_TELEMETRY`。
- [ ] 补 Vitest / snapshot / 纯函数测试。
- [ ] 运行：

```bash
cd frontend
pnpm test:run -- UseKeyModal
```

### 12.2 P0-02：用户文档 / README 同步

- [ ] 新增 / 更新 `hook/docs/sub2api-claude-code-usage-guide.md`。
- [ ] 同步 `README.md`、`README_CN.md`、`README_JA.md` 中 Claude Code 推荐片段。
- [ ] 明确标准推荐：`ANTHROPIC_AUTH_TOKEN + CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 + CLAUDE_CODE_ATTRIBUTION_HEADER=0`。
- [ ] 明确不默认推荐 `ANTHROPIC_API_KEY`。
- [ ] 明确 `DISABLE_TELEMETRY` 与 nonessential 的层级差异。
- [ ] 明确 `/api/event_logging/batch` 只是兜底，不能拦 Datadog / BigQuery metrics。
- [ ] 明确 BigQuery `metricsStatusCache.enabled=true` 与本机 API key 残留边界。
- [ ] 明确 3P OTEL 是用户显式配置，不是 Anthropic 侧 telemetry。
- [ ] 明确部分已知 `ANTHROPIC_BASE_URL` provider-owned host suffix 也可能参与 gateway detection，响应头过滤不承诺完全隐藏代理。
- [ ] README / README_CN / README_JA 只放短版配置和链接，不复制整份长说明。
- [ ] 静态检查：

```bash
rg -n 'ANTHROPIC_AUTH_TOKEN|CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC|CLAUDE_CODE_ATTRIBUTION_HEADER|ANTHROPIC_API_KEY|DISABLE_TELEMETRY|OTEL_' README* hook/docs
```

### 12.3 P0-03：条件头白名单与 wire casing

- [ ] 在 `allowedHeaders` 加入：
  - [ ] `x-claude-remote-container-id`
  - [ ] `x-claude-remote-session-id`
  - [ ] `x-client-app`
  - [ ] `x-anthropic-additional-protection`
- [ ] 在 `headerWireCasing` 加入上述 lowercase wire form。
- [ ] 在 `headerWireOrder` 加入 debug 输出顺序。
- [ ] 确认 `/v1/messages` 与 `/v1/messages/count_tokens` 都走同一白名单逻辑。
- [ ] 确认这些头只透传，不合成默认值。
- [ ] 补单测：存在即透传、缺失不生成、wire casing 正确。
- [ ] 运行：

```bash
cd backend
go test ./internal/service -run 'Header|ClaudeConditionalHeaders'
```

### 12.4 P0-04：日志脱敏基线

- [ ] 盘点默认日志、mimic debug、gateway snapshot debug 的输出内容。
- [ ] 默认日志不得输出：
  - [ ] `Authorization`
  - [ ] `x-api-key`
  - [ ] `cookie`
  - [ ] 完整 `metadata.user_id`
  - [ ] 完整 request body / message content
  - [ ] remote container/session 原文
- [ ] 对 `buildClaudeMimicDebugLine()` 中 `metadata.user_id` 改为 hash / 短摘要。
- [ ] 对新增条件头的日志输出做 hash / 不输出。
- [ ] 文档标注 `SUB2API_DEBUG_GATEWAY_BODY` 只限本地临时排障。
- [ ] 补 `LogRedaction` 测试。
- [ ] 运行：

```bash
cd backend
go test ./internal/service -run 'LogRedaction|ClaudeMimicDebug'
```

### 12.5 P0-05：响应头 gateway denylist

- [ ] 在 response header filter 增加 gateway prefix denylist。
- [ ] denylist 实现为 lower-case prefix match，不用 exact-match `force_remove` 替代。
- [ ] denylist 优先级高于 `additional_allowed`。
- [ ] 保留业务允许头：`x-request-id`、rate-limit、`retry-after` 等。
- [ ] 如设计危险 override，默认必须为 `false`。
- [ ] dangerous override 的 config 字段由 P0-06 / T5 owner 合入；P0-05 只消费已冻结字段，避免并行改 `config.go`。
- [ ] 覆盖 streaming / non-streaming / passthrough / count_tokens。
- [ ] 补测试：
  - [ ] 默认过滤 gateway 痕迹；
  - [ ] 混合大小写 gateway header 仍被过滤；
  - [ ] `additional_allowed` 误放行仍被 denylist 拦截；
  - [ ] 危险 override 开启后才允许；
  - [ ] passthrough 响应仍过滤。
- [ ] 运行：

```bash
cd backend
go test ./internal/service ./internal/util/... -run 'ResponseHeaders|AnthropicAPIKeyPassthrough'
```

### 12.6 P0-06：配置、迁移、回滚底座

- [ ] 明确新增配置项名称、默认值和存储位置。
- [ ] request-id 自动生成：
  - [ ] 使用 snake_case 配置名，例如 `gateway.claude_request_id.auto_generate_oauth`；
  - [ ] OAuth normal path 默认开启或配置开启；
  - [ ] API key passthrough 默认关闭；
  - [ ] 旧配置缺字段时行为确定。
- [ ] beta policy preset：
  - [ ] 旧 `beta_policy_settings` 缺 `preset` 时按 `conservative`；
  - [ ] 后端可独立保存 / 回显；
  - [ ] UI 不写死唯一事实；
  - [ ] 兼容旧 `beta_policy_settings`。
- [ ] response header dangerous override 默认关闭。
- [ ] response header dangerous override 优先作为静态 `security.response_headers.*` config/env，不默认接入 admin UI；如确需 UI，单独标红并加权限 / 审计说明。
- [ ] 补 API contract 测试。
- [ ] 写回滚说明。

### 12.7 P1-01：更新 `claude.DefaultHeaders`

- [ ] 确认选用 baseline 来源。
- [ ] 更新 `User-Agent`、`X-Stainless-Package-Version` 等明显过旧字段。
- [ ] 保持 `anthropic-dangerous-direct-browser-access` 与当前 SDK 行为一致。
- [ ] 在代码注释 / 维护文档记录来源与更新边界。
- [ ] 补测试确认默认值不再是 `2.1.22` / `0.70.0`。

### 12.8 P1-02：first-party request-id 自动生成

- [ ] 新增 `isFirstPartyAnthropicMessagesURL(targetURL string) bool` 或等价 helper。
- [ ] helper 只接受：
  - [ ] `https://api.anthropic.com/v1/messages`
  - [ ] `https://api.anthropic.com/v1/messages/count_tokens`
- [ ] helper 拒绝：
  - [ ] `http://api.anthropic.com/...`
  - [ ] `https://api.anthropic.com.evil/...`
  - [ ] 第三方 custom relay
- [ ] 在 OAuth / SetupToken normal path 缺失 `x-client-request-id` 时生成 UUID。
- [ ] 默认值按 P0-06 freeze 结果执行；若争议未解决，先以“开关存在、灰度开启”为准，不在首版无回滚地强改默认行为。
- [ ] API key passthrough 默认只透传，不自动生成。
- [ ] 补 messages / count_tokens 测试。

### 12.9 P1-03：beta policy 兼容预设

- [ ] 保守默认保持当前行为。
- [ ] 新增“Claude Code 兼容优先”预设。
- [ ] 明确 `fast-mode-2026-02-01`、`context-1m-2025-08-07` 在兼容预设下的行为。
- [ ] 后端保存 / 回显 / 旧配置兼容。
- [ ] UI 提示成本、配额、兼容性风险。
- [ ] 补策略差异测试。

### 12.10 P1-04：行为型变更观测摘要

- [ ] request-id 自动生成只记录计数 / 跳过原因 / 短 hash，不记录完整 ID 与用户内容绑定。
- [ ] beta policy 观测只记录 token 名称、动作、preset，不记录完整请求 body。
- [ ] response header denylist 只记录命中前缀 / 计数，不记录完整 header value。
- [ ] 指标 label 保持低基数：
  - [ ] 不使用 account name；
  - [ ] 不使用完整 request id；
  - [ ] 不使用完整 `metadata.user_id`；
  - [ ] 不使用完整 header value。
- [ ] 观测逻辑复用 P0-04 redaction 规则。
- [ ] 补测试或静态断言，确认敏感值不会进入默认日志。

### 12.11 P2 收口 Checklist

- [ ] 跑后端回归：

```bash
cd backend
go test ./internal/service ./internal/handler -run 'ClaudeCode|GatewayBeta|Metadata|SessionIDMasking|Header|BetaPolicy'
go test ./internal/service -run 'AnthropicAPIKeyPassthrough|ResponseHeaders|Privacy|AntigravityPrivacy|OpenAIPrivacy'
go test ./internal/service -run 'FirstPartyAnthropic|ClaudeRequestID|ClaudeConditionalHeaders|ClaudeDefaults|LogRedaction'
```

- [ ] 跑前端测试：

```bash
cd frontend
pnpm test:run -- UseKeyModal
```

- [ ] 手工验证 OAuth normal path。
- [ ] 手工验证 Anthropic API key passthrough。
- [ ] 手工验证 `/v1/messages/count_tokens`。
- [ ] 检查 README / docs 推荐片段。
- [ ] 完成 `sub2api-claude-code-maintenance-notes.md`。
- [ ] 完成发布说明 / 回滚说明。

---

## 13. Codex / 子代理任务单

> 使用方式：每张任务单都可以作为单独 Codex prompt。并行执行时优先按“文件归属”拆分，避免两个代理同时改 `gateway_service.go`、settings DTO、`SettingsView.vue` 等热点文件。所有 worker 都应遵守：不要回滚他人改动；如发现依赖缺失，记录阻塞点，不要擅自扩大范围。

所有任务完成时统一回报格式：

```text
Summary:
Files changed:
Config / migration:
Tests:
Rollback:
Known risks / follow-ups:
```

### Task T-1：执行前置编排 / 配置命名冻结

**优先级**：P0  
**建议角色**：lead / integrator  
**文件归属**：

- `.omx/plans/sub2api-claude-code-telemetry-hardening-plan.md`
- issue tracker / 执行看板

**任务**：

1. 冻结新增配置 key、DTO 字段和 UI label：
   - request-id 自动生成；
   - response header dangerous override；
   - beta policy preset。
2. 指定热点文件 owner，尤其是：
   - `gateway_service.go`；
   - settings DTO / handler / service；
   - `SettingsView.vue`；
   - usage guide。
3. 确认测试包路径和命令存在。
4. 把每个任务的 DoD / 回报模板贴到 issue。

**验收**：

- T4/T7/T8 不再各自发明配置名；
- 并行任务没有重叠写同一热点文件；
- 每个 issue 都能按固定格式交付。

### Task T0：前端 Claude Code env 片段修正

**优先级**：P0  
**建议角色**：frontend worker  
**文件归属**：

- `frontend/src/components/keys/UseKeyModal.vue`
- `frontend/src/components/keys/*test*` 或新增 env snippet builder

**任务**：

1. 让 Unix / CMD / PowerShell Claude Code 片段都包含 `CLAUDE_CODE_ATTRIBUTION_HEADER=0`。
2. 保持 `ANTHROPIC_AUTH_TOKEN` 与 `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`。
3. 不新增默认 `DISABLE_TELEMETRY`。
4. 补 Vitest / snapshot / 纯函数测试。

**验收**：

```bash
cd frontend
pnpm test:run -- UseKeyModal
```

---

### Task T1：Claude Code 用户文档与 README 同步

**优先级**：P0  
**建议角色**：docs worker  
**文件归属**：

- `hook/docs/sub2api-claude-code-usage-guide.md`
- `README.md`
- `README_CN.md`
- `README_JA.md`

**任务**：

1. 写清标准推荐配置。
2. 写清 `ANTHROPIC_AUTH_TOKEN` vs `ANTHROPIC_API_KEY`。
3. 写清 nonessential、attribution、`DISABLE_TELEMETRY`、event sink、Datadog、BigQuery metrics cache、3P OTEL。
4. 写清 response header gateway detection 与 `additional_allowed` 风险。
5. 写清已知 `ANTHROPIC_BASE_URL` host suffix detection 边界，不承诺完全隐藏代理，也不把任意自定义域名误写成会命中 host suffix。
6. README 多语言只放短版和链接，详细解释集中到 usage guide。

**验收**：

```bash
rg -n 'ANTHROPIC_AUTH_TOKEN|CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC|CLAUDE_CODE_ATTRIBUTION_HEADER|DISABLE_TELEMETRY|OTEL_|metricsStatusCache' README* hook/docs
```

---

### Task T2：条件头白名单与 wire casing

**优先级**：P0  
**建议角色**：backend worker  
**文件归属**：

- `backend/internal/service/gateway_service.go`
- `backend/internal/service/header_util.go`
- 相关 service 测试

**任务**：

1. 加入 Remote / Agent SDK / additional-protection 条件头白名单。
2. wire casing 使用 lowercase。
3. messages / count_tokens 一致。
4. 不合成默认值。
5. 默认日志不输出 remote container/session 原文。

**验收**：

```bash
cd backend
go test ./internal/service -run 'Header|ClaudeConditionalHeaders'
```

---

### Task T3：日志脱敏与 debug 安全

**优先级**：P0  
**建议角色**：backend security worker  
**文件归属**：

- `backend/internal/service/gateway_service.go`
- `backend/internal/util/logredact/*`
- `backend/internal/service/*log*_test.go`

**任务**：

1. 默认 mimic debug 只输出低敏摘要。
2. `metadata.user_id`、remote session/container 改 hash / 短摘要。
3. token / cookie / API key 不得明文进入日志。
4. 文档标注 `SUB2API_DEBUG_GATEWAY_BODY` 风险。

**验收**：

```bash
cd backend
go test ./internal/service -run 'LogRedaction|ClaudeMimicDebug'
```

---

### Task T4：响应头 gateway denylist

**优先级**：P0  
**建议角色**：backend worker  
**文件归属**：

- `backend/internal/util/responseheaders/responseheaders.go`
- `backend/internal/service/*response_headers*_test.go`
- 注：`backend/internal/config/config.go` 中 dangerous override 字段由 T5 owner 合入；T4 不单独拥有该文件

**任务**：

1. 增加 gateway header prefix denylist。
2. denylist 优先级高于 `additional_allowed`。
3. denylist 用 lower-case prefix match；不要用 exact-match `force_remove` 替代。
4. 消费 T5 冻结的危险 override 字段；如 T5 尚未完成，先实现默认 denylist，不并行改 `config.go`。
5. 覆盖 streaming / non-streaming / API key passthrough / count_tokens。

**验收**：

```bash
cd backend
go test ./internal/service ./internal/util/... -run 'ResponseHeaders|AnthropicAPIKeyPassthrough'
```

---

### Task T5：配置项、settings API 与回滚底座

**优先级**：P0  
**建议角色**：backend + admin UI worker  
**文件归属**：

- `backend/internal/config/config.go`
- `backend/internal/service/setting_service.go`
- `backend/internal/handler/dto/settings.go`
- `backend/internal/handler/admin/setting_handler.go`
- `backend/migrations/*`
- `frontend/src/api/admin/settings.ts`
- `frontend/src/views/admin/SettingsView.vue`

**任务**：

1. 增加 request-id 自动生成相关配置。
2. 增加 response header dangerous override；该项优先作为静态 `security.response_headers.*` config/env，不默认接 admin UI。
3. 为 beta policy preset 留好 settings 存储 / 回显 / 兼容入口。
4. 空库 / 旧库 / 缺字段都走安全默认。
5. 补 API contract 测试。
6. 对外配置名必须用 snake_case / mapstructure 风格；旧 `beta_policy_settings` 缺 `preset` 时按 `conservative`。

**验收**：

```bash
cd backend
go test ./internal/handler ./internal/service -run 'Settings|Config|BetaPolicy|ResponseHeaders'
```

---

### Task T6：更新 `claude.DefaultHeaders` 基线

**优先级**：P1  
**建议角色**：backend worker  
**文件归属**：

- `backend/internal/pkg/claude/constants.go`
- 相关测试
- `hook/docs/sub2api-claude-code-maintenance-notes.md` 对应小节

**任务**：

1. 更新明显过旧的 UA / SDK header baseline。
2. 注释记录 baseline 来源。
3. 不盲目使用未验证最新版。
4. 不破坏 `applyClaudeOAuthHeaderDefaults()` / `applyClaudeCodeMimicHeaders()`。

**验收**：

```bash
cd backend
go test ./internal/service -run 'ClaudeDefaults|Header'
```

---

### Task T7：first-party `x-client-request-id` 自动生成

**优先级**：P1  
**建议角色**：backend worker  
**文件归属**：

- `backend/internal/service/gateway_service.go`
- 可能新增 request-id helper 文件
- `backend/internal/service/*request_id*_test.go`

**任务**：

1. 实现 `isFirstPartyAnthropicMessagesURL()`。
2. OAuth / SetupToken normal path 缺失时生成 UUID。
3. 第三方 / custom relay / HTTP / evil suffix 不生成。
4. API key passthrough 默认不生成。
5. 默认值按 T-1 / T5 freeze 执行；若默认开启仍有争议，先支持配置开关和灰度，不无回滚地改变全量行为。
6. messages / count_tokens 都覆盖。

**验收**：

```bash
cd backend
go test ./internal/service -run 'FirstPartyAnthropic|ClaudeRequestID|AnthropicAPIKeyPassthrough'
```

---

### Task T8：beta policy 兼容预设

**优先级**：P1  
**建议角色**：backend + admin UI worker  
**文件归属**：

- `backend/internal/service/settings_view.go`
- `backend/internal/service/gateway_service.go`
- settings DTO / handler / UI
- 相关测试

**任务**：

1. 保守默认保持不变。
2. 新增 Claude Code 兼容优先预设。
3. 明确 `fast-mode-2026-02-01`、`context-1m-2025-08-07` 放行行为。
4. UI 显式提示成本 / 配额风险。
5. 旧配置兼容。

**验收**：

```bash
cd backend
go test ./internal/service ./internal/handler -run 'GatewayBeta|BetaPolicy|Settings'
```

---

### Task T8.5：行为型变更低敏观测

**优先级**：P1  
**建议角色**：backend observability worker  
**文件归属**：

- metrics / logger 相关 helper
- 新增或相关 log redaction 测试
- 不直接修改 `gateway_service.go` 调用点；如需接入，由 backend integrator 合入

**任务**：

1. 为 request-id 自动生成、beta policy、response header denylist 设计低敏计数 / 摘要。
2. 指标 label 保持低基数，不使用 account name、完整 request id、完整 `metadata.user_id`、完整 header value。
3. 复用 P0-04 redaction 规则。
4. 补测试或静态断言，确认敏感值不会进入默认日志。

**验收**：

```bash
cd backend
go test ./internal/service -run 'LogRedaction|Observability|ResponseHeaders|BetaPolicy|ClaudeRequestID'
```

---

### Task T9：测试矩阵总补齐

**优先级**：P2  
**建议角色**：test worker  
**文件归属**：

- 后端测试文件
- 前端测试文件
- 不改生产逻辑，除非修复测试暴露的小 bug

**任务**：

1. 补全 C1 所列测试矩阵。
2. 整理可重复运行的 test subset。
3. 对 flaky / 缺依赖测试给出说明。

**验收**：

```bash
cd backend
go test ./internal/service ./internal/handler -run 'ClaudeCode|GatewayBeta|Metadata|SessionIDMasking|Header|BetaPolicy|AnthropicAPIKeyPassthrough|ResponseHeaders|Privacy|AntigravityPrivacy|OpenAIPrivacy|FirstPartyAnthropic|ClaudeRequestID|LogRedaction'
cd ../frontend
pnpm test:run -- UseKeyModal
```

---

### Task T10：维护文档、灰度、发布说明

**优先级**：P2  
**建议角色**：docs / release worker  
**文件归属**：

- `hook/docs/sub2api-claude-code-maintenance-notes.md`
- `hook/docs/sub2api-claude-code-usage-guide.md`
- release / changelog 文档

**任务**：

1. 写维护文档。
2. 写灰度顺序。
3. 写回滚开关。
4. 写发布说明。
5. 明确观测只记录低敏摘要。

**验收**：

- 文档能回答：标准用法、为什么不用 API key、request-id 何时生成、beta policy 如何切换、如何回滚、debug 开关风险。

---

## 14. 推荐执行编排

### 14.1 串行主线

0. T-1：先冻结配置命名、文件 owner、测试命令和回报格式。
1. T0 + T1：先修用户入口与文档。
2. T3 + T5：先立住日志安全和配置回滚底座。
3. T2 + T4：补请求头 / 响应头边界。
4. T6：更新 header baseline。
5. T7：实现 request-id 自动生成。
6. T8：实现 beta policy 预设。
7. T8.5：接入低敏观测摘要。
8. T9 + T10：测试、文档、灰度收口。

### 14.2 可并行项

- T0 与 T1 可并行起草，写不同文件；但 T1 的 README 最终片段检查应在 T0 输出确定后再收口。
- T2 与 T4 可并行，但都可能依赖配置约定；必须先完成 T-1。
- T6 可与 T1 / T4 并行。
- T8.5 可在 T7 / T8 设计稳定后并行准备 helper/tests，但生产调用点由 integrator 统一合入。
- T9 不建议太早启动，最好等 P0/P1 主体完成后集中补齐。
- T7 / T8 不建议与仍在修改 `gateway_service.go` 的 T2 / T3 并行；如确需并行，T7 先写 helper + tests，最后由唯一 integrator 合入调用点。

### 14.3 不建议并行写的热点

- `backend/internal/service/gateway_service.go`
- `backend/internal/config/config.go`
- `backend/internal/service/setting_service.go`
- settings DTO / handler / frontend settings UI
- `hook/docs/sub2api-claude-code-usage-guide.md`

这些文件如果必须多人协作，应指定唯一 owner，其他任务只提 issue / patch 建议，不直接改。

### 14.4 推荐文件锁分配

| 热点文件 / 区域 | 推荐唯一 owner | 其他任务处理方式 |
|---|---|---|
| `backend/internal/service/gateway_service.go` | backend integrator | T2/T3/T7/T8 只提交 helper 或 patch 建议，由 owner 合入 |
| `backend/internal/config/config.go` | T5 owner | T4 只消费 frozen config 字段，不直接改 |
| settings DTO / handler / `setting_service.go` | T5 owner | T8 只提交 preset 需求，等 T5 API contract 冻结后接入 |
| `frontend/src/views/admin/SettingsView.vue` | admin UI owner | T5/T8 不并行改同一区块 |
| `hook/docs/sub2api-claude-code-usage-guide.md` | docs owner | T1/T10 用同一 owner 或先后顺序编辑 |

---

## 15. 最终决策建议

本轮不建议把 sub2api 往 CLIProxyAPI 那种“更深的强伪装”方向推。  
更合理的路线是：

- 保持 `ANTHROPIC_AUTH_TOKEN + CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` 这条正确默认路径；
- 补齐默认内置兼容缺口；
- 更新旧基线；
- 增强 first-party 普通 CLI 请求的真实贴合度；
- 用文档和测试把结论沉淀下来。

一句话：

> **本计划的方向不是“更像伪装器”，而是“把 sub2api 已经领先的 Claude Code 低噪声接入方案，做成默认正确、兼容更稳、维护成本更低的产品能力”。**
