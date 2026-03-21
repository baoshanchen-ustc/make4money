<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.copilotModels.title')"
    width="wide"
    @close="handleClose"
  >
    <div class="space-y-5">
      <!-- Account Info Header -->
      <div
        v-if="account"
        class="flex items-center justify-between rounded-xl border border-cyan-200 bg-gradient-to-r from-cyan-50 to-cyan-100 p-4 dark:border-cyan-700/50 dark:from-cyan-900/20 dark:to-cyan-800/20"
      >
        <div class="flex items-center gap-3">
          <div
            class="flex h-10 w-10 items-center justify-center rounded-lg bg-gradient-to-br from-cyan-500 to-cyan-600"
          >
            <Icon name="cube" size="md" class="text-white" />
          </div>
          <div>
            <div class="font-semibold text-gray-900 dark:text-gray-100">{{ account.name }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.copilotModels.subtitle') }}
            </div>
          </div>
        </div>
        <span
          :class="[
            'rounded-full px-2.5 py-1 text-xs font-semibold',
            account.status === 'active'
              ? 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-400'
              : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
          ]"
        >
          {{ account.status }}
        </span>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <!-- Error -->
      <div
        v-else-if="error"
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-700 dark:bg-red-900/20 dark:text-red-300"
      >
        {{ error }}
      </div>

      <!-- Models List -->
      <template v-else-if="models.length > 0">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.copilotModels.totalCount', { count: models.length }) }}
        </div>
        <div class="max-h-[60vh] overflow-y-auto rounded-lg border border-gray-200 dark:border-dark-500">
          <table class="w-full text-left text-sm">
            <thead class="sticky top-0 bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-700 dark:text-gray-400">
              <tr>
                <th class="whitespace-nowrap px-5 py-3">{{ t('admin.accounts.copilotModels.modelId') }}</th>
                <th class="whitespace-nowrap px-5 py-3">{{ t('admin.accounts.copilotModels.displayName') }}</th>
                <th class="whitespace-nowrap px-5 py-3">{{ t('admin.accounts.copilotModels.endpoints') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="model in models"
                :key="model.id"
                class="border-t border-gray-100 transition-colors hover:bg-gray-50/50 dark:border-dark-600 dark:hover:bg-dark-700/40"
              >
                <td class="px-5 py-3">
                  <code class="rounded bg-gray-100 px-2 py-1 text-xs font-medium text-gray-800 dark:bg-dark-600 dark:text-gray-200">
                    {{ model.id }}
                  </code>
                </td>
                <td class="px-5 py-3 text-gray-700 dark:text-gray-300">
                  {{ model.display_name || model.name || model.id }}
                </td>
                <td class="px-5 py-3">
                  <div class="flex flex-wrap gap-1.5">
                    <span
                      v-for="ep in (model.supported_endpoints || [])"
                      :key="ep"
                      class="rounded-md bg-cyan-100 px-2 py-0.5 text-[11px] font-medium text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300"
                    >
                      {{ ep }}
                    </span>
                    <span
                      v-if="!model.supported_endpoints || model.supported_endpoints.length === 0"
                      class="text-xs text-gray-400"
                    >—</span>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <!-- Empty -->
      <div v-else class="py-12 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.copilotModels.empty') }}
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { Icon } from '@/components/icons'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'

interface CopilotModel {
  id: string
  object?: string
  type?: string
  name?: string
  display_name?: string
  supported_endpoints?: string[]
}

const props = defineProps<{ show: boolean; account: Account | null }>()
const emit = defineEmits(['close'])
const { t } = useI18n()

const loading = ref(false)
const error = ref('')
const models = ref<CopilotModel[]>([])

const loadModels = async () => {
  if (!props.account) return
  loading.value = true
  error.value = ''
  models.value = []
  try {
    const data = await adminAPI.accounts.getAvailableModels(props.account.id)
    const list = data as unknown as CopilotModel[]
    list.sort((a, b) => a.id.localeCompare(b.id))
    models.value = list
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (visible) => {
    if (visible && props.account) loadModels()
  },
  { immediate: true }
)

const handleClose = () => emit('close')
</script>
