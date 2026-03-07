<template>
  <div class="relative" ref="containerRef">
    <span
      :class="badgeClass"
      class="inline-flex items-center justify-center w-5 h-5 rounded-full text-[10px] font-bold shrink-0 cursor-pointer"
      @mouseenter="handleMouseEnter"
      @mouseleave="handleMouseLeave"
    >
      {{ count }}
    </span>

    <!-- Popover -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition duration-150 ease-out"
        enter-from-class="opacity-0 scale-95"
        enter-to-class="opacity-100 scale-100"
        leave-active-class="transition duration-100 ease-in"
        leave-from-class="opacity-100 scale-100"
        leave-to-class="opacity-0 scale-95"
      >
        <div
          v-if="showPopover"
          class="fixed z-50 w-72 rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-600 dark:bg-dark-800"
          :style="popoverStyle"
          @mouseenter="handlePopoverEnter"
          @mouseleave="handlePopoverLeave"
        >
          <!-- Header -->
          <div class="flex items-center justify-between border-b border-gray-100 px-3 py-2 dark:border-dark-700">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.affinityClients', { count }) }}
            </span>
            <span v-if="loading" class="text-xs text-gray-400">
              <svg class="h-3.5 w-3.5 animate-spin" viewBox="0 0 24 24" fill="none">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
            </span>
          </div>

          <!-- Client list -->
          <div class="max-h-60 overflow-y-auto">
            <div v-if="loading && clients.length === 0" class="px-3 py-4 text-center text-xs text-gray-400">
              {{ t('common.loading') }}...
            </div>
            <div v-else-if="clients.length === 0" class="px-3 py-4 text-center text-xs text-gray-400">
              {{ t('admin.accounts.affinityNoClients') }}
            </div>
            <div v-else class="divide-y divide-gray-50 dark:divide-dark-700">
              <div
                v-for="(client, index) in clients"
                :key="index"
                class="flex items-center justify-between px-3 py-1.5"
              >
                <span class="font-mono text-xs text-gray-700 dark:text-gray-300 truncate mr-2" :title="client.client_id">
                  {{ client.client_id }}
                </span>
                <span class="text-[10px] text-gray-400 dark:text-gray-500 whitespace-nowrap shrink-0">
                  {{ formatRelativeTime(client.last_active) }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { getAffinityClients } from '@/api/admin/accounts'

interface Props {
  accountId: number
  count: number
}

const props = defineProps<Props>()
const { t } = useI18n()

const containerRef = ref<HTMLElement | null>(null)
const showPopover = ref(false)
const loading = ref(false)
const clients = ref<{ client_id: string; last_active: string }[]>([])
let loaded = false
let hideTimer: ReturnType<typeof setTimeout> | null = null
let showTimer: ReturnType<typeof setTimeout> | null = null

const badgeClass = computed(() => {
  const c = props.count
  if (c >= 16) return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  if (c >= 6) return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  if (c > 0) return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
  return 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
})

const popoverStyle = computed(() => {
  if (!containerRef.value) return {}
  const rect = containerRef.value.getBoundingClientRect()
  const viewportHeight = window.innerHeight
  const viewportWidth = window.innerWidth

  let top = rect.bottom + 6
  let left = rect.left - 40

  // 下方空间不足时显示在上方
  if (top + 280 > viewportHeight) {
    top = Math.max(8, rect.top - 280)
  }
  // 右侧空间不足时向左偏移
  if (left + 288 > viewportWidth) {
    left = Math.max(8, viewportWidth - 296)
  }
  // 不超出左边界
  if (left < 8) left = 8

  return { top: `${top}px`, left: `${left}px` }
})

function clearTimers() {
  if (hideTimer) { clearTimeout(hideTimer); hideTimer = null }
  if (showTimer) { clearTimeout(showTimer); showTimer = null }
}

function handleMouseEnter() {
  clearTimers()
  showTimer = setTimeout(() => {
    showPopover.value = true
    if (!loaded) fetchClients()
  }, 200)
}

function handleMouseLeave() {
  clearTimers()
  hideTimer = setTimeout(() => { showPopover.value = false }, 150)
}

function handlePopoverEnter() {
  clearTimers()
}

function handlePopoverLeave() {
  clearTimers()
  hideTimer = setTimeout(() => { showPopover.value = false }, 150)
}

async function fetchClients() {
  loading.value = true
  try {
    clients.value = await getAffinityClients(props.accountId)
    loaded = true
  } catch {
    clients.value = []
  } finally {
    loading.value = false
  }
}

function formatRelativeTime(isoStr: string): string {
  const now = Date.now()
  const then = new Date(isoStr).getTime()
  const diffSec = Math.floor((now - then) / 1000)

  if (diffSec < 60) return t('common.justNow', 'just now')
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`
  return `${Math.floor(diffSec / 86400)}d ago`
}
</script>
