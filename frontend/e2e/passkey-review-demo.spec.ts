import { expect, test } from '@playwright/test'
import {
  clearBrowserAuthState,
  gotoProfile,
  loginWithPassword,
  passkeyTestEnv
} from './helpers/test-app'
import { seedPasskeyTestUser } from './helpers/test-user-seed'
import { attachVirtualAuthenticator } from './helpers/virtual-authenticator'

test.use({
  viewport: { width: 1728, height: 1117 },
  video: {
    mode: 'on',
    size: { width: 1728, height: 1117 }
  }
})

test.beforeEach(async ({ request }) => {
  await seedPasskeyTestUser(request)
})

test('record passkey demo light', async ({ page }) => {
  await attachVirtualAuthenticator(page)

  await loginWithPassword(page)
  
  await gotoProfile(page)
  await page.waitForTimeout(1000)
  await page.getByTestId('passkey-enroll-button').click()
  await expect(page.getByTestId('passkey-row')).toHaveCount(1)
  await page.waitForTimeout(1000)

  await page.getByRole('button', { name: 'Rename' }).click()
  await page.waitForTimeout(1000)
  await page.getByRole('button', { name: 'Cancel' }).click()
  await page.waitForTimeout(1000)

  await clearBrowserAuthState(page)
  await page.waitForTimeout(1000)

  await expect(page.getByTestId('passkey-login-button')).toBeVisible()
  await page.getByTestId('passkey-login-button').click()
  await page.waitForURL(/\/dashboard(?:\?.*)?$/)
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(2000)
})

test('record passkey demo dark', async ({ page }) => {
  await attachVirtualAuthenticator(page)

  await page.goto(passkeyTestEnv.routes.login)
  await page.evaluate(() => {
    document.documentElement.classList.add('dark')
    localStorage.setItem('theme', 'dark')
  })

  await loginWithPassword(page)
  
  await gotoProfile(page)
  await page.waitForTimeout(1000)
  await page.getByTestId('passkey-enroll-button').click()
  await expect(page.getByTestId('passkey-row')).toHaveCount(1)
  await page.waitForTimeout(1000)

  await page.getByRole('button', { name: 'Rename' }).click()
  await page.waitForTimeout(1000)
  await page.getByRole('button', { name: 'Cancel' }).click()
  await page.waitForTimeout(1000)

  await clearBrowserAuthState(page)
  await page.evaluate(() => {
    document.documentElement.classList.add('dark')
    localStorage.setItem('theme', 'dark')
  })
  await page.waitForTimeout(1000)

  await expect(page.getByTestId('passkey-login-button')).toBeVisible()
  await page.getByTestId('passkey-login-button').click()
  await page.waitForURL(/\/dashboard(?:\?.*)?$/)
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(2000)
})
