import { useId, type ReactNode } from 'react'
import { PanelHeader } from './PanelHeader'
import { useBackdropClose, useDialogBehavior } from './useDialogBehavior'

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
  const backdropProps = useBackdropClose(onClose)
  const titleId = useId()
  return (
    <div
      data-testid="modal-backdrop"
      className="fixed inset-0 z-30 flex items-start justify-center overflow-x-hidden overflow-y-auto bg-slate-950/80 p-4 backdrop-blur-sm"
      {...backdropProps}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className="my-8 w-full max-w-2xl min-w-0 rounded-lg border border-slate-800 bg-slate-900 shadow-2xl outline-none"
      >
        <PanelHeader label={label} titleId={titleId} onClose={onClose} />
        <div className="min-w-0 p-4">{children}</div>
      </div>
    </div>
  )
}
