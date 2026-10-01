import { defineConfig, devices } from '@playwright/test'

const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:8080'

/**
 * End-to-end tests against a full ReliefMesh stack (API + PostgreSQL + built
 * web app). Set E2E_START_SERVER=1 together with RELIEFMESH_E2E_DATABASE_URL
 * to let Playwright start a disposable stack via e2e/start-server.sh.
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : [['list']],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    serviceWorkers: 'allow',
  },
  projects: [
    { name: 'desktop-chromium', use: { ...devices['Desktop Chrome'] }, testIgnore: /mobile\.spec\.ts/ },
    { name: 'mobile-chromium', use: { ...devices['Pixel 7'] }, testMatch: /mobile\.spec\.ts/ },
  ],
  webServer: process.env.E2E_START_SERVER
    ? { command: './e2e/start-server.sh', url: `${baseURL}/readyz`, timeout: 180_000, reuseExistingServer: false, stdout: 'pipe' }
    : undefined,
})
