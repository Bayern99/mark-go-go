import type { Node as PMNode } from '@milkdown/kit/prose/model'
import { Plugin } from '@milkdown/kit/prose/state'
import {
  cellAround,
  columnResizing,
  columnResizingPluginKey,
  TableMap,
  updateColumnsOnResize,
} from '@milkdown/kit/prose/tables'
import type { EditorView } from '@milkdown/kit/prose/view'

const CELL_NAMES = new Set(['table_cell', 'table_header'])
const DEFAULT_CELL_MIN_WIDTH = 112
const HANDLE_WIDTH = 6

type ColumnWidths = (number | null)[]
export type TableColumnWidths = Record<string, ColumnWidths>

export function readTableColumnWidths(docKey: string | undefined): TableColumnWidths {
  if (!docKey) return {}
  try {
    const raw = localStorage.getItem(`mgo.table-widths:${docKey}`)
    const parsed: unknown = raw ? JSON.parse(raw) : {}
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return {}
    return Object.fromEntries(
      Object.entries(parsed).flatMap(([key, value]) => {
        if (!Array.isArray(value)) return []
        const widths = value.map((width) =>
          typeof width === 'number' && Number.isFinite(width) && width > 0 ? width : null,
        )
        return [[key, widths]]
      }),
    )
  } catch {
    return {}
  }
}

export function restoreTableColumnWidths(
  doc: PMNode,
  stored: TableColumnWidths,
): PMNode {
  const storedMatches = new Map<string, ColumnWidths[]>()
  for (const [key, widths] of Object.entries(stored)) {
    const separator = key.indexOf(':')
    if (separator < 0) continue
    const signature = key.slice(separator + 1)
    const matches = storedMatches.get(signature) ?? []
    matches.push(widths)
    storedMatches.set(signature, matches)
  }
  const currentMatches = new Map<string, number>()
  doc.forEach((node) => {
    if (node.type.name !== 'table') return
    const signature = tableSignature(node)
    currentMatches.set(signature, (currentMatches.get(signature) ?? 0) + 1)
  })

  let changed = false
  const children: PMNode[] = []
  doc.forEach((node) => {
    if (node.type.name !== 'table') {
      children.push(node)
      return
    }
    const signature = tableSignature(node)
    const matches = storedMatches.get(signature) ?? []
    const widths = currentMatches.get(signature) === 1 && matches.length === 1 ? matches[0] : undefined
    const restored = widths ? withColumnWidths(node, widths) : node
    children.push(restored)
    changed ||= restored !== node
  })
  return changed ? doc.type.create(doc.attrs, children, doc.marks) : doc
}

export function createTableColumnWidthPlugins(
  docKey: string | undefined,
  stored: TableColumnWidths,
): Plugin[] {
  const resizing = columnResizing({
    cellMinWidth: 72,
    defaultCellMinWidth: DEFAULT_CELL_MIN_WIDTH,
    View: null,
  })
  const resizeHandlers = resizing.props.handleDOMEvents
  if (resizeHandlers) {
    resizeHandlers.mousemove = () => false
    resizeHandlers.mousedown = () => false
  }
  const storageKey = docKey ? `mgo.table-widths:${docKey}` : undefined

  const integration = new Plugin({
    view(view) {
      let selecting = false
      let resize: ReturnType<typeof startColumnResize>
      const sync = () => syncTables(view)
      const updateHandle = (handle: number) => {
        if (columnResizingPluginKey.getState(view.state)?.activeHandle === handle) return
        view.dispatch(view.state.tr.setMeta(columnResizingPluginKey, { setHandle: handle }))
      }
      const onMouseDown = (event: MouseEvent) => {
        if (event.button !== 0 || !view.editable) return
        if (columnResizingPluginKey.getState(view.state)?.dragging) return
        sync()
        const handle = columnHandleAt(view, event)
        updateHandle(handle)
        resize = handle >= 0 ? startColumnResize(view, event, handle) : undefined
        selecting = !resize
        if (resize) {
          event.preventDefault()
          event.stopImmediatePropagation()
        }
      }
      const onMouseMove = (event: MouseEvent) => {
        if (columnResizingPluginKey.getState(view.state)?.dragging) {
          sync()
          return
        }
        if (selecting || !view.editable) return
        updateHandle(columnHandleAt(view, event))
      }
      const onPointerMove = (event: PointerEvent) => {
        if (columnResizingPluginKey.getState(view.state)?.dragging) {
          event.stopImmediatePropagation()
        }
      }
      const onEnd = () => {
        selecting = false
      }
      const onBlur = () => {
        onEnd()
        resize?.cancel()
        resize = undefined
      }

      sync()
      view.dom.addEventListener('mousedown', onMouseDown, true)
      view.dom.addEventListener('pointermove', onPointerMove, true)
      window.addEventListener('mousemove', onMouseMove, true)
      window.addEventListener('mouseup', onEnd, true)
      window.addEventListener('blur', onBlur)
      return {
        update(currentView, previousState) {
          if (currentView.state.doc === previousState.doc) return
          resize?.cancel()
          resize = undefined
          sync()
          persist(currentView, storageKey, stored)
        },
        destroy() {
          resize?.dispose()
          resize = undefined
          view.dom.removeEventListener('mousedown', onMouseDown, true)
          view.dom.removeEventListener('pointermove', onPointerMove, true)
          window.removeEventListener('mousemove', onMouseMove, true)
          window.removeEventListener('mouseup', onEnd, true)
          window.removeEventListener('blur', onBlur)
        },
      }
    },
  })

  return [resizing, integration]
}

