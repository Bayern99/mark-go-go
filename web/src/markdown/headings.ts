import remarkFrontmatter from 'remark-frontmatter'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import remarkParse from 'remark-parse'
import { unified } from 'unified'

export interface MarkdownHeading {
  /** Heading level, 1–6. */
  level: number
  /** Plain text with inline formatting removed. */
  text: string
  /** UTF-16 offset of the heading in the Markdown source. */
  offset: number
}

interface MdastNode {
  type: string
  depth?: number
  alt?: string | null
  value?: string
  children?: MdastNode[]
  position?: {
    start: { offset?: number }
  }
}

interface MdastRoot {
  children: MdastNode[]
}

/**
 * Same remark stack the block segmentation uses, so the outline and GenUI see
 * exactly the headings the editor sees: ATX and setext count, while headings
 * inside fences, raw HTML and frontmatter do not.
 */
const processor = unified().use(remarkParse).use(remarkGfm).use(remarkFrontmatter).use(remarkMath)

/** Every real heading in the document, in source order. */
export function extractHeadings(markdown: string): MarkdownHeading[] {
  const tree = processor.parse(markdown) as unknown as MdastRoot
  const headings: MarkdownHeading[] = []
  collectHeadings(tree.children, headings)
  return headings
}

/** Comparison form for heading text: inline formatting already stripped. */
export function normalizeHeadingText(text: string): string {
  return text.replace(/\s+/g, ' ').trim().toLowerCase()
}

function collectHeadings(nodes: MdastNode[], out: MarkdownHeading[]): void {
  for (const node of nodes) {
    if (node.type === 'heading') {
      const offset = node.position?.start.offset
      if (typeof offset === 'number') {
        out.push({ level: node.depth ?? 1, text: plainText(node).trim(), offset })
      }
      continue
    }
    if (node.children) collectHeadings(node.children, out)
  }
}

function plainText(node: MdastNode): string {
  if (node.type === 'image') return node.alt ?? ''
  if (typeof node.value === 'string') return node.value
  if (!node.children) return ''
  let text = ''
  for (const child of node.children) {
    text += child.type === 'break' ? ' ' : plainText(child)
  }
  return text
}
