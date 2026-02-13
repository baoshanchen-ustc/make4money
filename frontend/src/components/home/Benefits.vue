<template>
  <section id="benefits" class="scroll-mt-16 py-12 sm:py-16">
    <h2 class="mb-3 text-center text-2xl font-bold text-gray-900 dark:text-white sm:text-3xl">
      {{ t('home.benefits.title') }}
    </h2>
    <p class="mb-8 text-center text-sm text-gray-500 dark:text-dark-400 sm:mb-12 sm:text-base">
      {{ t('home.benefits.subtitle') }}
    </p>
    <div class="grid gap-4 sm:grid-cols-2 md:gap-6 lg:grid-cols-4">
      <div
        v-for="item in benefits"
        :key="item.key"
        class="group rounded-2xl border border-gray-200/50 bg-white/60 p-5 backdrop-blur-sm transition-all hover:shadow-lg dark:border-dark-700/50 dark:bg-dark-800/60"
      >
        <div class="mb-3 flex h-10 w-10 items-center justify-center rounded-xl" :class="item.bgClass">
          <Icon :name="item.icon" size="md" class="text-white" />
        </div>
        <h3 class="mb-1.5 text-sm font-semibold text-gray-900 dark:text-white">
          {{ t(`home.benefits.items.${item.key}.title`) }}
        </h3>
        <p class="text-xs leading-relaxed text-gray-500 dark:text-dark-400">
          {{ t(`home.benefits.items.${item.key}.desc`) }}
        </p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const benefits = [
  {
    key: 'pureSubscription',
    icon: 'shield' as const,
    bgClass: 'bg-gradient-to-br from-primary-500 to-primary-600',
  },
  {
    key: 'extendedThinking',
    icon: 'brain' as const,
    bgClass: 'bg-gradient-to-br from-purple-500 to-purple-600',
  },
  {
    key: 'noInterruption',
    icon: 'bolt' as const,
    bgClass: 'bg-gradient-to-br from-emerald-500 to-emerald-600',
  },
  {
    key: 'fullContext',
    icon: 'document' as const,
    bgClass: 'bg-gradient-to-br from-blue-500 to-blue-600',
  },
  {
    key: 'autoCompression',
    icon: 'sparkles' as const,
    bgClass: 'bg-gradient-to-br from-amber-500 to-amber-600',
  },
  {
    key: 'highAvailability',
    icon: 'server' as const,
    bgClass: 'bg-gradient-to-br from-rose-500 to-rose-600',
  },
  {
    key: 'stickySession',
    icon: 'link' as const,
    bgClass: 'bg-gradient-to-br from-sky-500 to-sky-600',
  },
  {
    key: 'oneLineIntegration',
    icon: 'terminal' as const,
    bgClass: 'bg-gradient-to-br from-teal-500 to-teal-600',
  },
  {
    key: 'multiModel',
    icon: 'cube' as const,
    bgClass: 'bg-gradient-to-br from-indigo-500 to-indigo-600',
  },
  {
    key: 'realtimeMonitoring',
    icon: 'chartBar' as const,
    bgClass: 'bg-gradient-to-br from-cyan-500 to-cyan-600',
  },
  {
    key: 'payAsYouGo',
    icon: 'creditCard' as const,
    bgClass: 'bg-gradient-to-br from-orange-500 to-orange-600',
  },
  {
    key: 'flexibleQuota',
    icon: 'calculator' as const,
    bgClass: 'bg-gradient-to-br from-fuchsia-500 to-fuchsia-600',
  },
]
</script>
