import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const tsconfigPath = join(dirname(fileURLToPath(import.meta.url)), '..', 'tsconfig.json')

describe('web tsconfig exclude', () => {
  it('keeps Vitest files out of tsc --noEmit / npm run build', () => {
    const cfg = JSON.parse(readFileSync(tsconfigPath, 'utf8')) as { exclude?: unknown }
    expect(Array.isArray(cfg.exclude)).toBe(true)
    const exclude = cfg.exclude as string[]
    expect(exclude).toContain('src/**/*.test.ts')
    expect(exclude).toContain('src/**/*.test.tsx')
  })
})
