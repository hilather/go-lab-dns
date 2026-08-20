import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { YamlJsonEditor } from './YamlJsonEditor'

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

describe('YamlJsonEditor', () => {
  it('renders YAML/JSON text and announces parse errors', async () => {
    const onChange = vi.fn()
    await render(
      <YamlJsonEditor label="Candidate" value="kind: LabDNS" onChange={onChange} parseError="invalid document" />,
    )
    const ta = document.querySelector('#yaml-json-editor') as HTMLTextAreaElement
    expect(ta.value).toBe('kind: LabDNS')
    const alert = document.querySelector('[role="alert"]')
    expect(alert?.textContent).toBe('invalid document')
  })
})
