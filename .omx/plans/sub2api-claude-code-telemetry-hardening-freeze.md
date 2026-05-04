# sub2api Claude Code Telemetry Hardening — Freeze Decisions (T-1 / P0-00)

状态：Frozen v1
日期：2026-05-04
关联计划：`./sub2api-claude-code-telemetry-hardening-plan.md`

> **目的**：在并行执行前冻结所有跨任务约定（命名 / 文件 owner / 测试命令 / 报告格式），避免 T2/T3/T4/T5/T7/T8 写出冲突的代码或互相不兼容的 schema。

---

## 1. 配置命名冻结（snake_case / mapstructure）

> 对外配置一律用 snake_case + mapstructure tag，不出现 Go 风格名称（如 `Gateway.ClaudeRequestIDAutogenerateForAPIKeyPassthrough`）。

### 1.1 静态 config（viper / `backend/internal/config/config.go`）

| Config Key (viper) | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `gateway.claude_request_id.auto_generate_oauth` | bool | `true` | OAuth / SetupToken normal path 缺失 `x-client-request-id` 时是否自动生成 UUID。默认开启（first-party 真实 CLI 行为） |
| `gateway.claude_request_id.auto_generate_api_key_passthrough` | bool | `false` | API key passthrough 路径缺失 `x-client-request-id` 时是否自动生成。默认关闭（保持 passthrough 透明语义） |
| `security.response_headers.allow_gateway_trace_headers` | bool | `false` | **危险诊断 override**。开启后允许 `x-litellm-*` / `helicone-*` / `x-portkey-*` / `cf-aig-*` / `x-kong-*` / `x-bt-*` 等 gateway 痕迹响应头透传给客户端。默认关闭。**仅作为静态 config / env，不接入 admin UI。** |

对应 Go struct（在 `config.go` 中新增）：

```go
type GatewayConfig struct {
    // ... 既有字段
    ClaudeRequestID ClaudeRequestIDConfig `mapstructure:"claude_request_id"`
}

type ClaudeRequestIDConfig struct {
    AutoGenerateOAuth              bool `mapstructure:"auto_generate_oauth"`
    AutoGenerateAPIKeyPassthrough  bool `mapstructure:"auto_generate_api_key_passthrough"`
}

type ResponseHeaderConfig struct {
    Enabled                   bool     `mapstructure:"enabled"`
    AdditionalAllowed         []string `mapstructure:"additional_allowed"`
    ForceRemove               []string `mapstructure:"force_remove"`
    AllowGatewayTraceHeaders  bool     `mapstructure:"allow_gateway_trace_headers"` // 新增；危险诊断 override
}
```

`SetDefault` 增量：

```go
viper.SetDefault("gateway.claude_request_id.auto_generate_oauth", true)
viper.SetDefault("gateway.claude_request_id.auto_generate_api_key_passthrough", false)
viper.SetDefault("security.response_headers.allow_gateway_trace_headers", false)
```

### 1.2 DB Settings（`SettingKey*` / `setting_service.go` / `dto/settings.go`）

| Setting Key | DTO 字段 (json) | 默认值 | 备注 |
|---|---|---|---|
| `SettingKeyBetaPolicySettings`（既有） | `BetaPolicySettings.preset` (新增) | `"conservative"` | 取值：`"conservative"` 或 `"claude_code_compat"`。旧数据缺字段 → `conservative` |
| `SettingKeyBetaPolicySettings` | `BetaPolicySettings.rules`（既有） | `[...]` | 旧 rules 保留，迁移不清空 |

DTO 形态（最终冻结版）：

```go
// backend/internal/handler/dto/settings.go
type BetaPolicySettings struct {
    Preset string           `json:"preset,omitempty"` // "conservative" | "claude_code_compat"; 旧数据缺字段按 conservative 处理
    Rules  []BetaPolicyRule `json:"rules"`
}
```

JSON 兼容性约定（T8 owner 必须实现）：

- 反序列化：缺 `preset` 字段时，service 层 fallback 为 `"conservative"`；不修改持久化值。
- 写入：`SetBetaPolicySettings` 必须接受空 `preset`，并在写盘前归一化为 `"conservative"`。
- 旧 rules 迁移：保留旧 rules 数组不动；`preset` 仅作为附加策略层，不替换 rules 语义。

### 1.3 路径敏感检查 helper（T7 owner 实现）

helper 命名：`isFirstPartyAnthropicMessagesURL(targetURL string) bool`

接受：
- `https://api.anthropic.com/v1/messages`
- `https://api.anthropic.com/v1/messages?<query>`
- `https://api.anthropic.com/v1/messages/count_tokens`
- `https://api.anthropic.com/v1/messages/count_tokens?<query>`

拒绝：
- `http://api.anthropic.com/...`（非 https）
- `https://api.anthropic.com.evil/...`（host suffix 攻击）
- `https://api-anthropic.com/...`
- `https://example.com/v1/messages`
- 任何 path 非 `/v1/messages` 或 `/v1/messages/count_tokens`

