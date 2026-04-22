<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loadingKeys" class="flex justify-center py-12">
        <LoadingSpinner />
      </div>

      <!-- No API Keys State -->
      <div v-else-if="apiKeys.length === 0" class="card p-12 text-center">
        <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700">
          <Icon name="key" size="xl" class="text-gray-400" />
        </div>
        <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('models.noApiKeys') }}
        </h3>
        <p class="mb-4 text-gray-500 dark:text-dark-400">
          {{ t('models.noApiKeysDesc') }}
        </p>
        <button @click="$router.push('/keys')" class="btn btn-primary">
          <Icon name="plus" size="sm" class="mr-2" />
          {{ t('models.createApiKey') }}
        </button>
      </div>

      <!-- API Key Selection -->
      <template v-else>
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('models.selectApiKey') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('models.selectApiKeyHint') }}</p>
          </div>
          <div class="p-6">
            <Select
              v-model="selectedKeyId"
              :options="apiKeyOptions"
              :placeholder="t('models.selectApiKeyPlaceholder')"
              class="w-full sm:w-96"
              @update:model-value="onApiKeyChange"
            />
          </div>
        </div>

        <!-- Models Content -->
        <template v-if="selectedApiKey">
          <!-- Loading Models -->
          <div v-if="loading" class="flex justify-center py-12">
            <LoadingSpinner />
          </div>

          <!-- Error State -->
          <div v-else-if="error" class="card p-6">
            <div class="flex items-start gap-3">
              <div class="rounded-lg bg-red-100 p-2 dark:bg-red-900/30">
                <Icon name="x" size="md" class="text-red-600 dark:text-red-400" />
              </div>
              <div class="flex-1">
                <h3 class="font-semibold text-red-900 dark:text-red-200">{{ t('models.loadFailed') }}</h3>
                <p class="mt-1 text-sm text-red-700 dark:text-red-300">{{ error }}</p>
                <button @click="loadModels" class="btn btn-secondary mt-3">
                  <Icon name="refresh" size="sm" class="mr-2" />
                  {{ t('common.retry') }}
                </button>
              </div>
            </div>
          </div>

          <!-- No Models -->
          <div v-else-if="models.length === 0" class="card p-12 text-center">
            <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700">
              <Icon name="inbox" size="xl" class="text-gray-400" />
            </div>
            <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('models.noModels') }}
            </h3>
            <p class="text-gray-500 dark:text-dark-400">
              {{ t('models.noModelsDesc') }}
            </p>
          </div>

          <!-- Models List & Usage Guide -->
          <template v-else>
            <!-- Usage Guide -->
            <div class="card">
              <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('models.usageGuide') }}</h2>
              </div>
              <div class="space-y-6 p-6">
                <!-- API Endpoint -->
                <div>
                  <label class="input-label">{{ t('models.apiEndpoint') }}</label>
                  <div class="flex items-center gap-2">
                    <div class="code flex-1 overflow-x-auto">{{ apiBaseUrl }}</div>
                    <button
                      @click="copyToClipboard(apiBaseUrl)"
                      class="btn btn-secondary shrink-0"
                      :title="t('common.copy')"
                    >
                      <Icon name="clipboard" size="sm" />
                    </button>
                  </div>
                </div>

                <!-- API Key -->
                <div>
                  <label class="input-label">{{ t('models.yourApiKey') }}</label>
                  <div class="flex items-center gap-2">
                    <div class="code flex-1 overflow-x-auto">{{ selectedApiKey.key }}</div>
                    <button
                      @click="copyToClipboard(selectedApiKey.key)"
                      class="btn btn-secondary shrink-0"
                      :title="t('common.copy')"
                    >
                      <Icon name="clipboard" size="sm" />
                    </button>
                  </div>
                </div>

                <!-- Example Request -->
                <div>
                  <label class="input-label">{{ t('models.exampleRequest') }}</label>
                  <div class="relative">
                    <pre class="code overflow-x-auto rounded-lg bg-gray-900 p-4 text-xs text-gray-100 dark:bg-black"><code>{{ exampleCode }}</code></pre>
                    <button
                      @click="copyToClipboard(exampleCode)"
                      class="absolute right-2 top-2 rounded-lg bg-gray-800 p-2 text-gray-400 transition-colors hover:bg-gray-700 hover:text-white"
                      :title="t('common.copy')"
                    >
                      <Icon name="clipboard" size="sm" />
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Available Models -->
            <div class="card">
              <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
                <div class="flex items-center justify-between">
                  <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                    {{ t('models.availableModels') }} ({{ models.length }})
                  </h2>
                  <button @click="loadModels" :disabled="loading" class="btn btn-secondary">
                    <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
                  </button>
                </div>
              </div>
              <div class="overflow-x-auto">
                <table class="w-full">
                  <thead class="border-b border-gray-100 bg-gray-50 dark:border-dark-700 dark:bg-dark-800/50">
                    <tr>
                      <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                        {{ t('models.modelId') }}
                      </th>
                      <th class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                        {{ t('models.displayName') }}
                      </th>
                      <th class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-dark-400">
                        {{ t('models.actions') }}
                      </th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                    <tr
                      v-for="model in models"
                      :key="model.id"
                      class="transition-colors hover:bg-gray-50 dark:hover:bg-dark-800/50"
                    >
                      <td class="px-6 py-4">
                        <code class="code text-sm">{{ model.id }}</code>
                      </td>
                      <td class="px-6 py-4">
                        <span class="text-sm text-gray-900 dark:text-white">
                          {{ model.display_name || model.id }}
                        </span>
                      </td>
                      <td class="px-6 py-4 text-right">
                        <button
                          @click="copyToClipboard(model.id)"
                          class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm text-gray-600 transition-colors hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-700"
                        >
                          <Icon name="clipboard" size="sm" />
                          {{ t('common.copy') }}
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </template>
        </template>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { list as listApiKeys } from '@/api/keys'
