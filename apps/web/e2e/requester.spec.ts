import { expect, test } from '@playwright/test'
import { confirm, createRequest, login, logout, unique } from './helpers'

test('critical requests require reading the emergency notice', async ({ page }) => {
  await login(page, 'demo-requester-1')
  const title = unique('Critical water')
  await page.goto('/requests/new')
  await page.getByTestId('category-drinking_water').click()
  await page.getByTestId('urgency-critical').click()
  await page.getByTestId('request-title').fill(title)
  await page.getByTestId('area-label').fill('Riverside')
  await page.getByTestId('submit-request').click()
  const notice = page.getByTestId('critical-notice')
  await expect(notice).toBeVisible()
  await expect(notice).toContainText('emergency services')
  await expect(page.getByRole('dialog')).toContainText('does not notify emergency services')
  await confirm(page)
  await expect(page.getByTestId('request-reference')).toHaveText(/RM-\d{4}-\d{7}/)
  await expect(page.getByTestId('status-badge').first()).toContainText('Submitted')
  await expect(page.getByTestId('urgency-badge').first()).toContainText('Critical')
})

test('medicine pickup requests are logistics only', async ({ page }) => {
  await login(page, 'demo-requester-1')
  await page.goto('/requests/new')
  await page.getByTestId('category-medicine_pickup').click()
  await expect(page.getByTestId('medicine-notice')).toContainText('Do not enter diagnoses')
  await expect(page.getByTestId('request-title')).toHaveCount(0)
  await expect(page.getByTestId('request-description')).toHaveCount(0)
  await page.getByTestId('requires-authorization').check()
  await page.getByTestId('area-label').fill('Riverside community hall')
  await page.getByTestId('submit-request').click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Medicine pickup (logistics only)')
  await expect(page.getByText('Required for pickup')).toBeVisible()
})

test('requesters only see their own requests', async ({ page }) => {
  await login(page, 'demo-requester-1')
  const title = unique('Own blankets')
  await createRequest(page, { title, category: 'blankets' })
  const url = page.url()
  await logout(page)

  await login(page, 'demo-requester-2')
  await page.goto('/requests')
  await expect(page.getByText(title)).toHaveCount(0)
  await page.goto(url)
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Request not found')
})
