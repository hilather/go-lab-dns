import { QueryClientProvider } from '@tanstack/react-query'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { MemoryRouter, Outlet, Route, Routes } from 'react-router'
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { client } from '../../api/client'
import { clear, setCsrf } from '../../auth/sessionMemory'
import { createQueryClient } from '../../query/client'
import type { StatusView } from '../../status'
import { ResetPage } from './ResetPage'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

beforeAll(() => {
  const proto = HTMLDialogElement.prototype
  if (typeof proto.showModal !== 'function') {
    proto.showModal = function showModal(this: HTMLDialogElement) {
      this.setAttribute('open', '')
    }
  }
  if (typeof proto.close !== 'function') {
    proto.close = function close(this: HTMLDialogElement) {
      this.removeAttribute('open')
    }
  }
})

const REV = 'sha256:aaaaaaaaaaaaaaaa'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function result(status: number, data?: unknown, error?: unknown) {
  const ok = status >= 200 && status < 300
  return {
    data: ok ? data : undefined,
    error: ok ? undefined : error,
    response: new Response(ok && data === undefined ? null : JSON.stringify(ok ? data : error), { status }),
  }
}

function setNativeValue(el: HTMLInputElement, value: string) {
  const proto = Object.getPrototypeOf(el) as HTMLInputElement
  const protoSetter = Object.getOwnPropertyDescriptor(proto, 'value')?.set
  const instSetter = Object.getOwnPropertyDescriptor(el, 'value')?.set
  if (protoSetter && instSetter !== protoSetter) {
    protoSetter.call(el, value)
  } else if (protoSetter) {
    protoSetter.call(el, value)
  } else {
    el.value = value
  }
  el.dispatchEvent(new Event('input', { bubbles: true }))
}

let root: Root | undefined
let host: HTMLDivElement | undefined
let resetBodies: unknown[]
let actorScopes: string[]
let actorRole: string
let stateBody: unknown
let resetStatus: number
let resetError: unknown

async function renderPage(opts?: { status?: StatusView | null }) {
  const status: StatusView | null =
    opts?.status === undefined ? { revisions: { runtimeRevision: REV } } : opts.status
  host = document.createElement('div')
  document.body.appendChild(host)
  root = createRoot(host)
  await act(async () => {
    root!.render(
      <QueryClientProvider client={createQueryClient()}>
        <MemoryRouter initialEntries={['/reset']}>
          <Routes>
            <Route element={<Outlet context={{ status }} />}>
              <Route path="/reset" element={<ResetPage />} />
            </Route>
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )
  })
  await act(async () => {
    await vi.waitFor(() => {
      expect(host?.textContent).toContain('Type this phrase')
    })
  })
  await act(async () => {
    await Promise.resolve()
  })
  await act(async () => {
    await Promise.resolve()
  })
}

afterEach(async () => {
  if (root) {
    await act(async () => {
      root!.unmount()
    })
  }
  host?.remove()
  root = undefined
  host = undefined
  clear()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

beforeEach(() => {
  resetBodies = []
  actorScopes = ['dns.admin', 'dns.read', 'dns.write']
  actorRole = 'administrator'
  stateBody = {
    runtimeRevision: REV,
    canonical: { metadata: { name: 'lab-dns' }, spec: {} },
  }
  resetStatus = 200
  resetError = { code: 'forbidden', detail: 'missing scope dns.admin' }
  setCsrf('csrf-test')
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input instanceof Request ? input.url : String(input)
      if (url.includes('/v1/session')) {
        return jsonResponse(200, {
          csrf: 'csrf-test',
          actor: { id: 'admin', class: 'ui-session', role: actorRole, scopes: actorScopes },
        })
      }
      return jsonResponse(404, { code: 'not_found' })
    }),
  )
  vi.spyOn(client, 'GET').mockImplementation((async (path: unknown) => {
    if (path === '/v1/state') {
      return result(200, stateBody)
    }
    return result(404, undefined, { code: 'not_found' })
  }) as typeof client.GET)
  vi.spyOn(client, 'POST').mockImplementation((async (path: unknown, init?: unknown) => {
    const body = (init as { body?: unknown } | undefined)?.body
    if (path === '/v1/state:reset') {
      resetBodies.push(body)
      if (resetStatus >= 400) {
        return result(resetStatus, undefined, resetError)
      }
      return result(200, {
        applied: true,
        previousRevision: REV,
        candidateRevision: 'sha256:bbbbbbbbbbbbbbbb',
        generation: 3,
        drifted: false,
        auditEventId: 'evt-reset',
      })
    }
    return result(404, undefined, { code: 'not_found' })
  }) as typeof client.POST)
})

