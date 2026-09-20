import { useState } from 'react'
import { useSession } from '../hooks/session'
import { praiseKey } from '../lib/i18n'
import { InlineMarkdown } from './InlineMarkdown'
import type { PracticeAnswer, PracticeAttempt, RunnerQuestion } from '../lib/types'

// Shared finished-attempt review: celebration + metrics + per-question review
// with references and self-rating. Used by the runner's finished stage and
// the read-only history review page.
export function AttemptReview({
  attempt,
  questions,
  answers,
  timeUp,
  selfRate,
  selfRateParts,
}: {
  attempt: PracticeAttempt
  questions: RunnerQuestion[]
  answers: Record<string, PracticeAnswer>
  timeUp?: boolean
  selfRate: (question: RunnerQuestion, rating: 'proficient' | 'partial' | 'weak') => void
  selfRateParts: (question: RunnerQuestion, ratings: Record<string, 'proficient' | 'partial' | 'weak'>) => void
}) {
  const { t } = useSession()
  const answered = Object.values(answers)
  const correctCount = answered.filter((answer) => answer.isCorrect).length
  const mcqCount = questions.filter((question) => question.type === 'mcq').length
  return (
    <>
      {timeUp && <div className="error">{t.timeUp}</div>}
      {(() => {
        // 80%+ (or a subjective-only set) gets the cheering cat; the rest
        // get the thinking cat and an invitation to review.
        const total = attempt.totalMcq ?? mcqCount
        const great = total === 0 || (attempt.score ?? correctCount) / total >= 0.8
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
            {attempt.score ?? correctCount} / {attempt.totalMcq ?? mcqCount}
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
          <strong>{questions.length - answered.length}</strong>
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
                {answers[question.id]?.parts?.length ? (
                  <PartsReview question={question} answer={answers[question.id]} onRate={(ratings) => selfRateParts(question, ratings)} />
                ) : (
                  <>
                    <p className="muted">{answers[question.id]?.textAnswer?.slice(0, 200)}</p>
                    {(question.parts ?? []).map((part) => (
                      <div key={part.label} className="reference-block">
                        <strong>
                          {part.label}. {part.referenceAnswer}
                        </strong>
                        {part.rubric && part.rubric.length > 0 && <p>{part.rubric.join(' · ')}</p>}
                      </div>
                    ))}
                  </>
                )}
                {answers[question.id]?.textAnswer && (
                  <SelfRatingRow current={answers[question.id]?.selfRating} onRate={(rating) => selfRate(question, rating)} />
                )}
              </div>
            )}
          </div>
        ))}
      </div>
    </>
  )
}

// Graded pill for the review list. MCQ answers are auto-graded; subjective
// answers carry no isCorrect and show their self-rating instead.
export function ResultPill({ answer, type }: { answer: PracticeAnswer; type: string }) {
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

// Finished review for AAQ/EBQ answers: your text, the reference, the rubric,
// and per-part self-rating for every part.
export function PartsReview({
  question,
  answer,
  onRate,
}: {
  question: RunnerQuestion
  answer: PracticeAnswer
  onRate: (ratings: Record<string, 'proficient' | 'partial' | 'weak'>) => void
}) {
  const { t } = useSession()
  const [ratings, setRatings] = useState<Record<string, 'proficient' | 'partial' | 'weak'>>(() =>
    Object.fromEntries((answer.partRatings ?? []).map((row) => [row.label, row.rating])),
  )
  const partText = (label: string) => answer.parts?.find((row) => row.label === label)?.text ?? ''
  return (
    <div className="parts-review">
      {(question.parts ?? []).map((part) => (
        <div className="part-row" key={part.label}>
          <div className="part-label">
            <strong>({part.label})</strong>
            {part.points != null && <em> · {part.points} {t.points}</em>}
            <span>
              <InlineMarkdown text={part.prompt} />
            </span>
          </div>
          <p className="muted your-answer">{partText(part.label) || '—'}</p>
          {part.referenceAnswer && (
            <p>
              <strong>{t.reference}:</strong> {part.referenceAnswer}
            </p>
          )}
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
                className={ratings[part.label] === value ? 'active' : ''}
                onClick={() => {
                  const next = { ...ratings, [part.label]: value }
                  setRatings(next)
                  onRate(next)
                }}
              >
                {label}
              </button>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

export function SelfRatingRow({
  current,
  onRate,
}: {
  current?: string
  onRate: (rating: 'proficient' | 'partial' | 'weak') => void
}) {
  const { t } = useSession()
  return (
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
          <button key={value} className={current === value ? 'active' : ''} onClick={() => onRate(value)}>
            {label}
          </button>
        ))}
      </div>
    </>
  )
}
