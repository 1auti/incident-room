import { ApiError, request } from './client'

export type PostmortemStatus = 'borrador' | 'aprobado'

export interface Postmortem {
  id: string
  incident_id: string
  content: string
  status: PostmortemStatus
  generated_by_llm: boolean
  approved_by: string | null
  approved_at: string | null
  updated_at: string
}

function path(incidentId: string): string {
  return `/api/incidents/${encodeURIComponent(incidentId)}/postmortem`
}

function required(res: Postmortem | undefined): Postmortem {
  if (!res) throw new ApiError(500, 'empty response')
  return res
}

// null when the incident has no postmortem yet (404).
export async function getPostmortem(incidentId: string): Promise<Postmortem | null> {
  try {
    return required(await request<Postmortem>(path(incidentId)))
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) return null
    throw err
  }
}

export async function generatePostmortem(incidentId: string): Promise<Postmortem> {
  return required(await request<Postmortem>(`${path(incidentId)}/generate`, { method: 'POST' }))
}

export async function editPostmortem(incidentId: string, content: string): Promise<Postmortem> {
  return required(await request<Postmortem>(path(incidentId), { method: 'PUT', body: { content } }))
}

export async function approvePostmortem(incidentId: string): Promise<Postmortem> {
  return required(await request<Postmortem>(`${path(incidentId)}/approve`, { method: 'POST' }))
}
