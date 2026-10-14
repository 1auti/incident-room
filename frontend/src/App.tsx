import { useEffect, useState } from 'react'
import { AuthGate } from './features/auth/AuthGate'
import { CopilotPanel } from './features/copilot/CopilotPanel'
import { KpiDashboard } from './features/dashboard/KpiDashboard'
import { IncidentBoard } from './features/incidents/IncidentBoard'
import { IncidentDetail } from './features/incidents/IncidentDetail'
import { DeclareIncidentForm } from './features/incidents/DeclareIncidentForm'
import { MetricsDashboard } from './features/metrics/MetricsDashboard'
import { RunbooksPanel } from './features/runbooks/RunbooksPanel'
import { ServicesPage } from './features/services/ServicesPage'
import { AppShell } from './shell/AppShell'
import { useView } from './shell/useView'
import { Drawer } from './ui/Drawer'
import { Modal } from './ui/Modal'

type Dialog = 'declare' | 'runbooks' | 'copilot'

export default function App() {
  const [selectedIncidentId, setSelectedIncidentId] = useState<string | null>(null)
  const [view, setView] = useView()
  // One dialog at a time: opening one replaces the others.
  const [dialog, setDialog] = useState<Dialog | null>(null)
  const closeDialog = () => setDialog(null)
  // Navigating (nav buttons, back/forward, a typed hash) closes whatever dialog is open.
  useEffect(() => {
    const onHashChange = () => setDialog(null)
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])
  // Bumped after a declaration so the board and the KPIs remount and refetch the new incident.
  const [refreshKey, setRefreshKey] = useState(0)
  return (
    <AuthGate>
      {(user) => (
        <AppShell
          user={user}
          view={view}
          onNavigate={setView}
          onDeclare={() => setDialog('declare')}
          onOpenRunbooks={() => setDialog('runbooks')}
          onOpenCopilot={() => setDialog('copilot')}
        >
          {view === 'dashboard' && (
            <KpiDashboard
              key={refreshKey}
              user={user}
              onOpenIncident={(id) => {
                setSelectedIncidentId(id)
                setView('incidents')
              }}
            />
          )}
          {view === 'incidents' && (
            <div
              className={`grid grid-cols-1 items-start gap-4 ${selectedIncidentId !== null ? 'lg:grid-cols-[minmax(0,1fr)_30rem]' : ''}`}
            >
              <IncidentBoard key={refreshKey} selectedId={selectedIncidentId} onOpen={setSelectedIncidentId} />
              {selectedIncidentId !== null && (
                // key: opening another incident starts a fresh detail, so no draft (e.g. a root cause) leaks into it.
                <IncidentDetail
                  key={selectedIncidentId}
                  incidentId={selectedIncidentId}
                  onClose={() => setSelectedIncidentId(null)}
                />
              )}
            </div>
          )}
          {view === 'services' && <ServicesPage user={user} />}
          {view === 'metrics' && <MetricsDashboard />}
          <Modal open={dialog === 'declare'} onClose={closeDialog} label="Declarar incidente">
            <DeclareIncidentForm onDeclared={() => setRefreshKey((k) => k + 1)} />
          </Modal>
          <Drawer open={dialog === 'runbooks'} onClose={closeDialog} label="Runbooks">
            <RunbooksPanel user={user} />
          </Drawer>
          <Drawer open={dialog === 'copilot'} onClose={closeDialog} label="Copiloto" keepMounted>
            <CopilotPanel />
          </Drawer>
        </AppShell>
      )}
    </AuthGate>
  )
}
