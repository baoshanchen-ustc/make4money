# sub2api × Claude Code 使用指南

状态：v1（对应仓库 `review-sub2api-claude-plan-v5` 分支落地的能力）
适用读者：将 Claude Code 接入 sub2api 网关的最终用户与运维管理员。

> 本指南只覆盖**使用**与**风险边界**。如果你在维护 sub2api 仓库本身，请同时阅读 `sub2api-claude-code-maintenance-notes.md`。

---

## 1. 一分钟接入

将下列 4 个环境变量交给 Claude Code（任选一种 shell）：

```bash
# Unix / macOS / Linux
export ANTHROPIC_BASE_URL="https://your-sub2api.example.com"
export ANTHROPIC_AUTH_TOKEN="sk-...你的 sub2api key..."
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
export CLAUDE_CODE_ATTRIBUTION_HEADER=0
```

```cmd
:: Windows CMD
set ANTHROPIC_BASE_URL=https://your-sub2api.example.com
set ANTHROPIC_AUTH_TOKEN=sk-...
set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
set CLAUDE_CODE_ATTRIBUTION_HEADER=0
```

```powershell
# PowerShell
$env:ANTHROPIC_BASE_URL="https://your-sub2api.example.com"
$env:ANTHROPIC_AUTH_TOKEN="sk-..."
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
$env:CLAUDE_CODE_ATTRIBUTION_HEADER=0
```

或者写到 `~/.claude/settings.json`：

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://your-sub2api.example.com",
    "ANTHROPIC_AUTH_TOKEN": "sk-...",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0"
  }
}
```

> sub2api 前端 `生成配置` 按钮已经按上述模板输出。如果你看到的片段缺少 `CLAUDE_CODE_ATTRIBUTION_HEADER=0`，请刷新前端到当前版本。

---

## 2. 为什么推荐 `ANTHROPIC_AUTH_TOKEN` 而不是 `ANTHROPIC_API_KEY`

Claude Code 在 [`getAuthHeaders()`](https://github.com/anthropics/claude-code) 里对这两个 env 走完全不同的路径：

| Env | 上游头形态 | 适用账号 | sub2api 推荐 |
|---|---|---|---|
| `ANTHROPIC_AUTH_TOKEN` | `Authorization: Bearer <token>` | 任何代理 / OAuth-style 网关 | ✅ 默认 |
| `ANTHROPIC_API_KEY` | `x-api-key: <key>` | 直连 Anthropic 官方 API 的 first-party 场景 | ⚠ 不建议 |

如果你切到 `ANTHROPIC_API_KEY`：

- Claude Code 会把它视为"直连官方 API"，并启用更激进的 BigQuery metrics export（详见 §6）；
- 部分 keychain / `apiKeyHelper` / OS-级 API key 来源也会被纳入认证链，这些路径**绕过 `ANTHROPIC_BASE_URL`**，最终请求不会经过 sub2api；
- billing 归属与配额计算可能与你预期的代理账号不一致。

**唯一例外**：sub2api 后端管理员显式开启了 `Anthropic API key passthrough` 模式并给你下发的就是真实 Anthropic API key 时。这种情况下你应当遵循该模式的专门说明。

---

## 3. `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` 做了什么

这是 Claude Code 内置的 **essential-traffic** 开关。设为 `1` 后：

- 抑制 Datadog metrics intake；
- 抑制 BigQuery metrics export；
- 抑制 BigQuery opt-out 探测；
- 抑制 autoupdater 调用；
- 抑制非关键错误上报（部分 `/api/event_logging/batch` 流量）；
- 不影响 `/v1/messages` 主路径（你的对话和工具调用不会受影响）。

> 它的作用范围**比** `DISABLE_TELEMETRY` 更宽。详见下一节。

---

## 4. `CLAUDE_CODE_ATTRIBUTION_HEADER=0` 做了什么

Claude Code 默认会给请求附加形如 `x-anthropic-helper-method` / `x-anthropic-attribution-block` 的 attribution 头，作为 first-party billing 的 fingerprint。设为 `0` 后这些头不再注入。

为什么 sub2api 标准模板里要关掉它：

- 这些头的存在会让请求看起来更"first-party"，但 sub2api 走的是代理路径，对应账号通常不是该 header 描述的那个账号；
- 让上游收到的 attribution 与代理实际账号不一致，长期可能影响 billing 分析；
- 关掉后不影响功能 — `/v1/messages` 仍正常工作。

---

## 5. `DISABLE_TELEMETRY` vs `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC`

| 变量 | 作用范围 | 是否在 sub2api 标准模板 |
|---|---|---|
| `DISABLE_TELEMETRY=1` | 仅 Statsig telemetry（nonessential traffic 的子集） | ❌ 不内置 |
| `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` | telemetry + metrics + autoupdater + error sink + 大部分 nonessential traffic | ✅ 默认 |

**结论**：sub2api 标准模板只下发 `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`，已覆盖 `DISABLE_TELEMETRY` 的所有效果，并且范围更广。如果你看到第三方文档提到 `DISABLE_TELEMETRY=1`，把它当作"次优解"即可，不需要再补到模板里。

---

## 6. BigQuery metrics 边界：什么 sub2api 拦不到

Claude Code 在两个固定的 endpoint 上做 metrics：

| 用途 | URL |
|---|---|
| metrics export | `https://api.anthropic.com/api/claude_code/metrics` |
| metrics opt-out 探测 | `https://api.anthropic.com/api/claude_code/organizations/metrics_enabled` |

