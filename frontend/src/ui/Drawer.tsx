import type { ReactNode } from 'react'
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
        className="drawer-panel absolute inset-y-0 right-0 w-full max-w-lg overflow-y-auto border-l border-slate-800 bg-slate-900 p-4 shadow-2xl outline-none"
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
