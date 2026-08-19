import { QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createLabdnsClient } from '../../api/client'
import { clear, setCsrf } from '../../auth/sessionMemory'
import { createQueryClient } from '../../query/client'
import { ResolvePage } from './ResolvePage'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function requestOf(call: unknown[] | undefined): Request {
  const first = call?.[0]
  if (first instanceof Request) {
    return first
  }
  return new Request(String(first), (call?.[1] ?? {}) as RequestInit)
}

function pathnameOf(req: Request): string {
  return new URL(req.url, 'http://127.0.0.1:8080').pathname
}

let root: Root | undefined
let host: HTMLDivElement | undefined

async function render(ui: ReactNode) {
  host = document.createElement('div')
  document.body.appendChild(host)
  root = createRoot(host)
  await act(async () => {
    root!.render(<QueryClientProvider client={createQueryClient()}>{ui}</QueryClientProvider>)
  })
}

async function waitFor(fn: () => boolean, timeoutMs = 2000) {
  const start = Date.now()
  while (!fn()) {
    if (Date.now() - start > timeoutMs) {
      throw new Error(`timeout: ${document.body.textContent ?? ''}`)
    }
    await act(async () => {
      await new Promise((r) => setTimeout(r, 10))
    })
  }
}

function setInputValue(el: HTMLInputElement, value: string) {
  const proto = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')
  proto?.set?.call(el, value)
  el.dispatchEvent(new Event('input', { bubbles: true }))
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
})

function adminSession() {
  return {
    csrf: 'csrf-page',
    actor: { id: 'loopback', class: 'ui-session', role: 'administrator', scopes: [] as string[] },
  }
}

function mockFetch(sessionBody: unknown = adminSession()) {
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const req = input instanceof Request ? input : new Request(String(input), init)
    const path = pathnameOf(req)
    if (req.method === 'GET' && path === '/v1/session') {
      return jsonResponse(200, sessionBody)
    }
    if (req.method === 'POST' && path === '/v1/resolve') {
      return jsonResponse(200, {
        result: {
          rcode: 'NOERROR',
          source: 'exact',
          zoneId: 'lab-zone',
          zoneMode: 'authoritative',
          aa: true,
          ra: false,
          fallthrough: false,
          answers: [{ name: 'ns1.lab.example.net.', ttl: 30_000_000_000, type: 'A', data: '10.42.0.10' }],
        },
      })
    }
    if (req.method === 'POST' && path === '/v1/resolve:explain') {
      return jsonResponse(200, {
        result: { rcode: 'NOERROR', source: 'exact' },
        explanation: {
          zoneId: 'lab-zone',
          zoneMode: 'authoritative',
          source: 'exact',
          wildcardSource: '',
          forwardingId: '',
          chaosDisabled: true,
          chaosReason: 'applyChaos=false',
        },
      })
    }
    return jsonResponse(404, { code: 'not_found', detail: path })
  })
}

describe('ResolvePage', () => {
  it('renders applyChaos off and posts both endpoints side by side', async () => {
    setCsrf('csrf-page')
    const fetchMock = mockFetch()
    const api = createLabdnsClient({ fetch: fetchMock })
    await render(<ResolvePage api={api} />)
    await waitFor(() => {
      const box = document.querySelector('input[name="applyChaos"]') as HTMLInputElement | null
      return box !== null && !box.disabled
    })

    const chaos = document.querySelector('input[name="applyChaos"]') as HTMLInputElement
    const cache = document.querySelector('input[name="useCache"]') as HTMLInputElement
    expect(chaos.checked).toBe(false)
    expect(cache.checked).toBe(false)

    const name = document.querySelector('input[name="name"]') as HTMLInputElement
    await act(async () => {
      setInputValue(name, 'ns1.lab.example.net.')
    })
    const form = document.querySelector('form') as HTMLFormElement
    await act(async () => {
      form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })
    await waitFor(() => (document.body.textContent ?? '').includes('NOERROR'))

    expect(document.getElementById('resolve-answer-heading')?.textContent).toBe('Answer')
    expect(document.getElementById('resolve-explain-heading')?.textContent).toBe('Explain')
    expect(document.body.textContent).toContain('Matched zone')
    expect(document.body.textContent).toContain('lab-zone / authoritative')
    expect(document.body.textContent).toContain('Chaos decision')
    expect(document.body.textContent).toContain('disabled: applyChaos=false')
    expect(document.body.textContent).toContain('10.42.0.10')

    const posts = fetchMock.mock.calls
      .map((c) => requestOf(c as unknown[]))
      .filter((r) => r.method === 'POST')
    const paths = posts.map(pathnameOf).sort()
    expect(paths).toEqual(['/v1/resolve', '/v1/resolve:explain'])
    for (const req of posts) {
      const sent = JSON.parse(await req.clone().text()) as {
        name?: string
        options?: { applyChaos?: boolean; useCache?: boolean }
      }
      expect(sent.name).toBe('ns1.lab.example.net.')
      expect(sent.options?.applyChaos).toBe(false)
      expect(sent.options?.useCache).toBe(false)
    }
  })

  it('disables resolve and names dns.read when the actor lacks the scope', async () => {
    const fetchMock = mockFetch({
      csrf: 'csrf-page',
      actor: { id: 'v', class: 'ui-session', role: 'viewer', scopes: ['dns.audit.read'] },
    })
    const api = createLabdnsClient({ fetch: fetchMock })
    await render(<ResolvePage api={api} />)
    await waitFor(() => (document.body.textContent ?? '').includes('Missing scope dns.read'))
    const button = document.querySelector('button[type="submit"]') as HTMLButtonElement
    expect(button.disabled).toBe(true)
    const posts = fetchMock.mock.calls
      .map((c) => requestOf(c as unknown[]))
      .filter((r) => r.method === 'POST')
    expect(posts).toHaveLength(0)
  })

  it('announces per-column problem+json with role=alert', async () => {
    setCsrf('csrf-page')
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const req = input instanceof Request ? input : new Request(String(input), init)
      const path = pathnameOf(req)
      if (req.method === 'GET' && path === '/v1/session') {
        return jsonResponse(200, adminSession())
      }
      if (path === '/v1/resolve') {
        return jsonResponse(400, { code: 'invalid_value', detail: 'name is required' })
      }
      return jsonResponse(403, { code: 'forbidden', detail: 'dns.read required' })
    })
    const api = createLabdnsClient({ fetch: fetchMock })
    await render(<ResolvePage api={api} />)
    await waitFor(() => {
      const name = document.querySelector('input[name="name"]') as HTMLInputElement | null
      return name !== null && !name.disabled
    })
    const name = document.querySelector('input[name="name"]') as HTMLInputElement
    await act(async () => {
      setInputValue(name, 'missing.example.')
    })
    await act(async () => {
      document.querySelector('form')?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })
    await waitFor(() => document.querySelectorAll('[role="alert"]').length >= 2)
    const alerts = [...document.querySelectorAll('[role="alert"]')].map((el) => el.textContent)
    expect(alerts.some((t) => t?.includes('invalid_value'))).toBe(true)
    expect(alerts.some((t) => t?.includes('forbidden'))).toBe(true)
  })
})
