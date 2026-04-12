# Ops 请求排查页面：模型转换显示 + 用户/Key 三列 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在请求排查页面的列表中显示模型转换（`claude-sonnet-4.6 ↓ gpt-5.4`），并将"用户/Key"一列拆成"用户"、"Key 名称"、"密钥"三列。

**Architecture:** 后端 CTE SQL 补充 `upstream_model`（来自 `usage_logs`）和 `api_key_name`（来自 `api_keys.name`），`OpsRequestDetail` 结构体新增两个可选字段，前端类型同步后更新列表列渲染和详情面板的模型格子视觉。

**Tech Stack:** Go (database/sql, pq), PostgreSQL CTE, Vue 3 + TypeScript, Tailwind CSS

---

## 文件变更总览

| 文件 | 操作 | 说明 |
|------|------|------|
| `backend/internal/service/ops_request_details.go` | Modify | `OpsRequestDetail` 加 `UpstreamModel *string` 和 `APIKeyName *string` |
| `backend/internal/repository/ops_repo_request_details.go` | Modify | CTE success 分支加两列，SELECT/Scan/赋值同步 |
| `frontend/src/api/admin/ops.ts` | Modify | `OpsRequestDetail` 接口加 `upstream_model` 和 `api_key_name` |
| `frontend/src/views/admin/ops/OpsRequestInspectView.vue` | Modify | tableCols 拆三列；模型 td 加转换小字；用户/Key td 拆三个 td |
| `frontend/src/views/admin/ops/components/OpsRequestDetailPanel.vue` | Modify | 详情面板模型格子：当有 upstream_model 时用箭头 + 高亮色显示转换 |

---

## Task 1：后端 — `OpsRequestDetail` 结构体加两个字段

**Files:**
- Modify: `backend/internal/service/ops_request_details.go:23-90`

- [ ] **Step 1：在 `OpsRequestDetail` 结构体末尾（`FaultOwner` 字段前或 `AnomalyTypes` 后）加两个字段**

打开 `backend/internal/service/ops_request_details.go`，找到 `type OpsRequestDetail struct` 的 `AnomalyTypes []string` 字段，在其后加入：

```go
// UpstreamModel is the actual model used upstream after mapping.
// Nil when no mapping was applied (upstream model == client model).
UpstreamModel *string `json:"upstream_model,omitempty"`

// APIKeyName is the human-readable name of the API key (e.g. "wanggao").
APIKeyName *string `json:"api_key_name,omitempty"`
```

完成后该结构体末尾应如下（省略中间字段）：
```go
type OpsRequestDetail struct {
    // ... 已有字段 ...
    AnomalyTypes []string `json:"anomaly_types,omitempty"`

    // UpstreamModel is the actual model used upstream after mapping.
    // Nil when no mapping was applied (upstream model == client model).
    UpstreamModel *string `json:"upstream_model,omitempty"`

    // APIKeyName is the human-readable name of the API key (e.g. "wanggao").
    APIKeyName *string `json:"api_key_name,omitempty"`
}
```

- [ ] **Step 2：确认编译通过**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

期望：无错误输出。

- [ ] **Step 3：提交**

```bash
cd /Users/ziji/personal/github/sub2api
git add backend/internal/service/ops_request_details.go
git commit -m "Feature: OpsRequestDetail 加 UpstreamModel 和 APIKeyName 字段"
```

---

## Task 2：后端 — Repository CTE 补充 upstream_model 和 api_key_name

**Files:**
- Modify: `backend/internal/repository/ops_repo_request_details.go`

**背景：** CTE 的 success 分支从 `usage_logs`（别名 `ul`）和 `api_keys`（别名 `ak`）取数。两个字段已存在于数据库。

- [ ] **Step 1：在 CTE success 分支的 SELECT 列表中加两列**

找到 `ops_repo_request_details.go` 中 CTE 的 success 分支，在 `ul.spans::TEXT AS spans_json` 前加入：

```sql
    NULLIF(TRIM(ul.upstream_model), '') AS upstream_model,
    ak.name AS api_key_name,
```

