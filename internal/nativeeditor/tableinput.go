package nativeeditor

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// tableSelectionKind is what a table's selection covers: the rectangle
// between its corners, whole rows of it, or whole columns of it.
type tableSelectionKind uint8

const (
	tableRange tableSelectionKind = iota
	tableRows
	tableCols
)

// cellStep is the keyboard's move between a table's cells, by rows and
// columns of the grid.
type cellStep struct{ row, col int }

var (
	stepForward  = cellStep{col: 1}
	stepBackward = cellStep{col: -1}
	stepUp       = cellStep{row: -1}
	stepDown     = cellStep{row: 1}
)

// pendingCellMove is a keyboard move an input method holds back until its
// composition ends, so that no key cuts a candidate list short.
type pendingCellMove struct {
	child *Document
	from  cellPosition
	step  cellStep
}

// cellStepForKey returns the move a navigation key asks of a table's cells.
func cellStepForKey(ev ui.InputEvent) (cellStep, bool) {
	switch ev.Key {
	case ui.KeyTab:
		switch ev.Mods {
		case 0:
			return stepForward, true
		case ui.Shift:
			return stepBackward, true
		}
	case ui.KeyUp:
		if ev.Mods == 0 {
			return stepUp, true
		}
	case ui.KeyDown:
		if ev.Mods == 0 {
			return stepDown, true
		}
	}
	return cellStep{}, false
}

// selectCells marks the rectangle between two cells.
func (m *readingTable) selectCells(from, to cellPosition) {
	m.kind, m.anchor, m.head = tableRange, from, to
	m.selected = from != to
}

// selectRows marks whole rows, from the first to the last, as a spreadsheet's
// row handles do.
func (m *readingTable) selectRows(from, to int) {
	m.kind = tableRows
	m.anchor = cellPosition{row: min(from, to), col: 0}
	m.head = cellPosition{row: max(from, to), col: m.columns() - 1}
	m.selected = true
}

// selectColumns marks whole columns, from the first to the last.
func (m *readingTable) selectColumns(from, to int) {
	m.kind = tableCols
	m.anchor = cellPosition{row: 0, col: min(from, to)}
	m.head = cellPosition{row: len(m.cells) - 1, col: max(from, to)}
	m.selected = true
}

// columns is the grid's width, as the column widths set it.
func (m *readingTable) columns() int { return len(m.widths) }

// cellAt returns the document a cell edits, nil outside the grid.
func (m *readingTable) cellAt(pos cellPosition) *Document {
	if pos.row < 0 || pos.col < 0 || pos.row >= len(m.cells) || pos.col >= len(m.cells[pos.row]) {
		return nil
	}
	return m.cells[pos.row][pos.col]
}

// selectedBounds returns the cells a selection covers: the rectangle between
// its corners, the whole rows, or the whole columns it names.
func (m *readingTable) selectedBounds() (top, bottom, left, right int, ok bool) {
	if !m.selected || len(m.cells) == 0 || m.columns() == 0 {
		return 0, 0, 0, 0, false
	}
	top, bottom = min(m.anchor.row, m.head.row), max(m.anchor.row, m.head.row)
	left, right = min(m.anchor.col, m.head.col), max(m.anchor.col, m.head.col)
	switch m.kind {
	case tableRows:
		left, right = 0, m.columns()-1
	case tableCols:
		top, bottom = 0, len(m.cells)-1
	}
	top, bottom = clamp(top, 0, len(m.cells)-1), clamp(bottom, 0, len(m.cells)-1)
	left, right = clamp(left, 0, m.columns()-1), clamp(right, 0, m.columns()-1)
	return top, bottom, left, right, top <= bottom && left <= right
}

// chosen reports whether a cell is in the table's selection.
func (m *readingTable) chosen(pos cellPosition) bool {
	top, bottom, left, right, ok := m.selectedBounds()
	return ok && pos.row >= top && pos.row <= bottom && pos.col >= left && pos.col <= right
}

