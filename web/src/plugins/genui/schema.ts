export interface SourceRef {
  heading: string
}

export interface SummaryItem {
  text: string
  source?: SourceRef
}

export interface SummaryBlock {
  component: 'Summary'
  title: string
  items: SummaryItem[]
}

export interface KeyValueRow {
  key: string
  value: string
  source?: SourceRef
}

export interface KeyValueTableBlock {
  component: 'KeyValueTable'
  title: string
  rows: KeyValueRow[]
}

export interface ChecklistItem {
  label: string
  checked: boolean
  source?: SourceRef
}

export interface ChecklistBlock {
  component: 'Checklist'
  title: string
  items: ChecklistItem[]
}

export interface UnknownBlock {
  component: string
  /** Raw fixture data, kept only for display in the inline error. */
  data: unknown
}

export type GenUIBlock = SummaryBlock | KeyValueTableBlock | ChecklistBlock | UnknownBlock

export interface GenUIDocument {
  title: string
  blocks: GenUIBlock[]
}

/**
 * Parse untrusted fixture JSON into the internal schema. Anything that is not
 * one of the whitelisted components becomes an UnknownBlock; it is rendered as
 * an inline error and is never interpreted further.
 */
export function parseGenUIDocument(raw: unknown): GenUIDocument {
  const root = asRecord(raw)
  const title = typeof root.title === 'string' ? root.title : 'Generated content'
  const blocks = Array.isArray(root.blocks) ? root.blocks.map(parseBlock) : []
  return { title, blocks }
}

function parseBlock(raw: unknown): GenUIBlock {
  const block = asRecord(raw)
  const component = typeof block.component === 'string' ? block.component : ''
  switch (component) {
    case 'Summary': {
      const items = Array.isArray(block.items) ? block.items.map(parseSummaryItem).filter(isDefined) : []
      return { component, title: text(block.title, 'Summary'), items }
    }
    case 'KeyValueTable': {
      const rows = Array.isArray(block.rows) ? block.rows.map(parseKeyValueRow).filter(isDefined) : []
      return { component, title: text(block.title, 'Key values'), rows }
    }
    case 'Checklist': {
      const items = Array.isArray(block.items) ? block.items.map(parseChecklistItem).filter(isDefined) : []
      return { component, title: text(block.title, 'Checklist'), items }
    }
    default:
      return { component: component || 'unknown', data: raw }
  }
}

function parseSummaryItem(raw: unknown): SummaryItem | undefined {
  const item = asRecord(raw)
  if (typeof item.text !== 'string') return undefined
  return { text: item.text, source: parseSource(item.source) }
}

function parseKeyValueRow(raw: unknown): KeyValueRow | undefined {
  const row = asRecord(raw)
  if (typeof row.key !== 'string' || typeof row.value !== 'string') return undefined
  return { key: row.key, value: row.value, source: parseSource(row.source) }
}

function parseChecklistItem(raw: unknown): ChecklistItem | undefined {
  const item = asRecord(raw)
  if (typeof item.label !== 'string') return undefined
  return { label: item.label, checked: item.checked === true, source: parseSource(item.source) }
}

function parseSource(raw: unknown): SourceRef | undefined {
  if (!raw || typeof raw !== 'object') return undefined
  const source = asRecord(raw)
  return typeof source.heading === 'string' ? { heading: source.heading } : undefined
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? (value as Record<string, unknown>) : {}
}

function text(value: unknown, fallback: string): string {
  return typeof value === 'string' && value.trim() !== '' ? value : fallback
}

function isDefined<T>(value: T | undefined): value is T {
  return value !== undefined
}
