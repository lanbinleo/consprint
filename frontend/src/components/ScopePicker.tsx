import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import type { Unit } from '../lib/types'

export function useUnits() {
  return useQuery({
    queryKey: ['units'],
    queryFn: () => api.request<Unit[]>('/api/units'),
    staleTime: 5 * 60_000,
  })
}

export function ScopePicker({
  unitId,
  topicId,
  setUnitId,
  setTopicId,
}: {
  unitId: string
  topicId: string
  setUnitId: (id: string) => void
  setTopicId: (id: string) => void
}) {
  const { t } = useSession()
  const { data: units = [] } = useUnits()
  const topics = units.find((unit) => unit.id === unitId)?.topics ?? []
  return (
    <div className="scope-grid">
      <label>
        {t.unit}
        <select
          value={unitId}
          onChange={(e) => {
            setUnitId(e.target.value)
            setTopicId('')
          }}
        >
          <option value="">{t.allUnits}</option>
          {units.map((unit) => (
            <option key={unit.id} value={unit.id}>
              {unit.title}
            </option>
          ))}
        </select>
      </label>
      <label>
        {t.topic}
        <select value={topicId} onChange={(e) => setTopicId(e.target.value)} disabled={!unitId}>
          <option value="">{t.allTopics}</option>
          {topics.map((topic) => (
            <option key={topic.id} value={topic.id}>
              {topic.title}
            </option>
          ))}
        </select>
      </label>
    </div>
  )
}
