import { describe, expect, it } from 'vitest'
import { clusterRanges, groupQuestionClusters } from './questionGrouping'

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
