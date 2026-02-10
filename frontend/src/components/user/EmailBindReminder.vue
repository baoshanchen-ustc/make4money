<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition-all duration-300 ease-out"
      enter-from-class="translate-y-4 opacity-0"
      enter-to-class="translate-y-0 opacity-100"
      leave-active-class="transition-all duration-200 ease-in"
      leave-from-class="translate-y-0 opacity-100"
      leave-to-class="translate-y-4 opacity-0"
    >
      <div
        v-if="shouldShow"
        class="fixed bottom-4 right-4 z-50 max-w-sm"
      >
        <div class="bg-white dark:bg-dark-800 rounded-xl shadow-lg border border-gray-200 dark:border-dark-700 overflow-hidden">
          <div class="p-4">
            <div class="flex items-start gap-3">
              <div class="flex-shrink-0 p-2 bg-amber-100 dark:bg-amber-900/30 rounded-lg">
                <svg class="h-5 w-5 text-amber-600 dark:text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 01-2.25 2.25h-15a2.25 2.25 0 01-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25m19.5 0v.243a2.25 2.25 0 01-1.07 1.916l-7.5 4.615a2.25 2.25 0 01-2.36 0L3.32 8.91a2.25 2.25 0 01-1.07-1.916V6.75" />
                </svg>
              </div>
              <div class="flex-1 min-w-0">
                <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('emailReminder.title') }}
                </h4>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('emailReminder.description') }}
                </p>
              </div>
              <button
                type="button"
                class="flex-shrink-0 text-gray-400 hover:text-gray-500 dark:hover:text-gray-300"
                @click="dismiss"
              >
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div class="mt-4 flex items-center gap-3">
              <button
                type="button"
                class="btn btn-primary btn-sm"
                @click="openBindModal"
              >
                {{ t('emailReminder.bindNow') }}
              </button>
              <button
                type="button"
                class="btn btn-ghost btn-sm"
                @click="remindLater"
              >
                {{ t('emailReminder.later') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>

    <!-- Email Bind Modal (非强制模式) -->
    <EmailBindModal
      :show="showModal"
      :required="false"
      @close="showModal = false"
      @skip="showModal = false"
      @success="handleBindSuccess"
    />
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import EmailBindModal from '@/components/auth/EmailBindModal.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const STORAGE_KEY = 'email_reminder_dismissed_until'
const REMIND_LATER_HOURS = 24

const dismissed = ref(false)
const showModal = ref(false)

const needsEmailBind = computed(() => {
  const user = authStore.user
  if (!user) return false
  if (!user.email) return true
  if (user.email.endsWith('.invalid')) return true
  return false
})

const shouldShow = computed(() => {
  if (!authStore.isAuthenticated) return false
  if (!needsEmailBind.value) return false
  if (dismissed.value) return false
  // 强制模式下由路由守卫处理，不显示 toast 提醒
  if (appStore.cachedPublicSettings?.force_email_bind) return false
  return true
})

const checkDismissedUntil = () => {
  const dismissedUntil = localStorage.getItem(STORAGE_KEY)
  if (dismissedUntil) {
    const until = parseInt(dismissedUntil, 10)
    if (Date.now() < until) {
      dismissed.value = true
      return
    }
    localStorage.removeItem(STORAGE_KEY)
  }
  dismissed.value = false
}

const dismiss = () => {
  dismissed.value = true
}

const openBindModal = () => {
  showModal.value = true
}

const handleBindSuccess = () => {
  showModal.value = false
  dismissed.value = true
}

const remindLater = () => {
  const until = Date.now() + REMIND_LATER_HOURS * 60 * 60 * 1000
  localStorage.setItem(STORAGE_KEY, until.toString())
  dismissed.value = true
}

// Watch for user changes (login/logout)
watch(() => authStore.user, () => {
  checkDismissedUntil()
})

onMounted(() => {
  checkDismissedUntil()
})
</script>
