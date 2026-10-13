import { ApiError, request } from './client'

export interface ServiceCount {
  service_id: string
  name: string
  count: number
}

// null means "no value" (no data in the period), never 0 (BR-16, BR-18).
export interface Dashboard {
  from: string
  to: string
  mttr_seconds: number | null
  incidents_by_service: ServiceCount[]
  sla_compliance_percent: number | null
}

// A preset counts back from the backend's clock; a range is [from, to) as RFC 3339 instants.
export type MetricsPeriod = { preset: '7d' | '30d' } | { from: string; to: string }

export async function getDashboard(period: MetricsPeriod): Promise<Dashboard> {
  const query = new URLSearchParams('preset' in period ? { period: period.preset } : period)
  const res = await request<Dashboard>(`/api/metrics?${query.toString()}`)
  if (!res) throw new ApiError(500, 'empty response')
  return res
}
