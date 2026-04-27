import fs from 'node:fs'
import path from 'node:path'

const root = process.cwd()
const envText = fs.existsSync('.env') ? fs.readFileSync('.env', 'utf8') : ''
const key = envText.match(/^KEY=(.+)$/m)?.[1]?.trim() ?? process.env.KEY ?? process.env.OPENAI_API_KEY
const baseURL = envText.match(/https?:\/\/[^\s'"]+/)?.[0] ?? process.env.OPENAI_BASE_URL
const model = envText.includes('deepseek-v4-flash') ? 'deepseek-v4-flash' : (process.env.OPENAI_MODEL ?? 'deepseek-v4-flash')

if (!key || !baseURL) {
  console.error('Missing KEY or base URL in .env')
  process.exit(1)
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
  const res = await fetch(`${baseURL.replace(/\/$/, '')}/chat/completions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${key}`,
    },
    body: JSON.stringify(body),
    signal: AbortSignal.timeout(60000),
  })
  if (!res.ok) {
    throw new Error(`AI request failed ${res.status}: ${await res.text()}`)
  }
  const json = await res.json()
  return json.choices?.[0]?.message?.content?.trim() ?? ''
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

for (let i = 0; i < candidates.length; i += 10) {
  const batch = candidates.slice(i, i + 10)
  console.log(`enriching ${i + 1}-${i + batch.length} / ${candidates.length}`)
  const text = await callAI(batch)
  fs.appendFileSync(outPath, `\n${text}\n`)
}

console.log(`wrote ${outPath}`)

