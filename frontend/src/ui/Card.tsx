import type { HTMLAttributes, ReactNode } from 'react'

type Props = Omit<HTMLAttributes<HTMLElement>, 'title'> & { title?: ReactNode }

export function Card({ title, className = '', children, ...rest }: Props) {
  return (
    <section
      className={`rounded-lg border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-700 dark:bg-slate-800 ${className}`}
      {...rest}
    >
      {title !== undefined && <h2 className="mb-3">{title}</h2>}
      {children}
    </section>
  )
}
