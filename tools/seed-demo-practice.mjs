// Demo seed for the practice upgrade round: MCQ question set (shared
// passage with a chart image), an AAQ article question, an EBQ three-source
// question, plus a student account with attempt history. Idempotent — safe
// to re-run; existing rows are found by title/stem and reused.
//
//   node tools/seed-demo-practice.mjs
//
// Requires the backend on http://localhost:8080 and the dev seed account
// from docs/async-interaction.md (dev.seed@tsinglan.org).

import { readFile } from 'node:fs/promises'

const BASE = process.env.API_BASE ?? 'http://localhost:8080'
const TEACHER = { email: 'dev.seed@tsinglan.org', password: 'seed-test-2026' }
const STUDENT = { name: 'Demo Student', email: 'demo-student@tsinglan.org', password: 'demo-student-2026' }

async function call(token, method, path, body, isForm = false) {
  const res = await fetch(BASE + path, {
    method,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(body && !isForm ? { 'Content-Type': 'application/json' } : {}),
    },
    body: body ? (isForm ? body : JSON.stringify(body)) : undefined,
  })
  const text = await res.text()
  const data = text ? JSON.parse(text) : null
  if (!res.ok) throw new Error(`${method} ${path} → ${res.status}: ${text.slice(0, 200)}`)
  return data
}

async function login({ email, password }) {
  return call(null, 'POST', '/api/auth/login', { email, password })
}

async function ensureStudent() {
  try {
    return await login(STUDENT)
  } catch {
    return call(null, 'POST', '/api/auth/register', STUDENT)
  }
}

