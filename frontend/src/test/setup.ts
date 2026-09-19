import { vi } from 'vitest'

// Node has no localStorage, and the app touches it at import time (the API
// client reads its token). Give every test a memory-backed implementation.
const store = new Map<string, string>()
vi.stubGlobal('localStorage', {
  getItem: (key: string) => store.get(key) ?? null,
  setItem: (key: string, value: string) => void store.set(key, value),
  removeItem: (key: string) => void store.delete(key),
  key: (index: number) => [...store.keys()][index] ?? null,
  get length() {
    return store.size
  },
})
