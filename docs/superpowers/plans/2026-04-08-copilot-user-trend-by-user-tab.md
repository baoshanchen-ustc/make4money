# Copilot 用户请求趋势 — 按用户 Tab 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在"用户请求趋势"卡片新增"按用户"Tab，选择某个用户后展示其 Premium / Agent / 总量三条折线。

**Architecture:** 新增独立组件 `UserSingleChart.vue` 接受 `userId` + `dailyData` prop（数据由父组件传入，零额外请求）；父组件 `CopilotUsersView.vue` 新增 `trendTab` 和 `selectedUserId` 状态，切 Tab 时渲染对应图表组件。

**Tech Stack:** Vue 3 (Composition API), Chart.js, TypeScript

---

## 文件变更概览

| 操作 | 文件 |
|------|------|
| **新建** | `frontend/src/components/admin/copilot/UserSingleChart.vue` |
| **修改** | `frontend/src/views/admin/copilot/CopilotUsersView.vue` |
| **新建** | `frontend/src/components/admin/copilot/__tests__/UserSingleChart.spec.ts` |

---

## Task 1: 新建 UserSingleChart.vue 组件

**Files:**
- Create: `frontend/src/components/admin/copilot/UserSingleChart.vue`

### 说明

组件接收父组件已加载好的 `dailyData`（`CopilotUsersDailyStatsResult`）和 `userId`，从中筛选出该用户的每日数据，渲染 3 条折线：Premium / Agent / 总量。无需发起任何 API 请求。

- [ ] **Step 1: 新建组件文件**

创建 `frontend/src/components/admin/copilot/UserSingleChart.vue`，内容如下：

```vue
<template>
  <div class="relative">
    <div v-if="!userId" class="flex h-[300px] items-center justify-center text-sm text-gray-400">
      请选择一个用户
    </div>
    <div v-else-if="!dailyData" class="flex h-[300px] items-center justify-center text-sm text-gray-400">
      暂无数据
    </div>
    <div v-else class="h-[300px]">
      <canvas ref="chartRef" class="!h-full !w-full" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import {
  Chart,
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  CategoryScale,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import type { CopilotUsersDailyStatsResult } from '@/api/admin/copilotAnalytics'

Chart.register(LineController, LineElement, PointElement, LinearScale, CategoryScale, Tooltip, Legend, Filler)

const props = withDefaults(defineProps<{
  days?: number
  userId: number | null
  dailyData: CopilotUsersDailyStatsResult | null
}>(), {
  days: 30,
})

const chartRef = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null

/** 生成最近 N 天的日期字符串数组（本地时区，无 UTC 偏移） */
function buildDateRange(days: number): string[] {
  const dates: string[] = []
  const today = new Date()
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(today)
    d.setDate(d.getDate() - i)
    dates.push(
      `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`,
    )
  }
  return dates
}

function buildChart() {
  if (!props.userId || !props.dailyData || !chartRef.value) return

  const dates = buildDateRange(props.days ?? 30)

  // 按日期汇总该用户的 premium / agent
  const premiumByDate = new Map<string, number>()
  const agentByDate = new Map<string, number>()

  for (const entry of props.dailyData.days) {
    if (entry.user_id !== props.userId) continue
    premiumByDate.set(entry.date, (premiumByDate.get(entry.date) ?? 0) + entry.premium_count)
    agentByDate.set(entry.date, (agentByDate.get(entry.date) ?? 0) + entry.agent_count)
  }

  const premiumData = dates.map(d => premiumByDate.get(d) ?? 0)
  const agentData = dates.map(d => agentByDate.get(d) ?? 0)
  const totalData = dates.map((_, i) => premiumData[i] + agentData[i])

  const dark = document.documentElement.classList.contains('dark')
  const tickColor = dark ? '#9ca3af' : '#6b7280'
  const gridColor = dark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.06)'
  const legendColor = dark ? '#d1d5db' : '#374151'

  const pointRadius = (props.days ?? 30) <= 14 ? 3 : 0

  chart?.destroy()
  chart = new Chart(chartRef.value, {
    type: 'line',
    data: {
      labels: dates,
      datasets: [
        {
          label: 'Premium',
          data: premiumData,
          borderColor: '#10b981',
          backgroundColor: '#10b98118',
          borderWidth: 2,
          pointRadius,
          pointHoverRadius: 5,
          tension: 0.3,
          fill: false,
        },
        {
          label: 'Agent',
          data: agentData,
          borderColor: '#3b82f6',
          backgroundColor: '#3b82f618',
          borderWidth: 2,
          pointRadius,
          pointHoverRadius: 5,
          tension: 0.3,
          fill: false,
        },
        {
          label: '总量',
          data: totalData,
          borderColor: '#8b5cf6',
          backgroundColor: '#8b5cf618',
          borderWidth: 2,
          pointRadius,
          pointHoverRadius: 5,
          tension: 0.3,
          fill: false,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        legend: {
          position: 'bottom',
          labels: { boxWidth: 12, padding: 16, font: { size: 12 }, color: legendColor },
        },
        tooltip: {
          callbacks: {
            title: (items) => items[0]?.label ?? '',
            label: (item) => ` ${item.dataset.label}: ${item.parsed.y} 次`,
          },
        },
      },
      scales: {
        x: {
          grid: { display: false },
          ticks: { maxTicksLimit: 10, font: { size: 11 }, color: tickColor },
        },
        y: {
          beginAtZero: true,
          grid: { color: gridColor },
          ticks: { font: { size: 11 }, color: tickColor },
        },
      },
    },
  })
}

async function rebuild() {
  await nextTick()
  buildChart()
}

onMounted(rebuild)
watch(() => [props.userId, props.dailyData, props.days], rebuild)
onBeforeUnmount(() => chart?.destroy())
</script>
```

