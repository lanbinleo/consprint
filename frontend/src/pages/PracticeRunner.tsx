import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, CheckCircle2, Clock3, XCircle } from 'lucide-react'
import { api } from '../lib/api'
import { formatClock } from '../lib/format'
import { useSession } from '../hooks/session'
import { praiseKey } from '../lib/i18n'
import { Header } from '../components/ui'
import type { PracticeAnswer, PracticeAttempt, PracticeSet, RunnerQuestion } from '../lib/types'

type Stage = 'loading' | 'running' | 'finished' | 'error'

export function PracticeRunner() {
  const { setId = '' } = useParams()
  const { t } = useSession()
  const queryClient = useQueryClient()
  const [stage, setStage] = useState<Stage>('loading')
  const [attempt, setAttempt] = useState<PracticeAttempt | null>(null)
  const [questions, setQuestions] = useState<RunnerQuestion[]>([])
  const [answers, setAnswers] = useState<Record<string, PracticeAnswer>>({})
  const [index, setIndex] = useState(0)
  const [revealed, setRevealed] = useState<Record<string, boolean>>({})
  const [busy, setBusy] = useState(false)
  const [now, setNow] = useState(Date.now())
  const [showConfirm, setShowConfirm] = useState(false)
  const [error, setError] = useState('')

  const { data: set } = useQuery({
    queryKey: ['practice-set', setId],
    queryFn: () => api.request<{ set: PracticeSet; questions: RunnerQuestion[]; attempts: PracticeAttempt[] }>(`/api/practice/sets/${setId}`),
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
      const detail = await api.request<{ attempt: PracticeAttempt; questions: RunnerQuestion[]; answers: PracticeAnswer[] }>(
        `/api/practice/attempts/${attemptRow.id}`,
      )
      if (cancelled) return
      setAttempt(detail.attempt)
      setQuestions(detail.questions ?? [])
      const answerMap: Record<string, PracticeAnswer> = {}
      for (const answer of detail.answers ?? []) answerMap[answer.questionId] = answer
      setAnswers(answerMap)
      if (detail.attempt.finishedAt) setStage('finished')
      else setStage('running')
    }
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [setId])

  const current = questions[index]
  const examMode = attempt?.mode === 'exam'
  const finished = stage === 'finished'

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
      const summary = await api.request<{ attempt: PracticeAttempt; questions: RunnerQuestion[]; answers: PracticeAnswer[]; correct: number }>(
        `/api/practice/attempts/${attempt.id}/finish`,
        { method: 'POST', body: '{}' },
      )
      setAttempt(summary.attempt)
      setQuestions(summary.questions ?? [])
      setAnswers(Object.fromEntries((summary.answers ?? []).map((answer) => [answer.questionId, answer])))
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

  async function submitMCQ(question: RunnerQuestion, choiceKey: string) {
    if (!attempt || busy) return
    setBusy(true)
    setError('')
    try {
      const payload = await api.request<{ answer: PracticeAnswer; question?: RunnerQuestion }>(
        `/api/practice/attempts/${attempt.id}/answers`,
        { method: 'POST', body: JSON.stringify({ questionId: question.id, choiceKey }) },
      )
      setAnswers((current) => ({ ...current, [question.id]: payload.answer }))
      if (payload.question) {
        setQuestions((rows) => rows.map((row) => (row.id === question.id ? payload.question! : row)))
      }
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
      setAnswers((current) => ({ ...current, [question.id]: payload.answer }))
      if (payload.question) {
        setQuestions((rows) => rows.map((row) => (row.id === question.id ? payload.question! : row)))
        setRevealed((current) => ({ ...current, [question.id]: true }))
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    } finally {
      setBusy(false)
    }
  }

  async function selfRate(question: RunnerQuestion, rating: 'proficient' | 'partial' | 'weak') {
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
  }

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
    const answered = Object.values(answers)
    const correctCount = answered.filter((answer) => answer.isCorrect).length
    const mcqCount = questions.filter((question) => question.type === 'mcq').length
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
        {attempt?.deadlineAt && secondsLeft !== null && secondsLeft <= 0 && <div className="error">{t.timeUp}</div>}
        {(() => {
          // 80%+ (or a subjective-only set) gets the cheering cat; the rest
          // get the thinking cat and an invitation to review.
          const total = attempt?.totalMcq ?? mcqCount
          const great = total === 0 || (attempt?.score ?? correctCount) / total >= 0.8
          return (
            <div className="result-celebrate">
              <strong>{great ? t.cheerGreat : t.cheerKeep}</strong>
              {great && <span className="muted">{t[praiseKey()]}</span>}
            </div>
          )
        })()}
        <div className="metrics">
          <div className="metric">
            <span>{t.score}</span>
            <strong>
              {attempt?.score ?? correctCount} / {attempt?.totalMcq ?? mcqCount}
            </strong>
          </div>
          <div className="metric">
            <span>{t.correct}</span>
            <strong>{correctCount}</strong>
          </div>
          <div className="metric">
            <span>{t.incorrect}</span>
            <strong>{answered.filter((answer) => answer.isCorrect === false).length}</strong>
          </div>
          <div className="metric">
            <span>{t.unanswered}</span>
            <strong>{mcqCount - answered.filter((answer) => answer.choiceKey).length}</strong>
          </div>
        </div>
        <h3 className="section-title">{t.reviewAnswers}</h3>
        <div className="review-list">
          {questions.map((question, i) => (
            <div className="review-item" key={question.id}>
              <div className="review-head">
                <strong>
                  {i + 1}. {question.stem.slice(0, 120)}
                  {question.stem.length > 120 ? '…' : ''}
                </strong>
                {answers[question.id] && <ResultPill answer={answers[question.id]} type={question.type} />}
              </div>
              {question.type === 'mcq' && (
                <div className="review-detail">
                  <p>
                    {t.yourAnswer}: <strong>{answers[question.id]?.choiceKey || '—'}</strong> · {t.answerKey}:{' '}
                    <strong>{question.answerKey}</strong>
                  </p>
                  {question.explanation && <p className="muted">{question.explanation}</p>}
                </div>
              )}
              {question.type === 'subjective' && (
                <div className="review-detail">
                  <p className="muted">{answers[question.id]?.textAnswer?.slice(0, 200)}</p>
                  {(question.parts ?? []).map((part) => (
                    <div key={part.label} className="reference-block">
                      <strong>
                        {part.label}. {part.referenceAnswer}
                      </strong>
                      {part.rubric && part.rubric.length > 0 && <p>{part.rubric.join(' · ')}</p>}
                    </div>
                  ))}
                  {answers[question.id]?.textAnswer && (
                    <>
                      <p className="muted" style={{ marginTop: 8 }}>
                        {t.selfAssess}
                      </p>
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
                    </>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      </section>
    )
  }

  const showFeedback = !examMode && current?.type === 'mcq' && answers[current.id]
  const showSubjectiveForm = current?.type === 'subjective' && (!revealed[current.id] || !answers[current.id]?.textAnswer)
  const showSubjectiveReview = current?.type === 'subjective' && answers[current.id]?.textAnswer && (revealed[current.id] || !examMode)

  return (
    <section className="page runner-page">
      <Header
        eyebrow={set?.set.title ?? ''}
        title={`${index + 1} / ${questions.length}`}
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
          const answer = answers[question.id]
          const answered = answer && (answer.choiceKey || answer.textAnswer)
          return (
            <button
              key={question.id}
              className={`${i === index ? 'current' : ''} ${answered ? 'answered' : ''}`}
              onClick={() => setIndex(i)}
            >
              {i + 1}
            </button>
          )
        })}
      </div>
      {current && (
        <div className="question-card">
          {(current.materials ?? []).map((material, i) => (
            <div className="material-block" key={i}>
              {material.title && <strong>{material.title}</strong>}
              <p>{material.text}</p>
            </div>
          ))}
          {current.type === 'subjective' && (current.parts ?? []).length > 0 && (
            <div className="material-block">
              {(current.parts ?? []).map((part) => (
                <p key={part.label}>
                  <strong>({part.label})</strong> {part.prompt}
                </p>
              ))}
            </div>
          )}
          <h2>{current.stem}</h2>
          {current.type === 'mcq' ? (
            <div className="choice-list">
              {(current.choices ?? []).map((choice) => {
                const picked = answers[current.id]?.choiceKey === choice.key
                const isKey = showFeedback && current.answerKey === choice.key
                return (
                  <button
                    key={choice.key}
                    className={`choice ${picked ? 'picked' : ''} ${isKey ? 'correct' : ''} ${
                      showFeedback && picked && !isKey ? 'wrong' : ''
                    }`}
                    disabled={!!showFeedback || busy}
                    onClick={() => void submitMCQ(current, choice.key)}
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
            <SubjectiveForm key={current.id} busy={busy} onSubmit={(text) => void submitSubjective(current, text)} />
          ) : null}

          {showFeedback && (
            <div className={`feedback ${answers[current.id]?.isCorrect ? 'ok' : 'bad'}`}>
              <strong>{answers[current.id]?.isCorrect ? t.correct : t.incorrect}</strong>
              {current.explanation && <p>{current.explanation}</p>}
            </div>
          )}
          {current.type === 'subjective' && examMode && answers[current.id]?.textAnswer && (
            <div className="feedback neutral">
              <p className="muted">{answers[current.id]?.textAnswer}</p>
            </div>
          )}
          {showSubjectiveReview && (
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
              {!examMode && (
                <>
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
                        className={answers[current.id]?.selfRating === value ? 'active' : ''}
                        onClick={() => void selfRate(current, value)}
                      >
                        {label}
                      </button>
                    ))}
                  </div>
                </>
              )}
            </div>
          )}
        </div>
      )}
      {error && <div className="error">{error}</div>}
      <div className="runner-footer">
        <button className="secondary" disabled={index === 0} onClick={() => setIndex((value) => value - 1)}>
          {t.previous}
        </button>
        {index < questions.length - 1 ? (
          <button className="primary" onClick={() => setIndex((value) => value + 1)}>
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

// Graded pill for the finished review list. MCQ answers are auto-graded;
// subjective answers carry no isCorrect and show their self-rating instead.
function ResultPill({ answer, type }: { answer: PracticeAnswer; type: string }) {
  const { t } = useSession()
  if (answer.isCorrect != null) {
    return <span className={`pill ${answer.isCorrect ? 'ok' : 'bad'}`}>{answer.isCorrect ? t.correct : t.incorrect}</span>
  }
  if (type === 'subjective' && answer.selfRating) {
    const label =
      answer.selfRating === 'proficient' ? t.selfProficient : answer.selfRating === 'partial' ? t.selfPartial : t.selfWeak
    return <span className="pill">{label}</span>
  }
  return null
}

function SubjectiveForm({ busy, onSubmit }: { busy: boolean; onSubmit: (text: string) => void }) {
  const { t } = useSession()
  const [text, setText] = useState('')
  return (
    <div className="subjective-form">
      <label>
        {t.typeAnswer}
        <textarea rows={8} value={text} onChange={(e) => setText(e.target.value)} />
      </label>
      <button className="primary" disabled={busy || !text.trim()} onClick={() => onSubmit(text)}>
        {t.submitAnswer}
      </button>
    </div>
  )
}
