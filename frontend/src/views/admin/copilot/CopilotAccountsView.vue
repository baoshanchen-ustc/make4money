<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Page Header -->
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('admin.copilot.accounts.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.copilot.accounts.description') }}
          </p>
        </div>
        <button
          class="inline-flex items-center gap-2 rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 disabled:opacity-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700"
          :disabled="loading"
          @click="loadOverview"
        >
          <svg class="h-4 w-4" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          刷新
        </button>
      </div>

      <!-- Error Banner -->
      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
        <div class="flex items-center gap-2">
          <svg class="h-4 w-4 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
          </svg>
          {{ error }}
        </div>
      </div>

      <!-- KPI Cards -->
      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <!-- Estimated Monthly Cost -->
        <div class="card flex items-center gap-4 p-5">
          <div class="flex-shrink-0 rounded-xl bg-emerald-50 p-3 dark:bg-emerald-900/30">
            <svg class="h-6 w-6 text-emerald-600 dark:text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>
          <div class="min-w-0 flex-1">
            <p class="text-xs font-medium text-gray-500 dark:text-gray-400">预估月费用</p>
            <div v-if="loading" class="mt-1 h-7 w-20 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
            <p v-else class="mt-0.5 text-2xl font-bold text-emerald-600 dark:text-emerald-400">
              ${{ formatCost(overview?.estimated_monthly_cost ?? 0) }}
            </p>
            <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">所有账户合计</p>
          </div>
        </div>

        <!-- Today Premium Requests -->
        <div class="card flex items-center gap-4 p-5">
          <div class="flex-shrink-0 rounded-xl bg-blue-50 p-3 dark:bg-blue-900/30">
            <svg class="h-6 w-6 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
          </div>
          <div class="min-w-0 flex-1">
            <p class="text-xs font-medium text-gray-500 dark:text-gray-400">今日 Premium 请求</p>
            <div v-if="loading" class="mt-1 h-7 w-20 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
            <p v-else class="mt-0.5 text-2xl font-bold text-blue-600 dark:text-blue-400">
              {{ (overview?.today_premium_requests ?? 0).toLocaleString() }}
            </p>
            <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">所有账户汇总</p>
          </div>
        </div>

        <!-- Total Accounts -->
        <div class="card flex items-center gap-4 p-5">
          <div class="flex-shrink-0 rounded-xl bg-violet-50 p-3 dark:bg-violet-900/30">
            <svg class="h-6 w-6 text-violet-600 dark:text-violet-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-2 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
            </svg>
          </div>
          <div class="min-w-0 flex-1">
            <p class="text-xs font-medium text-gray-500 dark:text-gray-400">Copilot 账户</p>
            <div v-if="loading" class="mt-1 h-7 w-20 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
            <p v-else class="mt-0.5 text-2xl font-bold text-violet-600 dark:text-violet-400">
              {{ overview?.total_accounts ?? 0 }}
            </p>
            <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">已接入账户数</p>
          </div>
        </div>

        <!-- Alert Count -->
        <div class="card flex items-center gap-4 p-5">
          <div
            class="flex-shrink-0 rounded-xl p-3"
            :class="(overview?.alert_count ?? 0) > 0 ? 'bg-red-50 dark:bg-red-900/30' : 'bg-gray-50 dark:bg-gray-700/50'"
          >
            <svg
              class="h-6 w-6"
              :class="(overview?.alert_count ?? 0) > 0 ? 'text-red-600 dark:text-red-400' : 'text-gray-400'"
              fill="none" viewBox="0 0 24 24" stroke="currentColor"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
            </svg>
          </div>
          <div class="min-w-0 flex-1">
            <p class="text-xs font-medium text-gray-500 dark:text-gray-400">告警账户</p>
            <div v-if="loading" class="mt-1 h-7 w-20 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
            <p
              v-else
              class="mt-0.5 text-2xl font-bold"
              :class="(overview?.alert_count ?? 0) > 0 ? 'text-red-600 dark:text-red-400' : 'text-gray-700 dark:text-gray-300'"
            >
              {{ overview?.alert_count ?? 0 }}
            </p>
            <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">需要关注</p>
          </div>
        </div>
      </div>

      <!-- Cost Comparison Chart -->
      <div v-if="!loading && overview && overview.accounts.length > 0" class="card p-5">
        <div class="mb-4 flex items-center justify-between">
          <div>
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">账户成本对比</h2>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">各账户月估算成本 · 基于套餐定价</p>
          </div>
        </div>
        <Bar :data="costChartData" :options="costChartOptions" class="max-h-56" />
      </div>

      <!-- Daily Requests Line Chart -->
      <div class="card p-5">
        <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">每日请求趋势</h2>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">各账户每天 Premium 请求量 · 每账户一条折线</p>
          </div>
          <!-- Day range selector -->
          <div class="flex items-center gap-1 rounded-lg border border-gray-200 p-1 dark:border-gray-700">
            <button
              v-for="opt in DAY_RANGE_OPTIONS"
              :key="opt.value"
              class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
              :class="dailyChartDays === opt.value
                ? 'bg-blue-600 text-white shadow-sm'
                : 'text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-300'"
              @click="dailyChartDays = opt.value"
            >
              {{ opt.label }}
            </button>
          </div>
        </div>
        <AccountsDailyChart :days="dailyChartDays" />
      </div>

      <!-- Accounts Table -->
      <div class="card overflow-hidden">
        <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-gray-700">
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">账户明细</h2>
          <span class="text-xs text-gray-400 dark:text-gray-500">
            共 {{ overview?.accounts.length ?? 0 }} 个账户
          </span>
        </div>

        <!-- Loading -->
        <div v-if="loading" class="flex h-40 items-center justify-center">
          <LoadingSpinner />
        </div>

        <!-- Empty -->
        <div
          v-else-if="!overview || overview.accounts.length === 0"
          class="flex h-40 flex-col items-center justify-center gap-2 text-sm text-gray-400 dark:text-gray-500"
        >
          <svg class="h-8 w-8 opacity-40" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5" />
          </svg>
          {{ t('admin.copilot.accounts.noData') }}
        </div>

        <!-- Table -->
        <div v-else class="overflow-x-auto">
          <table class="w-full">
            <thead>
              <tr class="border-b border-gray-100 bg-gray-50/80 dark:border-gray-700 dark:bg-gray-800/60">
                <!-- Expand toggle col -->
                <th class="w-8 px-4 py-3" />
                <!-- Sortable: 账户名 -->
                <th class="px-4 py-3 text-left">
                  <button class="sortable-th" @click="setSort('name')">
                    账户
                    <SortIcon :active="sortKey === 'name'" :dir="sortKey === 'name' ? sortDir : null" />
                  </button>
                </th>
                <!-- Sortable: 配额使用 -->
                <th class="px-4 py-3 text-left">
                  <button class="sortable-th" @click="setSort('quota')">
                    配额使用
                    <SortIcon :active="sortKey === 'quota'" :dir="sortKey === 'quota' ? sortDir : null" />
                  </button>
                </th>
                <!-- Sortable: 月费用 -->
                <th class="px-4 py-3 text-right">
                  <button class="sortable-th justify-end" @click="setSort('cost')">
                    月费用
                    <SortIcon :active="sortKey === 'cost'" :dir="sortKey === 'cost' ? sortDir : null" />
                  </button>
                </th>
                <!-- Sortable: 今日请求 -->
                <th class="px-4 py-3 text-right">
                  <button class="sortable-th justify-end" @click="setSort('today')">
                    今日请求
                    <SortIcon :active="sortKey === 'today'" :dir="sortKey === 'today' ? sortDir : null" />
                  </button>
                </th>
                <!-- Sortable: 本月请求 -->
                <th class="px-4 py-3 text-right">
                  <button class="sortable-th justify-end" @click="setSort('month')">
                    本月请求
                    <SortIcon :active="sortKey === 'month'" :dir="sortKey === 'month' ? sortDir : null" />
                  </button>
                </th>
                <!-- Static: 单次成本 -->
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  单次成本
                </th>
                <!-- Sortable: 状态 -->
                <th class="px-4 py-3 text-center">
                  <button class="sortable-th justify-center" @click="setSort('status')">
                    状态
                    <SortIcon :active="sortKey === 'status'" :dir="sortKey === 'status' ? sortDir : null" />
                  </button>
                </th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  操作
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50 dark:divide-gray-700/60">
              <template v-for="account in sortedAccounts" :key="account.account_id">
                <!-- Main Row -->
                <tr
                  class="group cursor-pointer transition-colors hover:bg-gray-50/80 dark:hover:bg-gray-700/30"
                  :class="{ 'bg-blue-50/40 dark:bg-blue-900/10': expandedRows.has(account.account_id) }"
                  @click="toggleRow(account.account_id)"
                >
                  <!-- Expand toggle -->
                  <td class="px-4 py-4">
                    <svg
                      class="h-4 w-4 text-gray-400 transition-transform duration-200"
                      :class="{ 'rotate-90': expandedRows.has(account.account_id) }"
                      fill="none" viewBox="0 0 24 24" stroke="currentColor"
                    >
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                    </svg>
                  </td>

                  <!-- Account Name & Plan -->
                  <td class="px-4 py-4">
                    <div class="flex items-center gap-3">
                      <div
                        class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg text-sm font-bold text-white"
                        :class="accountAvatarColor(account.account_id)"
                      >
                        {{ account.name.charAt(0).toUpperCase() }}
                      </div>
                      <div>
                        <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ account.name }}</p>
                        <p class="text-xs text-gray-400 dark:text-gray-500">
                          {{ planLabel(account.plan_type) }}
                          <template v-if="account.seat_count > 0">
                            · {{ account.seat_count }} {{ t('admin.copilot.accounts.seats') }}
                          </template>
                        </p>
                      </div>
                    </div>
                  </td>

                  <!-- Quota Usage Bar -->
                  <td class="min-w-[180px] px-4 py-4">
                    <div v-if="account.quota_snapshot">
                      <div class="mb-1.5 flex justify-between text-xs">
                        <span class="font-medium text-gray-700 dark:text-gray-300">
                          {{ account.quota_snapshot.github_total_used.toLocaleString() }}
                          <span class="text-gray-400"> / {{ account.quota_snapshot.unlimited ? '∞' : account.quota_snapshot.entitlement.toLocaleString() }}</span>
                        </span>
                        <span :class="quotaPercentColor(quotaPercent(account))">
                          {{ account.quota_snapshot.unlimited ? '∞' : `${quotaPercent(account)}%` }}
                        </span>
                      </div>
                      <div class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                        <div
                          class="h-full rounded-full transition-all duration-500"
                          :class="quotaBarColor(quotaPercent(account))"
                          :style="{ width: `${Math.min(100, quotaPercent(account))}%` }"
                        />
                      </div>
                      <p v-if="account.quota_snapshot.overage > 0" class="mt-1 text-xs font-medium text-red-500">
                        超额 {{ account.quota_snapshot.overage.toLocaleString() }} 次
                      </p>
                    </div>
                    <span v-else class="text-xs text-gray-400">—</span>
                  </td>

                  <!-- Monthly Cost — plan-aware display -->
                  <td class="px-4 py-4 text-right">
                    <span class="text-sm font-bold text-gray-900 dark:text-white">${{ formatCost(account.monthly_cost) }}</span>
                    <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
                      {{ planCostBreakdown(account.plan_type, account.seat_count) }}
                    </p>
                    <p v-if="account.budget_alert?.enabled && account.budget_alert.monthly_budget > 0" class="text-xs text-gray-400">
                      / ${{ account.budget_alert.monthly_budget }} 预算
                    </p>
                  </td>

                  <!-- Today Requests -->
                  <td class="px-4 py-4 text-right">
                    <span class="inline-flex items-center rounded-full bg-blue-50 px-2.5 py-1 text-xs font-semibold text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                      {{ account.system_today_premium_requests.toLocaleString() }}
                    </span>
                  </td>

                  <!-- Month Requests -->
                  <td class="px-4 py-4 text-right">
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ account.system_month_premium_requests.toLocaleString() }}
                    </span>
                  </td>

                  <!-- Cost Per Request -->
                  <td class="px-4 py-4 text-right">
                    <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
                      ${{ account.cost_per_premium_request.toFixed(4) }}
                    </span>
                  </td>

                  <!-- Status Badge -->
                  <td class="px-4 py-4 text-center" @click.stop>
                    <span
                      class="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-semibold"
                      :class="statusBadgeClass(account.alert_status)"
                    >
                      <span class="h-1.5 w-1.5 rounded-full" :class="statusDotClass(account.alert_status)" />
                      {{ statusLabel(account.alert_status) }}
                    </span>
                  </td>

                  <!-- Actions -->
                  <td class="px-4 py-4 text-right" @click.stop>
                    <div class="flex items-center justify-end gap-2">
                      <button
                        :disabled="refreshing.has(account.account_id)"
                        class="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-600 hover:border-gray-300 hover:bg-gray-50 disabled:opacity-40 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-700"
                        :title="t('admin.copilot.accounts.refreshQuota')"
                        @click="doRefresh(account)"
                      >
                        <svg v-if="refreshing.has(account.account_id)" class="h-3.5 w-3.5 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                        <span v-else>{{ t('admin.copilot.accounts.refreshQuota') }}</span>
                      </button>
                      <button
                        class="rounded-lg border border-violet-200 px-3 py-1.5 text-xs font-medium text-violet-700 hover:border-violet-300 hover:bg-violet-50 dark:border-violet-700 dark:text-violet-400 dark:hover:bg-violet-900/20"
                        @click="openBudget(account)"
                      >
                        {{ t('admin.copilot.accounts.setBudget') }}
                      </button>
                    </div>
                  </td>
                </tr>

                <!-- Expanded Detail Row -->
                <tr v-if="expandedRows.has(account.account_id)" :key="`detail-${account.account_id}`">
                  <td colspan="9" class="bg-gray-50/60 px-6 pb-6 pt-2 dark:bg-gray-800/40">
                    <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
                      <!-- Quota Details Panel -->
                      <div class="rounded-xl border border-gray-100 bg-white p-4 dark:border-gray-700 dark:bg-gray-800">
                        <h4 class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">配额快照</h4>
                        <div v-if="account.quota_snapshot" class="space-y-2.5">
                          <div class="flex justify-between text-xs">
                            <span class="text-gray-500 dark:text-gray-400">配额总量</span>
                            <span class="font-semibold text-gray-800 dark:text-gray-200">{{ account.quota_snapshot.unlimited ? '无限制' : account.quota_snapshot.entitlement.toLocaleString() }}</span>
                          </div>
                          <div class="flex justify-between text-xs">
                            <span class="text-gray-500 dark:text-gray-400">GitHub 已用</span>
                            <span class="font-semibold text-gray-800 dark:text-gray-200">{{ account.quota_snapshot.github_total_used.toLocaleString() }}</span>
                          </div>
                          <div class="flex justify-between text-xs">
                            <span class="text-gray-500 dark:text-gray-400">剩余</span>
                            <span
                              class="font-semibold"
                              :class="account.quota_snapshot.remaining < 100 ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400'"
                            >
                              {{ account.quota_snapshot.unlimited ? '∞' : account.quota_snapshot.remaining.toLocaleString() }}
                            </span>
                          </div>
                          <div v-if="account.quota_snapshot.overage > 0" class="flex justify-between text-xs">
                            <span class="text-gray-500 dark:text-gray-400">超额量</span>
                            <span class="font-semibold text-red-600 dark:text-red-400">{{ account.quota_snapshot.overage.toLocaleString() }}</span>
                          </div>
                          <div class="flex justify-between text-xs">
                            <span class="text-gray-500 dark:text-gray-400">外部已用</span>
                            <span class="font-medium text-gray-600 dark:text-gray-400">{{ account.quota_snapshot.external_used.toLocaleString() }}</span>
                          </div>
                          <div v-if="account.quota_snapshot.cached_at" class="border-t border-gray-100 pt-2 text-xs text-gray-400 dark:border-gray-700 dark:text-gray-500">
                            快照更新：{{ formatTime(account.quota_snapshot.cached_at) }}
                          </div>
                        </div>
                        <p v-else class="text-xs text-gray-400">暂无配额数据</p>

                        <!-- Budget Alert Info -->
                        <div v-if="account.budget_alert?.enabled" class="mt-3 rounded-lg bg-amber-50 p-3 dark:bg-amber-900/20">
                          <p class="mb-1 text-xs font-semibold text-amber-700 dark:text-amber-400">告警设置</p>
                          <p class="text-xs text-amber-600 dark:text-amber-500">
                            使用率超过 <strong>{{ account.budget_alert.alert_threshold }}%</strong> 触发告警
                          </p>
                          <p v-if="account.budget_alert.monthly_budget > 0" class="text-xs text-amber-600 dark:text-amber-500">
                            参考月预算：${{ account.budget_alert.monthly_budget }}
                          </p>
                        </div>
                      </div>

                      <!-- Quota Trend Chart -->
                      <div class="rounded-xl border border-gray-100 bg-white p-4 dark:border-gray-700 dark:bg-gray-800 lg:col-span-2">
                        <h4 class="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">配额趋势（近 30 天）</h4>
                        <QuotaTrendChart :account-id="account.account_id" :days="30" />
                      </div>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- Budget Alert Dialog -->
    <BudgetAlertDialog
      :visible="budgetDialog.visible"
      :account-id="budgetDialog.accountId"
      :initial="budgetDialog.initial"
      @close="budgetDialog.visible = false"
      @saved="onBudgetSaved"
      @error="onBudgetError"
    />

    <!-- Toast -->
    <Transition name="toast">
      <div
        v-if="toast"
        class="fixed bottom-6 right-6 z-50 flex items-center gap-2 rounded-xl px-4 py-3 text-sm font-medium shadow-lg"
        :class="toast.type === 'success' ? 'bg-emerald-600 text-white' : 'bg-red-600 text-white'"
      >
        <svg v-if="toast.type === 'success'" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
        </svg>
        <svg v-else class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
        {{ toast.msg }}
      </div>
    </Transition>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bar } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  BarElement,
  Tooltip,
  Legend,
} from 'chart.js'
import {
  getCopilotAccountsOverview,
  refreshCopilotAccountQuota,
} from '@/api/admin/copilotAnalytics'
import { extractErrorMessage } from '@/api/client'
import type {
  CopilotAccountsOverviewResult,
  CopilotAccountOverviewEntry,
  CopilotAccountBudgetAlertInfo,
  CopilotAlertStatus,
} from '@/api/admin/copilotAnalytics'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import QuotaTrendChart from '@/components/admin/copilot/QuotaTrendChart.vue'
import AccountsDailyChart from '@/components/admin/copilot/AccountsDailyChart.vue'
import BudgetAlertDialog from '@/components/admin/copilot/BudgetAlertDialog.vue'

