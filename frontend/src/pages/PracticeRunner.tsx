import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Check, CheckCircle2, Clock3, XCircle } from 'lucide-react'
import { api } from '../lib/api'
import { formatClock } from '../lib/format'
import { useSession } from '../hooks/session'
import { Header } from '../components/ui'
import { InlineMarkdown, MarkdownText } from '../components/InlineMarkdown'
import { SplitPane } from '../components/SplitPane'
import { AutoGrowTextarea } from '../components/AutoGrowTextarea'
import { buildRenderItems } from '../lib/questionGrouping'
import { AttemptReview, SelfRatingRow } from '../components/AttemptReview'
import type { PracticeAnswer, PracticeAttempt, PracticeSet, RunnerQuestion, Stimulus } from '../lib/types'

type Stage = 'loading' | 'running' | 'finished' | 'error'

export function PracticeRunner() {
  const { setId = '' } = useParams()
  const { t } = useSession()
  const queryClient = useQueryClient()
  const [stage, setStage] = useState<Stage>('loading')
  const [attempt, setAttempt] = useState<PracticeAttempt | null>(null)
  const [questions, setQuestions] = useState<RunnerQuestion[]>([])
  const [answers, setAnswers] = useState<Record<string, PracticeAnswer>>({})
  const [stimuli, setStimuli] = useState<Record<string, Stimulus>>({})
  const [renderIndex, setRenderIndex] = useState(0)
  const [clusterInner, setClusterInner] = useState(0)
  const [revealed, setRevealed] = useState<Record<string, boolean>>({})
  const [busy, setBusy] = useState(false)
  const [now, setNow] = useState(Date.now())
  const [showConfirm, setShowConfirm] = useState(false)
  const [error, setError] = useState('')
  // Per-part drafts autosave on a debounce; finish() flushes them first so
  // nothing typed in the last second is lost.
  const flushDraftsRef = useRef<(() => Promise<void>) | null>(null)

  const { data: set } = useQuery({
    queryKey: ['practice-set', setId],
    queryFn: () => api.request<{ set: PracticeSet; questions: RunnerQuestion[]; attempts: PracticeAttempt[]; stimuli?: Stimulus[] }>(`/api/practice/sets/${setId}`),
    enabled: !!setId,
  })

  useEffect(() => {
    if (stage !== 'running') return
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [stage])

  // Boot: create (or resume) an attempt, then load its full state.
  useEffect(() => {
    if (!setId) return
    let cancelled = false
    async function boot() {
      try {
        const started = await api.request<{ attempt: PracticeAttempt }>('/api/practice/attempts', {
          method: 'POST',
          body: JSON.stringify({ setId }),
        })
        if (cancelled) return
        await loadAttempt(started.attempt)
      } catch {
        if (!cancelled) setStage('error')
      }
    }
    void boot()
    async function loadAttempt(attemptRow: PracticeAttempt) {
      const detail = await api.request<{
        attempt: PracticeAttempt
        questions: RunnerQuestion[]
        answers: PracticeAnswer[]
        stimuli?: Stimulus[]
      }>(`/api/practice/attempts/${attemptRow.id}`)
      if (cancelled) return
      setAttempt(detail.attempt)
      setQuestions(detail.questions ?? [])
      const answerMap: Record<string, PracticeAnswer> = {}
      for (const answer of detail.answers ?? []) answerMap[answer.questionId] = answer
      setAnswers(answerMap)
      setStimuli((current) => mergeStimuli(current, detail.stimuli))
      if (detail.attempt.finishedAt) setStage('finished')
      else setStage('running')
    }
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [setId])

  const examMode = attempt?.mode === 'exam'
  const finished = stage === 'finished'

  // The paper folds into render units: standalone questions, MCQ clusters
  // sharing a passage, and AAQ/EBQ composites next to their stimulus.
  const renderItems = useMemo(() => buildRenderItems(questions), [questions])

  // Per-question navigation: each pill jumps to its render item (and the
  // right question inside an MCQ cluster).
  const navMap = useMemo(() => {
    const map: { itemIndex: number; innerIndex: number }[] = []
    renderItems.forEach((item, itemIndex) => {
      if (item.kind === 'mcq-set') {
        item.questions.forEach((_, innerIndex) => map.push({ itemIndex, innerIndex }))
      } else {
        map.push({ itemIndex, innerIndex: 0 })
      }
    })
    return map
  }, [renderItems])

  const item = renderItems[renderIndex]
  const current =
    item?.kind === 'single' || item?.kind === 'aaq' || item?.kind === 'ebq'
      ? item.question
      : item?.kind === 'mcq-set'
        ? item.questions[clusterInner]
        : undefined
  const currentNumber = current ? questions.findIndex((question) => question.id === current.id) + 1 : 0

  const secondsLeft = useMemo(() => {
    if (!attempt?.deadlineAt) return null
    return Math.floor((new Date(attempt.deadlineAt).getTime() - now) / 1000)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [attempt, now])

  const finish = useCallback(async () => {
    if (!attempt || busy) return
    setBusy(true)
    setError('')
    try {
      await flushDraftsRef.current?.()
      const summary = await api.request<{
        attempt: PracticeAttempt
        questions: RunnerQuestion[]
        answers: PracticeAnswer[]
        stimuli?: Stimulus[]
        correct: number
      }>(`/api/practice/attempts/${attempt.id}/finish`, { method: 'POST', body: '{}' })
      setAttempt(summary.attempt)
      setQuestions(summary.questions ?? [])
      setAnswers(Object.fromEntries((summary.answers ?? []).map((answer) => [answer.questionId, answer])))
      setStimuli((currentStimuli) => mergeStimuli(currentStimuli, summary.stimuli))
      setStage('finished')
      void queryClient.invalidateQueries({ queryKey: ['practice-sets'] })
      void queryClient.invalidateQueries({ queryKey: ['practice-set', setId] })
      // Fresh wrong answers only show up once the derived wrong book refetches.
      void queryClient.invalidateQueries({ queryKey: ['wrongbook'] })
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }, [attempt, busy, queryClient, setId, t])

  // Auto-submit when the server deadline passes.
  useEffect(() => {
    if (stage === 'running' && secondsLeft !== null && secondsLeft <= 0) void finish()
  }, [stage, secondsLeft, finish])

  function applyAnswerPayload(questionId: string, payload: { answer: PracticeAnswer; question?: RunnerQuestion }) {
    setAnswers((current) => ({ ...current, [questionId]: payload.answer }))
    if (payload.question) {
      setQuestions((rows) => rows.map((row) => (row.id === questionId ? payload.question! : row)))
      setRevealed((current) => ({ ...current, [questionId]: true }))
    }
  }

  async function submitMCQ(question: RunnerQuestion, choiceKey: string) {
    if (!attempt || busy) return
    setBusy(true)
    setError('')
    try {
      const payload = await api.request<{ answer: PracticeAnswer; question?: RunnerQuestion }>(
        `/api/practice/attempts/${attempt.id}/answers`,
        { method: 'POST', body: JSON.stringify({ questionId: question.id, choiceKey }) },
      )
      applyAnswerPayload(question.id, payload)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  async function submitSubjective(question: RunnerQuestion, textAnswer: string) {
    if (!attempt || busy) return
    setBusy(true)
    setError('')
    try {
      const payload = await api.request<{ answer: PracticeAnswer; question?: RunnerQuestion }>(
        `/api/practice/attempts/${attempt.id}/answers`,
        { method: 'POST', body: JSON.stringify({ questionId: question.id, textAnswer }) },
      )
      applyAnswerPayload(question.id, payload)
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  const selfRate = useCallback(
    async (question: RunnerQuestion, rating: 'proficient' | 'partial' | 'weak') => {
      if (!attempt) return
      try {
        const answer = await api.request<PracticeAnswer>(`/api/practice/attempts/${attempt.id}/answers/${question.id}/self-rating`, {
          method: 'POST',
          body: JSON.stringify({ rating }),
        })
        setAnswers((current) => ({ ...current, [question.id]: answer }))
      } catch (err) {
        setError(err instanceof Error ? err.message : t.errorGeneric)
      }
    },
    [attempt, t],
  )

  const selfRateParts = useCallback(
    async (question: RunnerQuestion, partRatings: Record<string, 'proficient' | 'partial' | 'weak'>) => {
      if (!attempt) return
      try {
        const answer = await api.request<PracticeAnswer>(`/api/practice/attempts/${attempt.id}/answers/${question.id}/self-rating`, {
          method: 'POST',
          body: JSON.stringify({
            parts: (question.parts ?? []).map((part) => ({ label: part.label, rating: partRatings[part.label] ?? 'partial' })),
          }),
        })
        setAnswers((current) => ({ ...current, [question.id]: answer }))
      } catch (err) {
        setError(err instanceof Error ? err.message : t.errorGeneric)
      }
    },
    [attempt, t],
  )

  if (stage === 'loading') {
    return (
      <section className="page">
        <p className="muted">{t.loading}…</p>
      </section>
    )
  }

  if (stage === 'error') {
    return (
      <section className="page">
        <Header eyebrow={t.practice} title={t.errorGeneric} />
        <div className="error">{t.errorGeneric}</div>
        <div className="action-row" style={{ marginTop: 12 }}>
          <button className="primary" onClick={() => window.location.reload()}>
            {t.start}
          </button>
          <Link className="secondary" to="/practice">
            {t.back}
          </Link>
        </div>
      </section>
    )
  }

  if (finished) {
    return (
      <section className="page">
        <Header
          eyebrow={set?.set.title ?? ''}
          title={t.results}
          action={
            <Link className="secondary" to="/practice">
              <ArrowLeft size={16} /> {t.practice}
            </Link>
          }
        />
        <AttemptReview
          attempt={attempt!}
          questions={questions}
          answers={answers}
          timeUp={!!attempt?.deadlineAt && secondsLeft !== null && secondsLeft <= 0}
          selfRate={(question, rating) => void selfRate(question, rating)}
          selfRateParts={(question, ratings) => void selfRateParts(question, ratings)}
        />
      </section>
    )
  }

  // ---- running ----

  function goNext() {
    if (item?.kind === 'mcq-set' && clusterInner < item.questions.length - 1) {
      setClusterInner(clusterInner + 1)
      return
    }
    if (renderIndex < renderItems.length - 1) {
      setRenderIndex(renderIndex + 1)
      setClusterInner(0)
    }
  }

  function goPrev() {
    if (item?.kind === 'mcq-set' && clusterInner > 0) {
      setClusterInner(clusterInner - 1)
      return
    }
    if (renderIndex > 0) {
      const prev = renderItems[renderIndex - 1]
      setRenderIndex(renderIndex - 1)
      setClusterInner(prev.kind === 'mcq-set' ? prev.questions.length - 1 : 0)
    }
  }

  const answeredCheck = (answer?: PracticeAnswer) =>
    !!(answer && (answer.choiceKey || answer.textAnswer || answer.parts?.some((part) => part.text)))

  let body: React.ReactNode = null
  if (item?.kind === 'single') {
    body = <SingleQuestionCard question={item.question} answers={answers} revealed={revealed} examMode={examMode} busy={busy} onSubmitMCQ={submitMCQ} onSubmitSubjective={submitSubjective} selfRate={selfRate} />
  } else if (item?.kind === 'mcq-set') {
    const stimulus = stimuli[item.stimulusId]
    const groupQuestion = item.questions[clusterInner]
    body = (
      <SplitPane
        leftLabel={t.stimulus}
        rightLabel={t.questions}
        left={<StimulusPane stimulus={stimulus} />}
        right={
          <div className="cluster-pane">
            <div className="cluster-progress">
              {t.formatSet} · {clusterInner + 1} / {item.questions.length}
            </div>
            <SingleQuestionCard
              question={groupQuestion}
              answers={answers}
              revealed={revealed}
              examMode={examMode}
              busy={busy}
              onSubmitMCQ={submitMCQ}
              onSubmitSubjective={submitSubjective}
              selfRate={selfRate}
            />
          </div>
        }
      />
    )
  } else if (item?.kind === 'aaq' || item?.kind === 'ebq') {
    const stimulus = stimuli[item.stimulusId]
    body = (
      <SplitPane
        leftLabel={t.stimulus}
        rightLabel={t.questions}
        left={<StimulusPane stimulus={stimulus} />}
        right={
          <div className="cluster-pane">
            <div className="cluster-progress">{item.kind === 'aaq' ? t.formatAaq : t.formatEbg}</div>
            <PartAnswerForm
              key={item.question.id}
              question={item.question}
              answer={answers[item.question.id]}
              attemptId={attempt?.id ?? ''}
              onAnswered={(payload) => applyAnswerPayload(item.question.id, payload)}
              registerFlush={(flush) => {
                flushDraftsRef.current = flush
              }}
            />
          </div>
        }
      />
    )
  }

  return (
    <section className={`page runner-page ${item && item.kind !== 'single' ? 'split-runner' : ''}`}>
      <Header
        eyebrow={set?.set.title ?? ''}
        title={current ? `${currentNumber} / ${questions.length}` : t.loading}
        action={
          <div className="action-row">
            {secondsLeft !== null && (
              <span className={`timer ${secondsLeft < 60 ? 'urgent' : ''}`}>
                <Clock3 size={16} /> {formatClock(secondsLeft)}
              </span>
            )}
            {examMode ? (
              <button className="primary" onClick={() => setShowConfirm(true)} disabled={busy}>
                {t.finishExam}
              </button>
            ) : (
              <button className="secondary" onClick={() => void finish()} disabled={busy}>
                {t.finish}
              </button>
            )}
          </div>
        }
      />
      <div className="question-nav">
        {questions.map((question, i) => {
          const target = navMap[i]
          const grouped = !!question.stimulusId
          return (
            <button
              key={question.id}
              className={`${target && target.itemIndex === renderIndex ? 'current' : ''} ${answeredCheck(answers[question.id]) ? 'answered' : ''} ${grouped ? 'grouped' : ''}`}
              title={grouped ? t.formatSet : undefined}
              onClick={() => {
                if (!target) return
                setRenderIndex(target.itemIndex)
                setClusterInner(target.innerIndex)
              }}
            >
              {i + 1}
            </button>
          )
        })}
      </div>
      {body}
      {error && <div className="error">{error}</div>}
      <div className="runner-footer">
        <button className="secondary" disabled={renderIndex === 0 && clusterInner === 0} onClick={goPrev}>
          {t.previous}
        </button>
        {renderIndex < renderItems.length - 1 || (item?.kind === 'mcq-set' && clusterInner < item.questions.length - 1) ? (
          <button className="primary" onClick={goNext}>
            {t.next}
          </button>
        ) : (
          <button className="primary" onClick={() => (examMode ? setShowConfirm(true) : void finish())} disabled={busy}>
            {examMode ? t.finishExam : t.finish}
          </button>
        )}
      </div>
      {showConfirm && (
        <div className="modal-backdrop" onClick={() => setShowConfirm(false)}>
          <div className="modal" onClick={(event) => event.stopPropagation()}>
            <p>{t.confirmFinish}</p>
            <div className="action-row">
              <button className="secondary" onClick={() => setShowConfirm(false)}>
                {t.cancel}
              </button>
              <button
                className="primary"
                onClick={() => {
                  setShowConfirm(false)
                  void finish()
                }}
              >
                {t.confirm}
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  )
}

function mergeStimuli(current: Record<string, Stimulus>, incoming?: Stimulus[]): Record<string, Stimulus> {
  if (!incoming?.length) return current
  const next = { ...current }
  for (const stimulus of incoming) next[stimulus.id] = stimulus
  return next
}

// One standalone question (or one question of an MCQ cluster): the classic
// single-column card, unchanged behavior for MCQ and FRQ subjective.
function SingleQuestionCard({
  question,
  answers,
  revealed,
  examMode,
  busy,
  onSubmitMCQ,
  onSubmitSubjective,
  selfRate,
}: {
  question: RunnerQuestion
  answers: Record<string, PracticeAnswer>
  revealed: Record<string, boolean>
  examMode: boolean
  busy: boolean
  onSubmitMCQ: (question: RunnerQuestion, choiceKey: string) => void
  onSubmitSubjective: (question: RunnerQuestion, text: string) => void
  selfRate: (question: RunnerQuestion, rating: 'proficient' | 'partial' | 'weak') => void
}) {
  const { t } = useSession()
  const showFeedback = !examMode && question.type === 'mcq' && answers[question.id]
  const showSubjectiveForm = question.type === 'subjective' && (!revealed[question.id] || !answers[question.id]?.textAnswer)
  const showSubjectiveReview =
    question.type === 'subjective' && answers[question.id]?.textAnswer && (revealed[question.id] || !examMode)
  return (
    <div className="question-card">
      {(question.materials ?? []).map((material, i) => (
        <div className="material-block" key={i}>
          {material.title && <strong>{material.title}</strong>}
          <MarkdownText text={material.text} />
        </div>
      ))}
      {question.type === 'subjective' && question.format !== 'aaq' && question.format !== 'ebq' && (question.parts ?? []).length > 0 && (
        <div className="material-block">
          {(question.parts ?? []).map((part) => (
            <p key={part.label}>
              <strong>({part.label})</strong> {part.prompt}
            </p>
          ))}
        </div>
      )}
      <h2>
        <InlineMarkdown text={question.stem} />
      </h2>
      {question.type === 'mcq' ? (
        <div className="choice-list">
          {(question.choices ?? []).map((choice) => {
            const picked = answers[question.id]?.choiceKey === choice.key
            const isKey = showFeedback && question.answerKey === choice.key
            return (
              <button
                key={choice.key}
                className={`choice ${picked ? 'picked' : ''} ${isKey ? 'correct' : ''} ${
                  showFeedback && picked && !isKey ? 'wrong' : ''
                }`}
                disabled={!!showFeedback || busy}
                onClick={() => onSubmitMCQ(question, choice.key)}
              >
                <span className="choice-key">{choice.key}</span>
                <span>{choice.text}</span>
                {isKey && <CheckCircle2 size={18} />}
                {showFeedback && picked && !isKey && <XCircle size={18} />}
              </button>
            )
          })}
        </div>
      ) : showSubjectiveForm ? (
        <SubjectiveForm key={question.id} busy={busy} onSubmit={(text) => onSubmitSubjective(question, text)} />
      ) : null}

      {showFeedback && (
        <div className={`feedback ${answers[question.id]?.isCorrect ? 'ok' : 'bad'}`}>
          <strong>{answers[question.id]?.isCorrect ? t.correct : t.incorrect}</strong>
          {question.explanation && <p>{question.explanation}</p>}
        </div>
      )}
      {question.type === 'subjective' && examMode && answers[question.id]?.textAnswer && (
        <div className="feedback neutral">
          <p className="muted">{answers[question.id]?.textAnswer}</p>
        </div>
      )}
      {showSubjectiveReview && (
        <div className="feedback neutral">
          <strong>{t.referenceAnswer}</strong>
          {(question.parts ?? []).map((part) => (
            <div key={part.label}>
              <p>{part.referenceAnswer}</p>
              {part.rubric && part.rubric.length > 0 && (
                <ul className="rubric-list">
                  {part.rubric.map((point) => (
                    <li key={point}>{point}</li>
                  ))}
                </ul>
              )}
            </div>
          ))}
          {!examMode && <SelfRatingRow current={answers[question.id]?.selfRating} onRate={(rating) => selfRate(question, rating)} />}
        </div>
      )}
    </div>
  )
}

// Reading pane shared by MCQ clusters (passage), AAQ (single article), and
// EBQ (tabbed sources). Documents may embed uploaded images via markdown.
function StimulusPane({ stimulus }: { stimulus?: Stimulus }) {
  const { t } = useSession()
  const [activeDoc, setActiveDoc] = useState(0)
  if (!stimulus) return <div className="stimulus-pane"><p className="muted">{t.stimulusMissing}</p></div>
  const docs = stimulus.documents ?? []
  return (
    <div className="stimulus-pane">
      <div className="stimulus-head">
        <strong>{stimulus.title}</strong>
      </div>
      {docs.length > 1 && (
        <div className="source-tabs" role="tablist">
          {docs.map((_, i) => (
            <button
              key={i}
              role="tab"
              aria-selected={i === activeDoc}
              className={i === activeDoc ? 'active' : ''}
              onClick={() => setActiveDoc(i)}
            >
              {t.source} {i + 1}
            </button>
          ))}
        </div>
      )}
      <div className="stimulus-doc">
        {docs[activeDoc]?.title && <strong>{docs[activeDoc].title}</strong>}
        <MarkdownText text={docs[activeDoc]?.text ?? ''} />
      </div>
    </div>
  )
}

type SaveState = 'idle' | 'saving' | 'saved' | 'error'

// AAQ/EBQ composite: every part gets an auto-growing textarea; drafts
// autosave (debounced) as one per-part answer, and the reveal shows the
// reference answer + rubric + per-part self-rating under each part.
function PartAnswerForm({
  question,
  answer,
  attemptId,
  onAnswered,
  registerFlush,
}: {
  question: RunnerQuestion
  answer?: PracticeAnswer
  attemptId: string
  onAnswered: (payload: { answer: PracticeAnswer; question?: RunnerQuestion }) => void
  registerFlush: (flush: () => Promise<void>) => void
}) {
  const { t } = useSession()
  const parts = question.parts ?? []
  const [drafts, setDrafts] = useState<Record<string, string>>(() =>
    Object.fromEntries(parts.map((part) => [part.label, answer?.parts?.find((row) => row.label === part.label)?.text ?? ''])),
  )
  const [partRatings, setPartRatings] = useState<Record<string, 'proficient' | 'partial' | 'weak'>>(() =>
    Object.fromEntries((answer?.partRatings ?? []).map((row) => [row.label, row.rating])),
  )
  const [saveState, setSaveState] = useState<SaveState>('idle')
  const timerRef = useRef<number | null>(null)
  const draftsRef = useRef(drafts)
  draftsRef.current = drafts

  // Reference answers appear in question.parts only once revealed.
  const revealed = parts.some((part) => part.referenceAnswer != null)

  const save = useCallback(async () => {
    const current = draftsRef.current
    if (!parts.some((part) => (current[part.label] ?? '').trim())) return
    setSaveState('saving')
    try {
      const payload = await api.request<{ answer: PracticeAnswer; question?: RunnerQuestion }>(
        `/api/practice/attempts/${attemptId}/answers`,
        {
          method: 'POST',
          body: JSON.stringify({
            questionId: question.id,
            parts: parts.map((part) => ({ label: part.label, text: current[part.label] ?? '' })),
          }),
        },
      )
      onAnswered(payload)
      setSaveState('saved')
    } catch {
      setSaveState('error')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [attemptId, question.id])

  useEffect(() => {
    registerFlush(save)
    return () => {
      if (timerRef.current) window.clearTimeout(timerRef.current)
    }
  }, [registerFlush, save])

  function scheduleSave() {
    if (timerRef.current) window.clearTimeout(timerRef.current)
    timerRef.current = window.setTimeout(() => void save(), 900)
  }

  async function ratePart(label: string, rating: 'proficient' | 'partial' | 'weak') {
    const next = { ...partRatings, [label]: rating }
    setPartRatings(next)
    try {
      const updated = await api.request<PracticeAnswer>(`/api/practice/attempts/${attemptId}/answers/${question.id}/self-rating`, {
        method: 'POST',
        body: JSON.stringify({ parts: parts.map((part) => ({ label: part.label, rating: next[part.label] ?? 'partial' })) }),
      })
      onAnswered({ answer: updated })
    } catch {
      // Rating failures leave the local state as-is; the pills stay clickable.
    }
  }

  return (
    <div className="part-form">
      <h2>
        <InlineMarkdown text={question.stem} />
      </h2>
      {(question.materials ?? []).map((material, i) => (
        <div className="material-block" key={i}>
          {material.title && <strong>{material.title}</strong>}
          <MarkdownText text={material.text} />
        </div>
      ))}
      {parts.map((part) => (
        <div className="part-row" key={part.label}>
          <div className="part-label">
            <strong>({part.label})</strong>
            {part.points != null && <em> · {part.points} {t.points}</em>}
            <span>
              <InlineMarkdown text={part.prompt} />
            </span>
          </div>
          <AutoGrowTextarea
            ariaLabel={`${part.label}`}
            minRows={3}
            value={drafts[part.label] ?? ''}
            placeholder={t.typeAnswer}
            onChange={(value) => {
              setDrafts((current) => ({ ...current, [part.label]: value }))
              scheduleSave()
            }}
            onBlur={() => {
              if (timerRef.current) {
                window.clearTimeout(timerRef.current)
                timerRef.current = null
              }
              void save()
            }}
          />
          {revealed && part.referenceAnswer != null && (
            <div className="part-reference">
              <p>
                <strong>{t.reference}:</strong> {part.referenceAnswer}
              </p>
              {part.rubric && part.rubric.length > 0 && (
                <ul className="rubric-list">
                  {part.rubric.map((point) => (
                    <li key={point}>{point}</li>
                  ))}
                </ul>
              )}
              <div className="self-rating part-rating">
                {(
                  [
                    ['proficient', t.selfProficient],
                    ['partial', t.selfPartial],
                    ['weak', t.selfWeak],
                  ] as const
                ).map(([value, label]) => (
                  <button
                    key={value}
                    className={partRatings[part.label] === value ? 'active' : ''}
                    onClick={() => void ratePart(part.label, value)}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      ))}
      <div className="part-save-state">
        {saveState === 'saving' && <span className="muted">{t.saving}</span>}
        {saveState === 'saved' && (
          <span className="muted">
            <Check size={13} /> {t.saved}
          </span>
        )}
        {saveState === 'error' && (
          <button className="secondary" onClick={() => void save()}>
            {t.saveRetry}
          </button>
        )}
      </div>
    </div>
  )
}

function SubjectiveForm({ busy, onSubmit }: { busy: boolean; onSubmit: (text: string) => void }) {
  const { t } = useSession()
  const [text, setText] = useState('')
  return (
    <div className="subjective-form">
      <label>
        {t.typeAnswer}
        <AutoGrowTextarea ariaLabel={t.typeAnswer} minRows={6} value={text} onChange={setText} />
      </label>
      <button className="primary" disabled={busy || !text.trim()} onClick={() => onSubmit(text)}>
        {t.submitAnswer}
      </button>
    </div>
  )
}
