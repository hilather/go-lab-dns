import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { client } from '../../api/client'
import { EmergencyControl } from './EmergencyControl'
import { EMERGENCY_REASON } from './view'

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

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function ok(data: unknown) {
  return { data, error: undefined, response: jsonResponse(200, data) }
}

function fail(status: number, body: unknown) {
  return { data: undefined, error: body, response: jsonResponse(status, body) }
}

function buttonNamed(label: string): HTMLButtonElement | undefined {
  return [...(host?.querySelectorAll('button') ?? [])].find((b) => b.textContent === label)
}

const adminSession = {
  csrf: 'csrf',
  actor: {
    id: 'admin',
    class: 'ui-session',
    role: 'administrator',
    scopes: ['dns.admin', 'dns.chaos.activate', 'dns.chaos.emergency'],
  },
}

const emergencyOnly = {
  csrf: 'csrf',
  actor: {
    id: 'emerg',
    class: 'ui-session',
    role: 'emergency-operator',
    scopes: ['dns.read', 'dns.chaos.read', 'dns.chaos.emergency'],
  },
}

const viewerSession = {
  csrf: 'csrf',
  actor: {
    id: 'viewer',
    class: 'ui-session',
    role: 'viewer',
    scopes: ['dns.read', 'dns.chaos.read'],
  },
}

let root: Root | undefined
let host: HTMLDivElement | undefined

async function render(ui: ReactNode) {
  host = document.createElement('div')
  document.body.appendChild(host)
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
  })
  root = createRoot(host)
  await act(async () => {
    root!.render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
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
  vi.restoreAllMocks()
})

function mockSession(session: unknown) {
  vi.spyOn(client, 'GET').mockImplementation((async (path: string) => {
    if (path === '/v1/session') {
      return ok(session)
    }
    return fail(404, { code: 'not_found', detail: path })
  }) as typeof client.GET)
}

describe('EmergencyControl', () => {
  it('disables emergency actions for a viewer and names dns.chaos.emergency', async () => {
    mockSession(viewerSession)
    const post = vi.spyOn(client, 'POST')
    await render(<EmergencyControl emergencyDisabled={false} />)
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('Missing scope dns.chaos.emergency')
      })
    })
    expect(buttonNamed('Emergency disable')?.disabled).toBe(true)
    expect(buttonNamed('Emergency enable')?.disabled).toBe(true)
    expect(post).not.toHaveBeenCalled()
  })

  it('lets an emergency-only operator disable but not re-enable', async () => {
    mockSession(emergencyOnly)
    await render(<EmergencyControl emergencyDisabled={false} />)
    await act(async () => {
      await vi.waitFor(() => {
        expect(buttonNamed('Emergency disable')?.disabled).toBe(false)
      })
    })
    expect(host?.textContent).toContain('Missing scope dns.chaos.activate')
    expect(buttonNamed('Emergency enable')?.disabled).toBe(true)
  })

  it('confirms emergency disable once without a typed phrase', async () => {
    mockSession(adminSession)
    const posts: { path: string; body: unknown }[] = []
    vi.spyOn(client, 'POST').mockImplementation((async (path: string, init?: unknown) => {
      posts.push({ path, body: (init as { body?: unknown } | undefined)?.body })
      return ok({ applied: true, candidateRevision: 'sha256:x' })
    }) as typeof client.POST)
    await render(<EmergencyControl emergencyDisabled={false} />)
    await act(async () => {
      await vi.waitFor(() => {
        expect(buttonNamed('Emergency disable')?.disabled).toBe(false)
      })
    })
    await act(async () => {
      buttonNamed('Emergency disable')?.click()
    })
    const dialog = document.querySelector('dialog.confirm-dialog') as HTMLDialogElement
    expect(dialog?.open).toBe(true)
    expect(dialog.textContent).toContain('Disable chaos?')
    expect(dialog.textContent).toContain('One confirm; no typed phrase.')
    expect(dialog.querySelector('input')).toBeNull()
    expect(posts).toHaveLength(0)
    await act(async () => {
      const submit = dialog.querySelector('button[type="submit"]') as HTMLButtonElement
      submit.click()
    })
    await act(async () => {
      await vi.waitFor(() => {
        expect(posts).toEqual([{ path: '/v1/chaos:emergency-disable', body: { reason: EMERGENCY_REASON } }])
      })
    })
    expect(host?.textContent).toContain('Chaos emergency disabled')
  })

  it('does not flash Missing scope while session is pending', async () => {
    vi.spyOn(client, 'GET').mockImplementation((async (path: string) => {
      if (path === '/v1/session') {
        return new Promise(() => {})
      }
      return fail(404, { code: 'not_found', detail: path })
    }) as typeof client.GET)
    await render(<EmergencyControl emergencyDisabled={false} />)
    await act(async () => {
      await Promise.resolve()
    })
    expect(host?.textContent).not.toContain('Missing scope')
    expect(buttonNamed('Emergency disable')?.disabled).toBe(true)
  })
})