实现位置建议：`backend/internal/service/claude_request_id.go`（新文件，与 helper + tests 一起）。

### 1.4 已知现状校正

> 计划文档基于早期快照写成，部分数值已与当前代码不符。冻结时按现实校正：

- `claude.DefaultHeaders["User-Agent"]` 当前已是 `"claude-cli/2.1.92 (external, cli)"`，而非计划中说的 `2.1.22`。T6 任务范围调整为：
  - 复核当前 `2.1.92` 是否仍是合适基线（截至 2026-05），或更新到更近的稳定 baseline；
  - 重点检查 `X-Stainless-Package-Version`（当前 `0.70.0`）是否仍为最新 anthropic-sdk-typescript 版本；
  - 不盲目追未验证版本；如果 `2.1.92` 已经够用，T6 主要任务是补 baseline-source 注释 + 维护文档。
- `header_util.go` 注释引用的抓包版本是 `claude-cli/2.1.81`，T6 应同步更新该注释。
- 既有 `gateway_service.go:6598-6600` 已经在 `applyClaudeCodeMimicHeaders()` 中无条件生成 `x-client-request-id`，但 **不限于 first-party**。T7 任务是把这段无条件生成改成由 `isFirstPartyAnthropicMessagesURL()` + 配置开关共同 gating，避免在自定义 relay 或第三方域上误生成。

---

## 2. 文件 Owner 冻结

> 同一时间段每个热点文件只允许一个 owner 写入。其他任务通过 helper 文件、独立子文件或 patch 建议交付。

| 热点文件 / 区域 | 唯一 owner（任务） | 其他任务的处理方式 |
|---|---|---|
| `backend/internal/service/gateway_service.go` | **Backend integrator（顺序合入）** | T2 / T3 / T7 / T8 不并行写本文件；先后顺序由编排者控制 |
| `backend/internal/service/header_util.go` | T2 | 仅 T2 修改；T6 只读引用 |
| `backend/internal/pkg/claude/constants.go` | T6 | 其他任务只读 |
| `backend/internal/util/responseheaders/responseheaders.go` | T4 | T5 只在 `config.go` 中冻结 dangerous override 字段；不触本文件 |
| `backend/internal/config/config.go` | T5 | T4 只消费 T5 已冻结的字段，不直接改本文件 |
| `backend/internal/service/setting_service.go` | T5 | T8 只提交 preset 需求；T5 完成后 T8 再扩 service 函数 |
| `backend/internal/handler/dto/settings.go` | T5 | T8 在 T5 框架内增加 `Preset` 字段 |
| `backend/internal/handler/admin/setting_handler.go` | T5 | T8 在 T5 接好的 endpoint 上扩 preset |
| `backend/internal/util/logredact/*` | T3 | 其他任务调用，不修改 |
| `backend/internal/service/settings_view.go` | T8 | T5 不触；T8 只在 T5 DTO 接好后增量改 |
| `frontend/src/components/keys/UseKeyModal.vue` | T0 | 独占 |
| `frontend/src/api/admin/settings.ts` | T5 → T8 顺序合入 | dangerous override 不入 UI（T5 不动），preset 由 T8 在 T5 之后扩 |
| `frontend/src/views/admin/SettingsView.vue` | T8 | T5 不触；危险 override 不接入 UI |
| `hook/docs/sub2api-claude-code-usage-guide.md` | T1（先写）→ T10（补维护章节） | T2/T6/T7 提交事实勘误 patch，但不并行写整段 |
| `hook/docs/sub2api-claude-code-maintenance-notes.md` | T10 | T6 写 baseline 来源章节时由 T10 合入 |
| `README.md` / `README_CN.md` / `README_JA.md` | T1 | 短入口 + 链接，不复制长说明 |

---

## 3. 测试命令路径校正

> 计划列出的回归命令在执行前必须确认 package 路径存在。已确认的最终命令：

```bash
# 主回归子集（覆盖既有逻辑）
cd backend
go test ./internal/service ./internal/handler -run 'ClaudeCode|GatewayBeta|Metadata|SessionIDMasking|Header|BetaPolicy'
go test ./internal/service -run 'AnthropicAPIKeyPassthrough|ResponseHeaders|Privacy|AntigravityPrivacy|OpenAIPrivacy'

# 新增测试集（本计划新增）
go test ./internal/service -run 'FirstPartyAnthropic|ClaudeRequestID|ClaudeConditionalHeaders|ClaudeDefaults|LogRedaction'

# response header 包级测试（路径校正）
go test ./internal/util/responseheaders/... -run 'ResponseHeaders|GatewayDenylist'

# config 验证
go test ./internal/config -run 'ResponseHeaders|ClaudeRequestID|Settings'

# 前端
cd frontend
pnpm test:run -- UseKeyModal
```

