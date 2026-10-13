import { randomUUID } from 'node:crypto'
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'
import pg from 'pg'

// Like uc-04/uc-05, starting states are seeded straight in the disposable e2e database
// (DATABASE_URL, exported by `make e2e`). Postmortems are seeded with SQL too: generating and
// approving them arrives with UC-08, and UC-06 only reads their status (BR-07).

const adminEmail = process.env.ADMIN_EMAIL ?? ''
const adminPassword = process.env.ADMIN_PASSWORD ?? ''
const databaseUrl = process.env.DATABASE_URL ?? ''
const password = 'password-e2e'

type PlaywrightLike = {
  request: {
    newContext: (o: { baseURL?: string }) => Promise<APIRequestContext>
  }
}
type Role = 'ingeniero' | 'oncall' | 'admin'
type Severity = 'SEV1' | 'SEV2' | 'SEV3'

interface Actor {
  ctx: APIRequestContext
  id: string
  name: string
  email: string
}

interface IncidentDto {
  state: string
  acknowledged_at: string | null
  root_cause: string | null
  resolved_at: string | null
  closed_at: string | null
}

interface EventDto {
  id: string
  type: string
  author_id: string | null
  author_name: string | null
  body: string
  data: Record<string, string>
  occurred_at: string
}

async function login(ctx: APIRequestContext, email: string, pass: string): Promise<void> {
  const res = await ctx.post('/api/auth/login', {
    data: { email, password: pass },
  })
  expect(res.status()).toBe(200)
}

async function adminActor(playwright: PlaywrightLike, baseURL: string | undefined): Promise<APIRequestContext> {
  const ctx = await playwright.request.newContext({ baseURL })
  await login(ctx, adminEmail, adminPassword)
  return ctx
}

// Each user gets a unique name so the timeline author can be told apart in the UI.
async function actor(
  playwright: PlaywrightLike,
  baseURL: string | undefined,
  admin: APIRequestContext,
  role: Role,
): Promise<Actor> {
  const email = `user-${randomUUID()}@e2e.test`
  const name = `${role}-${randomUUID().slice(0, 8)}`
  const ctx = await playwright.request.newContext({ baseURL })
  const reg = await ctx.post('/api/auth/register', {
    data: { name, email, password },
  })
  expect(reg.status()).toBe(201)
  const { id } = (await reg.json()) as { id: string }
  if (role !== 'ingeniero') {
    expect((await admin.patch(`/api/users/${id}/role`, { data: { role } })).status()).toBe(200)
  }
  await login(ctx, email, password)
  return { ctx, id, name, email }
}

async function serviceWithOncall(admin: APIRequestContext, oncall: Actor): Promise<string> {
  const res = await admin.post('/api/services', {
    data: { name: `svc-${randomUUID()}`, criticality: 'importante' },
  })
  expect(res.status()).toBe(201)
  const { id } = (await res.json()) as { id: string }
  expect((await admin.put(`/api/services/${id}/oncall`, { data: { user_id: oncall.id } })).status()).toBe(200)
  return id
}

async function declare(ctx: APIRequestContext, serviceId: string, severity: Severity): Promise<string> {
  const res = await ctx.post('/api/incidents', {
    data: {
      title: `inc-${randomUUID()}`,
      service_id: serviceId,
      impact: 'degradacion',
      severity,
    },
  })
  expect(res.status()).toBe(201)
  return ((await res.json()) as { incident: { id: string } }).incident.id
}

async function timeline(ctx: APIRequestContext, id: string): Promise<EventDto[]> {
  const res = await ctx.get(`/api/incidents/${id}/timeline`)
  expect(res.status()).toBe(200)
  return (await res.json()) as EventDto[]
}

async function incidentOf(ctx: APIRequestContext, id: string): Promise<IncidentDto> {
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
    expect((await c.query('UPDATE incidents SET state = $1 WHERE id = $2', [state, id])).rowCount).toBe(1)
  })
}

