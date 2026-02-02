<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <!-- Settings Form -->
      <form v-else @submit.prevent="saveSettings" class="space-y-6">
        <!-- Admin API Key Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.adminApiKey.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.adminApiKey.description') }}
            </p>
          </div>
          <div class="space-y-4 p-6">
            <!-- Security Warning -->
            <div
              class="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-900/20"
            >
              <div class="flex items-start">
                <Icon
                  name="exclamationTriangle"
                  size="md"
                  class="mt-0.5 flex-shrink-0 text-amber-500"
                />
                <p class="ml-3 text-sm text-amber-700 dark:text-amber-300">
                  {{ t('admin.settings.adminApiKey.securityWarning') }}
                </p>
              </div>
            </div>

            <!-- Loading State -->
            <div v-if="adminApiKeyLoading" class="flex items-center gap-2 text-gray-500">
              <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
              {{ t('common.loading') }}
            </div>

            <!-- No Key Configured -->
            <div v-else-if="!adminApiKeyExists" class="flex items-center justify-between">
              <span class="text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.adminApiKey.notConfigured') }}
              </span>
              <button
                type="button"
                @click="createAdminApiKey"
                :disabled="adminApiKeyOperating"
                class="btn btn-primary btn-sm"
              >
                <svg
                  v-if="adminApiKeyOperating"
                  class="mr-1 h-4 w-4 animate-spin"
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
                  adminApiKeyOperating
                    ? t('admin.settings.adminApiKey.creating')
                    : t('admin.settings.adminApiKey.create')
                }}
              </button>
            </div>

            <!-- Key Exists -->
            <div v-else class="space-y-4">
              <div class="flex items-center justify-between">
                <div>
                  <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.adminApiKey.currentKey') }}
                  </label>
                  <code
                    class="rounded bg-gray-100 px-2 py-1 font-mono text-sm text-gray-900 dark:bg-dark-700 dark:text-gray-100"
                  >
                    {{ adminApiKeyMasked }}
                  </code>
                </div>
                <div class="flex gap-2">
                  <button
                    type="button"
                    @click="regenerateAdminApiKey"
                    :disabled="adminApiKeyOperating"
                    class="btn btn-secondary btn-sm"
                  >
                    {{
                      adminApiKeyOperating
                        ? t('admin.settings.adminApiKey.regenerating')
                        : t('admin.settings.adminApiKey.regenerate')
                    }}
                  </button>
                  <button
                    type="button"
                    @click="deleteAdminApiKey"
                    :disabled="adminApiKeyOperating"
                    class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                  >
                    {{ t('admin.settings.adminApiKey.delete') }}
                  </button>
                </div>
              </div>

              <!-- Newly Generated Key Display -->
              <div
                v-if="newAdminApiKey"
                class="space-y-3 rounded-lg border border-green-200 bg-green-50 p-4 dark:border-green-800 dark:bg-green-900/20"
              >
                <p class="text-sm font-medium text-green-700 dark:text-green-300">
                  {{ t('admin.settings.adminApiKey.keyWarning') }}
                </p>
                <div class="flex items-center gap-2">
                  <code
                    class="flex-1 select-all break-all rounded border border-green-300 bg-white px-3 py-2 font-mono text-sm dark:border-green-700 dark:bg-dark-800"
                  >
                    {{ newAdminApiKey }}
                  </code>
                  <button
                    type="button"
                    @click="copyNewKey"
                    class="btn btn-primary btn-sm flex-shrink-0"
                  >
                    {{ t('admin.settings.adminApiKey.copyKey') }}
                  </button>
                </div>
                <p class="text-xs text-green-600 dark:text-green-400">
                  {{ t('admin.settings.adminApiKey.usage') }}
                </p>
              </div>
            </div>
          </div>
        </div>

        <!-- Stream Timeout Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.streamTimeout.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.streamTimeout.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <!-- Loading State -->
            <div v-if="streamTimeoutLoading" class="flex items-center gap-2 text-gray-500">
              <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
              {{ t('common.loading') }}
            </div>

            <template v-else>
              <!-- Enable Stream Timeout -->
              <div class="flex items-center justify-between">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">{{
                    t('admin.settings.streamTimeout.enabled')
                  }}</label>
                  <p class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.streamTimeout.enabledHint') }}
                  </p>
                </div>
                <Toggle v-model="streamTimeoutForm.enabled" />
              </div>

              <!-- Settings - Only show when enabled -->
              <div
                v-if="streamTimeoutForm.enabled"
                class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700"
              >
                <!-- Action -->
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.streamTimeout.action') }}
                  </label>
                  <select v-model="streamTimeoutForm.action" class="input w-64">
                    <option value="temp_unsched">{{ t('admin.settings.streamTimeout.actionTempUnsched') }}</option>
                    <option value="error">{{ t('admin.settings.streamTimeout.actionError') }}</option>
                    <option value="none">{{ t('admin.settings.streamTimeout.actionNone') }}</option>
                  </select>
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.streamTimeout.actionHint') }}
                  </p>
                </div>

                <!-- Temp Unsched Minutes (only show when action is temp_unsched) -->
                <div v-if="streamTimeoutForm.action === 'temp_unsched'">
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.streamTimeout.tempUnschedMinutes') }}
                  </label>
                  <input
                    v-model.number="streamTimeoutForm.temp_unsched_minutes"
                    type="number"
                    min="1"
                    max="60"
                    class="input w-32"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.streamTimeout.tempUnschedMinutesHint') }}
                  </p>
                </div>

                <!-- Threshold Count -->
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.streamTimeout.thresholdCount') }}
                  </label>
                  <input
                    v-model.number="streamTimeoutForm.threshold_count"
                    type="number"
                    min="1"
                    max="10"
                    class="input w-32"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.streamTimeout.thresholdCountHint') }}
                  </p>
                </div>

                <!-- Threshold Window Minutes -->
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.streamTimeout.thresholdWindowMinutes') }}
                  </label>
                  <input
                    v-model.number="streamTimeoutForm.threshold_window_minutes"
                    type="number"
                    min="1"
                    max="60"
                    class="input w-32"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.streamTimeout.thresholdWindowMinutesHint') }}
                  </p>
                </div>
              </div>

              <!-- Save Button -->
              <div class="flex justify-end border-t border-gray-100 pt-4 dark:border-dark-700">
                <button
                  type="button"
                  @click="saveStreamTimeoutSettings"
                  :disabled="streamTimeoutSaving"
                  class="btn btn-primary btn-sm"
                >
                  <svg
                    v-if="streamTimeoutSaving"
                    class="mr-1 h-4 w-4 animate-spin"
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
                  {{ streamTimeoutSaving ? t('common.saving') : t('common.save') }}
                </button>
              </div>
            </template>
          </div>
        </div>

        <!-- Registration Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.registration.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.registration.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <!-- Enable Registration -->
            <div class="flex items-center justify-between">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.registration.enableRegistration')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.registration.enableRegistrationHint') }}
                </p>
              </div>
              <Toggle v-model="form.registration_enabled" />
            </div>

            <!-- Email Verification -->
            <div
              class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.registration.emailVerification')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.registration.emailVerificationHint') }}
                </p>
              </div>
              <Toggle v-model="form.email_verify_enabled" />
            </div>

            <!-- Promo Code -->
            <div
              class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.registration.promoCode')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.registration.promoCodeHint') }}
                </p>
              </div>
              <Toggle v-model="form.promo_code_enabled" />
            </div>

            <!-- Password Reset - Only show when email verification is enabled -->
            <div
              v-if="form.email_verify_enabled"
              class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.registration.passwordReset')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.registration.passwordResetHint') }}
                </p>
              </div>
              <Toggle v-model="form.password_reset_enabled" />
            </div>

            <!-- TOTP 2FA -->
            <div
              class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.registration.totp')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.registration.totpHint') }}
                </p>
                <!-- Warning when encryption key not configured -->
                <p
                  v-if="!form.totp_encryption_key_configured"
                  class="mt-2 text-sm text-amber-600 dark:text-amber-400"
                >
                  {{ t('admin.settings.registration.totpKeyNotConfigured') }}
                </p>
              </div>
              <Toggle
                v-model="form.totp_enabled"
                :disabled="!form.totp_encryption_key_configured"
              />
            </div>
          </div>
        </div>

        <!-- Cloudflare Turnstile Settings -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.turnstile.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.turnstile.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <!-- Enable Turnstile -->
            <div class="flex items-center justify-between">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.turnstile.enableTurnstile')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.turnstile.enableTurnstileHint') }}
                </p>
              </div>
              <Toggle v-model="form.turnstile_enabled" />
            </div>

            <!-- Turnstile Keys - Only show when enabled -->
            <div
              v-if="form.turnstile_enabled"
              class="border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div class="grid grid-cols-1 gap-6">
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.turnstile.siteKey') }}
                  </label>
                  <input
                    v-model="form.turnstile_site_key"
                    type="text"
                    class="input font-mono text-sm"
                    placeholder="0x4AAAAAAA..."
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.turnstile.siteKeyHint') }}
                    <a
                      href="https://dash.cloudflare.com/"
                      target="_blank"
                      class="text-primary-600 hover:text-primary-500"
                      >{{ t('admin.settings.turnstile.cloudflareDashboard') }}</a
                    >
                  </p>
                </div>
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.turnstile.secretKey') }}
                  </label>
                  <input
                    v-model="form.turnstile_secret_key"
                    type="password"
                    class="input font-mono text-sm"
                    placeholder="0x4AAAAAAA..."
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      form.turnstile_secret_key_configured
                        ? t('admin.settings.turnstile.secretKeyConfiguredHint')
                        : t('admin.settings.turnstile.secretKeyHint')
                    }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- LinuxDo Connect OAuth 登录 -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.linuxdo.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.linuxdo.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <div class="flex items-center justify-between">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.linuxdo.enable')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.linuxdo.enableHint') }}
                </p>
              </div>
              <Toggle v-model="form.linuxdo_connect_enabled" />
            </div>

            <div
              v-if="form.linuxdo_connect_enabled"
              class="border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div class="grid grid-cols-1 gap-6">
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.linuxdo.clientId') }}
                  </label>
                  <input
                    v-model="form.linuxdo_connect_client_id"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.settings.linuxdo.clientIdPlaceholder')"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.linuxdo.clientIdHint') }}
                  </p>
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.linuxdo.clientSecret') }}
                  </label>
                  <input
                    v-model="form.linuxdo_connect_client_secret"
                    type="password"
                    class="input font-mono text-sm"
                    :placeholder="
                      form.linuxdo_connect_client_secret_configured
                        ? t('admin.settings.linuxdo.clientSecretConfiguredPlaceholder')
                        : t('admin.settings.linuxdo.clientSecretPlaceholder')
                    "
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      form.linuxdo_connect_client_secret_configured
                        ? t('admin.settings.linuxdo.clientSecretConfiguredHint')
                        : t('admin.settings.linuxdo.clientSecretHint')
                    }}
                  </p>
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.linuxdo.redirectUrl') }}
                  </label>
                  <input
                    v-model="form.linuxdo_connect_redirect_url"
                    type="url"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.settings.linuxdo.redirectUrlPlaceholder')"
                  />
                  <div class="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm w-fit"
                      @click="setAndCopyLinuxdoRedirectUrl"
                    >
                      {{ t('admin.settings.linuxdo.quickSetCopy') }}
                    </button>
                    <code
                      v-if="linuxdoRedirectUrlSuggestion"
                      class="select-all break-all rounded bg-gray-50 px-2 py-1 font-mono text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300"
                    >
                      {{ linuxdoRedirectUrlSuggestion }}
                    </code>
                  </div>
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.linuxdo.redirectUrlHint') }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 微信公众号验证码登录 -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.wechat.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.wechat.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <!-- 配置说明提示框 -->
            <div class="rounded-lg border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-900/20">
              <div class="flex">
                <svg class="h-5 w-5 flex-shrink-0 text-blue-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd" />
                </svg>
                <div class="ml-3">
                  <h3 class="text-sm font-medium text-blue-800 dark:text-blue-200">
                    {{ t('admin.settings.wechat.configTip') }}
                  </h3>
                  <div class="mt-2 text-sm text-blue-700 dark:text-blue-300">
                    <ul class="list-inside list-disc space-y-1">
                      <li>{{ t('admin.settings.wechat.configTipService') }}</li>
                      <li>{{ t('admin.settings.wechat.configTipSubscription') }}</li>
                    </ul>
                  </div>
                </div>
              </div>
            </div>

            <div class="flex items-center justify-between">
              <div>
                <label class="font-medium text-gray-900 dark:text-white">{{
                  t('admin.settings.wechat.enable')
                }}</label>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.wechat.enableHint') }}
                </p>
              </div>
              <Toggle v-model="form.wechat_auth_enabled" />
            </div>

            <div
              v-if="form.wechat_auth_enabled"
              class="border-t border-gray-100 pt-4 dark:border-dark-700"
            >
              <div class="grid grid-cols-1 gap-6">
                <!-- 公众号类型选择 -->
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.wechat.accountType') }}
                  </label>
                  <div class="flex gap-6">
                    <label class="flex items-center gap-2 cursor-pointer">
                      <input
                        v-model="form.wechat_account_type"
                        type="radio"
                        value="subscription"
                        class="h-4 w-4 border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600"
                      />
                      <span class="text-sm text-gray-700 dark:text-gray-300">
                        {{ t('admin.settings.wechat.accountTypeSubscription') }}
                      </span>
                    </label>
                    <label class="flex items-center gap-2 cursor-pointer">
                      <input
                        v-model="form.wechat_account_type"
                        type="radio"
                        value="unverified_official"
                        class="h-4 w-4 border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600"
                      />
                      <span class="text-sm text-gray-700 dark:text-gray-300">
                        {{ t('admin.settings.wechat.accountTypeUnverifiedOfficial') }}
                      </span>
                    </label>
                  </div>
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.wechat.accountTypeHint') }}
                  </p>
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.wechat.serverAddress') }}
                  </label>
                  <input
                    v-model="form.wechat_server_address"
                    type="url"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.settings.wechat.serverAddressPlaceholder')"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.wechat.serverAddressHint') }}
                  </p>
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.wechat.serverToken') }}
                  </label>
                  <input
                    v-model="form.wechat_server_token"
                    type="password"
                    class="input font-mono text-sm"
                    :placeholder="
                      form.wechat_server_token_configured
                        ? t('admin.settings.wechat.serverTokenConfiguredPlaceholder')
                        : t('admin.settings.wechat.serverTokenPlaceholder')
                    "
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      form.wechat_server_token_configured
                        ? t('admin.settings.wechat.serverTokenConfiguredHint')
                        : t('admin.settings.wechat.serverTokenHint')
                    }}
                  </p>
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.wechat.appId') }}
                  </label>
                  <input
                    v-model="form.wechat_app_id"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.settings.wechat.appIdPlaceholder')"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.wechat.appIdHint') }}
                  </p>
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.wechat.appSecret') }}
                  </label>
                  <input
                    v-model="form.wechat_app_secret"
                    type="password"
                    class="input font-mono text-sm"
                    :placeholder="
                      form.wechat_app_secret_configured
                        ? t('admin.settings.wechat.appSecretConfiguredPlaceholder')
                        : t('admin.settings.wechat.appSecretPlaceholder')
                    "
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{
                      form.wechat_app_secret_configured
                        ? t('admin.settings.wechat.appSecretConfiguredHint')
                        : t('admin.settings.wechat.appSecretHint')
                    }}
                  </p>
                </div>

                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.wechat.qrcodeUrl') }}
                  </label>
                  <input
                    v-model="form.wechat_account_qrcode_url"
                    type="url"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.settings.wechat.qrcodeUrlPlaceholder')"
                  />
                  <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.wechat.qrcodeUrlHint') }}
                  </p>
                  <!-- Generate QR Code Button -->
                  <div class="mt-3">
                    <button
                      type="button"
                      class="btn btn-secondary"
                      :disabled="generatingQRCode || !form.wechat_app_id || (!form.wechat_app_secret && !form.wechat_app_secret_configured)"
                      @click="generateWeChatQRCode"
                    >
                      <span v-if="generatingQRCode" class="mr-2">
                        <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24">
                          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none" />
                          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                        </svg>
                      </span>
                      {{ t('admin.settings.wechat.generateQRCode') }}
                    </button>
                    <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.settings.wechat.generateQRCodeHint') }}
                    </p>
                  </div>

                  <!-- 手动上传分隔线 -->
                  <div class="relative mt-6">
                    <div class="absolute inset-0 flex items-center">
                      <div class="w-full border-t border-gray-200 dark:border-dark-600"></div>
                    </div>
                    <div class="relative flex justify-center">
                      <span class="bg-white px-3 text-sm text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                        {{ t('admin.settings.wechat.orManualUpload') }}
                      </span>
                    </div>
                  </div>

                  <!-- 手动上传二维码 -->
                  <div class="mt-6">
                    <div class="flex items-start gap-6">
                      <!-- 上传预览 -->
                      <div class="flex-shrink-0">
                        <div
                          class="flex h-24 w-24 items-center justify-center overflow-hidden rounded-lg border-2 border-dashed border-gray-300 bg-gray-50 dark:border-dark-600 dark:bg-dark-700"
                          :class="{ 'border-solid border-green-500': form.wechat_account_qrcode_data }"
                        >
                          <img
                            v-if="form.wechat_account_qrcode_data"
                            :src="form.wechat_account_qrcode_data"
                            alt="Uploaded QR Code"
                            class="h-full w-full object-contain"
                          />
                          <svg
                            v-else
                            class="h-8 w-8 text-gray-400"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                          </svg>
                        </div>
                      </div>
                      <!-- 上传控件 -->
                      <div class="flex-1 space-y-3">
                        <div class="flex items-center gap-3">
                          <label class="btn btn-secondary btn-sm cursor-pointer">
                            <input
                              type="file"
                              accept="image/*"
                              class="hidden"
                              @change="handleWeChatAccountQRCodeUpload"
                            />
                            <Icon name="upload" size="sm" class="mr-1.5" :stroke-width="2" />
                            {{ t('admin.settings.wechat.uploadQRCode') }}
                          </label>
                          <button
                            v-if="form.wechat_account_qrcode_data"
                            type="button"
                            @click="form.wechat_account_qrcode_data = ''"
                            class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400"
                          >
                            <Icon name="trash" size="sm" class="mr-1.5" :stroke-width="2" />
                            {{ t('admin.settings.site.remove') }}
                          </button>
                        </div>
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                          {{ t('admin.settings.wechat.uploadQRCodeHint') }}
                        </p>
                        <p v-if="wechatAccountQRCodeError" class="text-xs text-red-500">{{ wechatAccountQRCodeError }}</p>
                      </div>
                    </div>
                  </div>

                  <!-- 最终效果预览 -->
                  <div v-if="effectiveWeChatQRCode" class="mt-6 rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-700">
                    <div class="flex items-start gap-4">
                      <div class="rounded-lg border border-gray-200 bg-white p-2 dark:border-dark-600 dark:bg-dark-800">
                        <img
                          :src="effectiveWeChatQRCode"
                          :alt="t('admin.settings.wechat.qrcodePreviewAlt')"
                          class="h-32 w-32 object-contain"
                          @error="(e: Event) => (e.target as HTMLImageElement).style.display = 'none'"
                        />
                      </div>
                      <div>
                        <p class="text-sm font-medium text-gray-700 dark:text-gray-300">
                          {{ t('admin.settings.wechat.finalPreview') }}
                        </p>
                        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                          {{ t('admin.settings.wechat.source') }}:
                          <span class="ml-1 font-medium" :class="wechatQRCodeSource === 'uploaded' ? 'text-green-600 dark:text-green-400' : 'text-blue-600 dark:text-blue-400'">
                            {{ wechatQRCodeSource === 'uploaded' ? t('admin.settings.wechat.sourceUpload') : t('admin.settings.wechat.sourceGenerated') }}
                          </span>
                        </p>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 微信支付配置（只读） -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.wechatPay.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.wechatPay.description') }}
            </p>
          </div>
          <div class="space-y-4 p-6">
            <!-- 加载状态 -->
            <div v-if="wechatPayLoading" class="flex items-center gap-2 text-gray-500">
              <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
              {{ t('common.loading') }}
            </div>

            <!-- 配置内容 -->
            <template v-else>
              <!-- 启用状态 -->
              <div class="flex items-center justify-between">
                <span class="text-sm text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.wechatPay.status') }}
                </span>
                <span
                  :class="[
                    'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                    wechatPayStatus?.enabled
                      ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                      : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
                  ]"
                >
                  {{ wechatPayStatus?.enabled
                    ? t('admin.settings.wechatPay.enabled')
                    : t('admin.settings.wechatPay.disabled')
                  }}
                </span>
              </div>

              <!-- 配置状态 -->
              <div class="flex items-center justify-between">
                <span class="text-sm text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.wechatPay.configStatus') }}
                </span>
                <span
                  :class="[
                    'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                    wechatPayStatus?.configured
                      ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
                      : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
                  ]"
                >
                  {{ wechatPayStatus?.configured
                    ? t('admin.settings.wechatPay.configured')
                    : t('admin.settings.wechatPay.notConfigured')
                  }}
                </span>
              </div>

              <!-- AppID（脱敏） -->
              <div v-if="wechatPayStatus?.app_id_masked" class="flex items-center justify-between">
                <span class="text-sm text-gray-700 dark:text-gray-300">AppID</span>
                <code class="text-sm font-mono text-gray-600 dark:text-gray-400">
                  {{ wechatPayStatus.app_id_masked }}
                </code>
              </div>

              <!-- 商户号（脱敏） -->
              <div v-if="wechatPayStatus?.mch_id_masked" class="flex items-center justify-between">
                <span class="text-sm text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.wechatPay.mchId') }}
                </span>
                <code class="text-sm font-mono text-gray-600 dark:text-gray-400">
                  {{ wechatPayStatus.mch_id_masked }}
                </code>
              </div>

              <!-- 回调地址 -->
              <div v-if="wechatPayStatus?.notify_url" class="flex items-center justify-between">
                <span class="text-sm text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.wechatPay.notifyUrl') }}
                </span>
                <code class="text-sm font-mono text-gray-600 dark:text-gray-400 truncate max-w-xs" :title="wechatPayStatus.notify_url">
                  {{ wechatPayStatus.notify_url }}
                </code>
              </div>

              <!-- 未配置提示 -->
              <div v-if="!wechatPayStatus?.configured" class="mt-4 p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
                <p class="text-sm text-yellow-800 dark:text-yellow-200">
                  {{ t('admin.settings.wechatPay.configHint') }}
                </p>
              </div>
            </template>
          </div>
        </div>

        <!-- 充值业务配置 -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.recharge.title') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.recharge.description') }}
            </p>
          </div>
          <div class="space-y-5 p-6">
            <!-- 加载状态 -->
            <div v-if="rechargeLoading" class="flex items-center gap-2 text-gray-500">
              <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
              {{ t('common.loading') }}
            </div>

            <template v-else>
              <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
                <!-- 最小充值金额 -->
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.recharge.minAmount') }}
                  </label>
                  <input
                    v-model.number="rechargeForm.min_amount"
                    type="number"
                    step="0.01"
                    min="0.01"
                    class="input"
                    placeholder="1.00"
                  />
                </div>

                <!-- 最大充值金额 -->
                <div>
                  <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.settings.recharge.maxAmount') }}
                  </label>
                  <input
                    v-model.number="rechargeForm.max_amount"
                    type="number"
                    step="0.01"
                    min="0.01"
                    class="input"
                    placeholder="1000.00"
                  />
                </div>
              </div>

              <!-- 订单过期时间 -->
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.recharge.orderExpireMinutes') }}
                </label>
                <div class="flex items-center gap-2">
                  <input
                    v-model.number="rechargeForm.order_expire_minutes"
                    type="number"
                    min="1"
                    max="1440"
                    class="input w-32"
                    placeholder="120"
                  />
                  <span class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.recharge.minutes') }}
                  </span>
                </div>
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.recharge.expireHint') }}
                </p>
              </div>

              <!-- 汇率配置 -->
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.recharge.exchangeRate') }}
                </label>
                <div class="flex items-center gap-2">
                  <input
                    v-model.number="rechargeForm.exchange_rate"
                    type="number"
                    step="0.01"
                    min="0.01"
                    class="input w-32"
                    placeholder="1.00"
                  />
                  <span class="text-sm text-gray-500 dark:text-gray-400">
                    {{ t('admin.settings.recharge.exchangeRateUnit') }}
                  </span>
                </div>
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.recharge.exchangeRateHint') }}
                </p>
              </div>

              <!-- 默认金额选项 -->
              <div>
                <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('admin.settings.recharge.defaultAmounts') }}
                </label>
                <div class="flex flex-wrap gap-2 mb-2">
                  <span
                    v-for="(amount, index) in rechargeForm.default_amounts"
                    :key="index"
                    class="inline-flex items-center gap-1 px-3 py-1 rounded-full bg-primary-100 text-primary-800 dark:bg-primary-900/30 dark:text-primary-300 text-sm"
                  >
                    ¥{{ amount }}
                    <button
                      type="button"
                      class="hover:text-primary-600 dark:hover:text-primary-200"
                      @click="removeRechargeAmount(index)"
                    >
                      <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  </span>
                </div>
                <div class="flex items-center gap-2">
                  <input
                    v-model.number="newRechargeAmount"
                    type="number"
                    step="0.01"
                    min="0.01"
                    class="input w-32"
                    :placeholder="t('admin.settings.recharge.addAmount')"
                    @keyup.enter="addRechargeAmount"
                  />
                  <button
                    type="button"
                    class="btn btn-secondary"
                    :disabled="!newRechargeAmount || newRechargeAmount <= 0"
                    @click="addRechargeAmount"
                  >
                    {{ t('admin.settings.recharge.addAmount') }}
                  </button>
                </div>
                <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.settings.recharge.amountsHint') }}
                </p>
              </div>

              <!-- 保存按钮 -->
              <div class="flex justify-end pt-4 border-t border-gray-100 dark:border-dark-700">
                <button
                  type="button"
                  class="btn btn-primary"
                  :disabled="rechargeSaving"
                  @click="saveRechargeSettings"
                >
                  <svg v-if="rechargeSaving" class="mr-2 h-4 w-4 animate-spin" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none" />
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                  </svg>
                  {{ rechargeSaving ? t('common.saving') : t('common.save') }}
                </button>
              </div>
            </template>
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
                <!-- Logo Preview -->
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
                <!-- Upload Controls -->
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
                <!-- Logo Preview -->
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
                <!-- Upload Controls -->
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
              <!-- iframe CSP Warning -->
              <p class="mt-2 text-xs text-amber-600 dark:text-amber-400">
                {{ t('admin.settings.site.homeContentIframeWarning') }}
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
            <!-- Enable Toggle -->
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

            <!-- URL -->
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

        <!-- Save Button -->
        <div class="flex justify-end">
          <button type="submit" :disabled="saving" class="btn btn-primary">
            <svg v-if="saving" class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
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
            {{ saving ? t('admin.settings.saving') : t('admin.settings.saveSettings') }}
          </button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import type { SystemSettings, UpdateSettingsRequest, WeChatPayStatus, RechargeSettings } from '@/api/admin/settings'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const saving = ref(false)
