import { randomUUID } from 'node:crypto'
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'
import pg from 'pg'

// The backend escalates overdue incidents with a ticker (ESCALATION_INTERVAL, 1s in this suite)
// that reads the real clock, so expired SLAs are produced by moving declared_at into the past
// straight in the e2e database (DATABASE_URL, exported by `make e2e`, which is disposable).
// There is no API to raise a severity yet (UC-05), so UC-04.5 also seeds `severity` with SQL.
// Negative assertions ("does not escalate") wait for a few ticker intervals.

const adminEmail = process.env.ADMIN_EMAIL ?? ''
const adminPassword = process.env.ADMIN_PASSWORD ?? ''
const databaseUrl = process.env.DATABASE_URL ?? ''
const password = 'password-e2e'
const tickerSettleMs = 3500

type PlaywrightLike = { request: { newContext: (o: { baseURL?: string }) => Promise<APIRequestContext> } }
type Role = 'ingeniero' | 'oncall' | 'admin'

interface ServiceDto {
  id: string
  name: string
  oncall_user_id: string | null
}

interface UserDto {
  id: string
  email: string
}

interface IncidentDto {
  id: string
  state: string
  assigned_to: string | null
  acknowledged_at: string | null
  escalated_at: string | null
}

function uniqueName(): string {
  return `svc-${randomUUID()}`
}

async function loginViaApi(ctx: APIRequestContext, email: string, pass: string): Promise<void> {
  const res = await ctx.post('/api/auth/login', { data: { email, password: pass } })
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
  role: Role,
): Promise<{ email: string; user: UserDto }> {
  const email = `user-${randomUUID()}@e2e.test`
  const anon = await playwright.request.newContext({ baseURL })
  const reg = await anon.post('/api/auth/register', { data: { name: 'E2E User', email, password } })
  expect(reg.status()).toBe(201)
  const user = (await reg.json()) as UserDto
  await anon.dispose()
  if (role !== 'ingeniero') {
    const res = await admin.patch(`/api/users/${user.id}/role`, { data: { role } })
    expect(res.status()).toBe(200)
  }
  return { email, user }
}

async function userContext(
  playwright: PlaywrightLike,
  baseURL: string | undefined,
  admin: APIRequestContext,
  role: Role,
): Promise<{ ctx: APIRequestContext; email: string; user: UserDto }> {
  const { email, user } = await registerUser(playwright, baseURL, admin, role)
  const ctx = await playwright.request.newContext({ baseURL })
  await loginViaApi(ctx, email, password)
  return { ctx, email, user }
}

async function assignOncall(admin: APIRequestContext, serviceId: string, userId: string): Promise<void> {
  const res = await admin.put(`/api/services/${serviceId}/oncall`, { data: { user_id: userId } })
  expect(res.status()).toBe(200)
}

async function declare(ctx: APIRequestContext, serviceId: string, severity: 'SEV1' | 'SEV2' | 'SEV3'): Promise<IncidentDto> {
  const res = await ctx.post('/api/incidents', {
    data: { title: `inc-${randomUUID()}`, service_id: serviceId, impact: 'degradacion', severity },
  })
  expect(res.status()).toBe(201)
  return ((await res.json()) as { incident: IncidentDto }).incident
}

async function getIncident(ctx: APIRequestContext, id: string): Promise<IncidentDto> {
  const res = await ctx.get(`/api/incidents/${id}`)
  expect(res.status()).toBe(200)
  return (await res.json()) as IncidentDto
}

