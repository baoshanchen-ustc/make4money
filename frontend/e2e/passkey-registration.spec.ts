import { expect, test } from '@playwright/test'
import { gotoProfile, loginWithPassword } from './helpers/test-app'
import { seedPasskeyTestUser } from './helpers/test-user-seed'
import { attachVirtualAuthenticator } from './helpers/virtual-authenticator'

test.beforeEach(async ({ request }) => {
  await seedPasskeyTestUser(request)
})

test('enrolls the first passkey', async ({ page }) => {
  const authenticator = await attachVirtualAuthenticator(page)

  await loginWithPassword(page)
  await gotoProfile(page)

  await expect(page.getByTestId('passkey-row')).toHaveCount(0)
  await page.getByTestId('passkey-enroll-button').click()

  await expect(page.getByTestId('passkey-row')).toHaveCount(1)
  expect(await authenticator.listCredentials()).toHaveLength(1)
})

test('enrolls a second passkey', async ({ page }) => {
  const firstAuthenticator = await attachVirtualAuthenticator(page)

  await loginWithPassword(page)
  await gotoProfile(page)
  await page.getByTestId('passkey-enroll-button').click()

  await expect(page.getByTestId('passkey-row')).toHaveCount(1)
  expect(await firstAuthenticator.listCredentials()).toHaveLength(1)

  await firstAuthenticator.remove()

  const secondAuthenticator = await attachVirtualAuthenticator(page)
  await page.getByTestId('passkey-enroll-button').click()

  await expect(page.getByTestId('passkey-row')).toHaveCount(2)
  expect(await secondAuthenticator.listCredentials()).toHaveLength(1)
})
