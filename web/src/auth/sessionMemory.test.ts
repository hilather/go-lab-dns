import { afterEach, describe, expect, it, vi } from 'vitest'
import { clear, getCsrf, setCsrf } from './sessionMemory'

describe('sessionMemory', () => {
  afterEach(() => {
    clear()
    vi.restoreAllMocks()
  })

  it('does not call localStorage.setItem', () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    setCsrf('abc')
    expect(getCsrf()).toBe('abc')
    expect(setItem).not.toHaveBeenCalled()
    clear()
    expect(getCsrf()).toBe('')
    expect(setItem).not.toHaveBeenCalled()
  })

  it('does not call sessionStorage.setItem', () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    setCsrf('secret')
    expect(setItem).not.toHaveBeenCalled()
  })
})
