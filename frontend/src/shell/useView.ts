import { useCallback, useEffect, useState } from 'react'

export type View = 'dashboard' | 'incidents' | 'services' | 'metrics'

const hashByView: Record<View, string> = {
  dashboard: '#/dashboard',
  incidents: '#/incidentes',
  services: '#/servicios',
  metrics: '#/metricas',
}

function readView(): View {
  const entry = (Object.entries(hashByView) as [View, string][]).find(([, hash]) => hash === window.location.hash)
  return entry === undefined ? 'dashboard' : entry[0]
}

// View state lives in the URL hash so no router dependency is needed.
export function useView(): [View, (view: View) => void] {
  const [view, setViewState] = useState<View>(readView)

  useEffect(() => {
    const onHashChange = () => setViewState(readView())
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  const setView = useCallback((next: View) => {
    window.location.hash = hashByView[next]
  }, [])

  return [view, setView]
}
