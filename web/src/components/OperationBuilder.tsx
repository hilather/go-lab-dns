import {
  OP_KINDS,
  TARGET_KINDS,
  builderValueError,
  emptyBuilderRow,
  type BuilderRow,
  type OpKind,
} from '../pages/changes/changeIn'

export function OperationBuilder({
  rows,
  onChange,
  disabled = false,
}: {
  rows: BuilderRow[]
  onChange: (next: BuilderRow[]) => void
  disabled?: boolean
}) {
  function update(i: number, next: BuilderRow) {
    onChange(rows.map((row, idx) => (idx === i ? next : row)))
  }

  return (
    <div className="operation-builder">
      <p>
        <button type="button" disabled={disabled} onClick={() => onChange([...rows, emptyBuilderRow()])}>
          Add operation
        </button>
      </p>
      {rows.length === 0 ? <p>No operations. Add one, or paste a YAML/JSON change envelope.</p> : null}
      <ol className="operation-list">
        {rows.map((row, i) => {
          const valueError = builderValueError(row)
          return (
            <li key={row.key} className="operation-row" data-row-key={row.key}>
              <div className="operation-row-fields">
                <label>
                  Op
                  <select
                    value={row.op}
                    disabled={disabled}
                    onChange={(ev) => update(i, { ...row, op: ev.target.value as OpKind })}
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
                    value={row.target.kind}
                    disabled={disabled}
                    onChange={(ev) =>
                      update(i, { ...row, target: { ...row.target, kind: ev.target.value } })
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
                    value={row.target.id ?? ''}
                    disabled={disabled}
                    onChange={(ev) => {
                      const id = ev.target.value
                      const target = { ...row.target }
                      if (id === '') {
                        delete target.id
                      } else {
                        target.id = id
                      }
                      update(i, { ...row, target })
                    }}
                  />
                </label>
                <label>
                  Zone ID
                  <input
                    type="text"
                    value={row.target.zoneId ?? ''}
                    disabled={disabled}
                    onChange={(ev) => {
                      const zoneId = ev.target.value
                      const target = { ...row.target }
                      if (zoneId === '') {
                        delete target.zoneId
                      } else {
                        target.zoneId = zoneId
                      }
                      update(i, { ...row, target })
                    }}
                  />
                </label>
                <button
                  type="button"
                  disabled={disabled}
                  onClick={() => onChange(rows.filter((_, idx) => idx !== i))}
                >
                  Remove
                </button>
              </div>
              {row.op === 'remove' ? null : (
                <label className="operation-value">
                  Value (JSON)
                  <textarea
                    spellCheck={false}
                    rows={6}
                    disabled={disabled}
                    value={row.valueText}
                    onChange={(ev) => update(i, { ...row, valueText: ev.target.value })}
                  />
                  {valueError !== '' ? (
                    <p role="alert" className="problem">
                      {valueError}
                    </p>
                  ) : null}
                </label>
              )}
            </li>
          )
        })}
      </ol>
    </div>
  )
}
