import { test, expect } from '@playwright/test'

test.describe('topology', () => {
  test('topology page renders', async ({ page }) => {
    await page.goto('/manage/topology')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
