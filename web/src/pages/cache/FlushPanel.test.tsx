import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { client } from '../../api/client'
import { clear, setCsrf } from '../../auth/sessionMemory'
import { queryKeys } from '../../query/keys'
import { FlushPanel } from './FlushPanel'

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

function result(status: number, data?: unknown, error?: unknown) {
  const ok = status >= 200 && status < 300
  return {
    data: ok ? data : undefined,
    error: ok ? undefined : error,
    response: new Response(ok && data === undefined ? null : JSON.stringify(ok ? data : error), { status }),
  }
}

let root: Root | undefined
let host: HTMLDivElement | undefined
let queryClient: QueryClient | undefined
let flushBodies: unknown[]
let actorScopes: string[]
let actorRole: string
let flushStatus: number

function testQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchOnWindowFocus: false },
    },
  })
}

async function render(ui: ReactNode) {
  host = document.createElement('div')
  document.body.appendChild(host)
  queryClient = testQueryClient()
  root = createRoot(host)
  await act(async () => {
    root!.render(<QueryClientProvider client={queryClient!}>{ui}</QueryClientProvider>)
  })
  await act(async () => {
    await vi.waitFor(() => {
      expect(host?.querySelector('button')).not.toBeNull()
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
  queryClient = undefined
  clear()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

beforeEach(() => {
  flushBodies = []
  actorScopes = ['dns.admin', 'dns.read']
  actorRole = 'administrator'
  flushStatus = 204
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
  vi.spyOn(client, 'POST').mockImplementation((async (path: unknown, init?: unknown) => {
    const body = (init as { body?: unknown } | undefined)?.body
    if (path === '/v1/cache:flush') {
      flushBodies.push(body)
      if (flushStatus >= 400) {
        return result(flushStatus, undefined, { code: 'forbidden', detail: 'missing scope dns.admin' })
      }
      return result(204)
    }
    return result(404, undefined, { code: 'not_found' })
  }) as typeof client.POST)
})

function flushButton(): HTMLButtonElement {
  const buttons = [...(host?.querySelectorAll('button') ?? [])]
  const found = buttons.find((b) => b.textContent === 'Flush cache')
  if (!found) {
    throw new Error('Flush cache button missing')
  }
  return found as HTMLButtonElement
}

describe('FlushPanel', () => {
  it('posts cache:flush {all:true} for dns.admin and does not change desired state', async () => {
    await render(<FlushPanel />)
    expect(host?.textContent).toContain('does not change desired state')
    expect(flushButton().disabled).toBe(false)
    const selector = host?.querySelector('input[type="checkbox"]') as HTMLInputElement
    expect(selector.checked).toBe(true)
    expect(selector.disabled).toBe(true)

    await act(async () => {
      flushButton().click()
    })
    const confirm = document.querySelector('dialog button[type="submit"]') as HTMLButtonElement
    await act(async () => {
      confirm.click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect(flushBodies).toEqual([{ all: true }])
    expect(host?.textContent).toContain('Desired state is unchanged')
    expect(queryClient?.getQueryState(queryKeys.liveCache())?.isInvalidated).not.toBe(false)
  })

  it('disables flush without dns.admin', async () => {
    actorRole = 'viewer'
    actorScopes = ['dns.read']
    await render(<FlushPanel />)
    expect(flushButton().disabled).toBe(true)
    expect(host?.textContent).toContain('Missing scope dns.admin')
    await act(async () => {
      flushButton().click()
    })
    expect(flushBodies).toHaveLength(0)
  })

  it('blocks a second flush while the first is in flight', async () => {
    await render(<FlushPanel />)
    await act(async () => {
      flushButton().click()
    })
    const confirm = document.querySelector('dialog button[type="submit"]') as HTMLButtonElement
    await act(async () => {
      confirm.click()
      confirm.click()
    })
    expect(flushBodies).toEqual([{ all: true }])
    expect(host?.textContent).toContain('Desired state is unchanged')
  })

  it('announces problem+json when flush is forbidden', async () => {
    flushStatus = 403
    await render(<FlushPanel />)
    await act(async () => {
      flushButton().click()
    })
    const confirm = document.querySelector('dialog button[type="submit"]') as HTMLButtonElement
    await act(async () => {
      confirm.click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect(host?.querySelector('[role="alert"]')?.textContent).toBe('forbidden: missing scope dns.admin')
  })
})