const testingSmtp = ref(false)
const generatingQRCode = ref(false)
const sendingTestEmail = ref(false)
const testEmailAddress = ref('')
const logoError = ref('')
const logoDarkError = ref('')
const qrcodeError = ref('')

// Admin API Key 状态
const adminApiKeyLoading = ref(true)
const adminApiKeyExists = ref(false)
const adminApiKeyMasked = ref('')
const adminApiKeyOperating = ref(false)
const newAdminApiKey = ref('')

// Stream Timeout 状态
const streamTimeoutLoading = ref(true)
const streamTimeoutSaving = ref(false)
const streamTimeoutForm = reactive({
  enabled: true,
  action: 'temp_unsched' as 'temp_unsched' | 'error' | 'none',
  temp_unsched_minutes: 5,
  threshold_count: 3,
  threshold_window_minutes: 10
})

// 微信支付配置状态（只读）
const wechatPayLoading = ref(true)
const wechatPayStatus = ref<WeChatPayStatus | null>(null)

// 充值业务配置
const rechargeLoading = ref(true)
const rechargeSaving = ref(false)
const newRechargeAmount = ref<number | null>(null)
const rechargeForm = reactive<RechargeSettings>({
  min_amount: 1,
  max_amount: 1000,
  default_amounts: [10, 50, 100, 200, 500],
  order_expire_minutes: 120,
  exchange_rate: 1
})

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
  // Cloudflare Turnstile
  turnstile_enabled: false,
  turnstile_site_key: '',
  turnstile_secret_key: '',
  turnstile_secret_key_configured: false,
  // LinuxDo Connect OAuth 登录
  linuxdo_connect_enabled: false,
  linuxdo_connect_client_id: '',
  linuxdo_connect_client_secret: '',
  linuxdo_connect_client_secret_configured: false,
  linuxdo_connect_redirect_url: '',
  // 微信公众号验证码登录
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
  // Model fallback
  enable_model_fallback: false,
  fallback_model_anthropic: 'claude-3-5-sonnet-20241022',
  fallback_model_openai: 'gpt-4o',
  fallback_model_gemini: 'gemini-2.5-pro',
  fallback_model_antigravity: 'gemini-2.5-pro',
  // Identity patch (Claude -> Gemini)
  enable_identity_patch: true,
  identity_patch_prompt: '',
  // Ops monitoring (vNext)
  ops_monitoring_enabled: true,
  ops_realtime_monitoring_enabled: true,
  ops_query_mode_default: 'auto',
  ops_metrics_interval_seconds: 60,
  // Usage report settings
  usage_report_global_enabled: false,
  usage_report_target_scope: 'opted_in',
  usage_report_global_schedule: '09:00'
})

