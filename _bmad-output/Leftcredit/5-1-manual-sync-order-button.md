# Story 5.1: 手动同步订单状态按钮

Status: done

## Story

**作为** 普通用户
**我希望** 点击按钮主动查询订单的支付状态
**以便** 在回调延迟时确认支付结果

## Acceptance Criteria

- [x] AC1: 支付中页面显示「手动查询订单状态」按钮
- [x] AC2: 按钮点击后调用同步接口
- [x] AC3: 按钮有冷却时间（5秒内不可重复点击）
- [x] AC4: 同步中显示loading状态
- [x] AC5: 同步结果显示提示（已支付/支付中/支付失败）

## Tasks / Subtasks

- [x] Task 1: 前端 - 添加同步按钮组件 (AC: 1, 3, 4)
  - [x] 1.1 在 `RechargePaymentView.vue` 添加「手动查询订单状态」按钮
  - [x] 1.2 实现按钮冷却状态管理（5秒倒计时）
  - [x] 1.3 实现按钮 loading 状态

- [x] Task 2: 前端 - API 调用与结果处理 (AC: 2, 5)
  - [x] 2.1 在 `src/api/recharge.ts` 添加 `syncOrderStatus` 方法
  - [x] 2.2 点击按钮后调用同步接口
  - [x] 2.3 根据返回状态显示 Toast 提示
  - [x] 2.4 状态为 `paid` 时跳转到成功页面
  - [x] 2.5 状态为 `failed/expired` 时跳转到失败页面

- [x] Task 3: 国际化 (AC: 1, 5)
  - [x] 3.1 添加中文翻译到 `frontend/src/i18n/locales/zh.ts`
  - [x] 3.2 添加英文翻译到 `frontend/src/i18n/locales/en.ts`

## Dev Notes

### 依赖关系

**前置条件**:
- Story 4.6（前端订单状态轮询）完成，`RechargePayingView.vue` 已存在
- Story 5.2（查询微信支付订单状态）完成，后端接口已可用

### 前端实现

#### 1. 同步按钮组件逻辑

在 `frontend/src/views/user/recharge/RechargePayingView.vue` 添加：

```vue
<template>
  <!-- 在二维码/等待区域下方添加 -->
  <div class="mt-6 text-center">
    <button
      @click="handleSyncStatus"
      :disabled="syncCooldown > 0 || isSyncing"
      :class="[
        'px-4 py-2 rounded-lg text-sm font-medium transition-colors',
        syncCooldown > 0 || isSyncing
          ? 'bg-gray-200 text-gray-500 cursor-not-allowed dark:bg-gray-700 dark:text-gray-400'
          : 'bg-primary-100 text-primary-700 hover:bg-primary-200 dark:bg-primary-900/30 dark:text-primary-400'
      ]"
    >
      <span v-if="isSyncing" class="flex items-center gap-2">
        <LoadingSpinner size="sm" />
        {{ t('recharge.paying.syncing') }}
      </span>
      <span v-else-if="syncCooldown > 0">
        {{ t('recharge.paying.syncCooldown', { seconds: syncCooldown }) }}
      </span>
      <span v-else>
        {{ t('recharge.paying.syncStatus') }}
      </span>
    </button>
    <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
      {{ t('recharge.paying.syncHint') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useToast } from '@/composables/useToast'
import { rechargeAPI } from '@/api'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const { t } = useI18n()
const router = useRouter()
const toast = useToast()

const props = defineProps<{
  orderNo: string
}>()

const isSyncing = ref(false)
const syncCooldown = ref(0)
let cooldownTimer: ReturnType<typeof setInterval> | null = null

// 5秒冷却倒计时
function startCooldown() {
  syncCooldown.value = 5
  cooldownTimer = setInterval(() => {
    syncCooldown.value--
    if (syncCooldown.value <= 0 && cooldownTimer) {
      clearInterval(cooldownTimer)
      cooldownTimer = null
    }
  }, 1000)
}

async function handleSyncStatus() {
  if (isSyncing.value || syncCooldown.value > 0) return

  isSyncing.value = true
  try {
    const result = await rechargeAPI.syncOrderStatus(props.orderNo)

    // 根据状态显示提示并跳转
    switch (result.status) {
      case 'paid':
        toast.success(t('recharge.paying.syncResult.paid'))
        router.push({ name: 'recharge-success', params: { orderNo: props.orderNo } })
        break
      case 'pending':
        toast.info(t('recharge.paying.syncResult.pending'))
        break
      case 'failed':
      case 'expired':
        toast.warning(t('recharge.paying.syncResult.failed'))
        router.push({ name: 'recharge-failed', params: { orderNo: props.orderNo } })
        break
      default:
        toast.info(t('recharge.paying.syncResult.unknown'))
    }
  } catch (error: any) {
    toast.error(error.message || t('recharge.paying.syncError'))
  } finally {
    isSyncing.value = false
    startCooldown()
  }
}

onUnmounted(() => {
  if (cooldownTimer) {
    clearInterval(cooldownTimer)
  }
})
</script>
```