ChartJS.register(CategoryScale, LinearScale, BarElement, Tooltip, Legend)

const { t } = useI18n()

// ── Constants ─────────────────────────────────────────────────────────────────

const DAY_RANGE_OPTIONS = [
  { label: '7天', value: 7 },
  { label: '14天', value: 14 },
  { label: '30天', value: 30 },
  { label: '60天', value: 60 },
  { label: '90天', value: 90 },
] as const

// plan_type → display label
const PLAN_LABELS: Record<string, string> = {
  individual_free:     'Free',
  individual:          'Pro',
  individual_pro:      'Pro',
  individual_pro_plus: 'Pro+',
  business:            'Business',
  enterprise:          'Enterprise',
}

// plan_type → USD per seat per month
const PLAN_SEAT_COST: Record<string, number> = {
  individual_free:     0,
  individual:          10,
  individual_pro:      10,
  individual_pro_plus: 39,
  business:            19,
  enterprise:          39,
}

type SortKey = 'name' | 'quota' | 'cost' | 'today' | 'month' | 'status'
type SortDir = 'asc' | 'desc'

// Status ordering for sort: critical > warning > ok
const STATUS_ORDER: Record<CopilotAlertStatus, number> = {
  critical: 0,
  warning:  1,
  ok:       2,
}

