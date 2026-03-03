<template>
  <AppLayout>
    <div class="mx-auto max-w-3xl space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
      </div>

      <!-- Error State -->
      <div
        v-else-if="error"
        class="rounded-xl border border-red-100 bg-red-50 px-6 py-10 text-center dark:border-red-900/40 dark:bg-red-900/20"
      >
        <svg
          class="mx-auto mb-3 h-10 w-10 text-red-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="1.5"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z"
          />
        </svg>
        <p class="text-sm font-medium text-red-700 dark:text-red-300">{{ error }}</p>
        <button type="button" @click="reload" class="btn btn-outline btn-sm mt-4">
          {{ t('common.retry') }}
        </button>
      </div>

      <template v-else-if="info">
        <!-- Referral Code & Link Card -->
        <div class="card overflow-hidden">
          <div class="bg-gradient-to-br from-primary-500 to-primary-600 px-6 py-8 text-center">
            <div
              class="mb-4 inline-flex h-16 w-16 items-center justify-center rounded-2xl bg-white/20 backdrop-blur-sm"
            >
              <svg
                class="h-8 w-8 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="1.5"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M7.217 10.907a2.25 2.25 0 1 0 0 2.186m0-2.186c.18.324.283.696.283 1.093s-.103.77-.283 1.093m0-2.186 9.566-5.314m-9.566 7.5 9.566 5.314m0 0a2.25 2.25 0 1 0 3.935 2.186 2.25 2.25 0 0 0-3.935-2.186zm0-12.814a2.25 2.25 0 1 0 3.933-2.185 2.25 2.25 0 0 0-3.933 2.185z"
                />
              </svg>
            </div>
            <p class="text-sm font-medium text-primary-100">{{ t('referral.title') }}</p>
            <p class="mt-1 text-sm text-primary-200">{{ t('referral.description') }}</p>
          </div>

          <div class="space-y-5 p-6">
            <!-- Referral Code -->
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('referral.myCode') }}
              </label>
              <div class="flex items-center gap-3">
                <code
                  class="flex-1 rounded-lg bg-gray-100 px-4 py-2.5 font-mono text-lg font-semibold tracking-widest text-gray-900 dark:bg-dark-700 dark:text-gray-100"
                >
                  {{ info.referral_code }}
                </code>
                <button
                  type="button"
                  @click="copyCode"
                  class="btn btn-outline btn-sm whitespace-nowrap"
                >
                  <svg
                    class="mr-1.5 h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="1.5"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 0 1-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 0 1 1.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 0 0-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 0 1-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 0 0-3.375-3.375h-1.5a1.125 1.125 0 0 1-1.125-1.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H9.75"
                    />
                  </svg>
                  {{ t('referral.copyCode') }}
                </button>
              </div>
            </div>

            <!-- Referral Link -->
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('referral.myLink') }}
              </label>
              <div class="flex items-center gap-3">
                <input
                  type="text"
                  readonly
                  :value="referralLink"
                  class="input flex-1 truncate font-mono text-sm text-gray-600 dark:text-gray-400"
                />
                <button
                  type="button"
                  @click="copyLink"
                  class="btn btn-primary btn-sm whitespace-nowrap"
                >
                  <svg
                    class="mr-1.5 h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="1.5"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244"
                    />
                  </svg>
                  {{ t('referral.copyLink') }}
                </button>
              </div>
            </div>

            <!-- Copy Feedback Toast -->
            <Transition name="fade">
              <div
                v-if="copyFeedback"
                class="rounded-lg bg-green-50 px-4 py-2.5 text-sm font-medium text-green-700 dark:bg-green-900/30 dark:text-green-400"
              >
                {{ copyFeedback }}
              </div>
            </Transition>
          </div>
        </div>

        <!-- Stats Card -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('referral.stats') }}
            </h2>
          </div>
          <div class="grid grid-cols-2 divide-x divide-gray-100 dark:divide-dark-700">
            <div class="p-6 text-center">
              <p class="text-3xl font-bold text-gray-900 dark:text-white">
                {{ info.total_invitees }}
              </p>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('referral.totalInvitees') }}
              </p>
            </div>
            <div class="p-6 text-center">
              <p class="text-3xl font-bold text-primary-600 dark:text-primary-400">
                ${{ info.total_reward_earned.toFixed(2) }}
              </p>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('referral.totalRewardEarned') }}
              </p>
            </div>
          </div>
        </div>

        <!-- Invited By (if applicable) -->
        <div
          v-if="info.inviter_info"
          class="rounded-xl border border-blue-100 bg-blue-50 px-5 py-4 dark:border-blue-900/40 dark:bg-blue-900/20"
        >
          <p class="text-sm text-blue-700 dark:text-blue-300">
            {{ t('referral.invitedBy') }}:
            <span class="font-medium">{{ info.inviter_info.email_masked }}</span>
          </p>
        </div>

        <!-- Invitees Table -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('referral.inviteeList') }}
            </h2>
          </div>

          <div v-if="inviteesLoading" class="flex justify-center py-8">
            <div class="h-6 w-6 animate-spin rounded-full border-b-2 border-primary-600"></div>
          </div>

          <div
            v-else-if="inviteesError"
            class="px-6 py-8 text-center text-sm text-red-600 dark:text-red-400"
          >
            {{ inviteesError }}
          </div>

          <div v-else-if="invitees.length === 0" class="px-6 py-10 text-center">
            <svg
              class="mx-auto mb-3 h-10 w-10 text-gray-300 dark:text-dark-600"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z"
              />
            </svg>
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('referral.noInvitees') }}</p>
          </div>

          <div v-else class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b border-gray-100 dark:border-dark-700">
                  <th
                    class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400"
                  >
                    {{ t('referral.emailMasked') }}
                  </th>
                  <th
                    class="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400"
                  >
                    {{ t('referral.registeredAt') }}
                  </th>
                  <th
                    class="px-6 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400"
                  >
                    {{ t('referral.rewardEarned') }}
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-50 dark:divide-dark-700">
                <tr
                  v-for="(invitee, idx) in invitees"
                  :key="(currentPage - 1) * pageSize + idx"
                  class="hover:bg-gray-50 dark:hover:bg-dark-700/50"
                >
                  <td class="px-6 py-3 font-mono text-gray-900 dark:text-gray-100">
                    {{ invitee.email_masked }}
                  </td>
                  <td class="px-6 py-3 text-gray-500 dark:text-gray-400">
                    {{ formatDate(invitee.registered_at) }}
                  </td>
                  <td class="px-6 py-3 text-right font-medium text-primary-600 dark:text-primary-400">
                    ${{ invitee.reward_earned.toFixed(2) }}
                  </td>
                </tr>
              </tbody>
            </table>

            <!-- Pagination -->
            <div
              v-if="totalPages > 1"
              class="flex items-center justify-between border-t border-gray-100 px-6 py-3 dark:border-dark-700"
            >
              <p class="text-sm text-gray-500 dark:text-gray-400">
                {{ t('pagination.pageOf', { page: currentPage, total: totalPages }) }}
              </p>
              <div class="flex gap-2">
                <button
                  type="button"
                  :disabled="currentPage <= 1"
                  @click="goToPage(currentPage - 1)"
                  class="btn btn-outline btn-sm"
                >
                  {{ t('pagination.previous') }}
                </button>
                <button
                  type="button"
                  :disabled="currentPage >= totalPages"
                  @click="goToPage(currentPage + 1)"
                  class="btn btn-outline btn-sm"
                >
                  {{ t('pagination.next') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- How It Works -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('referral.howItWorks') }}
            </h2>
          </div>
          <ol class="divide-y divide-gray-50 p-6 dark:divide-dark-700">
            <li
              v-for="(step, i) in [t('referral.step1'), t('referral.step2'), t('referral.step3')]"
              :key="i"
              class="flex items-start gap-4 py-3 first:pt-0 last:pb-0"
            >
              <span
                class="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-primary-100 text-sm font-semibold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300"
              >
                {{ i + 1 }}
              </span>
              <p class="text-sm text-gray-600 dark:text-gray-400">{{ step }}</p>
            </li>
          </ol>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { referralAPI } from '@/api/referral'
