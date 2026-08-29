import { useEffect, useRef, type ReactNode } from 'react'

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
  const ref = useRef<HTMLDialogElement>(null)

  useEffect(() => {
    const el = ref.current
    if (!el) {
      return
    }
    if (open) {
      if (!el.open) {
        el.showModal()
      }
    } else if (el.open) {
      el.close()
    }
  }, [open])

  return (
    <dialog
      ref={ref}
      className="confirm-dialog"
      aria-labelledby="confirm-dialog-title"
      onCancel={(ev) => {
        ev.preventDefault()
        onCancel()
      }}
    >
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
          <button type="submit" className="btn-accent" disabled={confirmDisabled}>
            {confirmLabel}
          </button>
        </p>
      </form>
    </dialog>
  )
}
