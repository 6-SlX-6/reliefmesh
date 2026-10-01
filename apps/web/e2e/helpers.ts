import { type Locator, type Page, expect } from '@playwright/test'

export const DEMO_PASSWORD = 'reliefmesh-demo-exercise'

export async function login(page: Page, username: string, password = DEMO_PASSWORD) {
  await page.goto('/login')
  await page.getByTestId('login-username').fill(username)
  await page.getByTestId('login-password').fill(password)
  await page.getByTestId('login-submit').click()
  await expect(page).not.toHaveURL(/\/login/)
}

export async function logout(page: Page) {
  await page.goto('/account')
  await page.getByTestId('logout').click()
  // Confirm if there are unsynchronized changes.
  const dialogButton = page.getByRole('dialog').getByTestId('confirm-button')
  if (await dialogButton.isVisible().catch(() => false)) await dialogButton.click()
  await expect(page).toHaveURL(/\/login/)
}

/** Clicks the confirm button of the currently open dialog. */
export async function confirm(page: Page) {
  await page.getByRole('dialog').getByTestId('confirm-button').click()
}

export function unique(prefix: string) {
  return `${prefix} ${Date.now().toString(36)}${Math.random().toString(36).slice(2, 5)}`
}

/** Fills the minimal request form and submits it. */
export async function createRequest(page: Page, opts: { title: string; category?: string; urgency?: string; area?: string; quantity?: string; unit?: string; exactAddress?: string; contact?: string }) {
  await page.goto('/requests/new')
  await page.getByTestId(`category-${opts.category ?? 'drinking_water'}`).click()
  await page.getByTestId(`urgency-${opts.urgency ?? 'normal'}`).click()
  await page.getByTestId('request-title').fill(opts.title)
  await page.getByTestId('request-description').fill('Created by an end-to-end test.')
  if (opts.quantity) await page.getByTestId('request-quantity').fill(opts.quantity)
  if (opts.unit) await page.getByTestId('request-unit').fill(opts.unit)
  if (opts.exactAddress) {
    await page.getByTestId('location-protected_exact').click()
    await page.getByTestId('area-label').fill(opts.area ?? 'Test district')
    await page.getByTestId('exact-address').fill(opts.exactAddress)
  } else {
    await page.getByTestId('area-label').fill(opts.area ?? 'Test district')
  }
  if (opts.contact) {
    await page.getByTestId('contact-assigned_responders').click()
    await page.getByTestId('contact-details').fill(opts.contact)
  }
  await page.getByTestId('submit-request').click()
  await page.waitForURL(/\/requests\/(?!new)[^/]+$/)
}

/** Selects the first <option> whose text contains `text`. */
export async function selectByText(select: Locator, text: string) {
  const value = await select.locator('option', { hasText: text }).first().getAttribute('value')
  await select.selectOption(value ?? '')
}
