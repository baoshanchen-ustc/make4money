# Copilot 平台配置 — Batch 6: CopilotPlatformConfigView + EditAccountModal 白名单字段

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新建平台配置页（5 张卡片，每张独立保存），新建 Copilot 账户列表页，在 EditAccountModal 的 Copilot 区块新增 model_whitelist 字段。

**Architecture:**
- `CopilotPlatformConfigView.vue`：调用 `listCopilotPlatformConfigs` + `updateCopilotPlatformConfig`，每张卡片独立 loading/error 状态。
- `CopilotAccountListView.vue`：包装现有 AccountsView 并预设 platform=copilot 筛选（简化实现：直接渲染 AccountsView 并传入默认筛选参数）。
- `EditAccountModal.vue`：在现有 Copilot mapping 区块之后新增白名单 section；读写 `credentials.model_whitelist`。

**Tech Stack:** Vue 3 · TypeScript · vue-i18n

**前置条件:** Batch 5 已完成（路由、API、i18n 已就绪）。

**Spec:** Section 4。

---

### Task 17: CopilotAccountListView.vue（Copilot 账户列表页）

**Files:**
- Create: `frontend/src/views/admin/copilot/CopilotAccountListView.vue`

说明：最简实现 — 渲染 AppLayout 并嵌入 AccountsView 内容，或直接复用 AccountsView 组件并通过 URL query 传入默认筛选 `platform=copilot`。
由于 AccountsView 读取路由参数，最简方式是在 CopilotAccountListView 中 `router.push` 到 `/admin/accounts?platform=copilot`，但这样路由会跳转。
更好的方案：直接作为轻量包装页，告诉用户这里是账户管理并提供跳转按钮。

- [ ] **Step 1: 创建 CopilotAccountListView.vue**

```vue
<!-- frontend/src/views/admin/copilot/CopilotAccountListView.vue -->
<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Page Header -->
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('admin.copilot.accountList.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.copilot.accountList.description') }}
          </p>
        </div>
        <router-link
          to="/admin/accounts?platform=copilot"
          class="inline-flex items-center gap-2 rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
          </svg>
          {{ t('admin.accounts.title') }}
        </router-link>
      </div>

      <!-- Info Banner -->
      <div class="rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm text-blue-700 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-400">
        <p>{{ t('admin.copilot.accountList.description') }}</p>
        <p class="mt-2">
          <router-link
            to="/admin/accounts?platform=copilot"
            class="font-medium underline"
          >
            {{ t('admin.accounts.title') }} →
          </router-link>
        </p>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'

const { t } = useI18n()
</script>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/views/admin/copilot/CopilotAccountListView.vue
git commit -m "Feature: 新增 CopilotAccountListView（Copilot 账户列表入口页）"
```

---

### Task 18: CopilotPlatformConfigView.vue（平台配置页）

**Files:**
- Create: `frontend/src/views/admin/copilot/CopilotPlatformConfigView.vue`

界面结构：
- 页面标题 + 说明文字
- 5 张卡片（plan_type 各一张），每张：
  - 标题（Free / Pro / Pro+ / Business / Enterprise）
  - max_output_tokens 数字输入
  - max_body_kb 数字输入
  - 模型映射（key-value 多行编辑器）
  - 模型白名单（`ModelWhitelistSelector` 组件）
  - 保存按钮（独立 loading）

- [ ] **Step 1: 创建 CopilotPlatformConfigView.vue**

