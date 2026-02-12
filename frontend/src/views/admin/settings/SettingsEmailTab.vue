<template>
  <!-- eslint-disable vue/no-mutating-props -->
  <div class="space-y-6">
    <!-- SMTP Settings - Only show when email verification is enabled -->
    <div v-if="form.email_verify_enabled" class="card">
      <div
        class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700"
      >
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.settings.smtp.title') }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.smtp.description') }}
          </p>
        </div>
        <button
          type="button"
          @click="testSmtpConnection"
          :disabled="testingSmtp"
          class="btn btn-secondary btn-sm"
        >
          <svg v-if="testingSmtp" class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
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
          {{
            testingSmtp
              ? t('admin.settings.smtp.testing')
              : t('admin.settings.smtp.testConnection')
          }}
        </button>
      </div>
      <div class="space-y-6 p-6">
        <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.smtp.host') }}
            </label>
            <input
              v-model="form.smtp_host"
              type="text"
              class="input"
              :placeholder="t('admin.settings.smtp.hostPlaceholder')"
            />
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.smtp.port') }}
            </label>
            <input
              v-model.number="form.smtp_port"
              type="number"
              min="1"
              max="65535"
              class="input"
              :placeholder="t('admin.settings.smtp.portPlaceholder')"
            />
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.smtp.username') }}
            </label>
            <input
              v-model="form.smtp_username"
              type="text"
              class="input"
              :placeholder="t('admin.settings.smtp.usernamePlaceholder')"
            />
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.smtp.password') }}
            </label>
            <input
              v-model="form.smtp_password"
              type="password"
              class="input"
              :placeholder="
                form.smtp_password_configured
                  ? t('admin.settings.smtp.passwordConfiguredPlaceholder')
                  : t('admin.settings.smtp.passwordPlaceholder')
              "
            />
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{
                form.smtp_password_configured
                  ? t('admin.settings.smtp.passwordConfiguredHint')
                  : t('admin.settings.smtp.passwordHint')
              }}
            </p>
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.smtp.fromEmail') }}
            </label>
            <input
              v-model="form.smtp_from_email"
              type="email"
              class="input"
              :placeholder="t('admin.settings.smtp.fromEmailPlaceholder')"
            />
          </div>
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.smtp.fromName') }}
            </label>
            <input
              v-model="form.smtp_from_name"
              type="text"
              class="input"
              :placeholder="t('admin.settings.smtp.fromNamePlaceholder')"
            />
          </div>
        </div>

        <!-- Use TLS Toggle -->
        <div
          class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
        >
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{
              t('admin.settings.smtp.useTls')
            }}</label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.smtp.useTlsHint') }}
            </p>
          </div>
          <Toggle v-model="form.smtp_use_tls" />
        </div>
      </div>
    </div>

    <!-- No email verification hint -->
    <div v-if="!form.email_verify_enabled" class="card">
      <div class="p-6">
        <div class="rounded-lg border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-900/20">
          <p class="text-sm text-blue-700 dark:text-blue-300">
            {{ t('admin.settings.smtp.enableEmailVerifyHint') }}
          </p>
        </div>
      </div>
    </div>

    <!-- Send Test Email - Only show when email verification is enabled -->
    <div v-if="form.email_verify_enabled" class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.testEmail.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.testEmail.description') }}
        </p>
      </div>
      <div class="p-6">
        <div class="flex items-end gap-4">
          <div class="flex-1">
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.testEmail.recipientEmail') }}
            </label>
            <input
              v-model="testEmailAddress"
              type="email"
              class="input"
              :placeholder="t('admin.settings.testEmail.recipientEmailPlaceholder')"
            />
          </div>
          <button
            type="button"
            @click="sendTestEmail"
            :disabled="sendingTestEmail || !testEmailAddress"
            class="btn btn-secondary"
          >
            <svg
              v-if="sendingTestEmail"
              class="h-4 w-4 animate-spin"
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
            {{
              sendingTestEmail
                ? t('admin.settings.testEmail.sending')
                : t('admin.settings.testEmail.sendTestEmail')
            }}
          </button>
        </div>
      </div>
    </div>

    <!-- Usage Report Settings -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.usageReport.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.usageReport.description') }}
        </p>
      </div>
      <div class="space-y-4 p-6">
        <div class="flex items-center justify-between">
          <div>
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.usageReport.enabled') }}
            </label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.usageReport.enabledHint') }}
            </p>
          </div>
          <Toggle v-model="form.usage_report_global_enabled" />
        </div>

        <div v-if="form.usage_report_global_enabled" class="space-y-4 pl-0 sm:pl-4">
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.usageReport.targetScope') }}
            </label>
            <select v-model="form.usage_report_target_scope" class="input w-full sm:w-80">
              <option value="opted_in">{{ t('admin.settings.usageReport.scopeOptedIn') }}</option>
              <option value="active_today">{{ t('admin.settings.usageReport.scopeActiveToday') }}</option>
              <option value="all">{{ t('admin.settings.usageReport.scopeAll') }}</option>
            </select>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.usageReport.targetScopeHint') }}
            </p>
          </div>

          <div v-if="form.usage_report_target_scope !== 'opted_in'">
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.usageReport.globalSchedule') }}
            </label>
            <input
              v-model="form.usage_report_global_schedule"
              type="time"
              class="input w-40"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.usageReport.globalScheduleHint') }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <!-- Account Expiry Reminder Settings -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.accountExpiryReminder.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.accountExpiryReminder.description') }}
        </p>
      </div>
      <div class="space-y-4 p-6">
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.accountExpiryReminder.email') }}
          </label>
          <input
            v-model="form.account_expiry_reminder_email"
            type="email"
            class="input w-full sm:w-80"
            placeholder="admin@example.com"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.accountExpiryReminder.emailHint') }}
          </p>
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.accountExpiryReminder.advanceDays') }}
          </label>
          <input
            v-model.number="form.account_expiry_reminder_advance_days"
            type="number"
            min="1"
            max="30"
            class="input w-40"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.accountExpiryReminder.advanceDaysHint') }}
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
import { adminAPI } from '@/api'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores'

