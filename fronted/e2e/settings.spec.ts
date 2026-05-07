import { test, expect } from '@playwright/test'

test.describe('settings', () => {
  test('settings platform page renders', async ({ page }) => {
    await page.goto('/settings/platform')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })

  test('settings relays page renders', async ({ page }) => {
    await page.goto('/settings/relays')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
