import { isRecord } from '../../lib/guards'

/**
 * The view file schema (see `view-format.md`, the single description of the
 * format). Parsing never throws on untrusted input: a file that is unusable as
 * a whole becomes one error, and anything wrong inside a section becomes a
 * problem of that section while the rest still renders.
 */

export interface SummaryBlock {
  component: 'Summary'
  title: string
  items: Array<{ text: string }>
}

export interface KeyValueTableBlock {
  component: 'KeyValueTable'
  title: string
  rows: Array<{ key: string; value: string }>
}

export interface ChecklistBlock {
  component: 'Checklist'
  title: string
  items: Array<{ label: string; checked: boolean }>
}

export type GenUIBlock = SummaryBlock | KeyValueTableBlock | ChecklistBlock

export interface ViewSection {
  /** Heading text as written in the view file; empty when it had none. */
  heading: string
  /** Which section of that heading, from 1; 0 when the file's value was invalid. */
  occurrence: number
  sourceHash?: string
  blocks: GenUIBlock[]
  /** What was wrong in this section; the blocks that were fine are kept. */
  problems: string[]
}

export interface GenUIView {
  generatedAt?: string
  model?: string
  sections: ViewSection[]
}

export type ParsedView = { ok: true; view: GenUIView } | { ok: false; error: string }

export const VIEW_VERSION = 1

export function parseViewFile(text: string): ParsedView {
  let raw: unknown
  try {
    raw = JSON.parse(text)
  } catch (error) {
    return { ok: false, error: `The view file is not valid JSON: ${(error as Error).message}` }
  }
  if (!isRecord(raw)) return { ok: false, error: 'The view file must be a JSON object.' }
  if (raw.version !== VIEW_VERSION) {
    return {
      ok: false,
      error: `The view file has version ${JSON.stringify(raw.version) ?? 'none'}; this app reads version ${VIEW_VERSION}.`,
    }
  }
  if (!Array.isArray(raw.sections)) {
    return { ok: false, error: 'The view file has no "sections" array.' }
  }
  return {
    ok: true,
    view: {
      generatedAt: typeof raw.generatedAt === 'string' ? raw.generatedAt : undefined,
      model: typeof raw.model === 'string' && raw.model !== '' ? raw.model : undefined,
      sections: raw.sections.map(parseSection),
    },
  }
}

function parseSection(raw: unknown): ViewSection {
  const problems: string[] = []
  if (!isRecord(raw)) {
    return { heading: '', occurrence: 0, blocks: [], problems: ['The section is not an object.'] }
  }

  let heading = ''
  if (typeof raw.heading === 'string' && raw.heading.trim() !== '') heading = raw.heading
  else problems.push('The section has no "heading" text.')

  let occurrence = 1
  if (raw.occurrence !== undefined) {
    if (Number.isInteger(raw.occurrence) && (raw.occurrence as number) >= 1) {
      occurrence = raw.occurrence as number
    } else {
      occurrence = 0
      problems.push('"occurrence" must be a whole number from 1.')
    }
  }

  let sourceHash: string | undefined
  if (raw.sourceHash !== undefined) {
    if (typeof raw.sourceHash === 'string' && raw.sourceHash !== '') sourceHash = raw.sourceHash
    else problems.push('"sourceHash" must be a non-empty string.')
  }

  const blocks: GenUIBlock[] = []
  if (Array.isArray(raw.blocks)) {
    raw.blocks.forEach((block, index) => {
      const parsed = parseBlock(block, `Block ${index + 1}`, problems)
      if (parsed) blocks.push(parsed)
    })
  } else {
    problems.push('The section has no "blocks" array.')
  }

  return { heading, occurrence, sourceHash, blocks, problems }
}

function parseBlock(raw: unknown, where: string, problems: string[]): GenUIBlock | undefined {
  if (!isRecord(raw)) {
    problems.push(`${where} is not an object.`)
    return undefined
  }
  const component = raw.component
  if (typeof component !== 'string' || component === '') {
    problems.push(`${where} has no "component".`)
    return undefined
  }
  const label = `${where} (${component})`
  switch (component) {
    case 'Summary': {
      const items = list(raw.items, 'items', label, problems, (item) =>
        typeof item.text === 'string' ? { text: item.text } : 'needs a "text" string',
      )
      return items && { component, title: title(raw.title, 'Summary', label, problems), items }
    }
    case 'KeyValueTable': {
      const rows = list(raw.rows, 'rows', label, problems, (row) =>
        typeof row.key === 'string' && typeof row.value === 'string'
          ? { key: row.key, value: row.value }
          : 'needs "key" and "value" strings',
      )
      return rows && { component, title: title(raw.title, 'Key values', label, problems), rows }
    }
    case 'Checklist': {
      const items = list(raw.items, 'items', label, problems, (item) => {
        if (typeof item.label !== 'string') return 'needs a "label" string'
        if (item.checked !== undefined && typeof item.checked !== 'boolean') {
          return '"checked" must be true or false'
        }
        return { label: item.label, checked: item.checked === true }
      })
      return items && { component, title: title(raw.title, 'Checklist', label, problems), items }
    }
    default:
      problems.push(`${where}: "${component}" is not in the whitelist, so it was not rendered.`)
      return undefined
  }
}

function title(value: unknown, fallback: string, where: string, problems: string[]): string {
  if (value === undefined) return fallback
  if (typeof value === 'string') return value.trim() === '' ? fallback : value
  problems.push(`${where}: "title" must be a string.`)
  return fallback
}

/**
 * The valid entries of a block's list. `parse` returns the entry or why it is
 * not one; invalid entries are left out and reported with their position.
 * Without a list there is no block to show: undefined, and a problem.
 */
function list<T>(
  value: unknown,
  field: string,
  where: string,
  problems: string[],
  parse: (entry: Record<string, unknown>) => T | string,
): T[] | undefined {
  if (!Array.isArray(value)) {
    problems.push(`${where}: "${field}" must be an array.`)
    return undefined
  }
  const entries: T[] = []
  value.forEach((entry, index) => {
    const parsed = isRecord(entry) ? parse(entry) : 'is not an object'
    if (typeof parsed === 'string') problems.push(`${where}: ${field} ${index + 1} ${parsed}.`)
    else entries.push(parsed)
  })
  return entries
}
