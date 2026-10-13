import type { ReactNode } from 'react'
import { useDialogBehavior } from './useDialogBehavior'

interface Props {
  open: boolean
  onClose: () => void
  label: string
  children: ReactNode
}

export function Modal({ open, onClose, label, children }: Props) {
  if (!open) return null
  return (
    <ModalContent onClose={onClose} label={label}>
      {children}
    </ModalContent>
  )
}

function ModalContent({ onClose, label, children }: Omit<Props, 'open'>) {
  const panelRef = useDialogBehavior(onClose)
  return (
    <div
      data-testid="modal-backdrop"
      className="fixed inset-0 z-30 flex items-start justify-center overflow-y-auto bg-slate-950/80 p-4 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        tabIndex={-1}
        className="my-8 w-full max-w-2xl rounded-lg border border-slate-800 bg-slate-900 p-4 shadow-2xl outline-none"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="mb-2 flex justify-end">
          <button
            type="button"
            aria-label="Cerrar"
            onClick={onClose}
            className="rounded-md px-2 py-1 text-slate-400 hover:bg-slate-800 hover:text-white"
          >
            ✕
          </button>
        </div>
        {children}
      </div>
    </div>
  )
}