async function loginUi(page: Page, email: string, pass: string): Promise<void> {
  await page.goto('/')
  await page.getByTestId('login-email').fill(email)
  await page.getByTestId('login-password').fill(pass)
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('incident-board')).toBeVisible()
}

async function openDetail(page: Page, id: string): Promise<void> {
  await page
    .locator(`[data-testid="board-incident-row"][data-incident-id="${id}"]`)
    .getByTestId('board-incident-open')
    .click()
  await expect(page.getByTestId('timeline-event').first()).toBeVisible()
}

// Only state changes: declaring with a severity other than the suggested one adds a cambio_severidad (BR-02).
async function stateChanges(ctx: APIRequestContext, id: string): Promise<EventDto[]> {
  return (await timeline(ctx, id)).filter((e) => e.type === 'cambio_estado')
}

async function transition(ctx: APIRequestContext, id: string, to: string, rootCause = ''): Promise<number> {
  return (await ctx.post(`/api/incidents/${id}/transitions`, { data: { to, root_cause: rootCause } })).status()
}

async function seedPostmortem(incidentId: string, status: 'borrador' | 'aprobado', approverId: string): Promise<void> {
  const approved = status === 'aprobado'
  await withDb(async (c) => {
    await c.query(
      `INSERT INTO postmortems (incident_id, content, status, generated_by_llm, approved_by, approved_at, updated_at)
       VALUES ($1, 'Postmortem e2e', $2, true, $3, $4, now())`,
      [incidentId, status, approved ? approverId : null, approved ? new Date() : null],
    )
  })
}

test('UC-06.1 the on-call acknowledges and then mitigates, each step adding a cambio_estado (BR-08, BR-09, BR-05)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const id = await declare(engineer.ctx, await serviceWithOncall(admin, oncall), 'SEV2')

  await loginUi(page, oncall.email, password)
  await openDetail(page, id)
  await page.getByTestId('incident-acknowledge').click()
  await expect(page.getByTestId('incident-detail-state')).toHaveText('reconocido')
  await page.getByTestId('incident-mitigate').click()
  await expect(page.getByTestId('incident-detail-state')).toHaveText('mitigando')

  const inc = await incidentOf(admin, id)
  expect(inc.acknowledged_at).not.toBeNull()
  const changes = await stateChanges(admin, id)
  expect(changes.map((e) => e.data)).toEqual([
    { from: 'declarado', to: 'reconocido' },
    { from: 'reconocido', to: 'mitigando' },
  ])
  expect(changes.every((e) => e.author_id === oncall.id)).toBe(true)
  expect(Date.parse(changes[0].occurred_at)).toBe(Date.parse(inc.acknowledged_at ?? ''))
})

test('UC-06.2 resolving with an empty or blank root cause is rejected without changes (BR-06)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const id = await declare(admin, await serviceWithOncall(admin, oncall), 'SEV2')
  await seedState(id, 'mitigando')

  await loginUi(page, oncall.email, password)
  await openDetail(page, id)
  await page.getByTestId('incident-resolve').click()
  await expect(page.getByTestId('incident-detail-error')).toBeVisible()
  await page.getByTestId('incident-root-cause-input').fill('   ')
  await page.getByTestId('incident-resolve').click()
  await expect(page.getByTestId('incident-detail-error')).toBeVisible()
  await expect(page.getByTestId('incident-detail-state')).toHaveText('mitigando')

  expect(await transition(oncall.ctx, id, 'resuelto', ' \t ')).toBe(400)
  const inc = await incidentOf(admin, id)
  expect(inc.state).toBe('mitigando')
  expect(inc.root_cause).toBeNull()
  expect(await stateChanges(admin, id)).toHaveLength(0)
})

