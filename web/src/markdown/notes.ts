import type { MdastRoot } from './mdast'
import { parseMdast } from './mdast'

const NOTE_BLOCK =
  /^([ \t]*<!--[ \t]*note(?:\s+[A-Za-z_][\w-]*="(?:(?!-->)[^"])*")*\s*:)((?:(?!-->)[\s\S])*)-->\s*$/

export interface ParsedNote {
  head: string
  text: string
}

export function parseNote(source: string): ParsedNote | null {
  const match = NOTE_BLOCK.exec(source)
  if (!match) return null
  return { head: match[1], text: unescapeNoteText(match[2].trim()) }
}

export function noteBlock(text: string): string {
  return `<!-- note: ${escapeNoteText(text.trim())} -->`
}

export function withNoteText(source: string, text: string): string {
  const head = parseNote(source)?.head ?? '<!-- note:'
  return `${head} ${escapeNoteText(text.trim())} -->`
}

function escapeNoteText(text: string): string {
  return text.replace(/--(\\*)(!?)>/g, '--\\$1$2>')
}

function unescapeNoteText(text: string): string {
  return text.replace(/--\\(\\*)(!?)>/g, '--$1$2>')
}

export interface MarkdownNote {
  start: number
  end: number
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
