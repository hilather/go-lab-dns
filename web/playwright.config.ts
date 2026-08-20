import { defineConfig, devices } from '@playwright/test'

const host = process.env.LABDNS_E2E_HOST ?? '127.0.0.1'
const port = process.env.LABDNS_E2E_MGMT_PORT ?? '18765'
const baseURL = process.env.LABDNS_E2E_BASE_URL ?? `http://${host}:${port}`

export default defineConfig({
  testDir: './e2e',
  testMatch: '**/*.spec.ts',
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  timeout: 90_000,
  expect: { timeout: 15_000 },
  reporter: process.env.CI ? [['github'], ['list']] : 'list',
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'off',
    video: 'off',
  },
  webServer: {
    command: 'node e2e/start-labdns.mjs',
    url: `${baseURL}/v1/health/live`,
    reuseExistingServer: false,
    timeout: 180_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
