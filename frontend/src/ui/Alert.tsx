import type { HTMLAttributes } from 'react'

type Tone = 'error' | 'status'

const tones: Record<Tone, string> = {
  error: 'border-red-300 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200',
  status:
    'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-200',
}

type Props = HTMLAttributes<HTMLElement> & { tone?: Tone }

// Native props (role, data-testid) are forwarded as-is; the caller keeps its own role.
export function Alert({ tone = 'error', className = '', ...rest }: Props) {
  return <p className={`rounded-md border px-3 py-2 text-sm ${tones[tone]} ${className}`} {...rest} />
}
