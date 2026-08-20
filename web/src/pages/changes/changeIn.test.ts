import { describe, expect, it } from 'vitest'
import {
  builderValueError,
  changeFingerprint,
  compileBuilderRows,
  compileChangeIn,
  hasOperations,
  hasWriteScope,
  isPlanCurrent,
  operationToRow,
  parseEditorDocument,
  parseProblem,
  type PlannedChange,
} from './changeIn'

const addWww = {
  op: 'add' as const,
  target: { kind: 'record', id: 'www-a', zoneId: 'lab-zone' },
  value: { id: 'www-a', owner: 'www', type: 'A', values: ['10.42.0.80'] },
}

describe('compileChangeIn', () => {
  it('compiles a YAML envelope and a JSON document to the same ChangeIn shape', () => {
    const yaml = parseEditorDocument(`
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
      values:
        - 10.42.0.80
`)
    const json = parseEditorDocument(JSON.stringify({ operations: [addWww] }))
    const fromYaml = compileChangeIn(yaml, { expectedRevision: 'sha256:aaa' })
    const fromJson = compileChangeIn(json, { expectedRevision: 'sha256:aaa' })
    expect(fromYaml).toEqual(fromJson)
    expect(fromYaml.operations).toEqual([addWww])
    expect(fromYaml.expectedRevision).toBe('sha256:aaa')
    expect(hasOperations(fromYaml)).toBe(true)
  })

  it('places a LabDNS candidate document on state', () => {
    const parsed = parseEditorDocument(`
apiVersion: labdns.dev/v1alpha1
kind: LabDNS
spec:
  ui:
    enabled: false
`)
    const body = compileChangeIn(parsed, { expectedRevision: 'sha256:aaa' })
    expect(body.state).toEqual({
      apiVersion: 'labdns.dev/v1alpha1',
      kind: 'LabDNS',
      spec: { ui: { enabled: false } },
    })
    expect(body.operations).toBeUndefined()
  })

  it('uses structured operations from the builder', () => {
    const body = compileChangeIn(undefined, {
      expectedRevision: 'sha256:aaa',
      operations: [addWww],
      reason: 'add www',
    })
    expect(body.operations).toEqual([addWww])
    expect(body.reason).toBe('add www')
  })
})

describe('plan currency', () => {
  it('requires the same revision and fingerprint', () => {
    const body = compileChangeIn(undefined, { expectedRevision: 'r1', operations: [addWww] })
    const fp = changeFingerprint(body)
    const plan: PlannedChange = { revision: 'r1', fingerprint: fp, body: {} }
    expect(isPlanCurrent(plan, 'r1', fp)).toBe(true)
    expect(isPlanCurrent(plan, 'r2', fp)).toBe(false)
    expect(isPlanCurrent(null, 'r1', fp)).toBe(false)
    const edited = compileChangeIn(undefined, { expectedRevision: 'r1', operations: [] })
    expect(isPlanCurrent(plan, 'r1', changeFingerprint(edited))).toBe(false)
  })
})

describe('parseProblem', () => {
  it('reads revision_conflict currentRevision', () => {
    const p = parseProblem(
      { code: 'revision_conflict', detail: 'stale', currentRevision: 'sha256:live', expectedRevision: 'sha256:old' },
      409,
    )
    expect(p.code).toBe('revision_conflict')
    expect(p.currentRevision).toBe('sha256:live')
    expect(p.status).toBe(409)
  })
})

describe('compileBuilderRows', () => {
  it('does not keep a stale parsed value when JSON is invalid', () => {
    const row = operationToRow(addWww)
    expect(compileBuilderRows([row]).operations).toEqual([addWww])
    const broken = { ...row, valueText: '{' }
    expect(builderValueError(broken)).toBe('Invalid JSON value')
    const compiled = compileBuilderRows([broken])
    expect(compiled.error).toBe('Invalid JSON value')
    expect(compiled.operations).toEqual([])
  })
})

describe('hasWriteScope', () => {
  it('allows administrator and dns.write', () => {
    expect(hasWriteScope({ role: 'administrator', scopes: [] })).toBe(true)
    expect(hasWriteScope({ role: 'editor', scopes: ['dns.write'] })).toBe(true)
    expect(hasWriteScope({ role: 'viewer', scopes: ['dns.read'] })).toBe(false)
    expect(hasWriteScope(null)).toBe(false)
  })
})
