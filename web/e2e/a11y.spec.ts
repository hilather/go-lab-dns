import { expect, test } from '@playwright/test'
import { contrastAgainstBackground, expectNoSecretStorage, loadFixtureTokens } from './helpers'

test.describe('login a11y and CSP', () => {
  test('login HTML has labels, keyboard path, alert, contrast, and no inline styles', async ({
    page,
  }) => {
    const res = await page.goto('/login')
    expect(res, 'login must be served').not.toBeNull()
    expect(res?.ok()).toBeTruthy()

    const csp = res?.headers()['content-security-policy'] ?? ''
    expect(csp).toContain("style-src 'self'")
    expect(csp).not.toContain('unsafe-inline')
    expect(csp).toContain("script-src 'self'")

    const html = await page.content()
    expect(html).not.toMatch(/\sstyle\s*=/)
    expect(html).not.toMatch(/<style[\s>]/i)

    await expect(page.getByLabel('Bearer token')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Sign in' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Continue as local administrator' })).toBeVisible()

    await page.getByRole('button', { name: 'Sign in' }).click()
    await expect(page.getByRole('alert')).toContainText('bearer token is required')
    expect(await contrastAgainstBackground(page.getByRole('alert'))).toBeGreaterThanOrEqual(4.5)
    expect(await contrastAgainstBackground(page.getByRole('heading', { name: 'LabDNS' }))).toBeGreaterThanOrEqual(
      4.5,
    )

    await page.goto('/login')
    const continueBtn = page.getByRole('button', { name: 'Continue as local administrator' })
    await expect(continueBtn).toBeEnabled()
    await continueBtn.focus()
    await expect(continueBtn).toBeFocused()
    await page.keyboard.press('Enter')
    await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
  })

  test('bearer login does not persist the token in web storage', async ({ page }) => {
    const tokens = loadFixtureTokens()
    await page.goto('/login')
    await page.getByLabel('Bearer token').fill(tokens.admin)
    await page.getByRole('button', { name: 'Sign in' }).click()
    await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
    await expectNoSecretStorage(page, tokens.admin)
  })
})
