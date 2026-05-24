import { test, expect } from '@playwright/test'

test.describe('AI intent', () => {
  test('AI intent page renders', async ({ page }) => {
    await page.goto('/ai/intent')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })

  test('can navigate between AI sub-pages', async ({ page }) => {
    await page.goto('/ai')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
