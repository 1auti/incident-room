import { randomInt, randomUUID } from 'node:crypto'
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'
import { goTo, loginUi as uiLogin } from './helpers'
import pg from 'pg'

// MTTR and SLA compliance are global, and the other specs declare incidents at the real "now" in
// parallel. So each test seeds its incidents with SQL (DATABASE_URL, exported by `make e2e`) inside
// its own random one-day window and queries that free range: windows never overlap "now" or each
// other. UC-09.4 needs an incident still within its deadline, so its window is in the future.

const adminEmail = process.env.ADMIN_EMAIL ?? ''
const adminPassword = process.env.ADMIN_PASSWORD ?? ''
const databaseUrl = process.env.DATABASE_URL ?? ''
const password = 'password-e2e'
const day = 24 * 60 * 60 * 1000

type PlaywrightLike = { request: { newContext: (o: { baseURL?: string }) => Promise<APIRequestContext> } }
type Role = 'ingeniero' | 'oncall' | 'admin'

interface DashboardDto {
  from: string
  to: string
  mttr_seconds: number | null
  incidents_by_service: { service_id: string; name: string; count: number }[]
  sla_compliance_percent: number | null
}

interface SeedIncident {
  serviceId: string
  severity?: 'SEV1' | 'SEV2' | 'SEV3'
  declaredAt: Date
  acknowledgedAt?: Date
  resolvedAt?: Date
  escalatedAt?: Date
}

async function login(ctx: APIRequestContext, email: string, pass: string): Promise<void> {
  expect((await ctx.post('/api/auth/login', { data: { email, password: pass } })).status()).toBe(200)
}

async function adminActor(playwright: PlaywrightLike, baseURL: string | undefined): Promise<APIRequestContext> {
  const ctx = await playwright.request.newContext({ baseURL })
  await login(ctx, adminEmail, adminPassword)
  return ctx
}

async function actor(
  playwright: PlaywrightLike,
  baseURL: string | undefined,
  admin: APIRequestContext,
  role: Role,
): Promise<{ ctx: APIRequestContext; email: string }> {
  const email = `user-${randomUUID()}@e2e.test`
  const ctx = await playwright.request.newContext({ baseURL })
  const reg = await ctx.post('/api/auth/register', { data: { name: 'E2E User', email, password } })
  expect(reg.status()).toBe(201)
  const { id } = (await reg.json()) as { id: string }
  if (role !== 'ingeniero') {
    expect((await admin.patch(`/api/users/${id}/role`, { data: { role } })).status()).toBe(200)
  }
  await login(ctx, email, password)
  return { ctx, email }
}

async function createService(admin: APIRequestContext, prefix: string): Promise<{ id: string; name: string }> {
  const res = await admin.post('/api/services', {
    data: { name: `${prefix}-${randomUUID()}`, criticality: 'importante' },
  })
  expect(res.status()).toBe(201)
  return (await res.json()) as { id: string; name: string }
}

// A random UTC day in [startYear, startYear + 50): one window per test, practically never shared.
function randomDay(startYear: number): Date {
  const start = Date.UTC(startYear, 0, 1)
  return new Date(start + randomInt(0, 50 * 365) * day)
}

function at(base: Date, hours: number, minutes = 0): Date {
  return new Date(base.getTime() + (hours * 60 + minutes) * 60 * 1000)
}

function isoDay(d: Date): string {
  return d.toISOString().slice(0, 10)
}

async function seed(incidents: SeedIncident[]): Promise<void> {
  const client = new pg.Client({ connectionString: databaseUrl })
  await client.connect()
  try {
    const { rows } = await client.query<{ id: string }>('SELECT id FROM users LIMIT 1')
    for (const i of incidents) {
      const state = i.resolvedAt ? 'resuelto' : i.acknowledgedAt ? 'reconocido' : 'declarado'
      const severity = i.severity ?? 'SEV2'
      await client.query(
        `INSERT INTO incidents (title, service_id, impact, suggested_severity, severity, state, declared_by,
           declared_at, acknowledged_at, resolved_at, escalated_at, root_cause)
         VALUES ($1, $2, 'degradacion', $3, $3, $4, $5, $6, $7, $8, $9, $10)`,
        [
          `inc-${randomUUID()}`,
          i.serviceId,
          severity,
          state,
          rows[0].id,
          i.declaredAt,
          i.acknowledgedAt ?? null,
          i.resolvedAt ?? null,
          i.escalatedAt ?? null,
          i.resolvedAt ? 'causa e2e' : null,
        ],
      )
    }
  } finally {
    await client.end()
  }
}

