import { randomUUID } from 'node:crypto'
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'

// UC-11.6 (a service with incidents or runbooks cannot be removed) cannot be checked in e2e
// today: incidents (UC-02) and runbooks (UC-07) cannot be created yet. It is covered by
// TestBR20_*, TestUC116_* and TestPostgres_FKBloqueaBajaEsErrInUse in the backend.

const adminEmail = process.env.ADMIN_EMAIL ?? ''
const adminPassword = process.env.ADMIN_PASSWORD ?? ''

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

function uniqueName(): string {
  return `svc-${randomUUID()}`
}

async function loginViaApi(ctx: APIRequestContext, email: string, password: string): Promise<void> {
  const res = await ctx.post('/api/auth/login', { data: { email, password } })
  expect(res.status()).toBe(200)
}

async function loginAsAdminUi(page: Page): Promise<void> {
  await page.goto('/')
  await page.getByTestId('login-email').fill(adminEmail)
  await page.getByTestId('login-password').fill(adminPassword)
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('services-list')).toBeVisible()
}

async function listViaApi(ctx: APIRequestContext): Promise<ServiceDto[]> {
  const res = await ctx.get('/api/services')
  expect(res.status()).toBe(200)
  return (await res.json()) as ServiceDto[]
}

async function createViaApi(ctx: APIRequestContext, name: string, criticality: string): Promise<ServiceDto> {
  const res = await ctx.post('/api/services', { data: { name, criticality } })
  expect(res.status()).toBe(201)
  return (await res.json()) as ServiceDto
}

async function adminContext(playwright: { request: { newContext: (o: { baseURL?: string }) => Promise<APIRequestContext> } }, baseURL: string | undefined) {
  const ctx = await playwright.request.newContext({ baseURL })
  await loginViaApi(ctx, adminEmail, adminPassword)
  return ctx
}

async function userContext(
  playwright: { request: { newContext: (o: { baseURL?: string }) => Promise<APIRequestContext> } },
  baseURL: string | undefined,
  admin: APIRequestContext,
  role: 'ingeniero' | 'oncall' | 'admin',
): Promise<APIRequestContext> {
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
  const ctx = await playwright.request.newContext({ baseURL })
  await loginViaApi(ctx, email, 'password-e2e')
  return ctx
}

function row(page: Page, name: string) {
  return page.locator(`[data-testid="service-row"][data-service-name="${name}"]`)
}

async function createViaUi(page: Page, name: string, criticality: string) {
  await page.getByTestId('service-name-input').fill(name)
  await page.getByTestId('service-criticality-select').selectOption(criticality)
  const [response] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/api/services') && r.request().method() === 'POST'),
    page.getByTestId('service-create-submit').click(),
  ])
  return response
}

test('UC-11.1 admin creates a service with the chosen criticality and no on-call', async ({ page }) => {
  const name = uniqueName()
  await loginAsAdminUi(page)
  const response = await createViaUi(page, name, 'importante')
  expect(response.status()).toBe(201)
  const body = (await response.json()) as ServiceDto
  expect(body.criticality).toBe('importante')
  expect(body.oncall_user_id).toBeNull()

  await expect(row(page, name)).toBeVisible()
  await expect(row(page, name).getByTestId('service-criticality')).toHaveText('importante')
  await expect(row(page, name).getByTestId('service-oncall')).toHaveText('Sin on-call')
})

test('UC-11.2 admin gets a validation error for an empty or duplicate name and nothing is created', async ({
  page,
  playwright,
  baseURL,
}) => {
  const name = uniqueName()
  await loginAsAdminUi(page)
  expect((await createViaUi(page, name, 'estandar')).status()).toBe(201)
  await expect(row(page, name)).toBeVisible()
  const admin = await adminContext(playwright, baseURL)

  // The database is shared with parallel tests, so nothing here compares total counts:
  // each check looks only at the names this test touches.

  // Duplicate name, different capitalization.
  const dup = await createViaUi(page, name.toUpperCase(), 'critica')
  expect(dup.status()).toBe(409)
  await expect(page.getByTestId('service-error')).toBeVisible()

  // Empty name (spaces only): the name input has no native "required", so the backend validates.
  const empty = await createViaUi(page, '   ', 'critica')
  expect(empty.status()).toBe(400)
  await expect(page.getByTestId('service-error')).toBeVisible()

  // Invalid criticality: the UI only offers valid values, so it is checked through the API.
  const badName = uniqueName()
  const bad = await admin.post('/api/services', { data: { name: badName, criticality: 'CRITICA' } })
  expect(bad.status()).toBe(400)

  const after = await listViaApi(admin)
  expect(after.filter((s) => s.name.toLowerCase() === name.toLowerCase())).toHaveLength(1)
  expect(after.some((s) => s.name.trim() === '')).toBe(false)
  expect(after.some((s) => s.name === badName)).toBe(false)
  await admin.dispose()
})

