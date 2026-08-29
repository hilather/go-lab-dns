import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))

function css(name: string): string {
  return readFileSync(join(here, name), 'utf8')
}

describe('operator chrome tokens', () => {
  const app = css('app.css')
  const changes = css('../pages/changes/changes.css')
  const resolve = css('../pages/resolve/resolve.module.css')
  const reset = css('../pages/reset/reset.css')
  const all = [app, changes, resolve, reset].join('\n')

  it('has no leftover paper borders, Segoe, or Google Fonts', () => {
    expect(all.toLowerCase()).not.toContain('#ccc')
    expect(all.toLowerCase()).not.toContain('segoe')
    expect(all.toLowerCase()).not.toContain('googleapis')
  })

  it('defines charcoal/amber tokens on both .login and .shell', () => {
    expect(app).toContain('.login')
    expect(app).toContain('.shell')
    expect(app).toContain('--bg: #0d0d0c')
    expect(app).toContain('--surface: #161614')
    expect(app).toContain('--text: #f2efe6')
    expect(app).toContain('--accent: #e09a3e')
    expect(app).toMatch(/\.login[\s\S]*--bg: #0d0d0c/)
    expect(app).toMatch(/\.login,\s*\n\.shell/)
  })

  it('exposes shared page primitives', () => {
    expect(app).toContain('.page-lede')
    expect(app).toContain('.surface')
    expect(app).toContain('.data-table')
    expect(app).toContain('.code-block')
    expect(app).toContain('.page-head')
  })
})
