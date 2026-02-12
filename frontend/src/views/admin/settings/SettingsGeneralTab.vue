<template>
  <!-- eslint-disable vue/no-mutating-props -->
  <div class="space-y-6">
    <!-- Site Settings -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.site.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.site.description') }}
        </p>
      </div>
      <div class="space-y-6 p-6">
        <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.site.siteName') }}
            </label>
            <input
              v-model="form.site_name"
              type="text"
              class="input"
              :placeholder="t('admin.settings.site.siteNamePlaceholder')"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.site.siteNameHint') }}
            </p>
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.site.siteSubtitle') }}
            </label>
            <input
              v-model="form.site_subtitle"
              type="text"
              class="input"
              :placeholder="t('admin.settings.site.siteSubtitlePlaceholder')"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.site.siteSubtitleHint') }}
            </p>
          </div>
        </div>

        <!-- API Base URL -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.apiBaseUrl') }}
          </label>
          <input
            v-model="form.api_base_url"
            type="text"
            class="input font-mono text-sm"
            :placeholder="t('admin.settings.site.apiBaseUrlPlaceholder')"
          />
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.site.apiBaseUrlHint') }}
          </p>
        </div>

        <!-- Contact Info -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.contactInfo') }}
          </label>
          <input
            v-model="form.contact_info"
            type="text"
            class="input"
            :placeholder="t('admin.settings.site.contactInfoPlaceholder')"
          />
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.site.contactInfoHint') }}
          </p>
        </div>

        <!-- Contact QR Codes -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.contactQRCode') }}
          </label>
          <p class="mb-4 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.site.contactQRCodeHint') }}
          </p>
          <div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
            <!-- WeChat QR Code -->
            <div class="flex items-start gap-4">
              <div class="flex-shrink-0">
                <div
                  class="flex h-24 w-24 items-center justify-center overflow-hidden rounded-xl border-2 border-dashed border-gray-300 bg-gray-50 dark:border-dark-600 dark:bg-dark-800"
                  :class="{ 'border-solid': form.contact_qrcode_wechat }"
                >
                  <img
                    v-if="form.contact_qrcode_wechat"
                    :src="form.contact_qrcode_wechat"
                    alt="WeChat QR Code"
                    class="h-full w-full object-contain"
                  />
                  <svg
                    v-else
                    class="h-8 w-8 text-gray-400 dark:text-dark-500"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.5"
                      d="M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h4.01M16 20h4M4 12h4m12 0h.01M5 8h2a1 1 0 001-1V5a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1zm12 0h2a1 1 0 001-1V5a1 1 0 00-1-1h-2a1 1 0 00-1 1v2a1 1 0 001 1zM5 20h2a1 1 0 001-1v-2a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1z"
                    />
                  </svg>
                </div>
              </div>
              <div class="flex-1 space-y-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.site.qrcodeWechat') }}
                </span>
                <div class="flex flex-wrap items-center gap-2">
                  <label class="btn btn-secondary btn-sm cursor-pointer">
                    <input
                      type="file"
                      accept="image/*"
                      class="hidden"
                      @change="handleQRCodeUpload($event, 'wechat')"
                    />
                    <Icon name="upload" size="sm" class="mr-1.5" :stroke-width="2" />
                    {{ t('admin.settings.site.uploadImage') }}
                  </label>
                  <button
                    v-if="form.contact_qrcode_wechat"
                    type="button"
                    @click="form.contact_qrcode_wechat = ''"
                    class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                  >
                    <Icon name="trash" size="sm" class="mr-1.5" :stroke-width="2" />
                    {{ t('admin.settings.site.remove') }}
                  </button>
                </div>
              </div>
            </div>

            <!-- Group QR Code -->
            <div class="flex items-start gap-4">
              <div class="flex-shrink-0">
                <div
                  class="flex h-24 w-24 items-center justify-center overflow-hidden rounded-xl border-2 border-dashed border-gray-300 bg-gray-50 dark:border-dark-600 dark:bg-dark-800"
                  :class="{ 'border-solid': form.contact_qrcode_group }"
                >
                  <img
                    v-if="form.contact_qrcode_group"
                    :src="form.contact_qrcode_group"
                    alt="Group QR Code"
                    class="h-full w-full object-contain"
                  />
                  <svg
                    v-else
                    class="h-8 w-8 text-gray-400 dark:text-dark-500"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="1.5"
                      d="M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h4.01M16 20h4M4 12h4m12 0h.01M5 8h2a1 1 0 001-1V5a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1zm12 0h2a1 1 0 001-1V5a1 1 0 00-1-1h-2a1 1 0 00-1 1v2a1 1 0 001 1zM5 20h2a1 1 0 001-1v-2a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1z"
                    />
                  </svg>
                </div>
              </div>
              <div class="flex-1 space-y-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.site.qrcodeGroup') }}
                </span>
                <div class="flex flex-wrap items-center gap-2">
                  <label class="btn btn-secondary btn-sm cursor-pointer">
                    <input
                      type="file"
                      accept="image/*"
                      class="hidden"
                      @change="handleQRCodeUpload($event, 'group')"
                    />
                    <Icon name="upload" size="sm" class="mr-1.5" :stroke-width="2" />
                    {{ t('admin.settings.site.uploadImage') }}
                  </label>
                  <button
                    v-if="form.contact_qrcode_group"
                    type="button"
                    @click="form.contact_qrcode_group = ''"
                    class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                  >
                    <Icon name="trash" size="sm" class="mr-1.5" :stroke-width="2" />
                    {{ t('admin.settings.site.remove') }}
                  </button>
                </div>
              </div>
            </div>
          </div>
          <p v-if="qrcodeError" class="mt-2 text-xs text-red-500">{{ qrcodeError }}</p>
        </div>

        <!-- Doc URL -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.docUrl') }}
          </label>
          <input
            v-model="form.doc_url"
            type="url"
            class="input font-mono text-sm"
            :placeholder="t('admin.settings.site.docUrlPlaceholder')"
          />
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.site.docUrlHint') }}
          </p>
        </div>

        <!-- Site Logo Upload -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.siteLogo') }}
          </label>
          <div class="flex items-start gap-6">
            <div class="flex-shrink-0">
              <div
                class="flex h-20 w-20 items-center justify-center overflow-hidden rounded-xl border-2 border-dashed border-gray-300 bg-gray-50 dark:border-dark-600 dark:bg-dark-800"
                :class="{ 'border-solid': form.site_logo }"
              >
                <img
                  v-if="form.site_logo"
                  :src="form.site_logo"
                  alt="Site Logo"
                  class="h-full w-full object-contain"
                />
                <svg
                  v-else
                  class="h-8 w-8 text-gray-400 dark:text-dark-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.5"
                    d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
                  />
                </svg>
              </div>
            </div>
            <div class="flex-1 space-y-3">
              <div class="flex items-center gap-3">
                <label class="btn btn-secondary btn-sm cursor-pointer">
                  <input
                    type="file"
                    accept="image/*"
                    class="hidden"
                    @change="(e) => handleLogoUpload(e, 'light')"
                  />
                  <Icon name="upload" size="sm" class="mr-1.5" :stroke-width="2" />
                  {{ t('admin.settings.site.uploadImage') }}
                </label>
                <button
                  v-if="form.site_logo"
                  type="button"
                  @click="form.site_logo = ''"
                  class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                >
                  <Icon name="trash" size="sm" class="mr-1.5" :stroke-width="2" />
                  {{ t('admin.settings.site.remove') }}
                </button>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.site.logoHint') }}
              </p>
              <p v-if="logoError" class="text-xs text-red-500">{{ logoError }}</p>
            </div>
          </div>
        </div>

        <!-- Site Logo Dark Upload -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.siteLogoDark') }}
          </label>
          <div class="flex items-start gap-6">
            <div class="flex-shrink-0">
              <div
                class="flex h-20 w-20 items-center justify-center overflow-hidden rounded-xl border-2 border-dashed border-gray-600 bg-gray-900"
                :class="{ 'border-solid': form.site_logo_dark }"
              >
                <img
                  v-if="form.site_logo_dark"
                  :src="form.site_logo_dark"
                  alt="Site Logo Dark"
                  class="h-full w-full object-contain"
                />
                <svg
                  v-else
                  class="h-8 w-8 text-gray-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.5"
                    d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
                  />
                </svg>
              </div>
            </div>
            <div class="flex-1 space-y-3">
              <div class="flex items-center gap-3">
                <label class="btn btn-secondary btn-sm cursor-pointer">
                  <input
                    type="file"
                    accept="image/*"
                    class="hidden"
                    @change="(e) => handleLogoUpload(e, 'dark')"
                  />
                  <Icon name="upload" size="sm" class="mr-1.5" :stroke-width="2" />
                  {{ t('admin.settings.site.uploadImage') }}
                </label>
                <button
                  v-if="form.site_logo_dark"
                  type="button"
                  @click="form.site_logo_dark = ''"
                  class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                >
                  <Icon name="trash" size="sm" class="mr-1.5" :stroke-width="2" />
                  {{ t('admin.settings.site.remove') }}
                </button>
              </div>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.site.logoDarkHint') }}
              </p>
              <p v-if="logoDarkError" class="text-xs text-red-500">{{ logoDarkError }}</p>
            </div>
          </div>
        </div>

        <!-- Home Content -->
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.site.homeContent') }}
          </label>
          <textarea
            v-model="form.home_content"
            rows="6"
            class="input font-mono text-sm"
            :placeholder="t('admin.settings.site.homeContentPlaceholder')"
          ></textarea>
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.site.homeContentHint') }}
          </p>
          <p class="mt-2 text-xs text-amber-600 dark:text-amber-400">
            {{ t('admin.settings.site.homeContentIframeWarning') }}
          </p>
        </div>

        <!-- Install Guide Videos -->
        <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.installGuideVideos.title') }}
          </label>
          <p class="mb-3 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.installGuideVideos.description') }}
          </p>
          <div class="space-y-3">
            <div v-for="toolKey in ['claude_code', 'codex', 'gemini_cli']" :key="toolKey">
              <details class="rounded-lg border border-gray-200 dark:border-dark-700">
                <summary class="cursor-pointer px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-800">
                  {{ { claude_code: 'Claude Code', codex: 'Codex CLI', gemini_cli: 'Gemini CLI' }[toolKey] }}
                </summary>
                <div class="space-y-2 border-t border-gray-100 px-3 py-3 dark:border-dark-700">
                  <div>
                    <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.settings.installGuideVideos.overview') }}
                    </label>
                    <input
                      type="text"
                      :value="getVideoField(toolKey, 'overview')"
                      @input="setVideoField(toolKey, 'overview', ($event.target as HTMLInputElement).value)"
                      class="input text-sm"
                      :placeholder="t('admin.settings.installGuideVideos.urlPlaceholder')"
                    />
                  </div>
                </div>
              </details>
            </div>
          </div>
        </div>

        <!-- Home Testimonials -->
        <div class="border-t border-gray-100 pt-4 dark:border-dark-700">
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.homeTestimonials.title') }}
          </label>
          <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.homeTestimonials.description') }}
          </p>
          <textarea
            v-model="form.home_testimonials"
            rows="4"
            class="input font-mono text-sm"
            :placeholder="t('admin.settings.homeTestimonials.placeholder')"
          ></textarea>
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.homeTestimonials.hint') }}
          </p>
        </div>

        <!-- Hide CCS Import Button -->
        <div
          class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
        >
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{
              t('admin.settings.site.hideCcsImportButton')
            }}</label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.site.hideCcsImportButtonHint') }}
            </p>
          </div>
          <Toggle v-model="form.hide_ccs_import_button" />
        </div>
      </div>
    </div>

    <!-- Default Settings -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.defaults.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.defaults.description') }}
        </p>
      </div>
      <div class="p-6">
        <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.defaults.defaultBalance') }}
            </label>
            <input
              v-model.number="form.default_balance"
              type="number"
              step="0.01"
              min="0"
              class="input"
              placeholder="0.00"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.defaults.defaultBalanceHint') }}
            </p>
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.defaults.defaultConcurrency') }}
            </label>
            <input
              v-model.number="form.default_concurrency"
              type="number"
              min="1"
              class="input"
              placeholder="1"
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.defaults.defaultConcurrencyHint') }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <!-- Purchase Subscription Page -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.purchase.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.purchase.description') }}
        </p>
      </div>
      <div class="space-y-6 p-6">
        <div class="flex items-center justify-between">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{
              t('admin.settings.purchase.enabled')
            }}</label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.purchase.enabledHint') }}
            </p>
          </div>
          <Toggle v-model="form.purchase_subscription_enabled" />
        </div>
        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.purchase.url') }}
          </label>
          <input
            v-model="form.purchase_subscription_url"
            type="url"
            class="input font-mono text-sm"
            :placeholder="t('admin.settings.purchase.urlPlaceholder')"
          />
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.purchase.urlHint') }}
          </p>
          <p class="mt-2 text-xs text-amber-600 dark:text-amber-400">
            {{ t('admin.settings.purchase.iframeWarning') }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/* eslint-disable vue/no-mutating-props */
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'

const { t } = useI18n()

const props = defineProps<{
  form: any
}>()

const logoError = ref('')
const logoDarkError = ref('')
const qrcodeError = ref('')

function handleLogoUpload(event: Event, type: 'light' | 'dark' = 'light') {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  const errorRef = type === 'light' ? logoError : logoDarkError
  errorRef.value = ''

  if (!file) return

  const maxSize = 300 * 1024
  if (file.size > maxSize) {
    errorRef.value = t('admin.settings.site.logoSizeError', {
      size: (file.size / 1024).toFixed(1)
    })
    input.value = ''
    return
  }

  if (!file.type.startsWith('image/')) {
    errorRef.value = t('admin.settings.site.logoTypeError')
    input.value = ''
    return
  }

  const reader = new FileReader()
  reader.onload = (e) => {
    if (type === 'light') {
      props.form.site_logo = e.target?.result as string
    } else {
      props.form.site_logo_dark = e.target?.result as string
    }
  }
  reader.onerror = () => {
    errorRef.value = t('admin.settings.site.logoReadError')
  }
  reader.readAsDataURL(file)
  input.value = ''
}

function handleQRCodeUpload(event: Event, type: 'wechat' | 'group') {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  qrcodeError.value = ''

  if (!file) return

  const maxSize = 500 * 1024
  if (file.size > maxSize) {
    qrcodeError.value = t('admin.settings.site.qrcodeSizeError', {
      size: (file.size / 1024).toFixed(1)
    })
    input.value = ''
    return
  }

  if (!file.type.startsWith('image/')) {
    qrcodeError.value = t('admin.settings.site.qrcodeTypeError')
    input.value = ''
    return
  }

  const reader = new FileReader()
  reader.onload = (e) => {
    if (type === 'wechat') {
      props.form.contact_qrcode_wechat = e.target?.result as string
    } else {
      props.form.contact_qrcode_group = e.target?.result as string
    }
  }
  reader.onerror = () => {
    qrcodeError.value = t('admin.settings.site.qrcodeReadError')
  }
  reader.readAsDataURL(file)
  input.value = ''
}

function parseVideoConfig(): Record<string, Record<string, string>> {
  try {
    return props.form.install_guide_videos ? JSON.parse(props.form.install_guide_videos) : {}
  } catch {
    return {}
  }
}

function getVideoField(toolKey: string, field: string): string {
  const config = parseVideoConfig()
  return config[toolKey]?.[field] || ''
}

function setVideoField(toolKey: string, field: string, value: string) {
  const config = parseVideoConfig()
  if (!config[toolKey]) config[toolKey] = {}
  config[toolKey][field] = value
  props.form.install_guide_videos = JSON.stringify(config)
}
</script>
