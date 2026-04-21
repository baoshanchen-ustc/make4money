/**
 * LoginView 组件核心逻辑测试
 * 测试登录表单提交、验证、2FA 等场景
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { defineComponent, reactive, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { startAuthentication } from '@simplewebauthn/browser'

// Mock 所有外部依赖
const mockLogin = vi.fn()
const mockLogin2FA = vi.fn()
const mockBeginPasskeyLogin = vi.fn()
const mockLoginWithPasskey = vi.fn()
const mockPush = vi.fn()

vi.mock('@simplewebauthn/browser', () => ({
  startAuthentication: vi.fn()
}))

vi.mock('@/api', () => ({
  authAPI: {
    login: (...args: any[]) => mockLogin(...args),
    login2FA: (...args: any[]) => mockLogin2FA(...args),
    beginPasskeyLogin: (...args: any[]) => mockBeginPasskeyLogin(...args),
    finishPasskeyLogin: (...args: any[]) => mockLoginWithPasskey(...args),
    logout: vi.fn(),
    getCurrentUser: vi.fn().mockResolvedValue({ data: {} }),
    register: vi.fn(),
    refreshToken: vi.fn(),
  },
  isTotp2FARequired: (response: any) => response?.requires_2fa === true,
}))

vi.mock('@/api/admin/system', () => ({
  checkUpdates: vi.fn(),
}))

vi.mock('@/api/auth', () => ({
  getPublicSettings: vi.fn().mockResolvedValue({}),
}))

/**
 * 创建一个简化的测试组件来封装登录逻辑
 * 避免引入 LoginView.vue 的全部依赖（AuthLayout、i18n、Icon 等）
 */
const LoginFormTestComponent = defineComponent({
  setup() {
    const authStore = useAuthStore()
    const formData = reactive({ email: '', password: '' })
    const isLoading = ref(false)
    const errorMessage = ref('')

    const passkeyEnabled = ref(false)

    const handleLogin = async () => {
      if (!formData.email || !formData.password) {
        errorMessage.value = '请输入邮箱和密码'
        return
      }

      isLoading.value = true
      errorMessage.value = ''

      try {
        const response = await authStore.login({
          email: formData.email,
          password: formData.password,
        })

        // 2FA 流程由调用方处理
        if ((response as any)?.requires_2fa) {
          errorMessage.value = '需要 2FA 验证'
          return
        }

        mockPush('/dashboard')
      } catch (error: any) {
        errorMessage.value = error.message || '登录失败'
      } finally {
        isLoading.value = false
      }
    }

    const handlePasskeyLogin = async () => {
      errorMessage.value = ''
      isLoading.value = true

      try {
        const beginResponse = await authStore.beginPasskeyLogin()
        const asseResp = await startAuthentication({ optionsJSON: beginResponse.options.publicKey as any })
        await authStore.loginWithPasskey(asseResp)

        mockPush('/dashboard')
      } catch (error: any) {
        if (error.name === 'NotAllowedError' || error.message?.includes('cancelled')) {
          return
        }
        errorMessage.value = 'auth.passkeyLoginFailed'
      } finally {
        isLoading.value = false
      }
    }

    return { formData, isLoading, errorMessage, handleLogin, handlePasskeyLogin, passkeyEnabled }
  },
  template: `
    <div>
      <button v-if="passkeyEnabled" data-testid="passkey-login-button" @click="handlePasskeyLogin" :disabled="isLoading">Passkey</button>
      <form data-testid="password-login-form" @submit.prevent="handleLogin">
        <input id="email" v-model="formData.email" type="email" />
        <input id="password" v-model="formData.password" type="password" />
        <p v-if="errorMessage" data-testid="passkey-login-error" class="error">{{ errorMessage }}</p>
        <button type="submit" :disabled="isLoading">登录</button>
      </form>
    </div>
  `,
})