// LinuxDo OAuth redirect URL suggestion
const linuxdoRedirectUrlSuggestion = computed(() => {
  if (typeof window === 'undefined') return ''
  const origin =
    window.location.origin || `${window.location.protocol}//${window.location.host}`
  return `${origin}/api/v1/auth/oauth/linuxdo/callback`
})

// 微信公众号二维码 - 计算最终生效的二维码（上传优先）
const effectiveWeChatQRCode = computed(() => {
  return form.wechat_account_qrcode_data || form.wechat_account_qrcode_url || ''
})

// 微信公众号二维码来源标识
const wechatQRCodeSource = computed(() => {
  if (form.wechat_account_qrcode_data) return 'uploaded'
  if (form.wechat_account_qrcode_url) return 'generated'
  return ''
})

// 微信公众号二维码上传错误
const wechatAccountQRCodeError = ref('')

// 微信公众号二维码上传处理
function handleWeChatAccountQRCodeUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  wechatAccountQRCodeError.value = ''

  if (!file) return

  // Check file size (500KB = 512000 bytes)
  const maxSize = 500 * 1024
  if (file.size > maxSize) {
    wechatAccountQRCodeError.value = t('admin.settings.wechat.qrcodeSizeError', {
      size: (file.size / 1024).toFixed(1)
    })
    input.value = ''
    return
  }

  // Check file type
  if (!file.type.startsWith('image/')) {
    wechatAccountQRCodeError.value = t('admin.settings.wechat.qrcodeTypeError')
    input.value = ''
    return
  }

  // Convert to base64
  const reader = new FileReader()
  reader.onload = (e) => {
    form.wechat_account_qrcode_data = e.target?.result as string
  }
  reader.onerror = () => {
    wechatAccountQRCodeError.value = t('admin.settings.wechat.qrcodeReadError')
  }
  reader.readAsDataURL(file)

  // Reset input
  input.value = ''
}