- [ ] **Step 2: 确认文件存在**

```bash
ls frontend/src/components/admin/copilot/UserSingleChart.vue
```

Expected: 文件路径输出，无报错。

---

## Task 2: 修改 CopilotUsersView.vue — 新增 Tab 状态与用户选择器

**Files:**
- Modify: `frontend/src/views/admin/copilot/CopilotUsersView.vue`

### 说明

在现有"趋势折线图卡片"区域（第 83–108 行）：
1. 新增 `trendTab` 状态（`'metric' | 'user'`，默认 `'metric'`）
2. 新增 `selectedUserId` 状态（默认取 `topUser.userId`），用 `watch(topUser)` 初始化一次
3. 头部新增 Tab 切换：`[按指标] [按用户]`；当 Tab 为 `'user'` 时额外渲染用户选择器 `<select>`
4. 图表区按 `trendTab` 条件渲染：`metric` → `UsersDailyChart`，`user` → `UserSingleChart`
5. 引入 `UserSingleChart` 组件

- [ ] **Step 1: 在 `<script setup>` 中补充 import 和状态**

找到 `CopilotUsersView.vue` 第 212–229 行的 `<script setup>` 开头，**在现有 import 列表末尾追加** UserSingleChart 的 import：

```typescript
import UserSingleChart from '@/components/admin/copilot/UserSingleChart.vue'
```

- [ ] **Step 2: 在 State 区块新增两个 ref**

找到文件中 `// ─── State` 区块（约第 277–286 行），在 `loading` 那行之前插入：

```typescript
type TrendTab = 'metric' | 'user'
const trendTab = ref<TrendTab>('metric')
const selectedUserId = ref<number | null>(null)
```

- [ ] **Step 3: 在 KPI computeds 区块之后新增 watch，初始化 selectedUserId**

在 `topUser` computed（约第 386–393 行）之后追加：