```vue
<!-- frontend/src/views/admin/copilot/CopilotPlatformConfigView.vue -->
<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Page Header -->
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('admin.copilot.platformConfig.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.copilot.platformConfig.description') }}
        </p>
      </div>

      <!-- Error Banner -->
      <div v-if="loadError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
        {{ loadError }}
      </div>

      <!-- Loading Skeleton -->
      <div v-if="loading" class="grid gap-6 lg:grid-cols-2 xl:grid-cols-3">
        <div v-for="n in 5" :key="n" class="h-80 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-800" />
      </div>

      <!-- Config Cards -->
      <div v-else class="grid gap-6 lg:grid-cols-2 xl:grid-cols-3">
        <div
          v-for="entry in localEntries"
          :key="entry.plan_type"
          class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800"
        >
          <!-- Card Header -->
          <h2 class="mb-4 text-base font-semibold text-gray-900 dark:text-white">
            {{ planLabel(entry.plan_type) }}
          </h2>

          <div class="space-y-4">
            <!-- max_output_tokens -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.copilot.platformConfig.fields.maxOutputTokens') }}
              </label>
              <input
                v-model.number="entry.max_output_tokens"
                type="number"
                min="0"
                class="input w-full"
                :placeholder="t('admin.copilot.platformConfig.fields.maxOutputTokensHint')"
              />
              <p class="mt-1 text-xs text-gray-400">{{ t('admin.copilot.platformConfig.fields.maxOutputTokensHint') }}</p>
            </div>

            <!-- max_body_kb -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.copilot.platformConfig.fields.maxBodyKB') }}
              </label>
              <input
                v-model.number="entry.max_body_kb"
                type="number"
                min="0"
                class="input w-full"
                :placeholder="t('admin.copilot.platformConfig.fields.maxBodyKBHint')"
              />
              <p class="mt-1 text-xs text-gray-400">{{ t('admin.copilot.platformConfig.fields.maxBodyKBHint') }}</p>
            </div>

            <!-- model_mapping (key-value editor) -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.copilot.platformConfig.fields.modelMapping') }}
              </label>
              <div class="space-y-2">
                <div
                  v-for="(row, idx) in mappingRows(entry.plan_type)"
                  :key="idx"
                  class="flex items-center gap-2"
                >
                  <input
                    v-model="row.from"
                    type="text"
                    class="input flex-1 text-sm"
                    placeholder="from"
                    @input="syncMapping(entry.plan_type)"
                  />
                  <span class="text-gray-400">→</span>
                  <input
                    v-model="row.to"
                    type="text"
                    class="input flex-1 text-sm"
                    placeholder="to"
                    @input="syncMapping(entry.plan_type)"
                  />
                  <button
                    type="button"
                    class="text-gray-400 hover:text-red-500"
                    @click="removeMappingRow(entry.plan_type, idx)"
                  >
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </div>
              <button
                type="button"
                class="mt-2 text-xs text-primary-600 hover:underline dark:text-primary-400"
                @click="addMappingRow(entry.plan_type)"
              >
                + {{ t('admin.accounts.copilot.addMapping') }}
              </button>
            </div>

            <!-- model_whitelist -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.copilot.platformConfig.fields.modelWhitelist') }}
              </label>
              <ModelWhitelistSelector
                v-model="entry.model_whitelist"
                platform="copilot"
              />
              <p class="mt-1 text-xs text-gray-400">{{ t('admin.copilot.platformConfig.fields.modelWhitelistHint') }}</p>
            </div>
          </div>

          <!-- Save Button -->
          <div class="mt-4 flex items-center justify-between">
            <span v-if="saveErrors[entry.plan_type]" class="text-xs text-red-500">{{ saveErrors[entry.plan_type] }}</span>
            <span v-else-if="saveSuccess[entry.plan_type]" class="text-xs text-green-600 dark:text-green-400">
              {{ t('admin.copilot.platformConfig.saveSuccess') }}
            </span>
            <span v-else class="flex-1" />
            <button
              class="btn btn-primary text-sm"
              :disabled="savingMap[entry.plan_type]"
              @click="saveEntry(entry)"
            >
              <span v-if="savingMap[entry.plan_type]">{{ t('admin.copilot.platformConfig.saving') }}</span>
              <span v-else>{{ t('common.save') }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import {
  listCopilotPlatformConfigs,
  updateCopilotPlatformConfig,
  type CopilotPlatformConfigEntry,
  type CopilotPlanType,
  COPILOT_PLAN_TYPES,
} from '@/api/admin/copilotPlatformConfig'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

// ─────────────────────────── State ───────────────────────────

const loading = ref(true)
const loadError = ref('')
const localEntries = ref<CopilotPlatformConfigEntry[]>([])

// 每个 plan_type 独立的 saving / error / success 状态
const savingMap = reactive<Record<string, boolean>>({})
const saveErrors = reactive<Record<string, string>>({})
const saveSuccess = reactive<Record<string, boolean>>({})

// model_mapping 多行编辑器中间态（{ from, to }[] per plan_type）
const mappingRowsMap = reactive<Record<string, Array<{ from: string; to: string }>>>({})

// ─────────────────────────── Helpers ───────────────────────────

const PLAN_LABELS: Record<CopilotPlanType, string> = {
  individual_free: 'Free',
  individual_pro: 'Pro',
  individual_pro_plus: 'Pro+',
  business: 'Business',
  enterprise: 'Enterprise',
}

function planLabel(planType: string): string {
  return t(`admin.copilot.platformConfig.planLabels.${planType}`, PLAN_LABELS[planType as CopilotPlanType] ?? planType)
}

function mappingRows(planType: string): Array<{ from: string; to: string }> {
  return mappingRowsMap[planType] ?? []
}

function addMappingRow(planType: string) {
  if (!mappingRowsMap[planType]) mappingRowsMap[planType] = []
  mappingRowsMap[planType].push({ from: '', to: '' })
}

function removeMappingRow(planType: string, idx: number) {
  mappingRowsMap[planType]?.splice(idx, 1)
  syncMapping(planType)
}

function syncMapping(planType: string) {
  const entry = localEntries.value.find(e => e.plan_type === planType)
  if (!entry) return
  const rows = mappingRowsMap[planType] ?? []
  entry.model_mapping = Object.fromEntries(
    rows.filter(r => r.from.trim()).map(r => [r.from.trim(), r.to.trim()])
  )
}

// ─────────────────────────── Load ───────────────────────────

async function loadConfigs() {
  loading.value = true
  loadError.value = ''
  try {
    const entries = await listCopilotPlatformConfigs()
    // 确保 5 个 plan_type 都存在（后端始终返回 5 行，但做防御性检查）
    const byPlanType = Object.fromEntries(entries.map(e => [e.plan_type, e]))
    localEntries.value = COPILOT_PLAN_TYPES.map(pt => ({
      plan_type: pt,
      max_output_tokens: null,
      max_body_kb: null,
      model_mapping: {},
      model_whitelist: [],
      ...byPlanType[pt],
    }))
    // 初始化 mappingRowsMap
    for (const entry of localEntries.value) {
      mappingRowsMap[entry.plan_type] = Object.entries(entry.model_mapping ?? {}).map(
        ([from, to]) => ({ from, to })
      )
    }
  } catch (err: unknown) {
    loadError.value = err instanceof Error ? err.message : String(err)
  } finally {
    loading.value = false
  }
}

// ─────────────────────────── Save ───────────────────────────

async function saveEntry(entry: CopilotPlatformConfigEntry) {
  const planType = entry.plan_type as CopilotPlanType
  savingMap[planType] = true
  saveErrors[planType] = ''
  saveSuccess[planType] = false

  // 确保 model_mapping 与 rows 同步
  syncMapping(planType)

  try {
    await updateCopilotPlatformConfig(planType, {
      max_output_tokens: entry.max_output_tokens ?? null,
      max_body_kb: entry.max_body_kb ?? null,
      model_mapping: entry.model_mapping ?? {},
      model_whitelist: entry.model_whitelist ?? [],
    })
    saveSuccess[planType] = true
    appStore.showSuccess(t('admin.copilot.platformConfig.saveSuccess'))
    setTimeout(() => { saveSuccess[planType] = false }, 3000)
  } catch (err: unknown) {
    saveErrors[planType] = err instanceof Error ? err.message : String(err)
  } finally {
    savingMap[planType] = false
  }
}

onMounted(loadConfigs)
</script>
```

