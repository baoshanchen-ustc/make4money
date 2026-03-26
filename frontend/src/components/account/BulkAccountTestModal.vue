<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.bulkTest.title')"
    width="wide"
    @close="handleClose"
  >
    <div class="space-y-4">
      <div class="grid gap-3 sm:grid-cols-4">
        <div class="rounded-xl border border-gray-200 bg-gray-50 p-3 dark:border-dark-500 dark:bg-dark-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.bulkTest.total') }}</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-gray-100">{{ items.length }}</div>
        </div>
        <div class="rounded-xl border border-green-200 bg-green-50 p-3 dark:border-green-900/40 dark:bg-green-900/10">
          <div class="text-xs text-green-700 dark:text-green-400">{{ t('admin.accounts.bulkTest.success') }}</div>
          <div class="mt-1 text-xl font-semibold text-green-700 dark:text-green-300">{{ successCount }}</div>
        </div>
        <div class="rounded-xl border border-red-200 bg-red-50 p-3 dark:border-red-900/40 dark:bg-red-900/10">
          <div class="text-xs text-red-700 dark:text-red-400">{{ t('admin.accounts.bulkTest.failed') }}</div>
          <div class="mt-1 text-xl font-semibold text-red-700 dark:text-red-300">{{ failedCount }}</div>
        </div>
        <div class="rounded-xl border border-amber-200 bg-amber-50 p-3 dark:border-amber-900/40 dark:bg-amber-900/10">
          <div class="text-xs text-amber-700 dark:text-amber-400">{{ t('admin.accounts.bulkTest.progress') }}</div>
          <div class="mt-1 text-xl font-semibold text-amber-700 dark:text-amber-300">{{ completedCount }}/{{ items.length }}</div>
        </div>
      </div>

      <div v-if="showModelSelector" class="space-y-1.5">
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.accounts.bulkTest.sharedModel') }}
        </label>
        <Select
          v-model="selectedModelId"
          :options="availableModels"
          :disabled="loadingModels || running"
          value-key="id"
          label-key="display_name"
          :placeholder="loadingModels ? t('common.loading') + '...' : t('admin.accounts.selectTestModel')"
        />
      </div>

      <div v-if="supportsGeminiImagePrompt" class="space-y-1.5">
        <TextArea
          v-model="testPrompt"
          :label="t('admin.accounts.geminiImagePromptLabel')"
          :placeholder="t('admin.accounts.geminiImagePromptPlaceholder')"
          :hint="t('admin.accounts.geminiImageTestHint')"
          :disabled="running"
          rows="3"
        />
      </div>

      <div class="grid gap-4 lg:grid-cols-[260px,minmax(0,1fr)]">
        <div class="rounded-xl border border-gray-200 bg-white p-3 dark:border-dark-500 dark:bg-dark-700">
          <div class="mb-2 flex items-center justify-between">
            <div class="text-sm font-medium text-gray-700 dark:text-gray-200">
              {{ t('admin.accounts.bulkTest.queue') }}
            </div>
            <div v-if="currentItem" class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.bulkTest.current', { name: currentItem.account.name }) }}
            </div>
          </div>

          <div class="max-h-[360px] space-y-2 overflow-y-auto">
            <button
              v-for="item in items"
              :key="item.account.id"
              type="button"
              class="w-full rounded-lg border px-3 py-2 text-left transition-colors"
              :class="itemButtonClass(item)"
              @click="activeAccountId = item.account.id"
            >
              <div class="flex items-center justify-between gap-2">
                <div class="min-w-0">
                  <div class="truncate text-sm font-medium">{{ item.account.name }}</div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">{{ item.account.type }}</div>
                </div>
                <span class="rounded-full px-2 py-0.5 text-[11px] font-semibold" :class="itemStatusClass(item.status)">
                  {{ statusLabel(item.status) }}
                </span>
              </div>
            </button>
          </div>
        </div>

        <div class="group relative">
          <div
            ref="terminalRef"
            class="max-h-[420px] min-h-[260px] overflow-y-auto rounded-xl border border-gray-700 bg-gray-900 p-4 font-mono text-sm dark:border-gray-800 dark:bg-black"
          >
            <div v-if="!displayedItem" class="flex items-center gap-2 text-gray-500">
              <Icon name="play" size="sm" :stroke-width="2" />
              <span>{{ t('admin.accounts.bulkTest.ready') }}</span>
            </div>
            <template v-else>
              <div class="mb-3 flex items-center justify-between text-xs text-gray-400">
                <span>{{ displayedItem.account.name }}</span>
                <span>{{ statusLabel(displayedItem.status) }}</span>
              </div>

              <div v-for="(line, index) in displayedItem.outputLines" :key="index" :class="line.class">
                {{ line.text }}
              </div>

              <div v-if="displayedItem.streamingContent" class="text-green-400">
                {{ displayedItem.streamingContent }}<span class="animate-pulse">_</span>
              </div>

              <div
                v-if="displayedItem.status === 'success'"
                class="mt-3 flex items-center gap-2 border-t border-gray-700 pt-3 text-green-400"
              >
                <Icon name="check" size="sm" :stroke-width="2" />
                <span>{{ t('admin.accounts.testCompleted') }}</span>
              </div>
              <div
                v-else-if="displayedItem.status === 'error'"
                class="mt-3 flex items-center gap-2 border-t border-gray-700 pt-3 text-red-400"
              >
                <Icon name="x" size="sm" :stroke-width="2" />
                <span>{{ displayedItem.errorMessage || t('admin.accounts.testFailed') }}</span>
              </div>
              <div
                v-else-if="displayedItem.status === 'skipped'"
                class="mt-3 flex items-center gap-2 border-t border-gray-700 pt-3 text-amber-400"
              >
                <Icon name="clock" size="sm" :stroke-width="2" />
                <span>{{ t('admin.accounts.bulkTest.skippedByStop') }}</span>
              </div>
            </template>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button
          class="rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
          :disabled="running"
          @click="handleClose"
        >
          {{ t('common.close') }}
        </button>
        <button
          v-if="running"
          class="rounded-lg bg-amber-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-amber-600"
          :disabled="stopRequested"
          @click="stopRequested = true"
        >
          {{ stopRequested ? t('admin.accounts.bulkTest.stopping') : t('admin.accounts.bulkTest.stopAfterCurrent') }}
        </button>
        <button
          class="flex items-center gap-2 rounded-lg bg-primary-500 px-4 py-2 text-sm font-medium text-white transition-all hover:bg-primary-600 disabled:cursor-not-allowed disabled:bg-primary-300"
          :disabled="running || loadingModels || items.length === 0 || (showModelSelector && !selectedModelId)"
          @click="startTest"
        >
          <Icon
            v-if="running"
            name="refresh"
            size="sm"
            class="animate-spin"
            :stroke-width="2"
          />
          <Icon v-else name="play" size="sm" :stroke-width="2" />
          <span>{{ completed ? t('admin.accounts.retry') : t('admin.accounts.startTest') }}</span>
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import { Icon } from '@/components/icons'
import { adminAPI } from '@/api/admin'
import type { Account, ClaudeModel } from '@/types'
import { streamAccountTest, type AccountTestStreamEvent } from '@/utils/accountTestStream'

