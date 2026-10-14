import type { ReactNode } from 'react'
import { PanelHeader } from './PanelHeader'
import { useDialogBehavior } from './useDialogBehavior'

interface Props {
  open: boolean
  onClose: () => void
  label: string
  children: ReactNode
}

export function Drawer({ open, onClose, label, children }: Props) {
  if (!open) return null
  return (
    <DrawerContent onClose={onClose} label={label}>
      {children}
    </DrawerContent>
  )
}

function DrawerContent({ onClose, label, children }: Omit<Props, 'open'>) {
  const panelRef = useDialogBehavior(onClose)
  return (
    <div
      data-testid="drawer-backdrop"
      className="fixed inset-0 z-30 bg-slate-950/70 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        tabIndex={-1}
        className="drawer-panel absolute inset-y-0 right-0 flex w-full max-w-lg flex-col border-l border-slate-800 bg-slate-900 shadow-2xl outline-none"
        onClick={(event) => event.stopPropagation()}
      >
        <PanelHeader label={label} onClose={onClose} />
        <div className="min-h-0 flex-1 overflow-y-auto p-4">{children}</div>
      </div>
    </div>
  )
}
