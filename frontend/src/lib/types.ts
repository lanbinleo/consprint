export type Role = 'student' | 'teacher' | 'admin'
export type ConceptStatus = '' | 'proficient' | 'fuzzy' | 'unknown'
export type Lang = 'en' | 'zh'

export type Unit = {
  id: string
  title: string
  topics: Topic[]
}

export type Topic = {
  id: string
  unitId: string
  title: string
}

export type Block = {
  type: string
  text: string
}

export type ConceptContent = {
  definition: Block[] | null
  examples: Block[] | null
  pitfalls: Block[] | null
  notes: Block[] | null
  source: string
  confidence: number
  needsReview: boolean
}

export type Concept = {
  id: string
  unitId: string
  topicId: string
  term: string
  contentStatus: string
  unit?: Unit
  topic?: Topic
  content?: ConceptContent
}

export type ConceptState = {
  conceptId: string
  status: ConceptStatus
  reviewCount: number
  shortTermReview: boolean
}

export type ConceptRow = Concept & {
  state: ConceptState
}

export type User = {
  id: string
  name: string
  email: string
  role: Role
  provider: string
  avatarDataUrl?: string
  createdAt: string
}

export type AuthPayload = {
  token: string
  user: User
  tenant: { id: string; name: string }
}

export type AppMeta = {
  entra: boolean
  examDate: string
  timezone: string
}

export type StatBucket = {
  label: string
  reviews: number
  proficient: number
  fuzzy: number
  unknown: number
}

export type DashboardSummary = { totalConcepts: number; readyConcepts: number }

export type DashboardProgress = {
  reviewedConcepts: number
  markedConcepts: number
  proficientConcepts: number
  fuzzyConcepts: number
  unknownConcepts: number
  shortTermReviews: number
  todayReviews: number
  streakDays: number
}

export type WeakArea = { label: string; weak: number; marked: number }

export type DashboardAlerts = {
  recent: ReviewEvent[]
  weakConcepts: Concept[]
  weakUnits: WeakArea[]
  weakTopics: WeakArea[]
}

export type ReviewEvent = {
  id: string
  conceptId: string
  response: 'proficient' | 'fuzzy' | 'unknown'
  createdAt: string
}

// ---- Question bank ----

export type QuestionChoice = { key: string; text: string }

export type QuestionMaterial = { title: string; text: string }

export type QuestionPart = {
  label: string
  prompt: string
  referenceAnswer?: string
  rubric?: string[]
}

export type Tag = { id: string; name: string }

export type Question = {
  id: string
  type: 'mcq' | 'subjective'
  stem: string
  materials: QuestionMaterial[] | null
  choices: QuestionChoice[] | null
  answerKey: string
  explanation: string
  parts: QuestionPart[] | null
  unitId?: string | null
  topicId?: string | null
  status: 'draft' | 'published' | 'archived'
  source: string
  sourceNote: string
  tags: Tag[]
}

export type QuestionDraft = {
  type: 'mcq' | 'subjective'
  stem: string
  materials?: QuestionMaterial[]
  choices?: QuestionChoice[]
  answerKey?: string
  explanation?: string
  parts?: QuestionPart[]
  unit?: string
  topic?: string
  tags?: string[]
  sourceNote?: string
}

export type ImportPreview = {
  total: number
  valid: number
  channel: 'csv' | 'json'
  items: QuestionDraft[]
  errors: { index: number; message: string }[]
  importId?: string
}

// ---- Practice ----

export type PracticeSet = {
  id: string
  title: string
  description: string
  mode: 'instant' | 'exam'
  timeLimitSec?: number | null
  status: 'draft' | 'published' | 'archived'
  questionCount?: number
  attempts?: { id: string; finishedAt?: string | null; score?: number | null; totalMcq: number }[]
  bestScore?: number | null
}

export type PracticeAttempt = {
  id: string
  userId: string
  setId: string
  mode: 'instant' | 'exam'
  startedAt: string
  deadlineAt?: string | null
  finishedAt?: string | null
  score?: number | null
  totalMcq: number
}

export type PracticeAnswer = {
  id: string
  attemptId: string
  questionId: string
  choiceKey: string
  textAnswer: string
  isCorrect?: boolean | null
  selfRating: '' | 'proficient' | 'partial' | 'weak'
  answeredAt: string
}

// Student-facing question view: answerKey/explanation absent until revealed.
export type RunnerQuestion = {
  id: string
  type: 'mcq' | 'subjective'
  stem: string
  materials: QuestionMaterial[] | null
  choices: QuestionChoice[] | null
  tags: Tag[]
  answerKey?: string
  explanation?: string
  parts?: (QuestionPart & { referenceAnswer?: string; rubric?: string[] })[]
  myCorrect?: boolean
}

export type WrongEntry = {
  question: Question
  reason: 'wrong' | 'weak'
  lastAt: string
  wrongCount: number
}

export type AccuracyRow = {
  id: string
  label: string
  answered: number
  correct: number
  accuracy: number
}

export type PracticeStats = {
  attempts: number
  byUnit: AccuracyRow[]
  byTopic: AccuracyRow[]
  selfRatings: { rating: string; count: number }[]
}

// ---- Admin analytics ----

export type AnalyticsOverview = {
  roles: { role: string; count: number }[]
  activeUsers7d: number
  practice: { attempts: number; answered: number; correct: number; accuracy: number }
  accuracyByUnit: AccuracyRow[]
  flashcard: { status: string; count: number }[]
  weakConcepts: { term: string; unit: string; weak: number }[]
  daily: { label: string; reviews: number; attempts: number }[]
  students: {
    id: string
    name: string
    email: string
    role: string
    attempts: number
    answered: number
    correct: number
    marked: number
    accuracy: number
  }[]
}

export type AnalyticsUserDetail = {
  user: User
  flashcard: { status: string; count: number }[]
  streakDays: number
  recent: ReviewEvent[]
  attempts: (PracticeAttempt & { setTitle: string })[]
  accuracyByUnit: AccuracyRow[]
  wrongAnswers: number
}

// ---- Concept import (admin content manager) ----

export type ImportStatus = {
  units: number
  topics: number
  concepts: number
  readyConcepts: number
  byUnit: { unitId: string; unit: string; concepts: number; ready: number }[]
  runs: { id: string; source: string; status: string; message: string; counts: string; createdAt: string }[]
}
