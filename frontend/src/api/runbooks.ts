import { ApiError, request } from './client'

export interface Runbook {
  id: string
  service_id: string
  title: string
  content: string
  created_by: string
  updated_at: string
}

export async function listRunbooks(serviceId: string): Promise<Runbook[]> {
  return (await request<Runbook[]>(`/api/services/${encodeURIComponent(serviceId)}/runbooks`)) ?? []
}

export async function createRunbook(serviceId: string, title: string, content: string): Promise<Runbook> {
  const res = await request<Runbook>(`/api/services/${encodeURIComponent(serviceId)}/runbooks`, {
    method: 'POST',
    body: { title, content },
  })
  if (!res) throw new ApiError(500, 'empty response')
  return res
}

export async function updateRunbook(id: string, title: string, content: string): Promise<Runbook> {
  const res = await request<Runbook>(`/api/runbooks/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: { title, content },
  })
  if (!res) throw new ApiError(500, 'empty response')
  return res
}

export async function deleteRunbook(id: string): Promise<void> {
  await request<undefined>(`/api/runbooks/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
