import { cloneElement, type ReactElement } from 'react'

export function ScopeGate({
  allowed,
  missingScope,
  children,
}: {
  allowed: boolean
  missingScope: string
  children: ReactElement<{ disabled?: boolean }>
}) {
  if (allowed) {
    return children
  }
  return (
    <span className="scope-gate">
      {cloneElement(children, { disabled: true })}
      <span className="scope-missing">Missing scope {missingScope}</span>
    </span>
  )
}
