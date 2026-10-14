import type { ReactNode } from 'react'
import { PanelHeader } from './PanelHeader'
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
        className="my-8 w-full max-w-2xl rounded-lg border border-slate-800 bg-slate-900 shadow-2xl outline-none"
        onClick={(event) => event.stopPropagation()}
      >
        <PanelHeader label={label} onClose={onClose} />
        <div className="p-4">{children}</div>
      </div>
    </div>
  )
}
