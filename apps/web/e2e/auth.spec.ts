import { expect, test } from '@playwright/test'
import { login, logout } from './helpers'

test('login page always shows the emergency notice and exercise banner', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveURL(/\/login/)
  await expect(page.getByTestId('emergency-notice')).toContainText('112')
  await expect(page.getByTestId('emergency-notice')).toContainText('not an emergency dispatch service')
  await expect(page.getByTestId('exercise-banner')).toBeVisible()
})

test('wrong credentials show a generic error', async ({ page }) => {
  await page.goto('/login')
  await page.getByTestId('login-username').fill('demo-coordinator')
  await page.getByTestId('login-password').fill('not-the-password')
  await page.getByTestId('login-submit').click()
  await expect(page.getByTestId('login-error')).toContainText('Login failed')
})

test('each role lands on its own start page', async ({ page }) => {
  await login(page, 'demo-coordinator')
  await expect(page).toHaveURL(/\/dashboard/)
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Operational overview')
  await logout(page)

  await login(page, 'demo-volunteer-1')
  await expect(page).toHaveURL(/\/assignments/)
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('My tasks')
  await logout(page)

  await login(page, 'demo-requester-1')
  await expect(page).toHaveURL(/\/requests/)
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('My requests')
  await logout(page)

  await login(page, 'demo-admin')
  await expect(page).toHaveURL(/\/admin/)
  // Administrators have no operational access to requests.
  await page.goto('/requests')
  await expect(page).toHaveURL(/\/admin/)
})
