// Fake Ollama for the e2e suite: the backend talks to it through OLLAMA_URL exactly as to a real
// server, so the production binary carries no test code. POST /api/embed answers a deterministic
// 768-dimension bag of hashed words, normalized: the same text always gives the same vector, a
// different text a different one, and texts sharing words are close (useful for RAG in UC-10).
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
  // Like the real server's root: lets Playwright wait until it is up.
  if (req.method === 'GET' && req.url === '/') {
    res.writeHead(200, { 'Content-Type': 'text/plain' })
    res.end('Ollama is running')
    return
  }
  res.writeHead(404)
  res.end()
}).listen(port, '127.0.0.1')
