import { useState } from 'react'
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

export default function App() {
  const [selectedIncidentId, setSelectedIncidentId] = useState<string | null>(null)
  const [view, setView] = useView()
  const [declareOpen, setDeclareOpen] = useState(false)
  const [runbooksOpen, setRunbooksOpen] = useState(false)
  const [copilotOpen, setCopilotOpen] = useState(false)
  // Bumped after a declaration so the board and the KPIs remount and refetch the new incident.
  const [refreshKey, setRefreshKey] = useState(0)
  return (
    <AuthGate>
      {(user) => (
        <AppShell
          user={user}
          view={view}
          onNavigate={setView}
          onDeclare={() => setDeclareOpen(true)}
          onOpenRunbooks={() => setRunbooksOpen(true)}
          onOpenCopilot={() => setCopilotOpen(true)}
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
              className={`grid items-start gap-4 ${selectedIncidentId !== null ? 'lg:grid-cols-[minmax(0,1fr)_30rem]' : ''}`}
            >
              <IncidentBoard key={refreshKey} onOpen={setSelectedIncidentId} />
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
          <Modal open={declareOpen} onClose={() => setDeclareOpen(false)} label="Declarar incidente">
            <DeclareIncidentForm onDeclared={() => setRefreshKey((k) => k + 1)} />
          </Modal>
          <Drawer open={runbooksOpen} onClose={() => setRunbooksOpen(false)} label="Runbooks">
            <RunbooksPanel user={user} />
          </Drawer>
          <Drawer open={copilotOpen} onClose={() => setCopilotOpen(false)} label="Copiloto">
            <CopilotPanel />
          </Drawer>
        </AppShell>
      )}
    </AuthGate>
  )
}
