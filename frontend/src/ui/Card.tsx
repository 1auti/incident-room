import type { HTMLAttributes, ReactNode } from 'react'

type Props = Omit<HTMLAttributes<HTMLElement>, 'title'> & { title?: ReactNode }

export function Card({ title, className = '', children, ...rest }: Props) {
  return (
    <section className={`min-w-0 rounded-lg border border-slate-800 bg-slate-900 p-4 shadow-sm ${className}`} {...rest}>
      {title !== undefined && <h2 className="mb-3">{title}</h2>}
      {children}
    </section>
  )
}
