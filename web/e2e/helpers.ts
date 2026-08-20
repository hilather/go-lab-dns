import { expect, type Locator, type Page } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const repoRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..')

export type FixtureTokens = {
  admin: string
  viewer: string
}

type TokenFile = {
  tokens: { token: string; id: string; role: string }[]
}

export const RECORD_ID = 'e2e-www-a'
export const RECORD_OWNER = 'www'
export const RECORD_VALUE = {
  id: RECORD_ID,
  owner: RECORD_OWNER,
  type: 'A',
  values: ['10.42.0.80'],
}

export function loadFixtureTokens(): FixtureTokens {
  const raw = JSON.parse(readFileSync(join(repoRoot, 'testdata', 'web', 'tokens.json'), 'utf8')) as TokenFile
  const admin = raw.tokens.find((t) => t.role === 'administrator')?.token
  const viewer = raw.tokens.find((t) => t.role === 'viewer')?.token
  if (!admin || !viewer) {
    throw new Error('testdata/web/tokens.json must include administrator and viewer tokens')
  }
  return { admin, viewer }
}

export async function loginLoopback(page: Page): Promise<void> {
  await page.goto('/login')
  await page.getByRole('button', { name: 'Continue as local administrator' }).click()
  await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
}

export async function loginBearer(page: Page, token: string): Promise<void> {
  await page.goto('/login')
  await page.getByLabel('Bearer token').fill(token)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
}

export async function signOut(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'Sign out' }).click()
  await expect(page.getByRole('heading', { name: 'LabDNS' })).toBeVisible()
  await expect(page.getByLabel('Bearer token')).toBeVisible()
}

export async function expectNoSecretStorage(page: Page, token: string): Promise<void> {
  const dump = await page.evaluate(() => ({
    local: { ...window.localStorage },
    session: { ...window.sessionStorage },
    cookie: document.cookie,
  }))
  const blob = JSON.stringify(dump)
  expect(blob).not.toContain(token)
  expect(blob.toLowerCase()).not.toContain('csrf')
  expect(dump.cookie).not.toContain('labdns_session')
}

function parseColor(css: string): [number, number, number] | null {
  const m = css.match(/rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)/i)
  if (!m) {
    return null
  }
  return [Number(m[1]), Number(m[2]), Number(m[3])]
}

function channel(c: number): number {
  const s = c / 255
  return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
}

function contrastRatio(fg: [number, number, number], bg: [number, number, number]): number {
  const l1 = 0.2126 * channel(fg[0]) + 0.7152 * channel(fg[1]) + 0.0722 * channel(fg[2])
  const l2 = 0.2126 * channel(bg[0]) + 0.7152 * channel(bg[1]) + 0.0722 * channel(bg[2])
  const [hi, lo] = l1 > l2 ? [l1, l2] : [l2, l1]
  return (hi + 0.05) / (lo + 0.05)
}

export async function contrastAgainstBackground(el: Locator): Promise<number> {
  const ratio = await el.evaluate((node) => {
    const styleOf = (e: Element) => window.getComputedStyle(e)
    const fg = styleOf(node).color
    let bg = 'rgba(0, 0, 0, 0)'
    let cur: Element | null = node
    while (cur) {
      const c = styleOf(cur).backgroundColor
      if (c && !c.includes('0)') && c !== 'transparent') {
        bg = c
        break
      }
      cur = cur.parentElement
    }
    if (bg.includes('0)') || bg === 'transparent') {
      bg = 'rgb(255, 255, 255)'
    }
    return { fg, bg }
  })
  const fg = parseColor(ratio.fg)
  const bg = parseColor(ratio.bg)
  if (!fg || !bg) {
    throw new Error(`unparsed colors fg=${ratio.fg} bg=${ratio.bg}`)
  }
  return contrastRatio(fg, bg)
}
