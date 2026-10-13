import type { LabelHTMLAttributes } from 'react'

export function Field({ className = '', ...rest }: LabelHTMLAttributes<HTMLLabelElement>) {
  return <label className={`flex flex-col gap-1 text-sm font-medium ${className}`} {...rest} />
}

// Shared look for inputs, selects and textareas placed inside a Field.
export const controlClass =
  'rounded-md border border-slate-300 bg-white px-2 py-1.5 text-sm font-normal text-slate-900 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100'