async function main() {
  const teacher = await login(TEACHER)
  const teacherToken = teacher.token
  console.log(`teacher: ${TEACHER.email} (role ${teacher.user.role})`)
  const student = await ensureStudent()
  const studentToken = student.token
  console.log(`student:  ${STUDENT.email}`)

  // ---- 1. chart image through the real upload pipeline ----
  const existing = await call(teacherToken, 'GET', '/api/admin/stimuli')
  let chartUrl = null
  for (const stimulus of existing) {
    const doc = (stimulus.documents ?? [])[0]
    const match = doc?.text?.match(/!\[[^\]]*\]\((\/files\/qimg-[^)]+)\)/)
    if (match) chartUrl = match[1]
  }
  if (!chartUrl) {
    const bytes = await readFile(new URL('./demo-chart.png', import.meta.url))
    const form = new FormData()
    form.append('file', new Blob([bytes], { type: 'image/png' }), 'demo-chart.png')
    const uploaded = await call(teacherToken, 'POST', '/api/admin/question-images', form, true)
    chartUrl = uploaded.url
  }
  console.log(`chart:   ${chartUrl}`)

  // ---- 2. shared stimuli (find-or-create by title) ----
  async function ensureStimulus(spec) {
    const found = (await call(teacherToken, 'GET', `/api/admin/stimuli?search=${encodeURIComponent(spec.title)}`)).find(
      (row) => row.title === spec.title,
    )
    if (found) return found.id
    return (await call(teacherToken, 'POST', '/api/admin/stimuli', spec)).id
  }

  const passageId = await ensureStimulus({
    title: 'Caffeine and Word Recall (passage set)',
    kind: 'passage',
    documents: [
      {
        title: 'Study summary',
        text: `Sixty volunteers were randomly assigned to one of three groups (n = 20 each): 0 mg, 200 mg, or 400 mg of caffeine. After a 30-minute absorption period, every participant listened to the same 20-word list and then solved puzzles for 10 minutes as a distraction. The dependent variable was the number of words correctly recalled after the delay.

![Mean recall by dose group](${chartUrl})

The 200 mg group recalled the most words on average, while the 400 mg group did no better than the placebo group.`,
      },
    ],
  })
  const articleId = await ensureStimulus({
    title: 'Screen Time and Adolescent Mood (AAQ)',
    kind: 'article',
    documents: [
      {
        title: 'Summarized study',
        text: `A research team recruited 1,200 adolescents aged 14–16 through school registries in one large city. For two weeks, a diary app on each participant's phone recorded daily screen time, operationally defined as the hours per day of self-reported smartphone use. Mood was measured with the PHQ-9 mood questionnaire at the start of the study and again after the two weeks.

Across participants, more daily screen time was associated with slightly lower mood scores, a correlation of r = −0.21. The researchers noted that the association was small, that a few participants with very high screen time were not representative, and that all data were collected anonymously with parental consent on file.`,
      },
    ],
  })
  const sourcesId = await ensureStimulus({
    title: 'Social Media and Adolescent Well-Being (EBQ sources)',
    kind: 'sources',
    documents: [
      {
        title: 'Source 1',
        text: 'Source 1 (survey): In a cross-sectional survey of 1,500 adolescents, those reporting more than five hours of daily social media use rated their well-being noticeably lower than lighter users. Because all measures were taken at one time point, the direction of the relationship could not be established.',
      },
      {
        title: 'Source 2',
        text: 'Source 2 (experiment): Two hundred adolescents were randomly assigned to either give up social media apps for two weeks or use them as usual. The abstinence group reported a modest improvement in mood by the end of the study, while the control group showed no change.',
      },
      {
        title: 'Source 3',
        text: 'Source 3 (meta-analysis): A meta-analysis of 28 studies found the average association between social media use and adolescent well-being to be small, and noted that studies finding no effect were less likely to be published.',
      },
    ],
  })
  console.log(`stimuli: passage ${passageId} / article ${articleId} / sources ${sourcesId}`)

  // ---- 3. concept links (best effort; skipped when no match) ----
  async function conceptId(term) {
    const rows = await call(teacherToken, 'GET', `/api/concepts?search=${encodeURIComponent(term)}&limit=1`)
    return rows[0]?.id
  }
  const hindsight = await conceptId('hindsight bias')
  const correlation = await conceptId('correlation')
  const experiment = await conceptId('experimental group')

  // ---- 4. questions (find-or-create by stem) ----
  const bank = await call(teacherToken, 'GET', '/api/admin/questions?limit=1000')
  const byStem = new Map(bank.map((question) => [question.stem, question]))
  async function ensureQuestion(draft) {
    const found = byStem.get(draft.stem)
    if (found) return found.id
    const created = await call(teacherToken, 'POST', '/api/admin/questions', draft)
    await call(teacherToken, 'PATCH', `/api/admin/questions/${created.id}`, { status: 'published' })
    return created.id
  }

  const mcq1 = await ensureQuestion({
    type: 'mcq', stimulusId: passageId, unit: '2',
    stem: 'The design of the caffeine study is best described as:',
    choices: [
      { key: 'A', text: 'A cross-sectional survey of caffeine habits' },
      { key: 'B', text: 'A randomized experiment with a manipulated independent variable' },
      { key: 'C', text: 'A correlational study of dose and recall' },
      { key: 'D', text: 'A case study of a single dose group' },
    ],
    answerKey: 'B', explanation: 'Participants were randomly assigned to dose groups, so dose was manipulated — an experiment.',
    tags: ['demo', 'research-methods'], concepts: [experiment].filter(Boolean),
  })
  const mcq2 = await ensureQuestion({
    type: 'mcq', stimulusId: passageId, unit: '2',
    stem: 'According to the figure, which conclusion is best supported?',
    choices: [
      { key: 'A', text: 'Recall improves steadily as dose increases' },
      { key: 'B', text: 'The highest dose recalled no more words than the placebo group' },
      { key: 'C', text: 'Caffeine caused worse memory in every participant' },
      { key: 'D', text: 'The 0 mg group recalled the most words' },
    ],
    answerKey: 'B', explanation: 'The 400 mg mean (10.8) is close to the 0 mg mean (12.1) — no benefit at the highest dose.',
    tags: ['demo', 'data-analysis'],
  })
  const mcq3 = await ensureQuestion({
    type: 'mcq', stimulusId: passageId, unit: '2',
    stem: 'Which additional control would most improve internal validity?',
    choices: [
      { key: 'A', text: 'Letting participants choose their own dose group' },
      { key: 'B', text: 'Testing everyone at the same time of day and screening out heavy daily caffeine users' },
      { key: 'C', text: 'Using a longer word list for the 400 mg group' },
      { key: 'D', text: 'Telling participants their group assignment' },
    ],
    answerKey: 'B', explanation: 'Time of day and caffeine habit are confounds; holding them constant tightens the comparison.',
    tags: ['demo', 'confounds'],
  })
  const standalone = await ensureQuestion({
    type: 'mcq', unit: '1',
    stem: 'Which situation is the best example of hindsight bias?',
    choices: [
      { key: 'A', text: 'Assuming a stranger is friendly because they smiled' },
      { key: 'B', text: 'After exam results are posted, insisting the outcome was obvious all along' },
      { key: 'C', text: 'Overestimating how long an assignment will take' },
      { key: 'D', text: 'Blaming a poor grade on the teacher instead of the studying' },
    ],
    answerKey: 'B', explanation: 'Hindsight bias is the "I-knew-it-all-along" tendency once an outcome is known.',
    tags: ['demo', 'unit-1'], concepts: [hindsight].filter(Boolean),
  })
  const aaq = await ensureQuestion({
    type: 'subjective', format: 'aaq', stimulusId: articleId, unit: '2',
    stem: 'Use the source to answer the questions below in parts A–F.',
    parts: [
      { label: 'A', prompt: 'Identify the research method used in the study.', referenceAnswer: 'A correlational (survey/diary) study — screen time and mood were measured, not manipulated.', rubric: ['Names a correlational/survey method'], points: 1 },
      { label: 'B', prompt: 'Operationally define the screen-time variable.', referenceAnswer: 'Hours per day of self-reported smartphone use recorded by the diary app over two weeks.', rubric: ['Gives a measurable definition tied to the diary app'], points: 1 },
      { label: 'C', prompt: 'Interpret what a correlation of r = −0.21 means here.', referenceAnswer: 'A weak negative association: more daily screen time goes with slightly lower mood scores, but the relationship is small.', rubric: ['States negative direction', 'Notes the association is weak'], points: 1 },
      { label: 'D', prompt: 'Identify one ethical safeguard in the study.', referenceAnswer: 'Anonymous data collection with informed parental consent.', rubric: ['Names consent, anonymity, or confidentiality'], points: 1 },
      { label: 'E', prompt: 'Explain one limit on the generalizability of the findings.', referenceAnswer: 'The sample came from one city and one age band (14–16), so results may not extend to other ages or places.', rubric: ['Names a sampling limitation'], points: 1 },
      { label: 'F', prompt: 'A student claims "smartphones cause teen depression." Using the source, explain why this claim is not supported.', referenceAnswer: 'The study is correlational — it cannot establish direction or causation; low mood could drive more screen time, or a third variable could cause both.', rubric: ['States correlational design cannot show causation', 'Gives a plausible alternative explanation'], points: 2 },
    ],
    tags: ['demo', 'aaq'], concepts: [correlation].filter(Boolean),
  })
  const ebq = await ensureQuestion({
    type: 'subjective', format: 'ebq', stimulusId: sourcesId,
    stem: 'Should schools restrict social media use to improve adolescent well-being? Use the three sources to build your argument.',
    parts: [
      { label: 'A', prompt: 'State a defensible claim that responds to the question.', referenceAnswer: 'e.g. Moderate restrictions may help slightly, but the evidence overall shows only a small relationship, so restrictions alone are unlikely to be a strong fix.', rubric: ['Takes a clear position answerable from the sources'], points: 1 },
      { label: 'B', prompt: 'Cite one piece of specific evidence from Source 1 or Source 3.', referenceAnswer: 'e.g. Source 1: 5+ hour users rated well-being lower; Source 3: meta-analysis of 28 studies found only a small average association.', rubric: ['Cites the source correctly with a specific finding'], points: 1 },
      { label: 'C', prompt: 'Cite one piece of specific evidence from the other source.', referenceAnswer: 'e.g. Source 2: the two-week random-assignment abstinence group improved mood modestly while controls did not.', rubric: ['Cites the source correctly with a specific finding'], points: 1 },
      { label: 'D', prompt: 'Explain how your first piece of evidence supports your claim.', referenceAnswer: 'Ties the cited finding back to the claim, e.g. the survey cannot show direction, which is exactly why a blanket ban is not justified.', rubric: ['Links evidence to the claim with reasoning'], points: 1 },
      { label: 'E', prompt: 'Explain how your second piece of evidence supports your claim, applying a course concept.', referenceAnswer: 'e.g. Source 2 is an experiment with random assignment, so it suggests a causal benefit — applied to the claim, restrictions could produce small real gains.', rubric: ['Links evidence to the claim', 'Applies a course concept (e.g. random assignment, causation)'], points: 1 },
    ],
    tags: ['demo', 'ebq'],
  })
  console.log('questions: passage×3, standalone mcq, aaq, ebq ready')

  // ---- 5. sets (find-or-create by title) ----
  const sets = await call(teacherToken, 'GET', '/api/admin/sets')
  async function ensureSet(spec) {
    const found = sets.find((row) => row.title === spec.title)
    if (found) return found.id
    return (await call(teacherToken, 'POST', '/api/admin/sets', { ...spec, status: 'published' })).id
  }
  const setC = await ensureSet({
    title: 'C — 题组 · AAQ · EBQ（即时反馈）',
    description: 'Demo of the practice upgrade: MCQ set with a chart passage, an AAQ article, per-part answering.',
    mode: 'instant',
    questionIds: [mcq1, mcq2, mcq3, standalone, aaq],
  })
  const setD = await ensureSet({
    title: 'D — 全真模考 Mock Exam（25 分钟）',
    description: 'Exam-mode demo: passage set + standalone MCQ + AAQ + EBQ, feedback withheld until submit.',
    mode: 'exam',
    timeLimitSec: 1500,
    questionIds: [mcq1, mcq2, mcq3, standalone, aaq, ebq],
  })
  console.log(`sets:    ${setC} (instant) / ${setD} (exam)`)

  // ---- 6. student attempt history ----
  const attempts = await call(studentToken, 'GET', '/api/practice/attempts?limit=50')
  const doneC = attempts.some((row) => row.setId === setC && row.finishedAt)
  if (!doneC) {
    const started = await call(studentToken, 'POST', '/api/practice/attempts', { setId: setC })
    const attemptId = started.attempt.id
    const detail = await call(studentToken, 'GET', `/api/practice/attempts/${attemptId}`)
    const ordered = detail.questions ?? []
    const answer = (questionId, body) =>
      call(studentToken, 'POST', `/api/practice/attempts/${attemptId}/answers`, { questionId, ...body })
    for (const question of ordered) {
      if (question.type === 'mcq') {
        // The runner never sees keys; the seed looks them up as staff so the
        // demo attempt lands with a realistic correct/wrong mix.
        const admin = await call(teacherToken, 'GET', `/api/admin/questions/${question.id}`)
        const wrong = question.id === mcq2
        const key = wrong ? (admin.answerKey === 'A' ? 'B' : 'A') : admin.answerKey
        await answer(question.id, { choiceKey: key })
      } else if (question.format === 'aaq') {
        await answer(question.id, {
          parts: [
            { label: 'A', text: 'It is a correlational diary study.' },
            { label: 'B', text: 'Hours per day of phone use recorded by the diary app.' },
            { label: 'C', text: 'A weak negative relationship: more screen time, slightly lower mood.' },
            { label: 'D', text: 'Anonymous data and parental consent.' },
            { label: 'E', text: 'Only adolescents from one city, ages 14–16.' },
            { label: 'F', text: 'Correlation cannot show causation; low mood could drive screen time instead.' },
          ],
        })
      }
    }
    await call(studentToken, 'POST', `/api/practice/attempts/${attemptId}/answers/${aaq}/self-rating`, {
      parts: [
        { label: 'A', rating: 'proficient' }, { label: 'B', rating: 'proficient' }, { label: 'C', rating: 'proficient' },
        { label: 'D', rating: 'proficient' }, { label: 'E', rating: 'partial' }, { label: 'F', rating: 'weak' },
      ],
    })
    await call(studentToken, 'POST', `/api/practice/attempts/${attemptId}/finish`, {})
    console.log('student: finished instant set C (mixed answers, per-part ratings)')
  }
  const openD = attempts.find((row) => row.setId === setD && !row.finishedAt)
  if (!openD) {
    const started = await call(studentToken, 'POST', '/api/practice/attempts', { setId: setD })
    const attemptId = started.attempt.id
    await call(studentToken, 'POST', `/api/practice/attempts/${attemptId}/answers`, { questionId: mcq1, choiceKey: 'B' })
    await call(studentToken, 'POST', `/api/practice/attempts/${attemptId}/answers`, {
      questionId: aaq,
      parts: [
        { label: 'A', text: 'Correlational diary study.' },
        { label: 'B', text: 'Hours per day on the diary app.' },
      ],
    })
    console.log('student: exam set D left in progress (resumable)')
  } else {
    console.log('student: exam set D already in progress')
  }

  console.log('\nDone. Log in with:')
  console.log(`  teacher/admin: ${TEACHER.email} / ${TEACHER.password}`)
  console.log(`  student:       ${STUDENT.email} / ${STUDENT.password}`)
  console.log(`  instant set:   ${BASE.replace(':8080', ':5173')}/practice`)
}

main().catch((err) => {
  console.error(err.message)
  process.exit(1)
})
