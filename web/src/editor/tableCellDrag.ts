import { Plugin } from '@milkdown/kit/prose/state'
import { CellSelection } from '@milkdown/kit/prose/tables'
import type { EditorView } from '@milkdown/kit/prose/view'

const CELL_NAMES = new Set(['table_cell', 'table_header'])

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
  let crossed = false

  return new Plugin({
    filterTransaction(tr) {
      if (!crossed) return true
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
          window.getSelection()?.removeAllRanges()
        }
      }

      const done = () => {
        anchor = null
        if (!crossed) return
        window.getSelection()?.removeAllRanges()
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
