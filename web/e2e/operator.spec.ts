import { expect, test, type Page } from '@playwright/test'
import {
  RECORD_ID,
  RECORD_VALUE,
  expectNoSecretStorage,
  loadFixtureTokens,
  loginBearer,
  loginLoopback,
  signOut,
} from './helpers'

test.describe.configure({ mode: 'serial' })

const tokens = loadFixtureTokens()

async function planAndApplyRecord(page: Page): Promise<number> {
  await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Changes' }).click()
  await expect(page.getByRole('heading', { name: 'Changes' })).toBeVisible()
  await page.getByRole('radio', { name: 'Operations' }).check()
  await page.getByRole('button', { name: 'Add operation' }).click()
  await page.getByRole('textbox', { name: 'ID', exact: true }).fill(RECORD_ID)
  await page.getByRole('textbox', { name: 'Zone ID' }).fill('lab-zone')
  await page.getByLabel('Value (JSON)').fill(JSON.stringify(RECORD_VALUE))
  await page.getByLabel('Reason (required to apply)').fill('e2e apply record')
  await page.getByRole('button', { name: 'Plan' }).click()
  await expect(page.getByRole('heading', { name: 'Plan' })).toBeVisible()
  await expect(page.getByText('dns.write')).toBeVisible()

  let applyPosts = 0
  await page.route('**/v1/changes:apply', async (route) => {
    applyPosts += 1
    await route.continue()
  })
  await page.getByRole('button', { name: 'Apply' }).click()
  // ConfirmDialog instances share aria-labelledby ids; locate by heading text.
  const confirm = page.locator('dialog.confirm-dialog').filter({
    has: page.getByRole('heading', { name: 'Apply this plan?' }),
  })
  await expect(confirm).toBeVisible()
  await confirm.getByRole('button', { name: 'Apply' }).dblclick()
  await expect(page.getByRole('heading', { name: 'Apply result' })).toBeVisible()
  await expect(page.getByText(/Applied/)).toBeVisible()
  expect(applyPosts).toBeGreaterThanOrEqual(1)
  expect(applyPosts).toBeLessThanOrEqual(2)
  await page.unroute('**/v1/changes:apply')
  return applyPosts
}

