# Copilot Session Quota Optimization 方案评审报告（Round 4）

## 基本信息
- 复审日期：2026-04-13
- 复审对象：`docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md`
- 对应上一轮报告：`docs/reports/2026-04-13-copilot-session-quota-optimization-plan-review-round3.md`
- 复审类型：Task 4 分支级测试补充后的 follow-up review
- 复审方式：静态审阅更新后的计划文档，并对照当前 session key 解析契约核对新增测试输入是否真实可用

## 复审结论（摘要）
- 上一轮指出的主问题，这次大方向已经修到了：
  - Task 4 现在不只测 3 个顶层入口，已经明确补了 3 个分支级端到端测试
  - M2 里的“5 个 / 6 个调用点”计数也基本对齐了
- 但当前计划里仍有 1 个新的阻断级问题：`TestCopilotSessionCache_ViaMessagesBranch` 里的 `sessionUser` fixture 含有非法 UUID 字符，按当前 `ParseMetadataUserID` 契约根本解析不出 session key。这个测试将无法真正验证 viaMessages 分支的 session cache 行为。
- 当前结论仍为：`REQUEST CHANGES`。

---

## 主要发现

### HIGH

#### H1. `TestCopilotSessionCache_ViaMessagesBranch` 使用了不符合 `ParseMetadataUserID` 契约的 session id，测试不会真的命中 session cache

#### 问题
这轮新增的 viaMessages 分支测试本身方向是对的：

- `docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md:1178`

它通过：
- file parts
- `/models => ["/v1/messages"]`
- 无 `base_url`

来触发 `forwardChatCompletionsViaMessages`，这个分支选择和真实代码是一致的：

- `backend/internal/service/copilot_gateway_service.go:190`
- `backend/internal/service/copilot_gateway_service.go:195`
- `backend/internal/service/copilot_gateway_service.go:1751`

但这个测试里给 OpenAI `user` 字段塞的 session 值是：

- `docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md:1220`

其中 session UUID 尾部写成了：

- `ffffgggggggg`

而当前仓库对 legacy `metadata.user_id` 的解析规则是：

- `backend/internal/service/metadata_userid.go:24`

也就是 session 段必须匹配：

- `[a-fA-F0-9-]{36}`

换句话说只能是十六进制字符加连字符，`g` 根本不合法。按真实实现：

- `ParseMetadataUserID(...)` 会返回 `nil`：`backend/internal/service/metadata_userid.go:35`
- `extractSessionKeyFromOpenAIBody(...)` 也就会返回空字符串
- 该测试里的“第二次同 session 请求 → agent”实际上不会由 session cache 触发

而 `forwardChatCompletionsViaMessages` 的基础 initiator 又只是：

- `copilotInitiator(body)`：`backend/internal/service/copilot_gateway_service.go:576`

对于这个测试给的首轮 file body，没有 assistant/tool 历史，所以基础值仍然是 `"user"`。

#### 风险
- 按当前文档实现，这个新增测试大概率会失败在第二次请求仍然是 `"user"`。
- 更糟的是，它会让“我们已经给 viaMessages 分支补了 session cache 回归测试”这个结论失真，因为测试输入本身不满足 session 解析前提。
- 这会直接削弱上一轮刚补上的分支级护栏。

#### 建议
- 把 `docs/superpowers/plans/2026-04-13-copilot-session-quota-optimization.md:1220` 的 session UUID 替换成只含十六进制字符的合法值，例如：
  - `aaaabbbb-cccc-dddd-eeee-ffff11112222`
  - 或任意其他满足 `[a-fA-F0-9-]{36}` 的值
- 最稳妥的做法是直接复用文档里其他已验证合法的 session fixture，避免再引入手写 typo。

---

## 复审结论

### Recommendation
`REQUEST CHANGES`

### 原因
- 分支覆盖这次已经基本补齐，整体计划离可实施状态很近了。
- 但 `ViaMessagesBranch` 这个新增分支测试的 session fixture 本身不合法，会直接让测试失效。
- 先把这一处测试输入修正后，这份计划大概率就可以进入实现阶段。

### 备注
- 本次仍为静态 review。
- 未修改业务代码。
- 未执行构建、类型检查或测试，因为复审对象仍是计划文档而非已实现代码。
