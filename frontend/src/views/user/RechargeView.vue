<template>
  <main class="container mx-auto px-4 py-6">
    <div class="max-w-2xl mx-auto">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white mb-4">
        {{ t('recharge.title') }}
      </h1>
      <p class="text-gray-600 dark:text-gray-400">
        {{ t('recharge.comingSoon') }}
      </p>
    </div>
  </main>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>
