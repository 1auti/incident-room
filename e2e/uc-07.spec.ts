import { randomUUID } from 'node:crypto'
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'
import pg from 'pg'

// Embeddings come from fake-ollama.mjs (deterministic per text). The stored embedding is read
// straight from the disposable e2e database (DATABASE_URL, exported by `make e2e`).

const adminEmail = process.env.ADMIN_EMAIL ?? ''
const adminPassword = process.env.ADMIN_PASSWORD ?? ''
const databaseUrl = process.env.DATABASE_URL ?? ''
const password = 'password-e2e'

type PlaywrightLike = { request: { newContext: (o: { baseURL?: string }) => Promise<APIRequestContext> } }
type Role = 'ingeniero' | 'oncall' | 'admin'

interface RunbookDto {
  id: string
  service_id: string
  title: string
  content: string
  updated_at: string
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

async function createService(admin: APIRequestContext): Promise<{ id: string; name: string }> {
  const res = await admin.post('/api/services', { data: { name: `svc-${randomUUID()}`, criticality: 'importante' } })
  expect(res.status()).toBe(201)
  return (await res.json()) as { id: string; name: string }
}

async function createRunbook(
  admin: APIRequestContext,
  serviceId: string,
  title: string,
  content: string,
): Promise<RunbookDto> {
  const res = await admin.post(`/api/services/${serviceId}/runbooks`, { data: { title, content } })
  expect(res.status()).toBe(201)
  return (await res.json()) as RunbookDto
}

async function runbooksOf(ctx: APIRequestContext, serviceId: string): Promise<RunbookDto[]> {
  const res = await ctx.get(`/api/services/${serviceId}/runbooks`)
  expect(res.status()).toBe(200)
  return (await res.json()) as RunbookDto[]
}

// The stored embedding as text, or null when the row is gone (removal is physical, BR-12).
async function embeddingOf(id: string): Promise<string | null> {
  const client = new pg.Client({ connectionString: databaseUrl })
  await client.connect()
  try {
    const { rows } = await client.query<{ e: string }>('SELECT embedding::text AS e FROM runbooks WHERE id = $1', [id])
    return rows[0]?.e ?? null
  } finally {
    await client.end()
  }
}

async function loginUi(page: Page, email: string, pass: string): Promise<void> {
  await page.goto('/')
  await page.getByTestId('login-email').fill(email)
  await page.getByTestId('login-password').fill(pass)
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('runbooks-panel')).toBeVisible()
}

async function openService(page: Page, serviceId: string): Promise<void> {
  const select = page.getByTestId('runbooks-service-select')
  await expect(select.locator(`option[value="${serviceId}"]`)).toBeAttached()
  await select.selectOption(serviceId)
}

function row(page: Page, title: string) {
  return page.locator(`[data-testid="runbook-row"][data-runbook-title="${title}"]`)
}

test('UC-07.1 the admin creates a runbook and it is listed under its service (BR-12)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const svc = await createService(admin)
  const title = `Pool agotado ${randomUUID().slice(0, 8)}`

  await loginUi(page, adminEmail, adminPassword)
  await openService(page, svc.id)
  await expect(page.getByTestId('runbooks-empty')).toBeVisible()
  await page.getByTestId('runbook-form-title').fill(title)
  await page.getByTestId('runbook-form-content').fill('Paso 1: reiniciar el pool de conexiones')
  await page.getByTestId('runbook-form-submit').click()
  await expect(row(page, title).getByTestId('runbook-content')).toHaveText('Paso 1: reiniciar el pool de conexiones')

  const list = await runbooksOf(admin, svc.id)
  expect(list.map((r) => [r.title, r.service_id])).toEqual([[title, svc.id]])
  expect(await embeddingOf(list[0].id)).not.toBeNull()
})

test('UC-07.2 editing the content saves it, changes updated_at and recalculates the embedding (BR-12)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const svc = await createService(admin)
  const rb = await createRunbook(admin, svc.id, 'Latencia alta', 'Paso 1: revisar el balanceador')
  const embeddingBefore = await embeddingOf(rb.id)

  await loginUi(page, adminEmail, adminPassword)
  await openService(page, svc.id)
  await row(page, 'Latencia alta').getByTestId('runbook-edit').click()
  await page.getByTestId('runbook-form-content').fill('Paso 1: escalar réplicas a 6\nPaso 2: vaciar la caché')
  await page.getByTestId('runbook-form-submit').click()
  await expect(row(page, 'Latencia alta').getByTestId('runbook-content')).toHaveText(
    'Paso 1: escalar réplicas a 6\nPaso 2: vaciar la caché',
  )

  const [after] = await runbooksOf(admin, svc.id)
  expect(after.content).toBe('Paso 1: escalar réplicas a 6\nPaso 2: vaciar la caché')
  expect(Date.parse(after.updated_at)).toBeGreaterThan(Date.parse(rb.updated_at))
  const embeddingAfter = await embeddingOf(rb.id)
  expect(embeddingAfter).not.toBeNull()
  expect(embeddingAfter).not.toEqual(embeddingBefore)
})

