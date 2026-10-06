import { ApiError, request } from './client'

export type Impact = 'caida_total' | 'degradacion' | 'menor'
export type Severity = 'SEV1' | 'SEV2' | 'SEV3'

export interface Incident {
  id: string
  title: string
  description: string
  service_id: string
  impact: Impact
  suggested_severity: Severity
  severity: Severity
  state: string
  declared_by: string
  assigned_to: string | null
  declared_at: string
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
