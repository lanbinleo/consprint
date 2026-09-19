import { Fragment, type ReactNode } from 'react'
import type { InlineNode } from '../lib/inlineMarkdown'
import { parseInline } from '../lib/inlineMarkdown'

export function renderInlineNodes(nodes: InlineNode[], keyPrefix = ''): ReactNode[] {
  return nodes.map((node, index) => {
    const key = `${keyPrefix}-${index}`
    switch (node.type) {
      case 'strong':
        return <strong key={key}>{renderInlineNodes(node.children, key)}</strong>
      case 'em':
        return <em key={key}>{renderInlineNodes(node.children, key)}</em>
      case 'strongEm':
        return (
          <strong key={key}>
            <em>{renderInlineNodes(node.children, key)}</em>
          </strong>
        )
      case 'underline':
        return <u key={key}>{renderInlineNodes(node.children, key)}</u>
      case 'strike':
        return <del key={key}>{renderInlineNodes(node.children, key)}</del>
      case 'mark':
        return (
          <mark key={key} className={`mk-${node.color}`}>
            {renderInlineNodes(node.children, key)}
          </mark>
        )
      case 'code':
        return <code key={key}>{node.text}</code>
      case 'link':
        return (
          <a key={key} href={node.href} target="_blank" rel="noreferrer noopener">
            {node.text}
          </a>
        )
      case 'image':
        return <img key={key} src={node.src} alt={node.alt} loading="lazy" />
      default:
        return <Fragment key={key}>{node.text}</Fragment>
    }
  })
}

// Renders one line of concept content with inline markdown applied.
export function InlineMarkdown({ text }: { text: string }) {
  return <>{renderInlineNodes(parseInline(text))}</>
}