test('UC-07.3 an ingeniero or an oncall cannot create, edit or remove runbooks (BR-12, BR-10)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const svc = await createService(admin)
  const rb = await createRunbook(admin, svc.id, 'Disco lleno', 'Paso 1: rotar logs')
  const embedding = await embeddingOf(rb.id)
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')
  const oncall = await actor(playwright, baseURL, admin, 'oncall')

  for (const { ctx } of [engineer, oncall]) {
    expect((await ctx.post(`/api/services/${svc.id}/runbooks`, { data: { title: 't', content: 'c' } })).status()).toBe(
      403,
    )
    expect((await ctx.put(`/api/runbooks/${rb.id}`, { data: { title: 't', content: 'c' } })).status()).toBe(403)
    expect((await ctx.delete(`/api/runbooks/${rb.id}`)).status()).toBe(403)
  }
  expect(await runbooksOf(admin, svc.id)).toEqual([rb])
  expect(await embeddingOf(rb.id)).toEqual(embedding)

  // The UI shows them the runbook without management controls.
  await loginUi(page, engineer.email, password)
  await openService(page, svc.id)
  await expect(row(page, 'Disco lleno')).toBeVisible()
  await expect(page.getByTestId('runbook-edit')).toHaveCount(0)
  await expect(page.getByTestId('runbook-delete')).toHaveCount(0)
  await expect(page.getByTestId('runbook-form-submit')).toHaveCount(0)
})

test('UC-07.4 any authenticated user sees only the runbooks of the chosen service (BR-10, BR-12)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const payments = await createService(admin)
  const search = await createService(admin)
  await createRunbook(admin, payments.id, 'Pagos rechazados', 'Paso 1: revisar el proveedor')
  await createRunbook(admin, payments.id, 'Pool agotado', 'Paso 1: reiniciar el pool')
  await createRunbook(admin, search.id, 'Índice corrupto', 'Paso 1: reindexar')
  const oncall = await actor(playwright, baseURL, admin, 'oncall')
  const engineer = await actor(playwright, baseURL, admin, 'ingeniero')

  for (const ctx of [oncall.ctx, engineer.ctx, admin]) {
    expect((await runbooksOf(ctx, payments.id)).map((r) => r.title)).toEqual(['Pagos rechazados', 'Pool agotado'])
    expect((await runbooksOf(ctx, search.id)).map((r) => r.title)).toEqual(['Índice corrupto'])
  }

  await loginUi(page, oncall.email, password)
  await openService(page, payments.id)
  await expect(page.getByTestId('runbook-title')).toHaveText(['Pagos rechazados', 'Pool agotado'])
  await openService(page, search.id)
  await expect(page.getByTestId('runbook-title')).toHaveText(['Índice corrupto'])
})

test('UC-07.5 removing a runbook takes it out of the list and deletes its embedding (BR-12, BR-14)', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const svc = await createService(admin)
  const gone = await createRunbook(admin, svc.id, 'Certificado vencido', 'Paso 1: renovar el certificado')
  await createRunbook(admin, svc.id, 'Cola atascada', 'Paso 1: drenar la cola')

  await loginUi(page, adminEmail, adminPassword)
  await openService(page, svc.id)
  await row(page, 'Certificado vencido').getByTestId('runbook-delete').click()
  await expect(row(page, 'Certificado vencido')).toHaveCount(0)
  await expect(page.getByTestId('runbook-title')).toHaveText(['Cola atascada'])

  expect((await runbooksOf(admin, svc.id)).map((r) => r.title)).toEqual(['Cola atascada'])
  // The row and its embedding are gone, so the copilot (UC-10) has nothing left to retrieve.
  expect(await embeddingOf(gone.id)).toBeNull()
})

test('UC-07.6 a runbook without title or content is a validation error and is not created', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await adminActor(playwright, baseURL)
  const svc = await createService(admin)

  await loginUi(page, adminEmail, adminPassword)
  await openService(page, svc.id)
  await page.getByTestId('runbook-form-content').fill('Paso 1: sin título')
  await page.getByTestId('runbook-form-submit').click()
  await expect(page.getByTestId('runbooks-error')).toBeVisible()

  for (const data of [
    { title: 'Sin contenido', content: '' },
    { title: '   ', content: 'Paso 1' },
  ]) {
    expect((await admin.post(`/api/services/${svc.id}/runbooks`, { data })).status()).toBe(400)
  }
  expect(await runbooksOf(admin, svc.id)).toEqual([])
})
