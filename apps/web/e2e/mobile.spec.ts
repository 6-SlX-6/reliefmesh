import { expect, test } from '@playwright/test'
import { login } from './helpers'

test('mobile layout offers bottom navigation and large controls', async ({ page }) => {
  await login(page, 'demo-volunteer-1')
  const nav = page.getByRole('navigation', { name: 'Main' }).last()
  await expect(nav.getByRole('button', { name: 'Menu' })).toBeVisible()
  const box = await nav.getByRole('button', { name: 'Menu' }).boundingBox()
  expect(box!.height).toBeGreaterThanOrEqual(48)
  await expect(page.getByTestId('emergency-notice')).toBeVisible()
})
