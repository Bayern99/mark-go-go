import type { Node as PMNode } from '@milkdown/kit/prose/model'
import {
  Plugin,
  PluginKey,
  TextSelection,
  type EditorState,
} from '@milkdown/kit/prose/state'
import {
  Decoration,
  DecorationSet,
  type EditorView,
} from '@milkdown/kit/prose/view'
import { normalizeHeadingText } from '../markdown/headings'

export const foldPluginKey = new PluginKey<FoldState>('mgo-fold')

const HEADING = 'heading'
const CODE = 'code_block'

export interface FoldState {
  anchors: Set<number>
}

interface TopNode {
  node: PMNode
  pos: number
}

function topNodes(state: EditorState): TopNode[] {
  const out: TopNode[] = []
  state.doc.forEach((node, offset) => out.push({ node, pos: offset }))
  return out
}

function anchorKind(node: PMNode): 'heading' | 'code' | null {
  if (node.type.name === HEADING) return 'heading'
  if (node.type.name === CODE) return 'code'
  return null
}

function nodeKeyBase(node: PMNode): string | null {
  if (node.type.name === HEADING)
    return `h${(node.attrs.level as number) ?? 1}:${normalizeHeadingText(node.textContent)}`
  if (node.type.name === CODE) return 'code'
  return null
}

export function hiddenRange(
  state: EditorState,
  anchorPos: number,
): { from: number; to: number } | null {
  const anchor = state.doc.nodeAt(anchorPos)
  const kind = anchor && anchorKind(anchor)
  if (!kind || !anchor) return null
  if (kind === 'code') return { from: anchorPos, to: anchorPos + anchor.nodeSize }
  const level = (anchor.attrs.level as number) ?? 1
  let to = state.doc.content.size
  for (const { node, pos } of topNodes(state)) {
    if (pos <= anchorPos) continue
    if (node.type.name === HEADING && ((node.attrs.level as number) ?? 1) <= level) {
      to = pos
      break
    }
  }
  return { from: anchorPos + anchor.nodeSize, to }
}

export function foldsCovering(state: EditorState, pos: number): number[] {
  const anchors = foldPluginKey.getState(state)?.anchors
  if (!anchors || anchors.size === 0) return []
  const covering: number[] = []
  for (const a of anchors) {
    const range = hiddenRange(state, a)
    if (range && pos >= range.from && pos < range.to) covering.push(a)
  }
  return covering
}

function sectionAt(state: EditorState, pos: number): number | null {
  let found: number | null = null
  for (const { node, pos: p } of topNodes(state)) {
    if (p > pos) break
    if (node.type.name === HEADING) found = p
  }
  return found
}

function anchorSig(state: EditorState, pos: number): string | null {
  const node = state.doc.nodeAt(pos)
  const kind = node && anchorKind(node)
  if (!kind || !node) return null
  const range = hiddenRange(state, pos)
  const body =
    kind === 'code'
      ? node.textContent
      : range
        ? state.doc.textBetween(range.from, range.to, ' ')
        : ''
  const s = `${kind}|${(node.attrs.level as number) ?? ''}|${node.textContent}|${body}`
  let h = 5381
  for (let i = 0; i < s.length; i++) h = ((h << 5) + h + s.charCodeAt(i)) | 0
  return String(h >>> 0)
}

export interface FoldEntry {
  base: string
  idx: number
  sig: string
}

export interface PersistedFolds {
  entries: FoldEntry[]
}

function anchorPayload(state: EditorState, anchors: Set<number>): PersistedFolds {
  const seen = new Map<string, number>()
  const entries: FoldEntry[] = []
  for (const { node, pos } of topNodes(state)) {
    if (!anchors.has(pos)) continue
    const base = nodeKeyBase(node)
    const sig = anchorSig(state, pos)
    if (!base || !sig) continue
    const idx = seen.get(base) ?? 0
    seen.set(base, idx + 1)
    entries.push({ base, idx, sig })
  }
  return { entries }
}

