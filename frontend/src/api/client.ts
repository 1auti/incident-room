export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function errorMessage(res: Response): Promise<string> {
  const text = await res.text()
  try {
    const body: unknown = JSON.parse(text)
    if (typeof body === 'object' && body !== null && 'error' in body && typeof body.error === 'string') {
      return body.error
    }
  } catch {
    // Not JSON (e.g. the plain-text 401 without session): use the raw text.
  }
  return text || res.statusText
}

export async function request<T>(path: string, init: { method?: string; body?: unknown } = {}): Promise<T | undefined> {
  const res = await fetch(path, {
    method: init.method ?? 'GET',
    credentials: 'same-origin',
    headers: init.body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: init.body === undefined ? undefined : JSON.stringify(init.body),
  })
  if (!res.ok) {
    throw new ApiError(res.status, await errorMessage(res))
  }
  const text = await res.text()
  return text === '' ? undefined : (JSON.parse(text) as T)
}
