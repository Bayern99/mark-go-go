import { parseMdast, plainText, type MdastNode, type MdastRoot } from './mdast'

export interface MarkdownHeading {
  /** Heading level, 1–6. */
  level: number
  /** Plain text with inline formatting removed. */
  text: string
  /** UTF-16 offset of the heading in the Markdown source. */
  offset: number
}

/**
 * Every real heading in the document, in source order. Parsing is shared with
 * block segmentation, so ATX and setext headings count while headings inside
 * fences, raw HTML and frontmatter do not.
 */
export function extractHeadings(markdown: string): MarkdownHeading[] {
  return headingsIn(parseMdast(markdown))
}

export function headingsIn(tree: MdastRoot): MarkdownHeading[] {
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