function columnHandleAt(view: EditorView, event: MouseEvent): number {
  if (!(event.target instanceof Element)) return -1
  const cell = event.target.closest('th, td')
  const table = cell?.closest('table.children')
  if (
    !cell ||
    table?.closest('.milkdown-table-block')?.parentElement !== view.dom ||
    !table.querySelector(':scope > colgroup')
  ) return -1

  const { left, right } = cell.getBoundingClientRect()
  const atLeftEdge = event.clientX - left <= HANDLE_WIDTH
  if (!atLeftEdge && right - event.clientX > HANDLE_WIDTH) return -1

  const $cell = cellAround(view.state.doc.resolve(view.posAtDOM(cell, 0)))
  if (!$cell) return -1
  if (!atLeftEdge) return $cell.pos

  const start = $cell.start(-1)
  const map = TableMap.get($cell.node(-1))
  const index = map.map.indexOf($cell.pos - start)
  return index > 0 && index % map.width !== 0 ? start + map.map[index - 1]! : -1
}

function startColumnResize(
  view: EditorView,
  event: MouseEvent,
  cellPos: number,
): { cancel: () => void; dispose: () => void } | undefined {
  const cell = view.state.doc.nodeAt(cellPos)
  if (!cell) return undefined
  const $cell = view.state.doc.resolve(cellPos)
  if ($cell.node(-1).type.name !== 'table' || !$cell.nodeAfter) return undefined
  const startX = event.clientX
  const startWidth = currentColumnWidth(view, cellPos)
  view.dispatch(view.state.tr.setMeta(columnResizingPluginKey, {
    setDragging: { startX, startWidth },
  }))

  let settled = false
  const widthAt = (pointer: MouseEvent) =>
    Math.max(72, startWidth + pointer.clientX - startX)
  const dispose = () => {
    settled = true
    window.removeEventListener('mousemove', move)
    window.removeEventListener('mouseup', finish)
  }
  const cancel = () => {
    if (settled) return
    dispose()
    if (columnResizingPluginKey.getState(view.state)?.dragging) {
      view.dispatch(view.state.tr.setMeta(columnResizingPluginKey, {
        setDragging: null,
        setHandle: -1,
      }))
    }
    syncTables(view)
  }
  const move = (pointer: MouseEvent) => {
    if (!settled && columnResizingPluginKey.getState(view.state)?.dragging) {
      displayColumnWidth(view, cellPos, widthAt(pointer))
    }
  }
  const finish = (pointer: MouseEvent) => {
    if (settled) return
    dispose()
    if (!columnResizingPluginKey.getState(view.state)?.dragging) return
    view.dispatch(view.state.tr.setMeta(columnResizingPluginKey, { setDragging: null }))
    setColumnWidth(view, cellPos, widthAt(pointer))
  }
  window.addEventListener('mousemove', move)
  window.addEventListener('mouseup', finish)
  return { cancel, dispose }
}

// Column width helpers adapted from prosemirror-tables 1.8.5. See public/licenses/LICENSE-ProseMirror-Tables.txt.
function currentColumnWidth(view: EditorView, cellPos: number): number {
  const cell = view.state.doc.nodeAt(cellPos)
  if (!cell) return DEFAULT_CELL_MIN_WIDTH
  const colspan = cell.attrs.colspan as number
  const colwidth = cell.attrs.colwidth as number[] | null
  const existing = colwidth?.[colwidth.length - 1]
  if (existing) return existing
  const at = view.domAtPos(cellPos)
  const dom = at.node.childNodes[at.offset]
  if (!(dom instanceof HTMLElement)) return DEFAULT_CELL_MIN_WIDTH
  let width = dom.offsetWidth
  let parts = colspan
  if (colwidth) {
    for (let index = 0; index < colspan; index += 1) {
      if (!colwidth[index]) continue
      width -= colwidth[index]
      parts -= 1
    }
  }
  return width / parts
}

