import { randomUUID } from 'node:crypto'
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'
import pg from 'pg'

// Like uc-04.spec.ts, states and past declarations are seeded straight in the disposable e2e
// database (DATABASE_URL, exported by `make e2e`): there is no API for T2/T3 yet (UC-06), and the
// escalation ticker (ESCALATION_INTERVAL 1s) reads the real clock.

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

async function severityOf(ctx: APIRequestContext, id: string): Promise<string> {
  const res = await ctx.get(`/api/incidents/${id}`)
  expect(res.status()).toBe(200)
  return ((await res.json()) as { severity: string }).severity
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

test('UC-05.1 the timeline lists events chronologically, with the escalado as a system event (BR-09, BR-04, BR-10)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const reader = await actor(playwright, baseURL, admin, 'ingeniero')
  const id = await declare(engineer.ctx, await serviceWithOncall(admin, oncall), 'SEV2')

  // Declared 20 minutes ago: the SEV2 deadline (15 min) has passed and the ticker escalates it.
  await withDb(async (c) => {
    await c.query("UPDATE incidents SET declared_at = now() - interval '20 minutes' WHERE id = $1", [id])
  })
  await expect
    .poll(async () => (await timeline(engineer.ctx, id)).map((e) => e.type), {
      timeout: 15_000,
    })
    .toContain('escalado')
  expect(
    (await engineer.ctx.post(`/api/incidents/${id}/notes`, { data: { body: 'Revisando el pool' } })).status(),
  ).toBe(201)

  // Any authenticated user, not only the declarer, reads it in ascending occurred_at order.
  const events = await timeline(reader.ctx, id)
  expect(events.map((e) => e.type)).toEqual(['declaracion', 'escalado', 'nota'])
  const instants = events.map((e) => Date.parse(e.occurred_at))
  expect(instants).toEqual([...instants].sort((a, b) => a - b))
  expect(events[1].author_id).toBeNull()

  await loginUi(page, reader.email, password)
  await openDetail(page, id)
  const rows = page.getByTestId('timeline-event')
  await expect(rows.getByTestId('timeline-event-type')).toHaveText(['declaracion', 'escalado', 'nota'])
  await expect(rows.getByTestId('timeline-event-author')).toHaveText([engineer.name, 'Sistema', engineer.name])
  await expect(rows.nth(2).getByTestId('timeline-event-content')).toHaveText('Revisando el pool')
})

test('UC-05.2 a note is appended with its author; a correction keeps the original (BR-09, BR-05, BR-10)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const id = await declare(engineer.ctx, await serviceWithOncall(admin, oncall), 'SEV2')

  await loginUi(page, engineer.email, password)
  await openDetail(page, id)
  for (const body of ['Reinicié payments-api-1', 'Corrección: reinicié payments-api-2']) {
    await page.getByTestId('timeline-note-input').fill(body)
    await page.getByTestId('timeline-note-submit').click()
    await expect(page.getByTestId('timeline-event').last().getByTestId('timeline-event-content')).toHaveText(body)
  }
  const rows = page.getByTestId('timeline-event')
  await expect(rows.getByTestId('timeline-event-type')).toHaveText(['declaracion', 'nota', 'nota'])
  await expect(rows.nth(1).getByTestId('timeline-event-author')).toHaveText(engineer.name)

  const notes = (await timeline(engineer.ctx, id)).filter((e) => e.type === 'nota')
  expect(notes.map((n) => n.body)).toEqual(['Reinicié payments-api-1', 'Corrección: reinicié payments-api-2'])
  expect(notes.every((n) => n.author_id === engineer.id)).toBe(true)

  // Every role may add notes; an empty one is rejected without an event.
  for (const ctx of [oncall.ctx, admin]) {
    expect((await ctx.post(`/api/incidents/${id}/notes`, { data: { body: 'ok' } })).status()).toBe(201)
  }
  expect((await engineer.ctx.post(`/api/incidents/${id}/notes`, { data: { body: ' ' } })).status()).toBe(400)
  expect((await timeline(engineer.ctx, id)).filter((e) => e.type === 'nota')).toHaveLength(4)
})

