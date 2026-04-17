import { beforeEach, describe, expect, it, vi } from 'vitest'

const postMock = vi.fn()

vi.mock('@/api/client', () => ({
  apiClient: {
    post: (...args: unknown[]) => postMock(...args)
  }
}))

describe('auth oauth helpers', () => {
  beforeEach(() => {
    vi.resetModules()
    postMock.mockReset()
  })

  it('bindOAuthLogin posts to the provider bind-login endpoint', async () => {
    postMock.mockResolvedValue({
      data: {
        access_token: 'access-token',
        refresh_token: 'refresh-token',
        expires_in: 3600,
        token_type: 'Bearer'
      }
    })

    const { bindOAuthLogin } = await import('@/api/auth')

    const response = await bindOAuthLogin('wechat', {
      pendingOAuthToken: 'pending-token',
      email: 'user@example.com',
      password: 'secret',
      turnstileToken: 'turnstile-token'
    })

    expect(postMock).toHaveBeenCalledWith('/auth/oauth/wechat/bind-login', {
      pending_oauth_token: 'pending-token',
      pending_login_token: 'pending-token',
      email: 'user@example.com',
      password: 'secret',
      turnstile_token: 'turnstile-token'
    })
    expect(response.access_token).toBe('access-token')
  })

  it('createOAuthAccount uses the unified create-account endpoint for LinuxDo', async () => {
    postMock.mockResolvedValue({
      data: {
        access_token: 'access-token',
        refresh_token: 'refresh-token',
        expires_in: 3600,
        token_type: 'Bearer'
      }
    })

    const { createOAuthAccount } = await import('@/api/auth')

    await createOAuthAccount('linuxdo', {
      pendingOAuthToken: 'pending-token',
      invitationCode: 'invite-123'
    })

    expect(postMock).toHaveBeenCalledWith('/auth/oauth/linuxdo/create-account', {
      pending_oauth_token: 'pending-token',
      pending_login_token: 'pending-token',
      email: undefined,
      verify_code: undefined,
      invitation_code: 'invite-123'
    })
  })

  it('createOAuthAccount uses the unified create-account endpoint for WeChat', async () => {
    postMock.mockResolvedValue({
      data: {
        access_token: 'access-token',
        token_type: 'Bearer'
      }
    })

    const { createOAuthAccount } = await import('@/api/auth')

    await createOAuthAccount('wechat', {
      pendingOAuthToken: 'pending-token'
    })

    expect(postMock).toHaveBeenCalledWith('/auth/oauth/wechat/create-account', {
      pending_oauth_token: 'pending-token',
      pending_login_token: 'pending-token',
      email: undefined,
      verify_code: undefined,
      invitation_code: undefined
    })
  })
})
