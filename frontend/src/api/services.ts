import { ApiError, request } from './client'

export type Criticality = 'critica' | 'importante' | 'estandar'

export interface Service {
  id: string
  name: string
  criticality: Criticality
  oncall_user_id: string | null
}

export async function listServices(): Promise<Service[]> {
  return (await request<Service[]>('/api/services')) ?? []
}

export async function createService(name: string, criticality: Criticality): Promise<Service> {
  const service = await request<Service>('/api/services', { method: 'POST', body: { name, criticality } })
  if (!service) throw new ApiError(500, 'empty response')
  return service
}

export async function updateService(id: string, name: string, criticality: Criticality): Promise<Service> {
  const service = await request<Service>(`/api/services/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: { name, criticality },
  })
  if (!service) throw new ApiError(500, 'empty response')
  return service
}

export async function deleteService(id: string): Promise<void> {
  await request<never>(`/api/services/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export async function setServiceOncall(id: string, userId: string): Promise<Service> {
  const service = await request<Service>(`/api/services/${encodeURIComponent(id)}/oncall`, {
    method: 'PUT',
    body: { user_id: userId },
  })
  if (!service) throw new ApiError(500, 'empty response')
  return service
}
