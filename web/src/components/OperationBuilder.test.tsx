import { act, useState, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { emptyBuilderRow, operationToRow, type BuilderRow, type Operation } from '../pages/changes/changeIn'
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

function setNativeValue(el: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const proto = Object.getPrototypeOf(el) as HTMLInputElement | HTMLTextAreaElement
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

const opA: Operation = { op: 'add', target: { kind: 'record', id: 'www-a' }, value: { id: 'www-a' } }
const opB: Operation = {
  op: 'add',
  target: { kind: 'record', id: 'www-b', zoneId: 'lab-zone' },
  value: { id: 'www-b', owner: 'www', type: 'A', values: ['10.42.0.81'] },
}

function Harness({ initial }: { initial: BuilderRow[] }) {
  const [rows, setRows] = useState(initial)
  return <OperationBuilder rows={rows} onChange={setRows} />
}

describe('OperationBuilder', () => {
  it('appends an add-record operation', async () => {
    const onChange = vi.fn()
    await render(<OperationBuilder rows={[]} onChange={onChange} />)
    const add = document.querySelector('button') as HTMLButtonElement
    expect(add.textContent).toBe('Add operation')
    await act(async () => {
      add.click()
    })
    expect(onChange).toHaveBeenCalledTimes(1)
    const next = onChange.mock.calls[0]?.[0] as BuilderRow[]
    expect(next).toHaveLength(1)
    expect(next[0]?.op).toBe('add')
    expect(next[0]?.target).toEqual({ kind: 'record' })
    expect(next[0]?.valueText).toBe('{}')
    expect(next[0]?.key).toEqual(expect.any(String))
  })

  it('renders seeded operations for record apply', async () => {
    const onChange = vi.fn()
    await render(<OperationBuilder rows={[operationToRow(opA)]} onChange={onChange} />)
    const id = document.querySelectorAll('input[type="text"]')[0] as HTMLInputElement
    expect(id.value).toBe('www-a')
    expect(document.body.textContent).toContain('Value (JSON)')
  })

  it('keeps the remaining row value JSON when the first operation is removed', async () => {
    await render(<Harness initial={[operationToRow(opA), operationToRow(opB)]} />)
    const removeButtons = [...document.querySelectorAll('.operation-row button')].filter(
      (b) => b.textContent === 'Remove',
    ) as HTMLButtonElement[]
    expect(removeButtons).toHaveLength(2)
    await act(async () => {
      removeButtons[0]!.click()
    })
    const id = document.querySelectorAll('.operation-row input[type="text"]')[0] as HTMLInputElement
    expect(id.value).toBe('www-b')
    const ta = document.querySelector('.operation-value textarea') as HTMLTextAreaElement
    expect(JSON.parse(ta.value)).toEqual(opB.value)
    expect(document.querySelectorAll('.operation-row')).toHaveLength(1)
  })

  it('treats invalid JSON as a row error and keeps the typed text', async () => {
    await render(<Harness initial={[operationToRow(opA)]} />)
    const ta = document.querySelector('.operation-value textarea') as HTMLTextAreaElement
    await act(async () => {
      setNativeValue(ta, '{')
    })
    expect(ta.value).toBe('{')
    expect(document.body.textContent).toContain('Invalid JSON value')
  })

  it('does not mint the same key for two added rows', async () => {
    const a = emptyBuilderRow()
    const b = emptyBuilderRow()
    expect(a.key).not.toBe(b.key)
  })
})
