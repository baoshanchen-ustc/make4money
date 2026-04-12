# Copilot 平台配置 — Batch 5: 前端路由 + 侧边栏 + API + i18n

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重组 Copilot 菜单分组（新增"平台配置"和"账户列表"路由，原"成本分析"路由不变），更新侧边栏为 Copilot 分组，新增 API 调用层和 i18n 词条。

**Architecture:** 路由重组不使用重定向（用户没有书签）。侧边栏 Copilot 菜单项直接平铺在 `adminNavItems` 中（现有平铺结构，无需 group 折叠 UI）。

**Tech Stack:** Vue 3 · TypeScript · vue-i18n

**前置条件:** Batch 3 已完成（后端 API 已上线）。

**Spec:** Section 4，Section 1（菜单结构与路由）。

---

### Task 13: i18n 词条

**Files:**
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

- [ ] **Step 1: 在 zh.ts 的 nav 节点添加新词条**

找到 `nav` 对象（约第 335 行，含 `copilotUsers`、`copilotAccounts` 词条），添加：

```ts
copilotPlatformConfig: 'Copilot 平台配置',
copilotAccountList: 'Copilot 账户列表',
copilotGroup: 'Copilot 平台',
```

在 `admin.copilot` 节点（约第 3662 行）添加：

```ts
platformConfig: {
  title: 'Copilot 平台配置',
  description: '为各 plan 类型设置参数默认值，账号级配置优先',
  saveSuccess: '保存成功',
  saving: '保存中...',
  planLabels: {
    individual_free: 'Free（个人免费）',
    individual_pro: 'Pro（个人付费）',
    individual_pro_plus: 'Pro+（个人增强）',
    business: 'Business（商业版）',
    enterprise: 'Enterprise（企业版）',
  },
  fields: {
    maxOutputTokens: 'Max Output Tokens',
    maxOutputTokensHint: '留空表示不设默认，使用系统默认（8192）',
    maxBodyKB: 'Max Body Size (KB)',
    maxBodyKBHint: '留空表示不设默认',
    modelMapping: '模型映射',
    modelWhitelist: '模型白名单',
    modelWhitelistHint: '只有白名单内的模型才会被路由到该 plan 类型的账号，留空允许所有模型',
  },
},
accountList: {
  title: 'Copilot 账户列表',
  description: '查看和管理所有 Copilot 平台账号',
},
```

- [ ] **Step 2: 在 en.ts 的对应位置添加英文词条**

找到 en.ts 的 `nav` 节点，添加：

```ts
copilotPlatformConfig: 'Copilot Platform Config',
copilotAccountList: 'Copilot Accounts',
copilotGroup: 'Copilot Platform',
```

在 `admin.copilot` 节点添加（内容同 zh.ts，英文版）：

```ts
platformConfig: {
  title: 'Copilot Platform Config',
  description: 'Set default parameters per plan type. Account-level config takes priority.',
  saveSuccess: 'Saved',
  saving: 'Saving...',
  planLabels: {
    individual_free: 'Free (Individual)',
    individual_pro: 'Pro (Individual)',
    individual_pro_plus: 'Pro+ (Individual)',
    business: 'Business',
    enterprise: 'Enterprise',
  },
  fields: {
    maxOutputTokens: 'Max Output Tokens',
    maxOutputTokensHint: 'Leave empty to use system default (8192)',
    maxBodyKB: 'Max Body Size (KB)',
    maxBodyKBHint: 'Leave empty to use system default',
    modelMapping: 'Model Mapping',
    modelWhitelist: 'Model Whitelist',
    modelWhitelistHint: 'Only whitelisted models are routed to accounts of this plan type. Leave empty to allow all.',
  },
},
accountList: {
  title: 'Copilot Account List',
  description: 'View and manage all Copilot platform accounts',
},
```

- [ ] **Step 3: TypeScript 编译检查（可选，如有 type-check 脚本）**

```bash
cd frontend && npm run type-check 2>/dev/null || echo "no type-check script"
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "Feature: 新增 Copilot 平台配置 i18n 词条"
```

---

### Task 14: 前端 API 层

**Files:**
- Create: `frontend/src/api/admin/copilotPlatformConfig.ts`

- [ ] **Step 1: 创建 API 文件**

```ts
/**
 * Admin Copilot Platform Config API
 * GET  /api/v1/admin/copilot/platform-config         — 获取所有 plan_type 的配置
 * PUT  /api/v1/admin/copilot/platform-config/:plan_type — 更新指定 plan_type 的配置
 */

import { apiClient } from '../client'

export type CopilotPlanType =
  | 'individual_free'
  | 'individual_pro'
  | 'individual_pro_plus'
  | 'business'
  | 'enterprise'

export interface CopilotPlatformConfigEntry {
  plan_type: CopilotPlanType
  max_output_tokens: number | null
  max_body_kb: number | null
  model_mapping: Record<string, string>
  model_whitelist: string[]
}

export interface UpdateCopilotPlatformConfigRequest {
  max_output_tokens: number | null
  max_body_kb: number | null
  model_mapping: Record<string, string>
  model_whitelist: string[]
}

export const COPILOT_PLAN_TYPES: CopilotPlanType[] = [
  'individual_free',
  'individual_pro',
  'individual_pro_plus',
  'business',
  'enterprise',
]

/**
 * 获取全部 5 个 plan_type 的平台配置。
 */
export async function listCopilotPlatformConfigs(): Promise<CopilotPlatformConfigEntry[]> {
  const res = await apiClient.get<{ data: CopilotPlatformConfigEntry[] }>(
    '/admin/copilot/platform-config'
  )
  return res.data.data
}

/**
 * 更新指定 plan_type 的平台配置。
 * @param planType 目标 plan 类型
 * @param payload  完整配置（所有字段均写入）
 */
export async function updateCopilotPlatformConfig(
  planType: CopilotPlanType,
  payload: UpdateCopilotPlatformConfigRequest
): Promise<CopilotPlatformConfigEntry> {
  const res = await apiClient.put<{ data: CopilotPlatformConfigEntry }>(
    `/admin/copilot/platform-config/${planType}`,
    payload
  )
  return res.data.data
}
```