import axios from 'axios'
import type { ClaudeModel, ApiKey } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const apiKeys = ref<ApiKey[]>([])
const selectedKeyId = ref<number | null>(null)
const models = ref<ClaudeModel[]>([])
const loading = ref(false)
const loadingKeys = ref(false)
const error = ref<string | null>(null)

const selectedApiKey = computed(() => apiKeys.value.find(k => k.id === selectedKeyId.value) || null)
const apiBaseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || 'https://api.example.com')

const apiKeyOptions = computed(() => [
  ...apiKeys.value.map(key => ({
    value: key.id,
    label: `${key.name} (${maskKey(key.key)})`
  }))
])

const exampleCode = computed(() => {
  if (!selectedApiKey.value) return ''
  const modelId = models.value[0]?.id || 'claude-3-5-sonnet-20241022'
  return `curl ${apiBaseUrl.value}/v1/messages \\
  -H "Content-Type: application/json" \\
  -H "x-api-key: ${selectedApiKey.value.key}" \\
  -H "anthropic-version: 2023-06-01" \\
  -d '{
    "model": "${modelId}",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello, Claude"}
    ]
  }'`
})

function maskKey(key: string): string {
  if (key.length <= 12) return key
  return key.substring(0, 8) + '...' + key.substring(key.length - 4)
}

async function loadApiKeysData() {
  loadingKeys.value = true
  try {
    const response = await listApiKeys()
    apiKeys.value = response.items.filter((key: ApiKey) => key.status === 'active')

    // Auto-select first key if available
    if (apiKeys.value.length > 0 && !selectedKeyId.value) {
      selectedKeyId.value = apiKeys.value[0].id
      await loadModels()
    }
  } catch (err: any) {
    console.error('Failed to load API keys:', err)
  } finally {
    loadingKeys.value = false
  }
}

async function onApiKeyChange(value: string | number | boolean | null) {
  const keyId = typeof value === 'number' ? value : null
  selectedKeyId.value = keyId
  if (keyId) {
    await loadModels()
  }
}

async function loadModels() {
  if (!selectedApiKey.value) return

  loading.value = true
  error.value = null
  try {
    const response = await axios.get('/v1/models', {
      headers: {
        'x-api-key': selectedApiKey.value.key
      }
    })
    models.value = response.data.data || []
  } catch (err: any) {
    console.error('Failed to load models:', err)
    error.value = err.response?.data?.message || err.message || t('models.loadFailed')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadApiKeysData()
})
</script>
