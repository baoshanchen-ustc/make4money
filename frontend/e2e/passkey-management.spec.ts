import { expect, test } from '@playwright/test'
import {
  clearBrowserAuthState,
  gotoProfile,
  loginWithPassword,
  passkeyTestEnv
} from './helpers/test-app'
import { seedPasskeyTestUser } from './helpers/test-user-seed'
import { attachVirtualAuthenticator } from './helpers/virtual-authenticator'

async function enrollOnePasskey(page: Parameters<typeof gotoProfile>[0]): Promise<void> {
  await gotoProfile(page)
  await page.getByTestId('passkey-enroll-button').click()
  await expect(page.getByTestId('passkey-row')).toHaveCount(1)
}

async function revokeFirstPasskey(page: Parameters<typeof gotoProfile>[0]): Promise<void> {
  await page.getByTestId('passkey-revoke-button').click()
  await page.getByTestId('passkey-revoke-confirm-button').click()
  await expect(page.getByTestId('passkey-row')).toHaveCount(0)
}

test.beforeEach(async ({ request }) => {
  await seedPasskeyTestUser(request)
})

test('revokes an enrolled passkey', async ({ page }) => {
  const authenticator = await attachVirtualAuthenticator(page)

  await loginWithPassword(page)
  await enrollOnePasskey(page)
  expect(await authenticator.listCredentials()).toHaveLength(1)

  await revokeFirstPasskey(page)
})

test('rejects a revoked credential and still allows password fallback', async ({ page }) => {
  const authenticator = await attachVirtualAuthenticator(page)

  await loginWithPassword(page)
  await enrollOnePasskey(page)
  expect(await authenticator.listCredentials()).toHaveLength(1)

  await revokeFirstPasskey(page)
  expect(await authenticator.listCredentials()).toHaveLength(1)

  await clearBrowserAuthState(page)
  await page.getByTestId('passkey-login-button').click()

  await expect(page.getByTestId('passkey-login-error')).toHaveText(
    'Passkey login failed. Please try again or use another method.'
  )
  await expect(page.getByTestId('password-login-form')).toBeVisible()

  await page.locator('#email').fill(passkeyTestEnv.user.email)
  await page.locator('#password').fill(passkeyTestEnv.user.password)
  await page.getByTestId('password-login-form').locator('button[type="submit"]').click()

  await page.waitForURL(/\/dashboard(?:\?.*)?$/)
  await page.waitForLoadState('networkidle')
})
