import { expect, test } from '@playwright/test'
import { confirm, login, unique } from './helpers'

test('requests created offline are queued, survive a reload and sync when back online', async ({ page, context }) => {
  await login(page, 'demo-requester-1')
  await page.goto('/requests')
  // Wait until the service worker controls the page so the app shell is cached.
  await page.evaluate(async () => {
    await navigator.serviceWorker.ready
  })
  await page.reload()
  await expect.poll(() => page.evaluate(() => Boolean(navigator.serviceWorker.controller))).toBe(true)

  await context.setOffline(true)
  await expect(page.getByTestId('connection-label')).toHaveText('Offline')

  // The app still starts without a network connection.
  await page.reload()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('My requests')
  await expect(page.getByTestId('connection-label')).toHaveText('Offline')

  const title = unique('Offline water')
  await page.goto('/requests/new')
  await page.getByTestId('category-drinking_water').click()
  await page.getByTestId('request-title').fill(title)
  await page.getByTestId('area-label').fill('Shelter without internet')
  await page.getByTestId('submit-request').click()
  await expect(page.getByText('Saved on this device').first()).toBeVisible()
  await expect(page.getByText('Waiting to sync').first()).toBeVisible()
  await expect(page.getByTestId('pending-link')).toContainText('1 waiting')

  // Nothing is lost on reload while still offline.
  await page.reload()
  await expect(page.getByTestId('pending-link')).toContainText('1 waiting')
  await page.goto('/sync')
  await expect(page.getByTestId('outbox-entry')).toContainText(title)

  // Back online: the change is pushed automatically and gets a reference.
  await context.setOffline(false)
  await expect(page.getByTestId('sync-empty')).toBeVisible({ timeout: 30_000 })
  await page.goto('/requests')
  await page.getByRole('link', { name: title }).click()
  await expect(page.getByTestId('request-reference')).toHaveText(/RM-\d{4}-\d{7}/)
  await expect(page.getByTestId('timeline')).toContainText('created offline')
})

test('volunteers can update task status offline', async ({ page, context }) => {
  await login(page, 'demo-volunteer-1')
  await page.goto('/assignments')
  await page.evaluate(async () => { await navigator.serviceWorker.ready })
  const card = page.getByTestId('assignment-card').filter({ hasText: 'Blankets for overnight stay' })
  await expect(card).toBeVisible()

  await context.setOffline(true)
  await expect(page.getByTestId('connection-label')).toHaveText('Offline')
  await card.getByTestId('assignment-in_progress').click()
  await confirm(page)
  await expect(page.getByText('Saved on this device').first()).toBeVisible()
  await expect(page.getByTestId('pending-link')).toContainText('1 waiting')

  await context.setOffline(false)
  await expect(page.getByTestId('pending-link')).toHaveCount(0, { timeout: 30_000 })
  await page.reload()
  await expect(page.getByTestId('assignment-card').filter({ hasText: 'Blankets for overnight stay' }).getByTestId('status-badge')).toContainText('In progress')
})
