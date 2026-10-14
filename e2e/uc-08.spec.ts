import { randomUUID } from 'node:crypto'
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'
import { goTo, loginUi as uiLogin } from './helpers'
import pg from 'pg'

// The LLM is fake-ollama.mjs (POST /api/chat answers a fixed analysis). Incidents reach resuelto by
// seeding the state straight in the disposable e2e database (DATABASE_URL, exported by `make e2e`).

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
  await uiLogin(page, email, pass)
  await goTo(page, 'incidents')
  await expect(page.getByTestId('incident-board')).toBeVisible()
}

async function openDetail(page: Page, id: string): Promise<void> {
  await page
    .locator(`[data-testid="board-incident-row"][data-incident-id="${id}"]`)
    .getByTestId('board-incident-open')
    .click()
  await expect(page.getByTestId('timeline-event').first()).toBeVisible()
}

async function transition(ctx: APIRequestContext, id: string, to: string, rootCause = ''): Promise<number> {
  return (await ctx.post(`/api/incidents/${id}/transitions`, { data: { to, root_cause: rootCause } })).status()
}

interface PostmortemDto {
  id: string
  content: string
  status: string
  generated_by_llm: boolean
  approved_by: string | null
  approved_at: string | null
  updated_at: string
}

const llmAnalysis = 'Análisis del LLM de prueba'

async function postmortemOf(ctx: APIRequestContext, id: string): Promise<PostmortemDto | null> {
  const res = await ctx.get(`/api/incidents/${id}/postmortem`)
  if (res.status() === 404) return null
  expect(res.status()).toBe(200)
  return (await res.json()) as PostmortemDto
}

async function postmortemEvents(ctx: APIRequestContext, id: string): Promise<string[]> {
  return (await timeline(ctx, id)).filter((e) => e.type === 'postmortem').map((e) => e.data.action)
}

// A resolved incident of a service whose on-call is `oncall`, with a note in its timeline.
async function resolvedIncident(engineer: Actor, service: string, severity: Severity = 'SEV2'): Promise<string> {
  const id = await declare(engineer.ctx, service, severity)
  const note = await engineer.ctx.post(`/api/incidents/${id}/notes`, { data: { body: 'Reinicié payments-api-2' } })
  expect(note.status()).toBe(201)
  await seedState(id, 'resuelto')
  return id
}

test('UC-08.1 the on-call or an admin generates a draft that cites the timeline (BR-13, BR-09, BR-11)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const service = await serviceWithOncall(admin, oncall)
  const byOncall = await resolvedIncident(engineer, service)
  const byAdmin = await resolvedIncident(engineer, service)

  await loginUi(page, oncall.email, password)
  await openDetail(page, byOncall)
  await expect(page.getByTestId('postmortem-empty')).toBeVisible()
  await page.getByTestId('postmortem-generate').click()
  await expect(page.getByTestId('postmortem-status')).toHaveText('borrador')
  await expect(page.getByTestId('postmortem-content')).toHaveValue(new RegExp(llmAnalysis))
  await expect(page.getByTestId('timeline-event').last().getByTestId('timeline-event-type')).toHaveText('postmortem')

  expect((await admin.post(`/api/incidents/${byAdmin}/postmortem/generate`)).status()).toBe(200)

  for (const id of [byOncall, byAdmin]) {
    const pm = await postmortemOf(admin, id)
    expect(pm?.status).toBe('borrador')
    expect(pm?.generated_by_llm).toBe(true)
    // It cites the timeline events: the declaration and the note with its author (the state is seeded with SQL).
    for (const cited of ['declaracion', `nota · ${engineer.name} · Reinicié payments-api-2`, llmAnalysis]) {
      expect(pm?.content).toContain(cited)
    }
    expect(await postmortemEvents(admin, id)).toEqual(['generado'])
  }
})

test('UC-08.2 an unresolved incident or an ingeniero cannot generate the postmortem (BR-13, BR-10)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const service = await serviceWithOncall(admin, oncall)
  const mitigating = await declare(engineer.ctx, service, 'SEV2')
  await seedState(mitigating, 'mitigando')
  const resolved = await resolvedIncident(engineer, service)

  expect((await oncall.ctx.post(`/api/incidents/${mitigating}/postmortem/generate`)).status()).toBe(409)
  expect((await admin.post(`/api/incidents/${mitigating}/postmortem/generate`)).status()).toBe(409)
  expect((await engineer.ctx.post(`/api/incidents/${resolved}/postmortem/generate`)).status()).toBe(403)

  // The ingeniero sees the refusal in the UI.
  await loginUi(page, engineer.email, password)
  await openDetail(page, resolved)
  await page.getByTestId('postmortem-generate').click()
  await expect(page.getByTestId('postmortem-error')).toBeVisible()
  await expect(page.getByTestId('postmortem-empty')).toBeVisible()

  for (const id of [mitigating, resolved]) {
    expect(await postmortemOf(admin, id)).toBeNull()
    expect(await postmortemEvents(admin, id)).toEqual([])
  }
})

