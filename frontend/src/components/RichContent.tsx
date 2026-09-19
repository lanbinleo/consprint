import type { Block, ConceptContent } from '../lib/types'
import { useSession } from '../hooks/session'
import { imageOnlyBlock } from '../lib/inlineMarkdown'
import { InlineMarkdown } from './InlineMarkdown'

export function RichContent({ content }: { content?: ConceptContent }) {
  const { t } = useSession()
  if (!content) return <p className="muted">{t.noContent}</p>
  return (
    <div className="rich-content">
      <BlockGroup title={t.definition} blocks={content.definition} />
      <BlockGroup title={t.examples} blocks={content.examples} />
      <BlockGroup title={t.pitfalls} blocks={content.pitfalls} tone="warn" />
      <BlockGroup title={t.contentNotes} blocks={content.notes} />
    </div>
  )
}

function BlockGroup({ title, blocks, tone = '' }: { title: string; blocks?: Block[] | null; tone?: string }) {
  if (!blocks?.length) return null
  return (
    <div className={`block-group ${tone}`}>
      <h4>{title}</h4>
      {blocks.map((block, index) => {
        const figure = imageOnlyBlock(block.text)
        if (figure) {
          return (
            <figure key={`${title}-${index}`}>
              <img src={figure.src} alt={figure.alt} loading="lazy" />
            </figure>
          )
        }
        return (
          <p key={`${title}-${index}`}>
            <InlineMarkdown text={block.text} />
          </p>
        )
      })}
    </div>
  )
}