function displayColumnWidth(
  view: EditorView,
  cellPos: number,
  width: number,
): void {
  const $cell = view.state.doc.resolve(cellPos)
  const table = $cell.node(-1)
  const start = $cell.start(-1)
  const col = TableMap.get(table).colCount($cell.pos - start) + $cell.nodeAfter!.attrs.colspan - 1
  let dom: HTMLElement | null = view.domAtPos($cell.start(-1)).node as HTMLElement
  while (dom && dom.nodeName !== 'TABLE') dom = dom.parentElement
  if (!(dom instanceof HTMLTableElement)) return
  let colgroup = dom.querySelector<HTMLTableColElement>(':scope > colgroup')
  if (!colgroup) {
    colgroup = document.createElement('colgroup')
    dom.insertBefore(colgroup, dom.firstChild)
  }
  updateColumnsOnResize(table, colgroup, dom, DEFAULT_CELL_MIN_WIDTH, col, width)
}

function setColumnWidth(view: EditorView, cellPos: number, width: number): void {
  const $cell = view.state.doc.resolve(cellPos)
  const table = $cell.node(-1)
  const map = TableMap.get(table)
  const start = $cell.start(-1)
  const col = map.colCount($cell.pos - start) + $cell.nodeAfter!.attrs.colspan - 1
  const tr = view.state.tr
  for (let row = 0; row < map.height; row += 1) {
    const mapIndex = row * map.width + col
    if (row && map.map[mapIndex] === map.map[mapIndex - map.width]) continue
    const pos = map.map[mapIndex]
    if (pos === undefined) continue
    const cell = table.nodeAt(pos)
    if (!cell) continue
    const index = cell.attrs.colspan === 1 ? 0 : col - map.colCount(pos)
    const current = cell.attrs.colwidth as (number | null)[] | null
    if (current && current[index] === width) continue
    const colwidth = current ? [...current] : Array<number>(cell.attrs.colspan).fill(0)
    colwidth[index] = width
    tr.setNodeMarkup(start + pos, undefined, { ...cell.attrs, colwidth })
  }
  if (tr.docChanged) view.dispatch(tr)
}

export function tableSourceEquals(a: PMNode, b: PMNode): boolean {
  if (a.type.name !== 'table' || b.type.name !== 'table') return a.eq(b)
  return withoutColumnWidths(a).eq(withoutColumnWidths(b))
}

function syncTables(view: EditorView): void {
  view.state.doc.forEach((node, offset) => {
    if (node.type.name !== 'table') return
    const block = view.nodeDOM(offset)
    const table =
      block instanceof Element
        ? block.querySelector<HTMLTableElement>('table.children')
        : null
    if (!table) return
    let colgroup = table.querySelector<HTMLTableColElement>(':scope > colgroup')
    if (!colgroup) {
      colgroup = document.createElement('colgroup')
      table.insertBefore(colgroup, table.firstChild)
    }
    if (readWidths(node).some((width) => width !== null)) {
      updateColumnsOnResize(node, colgroup, table, DEFAULT_CELL_MIN_WIDTH)
      return
    }
    layoutDefaultTable(table, node, colgroup)
  })
}

function layoutDefaultTable(
  table: HTMLTableElement,
  node: PMNode,
  colgroup: HTMLTableColElement,
): void {
  const count = columnCount(node)
  if (colgroup.childElementCount !== count) {
    colgroup.replaceChildren(
      ...Array.from({ length: count }, () => document.createElement('col')),
    )
  }
  const preferred = preferredWidths(table, count)
  const total = preferred.reduce((sum, width) => sum + width, 0)
  table.style.width = '100%'
  table.style.minWidth = `${count * DEFAULT_CELL_MIN_WIDTH}px`
  preferred.forEach((width, index) => {
    ;(colgroup.children[index] as HTMLElement).style.width = `${(width / total) * 100}%`
  })
}

