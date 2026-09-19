import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useBlocker, useNavigate, useParams } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { ChevronDown, ChevronUp, Clock3, Loader2, X } from 'lucide-react'
import { api } from '../lib/api'
import { formatClock } from '../lib/format'
import { useSession } from '../hooks/session'
import { praiseKey } from '../lib/i18n'
import type { PracticeAnswer, PracticeAttempt, RunnerQuestion } from '../lib/types'

type Stage = 'loading' | 'writing' | 'done' | 'error'

export function WritingRunner() {
  const { setId = '' } = useParams()
  const { t } = useSession()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [stage, setStage] = useState<Stage>('loading')
  const [attempt, setAttempt] = useState<PracticeAttempt | null>(null)
  const [questions, setQuestions] = useState<RunnerQuestion[]>([])
  const [answers, setAnswers] = useState<Record<string, PracticeAnswer>>({})
  const [revealed, setRevealed] = useState<Record<string, boolean>>({})
  const [index, setIndex] = useState(0)
  const [now, setNow] = useState(Date.now())
  const [busy, setBusy] = useState(false)
  const [promptOpen, setPromptOpen] = useState(true)
  const [setTitle, setSetTitle] = useState('')
  const [error, setError] = useState('')
  const [dirty, setDirty] = useState(false)

  const current = questions[index]
  const examMode = attempt?.mode === 'exam'
  const writingInProgress = stage === 'writing' && dirty
  const blocker = useBlocker(writingInProgress)

  // An essay typed but not submitted is lost on navigation/close — guard it.
  useEffect(() => {
    if (!writingInProgress) return
    const handler = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', handler)
    return () => window.removeEventListener('beforeunload', handler)
  }, [writingInProgress])

  useEffect(() => {
    if (stage !== 'writing') return
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [stage])

  const secondsLeft = useMemo(() => {
    if (!attempt?.deadlineAt) return null
    return Math.floor((new Date(attempt.deadlineAt).getTime() - now) / 1000)
  }, [attempt, now])

  const finish = useCallback(async () => {
    if (!attempt || busy) return
    setBusy(true)
    setError('')
    try {
      await api.request(`/api/practice/attempts/${attempt.id}/finish`, { method: 'POST', body: '{}' })
      void queryClient.invalidateQueries({ queryKey: ['practice-sets'] })
      void queryClient.invalidateQueries({ queryKey: ['wrongbook'] })
      setDirty(false)
      setStage('done')
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }, [attempt, busy, queryClient, t])

  useEffect(() => {
    if (stage === 'writing' && secondsLeft !== null && secondsLeft <= 0) void finish()
  }, [stage, secondsLeft, finish])

  useEffect(() => {
    if (!setId) return
    let cancelled = false
    async function boot() {
      try {
        const started = await api.request<{ attempt: PracticeAttempt }>('/api/practice/attempts', {
          method: 'POST',
          body: JSON.stringify({ setId }),
        })
        const detail = await api.request<{ attempt: PracticeAttempt; questions: RunnerQuestion[]; answers: PracticeAnswer[] }>(
          `/api/practice/attempts/${started.attempt.id}`,
        )
        if (cancelled) return
        const subjective = (detail.questions ?? []).filter((question) => question.type === 'subjective')
        setAttempt(detail.attempt)
        setQuestions(subjective)
        setAnswers(Object.fromEntries((detail.answers ?? []).map((answer) => [answer.questionId, answer])))
        const setDetail = await api.request<{ set: { title: string } }>(`/api/practice/sets/${setId}`)
        if (cancelled) return
        setSetTitle(setDetail.set.title)
        setStage(detail.attempt.finishedAt ? 'done' : 'writing')
      } catch {
        if (!cancelled) setStage('error')
      }
    }
    void boot()
    return () => {
      cancelled = true
    }
  }, [setId])

  async function submit(question: RunnerQuestion, text: string) {
    if (!attempt || busy) return
    setBusy(true)
    setError('')
    try {
      const payload = await api.request<{ answer: PracticeAnswer; question?: RunnerQuestion }>(
        `/api/practice/attempts/${attempt.id}/answers`,
        { method: 'POST', body: JSON.stringify({ questionId: question.id, textAnswer: text }) },
      )
      localStorage.removeItem(draftKey(attempt.id, question.id))
      setDirty(false)
      setAnswers((currentAnswers) => ({ ...currentAnswers, [question.id]: payload.answer }))
      if (payload.question) {
        setQuestions((rows) => rows.map((row) => (row.id === question.id ? payload.question! : row)))
        setRevealed((currentRevealed) => ({ ...currentRevealed, [question.id]: true }))
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  async function selfRate(question: RunnerQuestion, rating: 'proficient' | 'partial' | 'weak') {
    if (!attempt) return
    const answer = await api.request<PracticeAnswer>(`/api/practice/attempts/${attempt.id}/answers/${question.id}/self-rating`, {
      method: 'POST',
      body: JSON.stringify({ rating }),
    })
    setAnswers((currentAnswers) => ({ ...currentAnswers, [question.id]: answer }))
  }

  if (stage === 'loading') {
    return (
      <div className="writing-shell">
        <div className="writing-paper">
          <p className="muted">{t.loading}…</p>
        </div>
      </div>
    )
  }

  if (stage === 'error') {
    return (
      <div className="writing-shell">
        <div className="writing-paper">
          <div className="empty-state">
            <strong>{t.errorGeneric}</strong>
            <div className="action-row">
              <button className="primary" onClick={() => window.location.reload()}>
                {t.start}
              </button>
              <button className="secondary" onClick={() => navigate('/practice/writing')}>
                {t.back}
              </button>
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (stage === 'done') {
    const subjectiveAnswers = questions.filter((question) => answers[question.id]?.textAnswer)
    return (
      <div className="writing-shell">
        <div className="writing-topbar">
          <span className="title">{setTitle}</span>
          <div className="tools">
            <button className="secondary" onClick={() => navigate('/practice/writing')}>
              <X size={16} /> {t.exitWriting}
            </button>
          </div>
        </div>
        <div className="writing-body">
          <div className="writing-paper">
            <div className="empty-state celebrate">
              <strong>{t.done}</strong>
              <span className="muted">{t[praiseKey()]}</span>
              <div className="action-row">
                <button className="primary" onClick={() => navigate('/practice/writing')}>
                  {t.writingTitle}
                </button>
                <button className="secondary" onClick={() => navigate('/practice')}>
                  {t.practiceSets}
                </button>
              </div>
            </div>
            {subjectiveAnswers.length > 0 && (
              <div className="feedback neutral" style={{ marginTop: 16 }}>
                <strong>{t.selfAssess}</strong>
                {subjectiveAnswers.map((question) => (
                  <div key={question.id} style={{ marginTop: 12 }}>
                    <p style={{ margin: 0 }}>{question.stem.slice(0, 120)}{question.stem.length > 120 ? '…' : ''}</p>
                    {(question.parts ?? []).map((part) => (
                      <p key={part.label} className="muted" style={{ margin: '4px 0' }}>
                        <strong>({part.label})</strong> {part.referenceAnswer}
                        {part.rubric && part.rubric.length > 0 ? ` · ${part.rubric.join(' · ')}` : ''}
                      </p>
                    ))}
                    <div className="self-rating">
                      {(
                        [
                          ['proficient', t.selfProficient],
                          ['partial', t.selfPartial],
                          ['weak', t.selfWeak],
                        ] as const
                      ).map(([value, label]) => (
                        <button
                          key={value}
                          className={answers[question.id]?.selfRating === value ? 'active' : ''}
                          onClick={() => void selfRate(question, value)}
                        >
                          {label}
                        </button>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    )
  }

  if (!current) {
    return (
      <div className="writing-shell">
        <div className="writing-topbar">
          <span className="title">{setTitle}</span>
          <div className="tools">
            <button className="secondary" onClick={() => navigate('/practice/writing')}>
              <X size={16} /> {t.exitWriting}
            </button>
          </div>
        </div>
        <div className="writing-body">
          <div className="writing-paper">
            <div className="empty-state">{t.writingEmpty}</div>
          </div>
        </div>
      </div>
    )
  }

  const answer = answers[current.id]
  const showReview = !examMode && revealed[current.id] && answer?.textAnswer

  return (
    <div className="writing-shell">
      <div className="writing-topbar">
        <div className="tools">
          <button className="secondary" onClick={() => navigate('/practice/writing')}>
            <X size={16} /> {t.exitWriting}
          </button>
          <span className="title">
            {setTitle}
            {questions.length > 1 ? ` · ${index + 1} / ${questions.length}` : ''}
          </span>
        </div>
        <div className="tools">
          {secondsLeft !== null && (
            <span className={`timer ${secondsLeft < 60 ? 'urgent' : ''}`}>
              <Clock3 size={15} /> {formatClock(secondsLeft)}
            </span>
          )}
          {questions.length > 1 && (
            <div className="question-nav" style={{ marginBottom: 0 }}>
              {questions.map((question, i) => (
                <button
                  key={question.id}
                  className={`${i === index ? 'current' : ''} ${answers[question.id]?.textAnswer ? 'answered' : ''}`}
                  onClick={() => setIndex(i)}
                >
                  {i + 1}
                </button>
              ))}
            </div>
          )}
          <button className="primary" onClick={() => void finish()} disabled={busy}>
            {busy ? <Loader2 className="spin" size={16} /> : null} {examMode ? t.finishExam : t.finish}
          </button>
        </div>
      </div>
      {error && (
        <div className="writing-body">
          <div className="feedback danger" role="alert">
            {error}
          </div>
        </div>
      )}
      <div className="writing-body">
        <div className="writing-paper">
          <button className="link-button" onClick={() => setPromptOpen((value) => !value)}>
            {promptOpen ? <ChevronUp size={14} /> : <ChevronDown size={14} />}{' '}
            {promptOpen ? t.hidePrompt : t.showPrompt}
          </button>
          {promptOpen && (
            <div className="question-card">
              {(current.materials ?? []).map((material, i) => (
                <div className="material-block" key={i}>
                  {material.title && <strong>{material.title}</strong>}
                  <p>{material.text}</p>
                </div>
              ))}
              {(current.parts ?? []).length > 0 && (
                <div className="material-block">
                  {(current.parts ?? []).map((part) => (
                    <p key={part.label}>
                      <strong>({part.label})</strong> {part.prompt}
                    </p>
                  ))}
                </div>
              )}
              <h1>{current.stem}</h1>
            </div>
          )}
          {showReview ? (
            <div className="feedback neutral">
              <strong>{t.referenceAnswer}</strong>
              {(current.parts ?? []).map((part) => (
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
              <p className="muted">{t.selfAssess}</p>
              <div className="self-rating">
                {(
                  [
                    ['proficient', t.selfProficient],
                    ['partial', t.selfPartial],
                    ['weak', t.selfWeak],
                  ] as const
                ).map(([value, label]) => (
                  <button
                    key={value}
                    className={answer?.selfRating === value ? 'active' : ''}
                    onClick={() => void selfRate(current, value)}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            <WritingEditor
              key={current.id}
              initial={answer?.textAnswer ?? ''}
              busy={busy}
              locked={examMode && !!answer?.textAnswer}
              draftKey={draftKey(attempt?.id ?? '', current.id)}
              onDirtyChange={setDirty}
              onSubmit={(text) => void submit(current, text)}
            />
          )}
        </div>
      </div>
      {blocker.state === 'blocked' && (
        <div className="modal-backdrop">
          <div className="modal leave-modal" role="dialog" aria-modal="true" onClick={(event) => event.stopPropagation()}>
            <h3>{t.draftTitle}</h3>
            <p className="muted">{t.draftBody}</p>
            <div className="action-row">
              <button className="secondary" onClick={() => blocker.reset()}>
                {t.keepWriting}
              </button>
              <button className="danger" onClick={() => blocker.proceed()}>
                {t.leaveNow}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// Local drafts survive reloads and crashes; they are cleared on submit.
function draftKey(attemptId: string, questionId: string) {
  return `writing-draft:${attemptId}:${questionId}`
}

function WritingEditor({
  initial,
  busy,
  locked,
  draftKey: key,
  onDirtyChange,
  onSubmit,
}: {
  initial: string
  busy: boolean
  locked: boolean
  draftKey: string
  onDirtyChange: (dirty: boolean) => void
  onSubmit: (text: string) => void
}) {
  const { t } = useSession()
  const [text, setText] = useState(() => localStorage.getItem(key) ?? initial)
  const ref = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    ref.current?.focus()
  }, [])

  useEffect(() => {
    onDirtyChange(!locked && text !== initial && text.trim() !== '')
  }, [text, initial, locked, onDirtyChange])

  // Persist the draft on every change; drop it once the text matches the
  // saved answer (submission replaces it).
  useEffect(() => {
    if (locked || text === initial || text.trim() === '') {
      localStorage.removeItem(key)
    } else {
      localStorage.setItem(key, text)
    }
  }, [text, initial, locked, key])

  const words = useMemo(() => {
    const trimmed = text.trim()
    if (!trimmed) return 0
    const latinWords = trimmed.split(/\s+/).filter((part) => /[a-zA-Z]/.test(part)).length
    const cjk = (trimmed.match(/[一-鿿]/g) ?? []).length
    return latinWords + cjk
  }, [text])

  if (locked) {
    return (
      <div className="writing-editor">
        <p style={{ margin: 0, whiteSpace: 'pre-wrap', lineHeight: 1.7 }}>{text}</p>
        <div className="word-count">
          {words} {t.words}
        </div>
      </div>
    )
  }

  return (
    <div className="writing-editor">
      <textarea
        ref={ref}
        value={text}
        onChange={(event) => setText(event.target.value)}
        placeholder={t.typeAnswer}
        rows={12}
      />
      <div className="word-count">
        {words} {t.words}
      </div>
      <div className="action-row">
        <button className="primary" disabled={busy || !text.trim()} onClick={() => onSubmit(text)}>
          {busy ? <Loader2 className="spin" size={16} /> : null} {t.submitAnswer}
        </button>
      </div>
    </div>
  )
}
