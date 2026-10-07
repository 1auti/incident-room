import { randomUUID } from 'node:crypto'
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

// UC-02.5 "no se crea el incidente ni ningún evento" is not observable in e2e: there is no
// endpoint to list incidents or timeline events yet. The backend covers it with
// TestUC025_SinCamposObligatoriosNoCrea and TestUC025_ValidacionAlDeclarar.

const adminEmail = process.env.ADMIN_EMAIL ?? ''
const adminPassword = process.env.ADMIN_PASSWORD ?? ''

type PlaywrightLike = { request: { newContext: (o: { baseURL?: string }) => Promise<APIRequestContext> } }

interface ServiceDto {
  id: string
  name: string
  criticality: string
  oncall_user_id: string | null
}

interface UserDto {
  id: string
  email: string
}

interface IncidentDto {
  id: string
  state: string
  severity: string
  suggested_severity: string
  declared_by: string
  assigned_to: string | null
  declared_at: string
}

interface EventDto {
  type: string
  author_id: string | null
  data: Record<string, string>
}

interface DeclareDto {
  incident: IncidentDto
  timeline: EventDto[]
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

async function createService(admin: APIRequestContext, criticality: string): Promise<ServiceDto> {
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

async function userContext(
  playwright: PlaywrightLike,
  baseURL: string | undefined,
  admin: APIRequestContext,
  role: 'ingeniero' | 'oncall' | 'admin',
): Promise<{ ctx: APIRequestContext; user: UserDto }> {
  const { email, user } = await registerUser(playwright, baseURL, admin, role)
  const ctx = await playwright.request.newContext({ baseURL })
  await loginViaApi(ctx, email, 'password-e2e')
  return { ctx, user }
}

async function loginAsEngineerUi(
  page: Page,
  playwright: PlaywrightLike,
  baseURL: string | undefined,
  admin: APIRequestContext,
): Promise<UserDto> {
  const { email, user } = await registerUser(playwright, baseURL, admin, 'ingeniero')
  await page.goto('/')
  await page.getByTestId('login-email').fill(email)
  await page.getByTestId('login-password').fill('password-e2e')
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('declare-incident-form')).toBeVisible()
  return user
}

async function fillForm(page: Page, opts: { title: string; serviceId: string; impact: string }) {
  await page.getByTestId('incident-title-input').fill(opts.title)
  await page.getByTestId('incident-service-select').selectOption(opts.serviceId)
  await page.getByTestId('incident-impact-select').selectOption(opts.impact)
}

async function submit(page: Page) {
  const [response] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/api/incidents') && r.request().method() === 'POST'),
    page.getByTestId('incident-declare-submit').click(),
  ])
  return response
}

async function suggestViaApi(ctx: APIRequestContext, serviceId: string, impact: string): Promise<string> {
  const res = await ctx.get(`/api/incidents/suggested-severity?service_id=${serviceId}&impact=${impact}`)
  expect(res.status()).toBe(200)
  return ((await res.json()) as { suggested_severity: string }).suggested_severity
}

