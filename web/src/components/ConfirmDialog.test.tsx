import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { ConfirmDialog } from './ConfirmDialog'
import { ProblemAlert } from './ProblemAlert'
import { ScopeGate } from './ScopeGate'

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
  vi.restoreAllMocks()
})

describe('ConfirmDialog', () => {
  it('opens with showModal and confirms', async () => {
    const onConfirm = vi.fn()
    const onCancel = vi.fn()
    const showModal = vi.spyOn(HTMLDialogElement.prototype, 'showModal')
    await render(
      <ConfirmDialog open title="Disable chaos?" onConfirm={onConfirm} onCancel={onCancel}>
        <p>This stops new faults immediately.</p>
      </ConfirmDialog>,
    )
    const dialog = document.querySelector('dialog.confirm-dialog') as HTMLDialogElement | null
    expect(dialog).not.toBeNull()
    expect(showModal).toHaveBeenCalled()
    expect(dialog?.open).toBe(true)
    expect(document.getElementById('confirm-dialog-title')?.textContent).toBe('Disable chaos?')
    const submit = document.querySelector('dialog button[type="submit"]') as HTMLButtonElement
    expect(submit.classList.contains('btn-accent')).toBe(true)
    await act(async () => {
      submit.click()
    })
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('routes the dialog cancel event (Escape) to onCancel', async () => {
    const onConfirm = vi.fn()
    const onCancel = vi.fn()
    await render(<ConfirmDialog open title="Reset?" onConfirm={onConfirm} onCancel={onCancel} />)
    const dialog = document.querySelector('dialog.confirm-dialog') as HTMLDialogElement
    await act(async () => {
      dialog.dispatchEvent(new Event('cancel', { cancelable: true }))
    })
    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onConfirm).not.toHaveBeenCalled()
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
