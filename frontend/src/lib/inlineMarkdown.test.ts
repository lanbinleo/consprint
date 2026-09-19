import { describe, expect, it } from 'vitest'
import { imageOnlyBlock, parseInline, safeUrl } from './inlineMarkdown'

function types(text: string) {
  return parseInline(text).map((node) => node.type)
}

describe('safeUrl', () => {
  it('allows http(s) and same-app paths', () => {
    expect(safeUrl('https://example.com/a')).toBe('https://example.com/a')
    expect(safeUrl('http://example.com')).toBe('http://example.com')
    expect(safeUrl('/files/notes-1-a.pdf')).toBe('/files/notes-1-a.pdf')
  })

  it('rejects dangerous and protocol-relative URLs', () => {
    expect(safeUrl('javascript:alert(1)')).toBeNull()
    expect(safeUrl('//evil.com/x')).toBeNull()
    expect(safeUrl('data:text/html,x')).toBeNull()
    expect(safeUrl('')).toBeNull()
  })
})

describe('parseInline', () => {
  it('keeps plain text untouched', () => {
    expect(parseInline('plain text')).toEqual([{ type: 'text', text: 'plain text' }])
    expect(parseInline('')).toEqual([])
  })

  it('parses emphasis markers', () => {
    expect(types('**bold**')).toEqual(['strong'])
    expect(types('*italic*')).toEqual(['em'])
    expect(types('***both***')).toEqual(['strongEm'])
    expect(types('++underline++')).toEqual(['underline'])
    expect(types('~~strike~~')).toEqual(['strike'])
    expect(parseInline('`code`')).toEqual([{ type: 'code', text: 'code' }])
  })

  it('mixes emphasis with surrounding text', () => {
    expect(parseInline('a **b** c')).toEqual([
      { type: 'text', text: 'a ' },
      { type: 'strong', children: [{ type: 'text', text: 'b' }] },
      { type: 'text', text: ' c' },
    ])
  })

  it('nests italic inside bold', () => {
    const [node] = parseInline('**bold *in* bold**')
    expect(node.type).toBe('strong')
    if (node.type === 'strong') {
      expect(node.children.map((child) => child.type)).toEqual(['text', 'em', 'text'])
    }
  })

  it('parses links and images with safe URLs', () => {
    expect(parseInline('[AP](https://apstudents.org)')).toEqual([
      { type: 'link', href: 'https://apstudents.org', text: 'AP' },
    ])
    expect(parseInline('![图解](/files/notes-1-a.png)')).toEqual([
      { type: 'image', src: '/files/notes-1-a.png', alt: '图解' },
    ])
  })

  it('renders unsafe link targets as literal text', () => {
    // The href charset stops at the first ")", so a parenthesised unsafe URL
    // falls back as two literal text runs — never a clickable link node.
    expect(parseInline('[click](javascript:alert(1))')).toEqual([
      { type: 'text', text: '[click](javascript:alert(1)' },
      { type: 'text', text: ')' },
    ])
    const relative = '[evil](//evil.com)'
    expect(parseInline(relative)).toEqual([{ type: 'text', text: relative }])
  })

  it('does not italicize multiplication spacing', () => {
    expect(types('2 * 3 * 4')).toEqual(['text'])
  })

  it('leaves unclosed markers as text', () => {
    expect(parseInline('**unclosed')).toEqual([{ type: 'text', text: '**unclosed' }])
    expect(parseInline('`unclosed code')).toEqual([{ type: 'text', text: '`unclosed code' }])
  })

  it('parses marker highlights with an optional color', () => {
    expect(parseInline('==重点==')).toEqual([
      { type: 'mark', color: 'lemon', children: [{ type: 'text', text: '重点' }] },
    ])
    expect(parseInline('==tangerine|周五模考==')).toEqual([
      { type: 'mark', color: 'tangerine', children: [{ type: 'text', text: '周五模考' }] },
    ])
    expect(parseInline('==mint|绿色== 和 ==sky|蓝色==')).toEqual([
      { type: 'mark', color: 'mint', children: [{ type: 'text', text: '绿色' }] },
      { type: 'text', text: ' 和 ' },
      { type: 'mark', color: 'sky', children: [{ type: 'text', text: '蓝色' }] },
    ])
    // Highlights must hug content: "a == b == c" stays literal, so a marker
    // ending in a space cannot close and swallows into the next valid close.
    expect(parseInline('==mint|绿色 ==')).toEqual([
      { type: 'text', text: '==mint|绿色 ==' },
    ])
  })

  it('nests emphasis inside marker highlights', () => {
    const [node] = parseInline('==pink|**加粗**高亮==')
    expect(node.type).toBe('mark')
    if (node.type === 'mark') {
      expect(node.color).toBe('pink')
      expect(node.children.map((child) => child.type)).toEqual(['strong', 'text'])
    }
  })

  it('keeps unknown marker colors and unclosed markers literal', () => {
    expect(types('==red|not a palette color==')).toEqual(['mark'])
    expect(parseInline('==red|not a palette color==')).toEqual([
      { type: 'mark', color: 'lemon', children: [{ type: 'text', text: 'red|not a palette color' }] },
    ])
    expect(parseInline('==unclosed highlight')).toEqual([{ type: 'text', text: '==unclosed highlight' }])
    expect(parseInline('a == b == c')).toEqual([{ type: 'text', text: 'a == b == c' }])
  })

  it('does not span emphasis across lines within a block', () => {
    // Blocks are single lines already, but a stray newline must not swallow content.
    expect(types('**a**\nnot')).toEqual(['strong', 'text'])
  })
})

describe('imageOnlyBlock', () => {
  it('detects a block that is exactly one image', () => {
    expect(imageOnlyBlock('![diagram](/files/a.png)')).toEqual({ src: '/files/a.png', alt: 'diagram' })
    expect(imageOnlyBlock('  ![diagram](https://x.com/a.png)  ')).toEqual({ src: 'https://x.com/a.png', alt: 'diagram' })
  })

  it('rejects mixed or unsafe blocks', () => {
    expect(imageOnlyBlock('See ![diagram](/files/a.png) here')).toBeNull()
    expect(imageOnlyBlock('![x](javascript:alert(1))')).toBeNull()
    expect(imageOnlyBlock('plain paragraph')).toBeNull()
  })
})
