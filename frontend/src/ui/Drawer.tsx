import { useId, type ReactNode } from 'react'
import { PanelHeader } from './PanelHeader'
import { useBackdropClose, useDialogBehavior } from './useDialogBehavior'

interface Props {
  open: boolean
  onClose: () => void
  label: string
  // Keeps the children mounted (hidden) while closed so their state survives, e.g. a conversation.
  keepMounted?: boolean
  children: ReactNode
}

export function Drawer({ open, onClose, label, keepMounted = false, children }: Props) {
  if (!open && !keepMounted) return null
  return (
    <DrawerContent open={open} onClose={onClose} label={label}>
      {children}
    </DrawerContent>
  )
}

function DrawerContent({ open, onClose, label, children }: Omit<Props, 'keepMounted'>) {
  const panelRef = useDialogBehavior(onClose, open)
  const backdropProps = useBackdropClose(onClose)
  const titleId = useId()
  return (
    <div
      data-testid="drawer-backdrop"
      hidden={!open}
      className="fixed inset-0 z-30 bg-slate-950/70 backdrop-blur-sm"
      {...backdropProps}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className="drawer-panel absolute inset-y-0 right-0 flex w-full max-w-lg flex-col border-l border-slate-800 bg-slate-900 shadow-2xl outline-none"
      >
        <PanelHeader label={label} titleId={titleId} onClose={onClose} />
        <div className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto p-4">{children}</div>
      </div>
    </div>
  )
}
