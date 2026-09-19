// Minimal inline-markdown support for concept content blocks. Only inline
// syntax is parsed — blocks are already paragraph-level units, so no headings
// or lists. Rendering happens through React elements (never innerHTML), and
// URLs are protocol-whitelisted, so malicious markup has nowhere to execute.

export type InlineNode =
  | { type: 'text'; text: string }
  | { type: 'strong'; children: InlineNode[] }
  | { type: 'em'; children: InlineNode[] }
  | { type: 'strongEm'; children: InlineNode[] }
  | { type: 'underline'; children: InlineNode[] }
  | { type: 'strike'; children: InlineNode[] }
  | { type: 'mark'; color: string; children: InlineNode[] }
  | { type: 'code'; text: string }
  | { type: 'link'; href: string; text: string }
  | { type: 'image'; src: string; alt: string }

// Marker highlight palette: ==text== is lemon, ==tangerine|text== picks a
// color. Anything outside the palette renders as literal text.
export const MARKER_COLORS = ['lemon', 'tangerine', 'mint', 'sky', 'pink'] as const
export type MarkerColor = (typeof MARKER_COLORS)[number]

// Only absolute http(s) URLs and same-app absolute paths may become hrefs or
// image sources. Protocol-relative "//host" is rejected explicitly.
export function safeUrl(url: string): string | null {
  const trimmed = url.trim()
  if (trimmed.startsWith('//')) return null
  if (/^https?:\/\//i.test(trimmed) || trimmed.startsWith('/')) return trimmed
  return null
}

// Delimiters must hug non-space content on both sides, so "2 * 3 * 4" stays
// plain text instead of turning "3" italic. Lookbehind/lookahead keep the
// spaces out of the captured content.
const INLINE_RE =
  /!\[(?<imgAlt>[^\]]*)\]\((?<imgSrc>[^)\s]+)\)|\[(?<linkText>[^\]]+)\]\((?<linkHref>[^)\s]+)\)|`(?<code>[^`]+)`|\*\*\*(?=\S)(?<strongEm>[\s\S]+?)(?<=\S)\*\*\*|\*\*(?=\S)(?<strong>[\s\S]+?)(?<=\S)\*\*|\*(?=\S)(?<em>[\s\S]+?)(?<=\S)\*|\+\+(?=\S)(?<underline>[\s\S]+?)(?<=\S)\+\+|~~(?=\S)(?<strike>[\s\S]+?)(?<=\S)~~|==(?=\S)(?:(?<markColor>lemon|tangerine|mint|sky|pink)\|)?(?<mark>[\s\S]+?)(?<=\S)==/g

export function parseInline(text: string): InlineNode[] {
  const nodes: InlineNode[] = []
  let last = 0
  // matchAll scans with an internal clone of the pattern, so the recursive
  // parseInline calls below cannot clobber this loop's position (a shared
  // lastIndex here used to re-match fragments forever and OOM the tab).
  for (const match of text.matchAll(INLINE_RE)) {
    if (match.index > last) nodes.push({ type: 'text', text: text.slice(last, match.index) })
    const groups = match.groups ?? {}
    if (groups.imgSrc !== undefined) {
      const src = safeUrl(groups.imgSrc)
      nodes.push(src ? { type: 'image', src, alt: groups.imgAlt } : { type: 'text', text: match[0] })
    } else if (groups.linkHref !== undefined) {
      const href = safeUrl(groups.linkHref)
      nodes.push(href ? { type: 'link', href, text: groups.linkText } : { type: 'text', text: match[0] })
    } else if (groups.code !== undefined) {
      nodes.push({ type: 'code', text: groups.code })
    } else if (groups.strongEm !== undefined) {
      nodes.push({ type: 'strongEm', children: parseInline(groups.strongEm) })
    } else if (groups.strong !== undefined) {
      nodes.push({ type: 'strong', children: parseInline(groups.strong) })
    } else if (groups.em !== undefined) {
      nodes.push({ type: 'em', children: parseInline(groups.em) })
    } else if (groups.underline !== undefined) {
      nodes.push({ type: 'underline', children: parseInline(groups.underline) })
    } else if (groups.strike !== undefined) {
      nodes.push({ type: 'strike', children: parseInline(groups.strike) })
    } else if (groups.mark !== undefined) {
      nodes.push({ type: 'mark', color: groups.markColor ?? 'lemon', children: parseInline(groups.mark) })
    }
    last = match.index + match[0].length
  }
  if (last < text.length) nodes.push({ type: 'text', text: text.slice(last) })
  return nodes
}

// A block whose entire text is one image markdown renders as a block-level
// figure instead of an image wedged inside a paragraph.
export function imageOnlyBlock(text: string): { src: string; alt: string } | null {
  const trimmed = text.trim()
  if (!trimmed.startsWith('![')) return null
  const match = /^!\[([^\]]*)\]\(([^)\s]+)\)$/.exec(trimmed)
  if (!match) return null
  const src = safeUrl(match[2])
  return src ? { src, alt: match[1] } : null
}