import type { ReferralInfo, ReferralInvitee } from '@/types'

const { t } = useI18n()

const loading = ref(true)
const error = ref('')
const info = ref<ReferralInfo | null>(null)
const invitees = ref<ReferralInvitee[]>([])
const inviteesLoading = ref(false)
const inviteesError = ref('')
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 20

const copyFeedback = ref('')
let feedbackTimer: ReturnType<typeof setTimeout> | null = null

const totalPages = computed(() => Math.ceil(totalItems.value / pageSize))

// Build the full referral link on the frontend using window.location.origin,
// so it is always correct regardless of whether the backend received an Origin header.
const referralLink = computed(() => {
  if (!info.value) return ''
  return `${window.location.origin}/register?ref=${info.value.referral_code}`
})

function showFeedback(msg: string) {
  copyFeedback.value = msg
  if (feedbackTimer) clearTimeout(feedbackTimer)
  feedbackTimer = setTimeout(() => {
    copyFeedback.value = ''
  }, 2500)
}

async function copyText(text: string, successKey: string) {
  // Prefer Clipboard API (requires HTTPS / secure context).
  // Fall back to execCommand for HTTP environments.
  if (window.isSecureContext && navigator.clipboard) {
    try {
      await navigator.clipboard.writeText(text)
      showFeedback(t(successKey))
      return
    } catch {
      // fall through to execCommand fallback
    }
  }
  try {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(textarea)
    showFeedback(ok ? t(successKey) : t('referral.copyFailed'))
  } catch {
    showFeedback(t('referral.copyFailed'))
  }
}

function copyCode() {
  if (info.value) copyText(info.value.referral_code, 'referral.codeCopied')
}

function copyLink() {
  copyText(referralLink.value, 'referral.linkCopied')
}

async function loadInvitees(page: number) {
  inviteesLoading.value = true
  inviteesError.value = ''
  try {
    const resp = await referralAPI.listMyInvitees(page, pageSize)
    invitees.value = resp.items
    totalItems.value = resp.total
    currentPage.value = page
  } catch (e) {
    inviteesError.value = e instanceof Error ? e.message : t('common.unknownError')
  } finally {
    inviteesLoading.value = false
  }
}

async function goToPage(page: number) {
  await loadInvitees(page)
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

async function reload() {
  loading.value = true
  error.value = ''
  inviteesError.value = ''
  info.value = null
  try {
    info.value = await referralAPI.getMyReferralInfo()
    await loadInvitees(1)
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('common.unknownError')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    info.value = await referralAPI.getMyReferralInfo()
    await loadInvitees(1)
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('common.unknownError')
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
