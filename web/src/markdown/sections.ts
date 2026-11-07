import { headingsIn, normalizeHeadingText, type MarkdownHeading } from './headings'
import { parseMdast } from './mdast'
import { notesIn, type MarkdownNote } from './notes'

/**
 * A heading and the source text it owns: from the heading's own offset up to
 * the next heading of the same or a higher level (a smaller or equal depth),
 * or the end of the document. Sub-sections are part of their parent.
 */
export interface MarkdownSection {
  heading: MarkdownHeading
  /** 1-based rank among headings with the same normalized text, in document order. */
  occurrence: number
  start: number
  end: number
  /** The notes in the section, in document order. */
  notes: MarkdownNote[]
  /** `sectionHash` of `markdown.slice(start, end)` with its notes cut out: notes never count as a change. */
  hash: string
}

export function extractSections(markdown: string): MarkdownSection[] {
  const tree = parseMdast(markdown)
  const headings = headingsIn(tree)
  const allNotes = notesIn(tree)
  const seen = new Map<string, number>()
  return headings.map((heading, index) => {
    const key = normalizeHeadingText(heading.text)
    const occurrence = (seen.get(key) ?? 0) + 1
    seen.set(key, occurrence)

    let end = markdown.length
    for (const next of headings.slice(index + 1)) {
      if (next.level <= heading.level) {
        end = next.offset
        break
      }
    }
    const start = heading.offset
    const notes = allNotes.filter((note) => note.start >= start && note.end <= end)
    return { heading, occurrence, start, end, notes, hash: sectionHash(withoutNotes(markdown, start, end, notes)) }
  })
}

function withoutNotes(markdown: string, start: number, end: number, notes: readonly MarkdownNote[]): string {
  let text = ''
  let at = start
  for (const note of notes) {
    text += markdown.slice(at, note.cutStart)
    at = note.end
  }
  return text + markdown.slice(at, end)
}

/** The section a view refers to by heading text and occurrence, if it still exists. */
export function findSection(
  sections: readonly MarkdownSection[],
  heading: string,
  occurrence: number,
): MarkdownSection | undefined {
  const key = normalizeHeadingText(heading)
  if (key === '') return undefined
  return sections.find(
    (section) =>
      section.occurrence === occurrence && normalizeHeadingText(section.heading.text) === key,
  )
}

const FNV_OFFSET_BASIS = 0x811c9dc5
const FNV_PRIME = 0x01000193
const encoder = new TextEncoder()

/**
 * `fnv1a32:` + 8 lowercase hex digits of the 32-bit FNV-1a hash of the text's
 * UTF-8 bytes. It only tells whether a section changed since a view was made,
 * so a small, synchronous, dependency-free hash is enough; Go's `hash/fnv`
 * `New32a` computes the same value.
 */
export function sectionHash(text: string): string {
  let hash = FNV_OFFSET_BASIS
  for (const byte of encoder.encode(text)) {
    hash ^= byte
    hash = Math.imul(hash, FNV_PRIME) >>> 0
  }
  return `fnv1a32:${hash.toString(16).padStart(8, '0')}`
}
