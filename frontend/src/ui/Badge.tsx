import type { HTMLAttributes } from 'react'

export type BadgeTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger'

const tones: Record<BadgeTone, string> = {
  neutral: 'bg-slate-100 text-slate-700 dark:bg-slate-700 dark:text-slate-200',
  info: 'bg-sky-100 text-sky-800 dark:bg-sky-900 dark:text-sky-200',
  success: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-200',
  warning: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
  danger: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
}

type Props = HTMLAttributes<HTMLSpanElement> & { tone?: BadgeTone }

// The caller picks the tone from the data value (severity, state); this component has no domain logic.
export function Badge({ tone = 'neutral', className = '', ...rest }: Props) {
  return (
    <span
      className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${tones[tone]} ${className}`}
      {...rest}
    />
  )
}
