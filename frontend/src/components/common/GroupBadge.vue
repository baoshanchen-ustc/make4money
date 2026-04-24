<template>
  <span
    :class="[
      'inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium transition-colors',
      badgeClass
    ]"
  >
    <!-- Platform logo: 优先使用 displayIcon -->
    <PlatformIcon v-if="platform || displayIcon" :platform="platform" :icon-key="displayIcon" size="sm" />
    <!-- Group name: 优先使用 displayName -->
    <span class="truncate">{{ displayName || name }}</span>
    <!-- Right side label -->
    <span v-if="showLabel" :class="labelClass">
      <template v-if="hasCustomRate">
        <!-- 原倍率删除线 + 专属倍率高亮 -->
        <span class="line-through opacity-50 mr-0.5">{{ rateMultiplier }}x</span>
        <span class="font-bold">{{ userRateMultiplier }}x</span>
      </template>
      <template v-else>
        {{ labelText }}
      </template>
    </span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionType, GroupPlatform } from '@/types'
import { resolveIconTheme, type IconColorTheme } from '@/constants/displayIcons'
import PlatformIcon from './PlatformIcon.vue'

interface Props {
  name: string
  platform?: GroupPlatform
  subscriptionType?: SubscriptionType
  rateMultiplier?: number
  userRateMultiplier?: number | null // 用户专属倍率
  showRate?: boolean
  daysRemaining?: number | null // 剩余天数（订阅类型时使用）
  /**
   * 订阅分组默认在右侧 label 展示"订阅"或剩余天数；
   * 开启后订阅分组也改为显示倍率（保留订阅主题色 label，配合可用渠道这类
   * 只关心费率、不关心有效期的场景）。
   */
  alwaysShowRate?: boolean
  /** 自定义图标 key（优先于 platform） */
  displayIcon?: string | null
  /** 自定义展示名称（优先于 name） */
  displayName?: string | null
}

const props = withDefaults(defineProps<Props>(), {
  subscriptionType: 'standard',
  showRate: true,
  daysRemaining: null,
  userRateMultiplier: null,
  alwaysShowRate: false,
  displayIcon: null,
  displayName: null
})

const { t } = useI18n()

const isSubscription = computed(() => props.subscriptionType === 'subscription')

// 主题色：按 displayIcon 优先，回退到 platform。屏蔽真实平台色泄露。
const theme = computed<IconColorTheme>(() => resolveIconTheme(props.displayIcon, props.platform))

// 是否有专属倍率（且与默认倍率不同）
const hasCustomRate = computed(() => {
  return (
    props.userRateMultiplier !== null &&
    props.userRateMultiplier !== undefined &&
    props.rateMultiplier !== undefined &&
    props.userRateMultiplier !== props.rateMultiplier
  )
})

// 是否显示右侧标签
const showLabel = computed(() => {
  if (!props.showRate) return false
  // 订阅类型：显示天数或"订阅"
  if (isSubscription.value) return true
  // 标准类型：显示倍率（包括专属倍率）
  return props.rateMultiplier !== undefined || hasCustomRate.value
})

// Label text
const labelText = computed(() => {
  const rateLabel = props.rateMultiplier !== undefined ? `${props.rateMultiplier}x` : ''
  if (isSubscription.value && !props.alwaysShowRate) {
    // 如果有剩余天数，显示天数
    if (props.daysRemaining !== null && props.daysRemaining !== undefined) {
      if (props.daysRemaining <= 0) {
        return t('admin.users.expired')
      }
      return t('admin.users.daysRemaining', { days: props.daysRemaining })
    }
    // 否则显示"订阅"
    return t('groups.subscription')
  }
  return rateLabel
})

// Label style based on type and days remaining
const labelClass = computed(() => {
  const base = 'px-1.5 py-0.5 rounded text-[10px] font-semibold'

  if (!isSubscription.value) {
    // Standard: subtle background (不再为专属倍率使用不同的背景色)
    return `${base} bg-black/10 dark:bg-white/10`
  }

  // 订阅类型：根据剩余天数显示不同颜色
  if (props.daysRemaining !== null && props.daysRemaining !== undefined) {
    if (props.daysRemaining <= 0 || props.daysRemaining <= 3) {
      // 已过期或紧急（<=3天）：红色
      return `${base} bg-red-200/80 text-red-800 dark:bg-red-800/50 dark:text-red-300`
    }
    if (props.daysRemaining <= 7) {
      // 警告（<=7天）：橙色
      return `${base} bg-amber-200/80 text-amber-800 dark:bg-amber-800/50 dark:text-amber-300`
    }
  }

  // 正常状态或无天数：按 theme（display_icon 优先）
  switch (theme.value) {
    case 'orange':
      return `${base} bg-orange-200/60 text-orange-800 dark:bg-orange-800/40 dark:text-orange-300`
    case 'green':
      return `${base} bg-emerald-200/60 text-emerald-800 dark:bg-emerald-800/40 dark:text-emerald-300`
    case 'blue':
      return `${base} bg-blue-200/60 text-blue-800 dark:bg-blue-800/40 dark:text-blue-300`
    case 'violet':
      return `${base} bg-violet-200/60 text-violet-800 dark:bg-violet-800/40 dark:text-violet-300`
    default:
      return `${base} bg-gray-200/60 text-gray-800 dark:bg-gray-700/50 dark:text-gray-300`
  }
})

// Badge color based on theme (display_icon 优先) + subscription type
const badgeClass = computed(() => {
  const sub = isSubscription.value
  switch (theme.value) {
    case 'orange':
      return sub
        ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
        : 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400'
    case 'green':
      return sub
        ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
        : 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
    case 'blue':
      return sub
        ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
        : 'bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-400'
    case 'violet':
      return sub
        ? 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400'
        : 'bg-purple-50 text-purple-700 dark:bg-purple-900/20 dark:text-purple-400'
    default:
      return sub
        ? 'bg-gray-100 text-gray-700 dark:bg-gray-800/40 dark:text-gray-300'
        : 'bg-gray-50 text-gray-700 dark:bg-gray-800/30 dark:text-gray-300'
  }
})
</script>
