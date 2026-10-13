import { randomUUID } from 'node:crypto'
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'
import pg from 'pg'

// The copilot's model and embeddings are fake-ollama.mjs: a rule-based stand-in that picks tools by
// keywords and answers from the runbook in its prompt (it "invents" a procedure when there is none).
// These tests check the backend's guarantees around it: tools run as the user with the API's rules,
// sources come from retrieval, and nothing is improvised without a runbook. Each test tags its texts
// and services with a unique token so the shared database does not leak into its results.

const adminEmail = process.env.ADMIN_EMAIL ?? ''
const adminPassword = process.env.ADMIN_PASSWORD ?? ''
const databaseUrl = process.env.DATABASE_URL ?? ''
const password = 'password-e2e'

type PlaywrightLike = { request: { newContext: (o: { baseURL?: string }) => Promise<APIRequestContext> } }
type Role = 'ingeniero' | 'oncall' | 'admin'

interface Actor {
  ctx: APIRequestContext
  id: string
  email: string
}

interface AnswerDto {
  reply: string
  sources: { runbook_id: string; title: string; service_name: string }[]
  actions: { tool: string; ok: boolean; result: string }[]
}

interface IncidentDto {
  id: string
  title: string
  severity: string
  suggested_severity: string
  state: string
  declared_by: string
}

interface EventDto {
  type: string
  author_id: string | null
  body: string
}

function tag(): string {
  return `t${randomUUID().replaceAll('-', '').slice(0, 10)}`
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
): Promise<Actor> {
  const email = `user-${randomUUID()}@e2e.test`
  const ctx = await playwright.request.newContext({ baseURL })
  const reg = await ctx.post('/api/auth/register', { data: { name: 'E2E User', email, password } })
  expect(reg.status()).toBe(201)
  const { id } = (await reg.json()) as { id: string }
  if (role !== 'ingeniero') {
    expect((await admin.patch(`/api/users/${id}/role`, { data: { role } })).status()).toBe(200)
  }
  await login(ctx, email, password)
  return { ctx, id, email }
}

async function createService(admin: APIRequestContext, name: string): Promise<string> {
  const res = await admin.post('/api/services', { data: { name, criticality: 'critica' } })
  expect(res.status()).toBe(201)
  return ((await res.json()) as { id: string }).id
}

async function createRunbook(
  admin: APIRequestContext,
  serviceId: string,
  title: string,
  content: string,
): Promise<string> {
  const res = await admin.post(`/api/services/${serviceId}/runbooks`, { data: { title, content } })
  expect(res.status()).toBe(201)
  return ((await res.json()) as { id: string }).id
}

async function ask(ctx: APIRequestContext, message: string): Promise<AnswerDto> {
  const res = await ctx.post('/api/copilot', { data: { message } })
  expect(res.status()).toBe(200)
  return (await res.json()) as AnswerDto
}

async function declare(ctx: APIRequestContext, serviceId: string, severity: string): Promise<string> {
  const res = await ctx.post('/api/incidents', {
    data: { title: `inc-${randomUUID()}`, service_id: serviceId, impact: 'degradacion', severity },
  })
  expect(res.status()).toBe(201)
  return ((await res.json()) as { incident: { id: string } }).incident.id
}

async function incidentOf(ctx: APIRequestContext, id: string): Promise<IncidentDto> {
  const res = await ctx.get(`/api/incidents/${id}`)
  expect(res.status()).toBe(200)
  return (await res.json()) as IncidentDto
}

async function timeline(ctx: APIRequestContext, id: string): Promise<EventDto[]> {
  const res = await ctx.get(`/api/incidents/${id}/timeline`)
  expect(res.status()).toBe(200)
  return (await res.json()) as EventDto[]
}

async function seedState(id: string, state: string): Promise<void> {
  const client = new pg.Client({ connectionString: databaseUrl })
  await client.connect()
  try {
    expect((await client.query('UPDATE incidents SET state = $1 WHERE id = $2', [state, id])).rowCount).toBe(1)
  } finally {
    await client.end()
  }
}

async function loginUi(page: Page, email: string, pass: string): Promise<void> {
  await page.goto('/')
  await page.getByTestId('login-email').fill(email)
  await page.getByTestId('login-password').fill(pass)
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('copilot-panel')).toBeVisible()
}

async function askUi(page: Page, message: string) {
  await page.getByTestId('copilot-input').fill(message)
  await page.getByTestId('copilot-send').click()
  const reply = page.locator('[data-testid="copilot-message"][data-role="copilot"]').last()
  await expect(page.getByTestId('copilot-input')).toHaveValue('')
  return reply
}

test('UC-10.1 a question about a runbook is answered from it and cites it (BR-10, BR-14)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const t = tag()
  const serviceId = await createService(admin, `payments-${t}`)
  const title = `Pool agotado ${t}`
  const runbookId = await createRunbook(
    admin,
    serviceId,
    title,
    `Cuando el pool ${t} de conexiones se agota: reiniciar payments-api y escalar a 6 réplicas`,
  )
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')

  const question = `¿cómo procedo cuando el pool ${t} de conexiones se agota?`
  const answer = await ask(engineer.ctx, question)
  expect(answer.sources.map((s) => s.runbook_id)).toContain(runbookId)
  expect(answer.reply).toContain(title)
  expect(answer.reply).toContain('escalar a 6 réplicas')

  await loginUi(page, engineer.email, password)
  const reply = await askUi(page, question)
  await expect(reply.getByTestId('copilot-reply')).toContainText('escalar a 6 réplicas')
  await expect(reply.getByTestId('copilot-source').filter({ hasText: title })).toHaveCount(1)
})