function keysToAnchors(state: EditorState, payload: PersistedFolds): Set<number> {
  const bySig = new Map<string, number[]>()
  for (const { node, pos } of topNodes(state)) {
    if (!anchorKind(node)) continue
    const sig = anchorSig(state, pos)
    if (!sig) continue
    const list = bySig.get(sig) ?? []
    list.push(pos)
    bySig.set(sig, list)
  }
  const anchors = new Set<number>()
  for (const e of payload.entries) {
    const candidates = bySig.get(e.sig) ?? []
    if (candidates.length === 1) anchors.add(candidates[0])
  }
  return anchors
}

function codeBlockPos(view: EditorView, dom: Element): number | null {
  const block = dom.closest('.milkdown-code-block')
  if (!block) return null
  const pos = view.posAtDOM(block, 0)
  for (const p of [pos, pos - 1]) {
    const node = p >= 0 ? view.state.doc.nodeAt(p) : null
    if (node?.type.name === CODE) return p
  }
  return null
}

function chevron(folded: boolean, onToggle: () => void): HTMLElement {
  const b = document.createElement('button')
  b.type = 'button'
  b.className = 'mgo-fold-toggle'
  b.contentEditable = 'false'
  b.setAttribute('aria-hidden', 'true')
  b.tabIndex = -1
  b.title = folded ? 'Expand section' : 'Collapse section'
  b.innerHTML = folded
    ? '<svg viewBox="0 0 16 16" width="14" height="14"><path d="M6 4l4 4-4 4" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>'
    : '<svg viewBox="0 0 16 16" width="14" height="14"><path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>'
  b.addEventListener('mousedown', (e) => {
    e.preventDefault()
    e.stopPropagation()
  })
  b.addEventListener('click', (e) => {
    e.preventDefault()
    e.stopPropagation()
    onToggle()
  })
  return b
}

export interface FoldFeature {
  plugins: Plugin[]
  earlyPlugins: Plugin[]
  toggle(pos: number): void
  expandAt(pos: number): boolean
  expandRange(from: number, to: number): boolean
  keys(): string[]
  payload(): PersistedFolds
}

