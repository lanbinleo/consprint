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
  cards?: Card[]
}

export type Card = {
  id: string
  conceptId: string
  type: string
  prompt: string
  back: string
}

export type ConceptState = {
  conceptId: string
  mastery: number
  manualRating: number | null
  reviewCount: number
  shortTermReview: boolean
}

export type ConceptRow = Concept & {
  state: ConceptState
}

export type AuthPayload = {
  token: string
  user: { id: string; name: string; email: string; role: 'admin' | 'student'; avatarDataUrl?: string }
  tenant: { id: string; name: string }
}

export type StatBucket = {
  label: string
  reviews: number
  learned: number
  masteryGain: number
  averageMastery: number
}

const API_BASE = import.meta.env.VITE_API_BASE ?? ''

export class ApiClient {
  token = localStorage.getItem('apPsychToken') ?? ''

  setToken(token: string) {
    this.token = token
    localStorage.setItem('apPsychToken', token)
  }

  logout() {
    this.token = ''
    localStorage.removeItem('apPsychToken')
  }

  async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const headers = new Headers(options.headers)
    headers.set('Content-Type', 'application/json')
    if (this.token) headers.set('Authorization', `Bearer ${this.token}`)
    const res = await fetch(`${API_BASE}${path}`, { ...options, headers })
    if (!res.ok) {
      let message = `Request failed: ${res.status}`
      try {
        const body = await res.json()
        message = body.error ?? message
      } catch {
        // keep default message
      }
      throw new Error(message)
    }
    return res.json() as Promise<T>
  }

  register(input: { tenantName: string; name: string; email: string; password: string; inviteCode: string }) {
    return this.request<AuthPayload>('/api/auth/register', { method: 'POST', body: JSON.stringify(input) })
  }

  login(input: { email: string; password: string }) {
    return this.request<AuthPayload>('/api/auth/login', { method: 'POST', body: JSON.stringify(input) })
  }
}

export const api = new ApiClient()