完整 success 分支末尾应如下（仅展示新增的两行和相邻行）：
```sql
    COALESCE(ul.input_tokens, 0) AS input_tokens,
    COALESCE(ul.output_tokens, 0) AS output_tokens,
    NULLIF(TRIM(ul.upstream_model), '') AS upstream_model,
    ak.name AS api_key_name,
    ul.spans::TEXT AS spans_json
  FROM (
```

- [ ] **Step 2：在 CTE error 分支的 SELECT 列表中加两个 NULL 占位列（保持 UNION ALL 列数一致）**

找到 error 分支，在 `o.spans::TEXT AS spans_json` 前加入：

```sql
    NULL::TEXT AS upstream_model,
    NULL::TEXT AS api_key_name,
```

完整 error 分支末尾：
```sql
    0 AS input_tokens,
    0 AS output_tokens,
    NULL::TEXT AS upstream_model,
    NULL::TEXT AS api_key_name,
    o.spans::TEXT AS spans_json
  FROM ops_error_logs o
```

- [ ] **Step 3：在 listQuery 的 SELECT 列表中加两列**

找到 `listQuery` 字符串中的 `SELECT` 列表（`spans_json` 之前），加入：

```sql
  upstream_model,
  api_key_name,
```

完整末尾：
```sql
  ...
  anomaly_types,
  upstream_model,
  api_key_name,
  spans_json
FROM combined
```

- [ ] **Step 4：在 Scan 变量声明块中加两个变量**

找到 `rows.Next()` 循环内 `var (` 块，在 `spansJSON sql.NullString` 后加入：

```go
upstreamModel sql.NullString
apiKeyName    sql.NullString
```

- [ ] **Step 5：在 `rows.Scan(...)` 调用中末尾加两个参数**

找到 `rows.Scan(` 调用，在 `&spansJSON,` 后加入（保持与 SELECT 列顺序一致）：

```go
&upstreamModel,
&apiKeyName,
```

- [ ] **Step 6：在 item 赋值块中加两个字段的赋值**

找到 `item := &service.OpsRequestDetail{...}` 之后的 `if userName.Valid` 系列赋值块，在其末尾（`if len(anomalyTypes) > 0` 块之后，`if spansJSON.Valid` 块之前）加入：

```go
if upstreamModel.Valid && upstreamModel.String != "" {
    s := upstreamModel.String
    item.UpstreamModel = &s
}
if apiKeyName.Valid && apiKeyName.String != "" {
    s := apiKeyName.String
    item.APIKeyName = &s
}
```

- [ ] **Step 7：编译确认**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

期望：无错误。

