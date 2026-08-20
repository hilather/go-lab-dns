import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { client } from '../../api/client'
import { queryKeys } from '../../query/keys'
import { LIVE_POLL_MS } from '../../query/live'
import { ChaosPage } from './ChaosPage'
import { ChaosPolicyPage } from './ChaosPolicyPage'
import { MUTATIONS_UI003 } from './view'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const REV = 'sha256:abc123'

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

const viewerSession = {
  csrf: 'csrf',
  actor: {
    id: 'viewer',
    class: 'ui-session',
    role: 'viewer',
    scopes: ['dns.read', 'dns.forwarders.read', 'dns.chaos.read'],
  },
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

const policy = {
  id: 'slow-tools',
  owner: 'platform-lab',
  reason: 'Test application startup timeouts',
  enabled: false,
  safetyClass: 'low',
  scope: { recordIds: ['tools-wildcard-a'], clientGroups: ['test-devices'] },
  selector: { mode: 'deterministic', seed: 'startup-v1', probability: 1 },
  outcomes: [
    {
      id: 'delayed',
      weight: 100,
      actions: [{ type: 'delay', phase: 'before-response', distribution: 'uniform', min: '100ms', max: '750ms' }],
    },
  ],
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

function mockGets(session: unknown, extra?: Record<string, unknown>) {
  return vi.spyOn(client, 'GET').mockImplementation((async (path: string) => {
    if (path === '/v1/status') {
      return ok({ revisions: { runtimeRevision: REV } })
    }
    if (path === '/v1/session') {
      return ok(session)
    }
    if (path === '/v1/chaos/status') {
      return ok({ enabled: true, emergencyDisabled: false, activePolicies: 0, nearestExpiry: '' })
    }
    if (path === '/v1/chaos/policies') {
      return ok({ policies: [policy] })
    }
    if (path === '/v1/chaos/policies/{policyId}') {
      return ok(policy)
    }
    if (extra && extra[path] !== undefined) {
      return ok(extra[path])
    }
    return fail(404, { code: 'not_found', detail: path })
  }) as typeof client.GET)
}

describe('ChaosPage', () => {
  it('polls chaos status on a revision-independent live key and lists policies', async () => {
    expect(LIVE_POLL_MS).toBe(5000)
    const get = mockGets(adminSession)
    const post = vi.spyOn(client, 'POST')
    await render(<ChaosPage />, '/chaos', '/chaos')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('slow-tools')
        expect(host?.textContent).toContain('Inactive')
        expect(host?.textContent).toContain('Enabled')
      })
    })
    expect(host?.textContent).toContain('low')
    expect(host?.textContent).toContain('platform-lab')
    expect(host?.querySelector('a')?.getAttribute('href')).toBe('/chaos/slow-tools')
    const simulate = [...(host?.querySelectorAll('button') ?? [])].find((b) => b.textContent === 'Simulate')
    expect(simulate?.disabled).toBe(true)
    expect(host?.textContent).toContain(MUTATIONS_UI003)
    expect(host?.querySelectorAll('input')[0]?.disabled).toBe(true)
    expect(get.mock.calls.map((c) => c[0])).toContain('/v1/chaos/status')
    expect(get.mock.calls.map((c) => c[0])).toContain('/v1/chaos/policies')
    expect(post).not.toHaveBeenCalled()
    expect(queryClient?.getQueryData(queryKeys.liveChaos())).toBeTruthy()
    expect(queryClient?.getQueryData(queryKeys.chaosPolicies(REV))).toBeTruthy()
    expect(queryKeys.liveChaos()).toEqual(['labdns', 'live', 'chaos'])
  })

  it('names the missing chaos.read scope for a simulate-gated viewer without it', async () => {
    mockGets({
      csrf: 'csrf',
      actor: { id: 'dns-editor', role: 'dns-editor', scopes: ['dns.read', 'dns.write'] },
    })
    await render(<ChaosPage />, '/chaos', '/chaos')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('Missing scope dns.chaos.read')
      })
    })
    const simulate = [...(host?.querySelectorAll('button') ?? [])].find((b) => b.textContent === 'Simulate')
    expect(simulate?.disabled).toBe(true)
  })

  it('announces problem+json when chaos status fails', async () => {
    vi.spyOn(client, 'GET').mockImplementation((async (path: string) => {
      if (path === '/v1/status') {
        return ok({ revisions: { runtimeRevision: REV } })
      }
      if (path === '/v1/session') {
        return ok(viewerSession)
      }
      if (path === '/v1/chaos/status') {
        return fail(403, { code: 'forbidden', detail: 'dns.chaos.read required' })
      }
      if (path === '/v1/chaos/policies') {
        return fail(403, { code: 'forbidden', detail: 'dns.chaos.read required' })
      }
      return fail(404, { code: 'not_found', detail: path })
    }) as typeof client.GET)
    await render(<ChaosPage />, '/chaos', '/chaos')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.querySelector('[role="alert"]')?.textContent).toContain('forbidden: dns.chaos.read required')
      })
    })
  })
})

describe('ChaosPolicyPage', () => {
  it('renders policy detail and leaves activation disabled for a viewer', async () => {
    mockGets(viewerSession)
    const post = vi.spyOn(client, 'POST')
    await render(<ChaosPolicyPage />, '/chaos/slow-tools', '/chaos/:policyId')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('Test application startup timeouts')
        expect(host?.textContent).toContain('tools-wildcard-a')
        expect(host?.textContent).toContain('delay before-response uniform 100ms–750ms')
      })
    })
    expect(host?.textContent).toContain('Missing scope dns.chaos.activate')
    for (const label of ['Activate', 'Deactivate', 'Set expiry']) {
      const button = [...(host?.querySelectorAll('button') ?? [])].find((b) => b.textContent === label)
      expect(button?.disabled).toBe(true)
    }
    expect(host?.querySelector('input[name="expiresAt"]')?.hasAttribute('disabled')).toBe(true)
    expect(post).not.toHaveBeenCalled()
    expect(queryClient?.getQueryData(queryKeys.chaosPolicy(REV, 'slow-tools'))).toBeTruthy()
  })

  it('keeps activation disabled for an administrator until UI-003', async () => {
    mockGets(adminSession)
    await render(<ChaosPolicyPage />, '/chaos/slow-tools', '/chaos/:policyId')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('Test application startup timeouts')
        expect(host?.textContent).toContain(MUTATIONS_UI003)
      })
    })
    expect(host?.textContent).not.toContain('Missing scope')
    const activate = [...(host?.querySelectorAll('button') ?? [])].find((b) => b.textContent === 'Activate')
    expect(activate?.disabled).toBe(true)
  })
})
