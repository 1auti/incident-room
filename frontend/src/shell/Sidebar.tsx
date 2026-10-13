import type { View } from './useView'

const items: { view: View; label: string; testId: string }[] = [
  { view: 'dashboard', label: 'Dashboard', testId: 'nav-dashboard' },
  { view: 'incidents', label: 'Incidentes', testId: 'nav-incidents' },
  { view: 'services', label: 'Servicios', testId: 'nav-services' },
  { view: 'metrics', label: 'Métricas', testId: 'nav-metrics' },
]

interface Props {
  view: View
  onNavigate: (view: View) => void
}

// Fixed on the left from md up; on mobile it collapses into a horizontal bar at the top.
export function Sidebar({ view, onNavigate }: Props) {
  return (
    <aside className="border-b border-slate-800 bg-slate-900 md:fixed md:inset-y-0 md:left-0 md:w-56 md:border-r md:border-b-0">
      <div className="flex items-center gap-2 px-4 py-3 md:py-5">
        <span aria-hidden="true" className="h-2.5 w-2.5 rounded-full bg-blue-500 shadow-[0_0_8px] shadow-blue-500" />
        <h1 className="text-base">Incident Room</h1>
      </div>
      <nav aria-label="Principal" className="flex gap-1 overflow-x-auto px-2 pb-2 md:flex-col md:pb-0">
        {items.map((item) => {
          const active = item.view === view
          return (
            <button
              key={item.view}
              type="button"
              data-testid={item.testId}
              aria-current={active ? 'page' : undefined}
              onClick={() => onNavigate(item.view)}
              className={`rounded-md px-3 py-2 text-left text-sm font-medium whitespace-nowrap transition-colors ${
                active
                  ? 'bg-blue-600/20 text-white ring-1 ring-blue-500/50'
                  : 'text-slate-400 hover:bg-slate-800 hover:text-white'
              }`}
            >
              {item.label}
            </button>
          )
        })}
      </nav>
    </aside>
  )
}
