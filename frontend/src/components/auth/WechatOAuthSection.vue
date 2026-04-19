<template>
  <div class="space-y-3">
    <button
      type="button"
      :disabled="buttonDisabled"
      class="btn w-full"
      :class="buttonDisabled ? 'btn-secondary opacity-70' : 'btn-secondary'"
      @click="startLogin"
    >
      <span
        class="mr-2 inline-flex h-5 w-5 items-center justify-center rounded-full bg-green-100 text-xs font-semibold text-green-700 dark:bg-green-900/30 dark:text-green-300"
      >
        微
      </span>
      {{ t('auth.wechat.signIn') }}
    </button>

    <p v-if="availabilityHint" class="text-sm text-gray-500 dark:text-dark-400">
      {{ availabilityHint }}
    </p>

    <div v-if="showDivider" class="flex items-center gap-3">
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
      <span class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('auth.oauthOrContinue') }}
      </span>
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getOAuthStartUrl, getWechatOAuthAvailabilityHintKey } from '@/api/auth'

const props = withDefaults(
  defineProps<{
    disabled?: boolean
    showDivider?: boolean
    openEnabled?: boolean
    openConfigured?: boolean
    mpEnabled?: boolean
    mpConfigured?: boolean
  }>(),
  {
    showDivider: true,
    openEnabled: false,
    openConfigured: true,
    mpEnabled: false,
    mpConfigured: true
  }
)

const route = useRoute()
const { t } = useI18n()

const userAgent = computed(() => (typeof navigator === 'undefined' ? '' : navigator.userAgent))

const startUrl = computed(() =>
  getOAuthStartUrl('wechat', (route.query.redirect as string) || '/dashboard', 'login', {
    wechatOpenEnabled: props.openEnabled,
    wechatOpenConfigured: props.openConfigured,
    wechatMpEnabled: props.mpEnabled,
    wechatMpConfigured: props.mpConfigured,
    userAgent: userAgent.value
  })
)

const availabilityHintKey = computed(() =>
  getWechatOAuthAvailabilityHintKey({
    wechatOpenEnabled: props.openEnabled,
    wechatOpenConfigured: props.openConfigured,
    wechatMpEnabled: props.mpEnabled,
    wechatMpConfigured: props.mpConfigured,
    userAgent: userAgent.value
  })
)

const buttonDisabled = computed(() => {
  return props.disabled || !startUrl.value
})

const availabilityHint = computed(() => {
  return availabilityHintKey.value ? t(availabilityHintKey.value) : ''
})

function startLogin(): void {
  if (buttonDisabled.value || !startUrl.value) {
    return
  }

  window.location.href = startUrl.value
}
</script>