// navigate takes a navigation key for the cell at pos: a move within the
// cell stays the editor's, a move to another cell becomes the table's.
func (m *readingTable) navigate(doc *Document, child *Document, from cellPosition, step cellStep) bool {
	if step == stepUp || step == stepDown {
		row, rows, ok := cellVisualRow(child)
		if !ok || step == stepUp && row > 0 || step == stepDown && row < rows-1 {
			return false
		}
	}
	return m.move(doc, from, step)
}

// move focuses the cell a step reaches, keeping the column on a vertical
// move. It reports whether it took the key: the edges of the table take the
// key without moving, so that no keystroke edits the cell instead. Tab in
// the last cell adds a row and goes to its first cell.
func (m *readingTable) move(doc *Document, from cellPosition, step cellStep) bool {
	if step.row != 0 {
		if target, ok := m.vertical(from, step.row); ok {
			m.focusCell(target)
		}
		return true
	}
	if target, ok := m.neighbour(from, step.col > 0); ok {
		m.focusCell(target)
	} else if step.col > 0 && from.row+1 >= len(m.cells) {
		m.appendRow(doc)
	}
	return true
}

// appendRow adds a row below the table's last one and gives it the caret, as
// Tab in the last cell does.
func (m *readingTable) appendRow(doc *Document) bool {
	rows := len(m.cells)
	if rows == 0 {
		return false
	}
	return m.editTable(doc, tableOp{kind: tableInsertRowBelow, row: rows - 1, col: m.anchor.col})
}

// vertical returns the nearest cell of the rows a vertical step reaches in
// the same column.
func (m *readingTable) vertical(from cellPosition, delta int) (cellPosition, bool) {
	for row := from.row + delta; row >= 0 && row < len(m.cells); row += delta {
		pos := cellPosition{row, from.col}
		if m.cellAt(pos) != nil {
			return pos, true
		}
	}
	return from, false
}

// neighbour returns the next cell of the reading order, skipping cells a
// short row leaves out.
func (m *readingTable) neighbour(from cellPosition, forward bool) (cellPosition, bool) {
	columns := m.columns()
	if columns == 0 {
		return from, false
	}
	at := from.row*columns + from.col
	for {
		if forward {
			at++
		} else {
			at--
		}
		if at < 0 || at >= len(m.cells)*columns {
			return from, false
		}
		pos := cellPosition{row: at / columns, col: at % columns}
		if m.cellAt(pos) != nil {
			return pos, true
		}
	}
}

// focusCell gives the cell the keyboard, as a Tab or an arrow between cells
// does, and drops the selection the key leaves behind.
func (m *readingTable) focusCell(pos cellPosition) {
	child := m.cellAt(pos)
	if child == nil {
		return
	}
	m.kind = tableRange
	m.anchor, m.head = pos, pos
	m.selected, m.dragging = false, false
	m.focus = child
}

// cellVisualRow returns the wrapped row its cell's caret is on, and how many
// rows the cell's line takes, as the last frame laid them out. A cell with a
// selection of its own, or a composition running, has no row to leave from.
func cellVisualRow(child *Document) (row, rows int, ok bool) {
	sel := child.state.Selection()
	if sel.Anchor != sel.Head || child.state.Composing() {
		return 0, 0, false
	}
	caret, ok := child.state.Rect(sel.Head)
	if !ok || caret.H <= 0 {
		return 0, 0, false
	}
	line := child.state.Buffer.Pos(sel.Head).Line
	top := float32(child.state.LineTop(line))
	height := float32(child.state.LineTop(line+1)) - top
	rows = max(1, int(math.Round(float64(height/caret.H))))
	row = clamp(int(math.Round(float64((caret.Y-top)/caret.H))), 0, rows-1)
	return row, rows, true
}

