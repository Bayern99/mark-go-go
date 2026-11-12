import type { Node as PMNode, Schema } from '@milkdown/kit/prose/model'
import type { Parser } from '@milkdown/kit/transformer'
import { LOCKED_NODE } from './locked'
import { segmentMarkdown, type LockKind, type SourceSegment } from './segment'

export interface BlockOrigin {
  segment: SourceSegment
  /** Raw source of the block at load time. */
  source: string
  /** The exact ProseMirror node that was built from this segment. */
  node: PMNode
}

export interface BuiltDocument {
  doc: PMNode
  /** Identity map: a node is unchanged iff `origins.get(node)?.node === node`. */
  origins: WeakMap<PMNode, BlockOrigin>
  /**
   * Origins in load order. Anything that re-creates a top-level node (undo,
   * for example) produces an object the identity map misses; a structurally
   * equal match here still proves the original bytes render it losslessly.
   */
  loaded: BlockOrigin[]
  segments: SourceSegment[]
  prefix: string
  trailing: string
}

/**
 * Build the initial ProseMirror document from per-block parses.
 *
 * Every segment is parsed on its own so that ProseMirror never sees the whole
 * document: that keeps blocks independent (a list cannot merge with the
 * paragraph above it) and lets us map each resulting node back to its exact
 * source bytes.
 */
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
    // ProseMirror needs a textblock to put the initial selection in; an
    // empty paragraph is invisible to compose() as long as it stays empty.
    nodes.push(schema.nodes.paragraph.create())
  }

  const doc = schema.nodes.doc.create(null, nodes)
  return { doc, origins, loaded, segments, prefix, trailing }
}

/** Parse one source segment into a top-level ProseMirror node. */
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
