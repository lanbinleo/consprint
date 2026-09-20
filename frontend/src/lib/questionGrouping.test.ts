import { describe, expect, it } from 'vitest'
import { buildRenderItems, clusterRanges, groupQuestionClusters } from './questionGrouping'

const q = (id: string, stimulusId?: string) => ({ id, stimulusId: stimulusId ?? null })

describe('groupQuestionClusters', () => {
  it('groups adjacent questions sharing a stimulus', () => {
    const clusters = groupQuestionClusters([q('a', 's1'), q('b', 's1'), q('c'), q('d', 's1')])
    expect(clusters).toHaveLength(3)
    expect(clusters[0]).toMatchObject({ stimulusId: 's1', items: [q('a', 's1'), q('b', 's1')] })
    expect(clusters[1]).toMatchObject({ stimulusId: null, items: [q('c')] })
    // The same stimulus appearing again after a break is its own cluster.
    expect(clusters[2]).toMatchObject({ stimulusId: 's1', items: [q('d', 's1')] })
    expect(clusters[0].key).not.toBe(clusters[2].key)
  })

  it('keeps standalone questions as singleton clusters', () => {
    const clusters = groupQuestionClusters([q('a'), q('b'), q('c')])
    expect(clusters).toHaveLength(3)
    expect(clusters.every((cluster) => cluster.stimulusId === null && cluster.items.length === 1)).toBe(true)
  })

  it('returns empty for an empty paper', () => {
    expect(groupQuestionClusters([])).toEqual([])
  })

  it('aligns clusterRanges with the flat item order', () => {
    const items = [q('a', 's1'), q('b', 's1'), q('c'), q('d', 's2'), q('e', 's2')]
    expect(clusterRanges(items)).toEqual([
      { start: 0, length: 2, stimulusId: 's1' },
      { start: 2, length: 1, stimulusId: null },
      { start: 3, length: 2, stimulusId: 's2' },
    ])
  })
})

describe('buildRenderItems', () => {
  const mcq = (id: string, stimulusId?: string) => ({ id, type: 'mcq', stimulusId: stimulusId ?? null })
  const aaq = (id: string, stimulusId?: string) => ({ id, type: 'subjective', format: 'aaq', stimulusId: stimulusId ?? null })
  const ebq = (id: string, stimulusId?: string) => ({ id, type: 'subjective', format: 'ebq', stimulusId: stimulusId ?? null })
  const frq = (id: string) => ({ id, type: 'subjective', format: 'frq', stimulusId: null })

  it('renders standalone questions one by one', () => {
    const items = buildRenderItems([mcq('a'), frq('b'), mcq('c')])
    expect(items.map((item) => item.kind)).toEqual(['single', 'single', 'single'])
  })

  it('renders an MCQ stimulus cluster as one set item', () => {
    const items = buildRenderItems([mcq('a', 's1'), mcq('b', 's1'), mcq('c', 's1')])
    expect(items).toEqual([{ kind: 'mcq-set', stimulusId: 's1', questions: [mcq('a', 's1'), mcq('b', 's1'), mcq('c', 's1')] }])
  })

  it('splits a mixed cluster into the AAQ composite plus its MCQ siblings', () => {
    const items = buildRenderItems([mcq('a', 's1'), aaq('x', 's1'), mcq('b', 's1')])
    expect(items.map((item) => item.kind)).toEqual(['aaq', 'mcq-set'])
    if (items[0].kind === 'aaq') expect(items[0].question.id).toBe('x')
    if (items[1].kind === 'mcq-set') expect(items[1].questions.map((question) => question.id)).toEqual(['a', 'b'])
  })

  it('keeps EBQ as its own composite item', () => {
    const items = buildRenderItems([ebq('e', 's2')])
    expect(items).toEqual([{ kind: 'ebq', stimulusId: 's2', question: ebq('e', 's2') }])
  })
})
