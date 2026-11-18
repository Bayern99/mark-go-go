import { parseMdast, plainText, type MdastNode, type MdastRoot } from './mdast'

export interface MarkdownHeading {
  level: number
  text: string
  offset: number
}

export function extractHeadings(markdown: string): MarkdownHeading[] {
  return headingsIn(parseMdast(markdown))
}

export function headingsIn(tree: MdastRoot): MarkdownHeading[] {
  const headings: MarkdownHeading[] = []
  collectHeadings(tree.children, headings)
  return headings
}

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
