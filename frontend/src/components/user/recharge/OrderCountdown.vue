<template>
  <div class="order-countdown flex items-center gap-2">
    <svg class="h-5 w-5 text-orange-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
    <span class="text-sm font-medium" :class="remainingTimeClass">
      {{ t('recharge.countdown', { time: formattedTime }) }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'

interface Props {
  expireAt: string // ISO 格式的过期时间
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'expired'): void
}>()

const { t } = useI18n()

// 剩余秒数
const remainingSeconds = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

// 计算剩余时间
const calculateRemainingTime = () => {
  const expireTime = new Date(props.expireAt).getTime()
  const now = Date.now()
  const diff = Math.max(0, Math.floor((expireTime - now) / 1000))
  remainingSeconds.value = diff

  // 如果倒计时归零，触发过期事件
  if (diff <= 0) {
    stopTimer()
    emit('expired')
  }
}

// 格式化时间显示 (XX分XX秒)
const formattedTime = computed(() => {
  const minutes = Math.floor(remainingSeconds.value / 60)
  const seconds = remainingSeconds.value % 60
  return `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`
})

// 剩余时间样式（剩余时间少于 60 秒时变红）
const remainingTimeClass = computed(() => {
  if (remainingSeconds.value <= 60) {
    return 'text-red-500'
  } else if (remainingSeconds.value <= 300) {
    return 'text-orange-500'
  }
  return 'text-gray-600 dark:text-gray-300'
})

// 启动定时器
const startTimer = () => {
  if (timer) return
  calculateRemainingTime()
  timer = setInterval(calculateRemainingTime, 1000)
}

// 停止定时器
const stopTimer = () => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

// 监听 expireAt 变化
watch(() => props.expireAt, () => {
  stopTimer()
  startTimer()
})

onMounted(() => {
  startTimer()
})

onUnmounted(() => {
  stopTimer()
})
</script>
