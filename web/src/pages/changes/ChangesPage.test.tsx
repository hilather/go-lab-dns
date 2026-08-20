import { QueryClientProvider } from '@tanstack/react-query'
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { MemoryRouter, Outlet, Route, Routes } from 'react-router'
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { client } from '../../api/client'
import { clear, setCsrf } from '../../auth/sessionMemory'
import { createQueryClient } from '../../query/client'
import type { StatusView } from '../../status'
import { ChangesPage } from './ChangesPage'
import type { Operation } from './changeIn'

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

const ADD_WWW: Operation = {
  op: 'add',
  target: { kind: 'record', id: 'www-a', zoneId: 'lab-zone' },
  value: { id: 'www-a', owner: 'www', type: 'A', values: ['10.42.0.80'] },
}

const PLAN_BODY = {
  previousRevision: REV,
  candidateRevision: 'sha256:bbbb',
  impact: { requiredPermissions: ['dns.write'], zones: ['lab-zone'] },
  warnings: [{ code: 'note', message: 'ttl default' }],
  diff: [{ path: '/spec/zones/lab-zone/records/www-a', op: 'add' }],
}

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
    response: new Response(JSON.stringify(ok ? data : error), { status }),
  }
}

let root: Root | undefined
let host: HTMLDivElement | undefined
let applyBodies: unknown[]
let planBodies: unknown[]
let validateBodies: unknown[]
let applyStatus: number
let uuidSeq: string[]

