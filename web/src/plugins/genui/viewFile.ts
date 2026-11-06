import { isRecord } from '../../lib/guards'

/**
 * The view file's own envelope around its A2UI messages (see
 * `view-format.md`, the one description of the format). Parsing never
 * throws: a file that cannot be used becomes one message for the reader.
 */
export const VIEW_VERSION = 2

export interface SourceRef {
  /** The heading's plain text as it reads in the document. */
  heading: string
  /** Which heading with that text, from 1 in document order. */
  occurrence: number
  /** `fnv1a32:` hash of the section when the view was made; absent in hand-written files. */
  hash?: string
}

export interface ViewFile {
  generatedAt?: string
  model?: string
  request?: string
  sources: Record<string, SourceRef>
  /** A2UI 1.0 messages, checked only when they are applied. */
  messages: unknown[]
}

export type ParsedViewFile = { ok: true; view: ViewFile } | { ok: false; error: string }

export const OLD_VIEW_MESSAGE = 'This view was made by an older version of Mark Go Go. Generate it again.'

export function parseViewFile(text: string): ParsedViewFile {
  let raw: unknown
  try {
    raw = JSON.parse(text)
  } catch {
    return { ok: false, error: 'The view file is not valid JSON.' }
  }
  if (!isRecord(raw)) return { ok: false, error: 'The view file must be a JSON object.' }
  if (raw.version === 1) return { ok: false, error: OLD_VIEW_MESSAGE }
  if (raw.version !== VIEW_VERSION) {
    return {
      ok: false,
      error: `The view file has version ${JSON.stringify(raw.version) ?? 'none'}; this app reads version ${VIEW_VERSION}.`,
    }
  }
  if (!Array.isArray(raw.a2ui) || raw.a2ui.length === 0) {
    return { ok: false, error: 'The view file needs an "a2ui" list of A2UI messages.' }
  }
  const sources = parseSources(raw.sources)
  if (typeof sources === 'string') return { ok: false, error: sources }
  return {
    ok: true,
    view: {
      generatedAt: optionalString(raw.generatedAt),
      model: optionalString(raw.model),
      request: optionalString(raw.request),
      sources,
      messages: raw.a2ui,
    },
  }
}

function parseSources(value: unknown): Record<string, SourceRef> | string {
  if (value === undefined) return {}
  if (!isRecord(value)) return '"sources" must be an object of source references.'
  const sources: Record<string, SourceRef> = {}
  for (const [id, entry] of Object.entries(value)) {
    if (!isRecord(entry) || typeof entry.heading !== 'string' || entry.heading.trim() === '') {
      return `Source "${id}" needs a "heading" string.`
    }
    const occurrence = entry.occurrence ?? 1
    if (!Number.isInteger(occurrence) || (occurrence as number) < 1) {
      return `Source "${id}": "occurrence" must be a whole number from 1.`
    }
    if (entry.hash !== undefined && typeof entry.hash !== 'string') {
      return `Source "${id}": "hash" must be a string.`
    }
    sources[id] = { heading: entry.heading, occurrence: occurrence as number, hash: entry.hash }
  }
  return sources
}

function optionalString(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined
}