注意：

1. **这两个 URL 默认直连 `api.anthropic.com`**，不会读 `ANTHROPIC_BASE_URL`，所以 sub2api 看不到也拦不到这部分流量。`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` 是抑制它们的主要手段。
2. Claude Code 内部有 `metricsStatusCache.enabled=true` 的 fast-path：当本地缓存说"上游已表态"时会跳过 opt-out 探测。这意味着：
   - 如果你是首次安装 + 没设 nonessential 开关，opt-out 探测**会**直连 Anthropic；
   - 如果你之前已经用真实 Anthropic API key 跑过 Claude Code 并缓存了结果，这缓存仍然影响后续行为。
3. 如果你的本机有残留的：
   - `ANTHROPIC_API_KEY` env；
   - `apiKeyHelper`（来自 `claude config get apiKeyHelper`）；
   - OS keychain 中的 Anthropic API key 凭据；
   ...这些会把请求的认证头改成 `x-api-key: <真 API key>`，从而被 Claude Code 视为 first-party 直连，重新启用 BigQuery metrics 路径。**这种情况下 `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` 仍然有效，但 sub2api 不能保证流量永远只走代理。**

> **`/api/event_logging/batch` 是兜底**：sub2api 在网关侧能对部分 logging batch 请求做拦截，但这只是兜底。Datadog metrics 与 BigQuery metrics 都不会通过 `event_logging/batch`，sub2api 也不应被宣传成"能拦下所有 telemetry"。

---

## 7. 用户显式开启 3P OpenTelemetry 的边界

Claude Code 还允许通过下面这些 env 开启**用户自配**的 OpenTelemetry 链路：

- `CLAUDE_CODE_ENABLE_TELEMETRY=1`
- `OTEL_EXPORTER_OTLP_ENDPOINT=...`
- 其它任何 `OTEL_*` 标准变量

**这是用户主动配置的 3P 链路，与 Anthropic 侧 1P telemetry 是两回事**：

- sub2api 标准模板**不**设置这些变量；
- 如果你/你的组织开启了它们，请按你**自己组织的 OTEL 收集器**评估隐私边界，sub2api 不能也不应承诺拦截这些流量；
- 验证标准模板是否含意外 OTEL：
  ```bash
  rg -n 'CLAUDE_CODE_ENABLE_TELEMETRY|OTEL_' frontend/src/components/keys/UseKeyModal.vue
  # 应当无输出
  ```

---

## 8. forwarding 类高级开关的语义

sub2api 后端 admin settings 里几个容易混淆的开关：

| 开关 | 真实语义 | 不等于… |
|---|---|---|
| `fingerprintUnification=false` | 不套用 sub2api 的统一 header fingerprint，仅透传客户端原 header | 不是"完全不改写 metadata.user_id" |
| `metadataPassthrough=false` | 仍会重写 body 中的 `metadata.user_id`（替换为 sub2api 的 session 形态） | 不是"100% 透明转发" |
| `session_id_masking_enabled=true` | 会进一步**固定 / 伪装** session 段；属于高级开关 | 风险更高，建议只在确认上游策略后启用 |

> 如果你只是想让 Claude Code 行为更贴近"真实 first-party"，**不需要**碰这三个开关。

---

## 9. 响应头 gateway 痕迹过滤

Claude Code 会把若干已知 LLM gateway / proxy 在响应里追加的痕迹头视为 `gateway detection` 信号。sub2api 默认会拦截以下前缀：

- `x-litellm-*`
- `helicone-*`
- `x-portkey-*`
- `cf-aig-*`
- `x-kong-*`
- `x-bt-*`

实现要点：