async function setAndCopyLinuxdoRedirectUrl() {
  const url = linuxdoRedirectUrlSuggestion.value
  if (!url) return

  form.linuxdo_connect_redirect_url = url
  await copyToClipboard(url, t('admin.settings.linuxdo.redirectUrlSetAndCopied'))
}

function handleLogoUpload(event: Event, type: 'light' | 'dark' = 'light') {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  const errorRef = type === 'light' ? logoError : logoDarkError
  errorRef.value = ''

  if (!file) return

  // Check file size (300KB = 307200 bytes)
  const maxSize = 300 * 1024
  if (file.size > maxSize) {
    errorRef.value = t('admin.settings.site.logoSizeError', {
      size: (file.size / 1024).toFixed(1)
    })
    input.value = ''
    return
  }

  // Check file type
  if (!file.type.startsWith('image/')) {
    errorRef.value = t('admin.settings.site.logoTypeError')
    input.value = ''
    return
  }

  // Convert to base64
  const reader = new FileReader()
  reader.onload = (e) => {
    if (type === 'light') {
      form.site_logo = e.target?.result as string
    } else {
      form.site_logo_dark = e.target?.result as string
    }
  }
  reader.onerror = () => {
    errorRef.value = t('admin.settings.site.logoReadError')
  }
  reader.readAsDataURL(file)

  // Reset input
  input.value = ''
}