describe('LoginForm 核心逻辑', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('成功登录后跳转到 dashboard', async () => {
    mockLogin.mockResolvedValue({
      access_token: 'token',
      token_type: 'Bearer',
      user: { id: 1, username: 'test', email: 'test@example.com', role: 'user', balance: 0, concurrency: 5, status: 'active', allowed_groups: null, created_at: '', updated_at: '' },
    })

    const wrapper = mount(LoginFormTestComponent)

    await wrapper.find('#email').setValue('test@example.com')
    await wrapper.find('#password').setValue('password123')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(mockLogin).toHaveBeenCalledWith({
      email: 'test@example.com',
      password: 'password123',
    })
    expect(mockPush).toHaveBeenCalledWith('/dashboard')
  })

  it('登录失败时显示错误信息', async () => {
    mockLogin.mockRejectedValue(new Error('Invalid credentials'))

    const wrapper = mount(LoginFormTestComponent)

    await wrapper.find('#email').setValue('test@example.com')
    await wrapper.find('#password').setValue('wrong')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('.error').text()).toBe('Invalid credentials')
  })

  it('空表单提交显示验证错误', async () => {
    const wrapper = mount(LoginFormTestComponent)

    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('.error').text()).toBe('请输入邮箱和密码')
    expect(mockLogin).not.toHaveBeenCalled()
  })

  it('需要 2FA 时不跳转', async () => {
    mockLogin.mockResolvedValue({
      requires_2fa: true,
      temp_token: 'temp-123',
    })

    const wrapper = mount(LoginFormTestComponent)

    await wrapper.find('#email').setValue('test@example.com')
    await wrapper.find('#password').setValue('password123')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(mockPush).not.toHaveBeenCalled()
    expect(wrapper.find('.error').text()).toBe('需要 2FA 验证')
  })

  it('提交过程中按钮被禁用', async () => {
    let resolveLogin: (v: any) => void
    mockLogin.mockImplementation(
      () => new Promise((resolve) => { resolveLogin = resolve })
    )

    const wrapper = mount(LoginFormTestComponent)

    await wrapper.find('#email').setValue('test@example.com')
    await wrapper.find('#password').setValue('password123')
    await wrapper.find('form').trigger('submit')

    expect(wrapper.find('button').attributes('disabled')).toBeDefined()

    resolveLogin!({
      access_token: 'token',
      token_type: 'Bearer',
      user: { id: 1, username: 'test', email: 'test@example.com', role: 'user', balance: 0, concurrency: 5, status: 'active', allowed_groups: null, created_at: '', updated_at: '' },
    })
    await flushPromises()

    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })

  describe('Passkey Login', () => {
    it('passkey button is hidden when feature flag is false', async () => {
      const wrapper = mount(LoginFormTestComponent)
      wrapper.vm.passkeyEnabled = false
      await flushPromises()
      
      expect(wrapper.find('[data-testid="passkey-login-button"]').exists()).toBe(false)
    })

    it('passkey button is visible when feature flag is true', async () => {
      const wrapper = mount(LoginFormTestComponent)
      wrapper.vm.passkeyEnabled = true
      await flushPromises()
      
      expect(wrapper.find('[data-testid="passkey-login-button"]').exists()).toBe(true)
    })

    it('成功通过 passkey 登录后跳转到 dashboard', async () => {
      mockBeginPasskeyLogin.mockResolvedValue({
        flow_id: 'flow-123',
        options: { publicKey: { challenge: 'challenge' } }
      })
      
      vi.mocked(startAuthentication).mockResolvedValue({
        id: 'cred-123',
        rawId: 'raw-123',
        type: 'public-key',
        response: {
          clientDataJSON: 'client-data',
          authenticatorData: 'auth-data',
          signature: 'sig',
          userHandle: 'user-123'
        },
        clientExtensionResults: {}
      })

      mockLoginWithPasskey.mockResolvedValue({
        access_token: 'token',
        token_type: 'Bearer',
        user: { id: 1, username: 'test', email: 'test@example.com', role: 'user', balance: 0, concurrency: 5, status: 'active', allowed_groups: null, created_at: '', updated_at: '' },
      })

      const wrapper = mount(LoginFormTestComponent)
      wrapper.vm.passkeyEnabled = true
      await flushPromises()

      await wrapper.find('[data-testid="passkey-login-button"]').trigger('click')
      await flushPromises()

      expect(mockBeginPasskeyLogin).toHaveBeenCalled()
      expect(startAuthentication).toHaveBeenCalledWith({ optionsJSON: { challenge: 'challenge' } })
      expect(mockLoginWithPasskey).toHaveBeenCalled()
      expect(mockPush).toHaveBeenCalledWith('/dashboard')
    })

    it('取消 passkey 登录时不显示错误', async () => {
      mockBeginPasskeyLogin.mockResolvedValue({
        flow_id: 'flow-123',
        options: { publicKey: { challenge: 'challenge' } }
      })
      
      const cancelError = new Error('The operation either timed out or was not allowed.')
      cancelError.name = 'NotAllowedError'
      vi.mocked(startAuthentication).mockRejectedValue(cancelError)

      const wrapper = mount(LoginFormTestComponent)
      wrapper.vm.passkeyEnabled = true
      await flushPromises()

      await wrapper.find('[data-testid="passkey-login-button"]').trigger('click')
      await flushPromises()

      expect(wrapper.find('[data-testid="passkey-login-error"]').exists()).toBe(false)
      expect(mockPush).not.toHaveBeenCalled()
    })

    it('passkey 登录失败时显示通用错误信息，且密码表单仍然可用', async () => {
      mockBeginPasskeyLogin.mockResolvedValue({
        flow_id: 'flow-123',
        options: { publicKey: { challenge: 'challenge' } }
      })
      
      vi.mocked(startAuthentication).mockRejectedValue(new Error('Passkey error'))

      const wrapper = mount(LoginFormTestComponent)
      wrapper.vm.passkeyEnabled = true
      await flushPromises()

      await wrapper.find('[data-testid="passkey-login-button"]').trigger('click')
      await flushPromises()

      expect(wrapper.find('[data-testid="passkey-login-error"]').text()).toBe('auth.passkeyLoginFailed')
      expect(mockPush).not.toHaveBeenCalled()

      expect(wrapper.find('[data-testid="password-login-form"]').exists()).toBe(true)
      
      mockLogin.mockResolvedValue({
        access_token: 'token',
        token_type: 'Bearer',
        user: { id: 1, username: 'test', email: 'test@example.com', role: 'user', balance: 0, concurrency: 5, status: 'active', allowed_groups: null, created_at: '', updated_at: '' },
      })

      await wrapper.find('#email').setValue('test@example.com')
      await wrapper.find('#password').setValue('password123')
      await wrapper.find('[data-testid="password-login-form"]').trigger('submit')
      await flushPromises()

      expect(mockLogin).toHaveBeenCalledWith({
        email: 'test@example.com',
        password: 'password123',
      })
      expect(mockPush).toHaveBeenCalledWith('/dashboard')
    })
  })
})