1. 拦截基于 **lower-case prefix match**，混合大小写也会被拦下；
2. 优先级**高于** `security.response_headers.additional_allowed`；即使管理员误把 `x-litellm-model-id` 加入 `additional_allowed`，仍不会泄漏到客户端；
3. 业务必需头 (`x-request-id`、`retry-after`、各 `x-ratelimit-*`) 不受影响；
4. 仅作为响应头的安全边界 — 它**不能**让 Claude Code "彻底看不到代理"（详见 §10）。

### 9.1 危险诊断 override

如果你确实要在排障时短暂透传这些头：

```yaml
security:
  response_headers:
    allow_gateway_trace_headers: true   # 默认 false；仅作静态 config / env
```

或：

```bash
SECURITY_RESPONSE_HEADERS_ALLOW_GATEWAY_TRACE_HEADERS=true
```

⚠ 开启后客户端会看到 gateway 痕迹头，**Claude Code 将能够识别 gateway**。该开关刻意**不接入 admin UI**，避免运营误开。诊断完成后请立即关闭。

---

## 10. gateway host detection 边界 —— 不要承诺"完全隐藏代理"

Claude Code 同时还会检查部分**已知** `ANTHROPIC_BASE_URL` 的 provider-owned host suffix（当前源码已确认 Databricks suffix 等）。这意味着：

- 如果你的 `ANTHROPIC_BASE_URL` 命中某个被 Claude Code 显式识别的 suffix（典型场景：自托管在 Databricks-hosted Anthropic 通道下），Claude Code 仍会把该流量识别为对应 provider；
- sub2api 的响应头过滤**不能**把已命中 host suffix 的 base URL 变成"未命中"；
- 反过来，**不是任意自定义域名都会触发 host suffix 检测** — 比如你部署到 `https://your-company-proxy.example.com` 不会匹配到任何已知 suffix。

> sub2api 只承诺**降低可控的响应头 gateway 痕迹泄漏面**和**默认抑制 Anthropic 侧 nonessential traffic**。不承诺"完全隐藏代理"或"完全抑制所有 telemetry"。

---

## 11. `security.response_headers.additional_allowed` 风险

- 该列表用于让运营临时允许某些**自定义业务头**透传给客户端；
- 不要把已知 gateway 前缀（`x-litellm-*` 等）放进来，§9 的 denylist 会把它们再次拦掉；
- 如果你确实需要短暂透传这些，请用 `allow_gateway_trace_headers=true`（§9.1），不要靠 `additional_allowed` 绕。

---

## 12. 关闭 / 回滚新行为

| 新行为 | 关闭方式 |
|---|---|
| `x-client-request-id` 自动生成（OAuth） | `gateway.claude_request_id.auto_generate_oauth=false` |
| `x-client-request-id` 自动生成（API key passthrough） | 默认就是关闭；要保持关闭无需操作 |
| 响应头 gateway prefix denylist | 临时透传：`security.response_headers.allow_gateway_trace_headers=true` |
| Beta policy 兼容预设 | 在 admin UI 中把 preset 切回 `Conservative` |
| `CLAUDE_CODE_ATTRIBUTION_HEADER=0` 模板项 | 自行从 env / settings.json 移除（不影响功能） |

---

## 13. 常见问题

**Q: 我把 `ANTHROPIC_API_KEY` 设成了 sub2api key，行不行？**
A: 通常**不行**。Claude Code 会把它当成 `x-api-key`，不少 sub2api 部署的 OAuth-style passthrough 模式不接受 `x-api-key` 形态。请改用 `ANTHROPIC_AUTH_TOKEN`。

**Q: 标准模板没有 `DISABLE_TELEMETRY=1`，是不是漏配？**
A: 不是。`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` 已覆盖更宽的范围（详见 §5）。

**Q: 我开了 OTEL 后发现还有数据上报，sub2api 拦不下来？**
A: 这是用户自配的 3P OpenTelemetry，由你自己的 OTEL collector 接收，sub2api 不在该路径上。详见 §7。

**Q: 我看到响应里有 `x-litellm-model-id`，怎么回事？**
A: 检查是否启用了 `security.response_headers.allow_gateway_trace_headers=true`。默认情况下该头会被 prefix denylist 拦住。

**Q: 兼容预设 (`claude_code_compat`) 真的安全吗？**
A: 它只是放过 `fast-mode-2026-02-01` 和 `context-1m-2025-08-07` 两个 beta，让上游认为客户端期望这些能力。注意两个边界：

1. **会产生**额外 priority 配额或 1M context 额外费用 — 在配额可承受时再开启。
2. 兼容预设对这两个 token **同时跳过 Filter 和 Block** 规则 — 也就是说你之前如果显式 `Block` 了 `context-1m-2025-08-07`，切到兼容预设后该 block **静默失效**。如需保留 block，请保持 `Conservative` 预设，或在 rules 中改用更精确的 scope / model_whitelist 限制条件。
