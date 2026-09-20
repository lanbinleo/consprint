import { useState } from 'react'
import { useNavigate } from 'react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus } from 'lucide-react'
import { api } from '../../lib/api'
import { useSession } from '../../hooks/session'
import { Header } from '../../components/ui'
import type { PracticeSet } from '../../lib/types'

// Set list; assembly itself lives on the full-page editor /admin/sets/:id.
type SetRow = PracticeSet & { questionCount: number; unitIds?: string[]; formats?: string[] }

export function Sets() {
  const { t } = useSession()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [actionError, setActionError] = useState('')

  const { data } = useQuery({
    queryKey: ['admin-sets'],
    queryFn: async () => (await api.request<SetRow[] | null>('/api/admin/sets')) ?? [],
  })
  const sets = data ?? []

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin-sets'] })

  async function patchStatus(id: string, status: 'published' | 'archived') {
    setActionError('')
    try {
      await api.request(`/api/admin/sets/${id}`, { method: 'PATCH', body: JSON.stringify({ status }) })
      void refresh()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t.errorGeneric)
    }
  }

  return (
    <div>
      <Header
        eyebrow={t.admin}
        title={t.setsTitle}
        action={
          <button className="primary" onClick={() => navigate('/admin/sets/new')}>
            <Plus size={16} /> {t.newSet}
          </button>
        }
      />
      {sets.length === 0 ? (
        <div className="empty-state">{t.noSetsYet}</div>
      ) : (
        <div className="table">
          {sets.map((set) => (
            <div className="concept-row" key={set.id}>
              <button className="row-main" onClick={() => navigate(`/admin/sets/${set.id}`)}>
                <span>
                  <strong>{set.title}</strong>
                  <small>
                    {set.mode === 'exam' ? t.examMode : t.instantMode} · {set.questionCount} {t.questions}
                    {set.formats && set.formats.length > 0 ? ` · ${set.formats.map((format) => format.toUpperCase()).join(' / ')}` : ''} ·{' '}
                    {set.status === 'published' ? t.published : set.status === 'draft' ? t.draft : t.archived}
                  </small>
                </span>
              </button>
              <div className="row-actions">
                <button className="secondary" onClick={() => navigate(`/admin/sets/${set.id}`)}>
                  <Pencil size={14} /> {t.editSet}
                </button>
                {set.status !== 'published' && (
                  <button className="secondary" onClick={() => void patchStatus(set.id, 'published')}>
                    {t.publish}
                  </button>
                )}
                {set.status === 'published' && (
                  <button className="secondary" onClick={() => void patchStatus(set.id, 'archived')}>
                    {t.unpublish}
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      {actionError && <div className="error" style={{ marginTop: 12 }}>{actionError}</div>}
    </div>
  )
}