test('UC-06.3 resolving with a root cause stores it, sets resolved_at and adds a cambio_estado (BR-06, BR-05, BR-09)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const id = await declare(admin, await serviceWithOncall(admin, oncall), 'SEV2')
  await seedState(id, 'mitigando')
  const cause = 'Pool de conexiones agotado por fuga en v2.3'

  await loginUi(page, oncall.email, password)
  await openDetail(page, id)
  await page.getByTestId('incident-root-cause-input').fill(cause)
  await page.getByTestId('incident-resolve').click()
  await expect(page.getByTestId('incident-detail-state')).toHaveText('resuelto')
  await expect(page.getByTestId('incident-detail-root-cause')).toHaveText(cause)

  const inc = await incidentOf(admin, id)
  expect(inc.root_cause).toBe(cause)
  expect(inc.resolved_at).not.toBeNull()
  const changes = await stateChanges(admin, id)
  expect(changes).toHaveLength(1)
  expect(changes[0].data).toEqual({ from: 'mitigando', to: 'resuelto' })
  expect(changes[0].body).toBe(cause)
})

test('UC-06.4 a resolved SEV1 without postmortem or with a draft cannot be closed (BR-07)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const service = await serviceWithOncall(admin, oncall)
  const withoutPm = await declare(admin, service, 'SEV1')
  const withDraft = await declare(admin, service, 'SEV1')
  for (const id of [withoutPm, withDraft]) await seedState(id, 'resuelto')
  await seedPostmortem(withDraft, 'borrador', oncall.id)

  await loginUi(page, oncall.email, password)
  await openDetail(page, withoutPm)
  await page.getByTestId('incident-close').click()
  await expect(page.getByTestId('incident-detail-error')).toBeVisible()
  await expect(page.getByTestId('incident-detail-state')).toHaveText('resuelto')

  expect(await transition(admin, withDraft, 'cerrado')).toBe(409)
  for (const id of [withoutPm, withDraft]) {
    const inc = await incidentOf(admin, id)
    expect(inc.state).toBe('resuelto')
    expect(inc.closed_at).toBeNull()
    expect(await stateChanges(admin, id)).toHaveLength(0)
  }
})

test('UC-06.5 a resolved SEV1 with an approved postmortem, or a SEV2 without one, closes (BR-07)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const service = await serviceWithOncall(admin, oncall)
  const sev1 = await declare(admin, service, 'SEV1')
  const sev2 = await declare(admin, service, 'SEV2')
  for (const id of [sev1, sev2]) await seedState(id, 'resuelto')
  await seedPostmortem(sev1, 'aprobado', oncall.id)

  expect(await transition(admin, sev1, 'cerrado')).toBe(200)

  await loginUi(page, oncall.email, password)
  await openDetail(page, sev2)
  await page.getByTestId('incident-close').click()
  await expect(page.getByTestId('incident-detail-state')).toHaveText('cerrado')

  for (const id of [sev1, sev2]) {
    const inc = await incidentOf(admin, id)
    expect(inc.state).toBe('cerrado')
    expect(inc.closed_at).not.toBeNull()
    expect((await stateChanges(admin, id)).map((e) => e.data)).toEqual([{ from: 'resuelto', to: 'cerrado' }])
  }
})

test('UC-06.6 unlisted transitions, an ingeniero or a foreign oncall are rejected without changes (BR-08, BR-10, BR-11)', async ({
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const foreign = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const service = await serviceWithOncall(admin, oncall)

  const declared = await declare(engineer.ctx, service, 'SEV2')
  const mitigating = await declare(engineer.ctx, service, 'SEV2')
  await seedState(mitigating, 'mitigando')

  expect(await transition(admin, declared, 'resuelto', 'causa')).toBe(409)
  expect(await transition(admin, mitigating, 'reconocido')).toBe(409)
  expect(await transition(engineer.ctx, declared, 'reconocido')).toBe(403)
  expect(await transition(foreign.ctx, declared, 'reconocido')).toBe(403)
  expect(await transition(foreign.ctx, mitigating, 'resuelto', 'causa')).toBe(403)

  expect((await incidentOf(admin, declared)).state).toBe('declarado')
  expect((await incidentOf(admin, mitigating)).state).toBe('mitigando')
  for (const id of [declared, mitigating]) expect(await stateChanges(admin, id)).toHaveLength(0)
})
