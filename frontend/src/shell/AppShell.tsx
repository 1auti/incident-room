import type { ReactNode } from 'react'
import type { User } from '../api/auth'
import { Sidebar } from './Sidebar'
import { Topbar } from './Topbar'
import type { View } from './useView'

interface Props {
  user: User
  view: View
  onNavigate: (view: View) => void
  onDeclare: () => void
  onOpenRunbooks: () => void
  onOpenCopilot: () => void
  children: ReactNode
}

export function AppShell({ user, view, onNavigate, onDeclare, onOpenRunbooks, onOpenCopilot, children }: Props) {
  return (
    <main data-testid="app-home" className="min-h-screen">
      <Sidebar view={view} onNavigate={onNavigate} />
      <div className="min-w-0 md:pl-56">
        <Topbar user={user} onDeclare={onDeclare} onOpenRunbooks={onOpenRunbooks} onOpenCopilot={onOpenCopilot} />
        <div className="mx-auto min-w-0 max-w-7xl p-4">{children}</div>
      </div>
    </main>
  )
}
