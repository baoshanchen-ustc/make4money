<template>
  <nav
    class="sticky top-0 z-40 border-b border-gray-200/50 bg-white/80 backdrop-blur-lg dark:border-dark-800/50 dark:bg-dark-950/80"
  >
    <div class="mx-auto flex max-w-6xl items-center justify-between px-4 py-3 sm:px-6">
      <!-- Logo -->
      <router-link to="/home" class="flex shrink-0 items-center gap-2">
        <div class="h-8 w-8 overflow-hidden rounded-lg shadow-sm">
          <img :src="currentLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
        </div>
        <span class="hidden text-sm font-semibold text-gray-900 dark:text-white sm:inline">{{ siteName }}</span>
      </router-link>

      <!-- Anchor Links (scrollable on mobile) -->
      <div class="mx-4 flex items-center gap-1 overflow-x-auto scrollbar-hide sm:gap-2">
        <a
          v-for="section in sections"
          :key="section.id"
          :href="'#' + section.id"
          @click.prevent="scrollToSection(section.id)"
          class="whitespace-nowrap rounded-md px-2.5 py-1 text-xs font-medium transition-colors sm:px-3 sm:py-1.5 sm:text-sm"
          :class="activeSection === section.id
            ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
            : 'text-gray-500 hover:text-gray-700 dark:text-dark-400 dark:hover:text-white'"
        >
          {{ section.label }}
        </a>
      </div>

      <!-- Right Actions -->
      <div class="flex shrink-0 items-center gap-2">
        <LocaleSwitcher />
        <!-- Theme Toggle -->
        <button
          @click="toggleTheme"
          class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
        >
          <Icon v-if="themeMode === 'light'" name="sun" size="sm" class="text-amber-500" />
          <Icon v-else-if="themeMode === 'dark'" name="moon" size="sm" class="text-indigo-400" />
          <Icon v-else name="clock" size="sm" class="text-emerald-500" />
        </button>
        <!-- Docs Link -->
        <a
          v-if="docUrl"
          :href="docUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="hidden rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800 sm:inline-flex"
          :title="t('home.docs')"
        >
          <Icon name="book" size="sm" />
        </a>
        <!-- Install Guide -->
        <router-link
          to="/install-guide"
          class="hidden rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800 sm:inline-flex"
          :title="t('home.installGuide')"
        >
          <Icon name="terminal" size="sm" />
        </router-link>
        <!-- Login/Dashboard -->
        <router-link v-if="isAuthenticated" :to="dashboardPath" class="btn btn-primary btn-sm">
          {{ t('home.dashboard') }}
        </router-link>
        <router-link v-else to="/login" class="btn btn-primary btn-sm">
          {{ t('home.login') }}
        </router-link>
      </div>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import { useTheme } from '@/composables/useTheme'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()
const { isDark, themeMode, toggleTheme } = useTheme()

// Site settings
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Code80')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const siteLogoDark = computed(() => appStore.cachedPublicSettings?.site_logo_dark || appStore.siteLogoDark || '')

const currentLogo = computed(() => {
  if (isDark.value && siteLogoDark.value) {
    return siteLogoDark.value
  }
  return siteLogo.value
})

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')

// Navigation sections
const sections = computed(() => [
  { id: 'benefits', label: t('home.nav.benefits') },
  { id: 'how-it-works', label: t('home.nav.howItWorks') },
  { id: 'pricing', label: t('home.nav.pricing') },
  { id: 'faq', label: t('home.nav.faq') },
])

// Active section tracking
const activeSection = ref<string>('')
let observer: IntersectionObserver | null = null

function scrollToSection(id: string) {
  const el = document.getElementById(id)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
}

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          activeSection.value = entry.target.id
        }
      }
    },
    {
      rootMargin: '-80px 0px -60% 0px',
      threshold: 0,
    }
  )

  for (const section of sections.value) {
    const el = document.getElementById(section.id)
    if (el) {
      observer.observe(el)
    }
  }
})

onUnmounted(() => {
  if (observer) {
    observer.disconnect()
    observer = null
  }
})
</script>
