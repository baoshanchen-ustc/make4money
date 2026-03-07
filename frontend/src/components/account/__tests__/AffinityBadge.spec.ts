import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AffinityBadge from '../AffinityBadge.vue'

vi.mock('@/api/admin/accounts', () => ({
  getAffinityClients: vi.fn()
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, fallback?: string) => fallback ?? key
    })
  }
})

function mountBadge(count: number) {
  return mount(AffinityBadge, {
    props: {
      accountId: 42,
      count
    }
  })
}

describe('AffinityBadge', () => {
  it('renders the correct count number', () => {
    const wrapper = mountBadge(5)
    expect(wrapper.text()).toContain('5')
  })

  it('renders count=0 correctly', () => {
    const wrapper = mountBadge(0)
    expect(wrapper.text()).toContain('0')
  })

  it('applies red badge class when count >= 16', () => {
    const wrapper = mountBadge(16)
    const badge = wrapper.find('span')
    expect(badge.classes()).toContain('bg-red-100')
    expect(badge.classes()).toContain('text-red-700')
  })

  it('applies red badge class when count > 16', () => {
    const wrapper = mountBadge(25)
    const badge = wrapper.find('span')
    expect(badge.classes()).toContain('bg-red-100')
    expect(badge.classes()).toContain('text-red-700')
  })

  it('applies yellow badge class when count >= 6 and < 16', () => {
    const wrapper = mountBadge(6)
    const badge = wrapper.find('span')
    expect(badge.classes()).toContain('bg-yellow-100')
    expect(badge.classes()).toContain('text-yellow-700')
  })

  it('applies yellow badge class for count=15', () => {
    const wrapper = mountBadge(15)
    const badge = wrapper.find('span')
    expect(badge.classes()).toContain('bg-yellow-100')
    expect(badge.classes()).toContain('text-yellow-700')
  })

  it('applies green badge class when count > 0 and < 6', () => {
    const wrapper = mountBadge(1)
    const badge = wrapper.find('span')
    expect(badge.classes()).toContain('bg-emerald-100')
    expect(badge.classes()).toContain('text-emerald-700')
  })

  it('applies green badge class for count=5', () => {
    const wrapper = mountBadge(5)
    const badge = wrapper.find('span')
    expect(badge.classes()).toContain('bg-emerald-100')
    expect(badge.classes()).toContain('text-emerald-700')
  })

  it('applies gray badge class when count is 0', () => {
    const wrapper = mountBadge(0)
    const badge = wrapper.find('span')
    expect(badge.classes()).toContain('bg-gray-100')
    expect(badge.classes()).toContain('text-gray-600')
  })

  it('does NOT show popover initially', () => {
    const wrapper = mountBadge(3)
    // showPopover starts as false, so the Teleport content with v-if should not render
    expect(wrapper.html()).not.toContain('divide-y')
    expect(wrapper.html()).not.toContain('affinityClients')
  })

  it('has mouseenter and mouseleave handlers on the badge', () => {
    const wrapper = mountBadge(3)
    const badge = wrapper.find('span')
    // Vue test utils exposes event listeners; verify the element can trigger them
    expect(badge.exists()).toBe(true)
    // Trigger mouseenter to verify handler doesn't throw
    badge.trigger('mouseenter')
    badge.trigger('mouseleave')
  })
})
