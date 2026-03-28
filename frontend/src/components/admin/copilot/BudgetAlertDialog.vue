<template>
  <div
    v-if="visible"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
    @click.self="emit('close')"
  >
    <div class="w-full max-w-sm rounded-lg bg-white p-6 shadow-xl dark:bg-gray-800">
      <h3 class="mb-4 text-base font-semibold text-gray-900 dark:text-white">
        {{ t('admin.copilot.accounts.budgetDialog.title') }}
      </h3>

      <div class="space-y-4">
        <!-- Enabled toggle -->
        <div class="flex items-center justify-between">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.copilot.accounts.budgetDialog.enabled') }}
          </label>
          <button
            type="button"
            class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none"
            :class="form.enabled ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'"
            @click="form.enabled = !form.enabled"
          >
            <span
              class="inline-block h-4 w-4 translate-x-1 rounded-full bg-white shadow transition-transform"
              :class="{ 'translate-x-6': form.enabled }"
            />
          </button>
        </div>

        <!-- Monthly budget -->
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.copilot.accounts.budgetDialog.threshold') }}
          </label>
          <input
            v-model.number="form.monthly_budget"
            type="number"
            min="0"
            step="1"
            class="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
          <p class="mt-1 text-xs text-gray-400">{{ t('admin.copilot.accounts.budgetDialog.thresholdHint') }}</p>
        </div>

        <!-- Alert threshold % -->
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            告警百分比 (%)
          </label>
          <input
            v-model.number="form.alert_threshold"
            type="number"
            min="1"
            max="100"
            step="1"
            class="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
          <p class="mt-1 text-xs text-gray-400">当月费用超过预算的此百分比时触发告警</p>
        </div>
      </div>

      <div class="mt-6 flex justify-end gap-3">
        <button
          class="rounded-md border border-gray-300 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
          @click="emit('close')"
        >
          {{ t('admin.copilot.accounts.budgetDialog.cancel') }}
        </button>
        <button
          :disabled="saving"
          class="rounded-md bg-blue-600 px-4 py-2 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
          @click="save"
        >
          {{ saving ? '保存中…' : t('admin.copilot.accounts.budgetDialog.save') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { upsertCopilotBudgetAlert } from '@/api/admin/copilotAnalytics'
import type { CopilotAccountBudgetAlertInfo } from '@/api/admin/copilotAnalytics'

const { t } = useI18n()

const props = defineProps<{
  visible: boolean
  accountId: number
  initial: CopilotAccountBudgetAlertInfo | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
  (e: 'error', msg: string): void
}>()

const saving = ref(false)
const form = ref({
  monthly_budget: 0,
  alert_threshold: 80,
  enabled: true,
})

watch(() => props.visible, (v) => {
  if (!v) return
  // Always reset form when dialog opens, then overlay with existing config if present.
  form.value = { monthly_budget: 0, alert_threshold: 80, enabled: true }
  if (props.initial) {
    form.value = {
      monthly_budget: props.initial.monthly_budget,
      alert_threshold: props.initial.alert_threshold,
      enabled: props.initial.enabled,
    }
  }
})

async function save() {
  saving.value = true
  try {
    await upsertCopilotBudgetAlert(props.accountId, form.value)
    emit('saved')
  } catch (e: unknown) {
    emit('error', e instanceof Error ? e.message : String(e))
  } finally {
    saving.value = false
  }
}
</script>
