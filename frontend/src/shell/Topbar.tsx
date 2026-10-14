import type { User } from '../api/auth'
import { Button } from '../ui/Button'

interface Props {
  user: User
  onDeclare: () => void
  onOpenRunbooks: () => void
  onOpenCopilot: () => void
}

export function Topbar({ user, onDeclare, onOpenRunbooks, onOpenCopilot }: Props) {
  return (
    <header className="md:sticky md:top-0 md:z-10 flex flex-wrap items-center justify-between gap-x-3 gap-y-2 border-b border-slate-800 bg-slate-950/90 px-3 py-2 sm:px-4 sm:py-3 md:backdrop-blur">
      <div className="flex flex-wrap items-center gap-1.5 sm:gap-2">
        {/* Below md it floats bottom-right (always reachable, the bar scrolls away); from md it sits in the bar. */}
        <Button
          data-testid="declare-incident-open"
          onClick={onDeclare}
          className="fixed right-4 bottom-4 z-20 rounded-full px-4 py-3 font-semibold shadow-lg shadow-slate-950/60 md:static md:z-auto md:rounded-md md:px-3 md:py-1.5 md:shadow-none"
        >
          Declarar incidente
        </Button>
        <Button variant="secondary" data-testid="runbooks-open" onClick={onOpenRunbooks} className="px-2.5 sm:px-3">
          Runbooks
        </Button>
        <Button variant="secondary" data-testid="copilot-open" onClick={onOpenCopilot} className="px-2.5 sm:px-3">
          Copiloto
        </Button>
      </div>
      <p className="order-first w-full min-w-0 truncate text-sm text-slate-400 sm:order-none sm:w-auto">
        <span data-testid="current-user-name" className="font-medium text-white">
          {user.name}
        </span>{' '}
        (<span data-testid="current-user-role">{user.role}</span>)
      </p>
    </header>
  )
}
