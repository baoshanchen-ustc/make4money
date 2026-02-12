<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <template v-else>
        <!-- Tab Navigation -->
        <div class="border-b border-gray-200 dark:border-dark-700">
          <nav class="-mb-px flex space-x-6 overflow-x-auto px-1" aria-label="Settings tabs">
            <button
              v-for="tab in tabs"
              :key="tab.key"
              type="button"
              @click="switchTab(tab.key)"
              :class="[
                'whitespace-nowrap border-b-2 px-1 py-3 text-sm font-medium transition-colors',
                activeTab === tab.key
                  ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:border-dark-500 dark:hover:text-gray-300'
              ]"
            >
              {{ tab.label }}
            </button>
          </nav>
        </div>

        <!-- Tab Content -->
        <form @submit.prevent="saveSettings" class="space-y-6">
          <SettingsGeneralTab v-if="activeTab === 'general'" :form="form" />
          <SettingsAuthTab v-else-if="activeTab === 'auth'" :form="form" />
          <SettingsPaymentTab v-else-if="activeTab === 'payment'" :form="form" />
          <SettingsEmailTab v-else-if="activeTab === 'email'" :form="form" />
          <SettingsAdvancedTab v-else-if="activeTab === 'advanced'" />

          <!-- Global Save Button -->
          <div
            v-if="activeTab !== 'advanced'"
            class="sticky bottom-4 z-10 flex justify-end"
          >
            <button
              type="submit"
              :disabled="saving"
              class="btn btn-primary shadow-lg"
            >
              <svg
                v-if="saving"
                class="mr-2 h-4 w-4 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                ></circle>
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                ></path>
              </svg>
              {{ saving ? t('common.saving') : t('common.save') }}
            </button>
          </div>
        </form>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { adminAPI } from '@/api'
import type { SystemSettings, UpdateSettingsRequest } from '@/api/admin/settings'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores'
import SettingsGeneralTab from './settings/SettingsGeneralTab.vue'
import SettingsAuthTab from './settings/SettingsAuthTab.vue'
import SettingsPaymentTab from './settings/SettingsPaymentTab.vue'
import SettingsEmailTab from './settings/SettingsEmailTab.vue'
import SettingsAdvancedTab from './settings/SettingsAdvancedTab.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(true)
const saving = ref(false)

// Tab definitions
const validTabs = ['general', 'auth', 'payment', 'email', 'advanced'] as const
type TabKey = (typeof validTabs)[number]

const tabs = computed(() => [
  { key: 'general' as TabKey, label: t('admin.settings.tabs.general') },
  { key: 'auth' as TabKey, label: t('admin.settings.tabs.auth') },
  { key: 'payment' as TabKey, label: t('admin.settings.tabs.payment') },
  { key: 'email' as TabKey, label: t('admin.settings.tabs.email') },
  { key: 'advanced' as TabKey, label: t('admin.settings.tabs.advanced') }
])

const activeTab = ref<TabKey>((validTabs.includes(route.query.tab as TabKey) ? route.query.tab : 'general') as TabKey)

function switchTab(tab: TabKey) {
  activeTab.value = tab
  router.replace({ query: { ...route.query, tab } })
}

// Shared form state
type SettingsForm = SystemSettings & {
  smtp_password: string
  turnstile_secret_key: string
  linuxdo_connect_client_secret: string
  wechat_server_token: string
  wechat_app_secret: string
}

const form = reactive<SettingsForm>({
  registration_enabled: true,
  email_verify_enabled: false,
  promo_code_enabled: true,
  invitation_code_enabled: false,
  password_reset_enabled: false,
  totp_enabled: false,
  totp_encryption_key_configured: false,
  default_balance: 0,
  default_concurrency: 1,
  site_name: 'Code80',
  site_logo: '',
  site_logo_dark: '',
  site_subtitle: 'Subscription to API Conversion Platform',
  api_base_url: '',
  contact_info: '',
  contact_qrcode_wechat: '',
  contact_qrcode_group: '',
  doc_url: '',
  home_content: '',
  install_guide_videos: '',
  home_testimonials: '',
  hide_ccs_import_button: false,
  purchase_subscription_enabled: false,
  purchase_subscription_url: '',
  smtp_host: '',
  smtp_port: 587,
  smtp_username: '',
  smtp_password: '',
  smtp_password_configured: false,
  smtp_from_email: '',
  smtp_from_name: '',
  smtp_use_tls: true,
  turnstile_enabled: false,
  turnstile_site_key: '',
  turnstile_secret_key: '',
  turnstile_secret_key_configured: false,
  linuxdo_connect_enabled: false,
  linuxdo_connect_client_id: '',
  linuxdo_connect_client_secret: '',
  linuxdo_connect_client_secret_configured: false,
  linuxdo_connect_redirect_url: '',
  force_email_bind: false,
  wechat_auth_enabled: false,
  wechat_account_type: 'subscription',
  wechat_server_address: '',
  wechat_server_token: '',
  wechat_server_token_configured: false,
  wechat_account_qrcode_url: '',
  wechat_account_qrcode_data: '',
  wechat_app_id: '',
  wechat_app_secret: '',
  wechat_app_secret_configured: false,
  enable_model_fallback: false,
  fallback_model_anthropic: 'claude-3-5-sonnet-20241022',
  fallback_model_openai: 'gpt-4o',
  fallback_model_gemini: 'gemini-2.5-pro',
  fallback_model_antigravity: 'gemini-2.5-pro',
  enable_identity_patch: true,
  identity_patch_prompt: '',
  ops_monitoring_enabled: true,
  ops_realtime_monitoring_enabled: true,
  ops_query_mode_default: 'auto',
  ops_metrics_interval_seconds: 60,
  usage_report_global_enabled: false,
  usage_report_target_scope: 'opted_in',
  usage_report_global_schedule: '09:00',
  account_expiry_reminder_email: '',
  account_expiry_reminder_advance_days: 7,
  balance_lot_expiry_days: 30,
  balance_expiry_reminder_enabled: false,
  balance_expiry_reminder_advance_days: 3
})

