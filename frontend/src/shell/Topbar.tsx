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
    <header className="sticky top-0 z-10 flex flex-wrap items-center justify-between gap-x-3 gap-y-2 border-b border-slate-800 bg-slate-950/90 px-3 py-2 sm:px-4 sm:py-3 backdrop-blur">
      <div className="flex flex-wrap items-center gap-1.5 sm:gap-2">
        <Button data-testid="declare-incident-open" onClick={onDeclare} className="px-2.5 font-semibold sm:px-3">
          Declarar incidente
        </Button>
        <Button variant="secondary" data-testid="runbooks-open" onClick={onOpenRunbooks} className="px-2.5 sm:px-3">
          Runbooks
        </Button>
        <Button variant="secondary" data-testid="copilot-open" onClick={onOpenCopilot} className="px-2.5 sm:px-3">
          Copiloto
        </Button>
      </div>
      <p className="min-w-0 max-w-full truncate text-xs text-slate-400 sm:text-sm">
        <span data-testid="current-user-name" className="font-medium text-white">
          {user.name}
        </span>{' '}
        (<span data-testid="current-user-role">{user.role}</span>)
      </p>
    </header>
  )
}
