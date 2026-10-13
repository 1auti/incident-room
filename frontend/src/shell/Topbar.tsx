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
    <header className="sticky top-0 z-10 flex flex-wrap items-center justify-between gap-3 border-b border-slate-800 bg-slate-950/90 px-4 py-3 backdrop-blur">
      <div className="flex flex-wrap items-center gap-2">
        <Button data-testid="declare-incident-open" onClick={onDeclare} className="px-4 py-2 font-semibold">
          Declarar incidente
        </Button>
        <Button variant="secondary" data-testid="runbooks-open" onClick={onOpenRunbooks}>
          Runbooks
        </Button>
        <Button variant="secondary" data-testid="copilot-open" onClick={onOpenCopilot}>
          Copiloto
        </Button>
      </div>
      <p className="text-sm text-slate-400">
        <span data-testid="current-user-name" className="font-medium text-white">
          {user.name}
        </span>{' '}
        (<span data-testid="current-user-role">{user.role}</span>)
      </p>
    </header>
  )
}