async function withDb<T>(run: (client: pg.Client) => Promise<T>): Promise<T> {
  const client = new pg.Client({ connectionString: databaseUrl })
  await client.connect()
  try {
    return await run(client)
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

// Moves declared_at into the past, as if the incident had been declared `minutes` ago.
async function seedDeclaredAgo(id: string, minutes: number): Promise<void> {
  await withDb(async (c) => {
    const res = await c.query('UPDATE incidents SET declared_at = now() - make_interval(mins => $1::int) WHERE id = $2', [
      minutes,
      id,
    ])
    expect(res.rowCount).toBe(1)
  })
}

async function seedSeverity(id: string, severity: string): Promise<void> {
  await withDb(async (c) => {
    const res = await c.query('UPDATE incidents SET severity = $1 WHERE id = $2', [severity, id])
    expect(res.rowCount).toBe(1)
  })
}

async function eventRows(id: string, type: string): Promise<{ author_id: string | null; data: Record<string, string> }[]> {
  return withDb(async (c) => {
    const res = await c.query('SELECT author_id, data FROM timeline_events WHERE incident_id = $1 AND type = $2', [id, type])
    return res.rows as { author_id: string | null; data: Record<string, string> }[]
  })
}

async function waitEscalated(ctx: APIRequestContext, id: string): Promise<void> {
  await expect.poll(async () => (await getIncident(ctx, id)).escalated_at, { timeout: 15_000 }).not.toBeNull()
}

async function loginUi(page: Page, email: string, pass: string): Promise<void> {
  await page.goto('/')
  await page.getByTestId('login-email').fill(email)
  await page.getByTestId('login-password').fill(pass)
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('incident-board')).toBeVisible()
}

function row(page: Page, id: string) {
  return page.locator(`[data-testid="board-incident-row"][data-incident-id="${id}"]`)
}

async function openDetail(page: Page, id: string): Promise<void> {
  await row(page, id).getByTestId('board-incident-open').click()
  await expect(page.getByTestId('incident-detail-title')).toBeVisible()
}

test('UC-04.1 only the admin assigns an on-call, who must have role oncall (BR-11, BR-12)', async ({ page, playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const svc = await createService(admin)
  const oncall = await userContext(playwright, baseURL, admin, 'oncall')
  const engineer = await userContext(playwright, baseURL, admin, 'ingeniero')

  // The admin assigns from the UI.
  await loginUi(page, adminEmail, adminPassword)
  const svcRow = page.locator(`[data-testid="service-row"][data-service-name="${svc.name}"]`)
  await expect(svcRow.getByTestId('service-oncall-select').locator(`option[value="${oncall.user.id}"]`)).toBeAttached()
  await svcRow.getByTestId('service-oncall-select').selectOption(oncall.user.id)
  const [response] = await Promise.all([
    page.waitForResponse((r) => r.url().endsWith(`/api/services/${svc.id}/oncall`) && r.request().method() === 'PUT'),
    svcRow.getByTestId('service-oncall-submit').click(),
  ])
  expect(response.status()).toBe(200)

  // New incidents of the service are assigned to that user.
  const inc = await declare(engineer.ctx, svc.id, 'SEV3')
  expect(inc.assigned_to).toBe(oncall.user.id)

  // An ingeniero and an oncall are rejected and the service does not change.
  const other = await userContext(playwright, baseURL, admin, 'oncall')
  for (const actor of [engineer.ctx, oncall.ctx]) {
    const res = await actor.put(`/api/services/${svc.id}/oncall`, { data: { user_id: other.user.id } })
    expect(res.status()).toBe(403)
  }
  // A target without role oncall is rejected.
  const badTarget = await admin.put(`/api/services/${svc.id}/oncall`, { data: { user_id: engineer.user.id } })
  expect(badTarget.status()).toBe(400)
  const services = (await (await admin.get('/api/services')).json()) as ServiceDto[]
  expect(services.find((s) => s.id === svc.id)?.oncall_user_id).toBe(oncall.user.id)
  await admin.dispose()
})

test('UC-04.2 changing the on-call reassigns the active incidents with an asignacion event (BR-11, BR-15, BR-09)', async ({
  playwright,
  baseURL,
}) => {
  const admin = await adminContext(playwright, baseURL)
  const svc = await createService(admin)
  const ana = await registerUser(playwright, baseURL, admin, 'oncall')
  const bruno = await registerUser(playwright, baseURL, admin, 'oncall')
  await assignOncall(admin, svc.id, ana.user.id)

  const declared = await declare(admin, svc.id, 'SEV3')
  const resolved = await declare(admin, svc.id, 'SEV3')
  const closed = await declare(admin, svc.id, 'SEV3')
  await seedState(resolved.id, 'resuelto')
  await seedState(closed.id, 'cerrado')

  await assignOncall(admin, svc.id, bruno.user.id)

  for (const inc of [declared, resolved]) {
    expect((await getIncident(admin, inc.id)).assigned_to).toBe(bruno.user.id)
    const events = await eventRows(inc.id, 'asignacion')
    expect(events).toHaveLength(1)
    expect(events[0].data).toEqual({ from: ana.user.id, to: bruno.user.id })
  }
  expect((await getIncident(admin, closed.id)).assigned_to).toBe(ana.user.id)
  expect(await eventRows(closed.id, 'asignacion')).toHaveLength(0)
  await admin.dispose()
})

test('UC-04.3 an unacknowledged incident escalates once, with or without on-call (BR-03, BR-04, BR-05)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminContext(playwright, baseURL)
  const withOncall = await createService(admin)
  const withoutOncall = await createService(admin)
  const oncall = await registerUser(playwright, baseURL, admin, 'oncall')
  await assignOncall(admin, withOncall.id, oncall.user.id)

  const a = await declare(admin, withOncall.id, 'SEV2')
  const b = await declare(admin, withoutOncall.id, 'SEV2')
  const fresh = await declare(admin, withOncall.id, 'SEV2')
  expect(b.assigned_to).toBeNull()
  await seedDeclaredAgo(a.id, 16)
  await seedDeclaredAgo(b.id, 16)
  await waitEscalated(admin, a.id)
  await waitEscalated(admin, b.id)

  // Several more ticks: still exactly one escalado event, written by the system.
  await new Promise((resolve) => setTimeout(resolve, tickerSettleMs))
  for (const id of [a.id, b.id]) {
    const events = await eventRows(id, 'escalado')
    expect(events).toHaveLength(1)
    expect(events[0].author_id).toBeNull()
  }
  expect((await getIncident(admin, b.id)).assigned_to).toBeNull()
  expect((await getIncident(admin, fresh.id)).escalated_at).toBeNull()

  const { email } = await registerUser(playwright, baseURL, admin, 'ingeniero')
  await loginUi(page, email, password)
  for (const id of [a.id, b.id]) {
    await expect(row(page, id).getByTestId('board-incident-escalated')).toHaveText('Escalado')
  }
  await expect(row(page, fresh.id).getByTestId('board-incident-escalated')).toHaveCount(0)
  await openDetail(page, a.id)
  await expect(page.getByTestId('incident-detail-escalated')).toHaveText('Escalado')
  await admin.dispose()
})

test('UC-04.4 an incident acknowledged by the on-call does not escalate when the deadline passes (BR-04, BR-05)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminContext(playwright, baseURL)
  const svc = await createService(admin)
  const oncall = await registerUser(playwright, baseURL, admin, 'oncall')
  await assignOncall(admin, svc.id, oncall.user.id)
  const inc = await declare(admin, svc.id, 'SEV2')

  await loginUi(page, oncall.email, password)
  await openDetail(page, inc.id)
  await expect(page.getByTestId('incident-detail-state')).toHaveText('declarado')
  await page.getByTestId('incident-acknowledge').click()
  await expect(page.getByTestId('incident-detail-state')).toHaveText('reconocido')
  await expect(page.getByTestId('incident-acknowledge')).toHaveCount(0)

  const acknowledged = await getIncident(admin, inc.id)
  expect(acknowledged.state).toBe('reconocido')
  expect(acknowledged.acknowledged_at).not.toBeNull()
  const changes = await eventRows(inc.id, 'cambio_estado')
  expect(changes).toHaveLength(1)
  expect(changes[0].data).toEqual({ from: 'declarado', to: 'reconocido' })

  // The deadline passes afterwards: the ticker evaluates it several times and does nothing.
  await seedDeclaredAgo(inc.id, 16)
  await new Promise((resolve) => setTimeout(resolve, tickerSettleMs))
  expect((await getIncident(admin, inc.id)).escalated_at).toBeNull()
  expect(await eventRows(inc.id, 'escalado')).toHaveLength(0)

  await page.reload()
  await expect(page.getByTestId('incident-board')).toBeVisible()
  await expect(row(page, inc.id).getByTestId('board-incident-escalated')).toHaveCount(0)
  await openDetail(page, inc.id)
  await expect(page.getByTestId('incident-detail-escalated')).toHaveCount(0)
  await admin.dispose()
})

