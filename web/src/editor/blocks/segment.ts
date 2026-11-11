import { parseMdast, plainText, type MdastNode } from '../../markdown/mdast'
import { parseNote } from '../../markdown/notes'

/**
 * Kinds of top-level blocks that are kept as raw, read-only source instead of
 * being handed to ProseMirror. They either cannot be represented by the
 * Milkdown schema or cannot be serialized back without changing their bytes.
 */
export type LockKind =
  | 'yaml-frontmatter'
  | 'github-alert'
  | 'footnote-definition'
  | 'link-reference-definition'
  | 'code-fence-meta'
  | 'mermaid'
  | 'raw-html'
  | 'note'
  | 'math'
  | 'unsupported'

export interface SourceSegment {
  index: number
  start: number
  end: number
  /** Raw text between the previous block and this one (blank lines, CRLF, ...). */
  gapBefore: string
  /** Raw bytes of the block itself, without the trailing gap. */
  source: string
  /** Non-null when the block must be kept as a locked atom. */
  lock: LockKind | null
}

export interface Segmentation {
  segments: SourceSegment[]
  /** Text before the first block. */
  prefix: string
  /** Text after the last block (usually the final newline). */
  trailing: string
}

const ALERT_RE = /^\[!(?:note|tip|important|warning|caution)\]/i
const FRONTMATTER_TYPES: Record<string, true> = { yaml: true, toml: true, frontmatter: true }

/**
 * Split a Markdown document into its top-level blocks using the source
 * positions reported by remark. Gaps are sliced from the original text, so
 * CRLF and blank-line runs survive untouched.
 */
export function segmentMarkdown(text: string): Segmentation {
  const tree = parseMdast(text)
  const segments: SourceSegment[] = []

  let previousEnd = 0
  for (const node of tree.children) {
    const start = node.position?.start.offset
    const end = node.position?.end.offset
    if (typeof start !== 'number' || typeof end !== 'number' || end < start) {
      // Defensive: if remark ever yields a node without positions, keep the
      // whole document as a single locked block rather than losing bytes.
      return {
        segments: [
          {
            index: 0,
            start: 0,
            end: text.length,
            gapBefore: '',
            source: text,
            lock: 'unsupported',
          },
        ],
        prefix: '',
        trailing: '',
      }
    }
    segments.push({
      index: segments.length,
      start,
      end,
      gapBefore: text.slice(previousEnd, start),
      source: text.slice(start, end),
      lock: classifyMdast(node),
    })
    previousEnd = end
  }

  if (segments.length === 0) {
    // Empty or whitespace-only documents have no block to anchor around. The
    // text stays a prefix, so it remains byte-exact if the user types the first
    // block into it (the original bytes stay before that block).
    return { segments, prefix: text, trailing: '' }
  }

  return {
    segments,
    prefix: segments[0].gapBefore,
    trailing: text.slice(segments[segments.length - 1].end),
  }
}

/** Classify a top-level mdast node; `null` means ProseMirror may own it. */
export function classifyMdast(node: MdastNode): LockKind | null {
  if (FRONTMATTER_TYPES[node.type] === true) return 'yaml-frontmatter'
  if (node.type === 'footnoteDefinition') return 'footnote-definition'
  if (node.type === 'definition') return 'link-reference-definition'
  if (node.type === 'math') return 'math'
  if (node.type === 'html') return parseNote(node.value ?? '') ? 'note' : 'raw-html'
  if (node.type === 'code' && (node.lang ?? '').trim().toLowerCase() === 'mermaid') {
    return 'mermaid'
  }
  if (node.type === 'code' && typeof node.meta === 'string' && node.meta.trim() !== '') {
    return 'code-fence-meta'
  }
  if (node.type === 'blockquote' && isGithubAlert(node)) return 'github-alert'
  return null
}

function isGithubAlert(node: MdastNode): boolean {
  const first = node.children?.[0]
  if (!first || first.type !== 'paragraph') return false
  return ALERT_RE.test(plainText(first).trimStart())
}
