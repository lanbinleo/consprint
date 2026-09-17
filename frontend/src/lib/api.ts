import type { AuthPayload } from './types'

const API_BASE = import.meta.env.VITE_API_BASE ?? ''

export class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

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
    if (!(options.body instanceof FormData)) headers.set('Content-Type', 'application/json')
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
      throw new ApiError(message, res.status)
    }
    return res.json() as Promise<T>
  }

  login(input: { email: string; password: string }) {
    return this.request<AuthPayload>('/api/auth/login', { method: 'POST', body: JSON.stringify(input) })
  }

  register(input: { name: string; email: string; password: string; inviteCode?: string }) {
    return this.request<AuthPayload>('/api/auth/register', { method: 'POST', body: JSON.stringify(input) })
  }
}

export const api = new ApiClient()
