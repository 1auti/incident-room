import { ApiError, request } from './client'

// The runbooks the backend retrieved for the answer; never something the model claims.
export interface CopilotSource {
  runbook_id: string
  title: string
  service_id: string
  service_name: string
}

export interface CopilotAction {
  tool: string
  ok: boolean
  result: string
}

export interface CopilotAnswer {
  reply: string
  sources: CopilotSource[]
  actions: CopilotAction[]
}

export async function askCopilot(message: string): Promise<CopilotAnswer> {
  const res = await request<CopilotAnswer>('/api/copilot', { method: 'POST', body: { message } })
  if (!res) throw new ApiError(500, 'empty response')
  return res
}
