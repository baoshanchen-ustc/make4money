import path from 'node:path'
import { defineConfig, devices } from '@playwright/test'
import { deployRootDir, passkeyTestEnv, quoteForShell, repoRootDir, shellEnv } from './e2e/helpers/test-app'

const composeFile = path.join(deployRootDir, 'docker-compose.dev.yml')
const backendRootDir = path.join(repoRootDir, 'backend')
const deployDataDir = path.join(deployRootDir, 'data')
const deployPostgresDir = path.join(deployRootDir, 'postgres_data')
const deployRedisDir = path.join(deployRootDir, 'redis_data')

const backendComposeEnv = shellEnv({
  POSTGRES_PASSWORD: passkeyTestEnv.compose.postgresPassword,
  POSTGRES_BIND_HOST: '127.0.0.1',
  POSTGRES_PORT: '5432',
  POSTGRES_USER: 'sub2api',
  POSTGRES_DB: 'sub2api',
  REDIS_BIND_HOST: '127.0.0.1',
  REDIS_PORT: '6379',
  REDIS_DB: '0',
  ADMIN_EMAIL: passkeyTestEnv.admin.email,
  ADMIN_PASSWORD: passkeyTestEnv.admin.password,
  JWT_SECRET: passkeyTestEnv.compose.jwtSecret,
  TOTP_ENCRYPTION_KEY: passkeyTestEnv.compose.totpEncryptionKey,
  SERVER_PORT: String(passkeyTestEnv.backendPort),
  FRONTEND_BASE_URL: passkeyTestEnv.frontendBaseURL,
  E2E_USER_EMAIL: passkeyTestEnv.user.email,
  E2E_USER_PASSWORD: passkeyTestEnv.user.password,
  E2E_USER_NAME: passkeyTestEnv.user.username
})

const backendRuntimeEnv = shellEnv({
  AUTO_SETUP: 'true',
  DATA_DIR: deployDataDir,
  SERVER_HOST: '127.0.0.1',
  SERVER_PORT: String(passkeyTestEnv.backendPort),
  SERVER_MODE: 'debug',
  RUN_MODE: 'standard',
  DATABASE_HOST: '127.0.0.1',
  DATABASE_PORT: '5432',
  DATABASE_USER: 'sub2api',
  DATABASE_PASSWORD: passkeyTestEnv.compose.postgresPassword,
  DATABASE_DBNAME: 'sub2api',
  DATABASE_SSLMODE: 'disable',
  REDIS_HOST: '127.0.0.1',
  REDIS_PORT: '6379',
  REDIS_PASSWORD: '',
  REDIS_DB: '0',
  ADMIN_EMAIL: passkeyTestEnv.admin.email,
  ADMIN_PASSWORD: passkeyTestEnv.admin.password,
  JWT_SECRET: passkeyTestEnv.compose.jwtSecret,
  TOTP_ENCRYPTION_KEY: passkeyTestEnv.compose.totpEncryptionKey,
  FRONTEND_BASE_URL: passkeyTestEnv.frontendBaseURL,
  TZ: 'Asia/Shanghai'
})

const backendCommand = `bash -lc ${quoteForShell(
  `set -eu; server_pid=''; cleanup() { if [ -n "$server_pid" ]; then kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true; fi; ${backendComposeEnv} docker compose -f ${quoteForShell(composeFile)} down --remove-orphans --volumes || true; }; trap cleanup EXIT INT TERM; ${backendComposeEnv} docker compose -f ${quoteForShell(composeFile)} down --remove-orphans --volumes || true; rm -rf ${quoteForShell(deployDataDir)} ${quoteForShell(deployPostgresDir)} ${quoteForShell(deployRedisDir)}; mkdir -p ${quoteForShell(deployDataDir)} ${quoteForShell(deployPostgresDir)} ${quoteForShell(deployRedisDir)}; ${backendComposeEnv} docker compose -f ${quoteForShell(composeFile)} up -d --wait postgres redis; cd ${quoteForShell(backendRootDir)}; ${backendRuntimeEnv} go run ./cmd/server & server_pid=$!; wait "$server_pid"`
)}`

const frontendCommand = `bash -lc ${quoteForShell(
  `${shellEnv({
    VITE_DEV_PROXY_TARGET: passkeyTestEnv.backendBaseURL,
    VITE_DEV_PORT: String(passkeyTestEnv.frontendPort)
  })} pnpm exec vite --host 127.0.0.1 --port ${passkeyTestEnv.frontendPort} --strictPort`
)}`

export default defineConfig({
  testDir: './e2e',
  outputDir: 'test-results',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  timeout: 90 * 1000,
  expect: {
    timeout: 15 * 1000
  },
  reporter: [
    ['list'],
    ['html', { open: 'never', outputFolder: 'playwright-report' }]
  ],
  use: {
    baseURL: passkeyTestEnv.frontendBaseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure'
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome']
      }
    }
  ],
  webServer: [
    {
      command: backendCommand,
      url: `${passkeyTestEnv.backendBaseURL}/health`,
      name: 'Backend stack',
      timeout: 15 * 60 * 1000,
      reuseExistingServer: !process.env.CI,
      gracefulShutdown: {
        signal: 'SIGTERM',
        timeout: 5 * 1000
      }
    },
    {
      command: frontendCommand,
      url: passkeyTestEnv.frontendBaseURL,
      name: 'Frontend Vite',
      timeout: 5 * 60 * 1000,
      reuseExistingServer: !process.env.CI
    }
  ]
})
