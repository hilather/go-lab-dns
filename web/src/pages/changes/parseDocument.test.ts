import { describe, expect, it } from 'vitest'
import { DocumentParseError, parseYamlOrJson } from './parseDocument'

describe('parseYamlOrJson', () => {
  it('parses JSON objects and arrays', () => {
    expect(parseYamlOrJson('{"op":"add"}')).toEqual({ op: 'add' })
    expect(parseYamlOrJson('[{"op":"add"}]')).toEqual([{ op: 'add' }])
  })

  it('parses a YAML operations envelope', () => {
    const doc = `
# change envelope
operations:
  - op: add
    target:
      kind: record
      id: www-a
      zoneId: lab-zone
    value:
      id: www-a
      owner: www
      type: A
      values: [10.42.0.80]
`
    expect(parseYamlOrJson(doc)).toEqual({
      operations: [
        {
          op: 'add',
          target: { kind: 'record', id: 'www-a', zoneId: 'lab-zone' },
          value: { id: 'www-a', owner: 'www', type: 'A', values: ['10.42.0.80'] },
        },
      ],
    })
  })

  it('parses a YAML LabDNS candidate fragment', () => {
    const doc = `
apiVersion: labdns.dev/v1alpha1
kind: LabDNS
metadata:
  name: primary-lab
spec:
  ui:
    enabled: true
  defaults:
    ttl: 30s
`
    expect(parseYamlOrJson(doc)).toEqual({
      apiVersion: 'labdns.dev/v1alpha1',
      kind: 'LabDNS',
      metadata: { name: 'primary-lab' },
      spec: { ui: { enabled: true }, defaults: { ttl: '30s' } },
    })
  })

  it('rejects empty documents', () => {
    expect(() => parseYamlOrJson('  \n  ')).toThrow(DocumentParseError)
  })
})
