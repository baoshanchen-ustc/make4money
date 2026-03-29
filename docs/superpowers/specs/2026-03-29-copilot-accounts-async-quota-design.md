# Copilot Accounts Async Quota Loading Design

**Goal:** Eliminate first-page-load lag on the accounts overview by splitting base account data (instant DB query) from quota data (potentially slow GitHub API call), loading both concurrently so the table renders immediately while quota columns fill in asynchronously.

**Architecture:** Two new backend endpoints replace the single blocking `GetAccountsOverview` call. The frontend fires both requests in parallel; base data renders the table within ~50ms while quota data patches in when ready (cache-warm: fast; cache-cold: GitHub API latency but non-blocking).

**Tech Stack:** Go (Gin, standard `database/sql`), Vue 3 Composition API, TypeScript

---

## Root Cause

`GetAccountsOverview` calls `s.quotaCache.FetchAllCached(ctx)` as its first step. `FetchAllCached` → `FetchAll` iterates all active Copilot accounts and, for any account with a cold cache entry, makes a synchronous GitHub API call before returning. The entire HTTP response is blocked until every account's quota is resolved.

On first page load (cold cache), this means N sequential or concurrent GitHub API calls must complete before the frontend receives any data.

---

## Backend Design

### New Service Methods (`copilot_analytics_service.go`)

**`GetAccountsOverviewBase(ctx) (*CopilotAccountsBaseResult, error)`**

Performs only DB queries — no quota cache access, no GitHub API calls.

Returns:
```go
type CopilotAccountsBaseResult struct {
    TotalAccounts        int                       `json:"total_accounts"`
    EstimatedMonthlyCost float64                   `json:"estimated_monthly_cost"`
    TodayPremiumRequests int                       `json:"today_premium_requests"`
    Accounts             []CopilotAccountBaseEntry `json:"accounts"`
}

type CopilotAccountBaseEntry struct {
    AccountID                  int64                          `json:"account_id"`
    Name                       string                         `json:"name"`
    PlanType                   string                         `json:"plan_type"`
    SeatCount                  int                            `json:"seat_count"`
    MonthlyCost                float64                        `json:"monthly_cost"`
    CostPerPremiumRequest      float64                        `json:"cost_per_premium_request"`
    SystemTodayPremiumRequests int                            `json:"system_today_premium_requests"`
    SystemMonthPremiumRequests int                            `json:"system_month_premium_requests"`
    BudgetAlert                *CopilotAccountBudgetAlertInfo `json:"budget_alert"`
}
```

Implementation: reuses `listAllActiveCopilotAccounts`, `fetchAccountUsageCounts`, `alertRepo.ListAll`, and the existing `extractCopilotPlan` / plan config logic. Does NOT call `quotaCache.FetchAllCached`.

**`GetAccountsOverviewQuotas(ctx) (*CopilotAccountsQuotasResult, error)`**

Reads quota from cache only (does not block on GitHub API for missing entries — returns nil quota for cold accounts). Computes alert status per account.

Returns:
```go
type CopilotAccountsQuotasResult struct {
    AlertCount int                        `json:"alert_count"`
    Quotas     []CopilotAccountQuotaEntry `json:"quotas"`
}

type CopilotAccountQuotaEntry struct {
    AccountID     int64                        `json:"account_id"`
    QuotaSnapshot *CopilotAccountQuotaSnapshot `json:"quota_snapshot"`
    AlertStatus   string                       `json:"alert_status"`
}
```

Implementation: calls `listAllActiveCopilotAccounts` (DB only), then for each account reads `quotaCache.GetCached(acc.ID)` (in-memory, never blocks). Accounts with no cache entry get `quota_snapshot: null`, `alert_status: "ok"`.

### New Handler Methods (`copilot_analytics_handler.go`)

```
GET /api/v1/admin/copilot/accounts/overview/base
GET /api/v1/admin/copilot/accounts/overview/quotas
```

Both are thin: call service method, return `response.Success(c, result)`.

### Existing Endpoint

`GET /api/v1/admin/copilot/accounts/overview` is **kept as-is** — no changes, no deprecation. It continues to work for any consumer that needs the full merged response.

### Router Registration

Two new routes registered alongside the existing overview route. Route ordering matters in Gin: `/overview/base` and `/overview/quotas` must be registered before any wildcard or parameterised routes that could shadow them.

---

## Frontend Design

### New API Functions (`copilotAnalytics.ts`)

