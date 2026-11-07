import type { MdastRoot } from './mdast'
import { parseMdast } from './mdast'

/**
 * A note is a whole top-level HTML comment block:
 * `<!-- note key="value": text -->`. The attributes are optional and kept as
 * they are; a comment with anything else on its line is not a note.
 */
const NOTE_BLOCK =
  /^([ \t]*<!--[ \t]*note(?:\s+[A-Za-z_][\w-]*="(?:(?!-->)[^"])*")*\s*:)((?:(?!-->)[\s\S])*)-->\s*$/

export interface ParsedNote {
  /** Everything up to and including the colon, attributes as written. */
  head: string
  text: string
}

/** The note a block's source is, or null when it is anything else. */
export function parseNote(source: string): ParsedNote | null {
  const match = NOTE_BLOCK.exec(source)
  if (!match) return null
  return { head: match[1], text: unescapeNoteText(match[2].trim()) }
}

/** The source of a new note. */
export function noteBlock(text: string): string {
  return `<!-- note: ${escapeNoteText(text.trim())} -->`
}

/** `source`, a note, with its text replaced and everything before the text kept. */
export function withNoteText(source: string, text: string): string {
  const head = parseNote(source)?.head ?? '<!-- note:'
  return `${head} ${escapeNoteText(text.trim())} -->`
}

/**
 * Comments end at `-->`, and browsers end them at `--!>` too. Either one in
 * the text gets a backslash before its `>` or `!`; one that already has
 * backslashes there gets another, so reading it back removes exactly one.
 */
function escapeNoteText(text: string): string {
  return text.replace(/--(\\*)(!?)>/g, '--\\$1$2>')
}

function unescapeNoteText(text: string): string {
  return text.replace(/--\\(\\*)(!?)>/g, '--$1$2>')
}

/** A note where it is in the Markdown text, in UTF-16 offsets. */
export interface MarkdownNote {
  start: number
  end: number
  /**
   * Where the note's own bytes begin once it counts as absent: the end of the
   * block before it, so that the gap it was given goes with it and the gap
   * after it stays, which is the text as it read before the note was added.
   */
  cutStart: number
  text: string
}

export function notesIn(tree: MdastRoot): MarkdownNote[] {
  const notes: MarkdownNote[] = []
  let previousEnd: number | undefined
  for (const node of tree.children) {
    const start = node.position?.start.offset
    const end = node.position?.end.offset
    if (typeof start !== 'number' || typeof end !== 'number') continue
    const note = node.type === 'html' ? parseNote(node.value ?? '') : null
    if (note) notes.push({ start, end, cutStart: previousEnd ?? start, text: note.text })
    previousEnd = end
  }
  return notes
}

/**
 * Where a note goes so that it ends the text before `offset`: right after the
 * last top-level block that starts before it. Undefined without such a block.
 */
export function noteInsertionOffset(markdown: string, offset: number): number | undefined {
  let at: number | undefined
  for (const node of parseMdast(markdown).children) {
    const start = node.position?.start.offset
    const end = node.position?.end.offset
    if (typeof start !== 'number' || typeof end !== 'number' || start >= offset) break
    at = end
  }
  return at
}
