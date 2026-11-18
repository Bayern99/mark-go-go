import type { Node as PMNode, Schema } from '@milkdown/kit/prose/model'
import type { Parser } from '@milkdown/kit/transformer'
import { LOCKED_NODE } from './locked'
import { segmentMarkdown, type LockKind, type SourceSegment } from './segment'

export interface BlockOrigin {
  segment: SourceSegment
  source: string
  node: PMNode
}

export interface BuiltDocument {
  doc: PMNode
  origins: WeakMap<PMNode, BlockOrigin>
  loaded: BlockOrigin[]
  segments: SourceSegment[]
  prefix: string
  trailing: string
}

export function buildDocument(text: string, schema: Schema, parser: Parser): BuiltDocument {
  const { segments, prefix, trailing } = segmentMarkdown(text)
  const origins = new WeakMap<PMNode, BlockOrigin>()
  const loaded: BlockOrigin[] = []
  const nodes: PMNode[] = []
  for (const segment of segments) {
    const node = nodeFromSegment(segment, schema, parser)
    nodes.push(node)
    const origin = { segment, source: segment.source, node }
    origins.set(node, origin)
    loaded.push(origin)
  }

  if (nodes.length === 0) {
    nodes.push(schema.nodes.paragraph.create())
  }

  const doc = schema.nodes.doc.create(null, nodes)
  return { doc, origins, loaded, segments, prefix, trailing }
}

export function nodeFromSegment(segment: SourceSegment, schema: Schema, parser: Parser): PMNode {
  let lock: LockKind | null = segment.lock
  let node: PMNode | null = null

  if (lock === null) {
    const parsed = tryParse(parser, segment.source)
    if (parsed && parsed.childCount === 1) {
      node = parsed.child(0)
    } else {
      lock = 'unsupported'
    }
  }

  if (lock !== null || node === null) {
    node = schema.nodes[LOCKED_NODE].create({ source: segment.source, kind: lock ?? 'unsupported' })
  }

  return node
}

function tryParse(parser: Parser, source: string): PMNode | null {
  try {
    return parser(source.endsWith('\n') ? source : `${source}\n`)
  } catch {
    return null
  }
}