function handleQRCodeUpload(event: Event, type: 'wechat' | 'group') {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  qrcodeError.value = ''

  if (!file) return

  // Check file size (500KB = 512000 bytes)
  const maxSize = 500 * 1024
  if (file.size > maxSize) {
    qrcodeError.value = t('admin.settings.site.qrcodeSizeError', {
      size: (file.size / 1024).toFixed(1)
    })
    input.value = ''
    return
  }

  // Check file type
  if (!file.type.startsWith('image/')) {
    qrcodeError.value = t('admin.settings.site.qrcodeTypeError')
    input.value = ''
    return
  }

  // Convert to base64
  const reader = new FileReader()
  reader.onload = (e) => {
    if (type === 'wechat') {
      form.contact_qrcode_wechat = e.target?.result as string
    } else {
      form.contact_qrcode_group = e.target?.result as string
    }
  }
  reader.onerror = () => {
    qrcodeError.value = t('admin.settings.site.qrcodeReadError')
  }
  reader.readAsDataURL(file)

  // Reset input
  input.value = ''
}

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
      // Usage report settings
      usage_report_global_enabled: form.usage_report_global_enabled,
      usage_report_target_scope: form.usage_report_target_scope,
      usage_report_global_schedule: form.usage_report_global_schedule
    }
    const updated = await adminAPI.settings.updateSettings(payload)
    Object.assign(form, updated)
    form.smtp_password = ''
    form.turnstile_secret_key = ''
    form.linuxdo_connect_client_secret = ''
    form.wechat_server_token = ''
    form.wechat_app_secret = ''
    // Refresh cached public settings so sidebar/header update immediately
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

