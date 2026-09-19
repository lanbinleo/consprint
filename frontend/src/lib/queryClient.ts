import { QueryClient } from '@tanstack/react-query'

// Shared client instance: non-React modules (reviewQueue) need to invalidate
// caches after background flushes, so the client cannot live inside main.tsx.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false, staleTime: 30_000 },
  },
})
