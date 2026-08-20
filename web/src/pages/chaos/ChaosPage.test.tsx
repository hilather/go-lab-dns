import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { client } from '../../api/client'
import { queryKeys } from '../../query/keys'
import { LIVE_POLL_MS } from '../../query/live'
import { ChaosPage } from './ChaosPage'
import { ChaosPolicyPage } from './ChaosPolicyPage'

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

function buttonNamed(label: string): HTMLButtonElement | undefined {
  return [...(host?.querySelectorAll('button') ?? [])].find((b) => b.textContent === label)
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

const operatorSession = {
  csrf: 'csrf',
  actor: {
    id: 'op',
    class: 'ui-session',
    role: 'chaos-operator',
    scopes: ['dns.read', 'dns.chaos.read', 'dns.chaos.activate'],
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
      return ok(extra?.policy ?? policy)
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
    expect(host?.textContent).toContain('Side-effect free')
    expect(buttonNamed('Simulate')?.disabled).toBe(true)
    expect(get.mock.calls.map((c) => c[0])).toContain('/v1/chaos/status')
    expect(get.mock.calls.map((c) => c[0])).toContain('/v1/chaos/policies')
    expect(post).not.toHaveBeenCalled()
    expect(queryClient?.getQueryData(queryKeys.liveChaos())).toBeTruthy()
    expect(queryClient?.getQueryData(queryKeys.chaosPolicies(REV))).toBeTruthy()
    expect(queryKeys.liveChaos()).toEqual(['labdns', 'live', 'chaos'])
  })

  it('posts simulate without calling activation or emergency routes', async () => {
    mockGets(adminSession)
    const posts: string[] = []
    vi.spyOn(client, 'POST').mockImplementation((async (path: string, init?: unknown) => {
      posts.push(path)
      const body = (init as { body?: unknown } | undefined)?.body
      expect(body).toMatchObject({
        name: 'foo.tools.lab.example.net.',
        type: 'A',
        clientContext: { clientGroup: 'test-devices' },
      })
      return ok({
        algorithm: 'hash-v1',
        disabled: false,
        triggered: true,
        decisions: [{ policyId: 'slow-tools', outcomeId: 'delayed', triggered: true }],
      })
    }) as typeof client.POST)
    await render(<ChaosPage />, '/chaos', '/chaos')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.querySelector('input[name="name"]')).not.toBeNull()
      })
    })
    const name = host!.querySelector('input[name="name"]') as HTMLInputElement
    const group = host!.querySelector('input[name="clientGroup"]') as HTMLInputElement
    await act(async () => {
      setNativeValue(name, 'foo.tools.lab.example.net.')
      setNativeValue(group, 'test-devices')
    })
    expect(buttonNamed('Simulate')?.disabled).toBe(false)
    await act(async () => {
      buttonNamed('Simulate')?.form?.requestSubmit()
    })
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('hash-v1')
        expect(host?.textContent).toContain('slow-tools')
      })
    })
    expect(posts).toEqual(['/v1/chaos:simulate'])
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
    expect(buttonNamed('Simulate')?.disabled).toBe(true)
    expect(host?.querySelector('input[name="name"]')?.hasAttribute('disabled')).toBe(true)
  })

  it('does not flash Missing scope before GET /v1/session settles', async () => {
    vi.spyOn(client, 'GET').mockImplementation((async (path: string) => {
      if (path === '/v1/status') {
        return ok({ revisions: { runtimeRevision: REV } })
      }
      if (path === '/v1/session') {
        return new Promise(() => {})
      }
      if (path === '/v1/chaos/status') {
        return ok({ enabled: true, emergencyDisabled: false, activePolicies: 0 })
      }
      if (path === '/v1/chaos/policies') {
        return ok({ policies: [policy] })
      }
      return fail(404, { code: 'not_found', detail: path })
    }) as typeof client.GET)
    await render(<ChaosPage />, '/chaos', '/chaos')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('slow-tools')
      })
    })
    expect(host?.textContent).not.toContain('Missing scope')
    expect(buttonNamed('Simulate')?.disabled).toBe(true)
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
      expect(buttonNamed(label)?.disabled).toBe(true)
    }
    expect(post).not.toHaveBeenCalled()
    expect(queryClient?.getQueryData(queryKeys.chaosPolicy(REV, 'slow-tools'))).toBeTruthy()
  })

  it('keeps activation disabled while GET /v1/session is pending without flashing Missing scope', async () => {
    vi.spyOn(client, 'GET').mockImplementation((async (path: string) => {
      if (path === '/v1/status') {
        return ok({ revisions: { runtimeRevision: REV } })
      }
      if (path === '/v1/session') {
        return new Promise(() => {})
      }
      if (path === '/v1/chaos/policies/{policyId}') {
        return ok(policy)
      }
      return fail(404, { code: 'not_found', detail: path })
    }) as typeof client.GET)
    await render(<ChaosPolicyPage />, '/chaos/slow-tools', '/chaos/:policyId')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('Test application startup timeouts')
      })
    })
    expect(host?.textContent).not.toContain('Missing scope')
    for (const label of ['Activate', 'Deactivate', 'Set expiry']) {
      expect(buttonNamed(label)?.disabled).toBe(true)
    }
  })

  it('activates with expectedRevision and an in-memory idempotency key', async () => {
    mockGets(adminSession)
    vi.spyOn(crypto, 'randomUUID').mockReturnValue('00000000-0000-4000-8000-000000000001')
    const posts: { path: string; body: unknown }[] = []
    vi.spyOn(client, 'POST').mockImplementation((async (path: string, init?: unknown) => {
      posts.push({ path, body: (init as { body?: unknown } | undefined)?.body })
      return ok({
        applied: true,
        previousRevision: REV,
        candidateRevision: 'sha256:def',
        auditEventId: 'evt-9',
      })
    }) as typeof client.POST)
    await render(<ChaosPolicyPage />, '/chaos/slow-tools', '/chaos/:policyId')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('Test application startup timeouts')
      })
    })
    const reason = host!.querySelector('input[name="reason"]') as HTMLInputElement
    await act(async () => {
      setNativeValue(reason, 'lab')
    })
    expect(buttonNamed('Activate')?.disabled).toBe(false)
    await act(async () => {
      buttonNamed('Activate')?.click()
    })
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('Audit event evt-9')
      })
    })
    expect(posts).toHaveLength(1)
    expect(posts[0]?.path).toBe('/v1/chaos/policies/{id}:activate')
    expect(posts[0]?.body).toMatchObject({
      expectedRevision: REV,
      reason: 'lab',
      idempotencyKey: '00000000-0000-4000-8000-000000000001',
    })
  })

  it('disables high-impact activate using CanActivateHigh while leaving deactivate available', async () => {
    mockGets(operatorSession, { policy: { ...policy, safetyClass: 'high', enabled: true } })
    const post = vi.spyOn(client, 'POST')
    await render(<ChaosPolicyPage />, '/chaos/slow-tools', '/chaos/:policyId')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.textContent).toContain('high')
      })
    })
    const reason = host!.querySelector('input[name="reason"]') as HTMLInputElement
    await act(async () => {
      setNativeValue(reason, 'lab')
    })
    expect(host?.textContent).toContain('Missing scope dns.chaos.emergency')
    expect(buttonNamed('Activate')?.disabled).toBe(true)
    expect(buttonNamed('Deactivate')?.disabled).toBe(false)
    expect(post).not.toHaveBeenCalled()
  })

  it('announces revision_conflict from typed activate without retrying', async () => {
    mockGets(adminSession)
    vi.spyOn(crypto, 'randomUUID').mockReturnValue('00000000-0000-4000-8000-000000000002')
    const post = vi.spyOn(client, 'POST').mockResolvedValue(
      fail(409, {
        code: 'revision_conflict',
        detail: 'revision moved',
        currentRevision: 'sha256:live',
        expectedRevision: REV,
      }) as never,
    )
    await render(<ChaosPolicyPage />, '/chaos/slow-tools', '/chaos/:policyId')
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.querySelector('input[name="reason"]')).not.toBeNull()
      })
    })
    await act(async () => {
      setNativeValue(host!.querySelector('input[name="reason"]') as HTMLInputElement, 'lab')
    })
    await act(async () => {
      buttonNamed('Activate')?.click()
    })
    await act(async () => {
      await vi.waitFor(() => {
        expect(host?.querySelector('[role="alert"]')?.textContent).toContain('revision_conflict')
      })
    })
    expect(post).toHaveBeenCalledTimes(1)
  })
})