async function testSmtpConnection() {
  testingSmtp.value = true
  try {
    const result = await adminAPI.settings.testSmtpConnection({
      smtp_host: form.smtp_host,
      smtp_port: form.smtp_port,
      smtp_username: form.smtp_username,
      smtp_password: form.smtp_password,
      smtp_use_tls: form.smtp_use_tls
    })
    // API returns { message: "..." } on success, errors are thrown as exceptions
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
      smtp_host: form.smtp_host,
      smtp_port: form.smtp_port,
      smtp_username: form.smtp_username,
      smtp_password: form.smtp_password,
      smtp_from_email: form.smtp_from_email,
      smtp_from_name: form.smtp_from_name,
      smtp_use_tls: form.smtp_use_tls
    })
    // API returns { message: "..." } on success, errors are thrown as exceptions
    appStore.showSuccess(result.message || t('admin.settings.testEmailSent'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.failedToSendTestEmail') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    sendingTestEmail.value = false
  }
}

// 微信公众号二维码生成
async function generateWeChatQRCode() {
  generatingQRCode.value = true
  try {
    const result = await adminAPI.settings.generateWeChatQRCode({
      app_id: form.wechat_app_id,
      app_secret: form.wechat_app_secret
    })
    // 自动填充生成的二维码 URL
    form.wechat_account_qrcode_url = result.qrcode_url
    appStore.showSuccess(t('admin.settings.wechat.generateQRCodeSuccess'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.wechat.generateQRCodeFailed') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    generatingQRCode.value = false
  }
}

// Admin API Key 方法
async function loadAdminApiKey() {
  adminApiKeyLoading.value = true
  try {
    const status = await adminAPI.settings.getAdminApiKey()
    adminApiKeyExists.value = status.exists
    adminApiKeyMasked.value = status.masked_key
  } catch (error: any) {
    console.error('Failed to load admin API key status:', error)
  } finally {
    adminApiKeyLoading.value = false
  }
}

async function createAdminApiKey() {
  adminApiKeyOperating.value = true
  try {
    const result = await adminAPI.settings.regenerateAdminApiKey()
    newAdminApiKey.value = result.key
    adminApiKeyExists.value = true
    adminApiKeyMasked.value = result.key.substring(0, 10) + '...' + result.key.slice(-4)
    appStore.showSuccess(t('admin.settings.adminApiKey.keyGenerated'))
  } catch (error: any) {
    appStore.showError(error.message || t('common.error'))
  } finally {
    adminApiKeyOperating.value = false
  }
}

async function regenerateAdminApiKey() {
  if (!confirm(t('admin.settings.adminApiKey.regenerateConfirm'))) return
  await createAdminApiKey()
}

async function deleteAdminApiKey() {
  if (!confirm(t('admin.settings.adminApiKey.deleteConfirm'))) return
  adminApiKeyOperating.value = true
  try {
    await adminAPI.settings.deleteAdminApiKey()
    adminApiKeyExists.value = false
    adminApiKeyMasked.value = ''
    newAdminApiKey.value = ''
    appStore.showSuccess(t('admin.settings.adminApiKey.keyDeleted'))
  } catch (error: any) {
    appStore.showError(error.message || t('common.error'))
  } finally {
    adminApiKeyOperating.value = false
  }
}

function copyNewKey() {
  navigator.clipboard
    .writeText(newAdminApiKey.value)
    .then(() => {
      appStore.showSuccess(t('admin.settings.adminApiKey.keyCopied'))
    })
    .catch(() => {
      appStore.showError(t('common.copyFailed'))
    })
}

// Stream Timeout 方法
async function loadStreamTimeoutSettings() {
  streamTimeoutLoading.value = true
  try {
    const settings = await adminAPI.settings.getStreamTimeoutSettings()
    Object.assign(streamTimeoutForm, settings)
  } catch (error: any) {
    console.error('Failed to load stream timeout settings:', error)
  } finally {
    streamTimeoutLoading.value = false
  }
}

async function saveStreamTimeoutSettings() {
  streamTimeoutSaving.value = true
  try {
    const updated = await adminAPI.settings.updateStreamTimeoutSettings({
      enabled: streamTimeoutForm.enabled,
      action: streamTimeoutForm.action,
      temp_unsched_minutes: streamTimeoutForm.temp_unsched_minutes,
      threshold_count: streamTimeoutForm.threshold_count,
      threshold_window_minutes: streamTimeoutForm.threshold_window_minutes
    })
    Object.assign(streamTimeoutForm, updated)
    appStore.showSuccess(t('admin.settings.streamTimeout.saved'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.streamTimeout.saveFailed') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    streamTimeoutSaving.value = false
  }
}

async function loadWeChatPayStatus() {
  wechatPayLoading.value = true
  try {
    wechatPayStatus.value = await adminAPI.settings.getWeChatPayStatus()
  } catch (error: any) {
    console.error('Failed to load WeChat Pay status:', error)
    // 静默失败，不影响其他设置加载
  } finally {
    wechatPayLoading.value = false
  }
}

async function loadRechargeSettings() {
  rechargeLoading.value = true
  try {
    const settings = await adminAPI.settings.getRechargeSettings()
    Object.assign(rechargeForm, settings)
  } catch (error: any) {
    console.error('Failed to load recharge settings:', error)
  } finally {
    rechargeLoading.value = false
  }
}

function addRechargeAmount() {
  if (!newRechargeAmount.value || newRechargeAmount.value <= 0) return
  if (rechargeForm.default_amounts.includes(newRechargeAmount.value)) {
    newRechargeAmount.value = null
    return
  }
  rechargeForm.default_amounts.push(newRechargeAmount.value)
  rechargeForm.default_amounts.sort((a, b) => a - b)
  newRechargeAmount.value = null
}

function removeRechargeAmount(index: number) {
  rechargeForm.default_amounts.splice(index, 1)
}

async function saveRechargeSettings() {
  // 验证
  if (rechargeForm.min_amount > rechargeForm.max_amount) {
    appStore.showError(t('admin.settings.recharge.errors.minGreaterThanMax'))
    return
  }

  if (rechargeForm.exchange_rate <= 0) {
    appStore.showError(t('admin.settings.recharge.errors.invalidExchangeRate'))
    return
  }

  for (const amount of rechargeForm.default_amounts) {
    if (amount < rechargeForm.min_amount || amount > rechargeForm.max_amount) {
      appStore.showError(t('admin.settings.recharge.errors.amountOutOfRange'))
      return
    }
  }

  rechargeSaving.value = true
  try {
    const updated = await adminAPI.settings.updateRechargeSettings({
      min_amount: rechargeForm.min_amount,
      max_amount: rechargeForm.max_amount,
      default_amounts: rechargeForm.default_amounts,
      order_expire_minutes: rechargeForm.order_expire_minutes,
      exchange_rate: rechargeForm.exchange_rate
    })
    Object.assign(rechargeForm, updated)
    appStore.showSuccess(t('admin.settings.recharge.saveSuccess'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.recharge.saveFailed') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    rechargeSaving.value = false
  }
}

onMounted(() => {
  loadSettings()
  loadAdminApiKey()
  loadStreamTimeoutSettings()
  loadWeChatPayStatus()
  loadRechargeSettings()
})
</script>