test('UC-11.3 admin edits a service; a duplicate name or invalid criticality leaves it unchanged', async ({
  page,
  playwright,
  baseURL,
}) => {
  const name = uniqueName()
  const other = uniqueName()
  const renamed = uniqueName()
  await loginAsAdminUi(page)
  expect((await createViaUi(page, name, 'estandar')).status()).toBe(201)
  expect((await createViaUi(page, other, 'estandar')).status()).toBe(201)

  await row(page, name).getByTestId('service-edit').click()
  await page.getByTestId('service-edit-name').fill(renamed)
  await page.getByTestId('service-edit-criticality').selectOption('critica')
  const [ok] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/api/services/') && r.request().method() === 'PUT'),
    page.getByTestId('service-edit-submit').click(),
  ])
  expect(ok.status()).toBe(200)
  await expect(row(page, renamed)).toBeVisible()
  await expect(row(page, renamed).getByTestId('service-criticality')).toHaveText('critica')
  await expect(row(page, name)).toHaveCount(0)

  await row(page, renamed).getByTestId('service-edit').click()
  await page.getByTestId('service-edit-name').fill(other.toUpperCase())
  const [conflict] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/api/services/') && r.request().method() === 'PUT'),
    page.getByTestId('service-edit-submit').click(),
  ])
  expect(conflict.status()).toBe(409)
  await expect(page.getByTestId('service-error')).toBeVisible()

  await page.reload()
  await expect(row(page, renamed)).toBeVisible()
  await expect(row(page, renamed).getByTestId('service-criticality')).toHaveText('critica')
  await expect(row(page, other)).toBeVisible()

  // Invalid criticality: the UI only offers valid values, so it is checked through the API.
  const admin = await adminContext(playwright, baseURL)
  const target = (await listViaApi(admin)).find((s) => s.name === renamed)
  expect(target).toBeDefined()
  const bad = await admin.put(`/api/services/${target?.id}`, { data: { name: renamed, criticality: 'CRITICA' } })
  expect(bad.status()).toBe(400)
  const unchanged = (await listViaApi(admin)).find((s) => s.name === renamed)
  expect(unchanged?.criticality).toBe('critica')
  await admin.dispose()
})

test('UC-11.4 only admin manages services; others get 403 and no session gets 401 (API)', async ({
  playwright,
  baseURL,
}) => {
  const admin = await adminContext(playwright, baseURL)
  const name = uniqueName()
  const svc = await createViaApi(admin, name, 'estandar')
  const asEngineer = await userContext(playwright, baseURL, admin, 'ingeniero')
  const asOncall = await userContext(playwright, baseURL, admin, 'oncall')
  const anon = await playwright.request.newContext({ baseURL })
  const attemptedNames: string[] = []

  const attempts: Array<[APIRequestContext, number]> = [
    [asEngineer, 403],
    [asOncall, 403],
    [anon, 401],
  ]
  for (const [ctx, status] of attempts) {
    const createdName = uniqueName()
    const updatedName = uniqueName()
    attemptedNames.push(createdName, updatedName)
    const created = await ctx.post('/api/services', { data: { name: createdName, criticality: 'critica' } })
    expect(created.status()).toBe(status)
    const updated = await ctx.put(`/api/services/${svc.id}`, { data: { name: updatedName, criticality: 'critica' } })
    expect(updated.status()).toBe(status)
    const deleted = await ctx.delete(`/api/services/${svc.id}`)
    expect(deleted.status()).toBe(status)
  }

  // The database is shared with other tests, so check only what this test touched.
  const after = await listViaApi(admin)
  expect(after.find((s) => s.id === svc.id)).toEqual(svc)
  expect(after.filter((s) => attemptedNames.includes(s.name))).toHaveLength(0)
  await Promise.all([admin, asEngineer, asOncall, anon].map((c) => c.dispose()))
})

test('UC-11.5 admin removes a service without dependencies', async ({ page }) => {
  const name = uniqueName()
  await loginAsAdminUi(page)
  expect((await createViaUi(page, name, 'estandar')).status()).toBe(201)
  await expect(row(page, name)).toBeVisible()

  const [response] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/api/services/') && r.request().method() === 'DELETE'),
    row(page, name).getByTestId('service-delete').click(),
  ])
  expect(response.status()).toBe(204)
  await expect(row(page, name)).toHaveCount(0)
  await page.reload()
  await expect(page.getByTestId('services-list')).toBeVisible()
  await expect(row(page, name)).toHaveCount(0)
})

test('UC-11.7 every authenticated role sees the service list; no session gets 401', async ({
  browser,
  playwright,
  baseURL,
}) => {
  const admin = await adminContext(playwright, baseURL)
  const name = uniqueName()
  await createViaApi(admin, name, 'importante')
  const asEngineer = await userContext(playwright, baseURL, admin, 'ingeniero')
  const asOncall = await userContext(playwright, baseURL, admin, 'oncall')
  const anon = await playwright.request.newContext({ baseURL })

  for (const ctx of [admin, asEngineer, asOncall]) {
    const found = (await listViaApi(ctx)).find((s) => s.name === name)
    expect(found?.criticality).toBe('importante')
    expect(found?.oncall_user_id).toBeNull()
  }
  expect((await anon.get('/api/services')).status()).toBe(401)

  // UI as an ingeniero: the list is visible and admin actions are not offered.
  const email = `user-${randomUUID()}@e2e.test`
  const reg = await anon.post('/api/auth/register', { data: { name: 'E2E User', email, password: 'password-e2e' } })
  expect(reg.status()).toBe(201)
  const page = await browser.newPage({ baseURL })
  await page.goto('/')
  await page.getByTestId('login-email').fill(email)
  await page.getByTestId('login-password').fill('password-e2e')
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('services-list')).toBeVisible()
  await expect(row(page, name)).toBeVisible()
  await expect(row(page, name).getByTestId('service-criticality')).toHaveText('importante')
  await expect(row(page, name).getByTestId('service-oncall')).toHaveText('Sin on-call')
  // Admin actions are presentation only (the backend enforces BR-12), but a non-admin must not be offered them.
  await expect(page.getByTestId('service-create-form')).toHaveCount(0)
  await expect(page.getByTestId('service-edit')).toHaveCount(0)
  await expect(page.getByTestId('service-delete')).toHaveCount(0)
  await page.close()

  await Promise.all([admin, asEngineer, asOncall, anon].map((c) => c.dispose()))
})
