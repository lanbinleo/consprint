import type { Block, ConceptContent } from '../lib/types'
import { sourceLabel } from '../lib/format'
import { useSession } from '../hooks/session'

export function RichContent({ content }: { content?: ConceptContent }) {
  const { t } = useSession()
  if (!content) return <p className="muted">{t.noContent}</p>
  return (
    <div className="rich-content">
      <BlockGroup title={t.definition} blocks={content.definition} />
      <BlockGroup title={t.examples} blocks={content.examples} />
      <BlockGroup title={t.pitfalls} blocks={content.pitfalls} tone="warn" />
      <BlockGroup title={t.notes} blocks={content.notes} />
      <small className="source-line">
        {t.source}: {sourceLabel(content.source)}
      </small>
    </div>
  )
}

function BlockGroup({ title, blocks, tone = '' }: { title: string; blocks?: Block[] | null; tone?: string }) {
  if (!blocks?.length) return null
  return (
    <div className={`block-group ${tone}`}>
      <h4>{title}</h4>
      {blocks.map((block, index) => (
        <p key={`${title}-${index}`}>{block.text}</p>
      ))}
    </div>
  )
}
