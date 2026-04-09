<template>
  <div class="space-y-0">
    <!-- Tab navigation -->
    <div class="border-b border-gray-200 dark:border-dark-700">
      <nav class="-mb-px flex space-x-6 overflow-x-auto">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          @click="activeTab = tab.key"
          class="whitespace-nowrap border-b-2 px-1 pb-3 text-sm font-medium transition-colors"
          :class="[
            activeTab === tab.key
              ? 'border-primary-500 text-primary-600 dark:text-primary-400'
              : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-200'
          ]"
        >
          {{ tab.label }}
        </button>
      </nav>
    </div>

    <!-- Tab content: directly mount real user view components with adminUserId prop -->
    <div class="mt-6">
      <DashboardView v-if="activeTab === 'dashboard'" :admin-user-id="userId" :key="`dashboard-${userId}`" />
      <KeysView v-else-if="activeTab === 'apikeys'" :admin-user-id="userId" :key="`keys-${userId}`" />
      <UsageView v-else-if="activeTab === 'usage'" :admin-user-id="userId" :key="`usage-${userId}`" />
      <SubscriptionsView v-else-if="activeTab === 'subscriptions'" :admin-user-id="userId" :key="`subs-${userId}`" />
      <RedeemView v-else-if="activeTab === 'redeem'" :admin-user-id="userId" :key="`redeem-${userId}`" />
      <ConnectivityTestView v-else-if="activeTab === 'connectivity'" :admin-user-id="userId" :key="`connectivity-${userId}`" />
      <ProfileView v-else-if="activeTab === 'profile'" :admin-user-id="userId" :key="`profile-${userId}`" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import DashboardView from '@/views/user/DashboardView.vue'
import KeysView from '@/views/user/KeysView.vue'
import UsageView from '@/views/user/UsageView.vue'
import SubscriptionsView from '@/views/user/SubscriptionsView.vue'
import RedeemView from '@/views/user/RedeemView.vue'
import ConnectivityTestView from '@/views/user/ConnectivityTestView.vue'
import ProfileView from '@/views/user/ProfileView.vue'

const { userId } = defineProps<{
  userId: number
}>()

type TabKey = 'dashboard' | 'apikeys' | 'usage' | 'subscriptions' | 'redeem' | 'connectivity' | 'profile'

const tabs: { key: TabKey; label: string }[] = [
  { key: 'dashboard', label: '仪表盘' },
  { key: 'apikeys', label: 'API 密钥' },
  { key: 'usage', label: '使用记录' },
  { key: 'subscriptions', label: '订阅' },
  { key: 'redeem', label: '兑换' },
  { key: 'connectivity', label: '连接测试' },
  { key: 'profile', label: '资料' },
]

const activeTab = ref<TabKey>('dashboard')
</script>