```typescript
// 首次加载后把默认选中用户设为 topUser（当日 Premium 最多）
watch(topUser, (val) => {
  if (val && selectedUserId.value === null) {
    selectedUserId.value = val.userId
  }
}, { immediate: true })
```

- [ ] **Step 4: 替换卡片头部 — 新增 Tab + 用户选择器**

找到 template 中"趋势折线图卡片"头部（第 84–108 行），将整个 `<div class="rounded-lg border ...">` 内的头部 div（`flex flex-col gap-3 border-b ...`）替换为：

```html
<!-- 趋势折线图卡片 -->
<div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
  <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-gray-700 sm:flex-row sm:items-center sm:justify-between">
    <div>
      <h2 class="text-sm font-semibold text-gray-900 dark:text-white">用户请求趋势</h2>
      <p class="text-xs text-gray-400">近 {{ selectedDays }} 天按用户拆分</p>
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <!-- Tab 切换 -->
      <div class="flex items-center gap-1 rounded-lg border border-gray-200 p-1 dark:border-gray-700">
        <button
          class="rounded px-2.5 py-1 text-xs font-semibold transition-colors"
          :class="trendTab === 'metric'
            ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900'
            : 'text-gray-500 hover:bg-gray-100 hover:text-gray-800 dark:text-gray-400 dark:hover:bg-gray-700'"
          @click="trendTab = 'metric'"
        >
          按指标
        </button>
        <button
          class="rounded px-2.5 py-1 text-xs font-semibold transition-colors"
          :class="trendTab === 'user'
            ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900'
            : 'text-gray-500 hover:bg-gray-100 hover:text-gray-800 dark:text-gray-400 dark:hover:bg-gray-700'"
          @click="trendTab = 'user'"
        >
          按用户
        </button>
      </div>
      <!-- 按指标时的指标选择器 -->
      <div
        v-if="trendTab === 'metric'"
        class="flex items-center gap-1 rounded-lg border border-gray-200 p-1 dark:border-gray-700"
      >
        <button
          v-for="m in METRIC_OPTIONS"
          :key="m.value"
          class="rounded px-2.5 py-1 text-xs font-semibold transition-colors"
          :class="chartMetric === m.value
            ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900'
            : 'text-gray-500 hover:bg-gray-100 hover:text-gray-800 dark:text-gray-400 dark:hover:bg-gray-700'"
          @click="chartMetric = m.value"
        >
          {{ m.label }}
        </button>
      </div>
      <!-- 按用户时的用户选择器 -->
      <select
        v-if="trendTab === 'user'"
        v-model="selectedUserId"
        class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs font-medium text-gray-700 shadow-sm focus:border-blue-500 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200"
      >
        <option
          v-for="user in aggregatedUsers"
          :key="user.userId"
          :value="user.userId"
        >
          {{ user.username }}
        </option>
      </select>
    </div>
  </div>
  <div class="p-4">
    <UsersDailyChart v-if="trendTab === 'metric'" :days="selectedDays" :metric="chartMetric" />
    <UserSingleChart
      v-else
      :days="selectedDays"
      :user-id="selectedUserId"
      :daily-data="dailyData"
    />
  </div>
</div>
```

- [ ] **Step 5: 确认 template 编译无报错**

```bash
cd /Users/ziji/personal/github/sub2api/frontend && npx vue-tsc --noEmit 2>&1 | head -40
```

Expected: 无 error 输出（warning 可忽略）。

---

## Task 3: 单元测试 UserSingleChart.vue

**Files:**
- Create: `frontend/src/components/admin/copilot/__tests__/UserSingleChart.spec.ts`

### 说明

测试组件的三种状态：无 userId 时显示提示、有数据时正确过滤用户数据生成 3 条 dataset。使用 vitest + vue-test-utils，与项目现有测试模式一致（参考 `AccountsDailyChart.spec.ts`）。

- [ ] **Step 1: 先查看现有测试文件作为参考**

