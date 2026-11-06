import remarkFrontmatter from 'remark-frontmatter'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import remarkParse from 'remark-parse'
import { unified } from 'unified'

/** The subset of mdast node fields the editor and heading modules rely on. */
export interface MdastNode {
  type: string
  depth?: number
  value?: string
  meta?: string | null
  lang?: string | null
  alt?: string | null
  ordered?: boolean | null
  children?: MdastNode[]
  position?: {
    start: { offset?: number }
    end: { offset?: number }
  }
}

export interface MdastRoot {
  children: MdastNode[]
}

/**
 * One parser for every consumer: block segmentation and heading extraction must
 * agree on what counts as code, HTML, frontmatter or math.
 */
const processor = unified().use(remarkParse).use(remarkGfm).use(remarkFrontmatter).use(remarkMath)

export function parseMdast(markdown: string): MdastRoot {
  return processor.parse(markdown) as unknown as MdastRoot
}

/**
 * Plain text of a node: inline formatting is dropped, raw inline HTML is
 * ignored, images contribute their alt text and hard breaks become spaces.
 */
export function plainText(node: MdastNode): string {
  if (node.type === 'html') return ''
  if (node.type === 'image') return node.alt ?? ''
  if (typeof node.value === 'string') return node.value
  if (!node.children) return ''
  let text = ''
  for (const child of node.children) {
    text += child.type === 'break' ? ' ' : plainText(child)
  }
  return text
}
