import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { loginAsAdminUi } from './helpers'

const adminEmail = process.env.ADMIN_EMAIL ?? ''
const adminPassword = process.env.ADMIN_PASSWORD ?? ''

test('UI dashboard: a declared incident shows in the KPIs and opens in the Incidents view', async ({
  page,
  playwright,
  baseURL,
}) => {
  const admin = await playwright.request.newContext({ baseURL })
  expect(
    (await admin.post('/api/auth/login', { data: { email: adminEmail, password: adminPassword } })).status(),
  ).toBe(200)
  const svc = await admin.post('/api/services', { data: { name: `svc-${randomUUID()}`, criticality: 'importante' } })
  expect(svc.status()).toBe(201)
  const { id: serviceId } = (await svc.json()) as { id: string }
  // No on-call for the service, so the incident is unassigned and listed under "Requieren atención".
  const declared = await admin.post('/api/incidents', {
    data: { title: `inc-${randomUUID()}`, service_id: serviceId, impact: 'degradacion', severity: 'SEV2' },
  })
  expect(declared.status()).toBe(201)
  const incidentId = ((await declared.json()) as { incident: { id: string } }).incident.id
  await admin.dispose()

  await loginAsAdminUi(page)
  await expect(page.getByTestId('kpi-dashboard')).toBeVisible()
  await expect(page.getByTestId('kpi-active')).not.toHaveText('0')

  const attention = page.locator(`[data-testid="kpi-attention-row"][data-incident-id="${incidentId}"]`)
  await expect(attention).toBeVisible()
  await attention.getByTestId('kpi-attention-open').click()
  await expect(page.getByTestId('incident-board')).toBeVisible()
  await expect(page.getByTestId('incident-detail-title')).toBeVisible()
})

test('UI dashboard: a route without session still shows the login (UC-01.4)', async ({ page }) => {
  await page.goto('/cualquier/ruta')
  await expect(page.getByTestId('login-submit')).toBeVisible()
  await expect(page.getByTestId('app-home')).toHaveCount(0)
})