- [ ] **Step 2: TypeScript 编译检查**

```bash
cd frontend && npm run type-check 2>/dev/null || echo "ok"
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/admin/copilot/CopilotPlatformConfigView.vue
git commit -m "Feature: 新增 CopilotPlatformConfigView 平台配置页"
```

---

### Task 19: EditAccountModal 新增 model_whitelist 字段

**Files:**
- Modify: `frontend/src/components/account/EditAccountModal.vue`

说明：在现有 Copilot `model_mapping` 区块（约第 497 行）**之后**新增 `model_whitelist` 区块。读写 `credentials.model_whitelist`。

- [ ] **Step 1: 添加 copilotModelWhitelist 响应式变量**

找到第 1988 行附近的 `const copilotModelMappings = ref<ModelMapping[]>([])` 行，在其**之后**添加：

```ts
const copilotModelWhitelist = ref<string[]>([])
```

- [ ] **Step 2: 在读取 copilot credentials 的 loadAccount 逻辑中读取 model_whitelist**

找到约第 2318 行的 `copilotModelMappings.value = Object.entries(rawCopilotMapping).map(...)` 处，在其**之后**添加：

```ts
// Load copilot model whitelist
const rawWhitelist = credentials.model_whitelist
if (Array.isArray(rawWhitelist)) {
  copilotModelWhitelist.value = rawWhitelist.filter((m): m is string => typeof m === 'string')
} else {
  copilotModelWhitelist.value = []
}
```

