import { test, expect } from '@playwright/test'

test.describe('manage policies', () => {
  test('policies page renders', async ({ page }) => {
    await page.goto('/manage/policies')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
