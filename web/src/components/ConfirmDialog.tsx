import type { ReactNode } from 'react'

export function ConfirmDialog({
  open,
  title,
  children,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  confirmDisabled = false,
  onConfirm,
  onCancel,
}: {
  open: boolean
  title: string
  children?: ReactNode
  confirmLabel?: string
  cancelLabel?: string
  confirmDisabled?: boolean
  onConfirm: () => void
  onCancel: () => void
}) {
  if (!open) {
    return null
  }
  return (
    <dialog className="confirm-dialog" open aria-labelledby="confirm-dialog-title" onCancel={onCancel}>
      <form
        method="dialog"
        onSubmit={(ev) => {
          ev.preventDefault()
          if (!confirmDisabled) {
            onConfirm()
          }
        }}
      >
        <h2 id="confirm-dialog-title">{title}</h2>
        {children}
        <p className="confirm-actions">
          <button type="button" onClick={onCancel}>
            {cancelLabel}
          </button>
          <button type="submit" disabled={confirmDisabled}>
            {confirmLabel}
          </button>
        </p>
      </form>
    </dialog>
  )
}
