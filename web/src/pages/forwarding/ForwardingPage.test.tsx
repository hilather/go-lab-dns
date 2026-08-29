import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { MemoryRouter, Outlet, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { client } from '../../api/client'
import type { ShellContext } from '../../components/Shell'
import { queryKeys } from '../../query/keys'
import { LIVE_POLL_MS } from '../../query/live'
import type { StatusView } from '../../status'
import { ForwardingPage } from './ForwardingPage'

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

const sampleStatus: StatusView = {
  revisions: { runtimeRevision: 'sha256:abc123' },
}

const sampleRoutes: Record<string, unknown> = {
  '/v1/forwarding/policies': {
    policies: [
      {
        id: 'corp-policy',
        suffix: 'corp.example.net.',
        upstreamPool: 'corporate',
        failover: { timeout: '1s', onTimeout: true },
      },
    ],
  },
  '/v1/upstream-pools': {
    pools: [
      {
        id: 'corporate',
        strategy: 'ordered',
        upstreams: [{ id: 'corp-1', endpoint: '10.0.0.53:53', transport: 'udp' }],
      },
    ],
  },
  '/v1/upstreams/status': {
    upstreams: [
      {
        id: 'corp-1',
        poolId: 'corporate',
        endpoint: '10.0.0.53:53',
        transport: 'udp',
        healthy: true,
      },
      {
        id: 'default-2',
        poolId: 'default',
        endpoint: '10.0.0.55:53',
        transport: 'tcp',
        healthy: false,
      },
    ],
  },
}

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

async function render(ui: ReactNode, status: StatusView | null = sampleStatus) {
  host = document.createElement('div')
  document.body.appendChild(host)
  queryClient = testQueryClient()
  root = createRoot(host)
  await act(async () => {
    root!.render(
      <QueryClientProvider client={queryClient!}>
        <MemoryRouter initialEntries={['/forwarding']}>
          <Routes>
            <Route element={<Outlet context={{ status } satisfies ShellContext} />}>
              <Route path="/forwarding" element={ui} />
            </Route>
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

describe('ForwardingPage', () => {
  it('loads snapshot policies/pools and live upstream health without a revision on the live key', async () => {
    expect(LIVE_POLL_MS).toBe(5000)
    const get = vi.spyOn(client, 'GET').mockImplementation((async (path: string) => {
      const body = sampleRoutes[path]
      if (body !== undefined) {
        return ok(body)
      }
      return fail(404, { code: 'not_found', detail: path })
    }) as typeof client.GET)
    const post = vi.spyOn(client, 'POST')

    await render(<ForwardingPage />)
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('corp-policy')
        expect(host?.textContent).toContain('corp-1')
        expect(host?.textContent).toContain('Healthy')
        expect(host?.textContent).toContain('Unhealthy')
      })
    })

    expect(host?.textContent).toContain('corp.example.net.')
    expect(host?.textContent).toContain('ordered')
    expect(host?.textContent).toContain('timeout 1s; on timeout')
    expect(host?.querySelector('.status-symbol')?.textContent).toBe('●')
    expect(host?.querySelectorAll('table.data-table').length).toBe(3)
    expect(host?.querySelector('.page-lede')).not.toBeNull()

    const paths = get.mock.calls.map((c) => c[0])
    expect(paths).toContain('/v1/forwarding/policies')
    expect(paths).toContain('/v1/upstream-pools')
    expect(paths).toContain('/v1/upstreams/status')
    expect(post).not.toHaveBeenCalled()

    const rev = 'sha256:abc123'
    expect(queryClient?.getQueryData(queryKeys.forwarding(rev))).toBeTruthy()
    expect(queryClient?.getQueryData(queryKeys.pools(rev))).toBeTruthy()
    expect(queryClient?.getQueryData(queryKeys.liveUpstreams())).toBeTruthy()
    expect(queryKeys.liveUpstreams()).toEqual(['labdns', 'live', 'upstreams'])
  })

  it('still live-polls upstreams when runtime revision is unknown', async () => {
    const get = vi.spyOn(client, 'GET').mockImplementation((async (path: string) => {
      if (path === '/v1/upstreams/status') {
        return ok(sampleRoutes['/v1/upstreams/status'])
      }
      return fail(500, { code: 'internal_error', detail: 'should not fetch snapshot lists' })
    }) as typeof client.GET)

    await render(<ForwardingPage />, null)
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('corp-1')
      })
    })
    const paths = get.mock.calls.map((c) => c[0])
    expect(paths).toContain('/v1/upstreams/status')
    expect(paths).not.toContain('/v1/forwarding/policies')
    expect(paths).not.toContain('/v1/upstream-pools')
  })

  it('announces problem+json when a live poll fails', async () => {
    vi.spyOn(client, 'GET').mockImplementation((async (path: string) => {
      if (path === '/v1/upstreams/status') {
        return fail(403, { code: 'forbidden', detail: 'dns.forwarders.read required' })
      }
      if (path === '/v1/forwarding/policies') {
        return ok({ policies: [] })
      }
      if (path === '/v1/upstream-pools') {
        return ok({ pools: [] })
      }
      return fail(404, { code: 'not_found', detail: path })
    }) as typeof client.GET)

    await render(<ForwardingPage />)
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.querySelector('[role="alert"]')?.textContent).toContain('forbidden')
      })
    })
  })
})