function preferredWidths(table: HTMLTableElement, count: number): number[] {
  const widths = Array<number>(count).fill(DEFAULT_CELL_MIN_WIDTH)
  for (const row of Array.from(table.rows)) {
    let column = 0
    for (const cell of Array.from(row.cells)) {
      const text = cell.textContent?.trim() ?? ''
      const units = Array.from(text).reduce(
        (sum, character) => sum + (/^[\u3400-\u9fff]$/.test(character) ? 1 : 0.55),
        0,
      )
      const cap = cell.querySelector('code')
        ? 260
        : /[\u3400-\u9fff]/.test(text)
          ? 360
          : 320
      const desired = Math.min(cap, Math.max(DEFAULT_CELL_MIN_WIDTH, units * 14 + 24))
      const share = desired / cell.colSpan
      const end = Math.min(column + cell.colSpan, count)
      for (let index = column; index < end; index += 1) {
        widths[index] = Math.max(widths[index]!, share)
      }
      column += cell.colSpan
    }
  }
  return widths
}

function persist(
  view: EditorView,
  storageKey: string | undefined,
  stored: TableColumnWidths,
): void {
  const next = collectWidths(view.state.doc)
  if (JSON.stringify(next) === JSON.stringify(stored)) return
  for (const key of Object.keys(stored)) delete stored[key]
  Object.assign(stored, next)
  if (!storageKey) return
  try {
    localStorage.setItem(storageKey, JSON.stringify(stored))
  } catch {
  }
}

function collectWidths(doc: PMNode): TableColumnWidths {
  const result: TableColumnWidths = {}
  doc.forEach((node, _offset, index) => {
    if (node.type.name !== 'table') return
    const widths = readWidths(node)
    if (widths.some((width) => width !== null)) result[tableKey(node, index)] = widths
  })
  return result
}

function withColumnWidths(table: PMNode, widths: ColumnWidths): PMNode {
  if (widths.length !== columnCount(table)) return table
  let tableChanged = false
  const rows: PMNode[] = []
  table.forEach((row) => {
    let column = 0
    let rowChanged = false
    const cells: PMNode[] = []
    row.forEach((cell) => {
      const colspan = cell.attrs.colspan as number
      const next = widths.slice(column, column + colspan)
      const current = (cell.attrs.colwidth as (number | null)[] | null) ?? null
      const normalized = next.some((width) => width !== null) ? next : null
      const same =
        current === normalized ||
        (current !== null && normalized !== null && JSON.stringify(current) === JSON.stringify(normalized))
      cells.push(
        same
          ? cell
          : cell.type.create({ ...cell.attrs, colwidth: normalized }, cell.content, cell.marks),
      )
      rowChanged ||= !same
      column += colspan
    })
    rows.push(rowChanged ? row.type.create(row.attrs, cells, row.marks) : row)
    tableChanged ||= rowChanged
  })
  return tableChanged ? table.type.create(table.attrs, rows, table.marks) : table
}

function withoutColumnWidths(table: PMNode): PMNode {
  const rows: PMNode[] = []
  let changed = false
  table.forEach((row) => {
    const cells: PMNode[] = []
    let rowChanged = false
    row.forEach((cell) => {
      if (!CELL_NAMES.has(cell.type.name) || cell.attrs.colwidth == null) {
        cells.push(cell)
        return
      }
      cells.push(cell.type.create({ ...cell.attrs, colwidth: null }, cell.content, cell.marks))
      rowChanged = true
    })
    rows.push(rowChanged ? row.type.create(row.attrs, cells, row.marks) : row)
    changed ||= rowChanged
  })
  return changed ? table.type.create(table.attrs, rows, table.marks) : table
}

function readWidths(table: PMNode): ColumnWidths {
  const widths: ColumnWidths = []
  table.firstChild?.forEach((cell) => {
    const colspan = cell.attrs.colspan as number
    const colwidth = cell.attrs.colwidth as (number | null)[] | null
    for (let index = 0; index < colspan; index += 1) {
      widths.push(colwidth?.[index] || null)
    }
  })
  return widths
}

function columnCount(table: PMNode): number {
  let count = 0
  table.firstChild?.forEach((cell) => {
    count += cell.attrs.colspan as number
  })
  return count
}

function tableKey(table: PMNode, index: number): string {
  return `${index}:${tableSignature(table)}`
}

function tableSignature(table: PMNode): string {
  const count = columnCount(table)
  const rows: string[][] = []
  table.forEach((row) => {
    const cells: string[] = []
    row.forEach((cell) => cells.push(cell.textContent))
    rows.push(cells)
  })
  const identity = JSON.stringify([count, rows])
  let hash = 5381
  for (const character of identity) hash = ((hash << 5) + hash) ^ character.codePointAt(0)!
  return `${count}:${(hash >>> 0).toString(36)}`
}
