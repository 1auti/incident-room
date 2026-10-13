import { ApiError, request } from './client'

export type Impact = 'caida_total' | 'degradacion' | 'menor'
export type Severity = 'SEV1' | 'SEV2' | 'SEV3'
export type State = 'declarado' | 'reconocido' | 'mitigando' | 'resuelto' | 'cerrado'

export interface Incident {
  id: string
  title: string
  description: string
  service_id: string
  impact: Impact
  suggested_severity: Severity
  severity: Severity
  state: State
  declared_by: string
  assigned_to: string | null
  declared_at: string
  acknowledged_at: string | null
  escalated_at: string | null
}

export interface TimelineEvent {
  id: string
  incident_id: string
  type: string
  author_id: string | null
  body: string
  data: Record<string, string>
  occurred_at: string
}

// A timeline entry as listed: author_name is null for system events (e.g. escalado).
export interface TimelineEntry extends TimelineEvent {
  author_name: string | null
}

export interface DeclareResult {
  incident: Incident
  timeline: TimelineEvent[]
}

export interface DeclareInput {
  title: string
  description: string
  service_id: string
  impact: Impact | ''
  severity: Severity | ''
}

export async function suggestSeverity(serviceId: string, impact: Impact): Promise<Severity> {
  const query = new URLSearchParams({ service_id: serviceId, impact })
  const res = await request<{ suggested_severity: Severity }>(`/api/incidents/suggested-severity?${query.toString()}`)
  if (!res) throw new ApiError(500, 'empty response')
  return res.suggested_severity
}

export async function declareIncident(input: DeclareInput): Promise<DeclareResult> {
  const res = await request<DeclareResult>('/api/incidents', { method: 'POST', body: input })
  if (!res) throw new ApiError(500, 'empty response')
  return res
}

export interface IncidentFilter {
  severity?: Severity | ''
  service_id?: string
  state?: State | ''
}

// Empty filters are omitted so the backend applies no filter for them.
export async function listIncidents(filter: IncidentFilter = {}): Promise<Incident[]> {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(filter)) {
    if (value) query.set(key, value)
  }
  const qs = query.toString()
  return (await request<Incident[]>(`/api/incidents${qs === '' ? '' : `?${qs}`}`)) ?? []
}

export async function getIncident(id: string): Promise<Incident> {
  const res = await request<Incident>(`/api/incidents/${encodeURIComponent(id)}`)
  if (!res) throw new ApiError(500, 'empty response')
  return res
}

export async function transitionIncident(id: string, to: State): Promise<Incident> {
  const res = await request<Incident>(`/api/incidents/${encodeURIComponent(id)}/transitions`, {
    method: 'POST',
    body: { to },
  })
  if (!res) throw new ApiError(500, 'empty response')
  return res
}

export async function getTimeline(id: string): Promise<TimelineEntry[]> {
  return (await request<TimelineEntry[]>(`/api/incidents/${encodeURIComponent(id)}/timeline`)) ?? []
}

export async function addNote(id: string, body: string): Promise<TimelineEvent> {
  const res = await request<TimelineEvent>(`/api/incidents/${encodeURIComponent(id)}/notes`, {
    method: 'POST',
    body: { body },
  })
  if (!res) throw new ApiError(500, 'empty response')
  return res
}

export async function changeSeverity(id: string, severity: Severity): Promise<Incident> {
  const res = await request<Incident>(`/api/incidents/${encodeURIComponent(id)}/severity`, {
    method: 'PATCH',
    body: { severity },
  })
  if (!res) throw new ApiError(500, 'empty response')
  return res
}
