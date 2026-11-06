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
  const nodes: PMNode[] = []

  for (const segment of segments) {
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

    nodes.push(node)
    origins.set(node, { segment, source: segment.source, node })
  }

  if (nodes.length === 0) {
    // ProseMirror needs a textblock to put the initial selection in; an
    // empty paragraph is invisible to compose() as long as it stays empty.
    nodes.push(schema.nodes.paragraph.create())
  }

  const doc = schema.nodes.doc.create(null, nodes)
  return { doc, origins, segments, prefix, trailing }
}

function tryParse(parser: Parser, source: string): PMNode | null {
  try {
    return parser(source.endsWith('\n') ? source : `${source}\n`)
  } catch {
    return null
  }
}
