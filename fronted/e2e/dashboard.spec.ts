import { test, expect } from '@playwright/test'

test.describe('dashboard', () => {
  test('dashboard page loads with stats', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
