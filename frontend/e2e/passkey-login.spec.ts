import { expect, test } from '@playwright/test'
import {
  clearBrowserAuthState,
  gotoProfile,
  loginWithPassword,
  passkeyTestEnv
} from './helpers/test-app'
import { seedPasskeyTestUser } from './helpers/test-user-seed'
import { attachVirtualAuthenticator } from './helpers/virtual-authenticator'

test.beforeEach(async ({ request }) => {
  await seedPasskeyTestUser(request)
})

test('signs in usernamelessly with a passkey', async ({ page }) => {
  const authenticator = await attachVirtualAuthenticator(page)

  await loginWithPassword(page)
  await gotoProfile(page)
  await page.getByTestId('passkey-enroll-button').click()

  await expect(page.getByTestId('passkey-row')).toHaveCount(1)
  expect(await authenticator.listCredentials()).toHaveLength(1)

  await clearBrowserAuthState(page)
  await expect(page.getByTestId('passkey-login-button')).toBeVisible()

  await page.getByTestId('passkey-login-button').click()

  await page.waitForURL(/\/dashboard(?:\?.*)?$/)
  await page.waitForLoadState('networkidle')
})

test('keeps password fallback available after enrollment', async ({ page }) => {
  const authenticator = await attachVirtualAuthenticator(page)

  await loginWithPassword(page)
  await gotoProfile(page)
  await page.getByTestId('passkey-enroll-button').click()

  await expect(page.getByTestId('passkey-row')).toHaveCount(1)
  expect(await authenticator.listCredentials()).toHaveLength(1)

  await clearBrowserAuthState(page)
  await expect(page.getByTestId('passkey-login-button')).toBeVisible()
  await expect(page.getByTestId('password-login-form')).toBeVisible()

  await page.locator('#email').fill(passkeyTestEnv.user.email)
  await page.locator('#password').fill(passkeyTestEnv.user.password)
  await page.getByTestId('password-login-form').locator('button[type="submit"]').click()

  await page.waitForURL(/\/dashboard(?:\?.*)?$/)
  await page.waitForLoadState('networkidle')
})
