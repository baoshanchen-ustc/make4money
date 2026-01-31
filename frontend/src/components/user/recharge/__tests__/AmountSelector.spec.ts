/**
 * AmountSelector 组件单元测试
 */
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import AmountSelector from '../AmountSelector.vue'

// 创建测试用 i18n 实例
const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  messages: {
    zh: {
      recharge: {
        amount: '充值金额',
        customAmount: '自定义金额',
        selectAmount: '选择充值金额{amount}元',
        amountRange: '充值范围：¥{min} - ¥{max}',
        invalidAmount: '请输入有效金额',
        amountTooSmall: '最小充值金额为 ¥{min}',
        amountTooLarge: '最大充值金额为 ¥{max}'
      }
    }
  }
})

describe('AmountSelector', () => {
  const defaultProps = {
    modelValue: null as number | null,
    defaultAmounts: [10, 20, 50, 100],
    minAmount: 1,
    maxAmount: 1000
  }

  const mountComponent = (props = {}) => {
    return mount(AmountSelector, {
      props: { ...defaultProps, ...props },
      global: {
        plugins: [i18n]
      }
    })
  }

  describe('Task 1: 组件基础结构', () => {
    it('应该正确渲染组件', () => {
      const wrapper = mountComponent()
      expect(wrapper.exists()).toBe(true)
    })

    it('应该显示充值金额标签', () => {
      const wrapper = mountComponent()
      expect(wrapper.text()).toContain('充值金额')
    })

    it('应该显示金额范围提示', () => {
      const wrapper = mountComponent({ minAmount: 1, maxAmount: 1000 })
      expect(wrapper.text()).toContain('充值范围')
    })
  })

  describe('Task 2: 快捷金额按钮', () => {
    it('应该显示所有快捷金额按钮', () => {
      const wrapper = mountComponent({ defaultAmounts: [10, 20, 50, 100] })
      const buttons = wrapper.findAll('[data-testid="quick-amount-btn"]')
      expect(buttons).toHaveLength(4)
    })

    it('按钮应该显示正确的金额', () => {
      const wrapper = mountComponent({ defaultAmounts: [10, 20, 50, 100] })
      const buttons = wrapper.findAll('[data-testid="quick-amount-btn"]')
      expect(buttons[0].text()).toContain('10')
      expect(buttons[1].text()).toContain('20')
      expect(buttons[2].text()).toContain('50')
      expect(buttons[3].text()).toContain('100')
    })

    it('点击快捷按钮应该选中对应金额', async () => {
      const wrapper = mountComponent()
      const button = wrapper.findAll('[data-testid="quick-amount-btn"]')[1] // ¥20
      await button.trigger('click')

      expect(wrapper.emitted('update:modelValue')).toBeTruthy()
      expect(wrapper.emitted('update:modelValue')![0]).toEqual([20])
    })

    it('选中的按钮应该高亮显示', async () => {
      const wrapper = mountComponent({ modelValue: 50 })
      const buttons = wrapper.findAll('[data-testid="quick-amount-btn"]')
      // ¥50 是第三个按钮
      expect(buttons[2].classes()).toContain('btn-selected')
    })
  })

  describe('Task 3: 自定义金额输入', () => {
    it('应该显示自定义金额输入框', () => {
      const wrapper = mountComponent()
      const input = wrapper.find('[data-testid="custom-amount-input"]')
      expect(input.exists()).toBe(true)
    })

    it('输入自定义金额应该触发 update:modelValue', async () => {
      const wrapper = mountComponent()
      const input = wrapper.find('[data-testid="custom-amount-input"]')
      await input.setValue('88')

      expect(wrapper.emitted('update:modelValue')).toBeTruthy()
      expect(wrapper.emitted('update:modelValue')![0]).toEqual([88])
    })

    it('输入自定义金额应该取消快捷按钮选中状态', async () => {
      const wrapper = mountComponent({ modelValue: 50 })
      const input = wrapper.find('[data-testid="custom-amount-input"]')
      await input.setValue('88')

      // 验证没有按钮被选中
      const buttons = wrapper.findAll('[data-testid="quick-amount-btn"]')
      buttons.forEach((btn) => {
        expect(btn.classes()).not.toContain('btn-selected')
      })
    })
  })

  describe('Task 4: 输入校验', () => {
    it('只允许输入数字和小数点，非法字符应被过滤', async () => {
      const wrapper = mountComponent()
      const input = wrapper.find('[data-testid="custom-amount-input"]')

      // 模拟输入非法字符
      await input.setValue('abc')
      // 组件应该过滤掉非数字字符，emit null（空字符串）
      const emitted = wrapper.emitted('update:modelValue')
      expect(emitted).toBeTruthy()
      // 过滤后为空，应该 emit null
      expect(emitted![emitted!.length - 1][0]).toBe(null)
    })

    it('混合输入应保留数字部分', async () => {
      const wrapper = mountComponent()
      const input = wrapper.find('[data-testid="custom-amount-input"]')

      await input.setValue('12abc34')
      const emitted = wrapper.emitted('update:modelValue')
      expect(emitted).toBeTruthy()
      // 过滤后应为 1234
      expect(emitted![emitted!.length - 1][0]).toBe(1234)
    })

    it('最多允许两位小数', async () => {
      const wrapper = mountComponent()
      const input = wrapper.find('[data-testid="custom-amount-input"]')

      await input.setValue('10.123')
      const emitted = wrapper.emitted('update:modelValue')
      expect(emitted).toBeTruthy()
      const lastValue = emitted![emitted!.length - 1][0] as number
      // 应该被截断到两位小数
      expect(lastValue).toBe(10.12)
    })

    it('金额小于最小值时显示错误提示', async () => {
      const wrapper = mountComponent({ minAmount: 10 })
      const input = wrapper.find('[data-testid="custom-amount-input"]')
      await input.setValue('5')
      await input.trigger('blur')

      expect(wrapper.text()).toContain('最小充值金额')
    })

    it('金额大于最大值时显示错误提示', async () => {
      const wrapper = mountComponent({ maxAmount: 100 })
      const input = wrapper.find('[data-testid="custom-amount-input"]')
      await input.setValue('150')
      await input.trigger('blur')

      expect(wrapper.text()).toContain('最大充值金额')
    })

    it('有效金额不显示错误', async () => {
      const wrapper = mountComponent({ minAmount: 1, maxAmount: 1000 })
      const input = wrapper.find('[data-testid="custom-amount-input"]')
      await input.setValue('50')
      await input.trigger('blur')

      expect(wrapper.find('[data-testid="error-message"]').exists()).toBe(false)
    })

    it('多个小数点应只保留第一个', async () => {
      const wrapper = mountComponent()
      const input = wrapper.find('[data-testid="custom-amount-input"]')

      await input.setValue('1.2.3')
      const emitted = wrapper.emitted('update:modelValue')
      expect(emitted).toBeTruthy()
      // 应该变成 1.23
      expect(emitted![emitted!.length - 1][0]).toBe(1.23)
    })
  })

  describe('双向绑定 (v-model)', () => {
    it('modelValue 变化应该更新输入框和按钮状态', async () => {
      const wrapper = mountComponent({ modelValue: 20 })

      // 输入框应该显示当前值
      const input = wrapper.find('[data-testid="custom-amount-input"]')
      expect((input.element as HTMLInputElement).value).toBe('20')

      // 对应按钮应该高亮
      const buttons = wrapper.findAll('[data-testid="quick-amount-btn"]')
      expect(buttons[1].classes()).toContain('btn-selected') // ¥20
    })
  })

  describe('ARIA 可访问性', () => {
    it('快捷按钮应该有 aria-label', () => {
      const wrapper = mountComponent()
      const buttons = wrapper.findAll('[data-testid="quick-amount-btn"]')
      buttons.forEach((btn) => {
        expect(btn.attributes('aria-label')).toBeTruthy()
      })
    })

    it('输入框应该有 aria-label', () => {
      const wrapper = mountComponent()
      const input = wrapper.find('[data-testid="custom-amount-input"]')
      expect(input.attributes('aria-label')).toBeTruthy()
    })
  })
})
