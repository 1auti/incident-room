import type { Role, User } from './auth'
import { request } from './client'

/** Lists the users with a role. Admin only (BR-12). */
export async function listUsers(role: Role): Promise<User[]> {
  const query = new URLSearchParams({ role })
  return (await request<User[]>(`/api/users?${query.toString()}`)) ?? []
}
