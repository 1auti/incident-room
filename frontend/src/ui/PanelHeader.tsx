// Title row shared by Modal and Drawer: the close button lives inside the panel header.
// `titleId` is the id the dialog points at with aria-labelledby.
export function PanelHeader({ label, titleId, onClose }: { label: string; titleId: string; onClose: () => void }) {
  return (
    <div className="flex shrink-0 items-center justify-between gap-3 border-b border-slate-800 px-4 py-3">
      <h2 id={titleId}>{label}</h2>
      <button
        type="button"
        aria-label="Cerrar"
        onClick={onClose}
        className="rounded-md px-2 py-1 text-slate-400 hover:bg-slate-800 hover:text-white"
      >
        ✕
      </button>
    </div>
  )
}
