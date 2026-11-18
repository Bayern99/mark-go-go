import { headingsIn, normalizeHeadingText, type MarkdownHeading } from './headings'
import { parseMdast } from './mdast'
import { notesIn, type MarkdownNote } from './notes'

export interface MarkdownSection {
  heading: MarkdownHeading
  occurrence: number
  start: number
  end: number
  notes: MarkdownNote[]
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

export function sectionHash(text: string): string {
  let hash = FNV_OFFSET_BASIS
  for (const byte of encoder.encode(text)) {
    hash ^= byte
    hash = Math.imul(hash, FNV_PRIME) >>> 0
  }
  return `fnv1a32:${hash.toString(16).padStart(8, '0')}`
}