interface OutputLine {
  text: string
  class: string
}

type BulkTestStatus = 'pending' | 'running' | 'success' | 'error' | 'skipped'

interface BulkTestItem {
  account: Account
  status: BulkTestStatus
  outputLines: OutputLine[]
  streamingContent: string
  errorMessage: string
  imageCount: number
}

const prioritizedGeminiModels = [
  'gemini-3.1-flash-image',
  'gemini-2.5-flash-image',
  'gemini-2.5-flash',
  'gemini-2.5-pro',
  'gemini-3-flash-preview',
  'gemini-3-pro-preview',
  'gemini-2.0-flash'
]

const props = defineProps<{
  show: boolean
  accounts: Account[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'completed', payload: { failedIds: number[]; skippedIds: number[]; retryIds: number[] }): void
}>()

const { t } = useI18n()

const terminalRef = ref<HTMLElement | null>(null)
const items = ref<BulkTestItem[]>([])
const activeAccountId = ref<number | null>(null)
const availableModels = ref<ClaudeModel[]>([])
const selectedModelId = ref('')
const testPrompt = ref('')
const loadingModels = ref(false)
const running = ref(false)
const completed = ref(false)
const stopRequested = ref(false)

const commonPlatform = computed(() => {
  const platforms = new Set(props.accounts.map((account) => account.platform))
  return platforms.size === 1 ? props.accounts[0]?.platform || '' : ''
})

const allAntigravityAPIKey = computed(() =>
  props.accounts.length > 0 && props.accounts.every((account) => account.platform === 'antigravity' && account.type === 'apikey')
)

const showModelSelector = computed(() =>
  !!commonPlatform.value && commonPlatform.value !== 'sora' && availableModels.value.length > 0
)

const supportsGeminiImagePrompt = computed(() => {
  if (!showModelSelector.value) return false
  const normalized = selectedModelId.value.trim().toLowerCase()
  if (!normalized.startsWith('gemini-') || !normalized.includes('-image')) return false
  return commonPlatform.value === 'gemini' || allAntigravityAPIKey.value
})

