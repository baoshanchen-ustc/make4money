<template>
  <div
    v-if="hasAnomalyTypes"
    class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900"
  >
    <!-- Header -->
    <div
      class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700"
    >
      <div>
        <h3 class="text-sm font-black uppercase tracking-wider text-gray-900 dark:text-white">
          {{ t('admin.ops.rawData.title') }}
        </h3>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.rawData.description') }}
        </p>
      </div>
      <div v-if="props.anomalyTypes?.length" class="flex flex-wrap gap-1.5">
        <AnomalyBadge v-for="type in props.anomalyTypes" :key="type" :type="type" small />
      </div>
    </div>

    <!-- Accordion sections (only when at least one body was saved) -->
    <div v-if="hasAnyBody" class="divide-y divide-gray-100 dark:divide-dark-700">
      <section v-for="section in sections" :key="section.key" class="px-4 py-3">
        <!-- Section header -->
        <div class="flex items-center gap-3">
          <button
            type="button"
            class="flex flex-1 items-center justify-between gap-3 text-left"
            :aria-expanded="openSections[section.key]"
            @click="toggleSection(section.key)"
          >
            <span class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t(section.titleKey) }}
            </span>
            <span class="text-[11px] font-bold text-gray-400 dark:text-gray-500">
              {{ openSections[section.key] ? '▲' : '▼' }}
            </span>
          </button>

          <button
            type="button"
            class="flex-shrink-0 rounded-md bg-gray-100 px-2 py-1 text-[10px] font-bold text-gray-600 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-40 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
            :disabled="!hasBody(section.data)"
            @click="handleCopy(section.data)"
          >
            {{ t('admin.ops.requestDetails.copy') }}
          </button>
        </div>

        <!-- Expanded body -->
        <div v-if="openSections[section.key]" class="mt-3 space-y-2">
          <!-- Truncation warning -->
          <div
            v-if="isTruncated(section.data)"
            class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200"
          >
            {{ t('admin.ops.rawData.truncated') }}
          </div>

          <div
            v-if="hasBody(section.data)"
            class="overflow-hidden rounded-xl border border-gray-200 bg-gray-950/95 dark:border-dark-700 dark:bg-dark-950"
          >
            <pre
              class="max-h-[400px] overflow-auto p-4"
            ><code class="block whitespace-pre text-xs leading-6 text-gray-100" v-html="highlightJson(section.data)"></code></pre>
          </div>

          <div
            v-else
            class="rounded-xl bg-gray-50 px-4 py-5 text-sm text-gray-500 dark:bg-dark-800 dark:text-gray-400"
          >
            {{ t('common.noData') }}
          </div>
        </div>
      </section>
    </div>

    <!-- When anomaly types exist but bodies were not saved (save_raw_data=false) -->
    <div
      v-if="!hasAnyBody"
      class="px-4 py-5 text-sm text-gray-500 dark:text-gray-400"
    >
      {{ t('admin.ops.rawData.notSaved') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import type { AnomalyType } from '@/api/admin/ops'
import { useClipboard } from '@/composables/useClipboard'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AnomalyBadge from './AnomalyBadge.vue'

type SectionKey = 'clientRequest' | 'upstreamRequest' | 'upstreamResponse'

interface SectionItem {
  key: SectionKey
  titleKey: string
  data: unknown
}

interface Props {
  requestBody?: unknown
  upstreamRequestBody?: unknown
  upstreamResponseBody?: unknown
  anomalyTypes?: AnomalyType[] | null
}

const props = defineProps<Props>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const openSections = ref<Record<SectionKey, boolean>>({
  clientRequest: true,
  upstreamRequest: false,
  upstreamResponse: false
})

const sections = computed<SectionItem[]>(() => [
  { key: 'clientRequest', titleKey: 'admin.ops.rawData.clientRequest', data: props.requestBody },
  { key: 'upstreamRequest', titleKey: 'admin.ops.rawData.upstreamRequest', data: props.upstreamRequestBody },
  { key: 'upstreamResponse', titleKey: 'admin.ops.rawData.upstreamResponse', data: props.upstreamResponseBody }
])

const hasAnyBody = computed(() => sections.value.some((s) => hasBody(s.data)))
const hasAnomalyTypes = computed(() => (props.anomalyTypes || []).length > 0)

function hasBody(value: unknown): boolean {
  return value !== null && value !== undefined
}

function toggleSection(key: SectionKey) {
  openSections.value[key] = !openSections.value[key]
}

function isTruncated(value: unknown): boolean {
  return (
    value !== null &&
    typeof value === 'object' &&
    '_truncated' in (value as Record<string, unknown>) &&
    (value as Record<string, unknown>)._truncated === true
  )
}

function prettyJson(value: unknown): string {
  if (!hasBody(value)) return ''
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

function escapeHtml(str: string): string {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function highlightJson(value: unknown): string {
  const json = escapeHtml(prettyJson(value))
  return json.replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\btrue\b|\bfalse\b|\bnull\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
    (match) => {
      let cls = 'text-cyan-300'
      if (match.startsWith('"')) {
        cls = match.endsWith(':') ? 'text-sky-300' : 'text-emerald-300'
      } else if (match === 'true' || match === 'false') {
        cls = 'text-amber-300'
      } else if (match === 'null') {
        cls = 'text-gray-400'
      }
      return `<span class="${cls}">${match}</span>`
    }
  )
}

async function handleCopy(value: unknown) {
  if (!hasBody(value)) return
  await copyToClipboard(prettyJson(value))
}
</script>
