<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6 px-4 py-6 sm:px-6 lg:px-8">
      <section class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div class="space-y-2">
            <p class="text-sm font-medium uppercase tracking-[0.2em] text-primary-500">
              {{ t('admin.cardsIssue.eyebrow') }}
            </p>
            <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
              {{ t('admin.cardsIssue.title') }}
            </h1>
            <p class="max-w-3xl text-sm leading-6 text-gray-600 dark:text-gray-300">
              {{ t('admin.cardsIssue.description') }}
            </p>
          </div>
          <div class="rounded-2xl border border-primary-100 bg-primary-50 px-4 py-3 text-sm text-primary-700 dark:border-primary-900/40 dark:bg-primary-900/10 dark:text-primary-300">
            <div class="font-medium">{{ t('admin.cardsIssue.endpointLabel') }}</div>
            <code class="mt-1 block break-all text-xs">{{ endpointUrl }}</code>
          </div>
        </div>
      </section>

      <section class="grid gap-6 lg:grid-cols-[1.1fr_0.9fr]">
        <div class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="flex items-center justify-between gap-4">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.cardsIssue.configTitle') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.cardsIssue.configDescription') }}
              </p>
            </div>
            <button
              class="btn btn-primary"
              :disabled="saving"
              @click="saveConfig"
            >
              {{ saving ? t('common.saving') : t('common.save') }}
            </button>
          </div>

          <div class="mt-6 space-y-5">
            <label class="flex items-start justify-between gap-4 rounded-2xl border border-gray-200 px-4 py-4 dark:border-dark-700">
              <div>
                <div class="font-medium text-gray-900 dark:text-white">
                  {{ t('admin.cardsIssue.enabledLabel') }}
                </div>
                <div class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.cardsIssue.enabledHelp') }}
                </div>
              </div>
              <input
                v-model="form.enabled"
                type="checkbox"
                class="mt-1 h-5 w-5 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              />
            </label>

            <div>
              <div class="mb-2 flex items-center justify-between gap-3">
                <label class="input-label mb-0">{{ t('admin.cardsIssue.templateLabel') }}</label>
                <span class="text-xs text-gray-400 dark:text-gray-500">
                  {{ t('admin.cardsIssue.templateHint') }}
                </span>
              </div>
              <textarea
                v-model="form.response_template"
                rows="10"
                class="input font-mono text-sm leading-6"
              />
              <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.cardsIssue.templatePlaceholders') }}
              </p>
            </div>
          </div>
        </div>

        <div class="space-y-6">
          <section class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <div class="flex items-start justify-between gap-4">
              <div>
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.cardsIssue.keyTitle') }}
                </h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.cardsIssue.keyDescription') }}
                </p>
              </div>
              <span :class="['badge', config?.key_exists ? 'badge-success' : 'badge-gray']">
                {{ config?.key_exists ? t('admin.cardsIssue.keyConfigured') : t('admin.cardsIssue.keyMissing') }}
              </span>
            </div>

            <div class="mt-5 rounded-2xl border border-dashed border-gray-300 bg-gray-50 px-4 py-4 dark:border-dark-600 dark:bg-dark-900/50">
              <div class="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400">
                {{ t('admin.cardsIssue.maskedKey') }}
              </div>
              <code class="mt-2 block break-all text-sm text-gray-800 dark:text-gray-200">
                {{ config?.masked_key || t('admin.cardsIssue.notConfigured') }}
              </code>
            </div>

            <div
              v-if="latestKey"
              class="mt-4 rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-4 text-sm text-emerald-800 dark:border-emerald-900/40 dark:bg-emerald-900/10 dark:text-emerald-200"
            >
              <div class="flex items-start justify-between gap-3">
                <div>
                  <div class="font-medium">{{ t('admin.cardsIssue.newKeyReady') }}</div>
                  <code class="mt-2 block break-all text-xs">{{ latestKey }}</code>
                </div>
                <button class="btn btn-secondary shrink-0 px-3" @click="copyLatestKey">
                  {{ t('common.copy') }}
                </button>
              </div>
            </div>

            <div class="mt-5 flex flex-wrap gap-3">
              <button class="btn btn-primary" :disabled="keyBusy" @click="regenerateKey">
                {{ keyBusy ? t('common.processing') : t('admin.cardsIssue.regenerateKey') }}
              </button>
              <button
                class="btn btn-secondary"
                :disabled="keyBusy || !config?.key_exists"
                @click="showDeleteDialog = true"
              >
                {{ t('admin.cardsIssue.deleteKey') }}
              </button>
            </div>
          </section>

          <section class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-800">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.cardsIssue.quickStartTitle') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.cardsIssue.quickStartDescription') }}
            </p>
            <div class="mt-4 overflow-x-auto rounded-2xl bg-gray-950 p-4 text-xs text-gray-100">
              <pre class="whitespace-pre-wrap break-all">{{ exampleCurl }}</pre>
            </div>
          </section>
        </div>
      </section>
    </div>

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.cardsIssue.deleteKey')"
      :message="t('admin.cardsIssue.deleteKeyConfirm')"
      :danger="true"
      @confirm="deleteKey"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { CardsIssueAdminConfig } from '@/api/admin/cardsIssue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(false)
const saving = ref(false)
const keyBusy = ref(false)
const showDeleteDialog = ref(false)
const latestKey = ref('')
const config = ref<CardsIssueAdminConfig | null>(null)
const form = reactive({
  enabled: false,
  response_template: '',
})

const endpointUrl = computed(() => `${window.location.origin}/api/custom/cards/issue`)
const exampleCurl = computed(() => `curl -X POST ${endpointUrl.value} \\
  -H "Authorization: Bearer ${latestKey.value || '<your-cards-issue-key>'}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "buyer_id": "buyer-1001",
    "buyer_name": "Alice",
    "order_id": "order-20260422-001",
    "order_amount": 19.9,
    "order_quantity": 2
  }'`)

const syncForm = (value: CardsIssueAdminConfig) => {
  form.enabled = value.enabled
  form.response_template = value.response_template
}

const loadConfig = async () => {
  loading.value = true
  try {
    const data = await adminAPI.cardsIssue.getConfig()
    config.value = data
    syncForm(data)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.cardsIssue.loadFailed'))
  } finally {
    loading.value = false
  }
}

const saveConfig = async () => {
  saving.value = true
  try {
    const data = await adminAPI.cardsIssue.updateConfig({
      enabled: form.enabled,
      response_template: form.response_template,
    })
    config.value = data
    syncForm(data)
    appStore.showSuccess(t('admin.cardsIssue.saveSuccess'))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.cardsIssue.saveFailed'))
  } finally {
    saving.value = false
  }
}

const regenerateKey = async () => {
  keyBusy.value = true
  try {
    const data = await adminAPI.cardsIssue.regenerateKey()
    latestKey.value = data.key
    await loadConfig()
    appStore.showSuccess(t('admin.cardsIssue.keyRegenerated'))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.cardsIssue.keyFailed'))
  } finally {
    keyBusy.value = false
  }
}

const deleteKey = async () => {
  keyBusy.value = true
  try {
    await adminAPI.cardsIssue.deleteKey()
    latestKey.value = ''
    showDeleteDialog.value = false
    await loadConfig()
    appStore.showSuccess(t('admin.cardsIssue.keyDeleted'))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.cardsIssue.keyDeleteFailed'))
  } finally {
    keyBusy.value = false
  }
}

const copyLatestKey = async () => {
  if (!latestKey.value) return
  await copyToClipboard(latestKey.value, t('admin.cardsIssue.copyKeySuccess'))
}

onMounted(() => {
  loadConfig()
})
</script>
