import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import type { ConceptChip } from '../lib/types'

// Multi-select concept linking for questions: chips of the current links plus
// a debounced search over the concept corpus. The parent only deals with ids.
export function ConceptPicker({
  initial,
  onChange,
}: {
  initial: ConceptChip[]
  onChange: (ids: string[]) => void
}) {
  const { t } = useSession()
  const [chips, setChips] = useState<ConceptChip[]>(initial)
  const [search, setSearch] = useState('')
  const [debounced, setDebounced] = useState('')
  const [open, setOpen] = useState(false)

  // Debounce the search term; the corpus endpoint does the matching.
  useMemo(() => {
    const timer = setTimeout(() => setDebounced(search.trim()), 250)
    return () => clearTimeout(timer)
  }, [search])

  const { data: results = [], isFetching } = useQuery({
    queryKey: ['concept-search', debounced],
    enabled: open && debounced.length > 0,
    queryFn: async () =>
      (await api.request<{ id: string; term: string }[] | null>(
        `/api/concepts?search=${encodeURIComponent(debounced)}&limit=12`,
      )) ?? [],
  })

  function add(chip: ConceptChip) {
    if (chips.some((existing) => existing.id === chip.id)) return
    const next = [...chips, chip]
    setChips(next)
    onChange(next.map((item) => item.id))
    setSearch('')
    setOpen(false)
  }

  function remove(id: string) {
    const next = chips.filter((chip) => chip.id !== id)
    setChips(next)
    onChange(next.map((item) => item.id))
  }

  return (
    <div className="concept-picker">
      {chips.length > 0 && (
        <div className="chip-row">
          {chips.map((chip) => (
            <span className="chip" key={chip.id}>
              {chip.term}
              <button type="button" onClick={() => remove(chip.id)} aria-label="remove">
                <X size={12} />
              </button>
            </span>
          ))}
        </div>
      )}
      <div className="concept-search">
        <input
          value={search}
          placeholder={t.linkConcepts}
          onFocus={() => setOpen(true)}
          onChange={(e) => {
            setSearch(e.target.value)
            setOpen(true)
          }}
        />
        {open && debounced.length > 0 && (
          <div className="concept-results">
            {isFetching && <small className="muted">{t.loading}</small>}
            {!isFetching && results.length === 0 && <small className="muted">{t.noConceptMatches}</small>}
            {results.map((concept) => (
              <button type="button" key={concept.id} onClick={() => add({ id: concept.id, term: concept.term })}>
                {concept.term}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
