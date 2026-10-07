import { randomUUID } from 'node:crypto'
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'
import pg from 'pg'

// DEBT: the states beyond "declarado" are seeded with SQL straight into the e2e database
// (DATABASE_URL, exported by `make e2e`, which is disposable). This is a deliberate, e2e-only use
// of `pg`: no API can move an incident through the lifecycle beyond T1 yet. When UC-06 (state
// transitions) exists, replace seedState with API calls and adjust these tests.
// Escalation is real since UC-04: the backend ticker escalates incidents whose declared_at is
// moved into the past (seedExpired).

const adminEmail = process.env.ADMIN_EMAIL ?? ''
const adminPassword = process.env.ADMIN_PASSWORD ?? ''
const databaseUrl = process.env.DATABASE_URL ?? ''

type PlaywrightLike = { request: { newContext: (o: { baseURL?: string }) => Promise<APIRequestContext> } }

interface ServiceDto {
  id: string
  name: string
}

interface UserDto {
  id: string
  email: string
}

interface IncidentDto {
  id: string
  title: string
  severity: string
  state: string
  escalated_at: string | null
}

function uniqueName(): string {
  return `svc-${randomUUID()}`
}

async function loginViaApi(ctx: APIRequestContext, email: string, password: string): Promise<void> {
  const res = await ctx.post('/api/auth/login', { data: { email, password } })
  expect(res.status()).toBe(200)
}

async function adminContext(playwright: PlaywrightLike, baseURL: string | undefined) {
  const ctx = await playwright.request.newContext({ baseURL })
  await loginViaApi(ctx, adminEmail, adminPassword)
  return ctx
}

async function createService(admin: APIRequestContext, criticality = 'importante'): Promise<ServiceDto> {
  const res = await admin.post('/api/services', { data: { name: uniqueName(), criticality } })
  expect(res.status()).toBe(201)
  return (await res.json()) as ServiceDto
}

async function registerUser(
  playwright: PlaywrightLike,
  baseURL: string | undefined,
  admin: APIRequestContext,
  role: 'ingeniero' | 'oncall' | 'admin',
): Promise<{ email: string; user: UserDto }> {
  const email = `user-${randomUUID()}@e2e.test`
  const anon = await playwright.request.newContext({ baseURL })
  const reg = await anon.post('/api/auth/register', { data: { name: 'E2E User', email, password: 'password-e2e' } })
  expect(reg.status()).toBe(201)
  const user = (await reg.json()) as UserDto
  await anon.dispose()
  if (role !== 'ingeniero') {
    const res = await admin.patch(`/api/users/${user.id}/role`, { data: { role } })
    expect(res.status()).toBe(200)
  }
  return { email, user }
}

async function declare(
  ctx: APIRequestContext,
  serviceId: string,
  severity: 'SEV1' | 'SEV2' | 'SEV3' = 'SEV2',
): Promise<IncidentDto> {
  const res = await ctx.post('/api/incidents', {
    data: { title: `inc-${randomUUID()}`, service_id: serviceId, impact: 'degradacion', severity },
  })
  expect(res.status()).toBe(201)
  return ((await res.json()) as { incident: IncidentDto }).incident
}

async function withDb(run: (client: pg.Client) => Promise<void>): Promise<void> {
  const client = new pg.Client({ connectionString: databaseUrl })
  await client.connect()
  try {
    await run(client)
  } finally {
    await client.end()
  }
}

async function seedState(id: string, state: string): Promise<void> {
  await withDb(async (c) => {
    const res = await c.query('UPDATE incidents SET state = $1 WHERE id = $2', [state, id])
    expect(res.rowCount).toBe(1)
  })
}

// Makes the SLA of a SEV2 incident (15 minutes) expire; the backend ticker escalates it (BR-04).
async function seedExpired(ctx: APIRequestContext, id: string): Promise<void> {
  await withDb(async (c) => {
    const res = await c.query("UPDATE incidents SET declared_at = now() - interval '16 minutes' WHERE id = $1", [id])
    expect(res.rowCount).toBe(1)
  })
  await expect
    .poll(async () => ((await (await ctx.get(`/api/incidents/${id}`)).json()) as IncidentDto).escalated_at, { timeout: 15_000 })
    .not.toBeNull()
}

async function loginUi(page: Page, email: string): Promise<void> {
  await page.goto('/')
  await page.getByTestId('login-email').fill(email)
  await page.getByTestId('login-password').fill('password-e2e')
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('incident-board')).toBeVisible()
}

// Changes a board filter and waits for the backend answer to that exact filter.
async function applyFilter(page: Page, testId: string, param: string, value: string): Promise<void> {
  await expect(page.getByTestId(testId).locator(`option[value="${value}"]`)).toBeAttached()
  await Promise.all([
    page.waitForResponse((r) => {
      const url = new URL(r.url())
      return url.pathname === '/api/incidents' && (url.searchParams.get(param) ?? '') === value
    }),
    page.getByTestId(testId).selectOption(value),
  ])
}

function row(page: Page, id: string) {
  return page.locator(`[data-testid="board-incident-row"][data-incident-id="${id}"]`)
}

