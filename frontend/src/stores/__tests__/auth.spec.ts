import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from '../auth'

// Mock authAPI
const mockLogin = vi.fn()
const mockLogin2FA = vi.fn()
const mockBeginPasskeyLogin = vi.fn()
const mockFinishPasskeyLogin = vi.fn()
const mockBeginPasskeyEnrollment = vi.fn()
const mockFinishPasskeyEnrollment = vi.fn()
const mockGetPasskeyStatus = vi.fn()
const mockListPasskeys = vi.fn()
const mockLogout = vi.fn()
const mockGetCurrentUser = vi.fn()
const mockRegister = vi.fn()
const mockRefreshToken = vi.fn()

vi.mock('@/api', () => ({
  authAPI: {
    login: (...args: any[]) => mockLogin(...args),
    login2FA: (...args: any[]) => mockLogin2FA(...args),
    beginPasskeyLogin: (...args: any[]) => mockBeginPasskeyLogin(...args),
    finishPasskeyLogin: (...args: any[]) => mockFinishPasskeyLogin(...args),
    beginPasskeyEnrollment: (...args: any[]) => mockBeginPasskeyEnrollment(...args),
    finishPasskeyEnrollment: (...args: any[]) => mockFinishPasskeyEnrollment(...args),
    getPasskeyStatus: (...args: any[]) => mockGetPasskeyStatus(...args),
    listPasskeys: (...args: any[]) => mockListPasskeys(...args),
    logout: (...args: any[]) => mockLogout(...args),
    getCurrentUser: (...args: any[]) => mockGetCurrentUser(...args),
    register: (...args: any[]) => mockRegister(...args),
    refreshToken: (...args: any[]) => mockRefreshToken(...args),
  },
  isTotp2FARequired: (response: any) => response?.requires_2fa === true,
}))

const fakeUser = {
  id: 1,
  username: 'testuser',
  email: 'test@example.com',
  role: 'user' as const,
  balance: 100,
  concurrency: 5,
  status: 'active' as const,
  allowed_groups: null,
  created_at: '2024-01-01',
  updated_at: '2024-01-01',
}

const fakeAdminUser = {
  ...fakeUser,
  id: 2,
  username: 'admin',
  email: 'admin@example.com',
  role: 'admin' as const,
}

const fakeAuthResponse = {
  access_token: 'test-token-123',
  refresh_token: 'refresh-token-456',
  expires_in: 3600,
  token_type: 'Bearer',
  user: { ...fakeUser },
}

const authStorageKeys = ['auth_token', 'auth_user', 'refresh_token', 'token_expires_at']

const fakePasskeyLoginBeginResponse = {
  flow_id: 'passkey-login-flow',
  countdown: 300,
  options: {
    publicKey: {
      challenge: 'login-challenge',
      userVerification: 'required' as const,
    },
  },
}

const fakePasskeyEnrollmentBeginResponse = {
  flow_id: 'passkey-enrollment-flow',
  countdown: 300,
  options: {
    publicKey: {
      challenge: 'enrollment-challenge',
      rp: {
        id: 'example.com',
        name: 'Example',
      },
      user: {
        id: 'user-1',
        name: 'test@example.com',
        displayName: 'testuser',
      },
      pubKeyCredParams: [{ type: 'public-key' as const, alg: -7 }],
      authenticatorSelection: {
        residentKey: 'required' as const,
        userVerification: 'required' as const,
      },
    },
  },
}

const fakePasskeyAssertion = {
  id: 'credential-login',
  rawId: 'credential-login',
  type: 'public-key' as const,
  authenticatorAttachment: 'platform' as const,
  clientExtensionResults: {},
  response: {
    clientDataJSON: 'client-data-json',
    authenticatorData: 'authenticator-data',
    signature: 'signature',
    userHandle: 'user-handle',
  },
}

const fakePasskeyRegistration = {
  id: 'credential-register',
  rawId: 'credential-register',
  type: 'public-key' as const,
  authenticatorAttachment: 'platform' as const,
  clientExtensionResults: {},
  response: {
    clientDataJSON: 'client-data-json',
    attestationObject: 'attestation-object',
    transports: ['internal' as const],
  },
}

const fakePasskeyStatus = {
  feature_enabled: true,
  can_manage: true,
  has_passkeys: true,
  active_count: 1,
  password_fallback_available: true,
}

const fakePasskeyCredential = {
  credential_id: 'credential-register',
  friendly_name: 'MacBook Pro',
  created_at: 1_711_111_111,
  last_used_at: 1_711_111_222,
  backup_eligible: true,
  synced: true,
}

