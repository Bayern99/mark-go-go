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
const CELL_MIN_WIDTH = 72
const DEFAULT_CELL_MIN_WIDTH = 112
const PROSE_MEASURE = 624
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
    cellMinWidth: CELL_MIN_WIDTH,
    defaultCellMinWidth: DEFAULT_CELL_MIN_WIDTH,
    View: null,
  })
  resizing.props.decorations = undefined
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
      const guide = createResizeGuide()
      const sync = () => syncTables(view)
      const updateHandle = (handle: number) => {
        if (handle >= 0) {
          guide.showForCellPos(view, handle)
        } else {
          guide.hide()
        }
        if (columnResizingPluginKey.getState(view.state)?.activeHandle === handle) return
        view.dispatch(view.state.tr.setMeta(columnResizingPluginKey, { setHandle: handle }))
      }
      const onMouseDown = (event: MouseEvent) => {
        if (event.button !== 0 || !view.editable) return
        if (columnResizingPluginKey.getState(view.state)?.dragging) return
        sync()
        const handle = columnHandleAt(view, event)
        updateHandle(handle)
        resize = handle >= 0 ? startColumnResize(view, event, handle, guide) : undefined
        selecting = !resize
        if (resize) {
          event.preventDefault()
          event.stopImmediatePropagation()
        }
      }
      const onMouseMove = (event: MouseEvent) => {
        if (columnResizingPluginKey.getState(view.state)?.dragging) {
          return
        }
        if (selecting || !view.editable) return
        updateHandle(columnHandleAt(view, event))
      }
      const onMouseLeave = () => {
        if (columnResizingPluginKey.getState(view.state)?.dragging) return
        updateHandle(-1)
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
        guide.hide()
      }

      sync()
      view.dom.addEventListener('mousedown', onMouseDown, true)
      view.dom.addEventListener('pointermove', onPointerMove, true)
      view.dom.addEventListener('mouseleave', onMouseLeave)
      window.addEventListener('mousemove', onMouseMove, true)
      window.addEventListener('mouseup', onEnd, true)
      window.addEventListener('blur', onBlur)
      return {
        update(currentView, previousState) {
          if (currentView.state.doc !== previousState.doc) {
            resize?.cancel()
            resize = undefined
            guide.hide()
            sync()
            persist(currentView, storageKey, stored)
            return
          }
          if (resize) {
            resize.reapply()
          } else {
            syncIfMissing(currentView)
            const activeHandle = columnResizingPluginKey.getState(currentView.state)?.activeHandle ?? -1
            if (activeHandle >= 0) {
              guide.showForCellPos(currentView, activeHandle)
            } else {
              guide.hide()
            }
          }
        },
        destroy() {
          resize?.dispose()
          resize = undefined
          guide.hide()
          view.dom.removeEventListener('mousedown', onMouseDown, true)
          view.dom.removeEventListener('pointermove', onPointerMove, true)
          view.dom.removeEventListener('mouseleave', onMouseLeave)
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
  if (event.target.closest('.cell-handle, .add-button')) return -1
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

function computeResizedWidths(
  startWidths: readonly number[],
  col: number,
  rawDelta: number,
  minWidth = CELL_MIN_WIDTH,
): number[] {
  const next = [...startWidths]
  const delta = Math.round(rawDelta)
  const left = startWidths[col]
  if (left === undefined) return next

  if (col >= startWidths.length - 1) {
    next[col] = Math.max(minWidth, left + delta)
    return next
  }

  const right = startWidths[col + 1]!
  const applied = Math.min(right - minWidth, Math.max(minWidth - left, delta))
  next[col] = left + applied
  next[col + 1] = right - applied
  return next
}

function measureRenderedColumnWidths(
  tableDom: HTMLTableElement,
  tableNode: PMNode,
): number[] {
  const map = TableMap.get(tableNode)
  const count = map.width
  const measured = Array<number>(count).fill(0)
  const rows = Array.from(tableDom.rows)

  for (let r = 0; r < rows.length; r += 1) {
    const row = rows[r]!
    let col = 0
    for (const cell of Array.from(row.cells)) {
      const span = cell.colSpan || 1
      if (span === 1 && col < count && measured[col] === 0) {
        measured[col] = cell.getBoundingClientRect().width
      }
      col += span
    }
    if (measured.every((w) => w > 0)) break
  }

  if (measured.some((w) => w <= 0)) {
    const firstRow = rows[0]
    if (firstRow) {
      let col = 0
      for (const cell of Array.from(firstRow.cells)) {
        const span = cell.colSpan || 1
        const share = cell.getBoundingClientRect().width / span
        for (let i = 0; i < span && col + i < count; i += 1) {
          if (measured[col + i]! <= 0) measured[col + i] = share
        }
        col += span
      }
    }
  }

  const rawTotal = measured.reduce((sum, w) => sum + (w || DEFAULT_CELL_MIN_WIDTH), 0)
  const rounded = measured.map((w) => Math.max(CELL_MIN_WIDTH, Math.round(w || DEFAULT_CELL_MIN_WIDTH)))
  const targetTotal = Math.round(rawTotal)
  let residual = targetTotal - rounded.reduce((sum, w) => sum + w, 0)
  if (residual !== 0 && rounded.length > 0) {
    let targetIndex = 0
    for (let i = 1; i < rounded.length; i += 1) {
      if (rounded[i]! > rounded[targetIndex]!) targetIndex = i
    }
    const adjusted = rounded[targetIndex]! + residual
    if (adjusted >= CELL_MIN_WIDTH) rounded[targetIndex] = adjusted
  }
  return rounded
}

function tableBlockOf(tableDom: HTMLTableElement, viewDom: HTMLElement): HTMLElement | null {
  const block = tableDom.closest<HTMLElement>('.milkdown-table-block')
  return block && block.parentElement === viewDom ? block : null
}

function setTableBlockFrame(block: HTMLElement | null, mode: 'prose' | 'full' | number): void {
  if (!block) return
  block.style.marginLeft = ''
  block.style.marginRight = ''
  if (mode === 'full') {
    block.style.maxWidth = 'none'
  } else if (typeof mode === 'number' && mode > PROSE_MEASURE) {
    block.style.maxWidth = `${Math.round(mode)}px`
  } else {
    block.style.maxWidth = ''
  }
}

function blockLeftOffset(block: HTMLElement, viewDom: HTMLElement): number {
  const proseRect = viewDom.getBoundingClientRect()
  const proseStyle = getComputedStyle(viewDom)
  const contentLeft =
    proseRect.left +
    (Number.parseFloat(proseStyle.borderLeftWidth) || 0) +
    (Number.parseFloat(proseStyle.paddingLeft) || 0)
  return Math.max(0, block.getBoundingClientRect().left - contentLeft)
}

function hideBlockHandle(view: EditorView): void {
  const root = view.dom.closest('.milkdown') ?? view.dom.parentElement
  root?.querySelectorAll<HTMLElement>('.milkdown-block-handle').forEach((handle) => {
    handle.dataset.show = 'false'
  })
}

function resetTableHandles(block: HTMLElement | null): void {
  if (!block) return
  block.querySelectorAll<HTMLElement>('.handle').forEach((handle) => {
    handle.dataset.show = 'false'
    handle.style.left = ''
    handle.style.top = ''
  })
}

interface ResizeGuide {
  showForCellPos: (view: EditorView, cellPos: number) => void
  showForColumn: (table: HTMLTableElement, widths: readonly number[], col: number) => void
  hide: () => void
}

function createResizeGuide(): ResizeGuide {
  const el = document.createElement('div')
  el.className = 'column-resize-handle'
  const place = (wrapper: HTMLElement, table: HTMLTableElement, leftPx: number) => {
    const wrapperRect = wrapper.getBoundingClientRect()
    const tableRect = table.getBoundingClientRect()
    if (el.parentElement !== wrapper) wrapper.appendChild(el)
    el.style.left = `${leftPx - 1}px`
    el.style.top = `${tableRect.top - wrapperRect.top + wrapper.scrollTop}px`
    el.style.height = `${tableRect.height}px`
  }
  return {
    showForCellPos(view, cellPos) {
      if (cellPos < 0 || cellPos >= view.state.doc.content.size) {
        el.remove()
        return
      }
      const dom = view.nodeDOM(cellPos)
      const cell = dom instanceof HTMLElement ? dom : null
      const table = cell?.closest<HTMLTableElement>('table.children')
      const wrapper = table?.closest<HTMLElement>('.table-wrapper')
      if (!cell || !table || !wrapper) {
        el.remove()
        return
      }
      const wrapperRect = wrapper.getBoundingClientRect()
      const cellRect = cell.getBoundingClientRect()
      place(wrapper, table, cellRect.right - wrapperRect.left + wrapper.scrollLeft)
    },
    showForColumn(table, widths, col) {
      const wrapper = table.closest<HTMLElement>('.table-wrapper')
      if (!wrapper) {
        el.remove()
        return
      }
      let offsetX = 0
      for (let i = 0; i <= col && i < widths.length; i += 1) {
        offsetX += Math.max(CELL_MIN_WIDTH, Math.round(widths[i] ?? DEFAULT_CELL_MIN_WIDTH))
      }
      const wrapperRect = wrapper.getBoundingClientRect()
      const tableRect = table.getBoundingClientRect()
      place(wrapper, table, tableRect.left - wrapperRect.left + wrapper.scrollLeft + offsetX)
    },
    hide() {
      el.remove()
    },
  }
}

function applyFixedColumnWidths(
  tableDom: HTMLTableElement,
  colgroup: HTMLTableColElement,
  widths: readonly number[],
  block: HTMLElement | null = null,
): void {
  const count = widths.length
  if (colgroup.childElementCount !== count) {
    colgroup.replaceChildren(
      ...Array.from({ length: count }, () => document.createElement('col')),
    )
  }
  let total = 0
  for (let i = 0; i < count; i += 1) {
    const width = Math.max(CELL_MIN_WIDTH, Math.round(widths[i] ?? DEFAULT_CELL_MIN_WIDTH))
    total += width
    const colEl = colgroup.children[i] as HTMLElement
    colEl.style.width = `${width}px`
    colEl.style.minWidth = ''
  }
  tableDom.classList.add('mgo-table-sized')
  tableDom.style.tableLayout = 'fixed'
  tableDom.style.minWidth = '0px'
  tableDom.style.width = `${total}px`
  if (block) setTableBlockFrame(block, total > PROSE_MEASURE ? total : 'prose')
}

function tableElementAt(view: EditorView, tableStart: number): HTMLTableElement | null {
  let dom: HTMLElement | null = view.domAtPos(tableStart).node as HTMLElement
  while (dom && dom.nodeName !== 'TABLE') dom = dom.parentElement
  return dom instanceof HTMLTableElement ? dom : null
}

function ensureColgroup(tableDom: HTMLTableElement): HTMLTableColElement {
  let colgroup = tableDom.querySelector<HTMLTableColElement>(':scope > colgroup')
  if (!colgroup) {
    colgroup = document.createElement('colgroup')
    tableDom.insertBefore(colgroup, tableDom.firstChild)
  }
  return colgroup
}

function startColumnResize(
  view: EditorView,
  event: MouseEvent,
  cellPos: number,
  guide: ResizeGuide,
): { cancel: () => void; dispose: () => void; reapply: () => void } | undefined {
  const cell = view.state.doc.nodeAt(cellPos)
  if (!cell) return undefined
  const $cell = view.state.doc.resolve(cellPos)
  const tableNode = $cell.node(-1)
  if (tableNode.type.name !== 'table' || !$cell.nodeAfter) return undefined

  const tableStart = $cell.start(-1)
  const initialTableDom = tableElementAt(view, tableStart)
  if (!initialTableDom) return undefined
  const map = TableMap.get(tableNode)
  const col = map.colCount($cell.pos - tableStart) + ($cell.nodeAfter.attrs.colspan as number) - 1
  const startX = event.clientX
  const startWidths = measureRenderedColumnWidths(initialTableDom, tableNode)
  const startWidth = startWidths[col] ?? DEFAULT_CELL_MIN_WIDTH
  let latestWidths = startWidths
  const initialBlock = tableBlockOf(initialTableDom, view.dom)
  const pinnedPrevMaxWidth = initialBlock?.style.maxWidth ?? ''
  const pinnedLeftOffset = initialBlock ? blockLeftOffset(initialBlock, view.dom) : 0
  hideBlockHandle(view)
  resetTableHandles(initialBlock)

  const applyLive = (widths: readonly number[]) => {
    latestWidths = [...widths]
    const liveTable = tableElementAt(view, tableStart)
    if (!liveTable) return
    const liveBlock = tableBlockOf(liveTable, view.dom)
    if (liveBlock) {
      liveBlock.style.marginLeft = `${pinnedLeftOffset}px`
      liveBlock.style.marginRight = '0px'
      liveBlock.style.maxWidth = 'none'
    }
    const liveColgroup = ensureColgroup(liveTable)
    applyFixedColumnWidths(liveTable, liveColgroup, widths)
    guide.showForColumn(liveTable, widths, col)
  }

  view.dom.classList.add('mgo-column-resizing')
  view.dispatch(view.state.tr.setMeta(columnResizingPluginKey, {
    setDragging: { startX, startWidth },
  }))
  applyLive(startWidths)

  let settled = false
  let moved = false
  const widthsAt = (pointer: MouseEvent) =>
    computeResizedWidths(startWidths, col, pointer.clientX - startX, CELL_MIN_WIDTH)

  const onKeyDown = (keyEvent: KeyboardEvent) => {
    if (keyEvent.key !== 'Escape') return
    keyEvent.preventDefault()
    keyEvent.stopPropagation()
    cancel()
  }
  const dispose = () => {
    settled = true
    const liveTable = tableElementAt(view, tableStart)
    const liveBlock = liveTable ? tableBlockOf(liveTable, view.dom) : initialBlock
    if (liveBlock) {
      liveBlock.style.marginLeft = ''
      liveBlock.style.marginRight = ''
      liveBlock.style.maxWidth = pinnedPrevMaxWidth
      resetTableHandles(liveBlock)
    }
    guide.hide()
    hideBlockHandle(view)
    view.dom.classList.remove('mgo-column-resizing')
    window.removeEventListener('mousemove', move)
    window.removeEventListener('mouseup', finish)
    window.removeEventListener('keydown', onKeyDown, true)
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
  const reapply = () => {
    if (settled) return
    applyLive(latestWidths)
  }
  const move = (pointer: MouseEvent) => {
    if (settled || !columnResizingPluginKey.getState(view.state)?.dragging) return
    if (pointer.clientX !== startX) moved = true
    applyLive(widthsAt(pointer))
  }
  const finish = (pointer: MouseEvent) => {
    if (settled) return
    dispose()
    if (!columnResizingPluginKey.getState(view.state)?.dragging) return
    const finalWidths = widthsAt(pointer)
    view.dispatch(view.state.tr.setMeta(columnResizingPluginKey, { setDragging: null }))
    if (!moved && pointer.clientX === startX) {
      syncTables(view)
      return
    }
    if (!setAllColumnWidths(view, tableStart, finalWidths)) {
      syncTables(view)
    }
  }
  window.addEventListener('mousemove', move)
  window.addEventListener('mouseup', finish)
  window.addEventListener('keydown', onKeyDown, true)
  return { cancel, dispose, reapply }
}

function setAllColumnWidths(view: EditorView, tableStart: number, widths: readonly number[]): boolean {
  const $table = view.state.doc.resolve(tableStart)
  const table = $table.parent
  if (table.type.name !== 'table') return false
  const map = TableMap.get(table)
  if (map.width !== widths.length) return false
  const tr = view.state.tr
  const seen = new Set<number>()
  for (let row = 0; row < map.height; row += 1) {
    for (let col = 0; col < map.width; col += 1) {
      const pos = map.map[row * map.width + col]
      if (pos === undefined || seen.has(pos)) continue
      seen.add(pos)
      const cell = table.nodeAt(pos)
      if (!cell) continue
      const colspan = cell.attrs.colspan as number
      const next = widths.slice(col, col + colspan)
      const current = cell.attrs.colwidth as (number | null)[] | null
      if (current && current.length === next.length && current.every((v, i) => v === next[i])) {
        continue
      }
      tr.setNodeMarkup(tableStart + pos, undefined, { ...cell.attrs, colwidth: next })
    }
  }
  if (!tr.docChanged) return false
  view.dispatch(tr)
  return true
}

export function tableSourceEquals(a: PMNode, b: PMNode): boolean {
  if (a.type.name !== 'table' || b.type.name !== 'table') return a.eq(b)
  return withoutColumnWidths(a).eq(withoutColumnWidths(b))
}

function syncIfMissing(view: EditorView): void {
  view.state.doc.forEach((node, offset) => {
    if (node.type.name !== 'table') return
    const block = view.nodeDOM(offset)
    const table =
      block instanceof Element
        ? block.querySelector<HTMLTableElement>('table.children')
        : null
    if (!table) return
    const blockEl = tableBlockOf(table, view.dom)
    const existingColgroup = table.querySelector<HTMLTableColElement>(':scope > colgroup')
    const saved = readWidths(node)
    const hasCustom = saved.some((width) => width !== null)
    const expectedCount = columnCount(node)
    if (
      existingColgroup &&
      existingColgroup.childElementCount === expectedCount &&
      (!hasCustom || table.classList.contains('mgo-table-sized'))
    ) {
      return
    }
    const colgroup = existingColgroup ?? ensureColgroup(table)
    if (hasCustom) {
      if (saved.every((width): width is number => width !== null)) {
        applyFixedColumnWidths(table, colgroup, saved, blockEl)
      } else {
        table.classList.remove('mgo-table-sized')
        table.style.tableLayout = ''
        const approxTotal = saved.reduce<number>((sum, w) => sum + (w ?? DEFAULT_CELL_MIN_WIDTH), 0)
        setTableBlockFrame(blockEl, approxTotal > PROSE_MEASURE ? approxTotal : 'prose')
        updateColumnsOnResize(node, colgroup, table, DEFAULT_CELL_MIN_WIDTH)
      }
      return
    }
    layoutDefaultTable(table, node, colgroup, blockEl)
  })
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
    const blockEl = tableBlockOf(table, view.dom)
    const colgroup = ensureColgroup(table)
    const saved = readWidths(node)
    if (saved.some((width) => width !== null)) {
      if (saved.every((width): width is number => width !== null)) {
        applyFixedColumnWidths(table, colgroup, saved, blockEl)
      } else {
        table.classList.remove('mgo-table-sized')
        table.style.tableLayout = ''
        const approxTotal = saved.reduce<number>((sum, w) => sum + (w ?? DEFAULT_CELL_MIN_WIDTH), 0)
        setTableBlockFrame(blockEl, approxTotal > PROSE_MEASURE ? approxTotal : 'prose')
        updateColumnsOnResize(node, colgroup, table, DEFAULT_CELL_MIN_WIDTH)
      }
      return
    }
    layoutDefaultTable(table, node, colgroup, blockEl)
  })
}

function layoutDefaultTable(
  table: HTMLTableElement,
  node: PMNode,
  colgroup: HTMLTableColElement,
  block: HTMLElement | null = null,
): void {
  table.classList.remove('mgo-table-sized')
  table.style.tableLayout = ''
  const count = columnCount(node)
  if (colgroup.childElementCount !== count) {
    colgroup.replaceChildren(
      ...Array.from({ length: count }, () => document.createElement('col')),
    )
  }
  const preferred = preferredWidths(table, count)
  const total = preferred.reduce((sum, width) => sum + width, 0)
  const minTotal = count * DEFAULT_CELL_MIN_WIDTH
  setTableBlockFrame(block, Math.max(total, minTotal) > PROSE_MEASURE ? 'full' : 'prose')
  table.style.width = '100%'
  table.style.minWidth = `${minTotal}px`
  preferred.forEach((width, index) => {
    const colEl = colgroup.children[index] as HTMLElement
    colEl.style.width = `${(width / total) * 100}%`
    colEl.style.minWidth = ''
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
