import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'
import { login } from './helpers'

async function audit(page: import('@playwright/test').Page) {
  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze()
  const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')
  expect(serious.map((v) => `${v.id}: ${v.help} (${v.nodes.length})`)).toEqual([])
}

test('login page has no serious accessibility violations', async ({ page }) => {
  await page.goto('/login')
  await expect(page.getByTestId('login-username')).toBeVisible()
  await audit(page)
})

test('main coordinator pages have no serious accessibility violations', async ({ page }) => {
  await login(page, 'demo-coordinator')
  for (const path of ['/dashboard', '/requests', '/requests/new', '/offers', '/assignments', '/sync']) {
    await page.goto(path)
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
    await page.waitForLoadState('networkidle')
    await audit(page)
  }
})

test('keyboard users can skip to main content', async ({ page }) => {
  await login(page, 'demo-requester-1')
  // The skip link is the first focusable element on every page.
  const first = await page.evaluate(() => {
    const el = document.querySelector<HTMLElement>('a[href], button, input, select, textarea, [tabindex]:not([tabindex="-1"])')
    return el?.textContent?.trim()
  })
  expect(first).toBe('Skip to main content')
  const skip = page.getByRole('link', { name: 'Skip to main content' })
  await skip.focus()
  await expect(skip).toBeVisible()
  await page.keyboard.press('Enter')
  await expect(page.locator('#main')).toBeFocused()
})
