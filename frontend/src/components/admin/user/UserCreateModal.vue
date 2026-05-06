<template>
  <BaseDialog
    :show="show"
    :title="t('admin.users.createUser')"
    width="normal"
    @close="$emit('close')"
  >
    <form id="create-user-form" @submit.prevent="submit" class="space-y-5">
      <div>
        <label class="input-label">{{ t('admin.users.email') }}</label>
        <input v-model="form.email" type="email" required class="input" :placeholder="t('admin.users.enterEmail')" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.password') }}</label>
        <div class="flex gap-2">
          <div class="relative flex-1">
            <input v-model="form.password" type="text" required class="input pr-10" :placeholder="t('admin.users.enterPassword')" />
          </div>
          <button type="button" @click="generateRandomPassword" class="btn btn-secondary px-3">
            <Icon name="refresh" size="md" />
          </button>
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.username') }}</label>
        <input v-model="form.username" type="text" class="input" :placeholder="t('admin.users.enterUsername')" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.columns.role') }}</label>
        <Select v-model="form.role" :options="roleOptions" />
      </div>
      <div v-if="isChannelAdmin" class="space-y-3">
        <div class="flex items-center justify-between gap-3">
          <label class="input-label mb-0">{{ t('admin.users.columns.groups') }}</label>
          <span class="text-xs text-gray-400 dark:text-dark-400">
            {{ t('admin.users.selectedGroupsCount', { count: form.allowed_groups.length }) }}
          </span>
        </div>
        <div v-if="groupsLoading" class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-300">
          {{ t('common.loading') }}
        </div>
        <div
          v-else-if="availableGroups.length > 0"
          class="max-h-56 space-y-2 overflow-y-auto rounded-lg border border-gray-200 p-3 dark:border-dark-600"
        >
          <label
            v-for="group in availableGroups"
            :key="group.id"
            class="flex cursor-pointer items-start gap-3 rounded-lg border border-transparent px-2 py-2 transition-colors hover:border-primary-200 hover:bg-primary-50 dark:hover:border-primary-800 dark:hover:bg-primary-900/20"
          >
            <input
              :checked="form.allowed_groups.includes(group.id)"
              type="checkbox"
              class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              @change="toggleGroup(group.id)"
            />
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ group.name }}</div>
              <div class="mt-0.5 text-xs text-gray-500 dark:text-dark-300">
                {{ t('admin.groups.platforms.' + group.platform, group.platform) }} · ID {{ group.id }}
              </div>
            </div>
          </label>
        </div>
        <p v-else class="text-sm text-gray-500 dark:text-dark-300">{{ t('common.noGroupsAvailable') }}</p>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('admin.users.columns.balance') }}</label>
          <input v-model.number="form.balance" type="number" step="any" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.users.columns.concurrency') }}</label>
          <input v-model.number="form.concurrency" type="number" class="input" />
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.form.rpmLimit') }}</label>
        <input
          v-model.number="form.rpm_limit"
          type="number"
          min="0"
          step="1"
          class="input"
          :placeholder="t('admin.users.form.rpmLimitPlaceholder')"
        />
        <p class="input-hint">{{ t('admin.users.form.rpmLimitHint') }}</p>
      </div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="$emit('close')" type="button" class="btn btn-secondary">{{ t('common.cancel') }}</button>
        <button type="submit" form="create-user-form" :disabled="loading" class="btn btn-primary">
          {{ loading ? t('admin.users.creating') : t('common.create') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminUserRole } from '@/api/admin/users'
import type { AdminGroup } from '@/types'
import { useForm } from '@/composables/useForm'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()

const roleOptions = computed(() => [
  { value: 'admin', label: t('admin.users.roles.admin') },
  { value: 'channel_admin', label: t('admin.users.roles.channel_admin') },
  { value: 'user', label: t('admin.users.roles.user') }
])

const availableGroups = ref<AdminGroup[]>([])
const groupsLoading = ref(false)
const groupsLoaded = ref(false)

const createInitialForm = () => ({
  email: '',
  password: '',
  username: '',
  notes: '',
  role: 'user' as AdminUserRole,
  allowed_groups: [] as number[],
  balance: 0,
  concurrency: 1,
  rpm_limit: 0
})

const form = reactive(createInitialForm())
const isChannelAdmin = computed(() => form.role === 'channel_admin')

const loadGroups = async () => {
  if (groupsLoading.value || groupsLoaded.value) return
  groupsLoading.value = true
  try {
    const groups = await adminAPI.groups.getAll()
    availableGroups.value = groups.filter((group) => group.status === 'active')
    groupsLoaded.value = true
  } finally {
    groupsLoading.value = false
  }
}

const toggleGroup = (groupId: number) => {
  const index = form.allowed_groups.indexOf(groupId)
  if (index >= 0) {
    form.allowed_groups.splice(index, 1)
  } else {
    form.allowed_groups.push(groupId)
  }
}

watch(isChannelAdmin, (enabled) => {
  if (enabled) {
    void loadGroups()
    return
  }
  form.allowed_groups = []
})

const { loading, submit } = useForm({
  form,
  submitFn: async (data) => {
    await adminAPI.users.create({
      email: data.email,
      password: data.password,
      username: data.username || undefined,
      notes: data.notes || undefined,
      role: data.role,
      balance: data.balance,
      concurrency: data.concurrency,
      rpm_limit: data.rpm_limit,
      allowed_groups: data.role === 'channel_admin' ? [...data.allowed_groups] : undefined,
    })
    emit('success')
    emit('close')
  },
  successMsg: t('admin.users.userCreated')
})

watch(() => props.show, (v) => {
  if (!v) return
  Object.assign(form, createInitialForm())
})

const generateRandomPassword = () => {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789!@#$%^&*'
  let p = ''
  for (let i = 0; i < 16; i++) p += chars.charAt(Math.floor(Math.random() * chars.length))
  form.password = p
}
</script>