test('UC-08.3 the draft is edited or regenerated, keeping a single postmortem (BR-13)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const id = await resolvedIncident(engineer, await serviceWithOncall(admin, oncall))
  expect((await oncall.ctx.post(`/api/incidents/${id}/postmortem/generate`)).status()).toBe(200)
  const generated = await postmortemOf(admin, id)

  await loginUi(page, oncall.email, password)
  await openDetail(page, id)
  await expect(page.getByTestId('postmortem-status')).toHaveText('borrador')
  await page.getByTestId('postmortem-content').fill('## Resumen\nTexto corregido por el on-call')
  // The textarea already shows the text after fill: wait for the PUT, not for the UI.
  const [saved] = await Promise.all([
    page.waitForResponse((r) => r.url().endsWith(`/api/incidents/${id}/postmortem`) && r.request().method() === 'PUT'),
    page.getByTestId('postmortem-save').click(),
  ])
  expect(saved.status()).toBe(200)
  await expect(page.getByTestId('postmortem-content')).toHaveValue('## Resumen\nTexto corregido por el on-call')

  const edited = await postmortemOf(admin, id)
  expect(edited?.content).toBe('## Resumen\nTexto corregido por el on-call')
  expect(edited?.status).toBe('borrador')
  expect(Date.parse(edited?.updated_at ?? '')).toBeGreaterThan(Date.parse(generated?.updated_at ?? ''))
  // Editing the text adds no event (BR-09).
  expect(await postmortemEvents(admin, id)).toEqual(['generado'])

  await page.getByTestId('postmortem-generate').click()
  await expect(page.getByTestId('postmortem-content')).toHaveValue(new RegExp(llmAnalysis))
  const regenerated = await postmortemOf(admin, id)
  expect(regenerated?.id).toBe(generated?.id)
  expect(regenerated?.content).not.toContain('Texto corregido por el on-call')
  expect(await postmortemEvents(admin, id)).toEqual(['generado', 'regenerado'])
  await withDb(async (c) => {
    const { rows } = await c.query<{ n: string }>('SELECT count(*) AS n FROM postmortems WHERE incident_id = $1', [id])
    expect(Number(rows[0].n)).toBe(1)
  })
})

test('UC-08.4 an admin approves the draft, recording who and when, and a resolved SEV1 can then close (BR-13, BR-09, BR-05, BR-07)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const id = await resolvedIncident(engineer, await serviceWithOncall(admin, oncall), 'SEV1')
  expect((await oncall.ctx.post(`/api/incidents/${id}/postmortem/generate`)).status()).toBe(200)
  // Before approval the SEV1 cannot close.
  expect(await transition(oncall.ctx, id, 'cerrado')).toBe(409)

  await loginUi(page, adminEmail, adminPassword)
  await openDetail(page, id)
  await page.getByTestId('postmortem-approve').click()
  await expect(page.getByTestId('postmortem-status')).toHaveText('aprobado')
  await expect(page.getByTestId('postmortem-approved-at')).toBeVisible()

  const adminId = ((await (await admin.get('/api/me')).json()) as { id: string }).id
  const pm = await postmortemOf(admin, id)
  expect(pm?.approved_by).toBe(adminId)
  expect(Math.abs(Date.now() - Date.parse(pm?.approved_at ?? ''))).toBeLessThan(60_000)
  expect(await postmortemEvents(admin, id)).toEqual(['generado', 'aprobado'])

  await page.getByTestId('incident-close').click()
  await expect(page.getByTestId('incident-detail-state')).toHaveText('cerrado')
})

test('UC-08.5 the on-call or an ingeniero cannot approve: the postmortem stays a draft (BR-13, BR-10)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const id = await resolvedIncident(engineer, await serviceWithOncall(admin, oncall))
  expect((await oncall.ctx.post(`/api/incidents/${id}/postmortem/generate`)).status()).toBe(200)

  for (const { ctx } of [oncall, engineer]) {
    expect((await ctx.post(`/api/incidents/${id}/postmortem/approve`)).status()).toBe(403)
  }
  await loginUi(page, oncall.email, password)
  await openDetail(page, id)
  await page.getByTestId('postmortem-approve').click()
  await expect(page.getByTestId('postmortem-error')).toBeVisible()
  await expect(page.getByTestId('postmortem-status')).toHaveText('borrador')

  const pm = await postmortemOf(admin, id)
  expect(pm?.status).toBe('borrador')
  expect(pm?.approved_by).toBeNull()
  expect(await postmortemEvents(admin, id)).toEqual(['generado'])
})

test('UC-08.6 an approved postmortem cannot be edited or regenerated (BR-13)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const id = await resolvedIncident(engineer, await serviceWithOncall(admin, oncall))
  expect((await oncall.ctx.post(`/api/incidents/${id}/postmortem/generate`)).status()).toBe(200)
  expect((await admin.post(`/api/incidents/${id}/postmortem/approve`)).status()).toBe(200)
  const approved = await postmortemOf(admin, id)

  for (const ctx of [admin, oncall.ctx]) {
    expect((await ctx.put(`/api/incidents/${id}/postmortem`, { data: { content: 'cambio' } })).status()).toBe(409)
    expect((await ctx.post(`/api/incidents/${id}/postmortem/generate`)).status()).toBe(409)
  }
  expect(await postmortemOf(admin, id)).toEqual(approved)
  expect(await postmortemEvents(admin, id)).toEqual(['generado', 'aprobado'])

  // The UI shows it read-only, without actions.
  await loginUi(page, adminEmail, adminPassword)
  await openDetail(page, id)
  await expect(page.getByTestId('postmortem-status')).toHaveText('aprobado')
  await expect(page.getByTestId('postmortem-content')).toHaveAttribute('readonly', '')
  await expect(page.getByTestId('postmortem-generate')).toHaveCount(0)
  await expect(page.getByTestId('postmortem-save')).toHaveCount(0)
})
