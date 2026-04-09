<template>
  <AppLayout>
    <div>
      <UserViewBanner
        :selected-user="selectedUser"
        @user-selected="onUserSelected"
      />
      <UserViewTabs
        v-if="selectedUser"
        :user-id="selectedUser.id"
      />
      <div
        v-else
        class="flex flex-col items-center justify-center rounded-lg border border-dashed border-gray-200 py-20 text-center dark:border-dark-700"
      >
        <svg
          class="mb-4 h-12 w-12 text-gray-300 dark:text-dark-600"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="1"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z" />
        </svg>
        <p class="text-sm font-medium text-gray-500 dark:text-dark-400">请从上方选择一个用户以查看其视图</p>
        <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">搜索用户邮箱或 ID，选中后即可查看该用户的仪表盘、密钥、使用记录等</p>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import UserViewBanner from '@/components/admin/user-view/UserViewBanner.vue'
import UserViewTabs from '@/components/admin/user-view/UserViewTabs.vue'

interface SelectedUser {
  id: number
  email: string
}

const route = useRoute()
const router = useRouter()

const selectedUser = ref<SelectedUser | null>(null)

const onUserSelected = (user: SelectedUser) => {
  selectedUser.value = user
  router.push(`/admin/user-view/${user.id}`)
}

// On mount, if URL has :userId, auto-load that user's info
onMounted(async () => {
  const userId = route.params.userId
  if (userId) {
    try {
      const user = await adminAPI.users.getById(Number(userId))
      selectedUser.value = { id: user.id, email: user.email }
    } catch (error) {
      console.error('Failed to load user from URL param:', error)
    }
  }
})
</script>
