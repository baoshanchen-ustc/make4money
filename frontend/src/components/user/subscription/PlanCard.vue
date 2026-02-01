<template>
  <div
    class="card group relative overflow-hidden transition-all hover:shadow-lg dark:hover:shadow-dark-900/50"
    :class="{ 'ring-2 ring-primary-500': isPopular }"
  >
    <!-- 热门标签 -->
    <div
      v-if="isPopular"
      class="absolute right-4 top-4 rounded-full bg-primary-500 px-3 py-1 text-xs font-medium text-white"
    >
      {{ t('subscriptionPlan.popular') }}
    </div>

    <div class="p-6">
      <!-- 套餐名称 -->
      <h3 class="mb-2 text-xl font-bold text-gray-900 dark:text-white">
        {{ plan.name }}
      </h3>

      <!-- 套餐描述 -->
      <p
        v-if="plan.purchasable_description || plan.description"
        class="mb-4 text-sm text-gray-500 dark:text-gray-400"
      >
        {{ plan.purchasable_description || plan.description }}
      </p>

      <!-- 价格 -->
      <div class="mb-6">
        <span class="text-4xl font-bold text-gray-900 dark:text-white">¥{{ plan.price_cny }}</span>
        <span class="text-gray-500 dark:text-gray-400">/月</span>
      </div>

      <!-- 额度信息 -->
      <div class="mb-6 space-y-2">
        <div v-if="plan.daily_limit_usd" class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
          <svg class="h-4 w-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
          </svg>
          <span>{{ t('subscriptionPlan.dailyQuota', { amount: plan.daily_limit_usd }) }}</span>
        </div>
        <div v-if="plan.weekly_limit_usd" class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
          <svg class="h-4 w-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
          </svg>
          <span>{{ t('subscriptionPlan.weeklyQuota', { amount: plan.weekly_limit_usd }) }}</span>
        </div>
        <div v-if="plan.monthly_limit_usd" class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
          <svg class="h-4 w-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
          </svg>
          <span>{{ t('subscriptionPlan.monthlyQuota', { amount: plan.monthly_limit_usd }) }}</span>
        </div>
        <div v-if="!plan.daily_limit_usd && !plan.weekly_limit_usd && !plan.monthly_limit_usd" class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
          <svg class="h-4 w-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
          </svg>
          <span>{{ t('subscriptionPlan.unlimitedQuota') }}</span>
        </div>
      </div>

      <!-- 购买按钮 -->
      <button
        type="button"
        class="btn w-full"
        :class="isPopular ? 'btn-primary' : 'btn-outline'"
        :disabled="loading"
        @click="handlePurchase"
      >
        <span v-if="loading" class="flex items-center justify-center gap-2">
          <div class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent"></div>
          {{ t('common.loading') }}
        </span>
        <span v-else>{{ t('subscriptionPlan.purchase') }}</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlan } from '@/api/subscriptionPlan'

const props = defineProps<{
  plan: SubscriptionPlan
  isPopular?: boolean
}>()

const emit = defineEmits<{
  purchase: [planId: number]
}>()

const { t } = useI18n()
const loading = ref(false)

const handlePurchase = () => {
  emit('purchase', props.plan.id)
}

defineExpose({
  setLoading: (value: boolean) => {
    loading.value = value
  }
})
</script>