// ── State ─────────────────────────────────────────────────────────────────────

const loading = ref(false)
const error = ref<string | null>(null)
const overview = ref<CopilotAccountsOverviewResult | null>(null)
const refreshing = ref(new Set<number>())
const expandedRows = ref(new Set<number>())
const toast = ref<{ type: 'success' | 'error'; msg: string } | null>(null)
let toastTimer: ReturnType<typeof setTimeout> | null = null

const dailyChartDays = ref<7 | 14 | 30 | 60 | 90>(30)

// Sort state — default: today DESC, month DESC, name ASC
const sortKey = ref<SortKey>('today')
const sortDir = ref<SortDir>('desc')

const budgetDialog = reactive<{
  visible: boolean
  accountId: number
  initial: CopilotAccountBudgetAlertInfo | null
}>({
  visible: false,
  accountId: 0,
  initial: null,
})

// ── Sorting ───────────────────────────────────────────────────────────────────

function setSort(key: SortKey) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    // Default direction per column:
    //   name → asc (A-Z), status → asc (critical first, since STATUS_ORDER: critical=0)
    //   all numeric columns → desc (largest first)
    sortDir.value = key === 'name' || key === 'status' ? 'asc' : 'desc'
  }
}

const sortedAccounts = computed<CopilotAccountOverviewEntry[]>(() => {
  const accounts = overview.value?.accounts ?? []
  if (accounts.length === 0) return accounts

  return [...accounts].sort((a, b) => {
    let cmp = 0

    switch (sortKey.value) {
      case 'name':
        cmp = a.name.localeCompare(b.name, 'zh-CN')
        break
      case 'quota':
        cmp = quotaPercent(a) - quotaPercent(b)
        break
      case 'cost':
        cmp = a.monthly_cost - b.monthly_cost
        break
      case 'today':
        cmp = a.system_today_premium_requests - b.system_today_premium_requests
        break
      case 'month':
        cmp = a.system_month_premium_requests - b.system_month_premium_requests
        break
      case 'status':
        cmp = STATUS_ORDER[a.alert_status] - STATUS_ORDER[b.alert_status]
        break
    }

    if (cmp !== 0) return sortDir.value === 'asc' ? cmp : -cmp

    // Stable secondary sort: today DESC → month DESC → name ASC
    const todayCmp = b.system_today_premium_requests - a.system_today_premium_requests
    if (todayCmp !== 0) return todayCmp
    const monthCmp = b.system_month_premium_requests - a.system_month_premium_requests
    if (monthCmp !== 0) return monthCmp
    return a.name.localeCompare(b.name, 'zh-CN')
  })
})

