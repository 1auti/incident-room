import { defineConfig } from '@playwright/test'

function required(name: string): string {
  const value = process.env[name]
  if (!value) {
    throw new Error(`${name} is required; run the suite through "make e2e" (scripts/e2e.sh)`)
  }
  return value
}

const databaseUrl = required('DATABASE_URL')
const adminEmail = required('ADMIN_EMAIL')
const adminPassword = required('ADMIN_PASSWORD')
const apiPort = process.env.E2E_API_PORT ?? '18080'
const webPort = process.env.E2E_WEB_PORT ?? '15173'
const apiUrl = `http://127.0.0.1:${apiPort}`
const webUrl = `http://127.0.0.1:${webPort}`

export default defineConfig({
  testDir: '.',
  testMatch: '*.spec.ts',
  fullyParallel: true,
  reporter: 'list',
  use: { baseURL: webUrl },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
  webServer: [
    {
      name: 'backend',
      command: 'go run ./cmd/api',
      cwd: '../backend',
      env: {
        DATABASE_URL: databaseUrl,
        PORT: apiPort,
        ADMIN_EMAIL: adminEmail,
        ADMIN_PASSWORD: adminPassword,
      },
      // 401 counts as ready: the server is up and unauthenticated.
      url: `${apiUrl}/api/me`,
      reuseExistingServer: false,
      timeout: 120_000,
    },
    {
      name: 'frontend',
      command: `npm run dev -- --host 127.0.0.1 --port ${webPort} --strictPort`,
      cwd: '../frontend',
      env: { API_PROXY_TARGET: apiUrl },
      url: webUrl,
      reuseExistingServer: false,
      timeout: 120_000,
    },
  ],
})
