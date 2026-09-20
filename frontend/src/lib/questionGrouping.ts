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