test('UC-10.2 without a relevant runbook, or once it is removed, the copilot says so and does not improvise (BR-12, BR-14)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const t = tag()

  // Nothing relevant: the fake model's invented steps never reach the user.
  const unknown = await ask(engineer.ctx, `¿qué hago si la cola kafka ${t} se llena de mensajes duplicados?`)
  expect(unknown.reply).toContain('No encontré información')
  expect(unknown.reply).not.toContain('reiniciá todos los servidores')
  expect(unknown.sources).toEqual([])

  // A runbook is cited while it exists, and no longer after an admin removes it. The wording shares
  // no words with other specs' runbooks, which run in parallel on the same database.
  const serviceId = await createService(admin, `billing-${t}`)
  const question = `¿cómo regenero la clave ${t} del firmador de facturas?`
  const runbookId = await createRunbook(
    admin,
    serviceId,
    `Firmador de facturas ${t}`,
    `Para regenerar la clave ${t} del firmador de facturas: usar la consola del HSM`,
  )
  expect((await ask(engineer.ctx, question)).sources.map((s) => s.runbook_id)).toContain(runbookId)
  expect((await admin.delete(`/api/runbooks/${runbookId}`)).status()).toBe(204)

  const after = await ask(engineer.ctx, question)
  expect(after.sources.map((s) => s.runbook_id)).not.toContain(runbookId)
  expect(after.reply).not.toContain('consola del HSM')

  await loginUi(page, engineer.email, password)
  const reply = await askUi(page, question)
  await expect(reply.getByTestId('copilot-reply')).toContainText('No encontré información')
  await expect(reply.getByTestId('copilot-source')).toHaveCount(0)
})

test('UC-10.3 an ingeniero opens an incident through the copilot with the suggested severity and as its author (BR-14, BR-01, BR-09)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const service = `payments-${tag()}`
  await createService(admin, service)

  await loginUi(page, engineer.email, password)
  const reply = await askUi(page, `abrí un incidente en ${service}, caída total`)
  const action = reply.getByTestId('copilot-action')
  await expect(action).toHaveAttribute('data-ok', 'true')
  const replyText = (await reply.getByTestId('copilot-reply').textContent()) ?? ''
  const id = replyText.match(/incidente (\S+) con severidad/)?.[1] ?? ''
  expect(id).not.toBe('')

  const inc = await incidentOf(admin, id)
  // critica x caida_total = SEV1, the suggested severity (BR-01).
  expect([inc.severity, inc.suggested_severity, inc.declared_by, inc.state]).toEqual([
    'SEV1',
    'SEV1',
    engineer.id,
    'declarado',
  ])
  const [declaration] = await timeline(admin, id)
  expect([declaration.type, declaration.author_id]).toEqual(['declaracion', engineer.id])
})

test('UC-10.4 a note added through the copilot is authored by the user (BR-14, BR-09)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const serviceId = await createService(admin, `payments-${tag()}`)
  const id = await declare(admin, serviceId, 'SEV1')

  await loginUi(page, oncall.email, password)
  const reply = await askUi(page, `agregá una nota al incidente ${id}: Rollback a v2.2 aplicado`)
  await expect(reply.getByTestId('copilot-action')).toHaveAttribute('data-ok', 'true')

  const notes = (await timeline(admin, id)).filter((e) => e.type === 'nota')
  expect(notes).toEqual([{ ...notes[0], type: 'nota', author_id: oncall.id, body: 'Rollback a v2.2 aplicado' }])
})

test('UC-10.5 listing open incidents returns only active ones matching the filter (BR-15, BR-14)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const service = `listing-${tag()}`
  const serviceId = await createService(admin, service)
  const activeSev1 = await declare(admin, serviceId, 'SEV1')
  const resolvedSev1 = await declare(admin, serviceId, 'SEV1')
  const activeSev2 = await declare(admin, serviceId, 'SEV2')
  const closedSev1 = await declare(admin, serviceId, 'SEV1')
  await seedState(resolvedSev1, 'resuelto')
  await seedState(closedSev1, 'cerrado')

  const all = await ask(engineer.ctx, `listá los abiertos del servicio ${service}`)
  const listed = (a: AnswerDto) => (JSON.parse(a.actions[0].result) as { id: string }[]).map((i) => i.id).sort()
  expect(listed(all)).toEqual([activeSev1, resolvedSev1, activeSev2].sort())

  await loginUi(page, engineer.email, password)
  const reply = await askUi(page, `listá los abiertos sev1 del servicio ${service}`)
  await expect(reply.getByTestId('copilot-reply')).toContainText(activeSev1)
  await expect(reply.getByTestId('copilot-reply')).toContainText(resolvedSev1)
  await expect(reply.getByTestId('copilot-reply')).not.toContainText(activeSev2)
  await expect(reply.getByTestId('copilot-reply')).not.toContainText(closedSev1)
})

test('UC-10.6 asking to close an incident is declined and its state does not change (BR-14, BR-08)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const serviceId = await createService(admin, `payments-${tag()}`)
  const id = await declare(admin, serviceId, 'SEV2')
  await seedState(id, 'resuelto')
  const before = await timeline(admin, id)

  await loginUi(page, adminEmail, adminPassword)
  const reply = await askUi(page, `cerrá el incidente ${id}`)
  await expect(reply.getByTestId('copilot-reply')).toContainText(/no puedo cerrar/i)

  expect((await incidentOf(admin, id)).state).toBe('resuelto')
  expect(await timeline(admin, id)).toEqual(before)
})
