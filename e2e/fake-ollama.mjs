// Fake Ollama for the e2e suite: the backend talks to it through OLLAMA_URL exactly as to a real
// server, so the production binary carries no test code. POST /api/embed answers a deterministic
// 768-dimension bag of hashed words, normalized: the same text always gives the same vector, a
// different text a different one, and texts sharing words are close (useful for RAG in UC-10).
// POST /v1/chat/completions is Ollama's OpenAI-compatible chat, used by both chat features:
// - without tools (the postmortem, UC-08) it answers a fixed analysis;
// - with tools (the copilot, UC-10) it is a rule-based stand-in for a model: keywords in the user's
//   message pick a tool call, tool results are summarized, and questions are answered from the
//   runbook in the system prompt. With no runbook it "invents" a procedure on purpose, so the e2e
//   proves the backend discards it. It exercises the backend's loop and rules, not model quality.
import { createServer } from 'node:http'

const port = Number(process.env.FAKE_OLLAMA_PORT ?? '11435')
const dimensions = 768

// FNV-1a: a stable, dependency-free string hash.
function hash(word) {
  let h = 0x811c9dc5
  for (const ch of word) {
    h ^= ch.codePointAt(0)
    h = Math.imul(h, 0x01000193) >>> 0
  }
  return h
}

function embed(text) {
  const v = new Array(dimensions).fill(0)
  for (const word of text.toLowerCase().match(/[\p{L}\p{N}]+/gu) ?? []) v[hash(word) % dimensions] += 1
  const norm = Math.hypot(...v)
  if (norm === 0) {
    v[0] = 1
    return v
  }
  return v.map((x) => x / norm)
}

const postmortemAnalysis = 'Análisis del LLM de prueba: el pool de conexiones se agotó.'

function completion({ content = null, toolCall = null }) {
  const message = { role: 'assistant', content }
  if (toolCall) {
    message.tool_calls = [
      {
        id: `call-${Date.now()}`,
        type: 'function',
        function: { name: toolCall.name, arguments: JSON.stringify(toolCall.args) },
      },
    ]
  }
  return {
    id: 'fake',
    object: 'chat.completion',
    model: 'fake',
    choices: [{ index: 0, finish_reason: toolCall ? 'tool_calls' : 'stop', message }],
  }
}

function textOf(message) {
  if (typeof message.content === 'string') return message.content
  return (message.content ?? []).map((p) => p.text ?? '').join('')
}

const impacts = { 'caída total': 'caida_total', 'caida total': 'caida_total', degradación: 'degradacion' }

// One turn of the rule-based copilot model.
function copilotTurn(messages) {
  const last = messages[messages.length - 1]
  if (last.role === 'tool') return { content: summarize(messages) }

  const said = textOf(last)
  const lower = said.toLowerCase()
  let m
  if ((m = lower.match(/abr[ií] un incidente en ([^,]+),\s*(.+)/))) {
    const impact = impacts[m[2].trim()] ?? 'menor'
    return { toolCall: { name: 'abrir_incidente', args: { servicio: m[1].trim(), impacto: impact, titulo: said } } }
  }
  if ((m = said.match(/agreg[aá] (?:una )?nota al incidente (\S+?):\s*(.+)/i))) {
    return { toolCall: { name: 'agregar_nota', args: { incidente_id: m[1], texto: m[2].trim() } } }
  }
  if (/list[aá] (?:los )?(?:incidentes )?abiertos/.test(lower)) {
    const args = {}
    if ((m = lower.match(/sev[123]/))) args.severidad = m[0].toUpperCase()
    if ((m = lower.match(/del servicio (\S+)/))) args.servicio = m[1]
    return { toolCall: { name: 'listar_abiertos', args } }
  }
  if (/cerr[aá]/.test(lower)) return { content: 'No puedo cerrar incidentes: no tengo una herramienta para eso.' }

  const system = messages
    .filter((m) => m.role === 'system')
    .map(textOf)
    .join('\n')
  const runbook = system.match(/\[Runbook: (.+?) \(servicio .+?\)\]\n([^\n]+)/)
  if (!runbook) return { content: 'Paso 1: reiniciá todos los servidores.' }
  return { content: `Según el runbook "${runbook[1]}": ${runbook[2]}` }
}

// Summarizes the result of the tool the model asked for in the previous turn.
function summarize(messages) {
  const result = textOf(messages[messages.length - 1])
  const name = messages[messages.length - 2].tool_calls?.[0]?.function?.name
  if (result.startsWith('error:')) return `No pude completar la acción: ${result}`
  const data = JSON.parse(result)
  if (name === 'abrir_incidente') return `Abrí el incidente ${data.id} con severidad ${data.severidad}.`
  if (name === 'agregar_nota') return `Agregué la nota al incidente ${data.incidente_id}.`
  if (data.length === 0) return 'No hay incidentes abiertos con ese filtro.'
  return `Incidentes abiertos: ${data.map((i) => `${i.id} (${i.severidad}, ${i.estado})`).join('; ')}.`
}

function readJson(req) {
  return new Promise((resolve, reject) => {
    let body = ''
    req.on('data', (chunk) => (body += chunk))
    req.on('end', () => {
      try {
        resolve(JSON.parse(body))
      } catch (err) {
        reject(err)
      }
    })
  })
}

createServer(async (req, res) => {
  if (req.method === 'POST' && req.url === '/api/embed') {
    let input
    try {
      ;({ input } = await readJson(req))
    } catch {
      res.writeHead(400)
      res.end()
      return
    }
    const inputs = Array.isArray(input) ? input : [input]
    res.writeHead(200, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ embeddings: inputs.map(embed) }))
    return
  }
  if (req.method === 'POST' && req.url === '/v1/chat/completions') {
    let body
    try {
      body = await readJson(req)
    } catch {
      res.writeHead(400)
      res.end()
      return
    }
    res.writeHead(200, { 'Content-Type': 'application/json' })
    res.end(
      JSON.stringify(completion(body.tools?.length ? copilotTurn(body.messages) : { content: postmortemAnalysis })),
    )
    return
  }
  // Like the real server's root: lets Playwright wait until it is up.
  if (req.method === 'GET' && req.url === '/') {
    res.writeHead(200, { 'Content-Type': 'text/plain' })
    res.end('Ollama is running')
    return
  }
  res.writeHead(404)
  res.end()
}).listen(port, '127.0.0.1')