> 若新增 test 文件命名为 `*_first_party_anthropic_test.go` / `*_claude_request_id_test.go` / `*_response_headers_gateway_denylist_test.go` / `*_log_redaction_test.go`，可直接被上述 -run 模式命中。

### 3.1 静态检查命令

```bash
# README / docs 推荐片段一致性
rg -n 'ANTHROPIC_AUTH_TOKEN|CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC|CLAUDE_CODE_ATTRIBUTION_HEADER' README*.md hook/docs

# 默认片段不应出现 3P OTEL
if rg -q 'CLAUDE_CODE_ENABLE_TELEMETRY|OTEL_' frontend/src/components/keys; then
  echo "ERROR: default Claude Code snippet should not include 3P OTEL env" >&2
  exit 1
fi
```

---

## 4. 任务回报模板（每个任务必须返回）

```text
Summary:
  <改了什么；是否改变默认行为>

Files changed:
  - <path>:<short why>
  - ...

Config / migration:
  <新增 / 变更的 config key、DTO 字段、默认值、迁移 / 兼容策略；如果不涉及，写 "n/a">

Tests:
  Commands run:
    - <command> → <pass | fail | skipped, reason>
  New test files:
    - <path>
  Coverage notes:
    <补了哪些断言；哪些尚未覆盖>

Rollback:
  <如何关闭新行为或退回旧版本；列出 config key + 默认值或 git revert 范围>

Known risks / follow-ups:
  - <剩余风险 / 后续 issue / 需要人工验证的点>
```

---

## 5. 事实边界（产品文档 / 注释禁用语）

> T1 / T10 写文档以及 T6 / T2 写注释时，必须避免以下绝对化表达：

- ❌ "完全隐藏代理"
- ❌ "完全抑制所有 telemetry"
- ❌ "Claude Code 检测不到 sub2api"
- ❌ "100% 等价于真实 Claude Code"

允许的表达：

- ✅ "降低可控的响应头 gateway 痕迹泄漏面"
- ✅ "默认抑制 Anthropic 侧 nonessential traffic"
- ✅ "在 first-party Anthropic 上游下与真实 CLI 的 header 形态对齐"
- ✅ "已知 `ANTHROPIC_BASE_URL` provider-owned host suffix（如 Databricks）仍可能参与 gateway detection；响应头过滤不能消除该命中"

---

## 6. 任务依赖图（执行顺序参考）

```
T-1 (本文档) ─── 完成
   │
   ├─→ T0  (frontend: UseKeyModal)              ← 独立，可并行
   ├─→ T1  (docs: usage guide + READMEs)        ← 独立，可并行
   ├─→ T6  (claude/constants.go baseline)       ← 独立，可并行
   ├─→ T5  (config + settings + DTO/handler)    ← gates T2/T4/T7/T8
   ├─→ T3  (logredact + mimic debug 脱敏)       ← gates T7/T8 的日志接入
   │
   ├──→ [T5 完成] ──→ T2 (header_util.go + gateway header allowlist)
   │                ──→ T4 (responseheaders.go + tests，消费 T5 字段)
   │
   ├──→ [T2/T3/T5 完成] ──→ T7 (gateway_service.go: first-party request-id)
   │                       ──→ T8 (settings_view.go + gateway_service.go: beta preset)
   │
   ├──→ [T7/T8 完成] ──→ T8.5 (低敏观测摘要)
   │
   └──→ [全部 P0/P1 完成] ──→ T9 (test matrix 收口) + T10 (维护文档 + release notes)
```

---

## 7. 已签署 / 待签署

- ✅ 配置命名 freeze
- ✅ 文件 owner freeze
- ✅ 测试命令路径校正
- ✅ 报告模板冻结
- ✅ 事实边界冻结
- ✅ 任务依赖图

后续任务必须严格遵循以上冻结。如执行中发现冻结项有阻塞，需先与 integrator 对齐更新本文件，再展开实现。

---

## 8. 现状对照（执行前盘点）

| 计划假设 | 实际现状 | 对任务的影响 |
|---|---|---|
| `claude.DefaultHeaders` UA = `claude-cli/2.1.22` | 实际 `claude-cli/2.1.92` | T6 范围缩窄为复核 + 注释；非"必更新" |
| `x-client-request-id` 未自动生成 | `gateway_service.go:6598-6600` 已无条件生成 | T7 任务变为"加 first-party 限制 + 配置开关" |
| `hook/docs/` 不存在 | 确认不存在 | T1 必须新建该目录 |
| `BetaPolicySettings.Preset` 字段不存在 | 确认 DTO 仅有 `Rules` | T5/T8 按本冻结新增 |
| 响应头 denylist 不存在 | `responseheaders.go` 仅有白名单 + force_remove | T4 完整新增 |
| 4 个条件头 (`x-claude-remote-*` 等) 不在 allowedHeaders | 已确认缺失 | T2 按计划新增 |
| `safeHeaderValueForLog` 已 redact authorization / x-api-key | 已确认 | T3 在此基础上扩 metadata.user_id / remote-* 字段 hash |