const displayedItem = computed(() => items.value.find((item) => item.account.id === activeAccountId.value) || null)
const currentItem = computed(() => items.value.find((item) => item.status === 'running') || null)

const successCount = computed(() => items.value.filter((item) => item.status === 'success').length)
const failedCount = computed(() => items.value.filter((item) => item.status === 'error').length)
const completedCount = computed(() => items.value.filter((item) => ['success', 'error', 'skipped'].includes(item.status)).length)

const sortTestModels = (models: ClaudeModel[]) => {
  const priorityMap = new Map(prioritizedGeminiModels.map((id, index) => [id, index]))
  return [...models].sort((a, b) => {
    const aPriority = priorityMap.get(a.id) ?? Number.MAX_SAFE_INTEGER
    const bPriority = priorityMap.get(b.id) ?? Number.MAX_SAFE_INTEGER
    if (aPriority !== bPriority) return aPriority - bPriority
    return 0
  })
}

const resetItems = () => {
  items.value = props.accounts.map((account) => ({
    account,
    status: 'pending',
    outputLines: [],
    streamingContent: '',
    errorMessage: '',
    imageCount: 0
  }))
  activeAccountId.value = items.value[0]?.account.id ?? null
}

const resetState = () => {
  resetItems()
  availableModels.value = []
  selectedModelId.value = ''
  testPrompt.value = ''
  loadingModels.value = false
  running.value = false
  completed.value = false
  stopRequested.value = false
}

const ensureDefaultPrompt = () => {
  if (supportsGeminiImagePrompt.value && !testPrompt.value.trim()) {
    testPrompt.value = t('admin.accounts.geminiImagePromptDefault')
  }
}

watch(selectedModelId, ensureDefaultPrompt)

watch(
  () => props.show,
  async (visible) => {
    if (!visible) {
      if (!running.value) {
        resetState()
      }
      return
    }

    resetState()
    if (commonPlatform.value && commonPlatform.value !== 'sora') {
      await loadSharedModels()
    }
  }
)

const loadSharedModels = async () => {
  if (!commonPlatform.value || commonPlatform.value === 'sora' || props.accounts.length === 0) return

  loadingModels.value = true
  try {
    const modelLists = await Promise.all(props.accounts.map((account) => adminAPI.accounts.getAvailableModels(account.id)))
    if (modelLists.length === 0) {
      availableModels.value = []
      return
    }

    const firstModels = commonPlatform.value === 'gemini' || commonPlatform.value === 'antigravity'
      ? sortTestModels(modelLists[0])
      : modelLists[0]

    const intersection = firstModels.filter((model) =>
      modelLists.every((models) => models.some((candidate) => candidate.id === model.id))
    )

    availableModels.value = intersection
    if (intersection.length > 0) {
      if (commonPlatform.value === 'gemini') {
        selectedModelId.value = intersection[0].id
      } else {
        const sonnetModel = intersection.find((model) => model.id.includes('sonnet'))
        selectedModelId.value = sonnetModel?.id || intersection[0].id
      }
      ensureDefaultPrompt()
    }
  } catch (error) {
    console.error('Failed to load shared test models:', error)
    availableModels.value = []
    selectedModelId.value = ''
  } finally {
    loadingModels.value = false
  }
}

const scrollToBottom = async () => {
  await nextTick()
  if (terminalRef.value) {
    terminalRef.value.scrollTop = terminalRef.value.scrollHeight
  }
}

const getItem = (accountId: number) => items.value.find((item) => item.account.id === accountId) || null

const addLine = async (accountId: number, text: string, className: string = 'text-gray-300') => {
  const item = getItem(accountId)
  if (!item) return
  item.outputLines.push({ text, class: className })
  if (activeAccountId.value === accountId) {
    await scrollToBottom()
  }
}

const flushStreamingContent = async (item: BulkTestItem, className: string = 'text-green-300') => {
  if (!item.streamingContent) return
  item.outputLines.push({ text: item.streamingContent, class: className })
  item.streamingContent = ''
  if (activeAccountId.value === item.account.id) {
    await scrollToBottom()
  }
}

const isSoraAccount = (account: Account) => account.platform === 'sora'

const usesGeminiImagePrompt = (account: Account) => {
  if (!supportsGeminiImagePrompt.value) return false
  if (account.platform === 'gemini') return true
  return account.platform === 'antigravity' && account.type === 'apikey'
}