#### 2. API 客户端方法

在 `frontend/src/api/recharge.ts` 添加：

```typescript
export interface SyncOrderStatusResponse {
  order_no: string
  status: 'pending' | 'paid' | 'failed' | 'expired' | 'refunded'
  wechat_status?: string  // 微信侧的原始状态
  synced_at: string
}

/**
 * 手动同步订单状态
 * 调用微信支付查询接口获取最新状态
 */
export async function syncOrderStatus(orderNo: string): Promise<SyncOrderStatusResponse> {
  const { data } = await apiClient.post<SyncOrderStatusResponse>(
    `/recharge/orders/${orderNo}/sync`
  )
  return data
}
```

#### 3. 国际化文本

**中文** (`frontend/src/locales/zh-CN.json`):
```json
{
  "recharge": {
    "paying": {
      "syncStatus": "手动查询订单状态",
      "syncing": "查询中...",
      "syncCooldown": "{seconds}秒后可再次查询",
      "syncHint": "如果支付成功但页面未跳转，请点击上方按钮手动查询",
      "syncError": "查询失败，请稍后重试",
      "syncResult": {
        "paid": "支付成功！正在跳转...",
        "pending": "订单仍在支付中，请完成支付",
        "failed": "订单已失败或过期",
        "unknown": "订单状态未知，请稍后再试"
      }
    }
  }
}
```

**英文** (`frontend/src/locales/en.json`):
```json
{
  "recharge": {
    "paying": {
      "syncStatus": "Check Order Status",
      "syncing": "Checking...",
      "syncCooldown": "Retry in {seconds}s",
      "syncHint": "If payment succeeded but page didn't redirect, click above to check",
      "syncError": "Check failed, please try again",
      "syncResult": {
        "paid": "Payment successful! Redirecting...",
        "pending": "Order still pending, please complete payment",
        "failed": "Order failed or expired",
        "unknown": "Unknown order status, please try again later"
      }
    }
  }
}
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `frontend/src/views/user/recharge/RechargePayingView.vue` | 添加同步按钮 |
| `frontend/src/api/recharge.ts` | 添加 syncOrderStatus 方法 |
| `frontend/src/locales/zh-CN.json` | 中文翻译 |
| `frontend/src/locales/en.json` | 英文翻译 |

### UI/UX 设计要点

1. **按钮位置**: 放置在二维码下方，倒计时上方
2. **按钮样式**: 使用次要按钮样式，避免与主操作冲突
3. **冷却提示**: 清晰显示剩余秒数，避免用户困惑
4. **结果反馈**: 使用 Toast 提示查询结果，成功/失败自动跳转

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-5.1] - 用户故事定义
- [Source: _bmad-output/planning-artifacts/ux-design.md] - UX 设计参考
- [Source: frontend/src/components/common/LoadingSpinner.vue] - 加载动画组件
- [Source: frontend/src/composables/useToast.ts] - Toast 提示（如存在）

### 测试要点

1. **冷却时间测试**: 点击后5秒内按钮不可再次点击
2. **并发测试**: 快速多次点击只触发一次请求
3. **状态跳转测试**: 不同状态跳转到正确页面
4. **网络错误处理**: 接口失败时显示错误提示

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Debug Log References

无

### Completion Notes List

- 在 `recharge.ts` 添加 `SyncOrderStatusResponse` 接口和 `syncOrderStatus` API 方法
- 在 `RechargePaymentView.vue` 添加手动同步按钮（同时支持 Native 和 JSAPI 支付区域）
- 实现 5 秒冷却倒计时，防止频繁调用
- 使用 `useAppStore` 的 toast 方法显示同步结果
- 根据同步结果自动跳转到成功或失败页面
- 组件卸载时自动清理计时器防止内存泄漏
- 添加中英文国际化翻译

### File List

- `frontend/src/api/recharge.ts` - 添加 syncOrderStatus 方法和 SyncOrderStatusResponse 接口
- `frontend/src/views/user/RechargePaymentView.vue` - 添加手动同步按钮和相关逻辑
- `frontend/src/i18n/locales/zh.ts` - 添加中文翻译
- `frontend/src/i18n/locales/en.ts` - 添加英文翻译