test('UC-05.3 a timeline event cannot be modified or deleted through the API (BR-09)', async ({
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const id = await declare(admin, await serviceWithOncall(admin, oncall), 'SEV3')
  expect((await admin.post(`/api/incidents/${id}/notes`, { data: { body: 'original' } })).status()).toBe(201)
  const before = await timeline(admin, id)
  const note = before[1]

  for (const ctx of [admin, oncall.ctx]) {
    const put = await ctx.put(`/api/incidents/${id}/timeline/${note.id}`, { data: { body: 'editado' } })
    const del = await ctx.delete(`/api/incidents/${id}/timeline/${note.id}`)
    expect([403, 405]).toContain(put.status())
    expect([403, 405]).toContain(del.status())
  }
  expect(await timeline(admin, id)).toEqual(before)
})

test('UC-05.4 the on-call of the service or an admin raises SEV2 to SEV1 while mitigando (BR-02, BR-11)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const service = await serviceWithOncall(admin, oncall)

  // The on-call from the UI.
  const viaUi = await declare(engineer.ctx, service, 'SEV2')
  await seedState(viaUi, 'mitigando')
  await loginUi(page, oncall.email, password)
  await openDetail(page, viaUi)
  await page.getByTestId('incident-detail-severity-select').selectOption('SEV1')
  await expect(page.getByTestId('incident-detail-severity')).toHaveText('SEV1')
  const last = page.getByTestId('timeline-event').last()
  await expect(last.getByTestId('timeline-event-type')).toHaveText('cambio_severidad')
  await expect(last.getByTestId('timeline-event-author')).toHaveText(oncall.name)
  await expect(last.getByTestId('timeline-event-content')).toHaveText('SEV2 → SEV1')

  // The admin through the API.
  const viaApi = await declare(engineer.ctx, service, 'SEV2')
  await seedState(viaApi, 'mitigando')
  expect((await admin.patch(`/api/incidents/${viaApi}/severity`, { data: { severity: 'SEV1' } })).status()).toBe(200)
  expect(await severityOf(admin, viaApi)).toBe('SEV1')

  const adminId = ((await (await admin.get('/api/me')).json()) as { id: string }).id
  for (const [id, author] of [
    [viaUi, oncall.id],
    [viaApi, adminId],
  ]) {
    const change = (await timeline(admin, id)).filter((e) => e.type === 'cambio_severidad')
    expect(change).toHaveLength(1)
    expect(change[0].data).toEqual({ from: 'SEV2', to: 'SEV1' })
    expect(change[0].author_id).toBe(author)
  }
})

test('UC-05.5 an ingeniero, a foreign oncall, or an admin on a resolved SEV1 cannot change the severity (BR-02, BR-11, BR-10)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const foreign = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const service = await serviceWithOncall(admin, oncall)

  const active = await declare(engineer.ctx, service, 'SEV2')
  await seedState(active, 'mitigando')
  // Declared SEV1 over a suggested SEV2: its timeline already holds the declaration's cambio_severidad.
  const resolved = await declare(engineer.ctx, service, 'SEV1')
  await seedState(resolved, 'resuelto')
  const before = { [active]: await timeline(admin, active), [resolved]: await timeline(admin, resolved) }

  const patchSeverity = async (ctx: APIRequestContext, id: string, severity: Severity) =>
    (await ctx.patch(`/api/incidents/${id}/severity`, { data: { severity } })).status()
  expect(await patchSeverity(engineer.ctx, active, 'SEV1')).toBe(403)
  expect(await patchSeverity(foreign.ctx, active, 'SEV1')).toBe(403)
  expect(await patchSeverity(admin, resolved, 'SEV2')).toBe(409)

  // The declarer tries from the UI and sees the refusal.
  await loginUi(page, engineer.email, password)
  await openDetail(page, active)
  await page.getByTestId('incident-detail-severity-select').selectOption('SEV1')
  await expect(page.getByTestId('incident-detail-error')).toBeVisible()
  await expect(page.getByTestId('incident-detail-severity')).toHaveText('SEV2')

  expect(await severityOf(admin, active)).toBe('SEV2')
  expect(await severityOf(admin, resolved)).toBe('SEV1')
  for (const id of [active, resolved]) {
    expect(await timeline(admin, id)).toEqual(before[id])
  }
})

test('UC-05.6 a visitor without session cannot read the timeline, add notes or change the severity (BR-10)', async ({
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const id = await declare(admin, await serviceWithOncall(admin, oncall), 'SEV2')
  const before = await timeline(admin, id)

  const anon = await playwright.request.newContext({ baseURL })
  expect((await anon.get(`/api/incidents/${id}/timeline`)).status()).toBe(401)
  expect((await anon.post(`/api/incidents/${id}/notes`, { data: { body: 'hola' } })).status()).toBe(401)
  expect((await anon.patch(`/api/incidents/${id}/severity`, { data: { severity: 'SEV1' } })).status()).toBe(401)
  await anon.dispose()

  expect(await timeline(admin, id)).toEqual(before)
  expect(await severityOf(admin, id)).toBe('SEV2')
})
