// Blocks of a practice paper: consecutive questions sharing a stimulus form
// one cluster (an MCQ question set, an AAQ article, an EBQ's sources);
// everything else renders as a standalone question.

export type QuestionCluster<T> = {
  key: string
  stimulusId: string | null
  items: T[]
}

export function groupQuestionClusters<T extends { stimulusId?: string | null }>(items: T[]): QuestionCluster<T>[] {
  const clusters: QuestionCluster<T>[] = []
  for (const item of items) {
    const stimulusId = item.stimulusId ?? null
    const previous = clusters[clusters.length - 1]
    // Only adjacent questions with the same non-null stimulus cluster
    // together — position in the paper is part of the grouping contract.
    if (stimulusId && previous && previous.stimulusId === stimulusId) {
      previous.items.push(item)
      continue
    }
    // Keys carry the cluster index so the same stimulus split across two
    // blocks stays unique as a React key.
    clusters.push({ key: stimulusId ? `${stimulusId}-${clusters.length}` : `solo-${clusters.length}`, stimulusId, items: [item] })
  }
  return clusters
}

// Cluster item ranges in the flat order (start index + length), aligned with
// groupQuestionClusters — used to move or render a cluster as one unit.
export function clusterRanges<T extends { stimulusId?: string | null }>(items: T[]): { start: number; length: number; stimulusId: string | null }[] {
  const ranges: { start: number; length: number; stimulusId: string | null }[] = []
  for (let i = 0; i < items.length; i++) {
    const stimulusId = items[i].stimulusId ?? null
    const previous = ranges[ranges.length - 1]
    if (stimulusId && previous && previous.stimulusId === stimulusId && previous.start + previous.length === i) {
      previous.length++
      continue
    }
    ranges.push({ start: i, length: 1, stimulusId })
  }
  return ranges
}

// A render unit of the runner: standalone question, MCQ cluster sharing a
// passage, or one AAQ/EBQ composite question next to its stimulus.
export type RenderItem<T> =
  | { kind: 'single'; question: T }
  | { kind: 'mcq-set'; stimulusId: string; questions: T[] }
  | { kind: 'aaq'; stimulusId: string; question: T }
  | { kind: 'ebq'; stimulusId: string; question: T }

export function buildRenderItems<T extends { type: string; format?: string; stimulusId?: string | null }>(items: T[]): RenderItem<T>[] {
  const out: RenderItem<T>[] = []
  for (const range of clusterRanges(items)) {
    const cluster = items.slice(range.start, range.start + range.length)
    if (!range.stimulusId) {
      for (const question of cluster) out.push({ kind: 'single', question })
      continue
    }
    const composite = cluster.find((question) => question.type !== 'mcq')
    if (composite && (composite.format === 'aaq' || composite.format === 'ebq')) {
      // AAQ/EBQ is one composite question; MCQ siblings sharing the same
      // stimulus render as their own set block right after it.
      out.push({ kind: composite.format, stimulusId: range.stimulusId, question: composite })
      const mcqs = cluster.filter((question) => question.type === 'mcq')
      if (mcqs.length > 0) out.push({ kind: 'mcq-set', stimulusId: range.stimulusId, questions: mcqs })
      continue
    }
    out.push({ kind: 'mcq-set', stimulusId: range.stimulusId, questions: cluster })
  }
  return out
}