test.describe('operator matrix', () => {
  test('login, dashboard, plan/apply, export, resolve, chaos, reset, viewer 403', async ({ page }) => {
    await loginLoopback(page)
    await expect(page.getByRole('heading', { name: 'Process' })).toBeVisible()
    await expect(page.locator('.shell-header .revision')).toContainText(/Revision/)
    await expect(page.locator('.shell-header > .status')).toContainText(/Ready|Degraded|Not ready/)
    await expectNoSecretStorage(page, tokens.admin)

    await planAndApplyRecord(page)

    await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Zones' }).click()
    await expect(page.getByRole('link', { name: 'lab-zone' })).toBeVisible()
    await page.getByRole('link', { name: 'lab-zone' }).click()
    await expect(page.getByRole('link', { name: RECORD_ID })).toBeVisible()

    await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'State' }).click()
    const downloadPromise = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Download YAML' }).click()
    const download = await downloadPromise
    const exportPath = await download.path()
    expect(exportPath).toBeTruthy()
    const yaml = await download.createReadStream().then(async (s) => {
      if (!s) {
        return ''
      }
      const chunks: Buffer[] = []
      for await (const chunk of s) {
        chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk))
      }
      return Buffer.concat(chunks).toString('utf8')
    })
    expect(yaml).toContain(RECORD_ID)
    expect(yaml).toContain('10.42.0.80')

    await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Resolve' }).click()
    await page.getByLabel('Name').fill('ns1.lab.example.net.')
    await page.getByLabel('Type').fill('A')
    await page.getByRole('button', { name: 'Resolve' }).click()
    await expect(page.getByRole('heading', { name: 'Answer', exact: true })).toBeVisible()
    await expect(page.getByText('10.42.0.53')).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Explain' })).toBeVisible()
    await expect(page.getByLabel('Explain').getByText('exact')).toBeVisible()
    await expect(page.getByLabel('Explain').getByText(/lab-zone/)).toBeVisible()

    await page.getByRole('navigation', { name: 'Primary' }).getByRole('link', { name: 'Chaos' }).click()
    await expect(page.getByRole('heading', { name: 'Simulate' })).toBeVisible()
    await page.getByLabel('Name').fill('foo.tools.lab.example.net.')
    await page.getByLabel('Type').fill('A')
    await page.getByLabel('Client group').fill('test-devices')
    await page.getByRole('button', { name: 'Simulate' }).click()
    await expect(page.getByRole('heading', { name: 'Simulation' })).toBeVisible()
    await expect(page.getByText('Algorithm', { exact: true })).toBeVisible()
    await expect(page.getByText('Triggered', { exact: true })).toBeVisible()

    await expect(page.getByRole('button', { name: 'Emergency disable' })).toBeEnabled()
    await page.getByRole('button', { name: 'Emergency disable' }).click()
    const emergency = page.locator('dialog.confirm-dialog').filter({
      has: page.getByRole('heading', { name: 'Disable chaos?' }),
    })
    await expect(emergency).toBeVisible()
    await emergency.getByRole('button', { name: 'Disable' }).click()
    await expect(page.getByText('Chaos emergency disabled')).toBeVisible()
    const statusRes = await page.request.get('/v1/chaos/status')
    expect(statusRes.ok()).toBeTruthy()
    const status = (await statusRes.json()) as { emergencyDisabled?: boolean }
    expect(status.emergencyDisabled).toBe(true)

    await page.goto('/reset')
    await expect(page.getByRole('heading', { name: 'Reset' })).toBeVisible()
    await page.getByLabel('Confirmation').fill('primary-lab')
    await page.getByLabel('Reason (required)').fill('e2e reset bootstrap')
    await page.getByRole('button', { name: 'Reset to bootstrap' }).click()
    const resetDlg = page.locator('dialog.confirm-dialog').filter({
      has: page.getByRole('heading', { name: 'Reset to bootstrap?' }),
    })
    await expect(resetDlg).toBeVisible()
    await resetDlg.getByRole('button', { name: 'Reset' }).click()
    await expect(page.getByText('Reset applied.')).toBeVisible()

    await page.goto('/zones/lab-zone')
    await expect(page.getByRole('link', { name: 'ns1-a' })).toBeVisible()
    await expect(page.getByRole('link', { name: RECORD_ID })).toHaveCount(0)

    const exportAfter = await page.request.get('/v1/state:export?format=yaml')
    expect(exportAfter.ok()).toBeTruthy()
    expect(await exportAfter.text()).not.toContain(RECORD_ID)

    await signOut(page)
    await loginBearer(page, tokens.viewer)
    await expectNoSecretStorage(page, tokens.viewer)

    await page.goto('/changes')
    await expect(page.getByText('Missing scope dns.write').first()).toBeVisible()
    await expect(page.getByRole('button', { name: 'Apply' })).toBeDisabled()

    await page.goto('/reset')
    await expect(page.getByText('Missing scope dns.admin')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Reset to bootstrap' })).toBeDisabled()

    await expect(page.getByRole('button', { name: 'Emergency disable' })).toBeDisabled()
    await expect(page.getByText('Missing scope dns.chaos.emergency').first()).toBeVisible()

    const sess = await page.request.get('/v1/session')
    expect(sess.ok()).toBeTruthy()
    const body = (await sess.json()) as { csrf?: string }
    expect(body.csrf).toBeTruthy()
    const forced = await page.request.post('/v1/changes:apply', {
      headers: {
        'Content-Type': 'application/json',
        'X-LabDNS-CSRF': body.csrf ?? '',
      },
      data: {
        reason: 'viewer-force',
        operations: [
          {
            op: 'add',
            target: { kind: 'record', id: RECORD_ID, zoneId: 'lab-zone' },
            value: RECORD_VALUE,
          },
        ],
      },
    })
    expect(forced.status()).toBe(403)
  })
})
