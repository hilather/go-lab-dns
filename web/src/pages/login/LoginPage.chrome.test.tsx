import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { MemoryRouter } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import * as sessionApi from '../../auth/sessionApi'
import { LoginPage } from './LoginPage'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let root: Root | undefined
let host: HTMLDivElement | undefined

afterEach(async () => {
  if (root) {
    await act(async () => {
      root!.unmount()
    })
  }
  host?.remove()
  root = undefined
  host = undefined
  vi.restoreAllMocks()
})

describe('LoginPage chrome', () => {
  it('paints the charcoal login card with an accent Sign in button', async () => {
    vi.spyOn(sessionApi, 'getSession').mockResolvedValue(null)
    host = document.createElement('div')
    document.body.appendChild(host)
    root = createRoot(host)
    await act(async () => {
      root!.render(
        <MemoryRouter>
          <LoginPage />
        </MemoryRouter>,
      )
    })
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.querySelector('button.btn-accent')?.textContent).toBe('Sign in')
      })
    })
    expect(host?.querySelector('main.login')).not.toBeNull()
    expect(host?.querySelector('.login-card.surface')).not.toBeNull()
    expect(host?.querySelector('.page-lede')?.textContent).toBe('Sign in to the operator console.')
    expect(host?.querySelector('button[type="submit"]')?.classList.contains('btn-accent')).toBe(true)
  })
})
