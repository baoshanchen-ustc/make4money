# 请求排查耗时改造 Round 5/6 审查修复报告

**日期**：2026-03-26
**关联复审（上轮）**：`docs/reports/2026-03-25-ops-latency-rereview-round3-fixes/fix-report.md`
**状态**：✅ Round 6 Codex 审查通过，无剩余问题

---

## 一、审查修复轮次摘要

| 轮次 | 类型 | 说明 |
|------|------|------|
| Round 5（Codex 审查） | 发现问题 | Important × 1，Minor × 1 |
| Round 6（修复后再审） | 审查通过 | 无剩余问题 |

---

## 二、Round 5 发现问题与修复详情

### P1（Important）：Copilot 三路径 auth_latency_ms 计时起点口径不一致

**问题**：
`CopilotGatewayHandler` 的 `ChatCompletions`、`Responses`、`Messages` 三个函数中，`requestStart`（或 `requestStartResponses`/`requestStartMessages`）变量声明在解析请求体（`ReadRequestBodyWithPrealloc` + JSON 解析）之后才调用 `time.Now()`，导致 `auth_latency_ms` 少计了读取/解析 body 的耗时。

相比之下，`gateway_handler.go`（L113）和 `openai_gateway_handler.go`（L90、L496、L1030）均在函数入口第一行声明 `requestStart := time.Now()`，保证同口径计算。

**根因**：历史上 Copilot handler 的计时注释写的是"从请求进入到获得处理资格（含计费检查 + 并发槽位获取）"，语义上只想计 billing + concurrency，而 OpenAI 路径是整个请求处理入口的时间戳。两者口径不一致，会导致跨路径横向比较时 Copilot 的 `auth_latency_ms` 系统性偏小。

**修复方案**：
- 将三个函数的 `requestStart` 系列变量移到函数体第一行（`func (h *CopilotGatewayHandler) xxx(c *gin.Context) {` 下紧接一行）
- 删除原错误位置的变量声明及相关注释

**修复文件**：`backend/internal/handler/copilot_gateway_handler.go`

| 函数 | 修复前位置 | 修复后位置 |
|------|-----------|-----------|
| `ChatCompletions` | L164（setOpsRequestContext 之后） | L108（函数入口） |
| `Responses` | L545（setOpsRequestContext 之后） | L492（函数入口） |
| `Messages` | L845（billing check 注释之后） | L778（函数入口） |

---

### P2（Minor）：OpenAI Messages 路径缩进混乱 + 变量名后缀 `2`

**问题**：
`openai_gateway_handler.go` 的 `Messages` 函数中，提交 usage 记录的代码块（`authLatencyMs2` 等变量声明 + `h.submitUsageRecordTask` 调用）缩进比应在层级（for 循环内 2 个 tab）少了 1 个 tab，造成阅读混乱。

变量名 `authLatencyMs2`、`routingLatencyMs2`、`upstreamLatencyMsVal2`、`responseLatencyMsVal2` 中的 `2` 后缀无语义价值，仅是前一函数（`Responses`）同名变量的遗留规避，应直接命名为 `authLatencyMs` 等。

**修复**：
- 修正 4 个变量声明及 `h.submitUsageRecordTask` 闭包的缩进为 2 个 tab（与 `if result != nil` 同层级）
- 变量名去掉 `2` 后缀

**修复文件**：`backend/internal/handler/openai_gateway_handler.go`（L756–789）

---

## 三、测试验证

```bash
cd backend && go build ./internal/handler/...  # 编译通过
go test ./internal/handler/... ./internal/service/... ./internal/repository/...
```

结果：
```
ok   github.com/Wei-Shaw/sub2api/internal/handler         18.468s
ok   github.com/Wei-Shaw/sub2api/internal/handler/admin    (cached)
ok   github.com/Wei-Shaw/sub2api/internal/handler/dto      (cached)
ok   github.com/Wei-Shaw/sub2api/internal/service          (cached)
ok   github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2  (cached)
ok   github.com/Wei-Shaw/sub2api/internal/repository       (cached)
```

---

## 四、修改文件清单

| 文件 | 修改性质 |
|------|----------|
| `backend/internal/handler/copilot_gateway_handler.go` | P1：三路径 requestStart 移到函数入口，删除旧位置声明和注释 |
| `backend/internal/handler/openai_gateway_handler.go` | P2：Messages 路径缩进修正，变量名去除 `2` 后缀 |

---

## 五、各路径 stage latency 采集完整状况（累计全轮最终状态）

| 路径 | auth | routing | upstream | response |
|------|------|---------|----------|----------|
| `POST /v1/chat/completions` (OpenAI) | ✅ 函数入口计时 | ✅ | ✅ | ✅ |
| `POST /v1/responses` (OpenAI) | ✅ 函数入口计时 | ✅ | ✅ | ✅ |
| `POST /v1/messages` (OpenAI) | ✅ 函数入口计时 | ✅ | ✅ | ✅ |
| `GET /v1/responses` WebSocket | ✅ wsRequestStart 入口计时 | ✅ | ✅ turn.Duration | nil（有意） |
| `POST /v1/chat/completions` Copilot | ✅ 函数入口计时（本轮修复） | ✅ | ✅ | ✅ |
| `POST /v1/responses` Copilot | ✅ 函数入口计时（本轮修复） | ✅ | ✅ | ✅ |
| `POST /v1/messages` Copilot | ✅ 函数入口计时（本轮修复） | ✅ | ✅ | ✅ |
| `POST /v1/messages` Anthropic direct | ✅ | ✅ | ✅ | ✅ |

---

## 六、Round 6 Codex 审查结论

> "未发现需要修复的 stage latency 相关问题，本轮无需提交 patch。"

**审查通过，stage latency 改造全部完成。**
