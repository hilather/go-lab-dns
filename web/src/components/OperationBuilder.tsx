import { useState } from 'react'
import { OP_KINDS, TARGET_KINDS, type Operation, type OpKind } from '../pages/changes/changeIn'

function emptyOp(): Operation {
  return { op: 'add', target: { kind: 'record' }, value: {} }
}

function valueText(op: Operation): string {
  if (op.value === undefined) {
    return ''
  }
  try {
    return JSON.stringify(op.value, null, 2)
  } catch {
    return ''
  }
}

function OperationValueField({
  op,
  disabled,
  onValue,
}: {
  op: Operation
  disabled: boolean
  onValue: (value: unknown | undefined) => void
}) {
  const [text, setText] = useState(() => valueText(op))
  const [error, setError] = useState('')

  return (
    <label className="operation-value">
      Value (JSON)
      <textarea
        spellCheck={false}
        rows={6}
        disabled={disabled}
        value={text}
        onChange={(ev) => {
          const next = ev.target.value
          setText(next)
          const trimmed = next.trim()
          if (trimmed === '') {
            setError('')
            onValue(undefined)
            return
          }
          try {
            onValue(JSON.parse(trimmed))
            setError('')
          } catch {
            setError('Invalid JSON value')
          }
        }}
      />
      {error !== '' ? (
        <p role="alert" className="problem">
          {error}
        </p>
      ) : null}
    </label>
  )
}

export function OperationBuilder({
  operations,
  onChange,
  disabled = false,
}: {
  operations: Operation[]
  onChange: (next: Operation[]) => void
  disabled?: boolean
}) {
  function update(i: number, next: Operation) {
    onChange(operations.map((op, idx) => (idx === i ? next : op)))
  }

  return (
    <div className="operation-builder">
      <p>
        <button
          type="button"
          disabled={disabled}
          onClick={() => onChange([...operations, emptyOp()])}
        >
          Add operation
        </button>
      </p>
      {operations.length === 0 ? (
        <p>No operations. Add one, or paste a YAML/JSON change envelope.</p>
      ) : null}
      <ol className="operation-list">
        {operations.map((op, i) => (
          <li key={i} className="operation-row">
            <div className="operation-row-fields">
              <label>
                Op
                <select
                  value={op.op}
                  disabled={disabled}
                  onChange={(ev) => update(i, { ...op, op: ev.target.value as OpKind })}
                >
                  {OP_KINDS.map((k) => (
                    <option key={k} value={k}>
                      {k}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Kind
                <select
                  value={op.target.kind}
                  disabled={disabled}
                  onChange={(ev) =>
                    update(i, { ...op, target: { ...op.target, kind: ev.target.value } })
                  }
                >
                  {TARGET_KINDS.map((k) => (
                    <option key={k} value={k}>
                      {k}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                ID
                <input
                  type="text"
                  value={op.target.id ?? ''}
                  disabled={disabled}
                  onChange={(ev) => {
                    const id = ev.target.value
                    const target = { ...op.target }
                    if (id === '') {
                      delete target.id
                    } else {
                      target.id = id
                    }
                    update(i, { ...op, target })
                  }}
                />
              </label>
              <label>
                Zone ID
                <input
                  type="text"
                  value={op.target.zoneId ?? ''}
                  disabled={disabled}
                  onChange={(ev) => {
                    const zoneId = ev.target.value
                    const target = { ...op.target }
                    if (zoneId === '') {
                      delete target.zoneId
                    } else {
                      target.zoneId = zoneId
                    }
                    update(i, { ...op, target })
                  }}
                />
              </label>
              <button
                type="button"
                disabled={disabled}
                onClick={() => onChange(operations.filter((_, idx) => idx !== i))}
              >
                Remove
              </button>
            </div>
            {op.op === 'remove' ? null : (
              <OperationValueField
                op={op}
                disabled={disabled}
                onValue={(value) => {
                  const copy: Operation = { op: op.op, target: { ...op.target } }
                  if (value !== undefined) {
                    copy.value = value
                  }
                  update(i, copy)
                }}
              />
            )}
          </li>
        ))}
      </ol>
    </div>
  )
}
