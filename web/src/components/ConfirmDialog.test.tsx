import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
import { ConfirmDialog } from './ConfirmDialog'
import { ProblemAlert } from './ProblemAlert'
import { ScopeGate } from './ScopeGate'

let root: Root | undefined
let host: HTMLDivElement | undefined

async function render(ui: ReactNode) {
  host = document.createElement('div')
  document.body.appendChild(host)
  root = createRoot(host)
  await act(async () => {
    root!.render(ui)
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
})

describe('ConfirmDialog', () => {
  it('opens a native dialog and confirms', async () => {
    const onConfirm = vi.fn()
    const onCancel = vi.fn()
    await render(
      <ConfirmDialog open title="Disable chaos?" onConfirm={onConfirm} onCancel={onCancel}>
        <p>This stops new faults immediately.</p>
      </ConfirmDialog>,
    )
    const dialog = document.querySelector('dialog.confirm-dialog') as HTMLDialogElement | null
    expect(dialog).not.toBeNull()
    expect(dialog?.open).toBe(true)
    expect(document.getElementById('confirm-dialog-title')?.textContent).toBe('Disable chaos?')
    const submit = document.querySelector('dialog button[type="submit"]') as HTMLButtonElement
    await act(async () => {
      submit.click()
    })
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })
})

describe('ScopeGate', () => {
  it('disables the action and names the missing scope', async () => {
    await render(
      <ScopeGate allowed={false} missingScope="dns.admin">
        <button type="button">Flush cache</button>
      </ScopeGate>,
    )
    const button = document.querySelector('button') as HTMLButtonElement
    expect(button.disabled).toBe(true)
    expect(document.body.textContent).toContain('Missing scope dns.admin')
  })
})

describe('ProblemAlert', () => {
  it('announces problem+json with role=alert', async () => {
    await render(<ProblemAlert code="forbidden" detail="dns.write required" />)
    const alert = document.querySelector('[role="alert"]')
    expect(alert).not.toBeNull()
    expect(alert?.textContent).toBe('forbidden: dns.write required')
  })
})
