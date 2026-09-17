import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Eye, EyeOff, Search } from 'lucide-react'
import { api } from '../lib/api'
import { useSession } from '../hooks/session'
import { Header, ListSkeleton, StatusPill } from '../components/ui'
import { RichContent } from '../components/RichContent'
import { useUnits } from '../components/ScopePicker'
import type { ConceptRow, ConceptStatus } from '../lib/types'

export function Learn() {
  const { t } = useSession()
  const queryClient = useQueryClient()
  const { data: units = [] } = useUnits()
  const [selectedTopic, setSelectedTopic] = useState('')
  const [selectedUnit, setSelectedUnit] = useState('')
  const [search, setSearch] = useState('')
  const [active, setActive] = useState<ConceptRow | null>(null)
  const [panelHidden, setPanelHidden] = useState(false)

  const params = new URLSearchParams()
  if (selectedTopic) params.set('topicId', selectedTopic)
  else if (selectedUnit) params.set('unitId', selectedUnit)
  if (search) params.set('search', search)
  const { data: concepts = [], isPending } = useQuery({
    queryKey: ['concepts', selectedUnit, selectedTopic, search],
    queryFn: () => api.request<ConceptRow[]>(`/api/concepts?${params.toString()}`),
  })

  async function mark(concept: ConceptRow, status: ConceptStatus) {
    const state = await api.request<ConceptRow['state']>(`/api/concepts/${concept.id}/status`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    })
    setActive((current) => (current?.id === concept.id ? { ...current, state } : current))
    void queryClient.invalidateQueries({ queryKey: ['concepts'] })
    void queryClient.invalidateQueries({ queryKey: ['dashboard'] })
  }

  return (
    <section className="page">
      <Header
        eyebrow={t.learn}
        title={t.learnTitle}
        action={
          <button className="secondary" onClick={() => setPanelHidden((value) => !value)}>
            {panelHidden ? <Eye size={16} /> : <EyeOff size={16} />} {panelHidden ? t.showCard : t.hideCard}
          </button>
        }
      />
      <div className={`browse-layout ${panelHidden ? 'panel-off' : ''}`}>
        <aside className="topic-list">
          <button
            className={selectedTopic === '' && selectedUnit === '' ? 'active' : ''}
            onClick={() => {
              setSelectedTopic('')
              setSelectedUnit('')
            }}
          >
            {t.filterAll}
          </button>
          {units.map((unit) => (
            <div key={unit.id}>
              <h3>{unit.title}</h3>
              <button
                className={selectedUnit === unit.id && !selectedTopic ? 'active' : ''}
                onClick={() => {
                  setSelectedUnit(unit.id)
                  setSelectedTopic('')
                }}
              >
                {t.all}
              </button>
              {unit.topics.map((topic) => (
                <button
                  key={topic.id}
                  className={selectedTopic === topic.id ? 'active' : ''}
                  onClick={() => setSelectedTopic(topic.id)}
                >
                  {topic.title}
                </button>
              ))}
            </div>
          ))}
        </aside>
        <div className="concept-list">
          <label className="search">
            <Search size={16} />
            <input placeholder={t.searchTerms} value={search} onChange={(e) => setSearch(e.target.value)} />
          </label>
          {isPending ? (
            <ListSkeleton />
          ) : (
            <div className="table">
              {concepts.slice(0, 180).map((concept) => (
                <button
                  className={`concept-row ${active?.id === concept.id ? 'selected' : ''}`}
                  key={concept.id}
                  onClick={() => setActive(concept)}
                >
                  <span>
                    <strong>{concept.term}</strong>
                    <small>{concept.topic?.title}</small>
                  </span>
                  <StatusPill status={concept.state.status} />
                </button>
              ))}
            </div>
          )}
        </div>
        {!panelHidden && active && (
          <aside className="concept-panel">
            <small>{active.unit?.title}</small>
            <h2>{active.term}</h2>
            <div className="panel-status">
              <StatusPill status={active.state.status} />
              <div className="mini-status">
                {(['proficient', 'fuzzy', 'unknown'] as const).map((status) => (
                  <button
                    key={status}
                    className={active.state.status === status ? 'active' : ''}
                    onClick={() => mark(active, status)}
                  >
                    {status === 'proficient' ? t.proficient : status === 'fuzzy' ? t.fuzzy : t.unknown}
                  </button>
                ))}
              </div>
            </div>
            <RichContent content={active.content} />
          </aside>
        )}
      </div>
    </section>
  )
}
