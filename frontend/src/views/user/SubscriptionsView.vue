<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <template v-else>
        <!-- 开通套餐区域 -->
        <div v-if="purchasablePlans.length > 0" class="card p-6">
          <h2 class="mb-6 text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('subscriptionPlan.title') }}
          </h2>
          <div class="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
            <PlanCard
              v-for="(plan, index) in purchasablePlans"
              :key="plan.id"
              :ref="el => setPlanCardRef(plan.id, el)"
              :plan="plan"
              :is-popular="index === 0"
              :subscribed="subscribedGroupIds.has(plan.id)"
              :upgrade-info="getUpgradeInfo(plan.id)"
              @purchase="handlePurchasePlan"
            />
          </div>
        </div>

        <!-- 无可售套餐时的提示 -->
        <div v-else-if="plansLoaded" class="card p-6 text-center">
          <div
            class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
          >
            <Icon name="creditCard" size="lg" class="text-gray-400" />
          </div>
          <h3 class="mb-2 text-base font-medium text-gray-900 dark:text-white">
            {{ t('subscriptionPlan.noPlans') }}
          </h3>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ t('subscriptionPlan.noPlansDesc') }}
          </p>
          <p v-if="contactInfo" class="mt-2 text-sm text-gray-500 dark:text-dark-400">
            {{ t('subscriptionPlan.contactUs') }}：{{ contactInfo }}
          </p>
        </div>

        <!-- Empty State - 无有效订阅 -->
        <div v-if="activeSubscriptions.length === 0" class="card p-12 text-center">
          <div
            class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
          >
            <Icon name="creditCard" size="xl" class="text-gray-400" />
          </div>
          <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('userSubscriptions.noActiveSubscriptions') }}
          </h3>
          <p class="text-gray-500 dark:text-dark-400">
            {{ t('userSubscriptions.noActiveSubscriptionsDesc') }}
          </p>
        </div>

        <!-- Subscriptions Grid - 只显示有效订阅 -->
        <div v-else>
          <h2 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('userSubscriptions.title') }}
          </h2>
          <div class="grid gap-6 lg:grid-cols-2">
            <div
              v-for="subscription in activeSubscriptions"
              :key="subscription.id"
              class="card overflow-hidden"
            >
              <!-- Header -->
              <div
                class="flex items-center justify-between border-b border-gray-100 p-4 dark:border-dark-700"
              >
                <div class="flex items-center gap-3">
                  <div
                    class="flex h-10 w-10 items-center justify-center rounded-xl bg-purple-100 dark:bg-purple-900/30"
                  >
                    <Icon name="creditCard" size="md" class="text-purple-600 dark:text-purple-400" />
                  </div>
                  <div>
                    <h3 class="font-semibold text-gray-900 dark:text-white">
                      {{ subscription.group?.name || `Group #${subscription.group_id}` }}
                    </h3>
                    <p class="text-xs text-gray-500 dark:text-dark-400">
                      {{ subscription.group?.description || '' }}
                    </p>
                  </div>
                </div>
                <span
                  :class="[
                    'badge',
                    subscription.status === 'active'
                      ? 'badge-success'
                      : subscription.status === 'expired'
                        ? 'badge-warning'
                        : 'badge-danger'
                  ]"
                >
                  {{ t(`userSubscriptions.status.${subscription.status}`) }}
                </span>
              </div>

              <!-- Usage Progress -->
              <div class="space-y-4 p-4">
                <!-- Expiration Info -->
                <div v-if="subscription.expires_at" class="flex items-center justify-between text-sm">
                  <span class="text-gray-500 dark:text-dark-400">{{
                    t('userSubscriptions.expires')
                  }}</span>
                  <span :class="getExpirationClass(subscription.expires_at)">
                    {{ formatExpirationDate(subscription.expires_at) }}
                  </span>
                </div>
                <div v-else class="flex items-center justify-between text-sm">
                  <span class="text-gray-500 dark:text-dark-400">{{
                    t('userSubscriptions.expires')
                  }}</span>
                  <span class="text-gray-700 dark:text-gray-300">{{
                    t('userSubscriptions.noExpiration')
                  }}</span>
                </div>

                <!-- Daily Usage -->
                <div v-if="subscription.group?.daily_limit_usd" class="space-y-2">
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('userSubscriptions.daily') }}
                    </span>
                    <span class="text-sm text-gray-500 dark:text-dark-400">
                      ${{ (subscription.daily_usage_usd || 0).toFixed(2) }} / ${{
                        subscription.group.daily_limit_usd.toFixed(2)
                      }}
                    </span>
                  </div>
                  <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                      :class="
                        getProgressBarClass(
                          subscription.daily_usage_usd,
                          subscription.group.daily_limit_usd
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          subscription.daily_usage_usd,
                          subscription.group.daily_limit_usd
                        )
                      }"
                    ></div>
                  </div>
                  <p
                    v-if="subscription.daily_window_start"
                    class="text-xs text-gray-500 dark:text-dark-400"
                  >
                    {{
                      t('userSubscriptions.resetIn', {
                        time: formatResetTime(subscription.daily_window_start, 24)
                      })
                    }}
                  </p>
                </div>

                <!-- Weekly Usage -->
                <div v-if="subscription.group?.weekly_limit_usd" class="space-y-2">
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('userSubscriptions.weekly') }}
                    </span>
                    <span class="text-sm text-gray-500 dark:text-dark-400">
                      ${{ (subscription.weekly_usage_usd || 0).toFixed(2) }} / ${{
                        subscription.group.weekly_limit_usd.toFixed(2)
                      }}
                    </span>
                  </div>
                  <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                      :class="
                        getProgressBarClass(
                          subscription.weekly_usage_usd,
                          subscription.group.weekly_limit_usd
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          subscription.weekly_usage_usd,
                          subscription.group.weekly_limit_usd
                        )
                      }"
                    ></div>
                  </div>
                  <p
                    v-if="subscription.weekly_window_start"
                    class="text-xs text-gray-500 dark:text-dark-400"
                  >
                    {{
                      t('userSubscriptions.resetIn', {
                        time: formatResetTime(subscription.weekly_window_start, 168)
                      })
                    }}
                  </p>
                </div>

                <!-- Monthly Usage -->
                <div v-if="subscription.group?.monthly_limit_usd" class="space-y-2">
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('userSubscriptions.monthly') }}
                    </span>
                    <span class="text-sm text-gray-500 dark:text-dark-400">
                      ${{ (subscription.monthly_usage_usd || 0).toFixed(2) }} / ${{
                        subscription.group.monthly_limit_usd.toFixed(2)
                      }}
                    </span>
                  </div>
                  <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                      :class="
                        getProgressBarClass(
                          subscription.monthly_usage_usd,
                          subscription.group.monthly_limit_usd
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          subscription.monthly_usage_usd,
                          subscription.group.monthly_limit_usd
                        )
                      }"
                    ></div>
                  </div>
                  <p
                    v-if="subscription.monthly_window_start"
                    class="text-xs text-gray-500 dark:text-dark-400"
                  >
                    {{
                      t('userSubscriptions.resetIn', {
                        time: formatResetTime(subscription.monthly_window_start, 720)
                      })
                    }}
                  </p>
                </div>

                <!-- No limits configured - Unlimited badge -->
                <div
                  v-if="
                    !subscription.group?.daily_limit_usd &&
                    !subscription.group?.weekly_limit_usd &&
                    !subscription.group?.monthly_limit_usd
                  "
                  class="flex items-center justify-center rounded-xl bg-gradient-to-r from-emerald-50 to-teal-50 py-6 dark:from-emerald-900/20 dark:to-teal-900/20"
                >
                  <div class="flex items-center gap-3">
                    <span class="text-4xl text-emerald-600 dark:text-emerald-400">∞</span>
                    <div>
                      <p class="text-sm font-medium text-emerald-700 dark:text-emerald-300">
                        {{ t('userSubscriptions.unlimited') }}
                      </p>
                      <p class="text-xs text-emerald-600/70 dark:text-emerald-400/70">
                        {{ t('userSubscriptions.unlimitedDesc') }}
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <!-- 历史订阅记录 -->
        <div v-if="historySubscriptions.length > 0" class="card p-6">
          <h2 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('userSubscriptions.history') }}
          </h2>
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b border-gray-200 dark:border-dark-600">
                  <th class="pb-3 pr-4 text-left font-medium text-gray-500 dark:text-dark-400">
                    {{ t('userSubscriptions.planName') }}
                  </th>
                  <th class="pb-3 pr-4 text-left font-medium text-gray-500 dark:text-dark-400">
                    {{ t('userSubscriptions.subscribeTime') }}
                  </th>
                  <th class="pb-3 pr-4 text-left font-medium text-gray-500 dark:text-dark-400">
                    {{ t('userSubscriptions.expireTime') }}
                  </th>
                  <th class="pb-3 text-left font-medium text-gray-500 dark:text-dark-400">
                    {{ t('userSubscriptions.statusLabel') }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="sub in historySubscriptions"
                  :key="sub.id"
                  class="border-b border-gray-100 last:border-0 dark:border-dark-700"
                >
                  <td class="py-3 pr-4 text-gray-900 dark:text-white">
                    {{ sub.group?.name || `Group #${sub.group_id}` }}
                  </td>
                  <td class="py-3 pr-4 text-gray-600 dark:text-gray-300">
                    {{ formatDateOnly(new Date(sub.created_at)) }}
                  </td>
                  <td class="py-3 pr-4 text-gray-600 dark:text-gray-300">
                    {{ sub.expires_at ? formatDateOnly(new Date(sub.expires_at)) : t('userSubscriptions.noExpiration') }}
                  </td>
                  <td class="py-3">
                    <span
                      :class="[
                        'badge',
                        sub.status === 'expired' ? 'badge-warning'
                          : sub.status === 'upgraded' ? 'badge-info'
                          : 'badge-danger'
                      ]"
                    >
                      {{ t(`userSubscriptions.status.${sub.status}`) }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>

      <!-- 支付弹框 -->
      <SubscriptionPaymentModal
        v-model:visible="paymentModalVisible"
        :order-no="currentOrderNo"
        @paid="handlePaymentSuccess"
        @expired="handlePaymentExpired"
        @close="handlePaymentModalClose"
      />

      <ConfirmDialog
        :show="showUpgradeKeyGuideDialog"
        :title="t('subscriptionPlan.upgradeKeyGuideTitle')"
        :message="t('subscriptionPlan.upgradeKeyGuideMessage')"
        :confirm-text="t('subscriptionPlan.goToKeys')"
        :cancel-text="t('subscriptionPlan.later')"
        @confirm="handleUpgradeGuideConfirm"
        @cancel="handleUpgradeGuideCancel"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import subscriptionsAPI from '@/api/subscriptions'
import { subscriptionPlanAPI, type SubscriptionPlan, type UpgradeOption } from '@/api/subscriptionPlan'
import { RateLimitExceededError } from '@/api/recharge'
import type { UpgradeInfo } from '@/components/user/subscription/PlanCard.vue'
import type { UserSubscription } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PlanCard from '@/components/user/subscription/PlanCard.vue'
import SubscriptionPaymentModal, { type PaidEventPayload } from '@/components/user/subscription/SubscriptionPaymentModal.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { formatDateOnly } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const router = useRouter()

const subscriptions = ref<UserSubscription[]>([])
const purchasablePlans = ref<SubscriptionPlan[]>([])
const loading = ref(true)
const plansLoaded = ref(false)

// 升级相关状态
const upgradeOptions = ref<Record<string, UpgradeOption>>({})
const upgradeSourceSubId = ref<number>(0)

// 有效订阅
const activeSubscriptions = computed(() =>
  subscriptions.value.filter(s => s.status === 'active')
)

// 历史订阅（过期/已撤销/已升级）
const historySubscriptions = computed(() =>
  subscriptions.value.filter(s => s.status !== 'active')
)

// 已订阅的 group_id 集合
const subscribedGroupIds = computed(() =>
  new Set(activeSubscriptions.value.map(s => s.group_id))
)

// 支付弹框状态
const paymentModalVisible = ref(false)
const currentOrderNo = ref('')
const showUpgradeKeyGuideDialog = ref(false)

// 套餐卡片引用
const planCardRefs = ref<Map<number, InstanceType<typeof PlanCard>>>(new Map())

const contactInfo = computed(() => appStore.contactInfo || '')

function setPlanCardRef(planId: number, el: unknown) {
  if (el) {
    planCardRefs.value.set(planId, el as InstanceType<typeof PlanCard>)
  } else {
    planCardRefs.value.delete(planId)
  }
}

function getUpgradeInfo(planId: number): UpgradeInfo | undefined {
  const option = upgradeOptions.value[String(planId)]
  if (!option) return undefined
  return {
    canUpgrade: true,
    sourceSubscriptionId: upgradeSourceSubId.value,
    upgradePrice: option.upgrade_price,
    originalPrice: option.original_price,
    remainingValue: option.remaining_value,
  }
}

async function loadData() {
  try {
    loading.value = true
    const [subs, plansResponse] = await Promise.all([
      subscriptionsAPI.getMySubscriptions(),
      subscriptionPlanAPI.listPlans().catch(() => ({ plans: [], payment_enabled: false }))
    ])
    subscriptions.value = subs
    purchasablePlans.value = plansResponse.plans
    plansLoaded.value = true

    // 加载升级选项
    if (activeSubscriptions.value.length > 0) {
      try {
        const upgradeData = await subscriptionPlanAPI.getUpgradeOptions()
        upgradeOptions.value = upgradeData.options || {}
        upgradeSourceSubId.value = upgradeData.source_subscription_id
      } catch (e) {
        console.error('Failed to load upgrade options:', e)
      }
    }
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

async function handlePurchasePlan(planId: number) {
  const cardRef = planCardRefs.value.get(planId)
  try {
    cardRef?.setLoading(true)

    const upgradeInfo = getUpgradeInfo(planId)
    let order

    if (upgradeInfo?.canUpgrade) {
      // 升级订单
      order = await subscriptionPlanAPI.createUpgradeOrder({
        source_subscription_id: upgradeInfo.sourceSubscriptionId,
        target_group_id: planId,
        payment_method: 'wechat_pay',
        payment_channel: 'native'
      })
    } else {
      // 普通购买
      order = await subscriptionPlanAPI.createOrder({
        group_id: planId,
        payment_method: 'wechat_pay',
        payment_channel: 'native'
      })
    }

    // 打开支付弹框
    currentOrderNo.value = order.order_no
    paymentModalVisible.value = true
  } catch (error: unknown) {
    console.error('Failed to create order:', error)
    let errorMessage: string
    if (error instanceof RateLimitExceededError) {
      errorMessage = error.message
    } else if (error instanceof Error) {
      errorMessage = error.message
    } else if (typeof error === 'object' && error !== null && 'message' in error) {
      errorMessage = String((error as Record<string, unknown>).message)
    } else {
      errorMessage = t('subscriptionPlan.createOrderFailed')
    }
    appStore.showError(errorMessage)
  } finally {
    cardRef?.setLoading(false)
  }
}

function handlePaymentSuccess(payload: PaidEventPayload) {
  paymentModalVisible.value = false
  currentOrderNo.value = ''
  if (payload.orderType === 'upgrade') {
    showUpgradeKeyGuideDialog.value = true
  }
  // 刷新订阅列表
  loadData()
}

function handlePaymentExpired() {
  paymentModalVisible.value = false
  currentOrderNo.value = ''
}

function handlePaymentModalClose() {
  currentOrderNo.value = ''
}

function handleUpgradeGuideConfirm() {
  showUpgradeKeyGuideDialog.value = false
  router.push('/keys')
}

function handleUpgradeGuideCancel() {
  showUpgradeKeyGuideDialog.value = false
}

function getProgressWidth(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return '0%'
  const percentage = Math.min(((used || 0) / limit) * 100, 100)
  return `${percentage}%`
}

function getProgressBarClass(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return 'bg-gray-400'
  const percentage = ((used || 0) / limit) * 100
  if (percentage >= 90) return 'bg-red-500'
  if (percentage >= 70) return 'bg-orange-500'
  return 'bg-green-500'
}

function formatExpirationDate(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (days < 0) {
    return t('userSubscriptions.status.expired')
  }

  const dateStr = formatDateOnly(expires)

  if (days === 0) {
    return `${dateStr} (Today)`
  }
  if (days === 1) {
    return `${dateStr} (Tomorrow)`
  }

  return t('userSubscriptions.daysRemaining', { days }) + ` (${dateStr})`
}

function getExpirationClass(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (days <= 0) return 'text-red-600 dark:text-red-400 font-medium'
  if (days <= 3) return 'text-red-600 dark:text-red-400'
  if (days <= 7) return 'text-orange-600 dark:text-orange-400'
  return 'text-gray-700 dark:text-gray-300'
}

function formatResetTime(windowStart: string | null, windowHours: number): string {
  if (!windowStart) return t('userSubscriptions.windowNotActive')

  const start = new Date(windowStart)
  const end = new Date(start.getTime() + windowHours * 60 * 60 * 1000)
  const now = new Date()
  const diff = end.getTime() - now.getTime()

  if (diff <= 0) return t('userSubscriptions.windowNotActive')

  const hours = Math.floor(diff / (1000 * 60 * 60))
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))

  if (hours > 24) {
    const days = Math.floor(hours / 24)
    const remainingHours = hours % 24
    return `${days}d ${remainingHours}h`
  }

  if (hours > 0) {
    return `${hours}h ${minutes}m`
  }

  return `${minutes}m`
}

onMounted(() => {
  loadData()
})
</script>
