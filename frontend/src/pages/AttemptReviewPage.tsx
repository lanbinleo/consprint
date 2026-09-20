import { useState } from 'react'
import { Link, useParams } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { Header } from '../components/ui'
import { AttemptReview } from '../components/AttemptReview'
import type { PracticeAnswer, PracticeAttempt, PracticeSet, RunnerQuestion, Stimulus } from '../lib/types'

// Read-only review of a past (finished) attempt: the same reveal + review
// the runner shows right after finishing, reachable from 做题记录.
export function AttemptReviewPage() {
  const { attemptId = '' } = useParams()
  const { t } = useSession()
  const [answers, setAnswers] = useState<Record<string, PracticeAnswer> | null>(null)
  const [error, setError] = useState('')

  const { data, isPending } = useQuery({
    queryKey: ['practice-attempt', attemptId],
    queryFn: () =>
      api.request<{
        attempt: PracticeAttempt
        set: PracticeSet
        questions: RunnerQuestion[]
        answers: PracticeAnswer[]
        stimuli?: Stimulus[]
      }>(`/api/practice/attempts/${attemptId}`),
    enabled: !!attemptId,
  })

  const answerMap =
    answers ?? Object.fromEntries((data?.answers ?? []).map((answer) => [answer.questionId, answer]))

  async function rate(question: RunnerQuestion, body: Record<string, unknown>) {
    try {
      const updated = await api.request<PracticeAnswer>(`/api/practice/attempts/${attemptId}/answers/${question.id}/self-rating`, {
        method: 'POST',
        body: JSON.stringify(body),
      })
      setAnswers((current) => ({ ...(current ?? answerMap), [question.id]: updated }))
    } catch (err) {
      setError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  if (isPending) {
    return (
      <section className="page">
        <p className="muted">{t.loading}…</p>
      </section>
    )
  }
  if (!data) {
    return (
      <section className="page">
        <Header eyebrow={t.practice} title={t.errorGeneric} />
        <div className="error">{t.errorGeneric}</div>
      </section>
    )
  }

  return (
    <section className="page">
      <Header
        eyebrow={data.set?.title ?? ''}
        title={t.results}
        action={
          <Link className="secondary" to="/practice/history">
            <ArrowLeft size={16} /> {t.historyTitle}
          </Link>
        }
      />
      {data.attempt.finishedAt ? (
        <AttemptReview
          attempt={data.attempt}
          questions={data.questions ?? []}
          answers={answerMap}
          selfRate={(question, ratingValue) => void rate(question, { rating: ratingValue })}
          selfRateParts={(question, partRatings) =>
            void rate(question, { parts: (question.parts ?? []).map((part) => ({ label: part.label, rating: partRatings[part.label] ?? 'partial' })) })
          }
        />
      ) : (
        <div className="empty-state">
          <p className="muted">{t.attemptStillOpen}</p>
          <Link className="primary" to={`/practice/${data.attempt.setId}`}>
            {t.resume}
          </Link>
        </div>
      )}
      {error && <div className="error">{error}</div>}
    </section>
  )
}
