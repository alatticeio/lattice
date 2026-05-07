import { test, expect } from '@playwright/test'

test.describe('workspace', () => {
  test('workspaces page renders', async ({ page }) => {
    await page.goto('/manage/workspaces')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })

  test('members page renders', async ({ page }) => {
    await page.goto('/manage/members')
    await page.waitForLoadState('networkidle')
    expect(await page.locator('body').innerHTML()).toBeTruthy()
  })
})
