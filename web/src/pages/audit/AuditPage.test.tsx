import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { client } from '../../api/client'
import { queryKeys } from '../../query/keys'
import { AuditEventPage } from './AuditEventPage'
import { AuditPage } from './AuditPage'
import { AUDIT_RING_MAX, DEFAULT_AUDIT_LIMIT } from './view'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

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

const events = [
  {
    id: 'aud-2',
    time: '2026-08-19T00:00:02Z',
    actorId: 'admin',
    actorClass: 'ui-session',
    transport: 'rest',
    capability: 'change.apply',
    result: 'ok',
    reason: 'lab',
    revision: 'sha256:new',
  },
  {
    id: 'aud-1',
    time: '2026-08-19T00:00:01Z',
    actorId: 'viewer',
    actorClass: 'ui-session',
    transport: 'rest',
    capability: 'change.apply',
    result: 'denied',
    reason: '<img src=x onerror=alert(1)>',
    errorCode: 'forbidden',
  },
]

let root: Root | undefined
let host: HTMLDivElement | undefined
let queryClient: QueryClient | undefined

function testQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchOnWindowFocus: false },
    },
  })
}

async function render(ui: ReactNode, path: string, route: string) {
  host = document.createElement('div')
  document.body.appendChild(host)
  queryClient = testQueryClient()
  root = createRoot(host)
  await act(async () => {
    root!.render(
      <QueryClientProvider client={queryClient!}>
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route path={route} element={ui} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )
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
  vi.restoreAllMocks()
})

describe('AuditPage', () => {
  it('lists the ring with limit and keeps redacted or HTML-looking values as text', async () => {
    const get = vi.spyOn(client, 'GET').mockImplementation((async (path: string) => {
      if (path === '/v1/audit') {
        return ok({ events })
      }
      return fail(404, { code: 'not_found', detail: path })
    }) as typeof client.GET)
    await render(<AuditPage />, '/audit', '/audit')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('aud-2')
        expect(host?.textContent).toContain('aud-1')
      })
    })
    expect(host?.textContent).toContain(`In-memory ring of ${AUDIT_RING_MAX}`)
    expect(host?.textContent).toContain('dns.audit.read')
    expect(host?.textContent).toContain('fetched page only')
    expect(host?.textContent).toContain('<img src=x onerror=alert(1)>')
    expect(host?.querySelector('img')).toBeNull()
    expect(host?.querySelector('a')?.getAttribute('href')).toBe('/audit/aud-2')
    expect(get).toHaveBeenCalledWith(
      '/v1/audit',
      expect.objectContaining({ params: { query: { limit: DEFAULT_AUDIT_LIMIT } } }),
    )
    expect(queryClient?.getQueryData([...queryKeys.audit(), DEFAULT_AUDIT_LIMIT])).toBeTruthy()

    const result = host?.querySelector('select[name="result"]') as HTMLSelectElement
    await act(async () => {
      result.value = 'denied'
      result.dispatchEvent(new Event('change', { bubbles: true }))
    })
    expect(host?.textContent).toContain('aud-1')
    expect(host?.textContent).not.toContain('aud-2')

    await act(async () => {
      result.value = 'error'
      result.dispatchEvent(new Event('change', { bubbles: true }))
    })
    expect(host?.textContent).toContain('No events match these filters.')
    expect(host?.textContent).not.toContain('No audit events.')
  })

  it('announces problem+json when the viewer cannot read audit', async () => {
    vi.spyOn(client, 'GET').mockImplementation((async () =>
      fail(403, { code: 'forbidden', detail: 'dns.audit.read required' })) as typeof client.GET)
    await render(<AuditPage />, '/audit', '/audit')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.querySelector('[role="alert"]')?.textContent).toBe('forbidden: dns.audit.read required')
      })
    })
  })
})

describe('AuditEventPage', () => {
  it('loads one event by id without mutating', async () => {
    const get = vi.spyOn(client, 'GET').mockImplementation((async (path: string) => {
      if (path === '/v1/audit/{eventId}') {
        return ok(events[1])
      }
      return fail(404, { code: 'not_found', detail: path })
    }) as typeof client.GET)
    const post = vi.spyOn(client, 'POST')
    await render(<AuditEventPage />, '/audit/aud-1', '/audit/:eventId')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('aud-1')
        expect(host?.textContent).toContain('denied')
        expect(host?.textContent).toContain('forbidden')
      })
    })
    expect(host?.textContent).toContain('<img src=x onerror=alert(1)>')
    expect(host?.querySelector('img')).toBeNull()
    expect(get).toHaveBeenCalledWith(
      '/v1/audit/{eventId}',
      expect.objectContaining({ params: { path: { eventId: 'aud-1' } } }),
    )
    expect(post).not.toHaveBeenCalled()
  })
})
