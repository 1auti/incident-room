import { useState } from 'react'
import { AuthGate } from './features/auth/AuthGate'
import { CopilotPanel } from './features/copilot/CopilotPanel'
import { IncidentBoard } from './features/incidents/IncidentBoard'
import { IncidentDetail } from './features/incidents/IncidentDetail'
import { DeclareIncidentForm } from './features/incidents/DeclareIncidentForm'
import { MetricsDashboard } from './features/metrics/MetricsDashboard'
import { RunbooksPanel } from './features/runbooks/RunbooksPanel'
import { ServicesPage } from './features/services/ServicesPage'
import { AppShell } from './shell/AppShell'
import { useView } from './shell/useView'
import { Card } from './ui/Card'
import { Drawer } from './ui/Drawer'
import { Modal } from './ui/Modal'

export default function App() {
  const [selectedIncidentId, setSelectedIncidentId] = useState<string | null>(null)
  const [view, setView] = useView()
  const [declareOpen, setDeclareOpen] = useState(false)
  const [runbooksOpen, setRunbooksOpen] = useState(false)
  const [copilotOpen, setCopilotOpen] = useState(false)
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
            <Card title="Dashboard" data-testid="kpi-dashboard-placeholder">
              <p className="text-sm">Resumen de incidentes próximamente.</p>
            </Card>
          )}
          {view === 'incidents' && (
            <div className="grid gap-4 lg:grid-cols-2">
              <IncidentBoard onOpen={setSelectedIncidentId} />
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
            <DeclareIncidentForm />
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