function actionButton(label: string): HTMLButtonElement {
  const buttons = [...document.querySelectorAll('button')]
  const found = buttons.find((b) => b.textContent === label)
  if (!found) {
    throw new Error(`button ${label} missing`)
  }
  return found as HTMLButtonElement
}

function labeledInput(text: string): HTMLInputElement {
  const labels = [...document.querySelectorAll('label')]
  const label = labels.find((l) => l.textContent?.includes(text))
  const input = label?.querySelector('input') as HTMLInputElement | null
  if (!input) {
    throw new Error(`${text} input missing`)
  }
  return input
}

async function fillRequired(phrase: string, reason = 'restore bootstrap') {
  await act(async () => {
    setNativeValue(labeledInput('Confirmation'), phrase)
    setNativeValue(labeledInput('Reason'), reason)
  })
}

describe('ResetPage', () => {
  it('keeps reset disabled until the compiled metadata name is typed', async () => {
    await renderPage()
    expect(host?.textContent).toContain('lab-dns')
    expect(host?.textContent).toContain(REV.slice('sha256:'.length, 'sha256:'.length + 12))
    expect(actionButton('Reset to bootstrap').disabled).toBe(true)
    await fillRequired('wrong')
    expect(actionButton('Reset to bootstrap').disabled).toBe(true)
    await fillRequired('lab-dns')
    expect(actionButton('Reset to bootstrap').disabled).toBe(false)
  })

  it('requires the literal RESET when compiled metadata name is empty', async () => {
    stateBody = { runtimeRevision: REV, canonical: { metadata: { name: '' }, spec: {} } }
    await renderPage()
    expect(host?.textContent).toContain('RESET')
    await fillRequired('lab-dns')
    expect(actionButton('Reset to bootstrap').disabled).toBe(true)
    await fillRequired('RESET')
    expect(actionButton('Reset to bootstrap').disabled).toBe(false)
  })

  it('keeps reset disabled when reason is empty', async () => {
    await renderPage()
    await act(async () => {
      setNativeValue(labeledInput('Confirmation'), 'lab-dns')
    })
    expect(actionButton('Reset to bootstrap').disabled).toBe(true)
  })

  it('gates reset on dns.admin and names the missing scope', async () => {
    actorRole = 'viewer'
    actorScopes = ['dns.read']
    await renderPage()
    await fillRequired('lab-dns')
    expect(actionButton('Reset to bootstrap').disabled).toBe(true)
    expect(host?.textContent).toContain('Missing scope dns.admin')
    await act(async () => {
      actionButton('Reset to bootstrap').click()
    })
    expect(resetBodies).toHaveLength(0)
    expect(document.querySelector('dialog.confirm-dialog')?.hasAttribute('open')).toBeFalsy()
  })

  it('posts reset once and blocks a second submit while in flight', async () => {
    await renderPage()
    await fillRequired('lab-dns')
    await act(async () => {
      actionButton('Reset to bootstrap').click()
    })
    const confirm = document.querySelector('dialog button[type="submit"]') as HTMLButtonElement
    expect(document.querySelector('dialog.confirm-dialog')?.hasAttribute('open')).toBe(true)
    await act(async () => {
      confirm.click()
      confirm.click()
    })
    expect(resetBodies).toHaveLength(1)
    expect(resetBodies[0]).toEqual({ reason: 'restore bootstrap' })
    expect(host?.textContent).toContain('evt-reset')
    expect(host?.textContent).toContain('Reset applied')
  })

  it('announces problem+json when reset is forbidden', async () => {
    resetStatus = 403
    await renderPage()
    await fillRequired('lab-dns')
    await act(async () => {
      actionButton('Reset to bootstrap').click()
    })
    const confirm = document.querySelector('dialog button[type="submit"]') as HTMLButtonElement
    await act(async () => {
      confirm.click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect(host?.querySelector('[role="alert"]')?.textContent).toBe('forbidden: missing scope dns.admin')
    expect(resetBodies).toHaveLength(1)
  })
})
