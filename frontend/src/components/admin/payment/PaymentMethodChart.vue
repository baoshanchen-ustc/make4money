<template>
  <div class="card p-4">
    <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
      {{ t('payment.admin.paymentDistribution') }}
    </h3>
    <div
      v-if="!normalizedMethods.length"
      class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
    >
      {{ t('payment.admin.noData') }}
    </div>
    <div v-else class="space-y-3">
      <div v-for="method in normalizedMethods" :key="method.type" class="space-y-1">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <span :class="['inline-block h-3 w-3 rounded-full', colorMap[method.type] || 'bg-gray-400']"></span>
            <div>
              <p class="text-sm text-gray-700 dark:text-gray-300">
                {{ paymentMethodLabel(method.type) }}
              </p>
              <p v-if="paymentMethodSecondary(method.rawTypes)" class="text-xs text-gray-500 dark:text-gray-400">
                {{ paymentMethodSecondary(method.rawTypes) }}
              </p>
            </div>
          </div>
          <div class="text-right">
            <span class="text-sm font-medium text-gray-900 dark:text-white">
              ¥{{ method.amount.toFixed(2) }}
            </span>
            <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">
              ({{ method.count }})
            </span>
          </div>
        </div>
        <div class="h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
          <div
            :class="['h-full rounded-full transition-all', barColorMap[method.type] || 'bg-gray-400']"
            :style="{ width: barWidth(method.amount) + '%' }"
          ></div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{
  methods: { type: string; amount: number; count: number }[]
}>()

interface AggregatedMethodStat {
  type: string
  amount: number
  count: number
  rawTypes: string[]
}

function normalizePaymentType(type: string): string {
  const lower = type.toLowerCase()
  if (lower === 'stripe' || lower.includes('stripe') || lower === 'card' || lower === 'link') return 'stripe'
  if (lower.includes('wxpay') || lower.includes('wechat')) return 'wxpay'
  if (lower.includes('alipay') || lower === 'easypay') return 'alipay'
  return type
}

function paymentMethodLabel(type: string): string {
  const normalized = normalizePaymentType(type)
  return t(`payment.methods.${normalized}`, normalized)
}

function paymentMethodSecondary(rawTypes: string[]): string {
  const uniqueRawTypes = [...new Set(rawTypes)]
  if (uniqueRawTypes.length === 1 && normalizePaymentType(uniqueRawTypes[0]) === uniqueRawTypes[0]) {
    return ''
  }
  return uniqueRawTypes.join(' / ')
}

const colorMap: Record<string, string> = {
  alipay: 'bg-blue-500',
  wxpay: 'bg-green-500',
  stripe: 'bg-purple-500',
}

const barColorMap: Record<string, string> = {
  alipay: 'bg-blue-500',
  wxpay: 'bg-green-500',
  stripe: 'bg-purple-500',
}

const normalizedMethods = computed<AggregatedMethodStat[]>(() => {
  const aggregated = new Map<string, AggregatedMethodStat>()

  for (const method of props.methods || []) {
    const normalizedType = normalizePaymentType(method.type)
    const current = aggregated.get(normalizedType)
    if (current) {
      current.amount += method.amount
      current.count += method.count
      if (!current.rawTypes.includes(method.type)) current.rawTypes.push(method.type)
      continue
    }

    aggregated.set(normalizedType, {
      type: normalizedType,
      amount: method.amount,
      count: method.count,
      rawTypes: [method.type],
    })
  }

  return ['alipay', 'wxpay', 'stripe']
    .map((type) => aggregated.get(type))
    .filter((method): method is AggregatedMethodStat => !!method)
})

const maxAmount = computed(() => {
  if (!normalizedMethods.value.length) return 1
  return Math.max(...normalizedMethods.value.map(m => m.amount), 1)
})

function barWidth(amount: number): number {
  return Math.min((amount / maxAmount.value) * 100, 100)
}
</script>
