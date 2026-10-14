import type { HTMLAttributes } from 'react'

type Tone = 'error' | 'status'

const tones: Record<Tone, string> = {
  error: 'border-red-900 bg-red-950/60 text-red-200',
  status: 'border-emerald-900 bg-emerald-950/60 text-emerald-200',
}

type Props = HTMLAttributes<HTMLElement> & { tone?: Tone }

// Native props (role, data-testid) are forwarded as-is; the caller keeps its own role.
export function Alert({ tone = 'error', className = '', ...rest }: Props) {
  return <p className={`rounded-md border px-3 py-2 text-sm ${tones[tone]} ${className}`} {...rest} />
}