const props = defineProps<{
  form: any
}>()

const { t } = useI18n()
const appStore = useAppStore()

const testingSmtp = ref(false)
const sendingTestEmail = ref(false)
const testEmailAddress = ref('')

async function testSmtpConnection() {
  testingSmtp.value = true
  try {
    const result = await adminAPI.settings.testSmtpConnection({
      smtp_host: props.form.smtp_host,
      smtp_port: props.form.smtp_port,
      smtp_username: props.form.smtp_username,
      smtp_password: props.form.smtp_password,
      smtp_use_tls: props.form.smtp_use_tls
    })
    appStore.showSuccess(result.message || t('admin.settings.smtpConnectionSuccess'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.failedToTestSmtp') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    testingSmtp.value = false
  }
}

async function sendTestEmail() {
  if (!testEmailAddress.value) {
    appStore.showError(t('admin.settings.testEmail.enterRecipientHint'))
    return
  }

  sendingTestEmail.value = true
  try {
    const result = await adminAPI.settings.sendTestEmail({
      email: testEmailAddress.value,
      smtp_host: props.form.smtp_host,
      smtp_port: props.form.smtp_port,
      smtp_username: props.form.smtp_username,
      smtp_password: props.form.smtp_password,
      smtp_from_email: props.form.smtp_from_email,
      smtp_from_name: props.form.smtp_from_name,
      smtp_use_tls: props.form.smtp_use_tls
    })
    appStore.showSuccess(result.message || t('admin.settings.testEmailSent'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.failedToSendTestEmail') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    sendingTestEmail.value = false
  }
}
</script>
