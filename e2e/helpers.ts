import { expect, type Page } from '@playwright/test'

export type NavView = 'dashboard' | 'incidents' | 'services' | 'metrics'

// Shared UI login; lands on the app shell (dashboard view).
export async function loginUi(page: Page, email: string, password: string): Promise<void> {
  await page.goto('/')
  await page.getByTestId('login-email').fill(email)
  await page.getByTestId('login-password').fill(password)
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('app-home')).toBeVisible()
}

export async function loginAsAdminUi(page: Page): Promise<void> {
  await loginUi(page, process.env.ADMIN_EMAIL ?? '', process.env.ADMIN_PASSWORD ?? '')
}

export async function goTo(page: Page, view: NavView): Promise<void> {
  await page.getByTestId(`nav-${view}`).click()
}

export async function openDeclare(page: Page): Promise<void> {
  await page.getByTestId('declare-incident-open').click()
}

export async function openRunbooks(page: Page): Promise<void> {
  await page.getByTestId('runbooks-open').click()
}

export async function openCopilot(page: Page): Promise<void> {
  await page.getByTestId('copilot-open').click()
}