async function loadSettings() {
  loading.value = true
  try {
    const settings = await adminAPI.settings.getSettings()
    Object.assign(form, settings)
    form.smtp_password = ''
    form.turnstile_secret_key = ''
    form.linuxdo_connect_client_secret = ''
    form.wechat_server_token = ''
    form.wechat_app_secret = ''
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.failedToLoad') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  saving.value = true
  try {
    const payload: UpdateSettingsRequest = {
      registration_enabled: form.registration_enabled,
      email_verify_enabled: form.email_verify_enabled,
      promo_code_enabled: form.promo_code_enabled,
      invitation_code_enabled: form.invitation_code_enabled,
      password_reset_enabled: form.password_reset_enabled,
      totp_enabled: form.totp_enabled,
      default_balance: form.default_balance,
      default_concurrency: form.default_concurrency,
      site_name: form.site_name,
      site_logo: form.site_logo,
      site_logo_dark: form.site_logo_dark,
      site_subtitle: form.site_subtitle,
      api_base_url: form.api_base_url,
      contact_info: form.contact_info,
      contact_qrcode_wechat: form.contact_qrcode_wechat,
      contact_qrcode_group: form.contact_qrcode_group,
      doc_url: form.doc_url,
      home_content: form.home_content,
      install_guide_videos: form.install_guide_videos,
      home_testimonials: form.home_testimonials,
      hide_ccs_import_button: form.hide_ccs_import_button,
      purchase_subscription_enabled: form.purchase_subscription_enabled,
      purchase_subscription_url: form.purchase_subscription_url,
      smtp_host: form.smtp_host,
      smtp_port: form.smtp_port,
      smtp_username: form.smtp_username,
      smtp_password: form.smtp_password || undefined,
      smtp_from_email: form.smtp_from_email,
      smtp_from_name: form.smtp_from_name,
      smtp_use_tls: form.smtp_use_tls,
      turnstile_enabled: form.turnstile_enabled,
      turnstile_site_key: form.turnstile_site_key,
      turnstile_secret_key: form.turnstile_secret_key || undefined,
      linuxdo_connect_enabled: form.linuxdo_connect_enabled,
      linuxdo_connect_client_id: form.linuxdo_connect_client_id,
      linuxdo_connect_client_secret: form.linuxdo_connect_client_secret || undefined,
      linuxdo_connect_redirect_url: form.linuxdo_connect_redirect_url,
      force_email_bind: form.force_email_bind,
      wechat_auth_enabled: form.wechat_auth_enabled,
      wechat_account_type: form.wechat_account_type,
      wechat_server_address: form.wechat_server_address,
      wechat_server_token: form.wechat_server_token || undefined,
      wechat_account_qrcode_url: form.wechat_account_qrcode_url,
      wechat_account_qrcode_data: form.wechat_account_qrcode_data,
      wechat_app_id: form.wechat_app_id,
      wechat_app_secret: form.wechat_app_secret || undefined,
      enable_model_fallback: form.enable_model_fallback,
      fallback_model_anthropic: form.fallback_model_anthropic,
      fallback_model_openai: form.fallback_model_openai,
      fallback_model_gemini: form.fallback_model_gemini,
      fallback_model_antigravity: form.fallback_model_antigravity,
      enable_identity_patch: form.enable_identity_patch,
      identity_patch_prompt: form.identity_patch_prompt,
      usage_report_global_enabled: form.usage_report_global_enabled,
      usage_report_target_scope: form.usage_report_target_scope,
      usage_report_global_schedule: form.usage_report_global_schedule,
      account_expiry_reminder_email: form.account_expiry_reminder_email,
      account_expiry_reminder_advance_days: form.account_expiry_reminder_advance_days,
      balance_lot_expiry_days: form.balance_lot_expiry_days,
      balance_expiry_reminder_enabled: form.balance_expiry_reminder_enabled,
      balance_expiry_reminder_advance_days: form.balance_expiry_reminder_advance_days
    }
    const updated = await adminAPI.settings.updateSettings(payload)
    Object.assign(form, updated)
    form.smtp_password = ''
    form.turnstile_secret_key = ''
    form.linuxdo_connect_client_secret = ''
    form.wechat_server_token = ''
    form.wechat_app_secret = ''
    await appStore.fetchPublicSettings(true)
    appStore.showSuccess(t('admin.settings.settingsSaved'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.failedToSave') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadSettings()
})
</script>