test('UC-04.5 the SLA uses the current severity measured from declared_at (BR-03, BR-04)', async ({ playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const svc = await createService(admin)

  // SEV3 raised to SEV1, declared 7 minutes ago: past the 5-minute SEV1 deadline.
  const raised = await declare(admin, svc.id, 'SEV3')
  await seedSeverity(raised.id, 'SEV1')
  await seedDeclaredAgo(raised.id, 7)
  // SEV3 unchanged: 59 minutes is inside the 60-minute deadline, 61 is not.
  const inside = await declare(admin, svc.id, 'SEV3')
  await seedDeclaredAgo(inside.id, 59)
  const outside = await declare(admin, svc.id, 'SEV3')
  await seedDeclaredAgo(outside.id, 61)

  await waitEscalated(admin, raised.id)
  await waitEscalated(admin, outside.id)
  await new Promise((resolve) => setTimeout(resolve, tickerSettleMs))
  expect((await getIncident(admin, inside.id)).escalated_at).toBeNull()
  expect(await eventRows(inside.id, 'escalado')).toHaveLength(0)
  expect(await eventRows(raised.id, 'escalado')).toHaveLength(1)
  expect(await eventRows(outside.id, 'escalado')).toHaveLength(1)
  await admin.dispose()
})

test('UC-04.6 an oncall of another service cannot acknowledge the incident (BR-11)', async ({ page, playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const svc = await createService(admin)
  const owner = await registerUser(playwright, baseURL, admin, 'oncall')
  await assignOncall(admin, svc.id, owner.user.id)
  const foreign = await registerUser(playwright, baseURL, admin, 'oncall')
  const engineer = await userContext(playwright, baseURL, admin, 'ingeniero')
  const inc = await declare(admin, svc.id, 'SEV2')

  await loginUi(page, foreign.email, password)
  await openDetail(page, inc.id)
  await page.getByTestId('incident-acknowledge').click()
  await expect(page.getByTestId('incident-detail-error')).toBeVisible()
  await expect(page.getByTestId('incident-detail-state')).toHaveText('declarado')

  const ack = await engineer.ctx.post(`/api/incidents/${inc.id}/transitions`, { data: { to: 'reconocido' } })
  expect(ack.status()).toBe(403)
  const stored = await getIncident(admin, inc.id)
  expect(stored.state).toBe('declarado')
  expect(stored.acknowledged_at).toBeNull()
  expect(await eventRows(inc.id, 'cambio_estado')).toHaveLength(0)
  await admin.dispose()
})
