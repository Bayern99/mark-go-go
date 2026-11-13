/**
 * Cross-cell drag selection. Crepe's TableNodeView.stopEvent swallows cell
 * mousedowns before ProseMirror's table plugin can arm its own drag
 * watcher, so prosemirror-tables never creates a CellSelection — and
 * because PM's LeftMouseDown never starts, its delayedSelectionSync never
 * suppresses DOM-selection syncing either. Two consequences handled here:
 *
 * 1. A raw mousedown listener records the anchor cell; once the pointer
 *    crosses into a different cell we dispatch CellSelection (upstream
 *    semantics: re-entering the anchor keeps a single-cell selection).
 * 2. While a cross-cell drag is live, filterTransaction rejects
 *    selection-only transactions that aren't CellSelection — this stands
 *    in for PM's suppressed DOM-selection sync so the browser's native
 *    drag selection can't overwrite the CellSelection.
 *
 * Untouched: single click (no crossing → no flag), intra-cell text drag
 * (flag only arms on crossing), and Crepe's delayed paragraph
 * NodeSelection (the Round 1 filter still drops it).
 */
import { Plugin } from '@milkdown/kit/prose/state'
import { CellSelection } from '@milkdown/kit/prose/tables'
import type { EditorView } from '@milkdown/kit/prose/view'

const CELL_NAMES = new Set(['table_cell', 'table_header'])

/** Position before the innermost table cell under the pointer, if any. */
function cellAt(view: EditorView, e: MouseEvent): number | null {
  const found = view.posAtCoords({ left: e.clientX, top: e.clientY })
  if (!found) return null
  const $pos = view.state.doc.resolve(
    Math.min(found.pos, view.state.doc.content.size),
  )
  for (let d = $pos.depth; d > 0; d--) {
    if (CELL_NAMES.has($pos.node(d).type.name)) return $pos.before(d)
  }
  return null
}

export function createCellDragPlugin(): Plugin {
  /** True while a cross-cell drag is in progress (mousedown → mouseup). */
  let crossed = false

  return new Plugin({
    filterTransaction(tr) {
      if (!crossed) return true
      // Only intercept the anomaly we confirmed: selection-only sync
      // transactions produced while the browser's native drag selection
      // fights our CellSelection. Content changes, paste, undo and plugin
      // state updates all pass through untouched.
      if (!tr.selectionSet || tr.docChanged) return true
      return tr.selection instanceof CellSelection
    },
    view(view) {
      let anchor: number | null = null

      const onDown = (e: MouseEvent) => {
        if (e.button !== 0) {
          anchor = null
          return
        }
        anchor =
          e.target instanceof Element && e.target.closest('td, th')
            ? cellAt(view, e)
            : null
      }

      const onMove = (e: MouseEvent) => {
        if (anchor === null || !(e.buttons & 1)) return
        const target = cellAt(view, e)
        if (target === anchor && !crossed) return
        if (target !== anchor) crossed = true
        if (!crossed) return
        const sel = CellSelection.create(view.state.doc, anchor, target ?? anchor)
        if (!sel.eq(view.state.selection)) {
          view.dispatch(view.state.tr.setSelection(sel))
          // Drop the browser's native text-selection highlight so it can't
          // fight .selectedCell; its selectionchange syncs are filtered
          // while `crossed` holds.
          window.getSelection()?.removeAllRanges()
        }
      }

      const done = () => {
        anchor = null
        if (!crossed) return
        window.getSelection()?.removeAllRanges()
        // Keep the filter up through any trailing selectionchange, then
        // release so normal editing resumes.
        setTimeout(() => {
          crossed = false
        }, 50)
      }

      view.dom.addEventListener('mousedown', onDown, true)
      window.addEventListener('mousemove', onMove, true)
      window.addEventListener('mouseup', done, true)
      window.addEventListener('blur', done)
      return {
        destroy() {
          view.dom.removeEventListener('mousedown', onDown, true)
          window.removeEventListener('mousemove', onMove, true)
          window.removeEventListener('mouseup', done, true)
          window.removeEventListener('blur', done)
        },
      }
    },
  })
}
