import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
loadEnvFile(path.join(root, '.env'))

const key = firstEnv('OPENAI_API_KEY', 'AI_API_KEY', 'KEY')
const baseURL = env('OPENAI_BASE_URL', 'https://api.openai.com/v1')
const model = env('OPENAI_MODEL', 'gpt-4.1-mini')
const batchSize = clampNumber(envNumber('AI_BATCH_SIZE', 10), 1, 25)
const timeoutMS = clampNumber(envNumber('AI_TIMEOUT_MS', 60000), 5000, 180000)
const retries = clampNumber(envNumber('AI_RETRIES', 1), 0, 3)

if (!key) {
  console.error('Missing OPENAI_API_KEY in .env or environment')
  process.exit(1)
}

function loadEnvFile(file) {
  if (!fs.existsSync(file)) return
  const lines = fs.readFileSync(file, 'utf8').split(/\r?\n/)
  for (const raw of lines) {
    let line = raw.trim()
    if (!line || line.startsWith('#')) continue
    if (line.startsWith('export ')) line = line.slice('export '.length).trim()
    const eq = line.indexOf('=')
    if (eq < 1) continue
    const name = line.slice(0, eq).trim()
    let value = line.slice(eq + 1).trim()
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1)
    }
    if (!process.env[name]) process.env[name] = value
  }
}

function env(name, fallback) {
  const value = process.env[name]?.trim()
  return value || fallback
}

function firstEnv(...names) {
  for (const name of names) {
    const value = process.env[name]?.trim()
    if (value) return value
  }
  return ''
}

function envNumber(name, fallback) {
  const value = Number(process.env[name])
  return Number.isFinite(value) ? value : fallback
}

function clampNumber(value, min, max) {
  return Math.min(max, Math.max(min, value))
}

function parseKeyterms(file) {
  const lines = fs.readFileSync(file, 'utf8').split(/\r?\n/)
  let inBody = false
  let unit = ''
  let topic = ''
  const rows = []
  for (const raw of lines) {
    const t = raw.trim()
    if (!t) continue
    if (t.includes('Quizlet review for that set')) {
      inBody = true
      unit = 'Science Practices'
      continue
    }
    if (t.includes('Quizlet review for that topic')) {
      inBody = true
      continue
    }
    if (!inBody) continue
    if (t.startsWith('Science Practices')) unit = 'Science Practices'
    else if (/^Unit\s+\d+:/.test(t)) unit = t
    else if (t.startsWith('Set ') || /^\d+\.\d+\s+-\s+/.test(t)) topic = t
    else if (t.startsWith('●')) rows.push({ unit, topic, term: t.slice(1).trim() })
  }
  return rows
}

function slug(s) {
  return s.toLowerCase().trim().replace(/[’]/g, "'").replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'item'
}

function unitSlug(title) {
  if (title.toLowerCase().startsWith('science practices')) return 'science-practices'
  const m = title.match(/^unit\s+(\d+)/i)
  return m ? `u${m[1]}` : `unit-${slug(title)}`
}

function topicSlug(title) {
  if (title.toLowerCase().startsWith('set ')) return slug(title)
  const m = title.match(/^(\d+)\.(\d+)/)
  return m ? `t${m[1]}-${m[2]}` : `topic-${slug(title)}`
}

function withIds(rows) {
  return rows.map((row) => {
    const unitID = `ap-psychology.${unitSlug(row.unit)}`
    const topicID = `${unitID}.${topicSlug(row.topic)}`
    return { ...row, id: `${topicID}.${slug(row.term)}` }
  })
}

async function callAI(batch) {
  const items = batch.map((x) => `${x.id} | ${x.term} | ${x.topic}`).join('\n')
  const body = {
    model,
    messages: [
      {
        role: 'system',
        content:
          'You create compact AP Psychology study content. Output only the requested compact format. Keep each line concise. Bilingual English/Chinese is preferred.',
      },
      {
        role: 'user',
        content:
          `For each concept, output exactly:\n@@ id\ndef: English definition / 中文解释\nex: one concrete example if useful\npit: common confusion or test trap if useful\nnote: AP exam cue if useful\n\nConcepts:\n${items}`,
      },
    ],
    temperature: 0.2,
    stream: false,
  }
  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      const res = await fetch(`${baseURL.replace(/\/$/, '')}/chat/completions`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${key}`,
        },
        body: JSON.stringify(body),
        signal: AbortSignal.timeout(timeoutMS),
      })
      if (!res.ok) {
        throw new Error(`AI request failed ${res.status}: ${await res.text()}`)
      }
      const json = await res.json()
      return json.choices?.[0]?.message?.content?.trim() ?? ''
    } catch (error) {
      if (attempt >= retries) throw error
      await sleep(750 * (attempt + 1))
    }
  }
  return ''
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

const rows = withIds(parseKeyterms(path.join(root, 'data/sources/keyterms.md')))
const unique = [...new Map(rows.map((row) => [row.id, row])).values()]
const topicArg = process.argv.find((arg) => arg.startsWith('--topic='))?.slice('--topic='.length)
const limit = Number(process.argv.find((arg) => arg.startsWith('--limit='))?.slice('--limit='.length) ?? '80')
const outPath = path.join(root, 'data/sources/ai-enrichment.compact')
const existing = fs.existsSync(outPath) ? fs.readFileSync(outPath, 'utf8') : ''
const done = new Set([...existing.matchAll(/^@@\s*(.+)$/gm)].map((m) => m[1].trim()))
const candidates = unique.filter((row) => !done.has(row.id) && (!topicArg || row.topic.includes(topicArg) || row.unit.includes(topicArg))).slice(0, limit)

fs.mkdirSync(path.dirname(outPath), { recursive: true })
if (!existing) fs.writeFileSync(outPath, '# Compact AI enrichment for AP Psych Final Sprint\n\n')

for (let i = 0; i < candidates.length; i += batchSize) {
  const batch = candidates.slice(i, i + batchSize)
  console.log(`enriching ${i + 1}-${i + batch.length} / ${candidates.length}`)
  const text = await callAI(batch)
  if (text) fs.appendFileSync(outPath, `\n${text}\n`)
}

console.log(`wrote ${outPath}`)