async function dashboard(ctx: APIRequestContext, from: Date, to: Date): Promise<DashboardDto> {
  const query = new URLSearchParams({ from: from.toISOString(), to: to.toISOString() })
  const res = await ctx.get(`/api/metrics?${query.toString()}`)
  expect(res.status()).toBe(200)
  return (await res.json()) as DashboardDto
}

function countOf(d: DashboardDto, serviceId: string): number | undefined {
  return d.incidents_by_service.find((s) => s.service_id === serviceId)?.count
}

async function loginUi(page: Page, email: string, pass: string): Promise<void> {
  await uiLogin(page, email, pass)
  await goTo(page, 'metrics')
  await expect(page.getByTestId('metrics-dashboard')).toBeVisible()
}

// Picks the free range covering exactly the window day ("hasta" is inclusive in the UI).
async function chooseDay(page: Page, d: Date): Promise<void> {
  await page.getByTestId('metrics-period-select').selectOption('range')
  await page.getByTestId('metrics-range-from').fill(isoDay(d))
  await page.getByTestId('metrics-range-to').fill(isoDay(d))
  await expect(page.getByTestId('metrics-period-from')).toHaveText(`${isoDay(d)}T00:00:00Z`)
}

test('UC-09.1 MTTR is the mean of resolved_at - declared_at in the period (BR-16)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const svc = await createService(admin, 'mttr')
  const d = randomDay(1950)
  await seed([
    { serviceId: svc.id, declaredAt: at(d, 10), acknowledgedAt: at(d, 10, 1), resolvedAt: at(d, 10, 30) },
    { serviceId: svc.id, declaredAt: at(d, 11), acknowledgedAt: at(d, 11, 1), resolvedAt: at(d, 12) },
  ])

  expect((await dashboard(admin, d, at(d, 24))).mttr_seconds).toBe(45 * 60)

  await loginUi(page, adminEmail, adminPassword)
  await chooseDay(page, d)
  await expect(page.getByTestId('metrics-mttr')).toHaveText('45 min')
})

test('UC-09.2 without resolved incidents MTTR has no value, and without eligible ones neither has SLA (BR-16, BR-18)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const svc = await createService(admin, 'empty')

  // An empty period: both without value, shown as "Sin datos" and not as 0. Its own range of
  // years, so no other test can seed data into it.
  const empty = randomDay(1850)
  const none = await dashboard(admin, empty, at(empty, 24))
  expect(none.mttr_seconds).toBeNull()
  expect(none.sla_compliance_percent).toBeNull()
  await loginUi(page, adminEmail, adminPassword)
  await chooseDay(page, empty)
  await expect(page.getByTestId('metrics-mttr')).toHaveText('Sin datos')
  await expect(page.getByTestId('metrics-sla')).toHaveText('Sin datos')

  // An unresolved incident does not enter the MTTR (it still counts for the SLA once acknowledged).
  const d = randomDay(1950)
  await seed([{ serviceId: svc.id, declaredAt: at(d, 10), acknowledgedAt: at(d, 10, 2) }])
  const unresolved = await dashboard(admin, d, at(d, 24))
  expect(unresolved.mttr_seconds).toBeNull()
  expect(unresolved.sla_compliance_percent).toBe(100)
})

test('UC-09.3 incidents per service include services with 0 (BR-17)', async ({ page, playwright, baseURL }) => {
  const admin = await adminActor(playwright, baseURL)
  const payments = await createService(admin, 'payments')
  const search = await createService(admin, 'search')
  const idle = await createService(admin, 'idle')
  const d = randomDay(1950)
  await seed([
    ...[10, 11, 12].map((h) => ({ serviceId: payments.id, declaredAt: at(d, h), acknowledgedAt: at(d, h, 1) })),
    { serviceId: search.id, declaredAt: at(d, 13), acknowledgedAt: at(d, 13, 1) },
  ])

  const got = await dashboard(admin, d, at(d, 24))
  expect([countOf(got, payments.id), countOf(got, search.id), countOf(got, idle.id)]).toEqual([3, 1, 0])

  await loginUi(page, adminEmail, adminPassword)
  await chooseDay(page, d)
  for (const [svc, count] of [
    [payments, '3'],
    [search, '1'],
    [idle, '0'],
  ] as const) {
    const row = page.locator(`[data-testid="metrics-service-row"][data-service-name="${svc.name}"]`)
    await expect(row.getByTestId('metrics-service-count')).toHaveText(count)
  }
})

