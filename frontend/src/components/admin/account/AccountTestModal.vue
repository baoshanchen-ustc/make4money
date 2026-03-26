<template>
  <AccountTestModalImpl
    :show="show"
    :account="account"
    @close="$emit('close')"
  />
</template>

<script setup lang="ts">
import AccountTestModalImpl from '@/components/account/AccountTestModal.vue'
import type { Account } from '@/types'

defineProps<{
  show: boolean
  account: Account | null
}>()

defineEmits<{
  (e: 'close'): void
}>()
</script>