export function createFoldFeature(
  getView: () => EditorView,
  persisted: PersistedFolds = { entries: [] },
  onChange?: (payload: PersistedFolds) => void,
): FoldFeature {
  const dispatchMeta = (payload: { toggle?: number; expand?: number[] }) => {
    const view = getView()
    const tr = view.state.tr
      .setMeta(foldPluginKey, payload)
      .setMeta('addToHistory', false)
      .setMeta('mgo-fold:self', true)
    view.dispatch(tr)
  }

  const api = {
    toggle(pos: number) {
      dispatchMeta({ toggle: pos })
      moveSelectionOutOfFold(getView(), pos)
    },
    expandAt(pos: number) {
      const covering = foldsCovering(getView().state, pos)
      if (covering.length === 0) return false
      dispatchMeta({ expand: covering })
      return true
    },
    expandRange(from: number, to: number) {
      const state = getView().state
      const anchors = foldPluginKey.getState(state)?.anchors
      if (!anchors || anchors.size === 0) return false
      const hit: number[] = []
      for (const a of anchors) {
        const r = hiddenRange(state, a)
        if (r && from < r.to && to > r.from) hit.push(a)
      }
      if (hit.length === 0) return false
      dispatchMeta({ expand: hit })
      return true
    },
    keys() {
      const p = api.payload()
      return p.entries.map((e) => `${e.base}#${e.idx}@${e.sig}`)
    },
    payload() {
      const state = foldPluginKey.getState(getView().state)
      return state
        ? anchorPayload(getView().state, state.anchors)
        : { entries: [] }
    },
  }


  const plugin = new Plugin<FoldState>({
    key: foldPluginKey,
    state: {
      init(_config, instance) {
        const live = api.payload()
        return { anchors: keysToAnchors(instance, live.entries.length > 0 ? live : persisted) }
      },
      apply(tr, value) {
        const action = tr.getMeta(foldPluginKey) as
          | { toggle?: number; expand?: number[] }
          | undefined
        let anchors = value.anchors
        if (action) {
          anchors = new Set(anchors)
          if (action.toggle !== undefined) {
            const node = tr.doc.nodeAt(action.toggle)
            if (node && anchorKind(node)) {
              if (anchors.has(action.toggle)) anchors.delete(action.toggle)
              else anchors.add(action.toggle)
            }
          }
          if (action.expand) for (const a of action.expand) anchors.delete(a)
        }
        if (anchors.size === 0) return { anchors }
        if (tr.docChanged || tr.mapping) {
          const mapped = new Set<number>()
          for (const a of anchors) {
            const p = tr.mapping.map(a, -1)
            const node = tr.doc.nodeAt(p)
            if (node && anchorKind(node)) mapped.add(p)
          }
          return { anchors: mapped }
        }
        return anchors === value.anchors ? value : { anchors }
      },
    },
    props: {
      decorations(state) {
        const field = foldPluginKey.getState(state)
        const decos: Decoration[] = []
        const folded = field?.anchors ?? new Set<number>()
        const tops = topNodes(state)

        const hidden = new Set<number>()
        for (const a of folded) {
          const node = state.doc.nodeAt(a)
          if (node?.type.name !== HEADING) continue
          const range = hiddenRange(state, a)
          if (!range) continue
          for (const t of tops) {
            if (t.pos >= range.from && t.pos < range.to) hidden.add(t.pos)
          }
        }

        for (const { node, pos } of tops) {
          if (hidden.has(pos)) {
            decos.push(Decoration.node(pos, pos + node.nodeSize, { class: 'mgo-fold-hidden' }))
            continue
          }
          if (node.type.name === HEADING) {
            const isFolded = folded.has(pos)
            decos.push(
              Decoration.node(pos, pos + node.nodeSize, {
                class: isFolded ? 'mgo-fold-heading mgo-is-folded' : 'mgo-fold-heading',
              }),
              Decoration.widget(
                pos + node.nodeSize - 1,
                () => chevron(isFolded, () => api.toggle(pos)),
                { side: 1, ignoreSelection: true, key: `fold-${pos}-${isFolded ? 1 : 0}` },
              ),
            )
          } else if (node.type.name === CODE) {
            const isFolded = folded.has(pos)
            const lines = node.textContent ? node.textContent.split('\n').length : 0
            decos.push(
              Decoration.node(pos, pos + node.nodeSize, {
                class: isFolded ? 'mgo-code-folded' : 'mgo-code-foldable',
                'data-fold-lines': String(lines),
              }),
            )
          }
        }
        return DecorationSet.create(state.doc, decos)
      },
    },
    view(view) {
      const injectToggles = () => {
        for (const tools of view.dom.querySelectorAll('.milkdown-code-block .tools')) {
          if (tools.querySelector('.mgo-code-fold-btn')) continue
          const pos = codeBlockPos(view, tools)
          if (pos === null) continue
          const block = tools.parentElement!
          const btn = chevron(false, () => {
            const p = codeBlockPos(view, block)
            if (p !== null) api.toggle(p)
          })
          btn.classList.add('mgo-code-fold-btn')
          btn.style.opacity = '1'
          tools.prepend(btn)
        }
      }
      injectToggles()
      const observer = new MutationObserver(() => injectToggles())
      observer.observe(view.dom, { childList: true, subtree: true })
      const onClick = (e: MouseEvent) => {
        if (!(e.target instanceof Element)) return
        const folded = e.target.closest('.mgo-code-folded')
        if (!folded) return
        const pos = codeBlockPos(view, folded)
        if (pos !== null) {
          e.preventDefault()
          api.toggle(pos)
        }
      }
      view.dom.addEventListener('click', onClick)
      return {
        update(v, prev) {
          injectToggles()
          const a = foldPluginKey.getState(v.state)?.anchors
          const b = foldPluginKey.getState(prev)?.anchors
          if (a !== b && onChange) onChange(api.payload())
        },
        destroy() {
          observer.disconnect()
          view.dom.removeEventListener('click', onClick)
        },
      }
    },
  })
  const autoExpandPlugin = new Plugin({
    appendTransaction(trs, _old, state) {
      if (trs.some((tr) => tr.getMeta('mgo-fold:self'))) return null
      const anchors = foldPluginKey.getState(state)?.anchors
      if (!anchors || anchors.size === 0) return null
      const { from, to } = state.selection
      const hit: number[] = []
      for (const a of anchors) {
        const r = hiddenRange(state, a)
        if (r && from < r.to && to > r.from) hit.push(a)
      }
      if (hit.length === 0) return null
      return state.tr
        .setMeta(foldPluginKey, { expand: hit })
        .setMeta('addToHistory', false)
    },
  })

  const boundaryPlugin = new Plugin({
    props: {
      handleKeyDown(view, event) {
        if (event.key !== 'Backspace' && event.key !== 'Delete') return false
        const { state } = view
        if (!state.selection.empty) return false
        const anchors = foldPluginKey.getState(state)?.anchors
        if (!anchors || anchors.size === 0) return false
        const caret = state.selection.from
        const probes =
          event.key === 'Backspace'
            ? [caret, caret - 1, caret - 2]
            : [caret, caret + 1, caret + 2]
        const hit = [...new Set(probes.flatMap((p) => foldsCovering(state, p)))]
        if (hit.length) {
          dispatchMeta({ expand: hit })
          return true
        }
        return false
      },
    },
  })

  const keyPlugin = new Plugin({
    props: {
      handleKeyDown(view, event) {
        const { state } = view
        if (!(event.metaKey || event.ctrlKey) || !event.altKey) return false
        const codeAncestor = (() => {
          const $from = state.selection.$from
          for (let d = $from.depth; d >= 0; d--) {
            if ($from.node(d).type.name === CODE) return $from.start(d) - 1
          }
          return null
        })()
        if (event.key === 'ArrowLeft') {
          if (codeAncestor !== null) {
            api.toggle(codeAncestor)
            return true
          }
          const h = sectionAt(state, state.selection.from)
          if (h === null) return false
          api.toggle(h)
          return true
        }
        if (event.key === 'ArrowRight') {
          if (codeAncestor !== null && foldPluginKey.getState(state)?.anchors.has(codeAncestor)) {
            api.toggle(codeAncestor)
            return true
          }
          const h = sectionAt(state, state.selection.from)
          if (h === null || !foldPluginKey.getState(state)?.anchors.has(h)) return false
          api.toggle(h)
          return true
        }
        return false
      },
    },
  })

  return {
    plugins: [plugin, autoExpandPlugin, keyPlugin],
    earlyPlugins: [boundaryPlugin],
    toggle: api.toggle,
    expandAt: api.expandAt,
    expandRange: api.expandRange,
    keys: api.keys,
    payload: api.payload,
  }
}

export function moveSelectionOutOfFold(view: EditorView, anchorPos: number): void {
  if (!foldPluginKey.getState(view.state)?.anchors.has(anchorPos)) return
  const anchor = view.state.doc.nodeAt(anchorPos)
  if (!anchor) return
  const { from, to } = view.state.selection
  const range = hiddenRange(view.state, anchorPos)
  if (!range || to <= range.from || from >= range.to) return
  const end = anchorPos + anchor.nodeSize - 1
  view.dispatch(view.state.tr.setSelection(TextSelection.create(view.state.doc, end)))
}
