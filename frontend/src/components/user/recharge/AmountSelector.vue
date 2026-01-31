<template>
  <div class="amount-selector">
    <!-- 金额标签 -->
    <label class="mb-3 block text-sm font-medium text-gray-700 dark:text-gray-300">
      {{ t('recharge.amount') }}
    </label>

    <!-- 输入框 + 快捷按钮容器 -->
    <div class="flex flex-wrap items-center gap-3">
      <!-- 自定义金额输入框 -->
      <div class="relative">
        <span
          class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-gray-500 dark:text-gray-400"
        >
          ¥
        </span>
        <input
          ref="inputRef"
          data-testid="custom-amount-input"
          type="text"
          inputmode="decimal"
          :value="displayValue"
          :aria-label="t('recharge.customAmount')"
          class="input w-[140px] pl-7 text-center transition-all duration-200"
          :class="{
            'input-error ring-2 ring-red-500/20': errorMessage,
            'ring-2 ring-primary-500/30 border-primary-500': isCustomInput
          }"
          :placeholder="t('recharge.customAmount')"
          @input="handleInput"
          @blur="handleBlur"
          @focus="handleFocus"
        />
      </div>

      <!-- 快捷金额按钮组 -->
      <div class="flex flex-wrap gap-3">
        <button
          v-for="amount in defaultAmounts"
          :key="amount"
          type="button"
          data-testid="quick-amount-btn"
          :aria-label="t('recharge.selectAmount', { amount })"
          class="quick-amount-btn transition-all duration-200"
          :class="isSelected(amount) ? 'btn-selected' : 'btn-default'"
          @click="selectAmount(amount)"
        >
          ¥{{ amount }}
        </button>
      </div>
    </div>

    <!-- 金额范围提示 / 错误信息 -->
    <div class="mt-2 min-h-[20px] text-sm">
      <p v-if="errorMessage" data-testid="error-message" class="text-red-500">
        {{ errorMessage }}
      </p>
      <p v-else class="text-gray-500 dark:text-gray-400">
        {{ t('recharge.amountRange', { min: minAmount, max: maxAmount }) }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

interface Props {
  modelValue: number | null
  defaultAmounts: number[]
  minAmount: number
  maxAmount: number
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: null,
  defaultAmounts: () => [10, 20, 50, 100],
  minAmount: 1,
  maxAmount: 1000
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: number | null): void
}>()

const { t } = useI18n()

// 内部输入值（字符串形式，用于显示）
const inputValue = ref('')
// 是否正在进行自定义输入（非快捷按钮选择）
const isCustomInput = ref(false)
// 错误信息
const errorMessage = ref('')
// 输入框引用
const inputRef = ref<HTMLInputElement | null>(null)

// 显示值：优先显示内部输入值，否则显示 modelValue
const displayValue = computed(() => {
  if (inputValue.value !== '') {
    return inputValue.value
  }
  if (props.modelValue !== null) {
    return String(props.modelValue)
  }
  return ''
})

// 判断快捷按钮是否选中
const isSelected = (amount: number): boolean => {
  return props.modelValue === amount && !isCustomInput.value
}

// 选择快捷金额
const selectAmount = (amount: number) => {
  isCustomInput.value = false
  inputValue.value = String(amount)
  errorMessage.value = ''
  emit('update:modelValue', amount)
}

// 解析并格式化金额输入
const parseAmount = (value: string): { valid: boolean; amount: number | null; formatted: string } => {
  // 去除前后空格
  const trimmed = value.trim()

  // 空值
  if (trimmed === '') {
    return { valid: true, amount: null, formatted: '' }
  }

  // 过滤非法字符，只保留数字和小数点
  let filtered = trimmed.replace(/[^\d.]/g, '')

  // 确保只有一个小数点
  const parts = filtered.split('.')
  if (parts.length > 2) {
    filtered = parts[0] + '.' + parts.slice(1).join('')
  }

  // 限制小数位数为两位
  if (parts.length === 2 && parts[1].length > 2) {
    filtered = parts[0] + '.' + parts[1].substring(0, 2)
  }

  // 尝试解析为数字
  const num = parseFloat(filtered)
  if (isNaN(num)) {
    return { valid: false, amount: null, formatted: filtered }
  }

  // 限制小数为两位
  const rounded = Math.round(num * 100) / 100

  return { valid: true, amount: rounded, formatted: filtered }
}

// 处理输入事件
const handleInput = (event: Event) => {
  const target = event.target as HTMLInputElement
  const { valid, amount, formatted } = parseAmount(target.value)

  // 更新内部值
  inputValue.value = formatted
  isCustomInput.value = true

  // 清除错误
  errorMessage.value = ''

  if (valid && amount !== null) {
    emit('update:modelValue', amount)
  } else if (formatted === '') {
    emit('update:modelValue', null)
  }
}

// 处理失焦事件 - 进行范围验证
const handleBlur = () => {
  // 检查输入框的实际值
  const currentInput = inputValue.value || (inputRef.value?.value ?? '')
  if (currentInput === '') {
    errorMessage.value = ''
    return
  }

  // 解析当前输入值
  const { valid, amount } = parseAmount(currentInput)
  if (!valid || amount === null) {
    errorMessage.value = ''
    return
  }

  if (amount < props.minAmount) {
    errorMessage.value = t('recharge.amountTooSmall', { min: props.minAmount })
  } else if (amount > props.maxAmount) {
    errorMessage.value = t('recharge.amountTooLarge', { max: props.maxAmount })
  } else {
    errorMessage.value = ''
  }
}

// 处理聚焦事件
const handleFocus = () => {
  isCustomInput.value = true
}

// 监听外部 modelValue 变化，同步内部状态
watch(
  () => props.modelValue,
  (newVal) => {
    if (newVal !== null) {
      // 如果新值是快捷金额之一，则不标记为自定义输入
      if (props.defaultAmounts.includes(newVal) && !isCustomInput.value) {
        inputValue.value = String(newVal)
      }
    }
  },
  { immediate: true }
)

// 暴露方法供父组件调用
defineExpose({
  focus: () => inputRef.value?.focus(),
  clearError: () => {
    errorMessage.value = ''
  }
})
</script>

<style scoped>
.quick-amount-btn {
  @apply flex h-11 w-16 items-center justify-center rounded-xl text-base font-medium;
}

.btn-default {
  @apply border border-gray-200 bg-white text-gray-900 hover:border-gray-300 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-100 dark:hover:border-dark-500 dark:hover:bg-dark-600;
}

.btn-selected {
  @apply border-primary-500 bg-primary-100 text-primary-600 dark:border-primary-400 dark:bg-primary-900/30 dark:text-primary-400;
}
</style>
