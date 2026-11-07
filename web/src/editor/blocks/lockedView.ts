import type { Node as PMNode } from '@milkdown/kit/prose/model'
import type { EditorView, NodeView } from '@milkdown/kit/prose/view'
import { highlightCode, classHighlighter } from '@lezer/highlight'
import { parseNote } from '../../markdown/notes'
import { languages } from '../languages'
import { LOCKED_NODE, lockedKindLabel, type LockedAttrs } from './locked'
import {
  codeLanguageLabel,
  parseAlert,
  parseCodeFence,
  parseFootnote,
  parseFrontmatterRows,
  type AlertKind,
} from './present'
import type { LockKind } from './segment'

const ALERT_ICON: Record<AlertKind, string> = {
  note: 'ph-note',
  tip: 'ph-lightbulb',
  important: 'ph-info',
  warning: 'ph-warning',
  caution: 'ph-warning-octagon',
}

export function lockedBlockView(node: PMNode, view: EditorView): NodeView {
  const dom = document.createElement(node.attrs.kind === 'note' ? 'aside' : 'div')
  fill(dom, node, !view.editable)
  return {
    dom,
    update(updated) {
      if (updated.type.name !== LOCKED_NODE) return false
      fill(dom, updated, !view.editable)
      return true
    },
    ignoreMutation: () => true,
    stopEvent(event) {
      const target = event.target
      if (!(target instanceof Element)) return false
      return target.closest('[data-block-action], [data-note-action], a, button, textarea') !== null
    },
  }
}

export function footnoteReferenceView(node: PMNode): NodeView {
  const label = String(node.attrs.label ?? '')
  const id = cssSafe(label)
  const dom = document.createElement('sup')
  const link = document.createElement('a')
  link.className = 'mgo-fnref'
  link.href = `#fn-${id}`
  link.id = `fnref-${id}`
  link.dataset.footnoteRef = label
  link.textContent = label
  link.setAttribute('aria-label', `Footnote ${label}`)
  link.tabIndex = 0
  dom.append(link)
  return {
    dom,
    update(updated) {
      if (updated.type.name !== 'footnote_reference') return false
      const next = String(updated.attrs.label ?? '')
      link.textContent = next
      link.id = `fnref-${cssSafe(next)}`
      link.href = `#fn-${cssSafe(next)}`
      link.dataset.footnoteRef = next
      link.setAttribute('aria-label', `Footnote ${next}`)
      return true
    },
    ignoreMutation: () => true,
  }
}

function fill(dom: HTMLElement, node: PMNode, readonly: boolean): void {
  const attrs = node.attrs as LockedAttrs
  const kind = (attrs.kind in kindSet ? attrs.kind : 'unsupported') as LockKind
  dom.className = ''
  dom.replaceChildren()
  applyLockedDataset(dom, kind, attrs.source)
  if (kind === 'note') {
    renderNote(dom, attrs.source, readonly)
    return
  }
  if (kind === 'yaml-frontmatter' && renderFrontmatter(dom, attrs.source, readonly)) return
  if (kind === 'github-alert' && renderAlert(dom, attrs.source, readonly)) return
  if (kind === 'footnote-definition' && renderFootnote(dom, attrs.source, readonly)) return
  if (kind === 'code-fence-meta' && renderCodeFence(dom, attrs.source, readonly)) return
  if (kind === 'math') {
    renderMath(dom, attrs.source, readonly)
    return
  }
  renderKept(dom, kind, attrs.source, readonly)
}

const kindSet: Record<LockKind, true> = {
  'yaml-frontmatter': true,
  'github-alert': true,
  'footnote-definition': true,
  'link-reference-definition': true,
  'code-fence-meta': true,
  'raw-html': true,
  note: true,
  math: true,
  unsupported: true,
}

function applyLockedDataset(dom: HTMLElement, kind: LockKind, source: string): void {
  dom.dataset.lockedBlock = 'true'
  dom.dataset.kind = kind
  dom.dataset.source = source
  dom.dataset.label = lockedKindLabel(kind)
  dom.contentEditable = 'false'
  dom.spellcheck = false
}

function renderNote(dom: HTMLElement, source: string, readonly: boolean): void {
  dom.className = 'mgo-note'
  dom.dataset.note = 'true'
  dom.setAttribute('aria-label', 'Note')
  const label = document.createElement('span')
  label.className = 'mgo-note-label'
  label.textContent = 'Note'
  const text = document.createElement('span')
  text.className = 'mgo-note-text'
  text.textContent = parseNote(source)?.text ?? ''
  const actions = document.createElement('span')
  actions.className = 'mgo-note-actions'
  if (!readonly) {
    actions.append(actionButton('edit', 'Edit', 'note'), actionButton('delete', 'Delete', 'note'))
  }
  dom.append(label, text, actions)
}

function renderFrontmatter(dom: HTMLElement, source: string, readonly: boolean): boolean {
  const rows = parseFrontmatterRows(source)
  if (rows.length === 0) return false
  dom.className = 'mgo-frontmatter-block'
  dom.setAttribute('aria-label', 'Frontmatter')
  const table = document.createElement('table')
  table.className = 'mgo-frontmatter'
  const body = document.createElement('tbody')
  for (const row of rows) {
    const tr = document.createElement('tr')
    const th = document.createElement('th')
    th.scope = 'row'
    th.textContent = row.key
    const td = document.createElement('td')
    td.textContent = row.value
    tr.append(th, td)
    body.append(tr)
  }
  table.append(body)
  dom.append(table, blockActions(readonly))
  return true
}

