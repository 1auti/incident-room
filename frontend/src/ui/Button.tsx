import type { ButtonHTMLAttributes } from 'react'

type Variant = 'primary' | 'secondary' | 'danger' | 'dangerGhost'

// Disabled buttons turn flat slate (readable text) instead of a dimmed version of their colour.
const disabled =
  'disabled:cursor-not-allowed disabled:border-slate-700 disabled:bg-slate-800 disabled:text-slate-400 disabled:shadow-none disabled:hover:bg-slate-800'

const variants: Record<Variant, string> = {
  primary: 'bg-blue-600 text-white shadow-sm hover:bg-blue-500',
  secondary: 'border-slate-700 bg-slate-800 text-slate-100 hover:border-slate-600 hover:bg-slate-700',
  danger: 'bg-red-600 text-white shadow-sm hover:bg-red-500',
  dangerGhost: 'border-red-900/70 bg-transparent text-red-300 hover:border-red-700 hover:bg-red-950/50',
}

type Props = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }

export function Button({ variant = 'primary', className = '', ...rest }: Props) {
  return (
    <button
      className={`rounded-md border border-transparent px-3 py-1.5 text-sm font-medium whitespace-nowrap transition-colors ${disabled} ${variants[variant]} ${className}`}
      {...rest}
    />
  )
}
