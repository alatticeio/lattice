import { test, expect } from '@playwright/test'

test.describe('manage nodes', () => {
  test('nodes page renders', async ({ page }) => {
    await page.goto('/manage/nodes')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
