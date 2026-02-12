<template>
  <!-- eslint-disable vue/no-mutating-props -->
  <div class="space-y-6">
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
                @click="regenerateAdminApiKeyHandler"
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

        <!-- Invitation Code -->
        <div
          class="flex items-center justify-between border-t border-gray-100 pt-4 dark:border-dark-700"
        >
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{
              t('admin.settings.registration.invitationCode')
            }}</label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.registration.invitationCodeHint') }}
            </p>
          </div>
          <Toggle v-model="form.invitation_code_enabled" />
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

    <!-- LinuxDo Connect OAuth -->
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

    <!-- WeChat Login -->
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
        <!-- Config tip -->
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

        <!-- Force email bind -->
        <div class="flex items-center justify-between">
          <div>
            <label class="font-medium text-gray-900 dark:text-white">{{
              t('admin.settings.wechat.forceEmailBind')
            }}</label>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.wechat.forceEmailBindHint') }}
            </p>
          </div>
          <Toggle v-model="form.force_email_bind" />
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
            <!-- Account type -->
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

              <!-- Manual upload divider -->
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

              <!-- Manual upload QR code -->
              <div class="mt-6">
                <div class="flex items-start gap-6">
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

              <!-- Final preview -->
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
  </div>
</template>

<script setup lang="ts">
/* eslint-disable vue/no-mutating-props */
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores'

const props = defineProps<{
  form: any
}>()

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const generatingQRCode = ref(false)

// Admin API Key state
const adminApiKeyLoading = ref(true)
const adminApiKeyExists = ref(false)
const adminApiKeyMasked = ref('')
const adminApiKeyOperating = ref(false)
const newAdminApiKey = ref('')

// WeChat QR code upload error
const wechatAccountQRCodeError = ref('')

// LinuxDo OAuth redirect URL suggestion
const linuxdoRedirectUrlSuggestion = computed(() => {
  if (typeof window === 'undefined') return ''
  const origin =
    window.location.origin || `${window.location.protocol}//${window.location.host}`
  return `${origin}/api/v1/auth/oauth/linuxdo/callback`
})

// WeChat QR code computed
const effectiveWeChatQRCode = computed(() => {
  return props.form.wechat_account_qrcode_data || props.form.wechat_account_qrcode_url || ''
})

const wechatQRCodeSource = computed(() => {
  if (props.form.wechat_account_qrcode_data) return 'uploaded'
  if (props.form.wechat_account_qrcode_url) return 'generated'
  return ''
})

// WeChat QR code upload handler
function handleWeChatAccountQRCodeUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  wechatAccountQRCodeError.value = ''

  if (!file) return

  const maxSize = 500 * 1024
  if (file.size > maxSize) {
    wechatAccountQRCodeError.value = t('admin.settings.wechat.qrcodeSizeError', {
      size: (file.size / 1024).toFixed(1)
    })
    input.value = ''
    return
  }

  if (!file.type.startsWith('image/')) {
    wechatAccountQRCodeError.value = t('admin.settings.wechat.qrcodeTypeError')
    input.value = ''
    return
  }

  const reader = new FileReader()
  reader.onload = (e) => {
    props.form.wechat_account_qrcode_data = e.target?.result as string
  }
  reader.onerror = () => {
    wechatAccountQRCodeError.value = t('admin.settings.wechat.qrcodeReadError')
  }
  reader.readAsDataURL(file)
  input.value = ''
}

async function setAndCopyLinuxdoRedirectUrl() {
  const url = linuxdoRedirectUrlSuggestion.value
  if (!url) return

  props.form.linuxdo_connect_redirect_url = url
  await copyToClipboard(url, t('admin.settings.linuxdo.redirectUrlSetAndCopied'))
}

// WeChat QR code generation
async function generateWeChatQRCode() {
  generatingQRCode.value = true
  try {
    const result = await adminAPI.settings.generateWeChatQRCode({
      app_id: props.form.wechat_app_id,
      app_secret: props.form.wechat_app_secret
    })
    props.form.wechat_account_qrcode_url = result.qrcode_url
    appStore.showSuccess(t('admin.settings.wechat.generateQRCodeSuccess'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.wechat.generateQRCodeFailed') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    generatingQRCode.value = false
  }
}

// Admin API Key methods
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

async function regenerateAdminApiKeyHandler() {
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

onMounted(() => {
  loadAdminApiKey()
})
</script>
