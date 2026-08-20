import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Operation } from '../pages/changes/changeIn'
import { OperationBuilder } from './OperationBuilder'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

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

describe('OperationBuilder', () => {
  it('appends an add-record operation', async () => {
    const onChange = vi.fn()
    await render(<OperationBuilder operations={[]} onChange={onChange} />)
    const add = document.querySelector('button') as HTMLButtonElement
    expect(add.textContent).toBe('Add operation')
    await act(async () => {
      add.click()
    })
    expect(onChange).toHaveBeenCalledWith([
      { op: 'add', target: { kind: 'record' }, value: {} },
    ] satisfies Operation[])
  })

  it('renders seeded operations for record apply', async () => {
    const onChange = vi.fn()
    const ops: Operation[] = [
      {
        op: 'add',
        target: { kind: 'record', id: 'www-a', zoneId: 'lab-zone' },
        value: { id: 'www-a', owner: 'www', type: 'A', values: ['10.42.0.80'] },
      },
    ]
    await render(<OperationBuilder operations={ops} onChange={onChange} />)
    const id = document.querySelectorAll('input[type="text"]')[0] as HTMLInputElement
    const zone = document.querySelectorAll('input[type="text"]')[1] as HTMLInputElement
    expect(id.value).toBe('www-a')
    expect(zone.value).toBe('lab-zone')
    expect(document.body.textContent).toContain('Value (JSON)')
  })
})