test('UC-03.1 a resolved incident is listed and a closed one is not (BR-15)', async ({ page, playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const svc = await createService(admin)
  const resolved = await declare(admin, svc.id)
  const closed = await declare(admin, svc.id)
  await seedState(resolved.id, 'resuelto')
  await seedState(closed.id, 'cerrado')

  const { email } = await registerUser(playwright, baseURL, admin, 'ingeniero')
  await loginUi(page, email)
  await expect(row(page, resolved.id)).toBeVisible()
  await expect(row(page, resolved.id).getByTestId('board-incident-state')).toHaveText('resuelto')
  await expect(row(page, closed.id)).toHaveCount(0)
  await admin.dispose()
})

test('UC-03.2 filters combine with AND (BR-15)', async ({ page, playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const svcA = await createService(admin)
  const svcB = await createService(admin)
  const sev1 = await declare(admin, svcA.id, 'SEV1')
  const sev2 = await declare(admin, svcA.id, 'SEV2')
  const aMitigating = await declare(admin, svcA.id, 'SEV3')
  const bMitigating = await declare(admin, svcB.id, 'SEV3')
  const bDeclared = await declare(admin, svcB.id, 'SEV3')
  await seedState(aMitigating.id, 'mitigando')
  await seedState(bMitigating.id, 'mitigando')

  const { email } = await registerUser(playwright, baseURL, admin, 'ingeniero')
  await loginUi(page, email)
  await expect(row(page, sev1.id)).toBeVisible()
  await expect(row(page, sev2.id)).toBeVisible()

  // Severity only.
  await applyFilter(page, 'board-filter-severity', 'severity', 'SEV1')
  await expect(row(page, sev1.id)).toBeVisible()
  await expect(row(page, sev2.id)).toHaveCount(0)
  const severities = await page.getByTestId('board-incident-severity').allTextContents()
  expect(severities.length).toBeGreaterThan(0)
  expect(severities.every((s) => s === 'SEV1')).toBe(true)

  // Service and state together.
  await applyFilter(page, 'board-filter-severity', 'severity', '')
  await applyFilter(page, 'board-filter-service', 'service_id', svcB.id)
  await applyFilter(page, 'board-filter-state', 'state', 'mitigando')
  await expect(row(page, bMitigating.id)).toBeVisible()
  await expect(row(page, bDeclared.id)).toHaveCount(0)
  await expect(row(page, aMitigating.id)).toHaveCount(0)
  await expect(page.getByTestId('board-incident-row')).toHaveCount(1)
  await expect(page.getByTestId('board-incident-state')).toHaveText('mitigando')
  await expect(page.getByTestId('board-incident-service')).toHaveText(svcB.name)
  await admin.dispose()
})

test('UC-03.3 a filter without matches shows the empty state, not an error', async ({ page, playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const empty = await createService(admin)
  const { email } = await registerUser(playwright, baseURL, admin, 'ingeniero')
  await loginUi(page, email)

  await applyFilter(page, 'board-filter-service', 'service_id', empty.id)
  await expect(page.getByTestId('board-empty')).toBeVisible()
  await expect(page.getByTestId('board-error')).toHaveCount(0)
  await expect(page.getByTestId('board-incident-row')).toHaveCount(0)
  await admin.dispose()
})

test('UC-03.4 only the SLA-escalated incident shows the Escalado indicator (BR-04)', async ({ page, playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const svc = await createService(admin)
  const escalated = await declare(admin, svc.id)
  const normal = await declare(admin, svc.id)
  await seedExpired(admin, escalated.id)

  const { email } = await registerUser(playwright, baseURL, admin, 'ingeniero')
  await loginUi(page, email)
  await expect(row(page, normal.id)).toBeVisible()
  await expect(row(page, escalated.id).getByTestId('board-incident-escalated')).toHaveText('Escalado')
  await expect(row(page, normal.id).getByTestId('board-incident-escalated')).toHaveCount(0)
  await admin.dispose()
})

test('UC-03.5 an ingeniero sees the active incidents of every service (BR-10)', async ({ page, playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const svcA = await createService(admin)
  const svcB = await createService(admin)
  const a = await declare(admin, svcA.id)
  const b = await declare(admin, svcB.id)

  const { email } = await registerUser(playwright, baseURL, admin, 'ingeniero')
  await loginUi(page, email)
  await expect(row(page, a.id)).toBeVisible()
  await expect(row(page, b.id)).toBeVisible()
  await expect(row(page, a.id).getByTestId('board-incident-service')).toHaveText(svcA.name)
  await expect(row(page, b.id).getByTestId('board-incident-service')).toHaveText(svcB.name)
  await admin.dispose()
})

test('UC-03.6 a visitor without session is denied (BR-10)', async ({ page, playwright, baseURL }) => {
  const anon = await playwright.request.newContext({ baseURL })
  const res = await anon.get('/api/incidents')
  expect(res.status()).toBe(401)
  await anon.dispose()

  await page.goto('/')
  await expect(page.getByTestId('login-email')).toBeVisible()
  await expect(page.getByTestId('incident-board')).toHaveCount(0)
})