- [ ] **Step 2: 编译检查**

```bash
cd frontend && npm run type-check 2>/dev/null || echo "ok"
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/api/admin/copilotPlatformConfig.ts
git commit -m "Feature: 新增 CopilotPlatformConfig 前端 API 层"
```

---

### Task 15: 路由重组

**Files:**
- Modify: `frontend/src/router/index.ts`

当前状态（约第 304 行）：
- `/admin/copilot/accounts` → `CopilotAccountsView.vue`（成本分析页）
- `/admin/copilot/users` → `CopilotUsersView.vue`

目标（spec Section 1）：
- `/admin/copilot/platform` → `CopilotPlatformConfigView.vue`（新增）
- `/admin/copilot/accounts` → `CopilotAccountListView.vue`（新增，Copilot 账户列表）
- `/admin/copilot/cost` → `CopilotAccountsView.vue`（原成本分析，路由名称变更）
- `/admin/copilot/users` → 不变

- [ ] **Step 1: 修改路由定义**

找到现有的 `/admin/copilot/accounts` 路由（约第 304 行）：

**将该路由改为：**
```ts
{
  path: '/admin/copilot/cost',
  name: 'AdminCopilotCost',
  component: () => import('@/views/admin/copilot/CopilotAccountsView.vue'),
  meta: {
    requiresAuth: true,
    requiresAdmin: true,
    title: 'Copilot Account Costs',
    titleKey: 'admin.copilot.accounts.title',
    descriptionKey: 'admin.copilot.accounts.description'
  }
},
```

**在该路由之后添加两条新路由：**
```ts
{
  path: '/admin/copilot/accounts',
  name: 'AdminCopilotAccountList',
  component: () => import('@/views/admin/copilot/CopilotAccountListView.vue'),
  meta: {
    requiresAuth: true,
    requiresAdmin: true,
    title: 'Copilot Account List',
    titleKey: 'admin.copilot.accountList.title',
    descriptionKey: 'admin.copilot.accountList.description'
  }
},
{
  path: '/admin/copilot/platform',
  name: 'AdminCopilotPlatformConfig',
  component: () => import('@/views/admin/copilot/CopilotPlatformConfigView.vue'),
  meta: {
    requiresAuth: true,
    requiresAdmin: true,
    title: 'Copilot Platform Config',
    titleKey: 'admin.copilot.platformConfig.title',
    descriptionKey: 'admin.copilot.platformConfig.description'
  }
},
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/router/index.ts
git commit -m "Feature: Copilot 路由重组（新增 platform/accounts，成本分析改为 cost）"
```

---

### Task 16: 侧边栏更新

**Files:**
- Modify: `frontend/src/components/layout/AppSidebar.vue`

- [ ] **Step 1: 修改 adminNavItems 中的 Copilot 部分**

找到 `adminNavItems` 计算属性中的两行 Copilot 菜单项（约第 640 行）：

**旧代码：**
```ts
{ path: '/admin/copilot/users', label: t('nav.copilotUsers'), icon: UsersIcon },
{ path: '/admin/copilot/accounts', label: t('nav.copilotAccounts'), icon: CreditCardIcon },
```

**新代码（4 项，按 spec 顺序）：**
```ts
{ path: '/admin/copilot/platform', label: t('nav.copilotPlatformConfig'), icon: CogIcon },
{ path: '/admin/copilot/accounts', label: t('nav.copilotAccountList'), icon: GlobeIcon },
{ path: '/admin/copilot/cost', label: t('nav.copilotAccounts'), icon: CreditCardIcon },
{ path: '/admin/copilot/users', label: t('nav.copilotUsers'), icon: UsersIcon },
```

注意：`CogIcon` 已在文件顶部 import（用于 Settings 菜单项），`GlobeIcon` 同理已存在。

- [ ] **Step 2: 检查 CogIcon / GlobeIcon 是否已 import**

```bash
grep -n "CogIcon\|GlobeIcon" frontend/src/components/layout/AppSidebar.vue | head -5
```

若未 import，在 script setup 的 import 块中补充：
```ts
import { CogIcon } from '@heroicons/vue/24/outline'
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/layout/AppSidebar.vue
git commit -m "Feature: 侧边栏 Copilot 分组重组，新增平台配置和账户列表入口"
```
