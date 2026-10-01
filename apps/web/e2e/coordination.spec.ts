import { expect, test } from '@playwright/test'
import { confirm, createRequest, login, logout, selectByText, unique } from './helpers'

test('request lifecycle: review, manual assignment, delivery and confirmed resolution', async ({ page }) => {
  const title = unique('Water for gym')
  await login(page, 'demo-requester-2')
  await createRequest(page, { title, quantity: '30', unit: 'litres', exactAddress: 'Gym Street 4, side entrance', contact: '+49 30 555 0101' })
  await expect(page.getByTestId('request-reference')).toHaveText(/RM-/)
  const requestUrl = page.url()
  await logout(page)

  // Coordinator verifies and assigns manually.
  await login(page, 'demo-coordinator')
  await page.goto(requestUrl)
  await page.getByTestId('status-verified').click()
  await confirm(page)
  await expect(page.getByTestId('status-badge').first()).toContainText('Verified')

  await page.getByTestId('open-assign').click()
  const dialog = page.getByRole('dialog')
  await selectByText(dialog.getByTestId('assign-offer'), 'Bottled water from depot')
  await dialog.getByTestId('assign-quantity').fill('100000')
  await expect(dialog.getByTestId('over-allocation-warning')).toBeVisible()
  await dialog.getByTestId('assign-quantity').fill('30')
  await expect(dialog.getByTestId('over-allocation-warning')).toHaveCount(0)
  await selectByText(dialog.getByTestId('assign-volunteer'), 'Vera Volunteer')
  await dialog.getByTestId('grant-destination').check()
  await confirm(page)
  await expect(page.getByTestId('assignment-card')).toBeVisible()
  await expect(page.getByTestId('status-badge').first()).toContainText('Assigned')
  await logout(page)

  // Volunteer accepts, sees the granted destination, starts and delivers.
  await login(page, 'demo-volunteer-1')
  await page.getByRole('link', { name: title }).click()
  await page.getByTestId('assignment-accepted').click()
  await confirm(page)
  await page.getByTestId('reveal-protected').click()
  await confirm(page)
  await expect(page.getByTestId('revealed-address')).toHaveText('Gym Street 4, side entrance')
  // Contact details were not granted.
  await expect(page.getByTestId('revealed-contact')).toHaveCount(0)
  await page.getByTestId('assignment-in_progress').click()
  await confirm(page)
  await page.getByTestId('assignment-delivered').click()
  await expect(page.getByTestId('evidence-type')).toHaveValue('volunteer_confirmation')
  await confirm(page)
  await expect(page.getByTestId('status-badge').first()).toContainText('Delivered')
  await logout(page)

  // Delivery does not resolve the request; the coordinator confirms it.
  await login(page, 'demo-coordinator')
  await page.goto(requestUrl)
  await expect(page.getByTestId('status-badge').first()).toContainText('In progress')
  await page.getByTestId('status-resolved').click()
  await page.getByTestId('resolution-summary').fill('30 litres delivered to the gym.')
  await confirm(page)
  await expect(page.getByTestId('status-badge').first()).toContainText('Resolved')
  const timeline = page.getByTestId('timeline')
  await expect(timeline).toContainText('Assignment created')
  await expect(timeline).toContainText('Protected details viewed')
  await expect(timeline).toContainText('Resolved')
  await logout(page)

  // The requester sees the outcome and that their data was accessed, without names of staff.
  await login(page, 'demo-requester-2')
  await page.goto(requestUrl)
  await expect(page.getByTestId('status-badge').first()).toContainText('Resolved')
  await expect(page.getByTestId('timeline')).toContainText('Protected details viewed')
  await expect(page.getByTestId('timeline')).not.toContainText('Vera Volunteer')
})

test('cancel requires a reason and appears in the history', async ({ page }) => {
  const title = unique('Cancel me')
  await login(page, 'demo-coordinator')
  await createRequest(page, { title })
  await page.getByTestId('status-cancelled').click()
  await expect(page.getByRole('dialog').getByTestId('confirm-button')).toBeDisabled()
  await page.getByTestId('status-reason').fill('Need covered by the municipality')
  await confirm(page)
  await expect(page.getByTestId('status-badge').first()).toContainText('Cancelled')
  await expect(page.getByTestId('timeline')).toContainText('Need covered by the municipality')
  await expect(page.getByTestId('reopen')).toBeVisible()
})
