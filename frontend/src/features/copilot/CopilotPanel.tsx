import { useState, type FormEvent } from 'react'
import { askCopilot, type CopilotAnswer } from '../../api/copilot'
import { Alert } from '../../ui/Alert'
import { Button } from '../../ui/Button'
import { Card } from '../../ui/Card'
import { Field, controlClass } from '../../ui/Field'

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : 'Error inesperado'
}

type Turn = { role: 'user'; text: string } | { role: 'copilot'; answer: CopilotAnswer }

// Each question is answered on its own (the backend keeps no conversation); the history lives only
// in this screen. Tools run with the user's permissions (BR-14), so a refusal shows up as a failed action.
export function CopilotPanel() {
  const [turns, setTurns] = useState<Turn[]>([])
  const [message, setMessage] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function send(e: FormEvent) {
    e.preventDefault()
    const text = message
    setError(null)
    setPending(true)
    try {
      const answer = await askCopilot(text)
      setTurns((prev) => [...prev, { role: 'user', text }, { role: 'copilot', answer }])
      setMessage('')
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setPending(false)
    }
  }

  return (
    <Card data-testid="copilot-panel" title="Copiloto">
      <ol className="mb-3 flex flex-col gap-2">
        {turns.map((turn, i) =>
          turn.role === 'user' ? (
            <li
              key={i}
              data-testid="copilot-message"
              data-role="user"
              className="max-w-[85%] self-end rounded-lg bg-indigo-50 px-3 py-2 text-sm dark:bg-indigo-950"
            >
              <strong>Vos:</strong> {turn.text}
            </li>
          ) : (
            <li
              key={i}
              data-testid="copilot-message"
              data-role="copilot"
              className="max-w-[85%] self-start rounded-lg bg-slate-100 px-3 py-2 text-sm dark:bg-slate-700"
            >
              <strong>Copiloto:</strong> <span data-testid="copilot-reply">{turn.answer.reply}</span>
              {turn.answer.sources.length > 0 && (
                <ul aria-label="Fuentes" className="mt-2 list-disc pl-5 text-slate-600 dark:text-slate-300">
                  {turn.answer.sources.map((s) => (
                    <li key={s.runbook_id} data-testid="copilot-source">
                      Runbook: {s.title} ({s.service_name})
                    </li>
                  ))}
                </ul>
              )}
              {turn.answer.actions.length > 0 && (
                <ul aria-label="Acciones" className="mt-2 flex flex-col gap-1">
                  {turn.answer.actions.map((a, j) => (
                    <li
                      key={j}
                      data-testid="copilot-action"
                      data-ok={a.ok}
                      className={a.ok ? 'text-emerald-700 dark:text-emerald-300' : 'text-red-700 dark:text-red-300'}
                    >
                      {a.ok ? '✔' : '✖'} {a.tool}: {a.result}
                    </li>
                  ))}
                </ul>
              )}
            </li>
          ),
        )}
      </ol>
      {error && (
        <Alert role="alert" data-testid="copilot-error" className="mb-3">
          {error}
        </Alert>
      )}
      <form onSubmit={(e) => void send(e)} className="flex flex-wrap items-end gap-3">
        <Field className="min-w-48 flex-1">
          Mensaje
          <input
            data-testid="copilot-input"
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            className={controlClass}
          />
        </Field>
        <Button type="submit" data-testid="copilot-send" disabled={pending}>
          Enviar
        </Button>
        {pending && (
          <span role="status" data-testid="copilot-pending" className="text-sm text-slate-600 dark:text-slate-300">
            Pensando...
          </span>
        )}
      </form>
    </Card>
  )
}