function renderAlert(dom: HTMLElement, source: string, readonly: boolean): boolean {
  const alert = parseAlert(source)
  if (!alert) return false
  dom.className = `mgo-alert ${alert.kind}`
  dom.setAttribute('aria-label', `${alert.label} alert`)
  const kicker = document.createElement('span')
  kicker.className = 'mgo-alert-k'
  const icon = document.createElement('i')
  icon.className = `ph ${ALERT_ICON[alert.kind]}`
  icon.setAttribute('aria-hidden', 'true')
  kicker.append(icon, document.createTextNode(alert.label))
  const body = document.createElement('div')
  body.className = 'mgo-alert-body'
  body.textContent = alert.body
  dom.append(kicker, body, blockActions(readonly))
  return true
}

function renderFootnote(dom: HTMLElement, source: string, readonly: boolean): boolean {
  const footnote = parseFootnote(source)
  if (!footnote) return false
  dom.className = 'mgo-footnote'
  dom.id = `fn-${cssSafe(footnote.label)}`
  dom.dataset.footnoteLabel = footnote.label
  dom.setAttribute('aria-label', `Footnote ${footnote.label}`)
  dom.tabIndex = -1
  const mark = document.createElement('a')
  mark.className = 'mgo-footnote-mark'
  mark.href = `#fnref-${cssSafe(footnote.label)}`
  mark.dataset.footnoteBack = footnote.label
  mark.textContent = footnote.label
  mark.tabIndex = 0
  const body = document.createElement('div')
  body.className = 'mgo-footnote-body'
  body.textContent = footnote.body
  const back = document.createElement('a')
  back.className = 'mgo-footnote-back'
  back.href = `#fnref-${cssSafe(footnote.label)}`
  back.dataset.footnoteBack = footnote.label
  back.textContent = '↩'
  back.setAttribute('aria-label', `Back to reference ${footnote.label}`)
  back.tabIndex = 0
  body.append(' ', back)
  dom.append(mark, body, blockActions(readonly))
  return true
}

function renderCodeFence(dom: HTMLElement, source: string, readonly: boolean): boolean {
  const fence = parseCodeFence(source)
  if (!fence) return false
  dom.className = 'mgo-code-block'
  dom.setAttribute('aria-label', fence.language ? `Code ${fence.language}` : 'Code block')
  const meta = document.createElement('div')
  meta.className = 'mgo-code-chrome'
  if (fence.language) {
    const lang = document.createElement('span')
    lang.className = 'mgo-code-lang'
    lang.textContent = codeLanguageLabel(fence.language)
    meta.append(lang)
  }
  if (fence.meta) {
    const extra = document.createElement('span')
    extra.className = 'mgo-code-meta'
    extra.textContent = fence.meta
    meta.append(extra)
  }
  const pre = document.createElement('pre')
  pre.className = 'mgo-code-body'
  const code = document.createElement('code')
  appendHighlighted(code, fence.code, fence.language)
  pre.append(code)
  dom.append(meta, pre, blockActions(readonly))
  return true
}

function renderMath(dom: HTMLElement, source: string, readonly: boolean): void {
  dom.className = 'mgo-math'
  dom.setAttribute('aria-label', 'Math block')
  const kicker = document.createElement('span')
  kicker.className = 'mgo-math-k'
  kicker.textContent = 'Math'
  const body = document.createElement('pre')
  body.className = 'mgo-math-source'
  body.textContent = source.replace(/^\$\$\s*|\s*\$\$$/g, '')
  dom.append(kicker, body, blockActions(readonly))
}

function renderKept(dom: HTMLElement, kind: LockKind, source: string, readonly: boolean): void {
  dom.className = 'mgo-locked-block'
  dom.setAttribute('aria-label', `Kept as written: ${lockedKindLabel(kind)}`)
  const code = document.createElement('code')
  code.className = 'mgo-locked-source'
  code.textContent = source
  dom.append(code, blockActions(readonly))
}

function blockActions(readonly: boolean): HTMLElement {
  const actions = document.createElement('span')
  actions.className = 'mgo-block-actions'
  if (!readonly) actions.append(actionButton('edit', 'Edit', 'block'))
  return actions
}

function actionButton(action: string, label: string, family: 'note' | 'block'): HTMLButtonElement {
  const button = document.createElement('button')
  button.type = 'button'
  button.className = 'pill xs'
  button.dataset[family === 'note' ? 'noteAction' : 'blockAction'] = action
  button.textContent = label
  return button
}

function appendHighlighted(parent: HTMLElement, code: string, language: string): void {
  const desc = language
    ? languages.find(
        (item) =>
          item.name.toLowerCase() === language.toLowerCase() || item.alias.includes(language.toLowerCase()),
      )
    : undefined
  const parser = desc?.support?.language.parser
  if (!parser) {
    parent.textContent = code
    return
  }
  highlightCode(
    code,
    parser.parse(code),
    classHighlighter,
    (text, classes) => {
      if (!classes) {
        parent.append(text)
        return
      }
      const span = document.createElement('span')
      span.className = classes
      span.textContent = text
      parent.append(span)
    },
    () => parent.append('\n'),
  )
}

function cssSafe(label: string): string {
  return encodeURIComponent(label).replace(/%/g, '_')
}