async function renderPage(opts?: { status?: StatusView | null; locationState?: unknown }) {
  const status: StatusView | null =
    opts?.status === undefined ? { revisions: { runtimeRevision: REV } } : opts.status
  host = document.createElement('div')
  document.body.appendChild(host)
  root = createRoot(host)
  const qc = createQueryClient()
  await act(async () => {
    root!.render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={[{ pathname: '/changes', state: opts?.locationState ?? null }]}>
          <Routes>
            <Route element={<Outlet context={{ status }} />}>
              <Route path="/changes" element={<ChangesPage />} />
            </Route>
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )
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
  applyBodies = []
  planBodies = []
  validateBodies = []
  applyStatus = 200
  uuidSeq = [
    '00000000-0000-4000-8000-000000000001',
    '00000000-0000-4000-8000-000000000002',
    '00000000-0000-4000-8000-000000000003',
  ]
  let uuidI = 0
  vi.spyOn(crypto, 'randomUUID').mockImplementation(
    () => (uuidSeq[uuidI++] ?? `00000000-0000-4000-8000-00000000000${uuidI}`) as ReturnType<typeof crypto.randomUUID>,
  )
  setCsrf('csrf-test')
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input instanceof Request ? input.url : String(input)
      if (url.includes('/v1/session')) {
        return jsonResponse(200, {
          csrf: 'csrf-test',
          actor: { id: 'admin', class: 'ui-session', role: 'administrator', scopes: ['dns.write'] },
        })
      }
      return jsonResponse(404, { code: 'not_found' })
    }),
  )
  vi.spyOn(client, 'POST').mockImplementation((async (path: unknown, init?: unknown) => {
    const body = (init as { body?: unknown } | undefined)?.body
    if (path === '/v1/state:validate') {
      validateBodies.push(body)
      return result(200, PLAN_BODY)
    }
    if (path === '/v1/changes:plan') {
      planBodies.push(body)
      return result(200, PLAN_BODY)
    }
    if (path === '/v1/changes:apply') {
      applyBodies.push(body)
      if (applyStatus === 409) {
        return result(409, undefined, {
          code: 'revision_conflict',
          detail: 'revision moved',
          currentRevision: 'sha256:live',
          expectedRevision: REV,
        })
      }
      if (applyStatus >= 400) {
        return result(applyStatus, undefined, { code: 'internal_error', detail: 'boom' })
      }
      return result(200, {
        ...PLAN_BODY,
        applied: true,
        generation: 2,
        auditEventId: 'evt-1',
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

const draft = { operations: [ADD_WWW], reason: 'add www' }

describe('ChangesPage', () => {
  it('keeps apply disabled until a current plan exists for this revision', async () => {
    await renderPage({ locationState: draft })
    expect(actionButton('Apply').disabled).toBe(true)
    await act(async () => {
      actionButton('Plan').click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect(planBodies).toHaveLength(1)
    const planned = planBodies[0] as { expectedRevision?: string; operations?: unknown[] }
    expect(planned.expectedRevision).toBe(REV)
    expect(planned.operations).toEqual([ADD_WWW])
    expect(actionButton('Apply').disabled).toBe(false)
    expect(document.body.textContent).toContain('Required permissions')
    expect(document.body.textContent).toContain('dns.write')
  })

  it('does not skip plan: apply is not posted without a plan', async () => {
    await renderPage({ locationState: draft })
    expect(actionButton('Apply').disabled).toBe(true)
    await act(async () => {
      actionButton('Apply').click()
    })
    expect(applyBodies).toHaveLength(0)
    expect(document.querySelector('dialog.confirm-dialog')?.hasAttribute('open')).toBeFalsy()
  })

  it('validate sends expectedRevision from latest status', async () => {
    await renderPage({ locationState: draft })
    await act(async () => {
      actionButton('Validate').click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect(validateBodies).toHaveLength(1)
    expect((validateBodies[0] as { expectedRevision?: string }).expectedRevision).toBe(REV)
  })

  it('plans a YAML operations envelope as the same ChangeIn', async () => {
    await renderPage({
      locationState: {
        document: `operations:
  - op: add
    target:
      kind: record
      id: www-a
      zoneId: lab-zone
    value:
      id: www-a
      owner: www
      type: A
      values:
        - 10.42.0.80
`,
        reason: 'add www',
      },
    })
    await act(async () => {
      actionButton('Plan').click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect(planBodies).toHaveLength(1)
    expect((planBodies[0] as { operations?: unknown[] }).operations).toEqual([ADD_WWW])
    expect(actionButton('Apply').disabled).toBe(false)
  })

  it('reuses the in-memory idempotency key on retry of the same confirm only', async () => {
    await renderPage({ locationState: draft })
    await act(async () => {
      actionButton('Plan').click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    applyStatus = 500
    await act(async () => {
      actionButton('Apply').click()
    })
    const confirm = document.querySelector('dialog button[type="submit"]') as HTMLButtonElement
    await act(async () => {
      confirm.click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect(applyBodies).toHaveLength(1)
    expect((applyBodies[0] as { idempotencyKey?: string }).idempotencyKey).toBe(
      '00000000-0000-4000-8000-000000000001',
    )
    await act(async () => {
      confirm.click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect(applyBodies).toHaveLength(2)
    expect((applyBodies[1] as { idempotencyKey?: string }).idempotencyKey).toBe(
      '00000000-0000-4000-8000-000000000001',
    )

    await act(async () => {
      const cancel = document.querySelector('dialog button[type="button"]') as HTMLButtonElement
      cancel.click()
    })
    await act(async () => {
      actionButton('Apply').click()
    })
    applyStatus = 200
    const confirm2 = document.querySelector('dialog button[type="submit"]') as HTMLButtonElement
    await act(async () => {
      confirm2.click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect((applyBodies[2] as { idempotencyKey?: string }).idempotencyKey).toBe(
      '00000000-0000-4000-8000-000000000002',
    )
    expect((applyBodies[2] as { reason?: string }).reason).toBe('add www')
    expect((applyBodies[2] as { expectedRevision?: string }).expectedRevision).toBe(REV)
  })

  it('on 409 revision_conflict shows current revision, discards the stale plan, and does not overwrite the candidate', async () => {
    await renderPage({ locationState: draft })
    await act(async () => {
      actionButton('Plan').click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect(actionButton('Apply').disabled).toBe(false)
    applyStatus = 409
    await act(async () => {
      actionButton('Apply').click()
    })
    const confirm = document.querySelector('dialog button[type="submit"]') as HTMLButtonElement
    await act(async () => {
      confirm.click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect(document.body.textContent).toContain('revision_conflict')
    expect(document.body.textContent).toContain('sha256:live')
    expect(actionButton('Apply').disabled).toBe(true)
    const id = document.querySelectorAll('.operation-row input[type="text"]')[0] as HTMLInputElement
    expect(id.value).toBe('www-a')
    expect(document.body.textContent).toContain('candidate was not overwritten')
  })

  it('does not clobber the in-progress candidate when the polled revision changes', async () => {
    function Harness() {
      const [status, setStatus] = useState<StatusView>({ revisions: { runtimeRevision: REV } })
      return (
        <div>
          <button type="button" id="bump-rev" onClick={() => setStatus({ revisions: { runtimeRevision: 'sha256:other' } })}>
            bump
          </button>
          <Outlet context={{ status }} />
        </div>
      )
    }

    host = document.createElement('div')
    document.body.appendChild(host)
    root = createRoot(host)
    const qc = createQueryClient()
    await act(async () => {
      root!.render(
        <QueryClientProvider client={qc}>
          <MemoryRouter initialEntries={[{ pathname: '/changes', state: { operations: [ADD_WWW], reason: 'keep me' } }]}>
            <Routes>
              <Route element={<Harness />}>
                <Route path="/changes" element={<ChangesPage />} />
              </Route>
            </Routes>
          </MemoryRouter>
        </QueryClientProvider>,
      )
    })
    await act(async () => {
      await Promise.resolve()
    })
    await act(async () => {
      actionButton('Plan').click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect(actionButton('Apply').disabled).toBe(false)
    await act(async () => {
      document.getElementById('bump-rev')!.click()
    })
    expect(actionButton('Apply').disabled).toBe(true)
    const id = document.querySelectorAll('.operation-row input[type="text"]')[0] as HTMLInputElement
    expect(id.value).toBe('www-a')
    const reason = document.querySelector('.changes-reason input') as HTMLInputElement
    expect(reason.value).toBe('keep me')
  })

  it('does not write secrets to Web Storage during plan/apply', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    await renderPage({ locationState: draft })
    await act(async () => {
      actionButton('Plan').click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    await act(async () => {
      actionButton('Apply').click()
    })
    const confirm = document.querySelector('dialog button[type="submit"]') as HTMLButtonElement
    await act(async () => {
      confirm.click()
    })
    await act(async () => {
      await Promise.resolve()
    })
    expect(setItem).not.toHaveBeenCalled()
    expect(window.localStorage.length).toBe(0)
    expect(window.sessionStorage.length).toBe(0)
  })
})
