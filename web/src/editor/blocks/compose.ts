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
 * Compose Markdown from the live document.
 *
 * Unchanged top-level nodes are emitted as their original bytes (identity is
 * tracked through `origins`). Changed or brand-new nodes are serialized on
 * their own inside a throwaway doc wrapper, so a list can never merge with a
 * neighbouring paragraph and a heading never rewrites the rest of the file.
 */
export function composeDocument(doc: PMNode, ctx: ComposeContext): string {
  const nodes: PMNode[] = []
  doc.forEach((node) => {
    nodes.push(node)
  })

  if (ctx.segments.length === 0) {
    const meaningful = nodes.filter((node) => !(node.type.name === 'paragraph' && node.content.size === 0))
    if (meaningful.length === 0) return ctx.prefix + ctx.trailing
    return `${meaningful.map((node) => renderBlock(node, undefined, ctx)).join('\n\n')}\n`
  }

  if (nodes.length === 0) return ''

  let out = ''
  for (let i = 0; i < nodes.length; i += 1) {
    const node = nodes[i]
    const origin = ctx.origins.get(node)

    if (i === 0) {
      if (origin && origin.segment.index === 0) out += ctx.prefix
    } else if (origin && origin.segment.index === i) {
      out += origin.segment.gapBefore
    } else {
      out += '\n\n'
    }

    out += renderBlock(node, origin, ctx)
  }

  const last = nodes[nodes.length - 1]
  const lastOrigin = ctx.origins.get(last)
  if (ctx.segments.length === 0) return out
  out += lastOrigin && lastOrigin.segment.index === ctx.segments.length - 1 ? ctx.trailing : '\n'
  return out
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