// clearCells empties the chosen cells through the shared buffer, as one step
// of undo. A read-only document takes the key and writes nothing, and so does
// a table whose cells moved since it was laid out.
func (m *readingTable) clearCells(doc *Document) bool {
	if doc.state.ReadOnly || doc.Version() != m.version {
		return true
	}
	top, bottom, left, right, ok := m.selectedBounds()
	if !ok {
		return false
	}
	idx := doc.sourceIndex()
	chosen := make([]tableCell, 0, (bottom-top+1)*(right-left+1))
	for row := max(top, 0); row <= bottom && row < len(m.spans); row++ {
		for col := max(left, 0); col <= right && col < len(m.spans[row]); col++ {
			cell := m.spans[row][col]
			if cell.end <= cell.start {
				continue
			}
			a, z := idx.rawAtEditor(cell.start), idx.rawAtEditor(cell.end)
			if z <= a || !validByteBoundary(idx.raw, a) || !validByteBoundary(idx.raw, z) {
				continue
			}
			chosen = append(chosen, tableCell{start: a, end: z, raw: idx.raw[a:z]})
		}
	}
	if len(chosen) == 0 {
		return true
	}
	doc.state.Buffer.Transact(func() {
		for i := len(chosen) - 1; i >= 0; i-- {
			cell := chosen[i]
			_ = doc.Apply(doc.Version(), cell.start, cell.end, cell.raw, "")
		}
	})
	return true
}

// shaped reports whether the cell editors match a table's grid.
func (m *readingTable) shaped(rows, columns int) bool {
	if len(m.cells) != rows {
		return false
	}
	for _, row := range m.cells {
		if len(row) != columns {
			return false
		}
	}
	return true
}

// resize keeps the editors of a table's current grid: rows and columns a
// structural edit removed take their editors, bounds, selection and
// composition with them.
func (m *readingTable) resize(doc *Document, rows, columns int) {
	lost := m.selected && (m.anchor.row >= rows || m.anchor.col >= columns || m.head.row >= rows || m.head.col >= columns)
	for row := rows; row < len(m.cells); row++ {
		for _, child := range m.cells[row] {
			m.retire(doc, child)
		}
		clear(m.cells[row])
	}
	for row := range min(rows, len(m.cells)) {
		kept := m.cells[row][:min(columns, len(m.cells[row]))]
		for _, child := range m.cells[row][len(kept):] {
			m.retire(doc, child)
		}
		clear(m.cells[row])
		m.cells[row] = kept
		for len(m.cells[row]) < columns {
			m.cells[row] = append(m.cells[row], nil)
		}
	}
	for len(m.cells) < rows {
		m.cells = append(m.cells, make([]*Document, columns))
	}
	clear(m.cells[rows:])
	m.cells = m.cells[:rows]
	for pos := range m.bounds {
		if pos.row >= rows || pos.col >= columns {
			delete(m.bounds, pos)
		}
	}
	if rows <= 0 || columns <= 0 {
		m.anchor, m.head = cellPosition{}, cellPosition{}
		m.selected, m.dragging = false, false
		return
	}
	if lost {
		m.selected, m.dragging = false, false
	}
	m.anchor = cellPosition{row: clamp(m.anchor.row, 0, rows-1), col: clamp(m.anchor.col, 0, columns-1)}
	m.head = cellPosition{row: clamp(m.head.row, 0, rows-1), col: clamp(m.head.col, 0, columns-1)}
	if m.selected && m.kind == tableRange && m.anchor == m.head {
		m.selected = false
	}
}

// retire drops an editor a structural edit removed, closing the undo group of
// a composition it was in rather than leaving it open behind the table.
func (m *readingTable) retire(doc *Document, child *Document) {
	if child == nil {
		return
	}
	if _, ok := m.composing[child]; ok {
		doc.state.Buffer.EndComposition(&child.state)
		delete(m.composing, child)
	}
	if m.focus == child {
		m.focus = nil
	}
	if pending := m.pending; pending != nil && pending.child == child {
		m.pending = nil
	}
}

func clamp(v, low, high int) int {
	if high < low {
		return low
	}
	return max(low, min(v, high))
}