const handleItemEvent = async (accountId: number, event: AccountTestStreamEvent) => {
  const item = getItem(accountId)
  if (!item) return

  switch (event.type) {
    case 'test_start':
      await addLine(accountId, t('admin.accounts.connectedToApi'), 'text-green-400')
      if (event.model) {
        await addLine(accountId, t('admin.accounts.usingModel', { model: event.model }), 'text-cyan-400')
      }
      await addLine(
        accountId,
        isSoraAccount(item.account)
          ? t('admin.accounts.soraTestingFlow')
          : usesGeminiImagePrompt(item.account)
            ? t('admin.accounts.sendingGeminiImageRequest')
            : t('admin.accounts.sendingTestMessage'),
        'text-gray-400'
      )
      await addLine(accountId, '', 'text-gray-300')
      await addLine(accountId, t('admin.accounts.response'), 'text-yellow-400')
      break

    case 'content':
      if (event.text) {
        item.streamingContent += event.text
        if (activeAccountId.value === accountId) {
          await scrollToBottom()
        }
      }
      break

    case 'image':
      item.imageCount += 1
      await addLine(accountId, t('admin.accounts.geminiImageReceived', { count: item.imageCount }), 'text-purple-300')
      break

    case 'test_complete':
      await flushStreamingContent(item)
      if (event.success) {
        item.status = 'success'
      } else {
        item.status = 'error'
        item.errorMessage = event.error || 'Test failed'
      }
      break

    case 'error':
      await flushStreamingContent(item)
      item.status = 'error'
      item.errorMessage = event.error || 'Unknown error'
      break
  }
}

const statusLabel = (status: BulkTestStatus) => {
  switch (status) {
    case 'running':
      return t('admin.accounts.bulkTest.statusRunning')
    case 'success':
      return t('admin.accounts.bulkTest.statusSuccess')
    case 'error':
      return t('admin.accounts.bulkTest.statusFailed')
    case 'skipped':
      return t('admin.accounts.bulkTest.statusSkipped')
    default:
      return t('admin.accounts.bulkTest.statusPending')
  }
}

const itemStatusClass = (status: BulkTestStatus) => {
  switch (status) {
    case 'running':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
    case 'success':
      return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
    case 'error':
      return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    case 'skipped':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-600 dark:text-gray-300'
  }
}

const itemButtonClass = (item: BulkTestItem) => {
  const isActive = item.account.id === activeAccountId.value
  if (isActive) {
    return 'border-primary-300 bg-primary-50 dark:border-primary-700 dark:bg-primary-900/20'
  }
  return 'border-gray-200 bg-gray-50 hover:bg-gray-100 dark:border-dark-500 dark:bg-dark-700 dark:hover:bg-dark-600'
}

const markRemainingSkipped = () => {
  for (const item of items.value) {
    if (item.status === 'pending') {
      item.status = 'skipped'
      item.errorMessage = t('admin.accounts.bulkTest.skippedByStop')
    }
  }
}

const startTest = async () => {
  if (running.value || items.value.length === 0) return

  resetItems()
  completed.value = false
  stopRequested.value = false
  running.value = true

  for (const item of items.value) {
    if (stopRequested.value) {
      markRemainingSkipped()
      break
    }

    activeAccountId.value = item.account.id
    item.status = 'running'
    await addLine(item.account.id, t('admin.accounts.startingTestForAccount', { name: item.account.name }), 'text-blue-400')
    await addLine(item.account.id, t('admin.accounts.testAccountTypeLabel', { type: item.account.type }), 'text-gray-400')
    await addLine(item.account.id, '', 'text-gray-300')

    try {
      await streamAccountTest(item.account.id, {
        isSora: isSoraAccount(item.account),
        modelId: showModelSelector.value ? selectedModelId.value : undefined,
        prompt: usesGeminiImagePrompt(item.account) ? testPrompt.value.trim() : undefined,
        onEvent: (event) => handleItemEvent(item.account.id, event)
      })
      if (item.status === 'running') {
        await flushStreamingContent(item)
        item.status = 'success'
      }
    } catch (error: any) {
      await flushStreamingContent(item)
      item.status = 'error'
      item.errorMessage = error?.message || 'Unknown error'
      await addLine(item.account.id, `Error: ${item.errorMessage}`, 'text-red-400')
    }
  }

  if (stopRequested.value) {
    markRemainingSkipped()
  }

  running.value = false
  completed.value = true

  const failedIds = items.value.filter((item) => item.status === 'error').map((item) => item.account.id)
  const skippedIds = items.value.filter((item) => item.status === 'skipped').map((item) => item.account.id)
  emit('completed', {
    failedIds,
    skippedIds,
    retryIds: [...failedIds, ...skippedIds]
  })
}

const handleClose = () => {
  if (running.value) return
  emit('close')
}
</script>
