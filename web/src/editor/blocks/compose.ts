import type { Node as PMNode, Schema } from '@milkdown/kit/prose/model'
import type { Serializer } from '@milkdown/kit/transformer'
import type { BlockOrigin, BuiltDocument } from './build'
import { LOCKED_NODE } from './locked'

export interface ComposeContext {
  schema: Schema
  serializer: Serializer
  origins: WeakMap<PMNode, BlockOrigin>
  segments: BuiltDocument['segments']
  prefix: string
  trailing: string
}

/**
 * Where a top-level node's bytes landed in the composed Markdown. Offsets are
 * UTF-16 offsets into the composed text.
 */
export interface ComposedBlock {
  node: PMNode
  start: number
  end: number
}

export interface ComposedDocument {
  text: string
  blocks: ComposedBlock[]
}

/**
 * Compose Markdown from the live document.
 *
 * Unchanged top-level nodes are emitted as their original bytes (identity is
 * tracked through `origins`). Changed or brand-new nodes are serialized on
 * their own inside a throwaway doc wrapper, so a list can never merge with a
 * neighbouring paragraph and a heading never rewrites the rest of the file.
 */
export function composeDocument(doc: PMNode, ctx: ComposeContext): string {
  return composeWithRanges(doc, ctx).text
}

/**
 * Compose the document and report the source range of every top-level block in
 * the composed text. Ranges are derived from the current text on every call, so
 * edits and insertions elsewhere in the document move them correctly.
 */
export function composeWithRanges(doc: PMNode, ctx: ComposeContext): ComposedDocument {
  const nodes: PMNode[] = []
  doc.forEach((node) => {
    nodes.push(node)
  })

  if (ctx.segments.length === 0) {
    const meaningful = nodes.filter((node) => !(node.type.name === 'paragraph' && node.content.size === 0))
    if (meaningful.length === 0) return { text: ctx.prefix + ctx.trailing, blocks: [] }

    // The document was empty or whitespace-only when loaded: its original text
    // is a prefix and must stay before the blocks the user just created.
    let text = ctx.prefix
    const blocks: ComposedBlock[] = []
    for (let i = 0; i < meaningful.length; i += 1) {
      if (i > 0) text += '\n\n'
      const node = meaningful[i]
      const start = text.length
      text += renderBlock(node, undefined, ctx)
      blocks.push({ node, start, end: text.length })
    }
    return { text: text + (ctx.trailing === '' ? '\n' : ctx.trailing), blocks }
  }

  if (nodes.length === 0) return { text: '', blocks: [] }

  let out = ''
  const blocks: ComposedBlock[] = []
  for (let i = 0; i < nodes.length; i += 1) {
    const node = nodes[i]
    const origin = ctx.origins.get(node)

    if (i === 0) {
      if (origin && origin.segment.index === 0) out += ctx.prefix
    } else {
      out += gapBefore(origin, i)
    }

    const start = out.length
    out += renderBlock(node, origin, ctx)
    blocks.push({ node, start, end: out.length })
  }

  out += endsAsLoaded(nodes, ctx) ? ctx.trailing : '\n'
  return { text: out, blocks }
}

const BLANK_LINE = /(?:\r\n?|\n)[ \t]*(?:\r\n?|\n)/

/**
 * The bytes before a block at top-level index `index`. A block keeps its own
 * gap, also when blocks were inserted or removed above it, so such an edit
 * leaves the rest of the file as it was. Away from its old place it needs a
 * blank line there, or it could merge into the block now before it.
 */
function gapBefore(origin: BlockOrigin | undefined, index: number): string {
  if (!origin) return '\n\n'
  const { gapBefore: gap, index: loadedAt } = origin.segment
  if (loadedAt === index) return gap
  return loadedAt > 0 && BLANK_LINE.test(gap) ? gap : '\n\n'
}

/**
 * Whether the file still ends with the block it ended with when loaded, so
 * its trailing bytes stay. Notes do not count: adding or removing one at the
 * end of the file changes nothing else.
 */
function endsAsLoaded(nodes: readonly PMNode[], ctx: ComposeContext): boolean {
  const lastNode = nodes.findLast((node) => !isNoteNode(node))
  const lastSegment = ctx.segments.findLast((segment) => segment.lock !== 'note')
  if (!lastNode || !lastSegment) return true
  return ctx.origins.get(lastNode)?.segment.index === lastSegment.index
}

function isNoteNode(node: PMNode): boolean {
  return node.type.name === LOCKED_NODE && node.attrs.kind === 'note'
}

/**
 * The block an offset belongs to. A block owns its own text plus its end
 * boundary, so an offset at `end` stays in that block; only offsets strictly
 * inside the gap between two blocks resolve to the following block. Offsets
 * past the trailing text resolve to the last block.
 */
export function blockAtOffset(blocks: ComposedBlock[], offset: number): ComposedBlock | undefined {
  for (const block of blocks) {
    if (offset >= block.start && offset <= block.end) return block
  }
  for (const block of blocks) {
    if (block.start > offset) return block
  }
  return blocks[blocks.length - 1]
}

function renderBlock(node: PMNode, origin: BlockOrigin | undefined, ctx: ComposeContext): string {
  if (node.type.name === LOCKED_NODE) {
    return node.attrs.source as string
  }
  if (origin && origin.node === node) {
    return origin.source
  }
  const wrapper = ctx.schema.nodes.doc.create(null, node)
  return ctx.serializer(wrapper).replace(/\r?\n+$/, '')
}