// ── Chart ─────────────────────────────────────────────────────────────────────

const AVATAR_COLORS = [
  'bg-blue-500', 'bg-violet-500', 'bg-emerald-500', 'bg-amber-500',
  'bg-rose-500', 'bg-cyan-500', 'bg-pink-500', 'bg-indigo-500',
]

const CHART_COLORS = [
  '#10b981', '#6366f1', '#f59e0b', '#3b82f6',
  '#f43f5e', '#06b6d4', '#ec4899', '#8b5cf6',
]

function accountAvatarColor(id: number): string {
  return AVATAR_COLORS[id % AVATAR_COLORS.length]
}

const costChartData = computed(() => {
  const accounts = overview.value?.accounts ?? []
  return {
    labels: accounts.map(a => a.name),
    datasets: [
      {
        label: '月费用 ($)',
        data: accounts.map(a => a.monthly_cost),
        backgroundColor: accounts.map((_, i) => CHART_COLORS[i % CHART_COLORS.length] + 'CC'),
        borderColor: accounts.map((_, i) => CHART_COLORS[i % CHART_COLORS.length]),
        borderWidth: 1.5,
        borderRadius: 6,
        borderSkipped: false,
        yAxisID: 'y',
      },
    ],
  }
})

const costChartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        label: (ctx: any) => `${ctx.dataset.label}: $${Number(ctx.parsed.y).toFixed(2)}`,
      },
    },
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { font: { size: 11 } },
    },
    y: {
      beginAtZero: true,
      grid: { color: 'rgba(156,163,175,0.15)' },
      ticks: {
        font: { size: 10 },
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        callback: (value: any) => `$${Number(value).toFixed(0)}`,
      },
    },
  },
}

