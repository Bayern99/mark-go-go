/**
 * Section and code-block folding. Fold state is plugin data only: it never
 * touches the document, so Markdown bytes, undo history and the dirty flag
 * are unaffected. Anchors are document positions mapped through edits; a
 * mark dies when it no longer points at a foldable node (heading deleted or
 * retitled is still a heading; a heading turned to a paragraph is not).
 */
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
  /** Document positions of fold anchors (heading or code-block nodes). */
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

/** Hidden range (exclusive of the anchor) for one anchor node. */
export function hiddenRange(
  state: EditorState,
  anchorPos: number,
): { from: number; to: number } | null {
  const anchor = state.doc.nodeAt(anchorPos)
  const kind = anchor && anchorKind(anchor)
  if (!kind || !anchor) return null
  if (kind === 'code') return { from: anchorPos, to: anchorPos + anchor.nodeSize }
  // A section runs to the next heading of the same or higher level.
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

/** Anchors of folds hiding `pos`, outermost first. */
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

/** The last top-level heading at-or-before `pos` — the caret's section. */
function sectionAt(state: EditorState, pos: number): number | null {
  let found: number | null = null
  for (const { node, pos: p } of topNodes(state)) {
    if (p > pos) break
    if (node.type.name === HEADING) found = p
  }
  return found
}

/** Stable-ish identity for persistence: kind+text base plus occurrence order. */
function anchorKeys(state: EditorState, anchors: Set<number>): string[] {
  const seen = new Map<string, number>()
  const keys: string[] = []
  for (const { node, pos } of topNodes(state)) {
    if (!anchors.has(pos)) continue
    const base = nodeKeyBase(node)
    if (!base) continue
    const idx = seen.get(base) ?? 0
    seen.set(base, idx + 1)
    keys.push(`${base}#${idx}`)
  }
  return keys
}

export interface PersistedFolds {
  keys: string[]
  /** Occurrence totals per base at save time; mismatch => drop that base. */
  counts: Record<string, number>
}

function anchorPayload(state: EditorState, anchors: Set<number>): PersistedFolds {
  const counts: Record<string, number> = {}
  for (const { node } of topNodes(state)) {
    const base = nodeKeyBase(node)
    if (base) counts[base] = (counts[base] ?? 0) + 1
  }
  return { keys: anchorKeys(state, anchors), counts }
}

/**
 * Resolve persisted keys to current positions. When a base's occurrence
 * count changed since saving (a same-named heading was inserted, removed
 * or reordered outside the session), every key of that base is dropped —
 * an ambiguous match must expand, not fold the wrong section.
 */
function keysToAnchors(state: EditorState, payload: PersistedFolds): Set<number> {
  const wanted = new Set(payload.keys)
  const now: Record<string, number> = {}
  for (const { node } of topNodes(state)) {
    const base = nodeKeyBase(node)
    if (base) now[base] = (now[base] ?? 0) + 1
  }
  const seen = new Map<string, number>()
  const anchors = new Set<number>()
  for (const { node, pos } of topNodes(state)) {
    const base = nodeKeyBase(node)
    if (!base) continue
    const idx = seen.get(base) ?? 0
    seen.set(base, idx + 1)
    if (payload.counts[base] === now[base] && wanted.has(`${base}#${idx}`)) anchors.add(pos)
  }
  return anchors
}

/** Start position of the top-level code_block nodeview owning `dom`. */
function codeBlockPos(view: EditorView, dom: Element): number | null {
  const block = dom.closest('.milkdown-code-block')
  if (!block) return null
  const pos = view.posAtDOM(block, 0)
  // posAtDOM may land on the node start or just inside the leaf nodeview.
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
  // aria-hidden: the button's label would otherwise append itself to the
  // heading's accessible name ('Deep heading' -> 'Deep heading Collapse
  // section'), breaking AT locators. Keyboard folding uses Mod-Alt-Arrows.
  b.setAttribute('aria-hidden', 'true')
  b.tabIndex = -1
  b.title = folded ? 'Expand section' : 'Collapse section'
  b.innerHTML = folded
    ? '<svg viewBox="0 0 16 16" width="14" height="14"><path d="M6 4l4 4-4 4" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>'
    : '<svg viewBox="0 0 16 16" width="14" height="14"><path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>'
  b.addEventListener('mousedown', (e) => {
    e.preventDefault() // keep editor focus/selection
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
  /** Boundary-key handler that must precede the base keymap. */
  earlyPlugins: Plugin[]
  /** Toggle the fold anchored at `pos` (heading or code block). */
  toggle(pos: number): void
  /** Expand every fold hiding `pos`; returns true when something changed. */
  expandAt(pos: number): boolean
  /** Expand folds whose hidden range intersects [from, to]. */
  expandRange(from: number, to: number): boolean
  /** Persisted keys for the current fold state. */
  keys(): string[]
  /** Full persistence payload (keys + occurrence counts). */
  payload(): PersistedFolds
}

export function createFoldFeature(
  getView: () => EditorView,
  persisted: PersistedFolds = { keys: [], counts: {} },
  onChange?: (payload: PersistedFolds) => void,
): FoldFeature {
  const persistedKeys = persisted
  const dispatchMeta = (payload: { toggle?: number; expand?: number[] }) => {
    const view = getView()
    const tr = view.state.tr
      .setMeta(foldPluginKey, payload)
      .setMeta('addToHistory', false)
      // The auto-expander must not undo a toggle while the caret is still
      // inside the freshly hidden range; the caller relocates it next.
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
      const state = foldPluginKey.getState(getView().state)
      return state ? anchorKeys(getView().state, state.anchors) : []
    },
    payload() {
      const state = foldPluginKey.getState(getView().state)
      return state
        ? anchorPayload(getView().state, state.anchors)
        : { keys: [], counts: {} }
    },
  }

  // Count of each key base in a fresh state, for reload-time restore.
  function liveCounts(instance: EditorState): Record<string, number> {
    const counts: Record<string, number> = {}
    for (const { node } of topNodes(instance)) {
      const base = nodeKeyBase(node)
      if (base) counts[base] = (counts[base] ?? 0) + 1
    }
    return counts
  }

  const plugin = new Plugin<FoldState>({
    key: foldPluginKey,
    state: {
      init(_config, instance) {
        // On reloads (load() rebuilds the state) keep the live session
        // folds; only the first state falls back to persisted keys.
        const live = api.keys()
        return {
          anchors: keysToAnchors(
            instance,
            live.length > 0 ? { keys: live, counts: liveCounts(instance) } : persistedKeys,
          ),
        }
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
        // Map anchors through the transaction; drop marks that no longer
        // point at their node kind.
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

        // Collect nodes hidden by section folds (a heading's own DOM stays).
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
      // The .tools row only appears when Crepe lazily mounts CodeMirror
      // (often on first scroll into view), which happens without a PM
      // transaction — watch the DOM instead of relying on update() alone.
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
  // Any selection landing inside hidden content expands the folds covering
  // it — arrow keys, Backspace merges and programmatic moves must never
  // leave an invisible caret or selection span.
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

  // Backspace/Delete at a fold boundary must run BEFORE the base keymap's
  // joinBackward/joinForward: it would otherwise merge hidden content
  // invisibly. createEditor unshifts this plugin ahead of Crepe's keymaps.
  const boundaryPlugin = new Plugin({
    props: {
      handleKeyDown(view, event) {
        if (event.key !== 'Backspace' && event.key !== 'Delete') return false
        const { state } = view
        if (!state.selection.empty) return false
        const anchors = foldPluginKey.getState(state)?.anchors
        if (!anchors || anchors.size === 0) return false
        const caret = state.selection.from
        // Backspace deletes backward from the caret: at a block boundary
        // the victim is the node before it, which can be the last hidden
        // node (caret-1 lands exactly on the range edge — probe caret-2).
        // Delete mirrors that on the forward edge.
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

  // Keyboard: Mod-Alt-ArrowLeft folds the caret's section; ArrowRight expands it.
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

/**
 * When the user folds the section holding the selection, put the caret at
 * the end of the heading so the next keystroke lands in a visible place.
 */
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
