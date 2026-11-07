import type { Node as PMNode, Schema } from '@milkdown/kit/prose/model'
import type { Parser } from '@milkdown/kit/transformer'
import { nodeFromSegment } from './build'
import { LOCKED_NODE } from './locked'
import { segmentMarkdown, type SourceSegment } from './segment'

export interface RebuiltBlock {
  node: PMNode
  /** Bytes this node should emit on compose. */
  source: string
}

/**
 * Turn a composer string into the top-level nodes it represents. A single
 * segment keeps the whole string as its source so a no-op edit is byte-identical
 * and a type change still writes exactly what the user typed.
 */
export function rebuildBlocks(text: string, schema: Schema, parser: Parser): RebuiltBlock[] {
  const { segments, prefix, trailing } = segmentMarkdown(text)
  if (segments.length === 0) {
    return [
      {
        node: schema.nodes[LOCKED_NODE].create({ source: text, kind: 'unsupported' }),
        source: text,
      },
    ]
  }
  if (segments.length === 1) {
    return [{ node: nodeFromFull(segments[0], text, schema, parser), source: text }]
  }
  const last = segments.length - 1
  return segments.map((segment, index) => {
    const source =
      index === 0 ? prefix + segment.source : index === last ? segment.source + trailing : segment.source
    return { node: nodeFromFull(segment, source, schema, parser), source }
  })
}

function nodeFromFull(segment: SourceSegment, source: string, schema: Schema, parser: Parser): PMNode {
  return nodeFromSegment(
    { ...segment, source, start: 0, end: source.length, gapBefore: '' },
    schema,
    parser,
  )
}
