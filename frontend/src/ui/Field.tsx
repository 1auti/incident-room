import type { LabelHTMLAttributes } from 'react'

export function Field({ className = '', ...rest }: LabelHTMLAttributes<HTMLLabelElement>) {
  return <label className={`flex min-w-0 flex-col gap-1 text-sm font-medium text-slate-200 ${className}`} {...rest} />
}

// Shared look for inputs, selects and textareas placed inside a Field.
export const controlClass =
  'max-w-full min-w-0 rounded-md border border-slate-700 bg-slate-950 px-2 py-1.5 text-sm font-normal text-slate-100 focus:border-blue-500'