```ts
export interface CopilotAccountBaseEntry {
  account_id: number
  name: string
  plan_type: string
  seat_count: number
  monthly_cost: number
  cost_per_premium_request: number
  system_today_premium_requests: number
  system_month_premium_requests: number
  budget_alert: CopilotAccountBudgetAlertInfo | null
}

export interface CopilotAccountsBaseResult {
  total_accounts: number
  estimated_monthly_cost: number
  today_premium_requests: number
  accounts: CopilotAccountBaseEntry[]
}

export interface CopilotAccountQuotaEntry {
  account_id: number
  quota_snapshot: CopilotAccountQuotaSnapshot | null
  alert_status: CopilotAlertStatus
}

export interface CopilotAccountsQuotasResult {
  alert_count: number
  quotas: CopilotAccountQuotaEntry[]
}

export async function getCopilotAccountsOverviewBase(): Promise<CopilotAccountsBaseResult>
export async function getCopilotAccountsOverviewQuotas(): Promise<CopilotAccountsQuotasResult>
```

### State Changes (`CopilotAccountsView.vue`)

Replace:
```ts
const loading = ref(false)
const overview = ref<CopilotAccountsOverviewResult | null>(null)
```

With:
```ts
const loadingBase = ref(false)
const loadingQuotas = ref(false)
const baseData = ref<CopilotAccountsBaseResult | null>(null)
const quotasData = ref<CopilotAccountsQuotasResult | null>(null)

// Derived: button disabled / spinner
const loading = computed(() => loadingBase.value || loadingQuotas.value)
```

### Merged View Computed

```ts
const accounts = computed<CopilotAccountOverviewEntry[]>(() => {
  const base = baseData.value?.accounts ?? []
  const quotaMap = new Map(
    (quotasData.value?.quotas ?? []).map(q => [q.account_id, q])
  )
  return base.map(acc => ({
    ...acc,
    quota_snapshot: quotaMap.get(acc.account_id)?.quota_snapshot ?? null,
    alert_status: (quotaMap.get(acc.account_id)?.alert_status ?? 'ok') as CopilotAlertStatus,
  }))
})
```

`sortedAccounts` is based on `accounts` (no change to sort logic).

KPI cards read from `baseData` and `quotasData` directly:
- `estimated_monthly_cost`, `today_premium_requests`, `total_accounts` → `baseData`
- `alert_count` → `quotasData` (show skeleton while `loadingQuotas`)

### Load Functions

```ts
async function loadBase() {
  loadingBase.value = true
  error.value = null
  try {
    baseData.value = await getCopilotAccountsOverviewBase()
  } catch (e) {
    error.value = extractErrorMessage(e)
  } finally {
    loadingBase.value = false
  }
}

async function loadQuotas() {
  loadingQuotas.value = true
  try {
    quotasData.value = await getCopilotAccountsOverviewQuotas()
  } catch {
    // Silent — table is already usable; quota columns show "暂无数据"
  } finally {
    loadingQuotas.value = false
  }
}

async function loadAll() {
  if (loadingBase.value || loadingQuotas.value) return
  await Promise.all([loadBase(), loadQuotas()])
}
```

`onMounted(loadAll)`, refresh button calls `loadAll()`, `onBudgetSaved` calls `loadAll()`.

### Quota Column Skeleton

In the table quota cell:
- `loadingQuotas && !quotaMap.has(account.account_id)` → show `animate-pulse` skeleton bar
- `!loadingQuotas && quota_snapshot === null` → show "暂无数据" (existing behaviour)

### Alert Count KPI Card

- `loadingQuotas` → show `animate-pulse` skeleton (same pattern as other KPI cards)
- `quotasData === null && !loadingQuotas` → show `—` (quota fetch failed, distinguish from "zero alerts")
- Otherwise → show `quotasData.alert_count`

---

## Error Handling

| Scenario | Behaviour |
|---|---|
| Base request fails | Red error banner shown; table not rendered; quota request result discarded |
| Quota request fails | Silent; quota columns show "暂无数据"; alert_count KPI shows `—` |
| Both fail | Red error banner from base failure |
| Concurrent loadAll called twice | Second call returns early (guard check) |
| `doRefresh()` (single-row quota refresh) | No change — patches `accounts` array directly as before |

---

## Files Changed

| File | Change |
|---|---|
| `backend/internal/service/copilot_analytics_service.go` | Add `GetAccountsOverviewBase` and `GetAccountsOverviewQuotas` methods |
| `backend/internal/handler/admin/copilot_analytics_handler.go` | Add `GetAccountsOverviewBase` and `GetAccountsOverviewQuotas` handlers |
| `backend/internal/router/` (router file) | Register two new routes |
| `frontend/src/api/admin/copilotAnalytics.ts` | Add new interfaces and API functions |
| `frontend/src/views/admin/copilot/CopilotAccountsView.vue` | Split loading state, add merged computed, update KPI cards and quota skeleton |

Existing `GET /accounts/overview` endpoint and `getCopilotAccountsOverview()` API function are **not modified**.
