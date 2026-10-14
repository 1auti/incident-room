// Title row shared by Modal and Drawer: the close button lives inside the panel header.
export function PanelHeader({ label, onClose }: { label: string; onClose: () => void }) {
  return (
    <div className="flex shrink-0 items-center justify-between gap-3 border-b border-slate-800 px-4 py-3">
      <h2>{label}</h2>
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