function getPersistedStorageKeys(): string[] {
  const keys = Array.from({ length: localStorage.length }, (_, index) => localStorage.key(index))
    .filter((key): key is string => key !== null)
    .sort()

  return keys
}

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.useFakeTimers()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  // --- login ---

  describe('login', () => {
    it('成功登录后设置 token 和 user', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })

      expect(store.token).toBe('test-token-123')
      expect(store.user).toEqual(fakeUser)
      expect(store.isAuthenticated).toBe(true)
      expect(localStorage.getItem('auth_token')).toBe('test-token-123')
      expect(localStorage.getItem('auth_user')).toBe(JSON.stringify(fakeUser))
    })

    it('登录失败时清除状态并抛出错误', async () => {
      mockLogin.mockRejectedValue(new Error('Invalid credentials'))
      const store = useAuthStore()

      await expect(store.login({ email: 'test@example.com', password: 'wrong' })).rejects.toThrow(
        'Invalid credentials'
      )

      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })

    it('需要 2FA 时返回响应但不设置认证状态', async () => {
      const twoFAResponse = { requires_2fa: true, temp_token: 'temp-123' }
      mockLogin.mockResolvedValue(twoFAResponse)
      const store = useAuthStore()

      const result = await store.login({ email: 'test@example.com', password: '123456' })

      expect(result).toEqual(twoFAResponse)
      expect(store.token).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })
  })

  // --- login2FA ---

  describe('login2FA', () => {
    it('2FA 验证成功后设置认证状态', async () => {
      mockLogin2FA.mockResolvedValue(fakeAuthResponse)
      const store = useAuthStore()

      const user = await store.login2FA('temp-123', '654321')

      expect(store.token).toBe('test-token-123')
      expect(store.user).toEqual(fakeUser)
      expect(user).toEqual(fakeUser)
      expect(mockLogin2FA).toHaveBeenCalledWith({
        temp_token: 'temp-123',
        totp_code: '654321',
      })
    })

    it('2FA 验证失败时清除状态并抛出错误', async () => {
      mockLogin2FA.mockRejectedValue(new Error('Invalid TOTP'))
      const store = useAuthStore()

      await expect(store.login2FA('temp-123', '000000')).rejects.toThrow('Invalid TOTP')
      expect(store.token).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })
  })

  describe('passkey login', () => {
    it('成功后复用相同的认证持久化和刷新调度', async () => {
      vi.setSystemTime(new Date('2024-01-01T00:00:00Z'))
      mockBeginPasskeyLogin.mockResolvedValue(fakePasskeyLoginBeginResponse)
      mockFinishPasskeyLogin.mockResolvedValue({
        ...fakeAuthResponse,
        expires_in: 121,
      })
      mockRefreshToken.mockResolvedValue({
        access_token: 'refreshed-token',
        refresh_token: 'refreshed-refresh-token',
        expires_in: 3600,
      })

      const store = useAuthStore()

      await store.beginPasskeyLogin()
      expect(getPersistedStorageKeys()).toEqual([])

      const user = await store.loginWithPasskey(fakePasskeyAssertion)

      expect(user).toEqual(fakeUser)
      expect(store.token).toBe('test-token-123')
      expect(store.user).toEqual(fakeUser)
      expect(store.isAuthenticated).toBe(true)
      expect(mockFinishPasskeyLogin).toHaveBeenCalledWith('passkey-login-flow', fakePasskeyAssertion)
      expect(getPersistedStorageKeys()).toEqual(authStorageKeys)
      expect(localStorage.getItem('auth_token')).toBe('test-token-123')
      expect(localStorage.getItem('refresh_token')).toBe('refresh-token-456')
      expect(localStorage.getItem('auth_user')).toBe(JSON.stringify(fakeUser))
      expect(localStorage.getItem('token_expires_at')).not.toBeNull()

      await vi.advanceTimersByTimeAsync(1000)

      expect(mockRefreshToken).toHaveBeenCalledTimes(1)
    })
  })

  describe('passkey enrollment', () => {
    it('flow 状态只保存在内存中，不新增本地存储键', async () => {
      mockBeginPasskeyLogin.mockResolvedValue(fakePasskeyLoginBeginResponse)
      mockBeginPasskeyEnrollment.mockResolvedValue(fakePasskeyEnrollmentBeginResponse)
      mockFinishPasskeyEnrollment.mockResolvedValue({
        credential_id: 'credential-register',
        friendly_name: 'MacBook Pro',
      })
      mockGetPasskeyStatus.mockResolvedValue(fakePasskeyStatus)
      mockListPasskeys.mockResolvedValue({ items: [fakePasskeyCredential] })

      const store = useAuthStore()

      await store.beginPasskeyLogin()
      expect(getPersistedStorageKeys()).toEqual([])

      await store.beginPasskeyEnrollment()
      expect(getPersistedStorageKeys()).toEqual([])

      const result = await store.finishPasskeyEnrollment(fakePasskeyRegistration, 'MacBook Pro')

      expect(result).toEqual({
        credential_id: 'credential-register',
        friendly_name: 'MacBook Pro',
      })
      expect(mockFinishPasskeyEnrollment).toHaveBeenCalledWith(
        'passkey-enrollment-flow',
        fakePasskeyRegistration,
        'MacBook Pro'
      )
      expect(store.passkeyStatus).toEqual(fakePasskeyStatus)
      expect(store.passkeys).toEqual([fakePasskeyCredential])
      expect(getPersistedStorageKeys()).toEqual([])
    })
  })

  describe('passkey management refresh', () => {
    it('使用 status 接口中的 can_manage 作为 recent-auth 结果并刷新列表', async () => {
      const status = {
        ...fakePasskeyStatus,
        can_manage: false,
        active_count: 2,
      }
      const credentials = [
        fakePasskeyCredential,
        {
          ...fakePasskeyCredential,
          credential_id: 'credential-2',
          friendly_name: 'iPhone',
        },
      ]
      mockGetPasskeyStatus.mockResolvedValue(status)
      mockListPasskeys.mockResolvedValue({ items: credentials })

      const store = useAuthStore()
      const result = await store.refreshPasskeyManagement()

      expect(mockGetPasskeyStatus).toHaveBeenCalledTimes(1)
      expect(mockListPasskeys).toHaveBeenCalledTimes(1)
      expect(result).toEqual({
        status,
        passkeys: credentials,
      })
      expect(store.passkeyStatus).toEqual(status)
      expect(store.passkeys).toEqual(credentials)
      expect(store.passkeyStatus?.can_manage).toBe(false)
    })
  })

  // --- logout ---

  describe('logout', () => {
    it('注销后清除所有状态和 localStorage', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      mockLogout.mockResolvedValue(undefined)
      const store = useAuthStore()

      // 先登录
      await store.login({ email: 'test@example.com', password: '123456' })
      expect(store.isAuthenticated).toBe(true)

      // 注销
      await store.logout()

      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
      expect(localStorage.getItem('auth_token')).toBeNull()
      expect(localStorage.getItem('auth_user')).toBeNull()
      expect(localStorage.getItem('refresh_token')).toBeNull()
      expect(localStorage.getItem('token_expires_at')).toBeNull()
    })
  })

  // --- checkAuth ---

  describe('checkAuth', () => {
    it('从 localStorage 恢复持久化状态', () => {
      localStorage.setItem('auth_token', 'saved-token')
      localStorage.setItem('auth_user', JSON.stringify(fakeUser))

      // Mock refreshUser (getCurrentUser) 防止后台刷新报错
      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })

      const store = useAuthStore()
      store.checkAuth()

      expect(store.token).toBe('saved-token')
      expect(store.user).toEqual(fakeUser)
      expect(store.isAuthenticated).toBe(true)
    })

    it('localStorage 无数据时保持未认证状态', () => {
      const store = useAuthStore()
      store.checkAuth()

      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })

    it('localStorage 中用户数据损坏时清除状态', () => {
      localStorage.setItem('auth_token', 'saved-token')
      localStorage.setItem('auth_user', 'invalid-json{{{')

      const store = useAuthStore()
      store.checkAuth()

      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(localStorage.getItem('auth_token')).toBeNull()
    })

    it('恢复 refresh token 和过期时间', () => {
      const futureTs = String(Date.now() + 3600_000)
      localStorage.setItem('auth_token', 'saved-token')
      localStorage.setItem('auth_user', JSON.stringify(fakeUser))
      localStorage.setItem('refresh_token', 'saved-refresh')
      localStorage.setItem('token_expires_at', futureTs)

      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })

      const store = useAuthStore()
      store.checkAuth()

      expect(store.isAuthenticated).toBe(true)
    })
  })

  // --- isAdmin ---

  describe('isAdmin', () => {
    it('管理员用户返回 true', async () => {
      const adminResponse = { ...fakeAuthResponse, user: { ...fakeAdminUser } }
      mockLogin.mockResolvedValue(adminResponse)
      const store = useAuthStore()

      await store.login({ email: 'admin@example.com', password: '123456' })

      expect(store.isAdmin).toBe(true)
    })

    it('普通用户返回 false', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })

      expect(store.isAdmin).toBe(false)
    })

    it('未登录时返回 false', () => {
      const store = useAuthStore()
      expect(store.isAdmin).toBe(false)
    })
  })

  // --- refreshUser ---

  describe('refreshUser', () => {
    it('刷新用户数据并更新 localStorage', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      const store = useAuthStore()
      await store.login({ email: 'test@example.com', password: '123456' })

      const updatedUser = { ...fakeUser, username: 'updated-name' }
      mockGetCurrentUser.mockResolvedValue({ data: updatedUser })

      const result = await store.refreshUser()

      expect(result).toEqual(updatedUser)
      expect(store.user).toEqual(updatedUser)
      expect(JSON.parse(localStorage.getItem('auth_user')!)).toEqual(updatedUser)
    })

    it('未认证时抛出错误', async () => {
      const store = useAuthStore()
      await expect(store.refreshUser()).rejects.toThrow('Not authenticated')
    })
  })

  // --- isSimpleMode ---

  describe('isSimpleMode', () => {
    it('run_mode 为 simple 时返回 true', async () => {
      const simpleResponse = {
        ...fakeAuthResponse,
        user: { ...fakeUser, run_mode: 'simple' as const },
      }
      mockLogin.mockResolvedValue(simpleResponse)
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })

      expect(store.isSimpleMode).toBe(true)
    })

    it('默认为 standard 模式', () => {
      const store = useAuthStore()
      expect(store.isSimpleMode).toBe(false)
    })
  })
})
