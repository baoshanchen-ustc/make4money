import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, type Page } from '@playwright/test'

export interface PasskeyTestUserCredentials {
  email: string
  password: string
  username: string
}

export interface PasskeyTestEnvironment {
  frontendBaseURL: string
  frontendOrigin: string
  frontendPort: number
  backendBaseURL: string
  backendOrigin: string
  backendPort: number
  admin: PasskeyTestUserCredentials
  user: PasskeyTestUserCredentials
  compose: {
    postgresPassword: string
    jwtSecret: string
    totpEncryptionKey: string
  }
  routes: {
    login: string
    profile: string
    dashboard: string
  }
}

const frontendURL = new URL(process.env.FRONTEND_BASE_URL ?? 'http://localhost:3000')
const backendURL = new URL(process.env.BASE_URL ?? 'http://localhost:8080')

const helperFilePath = fileURLToPath(import.meta.url)
const helperDir = path.dirname(helperFilePath)

function resolvePort(url: URL): number {
  if (url.port) {
    return Number(url.port)
  }

  return url.protocol === 'https:' ? 443 : 80
}

export const frontendRootDir = path.resolve(helperDir, '../..')
export const repoRootDir = path.resolve(frontendRootDir, '..')
export const deployRootDir = path.join(repoRootDir, 'deploy')

export const passkeyTestEnv: PasskeyTestEnvironment = {
  frontendBaseURL: frontendURL.origin,
  frontendOrigin: frontendURL.origin,
  frontendPort: resolvePort(frontendURL),
  backendBaseURL: backendURL.origin,
  backendOrigin: backendURL.origin,
  backendPort: resolvePort(backendURL),
  admin: {
    email: process.env.ADMIN_EMAIL ?? 'admin@sub2api.local',
    password: process.env.ADMIN_PASSWORD ?? 'AdminPasskey@12345',
    username: 'admin'
  },
  user: {
    email: process.env.E2E_USER_EMAIL ?? 'e2e-passkey-user@sub2api.local',
    password: process.env.E2E_USER_PASSWORD ?? 'E2ePasskey@12345',
    username: process.env.E2E_USER_NAME ?? 'e2e-passkey-user'
  },
  compose: {
    postgresPassword: process.env.POSTGRES_PASSWORD ?? 'passkey-postgres-password',
    jwtSecret: process.env.JWT_SECRET ?? '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
    totpEncryptionKey: process.env.TOTP_ENCRYPTION_KEY ?? 'abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789'
  },
  routes: {
    login: '/login',
    profile: '/profile',
    dashboard: '/dashboard'
  }
}

export function quoteForShell(value: string): string {
  return `'${value.replace(/'/g, `'"'"'`)}'`
}

export function shellEnv(env: Record<string, string>): string {
  return Object.entries(env)
    .map(([key, value]) => `${key}=${quoteForShell(value)}`)
    .join(' ')
}

export async function gotoLogin(page: Page): Promise<void> {
  await page.goto(passkeyTestEnv.routes.login)
  await page.waitForLoadState('networkidle')
  await expect(page.locator('#email')).toBeVisible()
}

export async function gotoProfile(page: Page): Promise<void> {
  await page.goto(passkeyTestEnv.routes.profile)
  await page.waitForLoadState('networkidle')
  await expect(page.getByTestId('passkey-enroll-button')).toBeVisible()
}

export async function loginWithPassword(
  page: Page,
  user: PasskeyTestUserCredentials = passkeyTestEnv.user
): Promise<void> {
  await gotoLogin(page)
  await page.locator('#email').fill(user.email)
  await page.locator('#password').fill(user.password)
  await page.getByTestId('password-login-form').locator('button[type="submit"]').click()
  await page.waitForURL(/\/dashboard(?:\?.*)?$/)
  await page.waitForLoadState('networkidle')
}

export async function clearBrowserAuthState(page: Page): Promise<void> {
  await page.evaluate(() => {
    localStorage.clear()
    sessionStorage.clear()
  })
  await gotoLogin(page)
}
