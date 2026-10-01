import { randomUUID } from 'node:crypto'
import { expect, test, type APIRequestContext } from '@playwright/test'

// UC-01.6 "if an admin already exists, no other is created" cannot be checked in e2e
// without restarting the app; it is covered by TestBR19_PrimerAdminDesdeEntorno and
// the manual test from T2. Only the "admin from env can log in" half is checked here.

const adminEmail = process.env.ADMIN_EMAIL ?? ''
const adminPassword = process.env.ADMIN_PASSWORD ?? ''

interface UserDto {
  id: string
  name: string
  email: string
  role: string
}

function uniqueEmail(): string {
  return `user-${randomUUID()}@e2e.test`
}

async function registerViaApi(request: APIRequestContext, email: string, password: string): Promise<UserDto> {
  const res = await request.post('/api/auth/register', { data: { name: 'E2E User', email, password } })
  expect(res.status()).toBe(201)
  return (await res.json()) as UserDto
}

async function loginViaApi(request: APIRequestContext, email: string, password: string): Promise<number> {
  const res = await request.post('/api/auth/login', { data: { email, password } })
  return res.status()
}

async function meViaApi(request: APIRequestContext): Promise<UserDto> {
  const res = await request.get('/api/me')
  expect(res.status()).toBe(200)
  return (await res.json()) as UserDto
}

test('UC-01.1 registration creates an ingeniero without password_hash', async ({ page }) => {
  const email = uniqueEmail()
  await page.goto('/')
  await page.getByTestId('show-register').click()
  await page.getByTestId('register-name').fill('Ana Test')
  await page.getByTestId('register-email').fill(email)
  await page.getByTestId('register-password').fill('s3cret-pass')
  const [response] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/api/auth/register')),
    page.getByTestId('register-submit').click(),
  ])
  expect(response.status()).toBe(201)
  const body = (await response.json()) as Record<string, unknown>
  expect(body.role).toBe('ingeniero')
  expect(body).not.toHaveProperty('password_hash')
  await expect(page.getByTestId('register-success')).toBeVisible()

  await page.getByTestId('login-email').fill(email)
  await page.getByTestId('login-password').fill('s3cret-pass')
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('current-user-role')).toHaveText('ingeniero')
})

test('UC-01.2 duplicate email is rejected and no second user is created', async ({ page, request, playwright, baseURL }) => {
  const email = uniqueEmail()
  await registerViaApi(request, email, 'password-one')

  await page.goto('/')
  await page.getByTestId('show-register').click()
  await page.getByTestId('register-name').fill('Otro')
  await page.getByTestId('register-email').fill(email)
  await page.getByTestId('register-password').fill('password-two')
  const [response] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/api/auth/register')),
    page.getByTestId('register-submit').click(),
  ])
  expect(response.status()).toBe(409)
  await expect(page.getByTestId('register-error')).toBeVisible()

  const fresh = await playwright.request.newContext({ baseURL })
  expect(await loginViaApi(fresh, email, 'password-two')).toBe(401)
  expect(await loginViaApi(fresh, email, 'password-one')).toBe(200)
  await fresh.dispose()
})

test('UC-01.3 valid login opens a session; wrong password does not', async ({ page, request, context }) => {
  const email = uniqueEmail()
  await registerViaApi(request, email, 'right-password')

  await page.goto('/')
  await page.getByTestId('login-email').fill(email)
  await page.getByTestId('login-password').fill('right-password')
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('app-home')).toBeVisible()
  const session = (await context.cookies()).find((c) => c.name === 'session')
  expect(session?.httpOnly).toBe(true)
  await page.reload()
  await expect(page.getByTestId('app-home')).toBeVisible()
})

test('UC-01.3 wrong password is rejected and creates no session', async ({ page, request, context }) => {
  const email = uniqueEmail()
  await registerViaApi(request, email, 'right-password')

  await page.goto('/')
  await page.getByTestId('login-email').fill(email)
  await page.getByTestId('login-password').fill('wrong-password')
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('login-error')).toBeVisible()
  expect((await context.cookies()).some((c) => c.name === 'session')).toBe(false)
  await expect(page.getByTestId('app-home')).toHaveCount(0)
})

test('UC-01.4 visitor without session gets login and 401, with no effects', async ({ page, request }) => {
  const user = await registerViaApi(request, uniqueEmail(), 'some-password')

  await page.goto('/')
  await expect(page.getByTestId('login-form')).toBeVisible()
  await expect(page.getByTestId('app-home')).toHaveCount(0)
  await page.goto('/cualquier/ruta')
  await expect(page.getByTestId('login-form')).toBeVisible()
  await expect(page.getByTestId('app-home')).toHaveCount(0)

  expect((await request.get('/api/me')).status()).toBe(401)
  const patch = await request.patch(`/api/users/${user.id}/role`, { data: { role: 'admin' } })
  expect(patch.status()).toBe(401)

  expect(await loginViaApi(request, user.email, 'some-password')).toBe(200)
  expect((await meViaApi(request)).role).toBe('ingeniero')
})

test('UC-01.5 only admin changes roles (API)', async ({ playwright, baseURL }) => {
  const newCtx = () => playwright.request.newContext({ baseURL })
  const anon = await newCtx()
  const emailA = uniqueEmail()
  const emailB = uniqueEmail()
  const a = await registerViaApi(anon, emailA, 'password-a')
  const b = await registerViaApi(anon, emailB, 'password-b')

  const admin = await newCtx()
  expect(await loginViaApi(admin, adminEmail, adminPassword)).toBe(200)
  const asA = await newCtx()
  expect(await loginViaApi(asA, emailA, 'password-a')).toBe(200)
  const asB = await newCtx()
  expect(await loginViaApi(asB, emailB, 'password-b')).toBe(200)

  const setRole = (ctx: APIRequestContext, id: string, role: string) =>
    ctx.patch(`/api/users/${id}/role`, { data: { role } })

  expect((await setRole(admin, a.id, 'oncall')).status()).toBe(200)
  expect((await meViaApi(asA)).role).toBe('oncall')
  expect((await setRole(admin, a.id, 'admin')).status()).toBe(200)
  expect((await meViaApi(asA)).role).toBe('admin')

  expect((await setRole(asB, a.id, 'ingeniero')).status()).toBe(403)
  expect((await meViaApi(asA)).role).toBe('admin')

  expect((await setRole(admin, b.id, 'oncall')).status()).toBe(200)
  expect((await setRole(asB, a.id, 'ingeniero')).status()).toBe(403)
  expect((await meViaApi(asA)).role).toBe('admin')

  await Promise.all([anon, admin, asA, asB].map((c) => c.dispose()))
})

test('UC-01.6 admin from environment can log in', async ({ page }) => {
  await page.goto('/')
  await page.getByTestId('login-email').fill(adminEmail)
  await page.getByTestId('login-password').fill(adminPassword)
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('current-user-role')).toHaveText('admin')
})
