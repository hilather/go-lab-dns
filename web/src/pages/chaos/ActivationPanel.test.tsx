import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { client } from '../../api/client'
import { ActivationPanel } from './ActivationPanel'
;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
it('preserves exact retries and rotates keys on payload edits', async () => {
  const post = vi.spyOn(client, 'POST').mockResolvedValue({ response: new Response(null, { status: 503 }), error: { detail: 'unavailable' } } as never)
  const host = document.createElement('div'); document.body.appendChild(host)
  const root = createRoot(host); const qc = new QueryClient()
  const render = async (policyId = 'p1', expectedRevision = 'r1') => { await act(async () => root.render(<QueryClientProvider client={qc}><ActivationPanel actor={{ id: 'admin', role: 'administrator', scopes: ['dns.admin'] }} safetyClass="low" sessionKnown policyId={policyId} expectedRevision={expectedRevision} /></QueryClientProvider>)) }
  const edit = async (name: string, value: string) => { await act(async () => {
    const input = host.querySelector(`input[name="${name}"]`) as HTMLInputElement
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  }) }
  const activate = async () => {
    await act(async () => [...host.querySelectorAll('button')].find((button) => button.textContent === 'Activate')!.click())
    return (post.mock.calls.at(-1)![1] as { body: { idempotencyKey: string } }).body.idempotencyKey
  }
  try {
    await render(); await edit('reason', 'first')
    const first = await activate(); expect(await activate()).toBe(first)
    await edit('reason', 'second'); const second = await activate(); expect(second).not.toBe(first)
    await edit('expiresAt', '2026-12-01T10:00'); const expiry = await activate(); expect(expiry).not.toBe(second)
    await render('p2'); const policy = await activate(); expect(policy).not.toBe(expiry)
    await render('p2', 'r2'); expect(await activate()).not.toBe(policy)
  } finally { await act(async () => root.unmount()); qc.clear(); host.remove(); post.mockRestore() }
})
