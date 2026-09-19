import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import type { Unit } from '../lib/types'

export function useUnits() {
  return useQuery({
    queryKey: ['units'],
    // Normalize null (Go nil slice) so every consumer can rely on an array.
    queryFn: async () => (await api.request<Unit[] | null>('/api/units')) ?? [],
    staleTime: 5 * 60_000,
  })
}