- [ ] **Step 8：跑现有 repo 测试**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/repository/... -v -run TestOps 2>&1 | tail -30
```

期望：所有测试 PASS（或无相关测试，输出 `no test files`）。

- [ ] **Step 9：提交**

```bash
cd /Users/ziji/personal/github/sub2api
git add backend/internal/repository/ops_repo_request_details.go
git commit -m "Feature: CTE 补充 upstream_model 和 api_key_name 列"
```

---

## Task 3：前端 — API 类型更新

**Files:**
- Modify: `frontend/src/api/admin/ops.ts:191-242`

- [ ] **Step 1：在 `OpsRequestDetail` 接口加两个字段**

打开 `frontend/src/api/admin/ops.ts`，找到 `export interface OpsRequestDetail {`，在 `fault_owner?: FaultOwner | null` 后加入：

```typescript
/** Upstream model after mapping. Only present when upstream model differs from client model. */
upstream_model?: string | null
/** Human-readable name of the API key (e.g. "wanggao"). */
api_key_name?: string | null
```

- [ ] **Step 2：确认 TypeScript 编译通过**

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npx tsc --noEmit 2>&1 | head -20
```

期望：无新增类型错误。

- [ ] **Step 3：提交**

```bash
cd /Users/ziji/personal/github/sub2api
git add frontend/src/api/admin/ops.ts
git commit -m "Feature: OpsRequestDetail 前端类型加 upstream_model 和 api_key_name"
```

---

## Task 4：前端 — 列表视图：tableCols 拆三列 + 三个 td

**Files:**
- Modify: `frontend/src/views/admin/ops/OpsRequestInspectView.vue`

**背景：** 当前 `tableCols` 中有 `t('admin.ops.requestDetails.table.userKey')`，对应单个 `<td>` 内叠显示用户名和 masked key。目标是拆成三列：用户 / Key 名称 / 密钥。

- [ ] **Step 1：更新 `tableCols` computed**

找到 `tableCols` computed 内的列数组，将：
```typescript
  t('admin.ops.requestDetails.table.userKey'),
```
替换为：
```typescript
  t('admin.ops.requestDetails.table.user'),
  t('admin.ops.requestDetails.table.keyName'),
  t('admin.ops.requestDetails.table.keyMasked'),
```

- [ ] **Step 2：更新模板中 "用户 / Key" 的 `<td>` — 拆成三个独立 `<td>`**

找到模板中带注释 `<!-- 用户 / Key -->` 的 `<td>`，将整个 `<td>...</td>` 块替换为：

```html
<!-- 用户 -->
<td class="px-4 py-2">
  <div class="max-w-[120px] truncate text-[11px] font-medium text-gray-800 dark:text-gray-200" :title="row.user_name || ''">
    {{ row.user_name || '—' }}
  </div>
</td>
<!-- Key 名称 -->
<td class="px-4 py-2">
  <div class="max-w-[100px] truncate text-[11px] font-medium text-gray-800 dark:text-gray-200" :title="row.api_key_name || ''">
    {{ row.api_key_name || '—' }}
  </div>
</td>
<!-- 密钥 -->
<td class="px-4 py-2">
  <div class="max-w-[80px] truncate font-mono text-[10px] text-gray-500 dark:text-gray-400" :title="row.api_key_label || ''">
    {{ row.api_key_label || '—' }}
  </div>
</td>
```

- [ ] **Step 3：更新模型列 `<td>` — 加转换小字**

找到模型列的 `<td>`（含 `max-w-[160px] truncate font-mono text-[11px]` 的 div），将整个 `<td>...</td>` 替换为：

```html
<td class="px-4 py-2">
  <div class="flex flex-col gap-0.5">
    <div
      class="max-w-[160px] truncate font-mono text-[11px] text-gray-700 dark:text-gray-300"
      :title="row.model || ''"
    >
      {{ row.model || '—' }}
    </div>
    <div
      v-if="row.upstream_model && row.upstream_model !== row.model"
      class="flex items-center gap-0.5 max-w-[160px]"
    >
      <span class="text-[9px] text-amber-500 dark:text-amber-400">↓</span>
      <span
        class="truncate font-mono text-[10px] text-amber-600 dark:text-amber-400"
        :title="row.upstream_model"
      >{{ row.upstream_model }}</span>
    </div>
  </div>
</td>
```

- [ ] **Step 4：确认 TypeScript 编译**

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npx tsc --noEmit 2>&1 | head -20
```

期望：无新增类型错误。

- [ ] **Step 5：提交**

```bash
cd /Users/ziji/personal/github/sub2api
git add frontend/src/views/admin/ops/OpsRequestInspectView.vue
git commit -m "Feature: 请求排查列表拆用户/Key名称/密钥三列，模型列显示转换"
```

---

## Task 5：前端 — 详情面板：模型格子视觉强调

**Files:**
- Modify: `frontend/src/views/admin/ops/components/OpsRequestDetailPanel.vue`

**背景：** 详情面板里已有"客户端模型"和"上游模型"两个格子，用 `usageInspect`（usage-inspect API 返回）的数据。此 task 在有映射时给上游模型格子加视觉强调（橙色高亮 + 箭头标记），让管理员一眼看出有转换。

- [ ] **Step 1：修改"上游模型"格子，当与客户端模型不同时加视觉标记**

找到 `OpsRequestDetailPanel.vue` 中 `fields.upstreamModel` 的格子：

```html
<div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900">
  <div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.requestInspect.fields.upstreamModel') }}</div>
  <div class="mt-1 font-mono text-xs text-gray-900 dark:text-white">
    {{ usageUpstreamModelDisplay }}
  </div>
</div>
```

替换为：

```html
<div
  class="rounded-lg p-3"
  :class="isModelMapped
    ? 'bg-amber-50 dark:bg-amber-950/30 ring-1 ring-amber-200 dark:ring-amber-800/50'
    : 'bg-gray-50 dark:bg-dark-900'"
>
  <div class="flex items-center gap-1">
    <span
      class="text-[10px] font-bold uppercase"
      :class="isModelMapped ? 'text-amber-600 dark:text-amber-400' : 'text-gray-400'"
    >{{ t('admin.ops.requestInspect.fields.upstreamModel') }}</span>
    <span v-if="isModelMapped" class="text-[10px] font-bold text-amber-500 dark:text-amber-400">↓ 已映射</span>
  </div>
  <div
    class="mt-1 font-mono text-xs"
    :class="isModelMapped ? 'text-amber-700 dark:text-amber-300 font-bold' : 'text-gray-900 dark:text-white'"
  >
    {{ usageUpstreamModelDisplay }}
  </div>
</div>
```

- [ ] **Step 2：在 `<script setup>` 中加 `isModelMapped` computed**

找到 `usageUpstreamModelDisplay` computed，在其后加入：

```typescript
/** true when the upstream model differs from the client-requested model */
const isModelMapped = computed(() => {
  const u = usageInspect.value
  if (!u) return false
  const clientModel = (u.model || '').trim()
  const upstreamModel = (u.upstream_model || '').trim()
  return upstreamModel !== '' && upstreamModel !== clientModel
})
```

- [ ] **Step 3：确认 TypeScript 编译**

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npx tsc --noEmit 2>&1 | head -20
```

期望：无新增类型错误。

- [ ] **Step 4：提交**

```bash
cd /Users/ziji/personal/github/sub2api
git add frontend/src/views/admin/ops/components/OpsRequestDetailPanel.vue
git commit -m "Feature: 详情面板上游模型格子在有映射时高亮显示"
```

---

## Task 6：i18n 补全缺少的 key

**Files:**
- Modify: `frontend/src/i18n/` 下的语言文件（zh/en）

**背景：** Task 4 引入了 `t('admin.ops.requestDetails.table.user')`、`t('admin.ops.requestDetails.table.keyName')`、`t('admin.ops.requestDetails.table.keyMasked')` 三个新 key，需要在 i18n 文件中补全。

- [ ] **Step 1：找到 i18n 文件路径及 `userKey` 的位置**

```bash
grep -rn "userKey\|user_key" /Users/ziji/personal/github/sub2api/frontend/src/i18n/ | head -10
```

记录文件路径（通常是 `zh.ts`/`en.ts` 或 `zh.json`/`en.json`）。

- [ ] **Step 2：在所有语言文件中，在 `userKey` 条目旁加三个新 key**

中文示例（在 `userKey: '用户 / Key'` 附近加）：

```typescript
user: '用户',
keyName: 'Key 名称',
keyMasked: '密钥',
```

英文示例：

```typescript
user: 'User',
keyName: 'Key Name',
keyMasked: 'Key',
```

- [ ] **Step 3：确认 TypeScript 编译**

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Step 4：提交**

```bash
cd /Users/ziji/personal/github/sub2api
git add frontend/src/i18n/
git commit -m "Feature: i18n 补全用户/Key名称/密钥三列标题"
```

---

## Task 7：构建验证

- [ ] **Step 1：后端完整构建**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go build ./...
```

期望：无错误。

- [ ] **Step 2：后端完整测试**

```bash
cd /Users/ziji/personal/github/sub2api/backend
go test ./internal/service/... ./internal/repository/... -count=1 2>&1 | tail -20
```

期望：PASS（集成测试可能 skip，属正常）。

- [ ] **Step 3：前端生产构建**

```bash
cd /Users/ziji/personal/github/sub2api/frontend
npm run build 2>&1 | tail -20
```

期望：`built in Xs` 无 error。

- [ ] **Step 4：最终提交（如有未提交内容）**

```bash
cd /Users/ziji/personal/github/sub2api
git status
```

确认所有变更已提交。

---

## 验收标准

1. 请求排查列表表头由 "用户/Key" 一列变为 "用户"、"Key 名称"、"密钥" 三列
2. 当 `api_key_name` 存在时，Key 名称列显示 key 的名字（如 `wanggao`）
3. 当 `upstream_model` 与 `model` 不同时，列表模型列下方显示橙色 `↓ gpt-5.4`
4. 详情面板右侧"上游模型"格子在有映射时呈现橙色高亮 + "↓ 已映射" 标记
5. 无映射时（`upstream_model` 为 nil 或等于 `model`）所有样式保持原样
