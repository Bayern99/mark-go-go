import { languages } from '../languages'

export type AlertKind = 'note' | 'tip' | 'important' | 'warning' | 'caution'

export interface AlertPresentation {
  kind: AlertKind
  label: string
  body: string
}

export interface FrontmatterRow {
  key: string
  value: string
}

export interface CodeFencePresentation {
  language: string
  meta: string
  code: string
}

export interface FootnotePresentation {
  label: string
  body: string
}

const ALERT_LINE = /^\[!(note|tip|important|warning|caution)\]\s*(.*)$/i
const ALERT_LABEL: Record<AlertKind, string> = {
  note: 'Note',
  tip: 'Tip',
  important: 'Important',
  warning: 'Warning',
  caution: 'Caution',
}
const YAML_PAIR = /^([A-Za-z_][\w-]*)\s*:\s*(.*)$/
const FENCE_OPEN = /^( {0,3})(`{3,}|~{3,})([^\n]*)\n/

function asLf(source: string): string {
  return source.replace(/\r\n?/g, '\n')
}

export function parseAlert(source: string): AlertPresentation | null {
  const lines = asLf(source).split('\n')
  let kind: AlertKind | undefined
  const body: string[] = []
  for (const raw of lines) {
    const line = raw.replace(/^ {0,3}> ?/, '')
    if (kind === undefined) {
      const match = ALERT_LINE.exec(line.trimStart())
      if (!match) return null
      kind = match[1].toLowerCase() as AlertKind
      if (match[2]) body.push(match[2])
      continue
    }
    body.push(line)
  }
  if (!kind) return null
  return { kind, label: ALERT_LABEL[kind], body: body.join('\n').trim() }
}

export function parseFrontmatterRows(source: string): FrontmatterRow[] {
  const text = asLf(source)
  const yaml = text.match(/^---(?:toml)?\s*\n([\s\S]*?)\n(?:---|\.{3})\s*$/i)
  const toml = yaml ? null : text.match(/^\+\+\+\s*\n([\s\S]*?)\n\+\+\+\s*$/)
  const inner = yaml?.[1] ?? toml?.[1]
  if (inner === undefined) return []
  const rows: FrontmatterRow[] = []
  let current: FrontmatterRow | undefined
  for (const line of inner.split('\n')) {
    if (/^\s*(#|;)/.test(line) && !current) continue
    const pair = YAML_PAIR.exec(line)
    if (pair && !/^\s/.test(line)) {
      current = { key: pair[1], value: unwrapScalar(pair[2]) }
      rows.push(current)
      continue
    }
    if (current && /^\s+\S/.test(line)) {
      const extra = line.trim()
      current.value = current.value ? `${current.value}\n${extra}` : extra
      continue
    }
    if (line.trim() === '') continue
    current = undefined
  }
  return rows
}

function unwrapScalar(value: string): string {
  const trimmed = value.trim()
  if (
    (trimmed.startsWith('"') && trimmed.endsWith('"') && trimmed.length >= 2) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'") && trimmed.length >= 2)
  ) {
    return trimmed.slice(1, -1)
  }
  return trimmed
}

export function parseCodeFence(source: string): CodeFencePresentation | null {
  const text = asLf(source)
  const match = FENCE_OPEN.exec(text)
  if (!match) return null
  const indent = match[1]
  const marker = match[2]
  const info = match[3].trim()
  const close = new RegExp('\\n' + indent + marker[0] + '{' + String(marker.length) + ',}[ \\t]*$')
  const closed = close.exec(text)
  const body = closed ? text.slice(match[0].length, closed.index) : text.slice(match[0].length)
  const [language = '', ...rest] = info.split(/\s+/)
  return { language, meta: rest.join(' '), code: body }
}

export function parseFootnote(source: string): FootnotePresentation | null {
  const match = asLf(source).match(/^\[\^([^\]]+)\]:\s*([\s\S]*)$/)
  if (!match) return null
  return {
    label: match[1],
    body: match[2].replace(/\n[ \t]+/g, '\n').replace(/\s+$/, ''),
  }
}

export function codeLanguageLabel(language: string): string {
  if (!language) return ''
  const known = languages.find(
    (item) =>
      item.name.toLowerCase() === language.toLowerCase() || item.alias.includes(language.toLowerCase()),
  )
  return known?.name ?? language
}
