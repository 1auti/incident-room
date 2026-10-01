import { ApiError, request } from './client'

export type Role = 'ingeniero' | 'oncall' | 'admin'

export interface User {
  id: string
  name: string
  email: string
  role: Role
}

export async function register(name: string, email: string, password: string): Promise<User> {
  const user = await request<User>('/api/auth/register', { method: 'POST', body: { name, email, password } })
  if (!user) throw new ApiError(500, 'empty response')
  return user
}

export async function login(email: string, password: string): Promise<void> {
  await request<never>('/api/auth/login', { method: 'POST', body: { email, password } })
}

/** Returns the current user, or null when there is no valid session. */
export async function getMe(): Promise<User | null> {
  try {
    return (await request<User>('/api/me')) ?? null
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) return null
    throw err
  }
}