test('UC-09.4 SLA compliance counts eligible incidents only; a pending one is left out (BR-18, BR-03, BR-05)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const svc = await createService(admin, 'sla')
  // Future window: the unacknowledged incident is still within its deadline for the real clock.
  const d = randomDay(2200)
  await seed([
    // 3 acknowledged within the SEV2 deadline (15 min).
    ...[10, 11, 12].map((h) => ({ serviceId: svc.id, declaredAt: at(d, h), acknowledgedAt: at(d, h, 10) })),
    // 1 escalated: acknowledged after its deadline passed.
    { serviceId: svc.id, declaredAt: at(d, 13), escalatedAt: at(d, 13, 15), acknowledgedAt: at(d, 13, 40) },
    // Pending: within its deadline and not acknowledged.
    { serviceId: svc.id, declaredAt: at(d, 14) },
  ])

  const got = await dashboard(admin, d, at(d, 24))
  expect(got.sla_compliance_percent).toBe(75)
  expect(countOf(got, svc.id)).toBe(5)

  await loginUi(page, adminEmail, adminPassword)
  await chooseDay(page, d)
  await expect(page.getByTestId('metrics-sla')).toHaveText('75 %')
})

test('UC-09.5 a free range filters MTTR by resolved_at and the rest by declared_at (BR-16, BR-17, BR-18)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const svc = await createService(admin, 'range')
  const d = randomDay(1950)
  const before = at(d, -24)
  const after = at(d, 24)
  await seed([
    // Declared before, resolved inside: in the MTTR (2h), not in the count nor the SLA.
    { serviceId: svc.id, declaredAt: at(d, -1), acknowledgedAt: at(d, -1, 1), resolvedAt: at(d, 1) },
    // Declared inside, resolved after: in the count and the SLA (on time), not in the MTTR.
    { serviceId: svc.id, declaredAt: at(d, 23), acknowledgedAt: at(d, 23, 1), resolvedAt: at(after, 2) },
    // Declared inside and escalated: in the count and the SLA (late).
    { serviceId: svc.id, declaredAt: at(d, 9), escalatedAt: at(d, 9, 15), acknowledgedAt: at(d, 9, 30) },
    // Entirely outside the range.
    { serviceId: svc.id, declaredAt: at(before, 3), acknowledgedAt: at(before, 3, 1), resolvedAt: at(before, 4) },
    { serviceId: svc.id, declaredAt: after, acknowledgedAt: at(after, 0, 1), resolvedAt: at(after, 1) },
  ])

  const got = await dashboard(admin, d, after)
  expect(got.mttr_seconds).toBe(2 * 60 * 60)
  expect(countOf(got, svc.id)).toBe(2)
  expect(got.sla_compliance_percent).toBe(50)

  await loginUi(page, adminEmail, adminPassword)
  await chooseDay(page, d)
  await expect(page.getByTestId('metrics-mttr')).toHaveText('120 min')
  await expect(page.getByTestId('metrics-sla')).toHaveText('50 %')
})

test('UC-09.6 every role opens the dashboard on the last 30 days and can switch to the last 7 (BR-10, BR-05)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const oncall = await actor(playwright, baseURL, admin, 'oncall')

  for (const ctx of [engineer.ctx, oncall.ctx, admin]) {
    const res = await ctx.get('/api/metrics')
    expect(res.status()).toBe(200)
    const d = (await res.json()) as DashboardDto
    expect(Date.parse(d.to) - Date.parse(d.from)).toBe(30 * day)
  }
  const anon = await playwright.request.newContext({ baseURL })
  expect((await anon.get('/api/metrics')).status()).toBe(401)
  await anon.dispose()

  const spanShown = async () =>
    Date.parse((await page.getByTestId('metrics-period-to').textContent()) ?? '') -
    Date.parse((await page.getByTestId('metrics-period-from').textContent()) ?? '')

  await loginUi(page, engineer.email, password)
  await expect(page.getByTestId('metrics-period-select')).toHaveValue('30d')
  await expect(page.getByTestId('metrics-period-from')).toBeVisible()
  expect(await spanShown()).toBe(30 * day)
  // The end of the default period is the backend clock's "now".
  const to = Date.parse((await page.getByTestId('metrics-period-to').textContent()) ?? '')
  expect(Math.abs(Date.now() - to)).toBeLessThan(60_000)

  const [response] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/api/metrics?period=7d')),
    page.getByTestId('metrics-period-select').selectOption('7d'),
  ])
  expect(response.status()).toBe(200)
  await expect.poll(spanShown).toBe(7 * day)
})