test('UC-02.1 the suggestion follows service criticality and impact (BR-01)', async ({ page, playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const critical = await createService(admin, 'critica')
  const standard = await createService(admin, 'estandar')

  expect(await suggestViaApi(admin, critical.id, 'caida_total')).toBe('SEV1')
  expect(await suggestViaApi(admin, standard.id, 'degradacion')).toBe('SEV3')

  await loginAsEngineerUi(page, playwright, baseURL, admin)
  await page.getByTestId('incident-service-select').selectOption(critical.id)
  await page.getByTestId('incident-impact-select').selectOption('caida_total')
  await expect(page.getByTestId('incident-suggested-severity')).toHaveText('SEV1')
  await expect(page.getByTestId('incident-severity-select')).toHaveValue('SEV1')

  await page.getByTestId('incident-service-select').selectOption(standard.id)
  await page.getByTestId('incident-impact-select').selectOption('degradacion')
  await expect(page.getByTestId('incident-suggested-severity')).toHaveText('SEV3')
  await expect(page.getByTestId('incident-severity-select')).toHaveValue('SEV3')
  await admin.dispose()
})

test('UC-02.2 any allowed role declares accepting the suggested severity', async ({ page, playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const svc = await createService(admin, 'critica') // caida_total => SEV1
  const { user: oncallUser } = await registerUser(playwright, baseURL, admin, 'oncall')
  expect((await admin.put(`/api/services/${svc.id}/oncall`, { data: { user_id: oncallUser.id } })).status()).toBe(200)

  // ingeniero through the UI.
  const engineer = await loginAsEngineerUi(page, playwright, baseURL, admin)
  await fillForm(page, { title: 'DB caida', serviceId: svc.id, impact: 'caida_total' })
  await expect(page.getByTestId('incident-suggested-severity')).toHaveText('SEV1')
  const before = Date.now()
  const response = await submit(page)
  const after = Date.now()
  expect(response.status()).toBe(201)
  const body = (await response.json()) as DeclareDto

  expect(body.incident.state).toBe('declarado')
  expect(body.incident.severity).toBe(body.incident.suggested_severity)
  expect(body.incident.declared_by).toBe(engineer.id)
  expect(body.incident.assigned_to).toBe(oncallUser.id)
  const declaredAt = Date.parse(body.incident.declared_at)
  expect(declaredAt).toBeGreaterThanOrEqual(before)
  expect(declaredAt).toBeLessThanOrEqual(after)
  const declaration = body.timeline.find((e) => e.type === 'declaracion')
  expect(declaration?.author_id).toBe(engineer.id)

  await expect(page.getByTestId('declared-incident')).toBeVisible()
  await expect(page.getByTestId('incident-state')).toHaveText('declarado')
  await expect(page.getByTestId('incident-severity')).toHaveText('SEV1')
  await expect(page.locator('[data-testid="timeline-event"][data-event-type="declaracion"]')).toHaveCount(1)

  // oncall and admin through the API.
  for (const role of ['oncall', 'admin'] as const) {
    const { ctx, user } = await userContext(playwright, baseURL, admin, role)
    const t0 = Date.now()
    const res = await ctx.post('/api/incidents', {
      data: { title: `Incidente ${role}`, service_id: svc.id, impact: 'caida_total' },
    })
    const t1 = Date.now()
    expect(res.status(), role).toBe(201)
    const dto = (await res.json()) as DeclareDto
    expect(dto.incident.state).toBe('declarado')
    expect(dto.incident.severity).toBe(dto.incident.suggested_severity)
    expect(dto.incident.declared_by).toBe(user.id)
    expect(dto.incident.assigned_to).toBe(oncallUser.id)
    const at = Date.parse(dto.incident.declared_at)
    expect(at).toBeGreaterThanOrEqual(t0)
    expect(at).toBeLessThanOrEqual(t1)
    expect(dto.timeline.find((e) => e.type === 'declaracion')?.author_id).toBe(user.id)
    await ctx.dispose()
  }
  await admin.dispose()
})

test('UC-02.3 choosing a different severity keeps the suggestion and records cambio_severidad', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminContext(playwright, baseURL)
  const svc = await createService(admin, 'importante') // + caida_total => SEV2
  await loginAsEngineerUi(page, playwright, baseURL, admin)
  await fillForm(page, { title: 'Latencia alta', serviceId: svc.id, impact: 'caida_total' })
  await expect(page.getByTestId('incident-suggested-severity')).toHaveText('SEV2')
  await page.getByTestId('incident-severity-select').selectOption('SEV1')

  const response = await submit(page)
  expect(response.status()).toBe(201)
  const body = (await response.json()) as DeclareDto
  expect(body.incident.suggested_severity).toBe('SEV2')
  expect(body.incident.severity).toBe('SEV1')

  const me = (await (await page.request.get('/api/me')).json()) as UserDto
  const change = body.timeline.find((e) => e.type === 'cambio_severidad')
  expect(change?.data).toEqual({ from: 'SEV2', to: 'SEV1' })
  expect(change?.author_id).toBe(me.id)

  await expect(page.getByTestId('incident-severity')).toHaveText('SEV1')
  await expect(page.locator('[data-testid="timeline-event"][data-event-type="cambio_severidad"]')).toHaveCount(1)
  await admin.dispose()
})

test('UC-02.4 a service without on-call yields an unassigned incident', async ({ page, playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const svc = await createService(admin, 'estandar')
  expect(svc.oncall_user_id).toBeNull()
  await loginAsEngineerUi(page, playwright, baseURL, admin)
  await fillForm(page, { title: 'Sin guardia', serviceId: svc.id, impact: 'menor' })
  await expect(page.getByTestId('incident-suggested-severity')).toHaveText('SEV3')

  const response = await submit(page)
  expect(response.status()).toBe(201)
  expect(((await response.json()) as DeclareDto).incident.assigned_to).toBeNull()
  await expect(page.getByTestId('incident-assigned-to')).toHaveText('Sin asignar')
  await admin.dispose()
})

test('UC-02.5 missing title, service or impact is a validation error', async ({ page, playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const svc = await createService(admin, 'estandar')

  // Missing title through the UI: the form has no native "required", so the backend validates.
  await loginAsEngineerUi(page, playwright, baseURL, admin)
  await fillForm(page, { title: '   ', serviceId: svc.id, impact: 'menor' })
  const response = await submit(page)
  expect(response.status()).toBe(400)
  await expect(page.getByTestId('declare-error')).toBeVisible()
  await expect(page.getByTestId('declared-incident')).toHaveCount(0)

  // Missing service or impact: the UI cannot submit a half-filled selector reliably, so use the API.
  const { ctx } = await userContext(playwright, baseURL, admin, 'ingeniero')
  const noService = await ctx.post('/api/incidents', { data: { title: 'x', impact: 'menor' } })
  expect(noService.status()).toBe(400)
  const noImpact = await ctx.post('/api/incidents', { data: { title: 'x', service_id: svc.id } })
  expect(noImpact.status()).toBe(400)
  await ctx.dispose()
  await admin.dispose()
})

test('UC-02.6 a visitor without session cannot declare (API)', async ({ playwright, baseURL }) => {
  const admin = await adminContext(playwright, baseURL)
  const svc = await createService(admin, 'estandar')
  const anon = await playwright.request.newContext({ baseURL })
  const res = await anon.post('/api/incidents', { data: { title: 'x', service_id: svc.id, impact: 'menor' } })
  expect(res.status()).toBe(401)
  await anon.dispose()
  await admin.dispose()
})
