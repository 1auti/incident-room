import type { HTMLAttributes } from 'react'

export type BadgeTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger'

const tones: Record<BadgeTone, string> = {
  neutral: 'bg-slate-800 text-slate-300 ring-1 ring-inset ring-slate-700',
  info: 'bg-blue-950 text-blue-300 ring-1 ring-inset ring-blue-800',
  success: 'bg-emerald-950 text-emerald-300 ring-1 ring-inset ring-emerald-800',
  warning: 'bg-amber-950 text-amber-300 ring-1 ring-inset ring-amber-800',
  danger: 'bg-red-950 text-red-300 ring-1 ring-inset ring-red-800',
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