```bash
cat frontend/src/components/admin/copilot/__tests__/AccountsDailyChart.spec.ts
```

- [ ] **Step 2: 新建测试文件**

创建 `frontend/src/components/admin/copilot/__tests__/UserSingleChart.spec.ts`：

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import UserSingleChart from '../UserSingleChart.vue'
import type { CopilotUsersDailyStatsResult } from '@/api/admin/copilotAnalytics'

// Mock Chart.js — 避免 canvas 渲染问题
vi.mock('chart.js', () => {
  const Chart = vi.fn().mockImplementation(() => ({ destroy: vi.fn() }))
  return {
    Chart,
    LineController: {},
    LineElement: {},
    PointElement: {},
    LinearScale: {},
    CategoryScale: {},
    Tooltip: {},
    Legend: {},
    Filler: {},
  }
})

const MOCK_DAILY_DATA: CopilotUsersDailyStatsResult = {
  users: [
    { user_id: 1, username: 'alice' },
    { user_id: 2, username: 'bob' },
  ],
  days: [
    { user_id: 1, date: '2026-04-07', premium_count: 10, agent_count: 3 },
    { user_id: 1, date: '2026-04-08', premium_count: 5, agent_count: 2 },
    { user_id: 2, date: '2026-04-07', premium_count: 8, agent_count: 1 },
  ],
}

describe('UserSingleChart', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('当 userId 为 null 时显示提示文案', () => {
    const wrapper = mount(UserSingleChart, {
      props: { userId: null, dailyData: MOCK_DAILY_DATA },
    })
    expect(wrapper.text()).toContain('请选择一个用户')
  })

  it('当 dailyData 为 null 时显示暂无数据', () => {
    const wrapper = mount(UserSingleChart, {
      props: { userId: 1, dailyData: null },
    })
    expect(wrapper.text()).toContain('暂无数据')
  })

  it('当 userId 和 dailyData 都有效时渲染 canvas', () => {
    const wrapper = mount(UserSingleChart, {
      props: { userId: 1, dailyData: MOCK_DAILY_DATA },
      attachTo: document.body,
    })
    expect(wrapper.find('canvas').exists()).toBe(true)
  })
})
```

- [ ] **Step 3: 运行测试**

```bash
cd /Users/ziji/personal/github/sub2api/frontend && npx vitest run src/components/admin/copilot/__tests__/UserSingleChart.spec.ts
```

Expected: 3 tests pass，0 fail。

- [ ] **Step 4: Commit**

```bash
cd /Users/ziji/personal/github/sub2api
git add frontend/src/components/admin/copilot/UserSingleChart.vue \
        frontend/src/components/admin/copilot/__tests__/UserSingleChart.spec.ts \
        frontend/src/views/admin/copilot/CopilotUsersView.vue
git commit -m "Feature: 用户请求趋势新增按用户 Tab，支持选择用户查看 Premium/Agent/总量三条折线"
```

---

## Self-Review

**Spec coverage:**
- ✅ 新增 Tab（按指标 / 按用户）切换
- ✅ 默认展示当日 Premium 最多的用户（`watch(topUser)` 初始化 `selectedUserId`）
- ✅ 用户选择器，切换用户后图表联动
- ✅ 三条折线：Premium / Agent / 总量
- ✅ 时间范围跟随父组件 `selectedDays`（7/14/30/60 天）
- ✅ 零额外 API 请求（`dailyData` 从父组件传入）

**Placeholder scan:** 无 TBD / TODO。

**Type consistency:**
- `CopilotUsersDailyStatsResult` 贯穿全部任务，来自 `@/api/admin/copilotAnalytics`，与现有代码一致
- `selectedUserId: number | null` 在 Task 2 定义，在 Task 2 Step 4 的 `:user-id` prop 传入，与 `UserSingleChart` props 中 `userId: number | null` 完全对应
- `dailyData` 类型与父组件已有的 `dailyData` ref 类型一致