// ── Plan Helpers ──────────────────────────────────────────────────────────────

function planLabel(planType: string | null): string {
  if (!planType) return '—'
  return PLAN_LABELS[planType] ?? planType
}

function planCostBreakdown(planType: string | null, seatCount: number): string {
  if (!planType) return ''
  const perSeat = PLAN_SEAT_COST[planType]
  if (perSeat === undefined || perSeat === 0) return planLabel(planType)
  if (seatCount <= 1) return `$${perSeat}/seat · ${planLabel(planType)}`
  return `$${perSeat} × ${seatCount} seats · ${planLabel(planType)}`
}

// ── Other Helpers ─────────────────────────────────────────────────────────────

function formatCost(v: number): string {
  return v >= 1000 ? v.toFixed(0) : v.toFixed(2)
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString('zh-CN', {
    month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

function quotaPercent(account: CopilotAccountOverviewEntry): number {
  const snap = account.quota_snapshot
  if (!snap || snap.unlimited || snap.entitlement === 0) return 0
  return Math.min(100, Math.round((snap.github_total_used / snap.entitlement) * 100))
}

function quotaBarColor(pct: number): string {
  if (pct >= 95) return 'bg-red-500'
  if (pct >= 75) return 'bg-amber-500'
  return 'bg-emerald-500'
}

function quotaPercentColor(pct: number): string {
  if (pct >= 95) return 'font-semibold text-red-600 dark:text-red-400'
  if (pct >= 75) return 'font-semibold text-amber-600 dark:text-amber-400'
  return 'text-gray-500 dark:text-gray-400'
}

function statusBadgeClass(status: CopilotAlertStatus): string {
  switch (status) {
    case 'critical': return 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-400'
    case 'warning':  return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
    default:         return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
  }
}

function statusDotClass(status: CopilotAlertStatus): string {
  switch (status) {
    case 'critical': return 'bg-red-500'
    case 'warning':  return 'bg-amber-500'
    default:         return 'bg-emerald-500'
  }
}

function statusLabel(status: CopilotAlertStatus): string {
  switch (status) {
    case 'critical': return 'Critical'
    case 'warning':  return 'Warning'
    default:         return 'Normal'
  }
}

function showToast(type: 'success' | 'error', msg: string) {
  if (toastTimer !== null) clearTimeout(toastTimer)
  toast.value = { type, msg }
  toastTimer = setTimeout(() => { toast.value = null; toastTimer = null }, 3000)
}

// ── Row Expand ────────────────────────────────────────────────────────────────

function toggleRow(accountId: number) {
  if (expandedRows.value.has(accountId)) {
    expandedRows.value.delete(accountId)
  } else {
    expandedRows.value.add(accountId)
  }
}

// ── Data Loading ──────────────────────────────────────────────────────────────

async function loadOverview() {
  if (loading.value) return
  loading.value = true
  error.value = null
  try {
    overview.value = await getCopilotAccountsOverview()
  } catch (e: unknown) {
    error.value = extractErrorMessage(e)
  } finally {
    loading.value = false
  }
}

async function doRefresh(account: CopilotAccountOverviewEntry) {
  refreshing.value.add(account.account_id)
  try {
    await refreshCopilotAccountQuota(account.account_id)
    showToast('success', t('admin.copilot.accounts.refreshSuccess'))
    await loadOverview()
  } catch {
    showToast('error', t('admin.copilot.accounts.refreshFailed'))
  } finally {
    refreshing.value.delete(account.account_id)
  }
}

// ── Budget Dialog ─────────────────────────────────────────────────────────────

function openBudget(account: CopilotAccountOverviewEntry) {
  budgetDialog.accountId = account.account_id
  budgetDialog.initial = account.budget_alert
  budgetDialog.visible = true
}

async function onBudgetSaved() {
  budgetDialog.visible = false
  showToast('success', t('admin.copilot.accounts.budgetSaved'))
  await loadOverview()
}

function onBudgetError(msg: string) {
  showToast('error', `${t('admin.copilot.accounts.budgetFailed')}: ${msg}`)
}

onMounted(loadOverview)

onUnmounted(() => {
  if (toastTimer !== null) clearTimeout(toastTimer)
})
</script>

<!-- SortIcon inline component -->
<script lang="ts">
import { defineComponent, h } from 'vue'

export const SortIcon = defineComponent({
  name: 'SortIcon',
  props: {
    active: { type: Boolean, default: false },
    dir: { type: String as () => 'asc' | 'desc' | null, default: null },
  },
  setup(props) {
    return () => {
      const color = props.active ? 'currentColor' : 'rgba(156,163,175,0.5)'
      // Up chevron (asc) and down chevron (desc)
      if (props.active && props.dir === 'asc') {
        return h('svg', { class: 'ml-1 inline-block h-3 w-3', fill: 'none', viewBox: '0 0 24 24', stroke: color },
          h('path', { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2.5', d: 'M5 15l7-7 7 7' }),
        )
      }
      if (props.active && props.dir === 'desc') {
        return h('svg', { class: 'ml-1 inline-block h-3 w-3', fill: 'none', viewBox: '0 0 24 24', stroke: color },
          h('path', { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2.5', d: 'M19 9l-7 7-7-7' }),
        )
      }
      // Inactive: show both arrows stacked
      return h('svg', { class: 'ml-1 inline-block h-3 w-3 opacity-40', fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor' },
        [
          h('path', { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2', d: 'M8 9l4-4 4 4' }),
          h('path', { 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '2', d: 'M16 15l-4 4-4-4' }),
        ],
      )
    }
  },
})
</script>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.25s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(8px) scale(0.96);
}

.card {
  @apply rounded-xl border border-gray-100 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800;
}

.sortable-th {
  @apply flex w-full items-center text-xs font-semibold uppercase tracking-wider text-gray-500 transition-colors hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200;
}
</style>
