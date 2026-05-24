import { test, expect } from '@playwright/test'

test.describe('visual regression', () => {
  test('dashboard page screenshot baseline', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')
    await expect(page).toHaveScreenshot('dashboard.png', {
      maxDiffPixelRatio: 0.05,
    })
  })
})