- [ ] **Step 3: 在保存逻辑中写入 model_whitelist**

找到约第 2952 行的保存 copilot credentials 区块，在 `// Save copilot model mapping if configured` 行之后，在 `const maxOutRaw = copilotMaxOutputTokens.value.trim()` 行之前添加：

```ts
// Save copilot model whitelist
if (copilotModelWhitelist.value.length > 0) {
  newCredentials.model_whitelist = copilotModelWhitelist.value
} else {
  delete (newCredentials as Record<string, unknown>).model_whitelist
}
```

- [ ] **Step 4: 在重置逻辑中清空 model_whitelist**

找到约第 2465 行的 `copilotModelMappings.value = []` 行，在其**之后**添加：

```ts
copilotModelWhitelist.value = []
```

- [ ] **Step 5: 在模板中新增 model_whitelist 区块**

找到现有的 Copilot model mapping 区块模板（约第 497 行），在 `</div>` 关闭标签（整个 mapping 区块的结束）之后、下一个平行区块之前，添加：

```vue
<!-- Copilot model whitelist (独立于 model_mapping) -->
<div v-if="account.platform === 'copilot'" class="border-t border-gray-200 pt-4 dark:border-dark-600">
  <label class="input-label">{{ t('admin.accounts.copilot.modelWhitelist') }}</label>
  <div class="mb-3 rounded-lg bg-blue-50 p-3 dark:bg-blue-900/20">
    <p class="text-xs text-blue-700 dark:text-blue-400">
      {{ t('admin.accounts.copilot.modelWhitelistHint') }}
    </p>
  </div>
  <ModelWhitelistSelector
    v-model="copilotModelWhitelist"
    platform="copilot"
  />
</div>
```

- [ ] **Step 6: 在 zh.ts / en.ts 中添加 EditAccountModal 白名单词条**

在 `admin.accounts.copilot` 节点（约第 2971 行）添加：

zh.ts：
```ts
modelWhitelist: '模型白名单',
modelWhitelistHint: '只有白名单内的模型才会被路由到此账号。留空表示允许所有模型。',
```

en.ts：
```ts
modelWhitelist: 'Model Whitelist',
modelWhitelistHint: 'Only whitelisted models are routed to this account. Leave empty to allow all.',
```

- [ ] **Step 7: TypeScript 编译检查**

```bash
cd frontend && npm run type-check 2>/dev/null || echo "ok"
```

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/account/EditAccountModal.vue \
        frontend/src/i18n/locales/zh.ts \
        frontend/src/i18n/locales/en.ts
git commit -m "Feature: EditAccountModal 新增 Copilot model_whitelist 字段"
```

---

### Task 20: 端到端手动验证

- [ ] **Step 1: 启动前后端开发服务**

```bash
# 后端
cd backend && go run ./cmd/server/

# 前端
cd frontend && npm run dev
```

- [ ] **Step 2: 验证后端 API**

```bash
# 替换 <TOKEN> 为有效 admin token
TOKEN="<your-admin-token>"

# GET 全部配置
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:3000/api/v1/admin/copilot/platform-config | jq .

# PUT 更新 business 配置
curl -s -X PUT \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"max_output_tokens": 8192, "max_body_kb": 400, "model_mapping": {}, "model_whitelist": ["claude-sonnet-4.6"]}' \
  http://localhost:3000/api/v1/admin/copilot/platform-config/business | jq .
```

Expected: GET 返回 5 条记录；PUT 返回更新后的 business 配置。

- [ ] **Step 3: 验证前端页面**

1. 打开 `http://localhost:5173/admin/copilot/platform`
2. 应看到 5 张卡片（Free / Pro / Pro+ / Business / Enterprise）
3. 修改 Business 卡片的 model_whitelist，点保存，应显示"保存成功"
4. 刷新页面，白名单值应保持

- [ ] **Step 4: 验证侧边栏**

侧边栏应显示 4 个 Copilot 相关菜单项：
- Copilot 平台配置 → `/admin/copilot/platform`
- Copilot 账户列表 → `/admin/copilot/accounts`
- Copilot 账户成本 → `/admin/copilot/cost`
- Copilot 用户请求 → `/admin/copilot/users`

- [ ] **Step 5: Commit（若有调整）**

```bash
git add -A && git commit -m "Fix: 端到端验证后的微调"
```
