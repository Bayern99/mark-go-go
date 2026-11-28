package nativeeditor

import (
	"strconv"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// showingHandles reports whether a table shows its handle slots: while it is
// hovered, part of it is chosen, or a cell has the keyboard. A pointer that
// cannot hover shows them always.
func (m *readingTable) showingHandles(c *ui.Context, hovered bool) bool {
	return hovered || m.selected || m.focusedCell() || !c.Preferences().Hover()
}

// focusedCell reports whether one of a table's cells has the keyboard.
func (m *readingTable) focusedCell() bool {
	for _, row := range m.cells {
		for _, child := range row {
			if child != nil && child.state.Focused() {
				return true
			}
		}
	}
	return false
}

// rowSlot draws the handle slot beside one row, inside the table's scrolling
// content and pinned to its left edge: the grip that selects the row and the
// pin at the row's lower edge that adds a row below it.
func (m *readingTable) rowSlot(c *ui.Context, doc *Document, row int, visible bool) ui.Element {
	slot := ui.Column(c.Key("row slot " + strconv.Itoa(row))).Width(portrait.Editor.TableGutter).Shrink(0).
		Sticky().Left(0).Justify(ui.Center).AlignItems(ui.Center).AccessHidden(true)
	slot.Children(func() {
		if !visible {
			return
		}
		adding := m.rowPin(c, doc, row)
		m.rowGrip(c, doc, row, !adding)
	})
	return slot
}

// rowGrip selects a whole row, and Shift-click extends the rows chosen. It
// steps aside while the row's pin shows.
func (m *readingTable) rowGrip(c *ui.Context, doc *Document, row int, shown bool) {
	grip := ui.Box(c.Key("row grip")).Width(portrait.Editor.TableHandle).Height(portrait.Editor.TableHandle).
		Radius(portrait.Radii.XS).Label("Select table row " + strconv.Itoa(row+1)).Role(ui.RoleButton).KeepFocus()
	if m.kind == tableRows && m.chosen(cellPosition{row, 0}) {
		grip.Background(portrait.Colors.Accent)
	} else {
		grip.Background(portrait.Colors.LineStrong)
	}
	if !shown {
		grip.Opacity(0).IgnoreInput(true).AccessHidden(true)
	}
	open := ui.Local(grip, "Portrait context menu", func() bool { return false })
	portrait.ContextMenu(c, grip, open, func(menu *portrait.MenuBuilder) {
		m.menuItems(c, doc, menu, cellPosition{row: row, col: m.anchor.col})
	})
	grip.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputPointerDown || ev.Button != 0 {
			return false
		}
		if ev.Mods&ui.Shift != 0 {
			m.extendRows(row)
		} else {
			m.selectRows(row, row)
		}
		c.Invalidate()
		return true
	})
}

// rowPin adds a row below one row from the pin at its lower edge, and reports
// whether the pointer is over it.
func (m *readingTable) rowPin(c *ui.Context, doc *Document, row int) bool {
	label := "Insert table row below row " + strconv.Itoa(row+1)
	zone := ui.Box(c.Key("row pin")).Absolute().Bottom(0).Left(0).FillWidth().
		Height(portrait.Editor.TableInsertZone).Label(label).Role(ui.RoleButton).KeepFocus()
	return m.insertPin(c, doc, zone, true, tableOp{kind: tableInsertRowBelow, row: row})
}

// columnSlot draws the handle slot above one column: the grip that selects
// the column and the pin at its right edge that adds a column after it.
func (m *readingTable) columnSlot(c *ui.Context, doc *Document, col int, visible bool) ui.Element {
	slot := ui.Column(c.Key("column slot " + strconv.Itoa(col))).Width(m.widths[col]).Shrink(0).
		AlignItems(ui.Center).Justify(ui.Center)
	slot.Children(func() {
		if !visible {
			return
		}
		adding := m.columnPin(c, doc, col)
		m.columnGrip(c, doc, col, !adding)
	})
	return slot
}

// columnGrip selects a whole column, and Shift-click extends the columns
// chosen. It steps aside while the column's pin shows.
func (m *readingTable) columnGrip(c *ui.Context, doc *Document, col int, shown bool) {
	grip := ui.Box(c.Key("column grip")).Width(portrait.Editor.TableHandle).Height(portrait.Editor.TableHandle).
		Radius(portrait.Radii.XS).Label("Select table column " + strconv.Itoa(col+1)).Role(ui.RoleButton).KeepFocus()
	if m.kind == tableCols && m.chosen(cellPosition{0, col}) {
		grip.Background(portrait.Colors.Accent)
	} else {
		grip.Background(portrait.Colors.LineStrong)
	}
	if !shown {
		grip.Opacity(0).IgnoreInput(true).AccessHidden(true)
	}
	open := ui.Local(grip, "Portrait context menu", func() bool { return false })
	portrait.ContextMenu(c, grip, open, func(menu *portrait.MenuBuilder) {
		m.menuItems(c, doc, menu, cellPosition{row: m.anchor.row, col: col})
	})
	grip.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputPointerDown || ev.Button != 0 {
			return false
		}
		if ev.Mods&ui.Shift != 0 {
			m.extendColumns(col)
		} else {
			m.selectColumns(col, col)
		}
		c.Invalidate()
		return true
	})
}

// columnPin adds a column after one column from the pin at its right edge,
// and reports whether the pointer is over it.
func (m *readingTable) columnPin(c *ui.Context, doc *Document, col int) bool {
	label := "Insert table column right of column " + strconv.Itoa(col+1)
	zone := ui.Box(c.Key("column pin")).Absolute().Right(0).Top(0).Bottom(0).
		Width(portrait.Editor.TableInsertZone).Label(label).Role(ui.RoleButton).KeepFocus()
	return m.insertPin(c, doc, zone, false, tableOp{kind: tableInsertColumnRight, col: col, row: m.anchor.row})
}

// insertPin shows a boundary's insertion line and its + as the pointer comes
// over the zone, and inserts there as it is clicked.
func (m *readingTable) insertPin(c *ui.Context, doc *Document, zone ui.Element, horizontal bool, op tableOp) bool {
	hot := zone.Hovered()
	if hot {
		zone.Children(func() {
			line := ui.Box(c.Key("insert line")).Absolute().Background(portrait.Colors.Accent).IgnoreInput(true).AccessHidden(true)
			pin := ui.Box(c.Key("insert pin")).Absolute().IgnoreInput(true).AccessHidden(true).Radius(portrait.Radii.Pill).Background(portrait.Colors.Accent)
			// The line marks the boundary, and the + sits inside the table
			// beside it: a pin that hangs past the last row or column would
			// be clipped away.
			if horizontal {
				line.Left(0).Right(0).Bottom(0).Height(portrait.Editor.TableIndicator)
				pin.Left((portrait.Editor.TableGutter - portrait.Editor.TablePin) / 2).Bottom(portrait.Editor.TableIndicator).
					Width(portrait.Editor.TablePin).Height(portrait.Editor.TablePin)
			} else {
				line.Top(0).Bottom(0).Right(0).Width(portrait.Editor.TableIndicator)
				pin.Top((portrait.Editor.TableGutter - portrait.Editor.TablePin) / 2).Right(portrait.Editor.TableIndicator).
					Width(portrait.Editor.TablePin).Height(portrait.Editor.TablePin)
			}
			pin.Children(func() {
				ui.Row(c.Key("insert glyph")).Fill().AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					portrait.Glyph(c, portrait.IconPlus, portrait.IconSizes.XS, portrait.Colors.OnAccent)
				})
			})
		})
	}
	zone.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputPointerDown || ev.Button != 0 {
			return false
		}
		m.editTable(doc, op)
		c.Invalidate()
		return true
	})
	return hot
}

// extendRows makes the rows from the chosen one to row the selection.
func (m *readingTable) extendRows(row int) {
	if m.selected && m.kind == tableRows {
		m.selectRows(m.anchor.row, row)
		return
	}
	m.selectRows(row, row)
}

// extendColumns makes the columns from the chosen one to col the selection.
func (m *readingTable) extendColumns(col int) {
	if m.selected && m.kind == tableCols {
		m.selectColumns(m.anchor.col, col)
		return
	}
	m.selectColumns(col, col)
}

// tableMenu gives an element of a table its context menu, as Portrait draws
// it: the row and column edits, the alignment of a column and the cells the
// selection empties.
func (m *readingTable) tableMenu(c *ui.Context, doc *Document, target ui.Element, pos cellPosition) ui.Element {
	return portrait.ContextMenu(c, target, &m.menuOpen, func(menu *portrait.MenuBuilder) {
		m.menuItems(c, doc, menu, pos)
	})
}

// menuItems fills a table's context menu. A read-only document, and one
// whose input method is composing, can take none of its structural edits.
func (m *readingTable) menuItems(c *ui.Context, doc *Document, menu *portrait.MenuBuilder, pos cellPosition) {
	locked := doc.state.ReadOnly || len(m.composing) != 0
	columns := m.columns()
	row, col := pos.row, clamp(pos.col, 0, max(0, columns-1))
	if menu.Item(portrait.MenuItemSpec{Label: "Insert Row Above", Icon: portrait.IconPlus, Disabled: locked || row == 0}).Clicked() {
		m.editTable(doc, tableOp{kind: tableInsertRowAbove, row: row, col: col})
	}
	if menu.Item(portrait.MenuItemSpec{Label: "Insert Row Below", Icon: portrait.IconPlus, Disabled: locked}).Clicked() {
		m.editTable(doc, tableOp{kind: tableInsertRowBelow, row: row, col: col})
	}
	if menu.Item(portrait.MenuItemSpec{Label: "Insert Column Left", Icon: portrait.IconPlus, Disabled: locked}).Clicked() {
		m.editTable(doc, tableOp{kind: tableInsertColumnLeft, row: row, col: col})
	}
	if menu.Item(portrait.MenuItemSpec{Label: "Insert Column Right", Icon: portrait.IconPlus, Disabled: locked}).Clicked() {
		m.editTable(doc, tableOp{kind: tableInsertColumnRight, row: row, col: col})
	}
	menu.Separator()
	for _, align := range []struct {
		label string
		align parser.Align
	}{
		{"Align Left", parser.AlignLeft},
		{"Align Center", parser.AlignCenter},
		{"Align Right", parser.AlignRight},
	} {
		current := col < len(m.aligns) && m.aligns[col] == align.align
		if menu.Radio(portrait.MenuItemSpec{Label: align.label, Disabled: locked}, current).Clicked() {
			m.editTable(doc, tableOp{kind: tableAlignColumn, row: row, col: col, align: align.align})
		}
	}
	menu.Separator()
	if menu.Item(portrait.MenuItemSpec{Label: "Clear Cells", Disabled: locked || !m.selected}).Clicked() {
		m.clearCells(doc)
	}
	if menu.Item(portrait.MenuItemSpec{Label: "Delete Row", Icon: portrait.IconTrash, Danger: true, Disabled: locked || row == 0}).Clicked() {
		m.editTable(doc, tableOp{kind: tableDeleteRow, row: row, col: col})
	}
	if menu.Item(portrait.MenuItemSpec{Label: "Delete Column", Icon: portrait.IconTrash, Danger: true, Disabled: locked || columns <= 1}).Clicked() {
		m.editTable(doc, tableOp{kind: tableDeleteColumn, row: row, col: col})
	}
}

// menuClosed gives the keyboard back to the cell whose menu closed, as
// Escape and a chosen item both do.
func (m *readingTable) menuClosed(c *ui.Context) {
	switch {
	case m.menuOpen:
		m.menuWas = true
	case m.menuWas:
		m.menuWas = false
		if child := m.cellAt(m.menuCell); child != nil {
			m.focus = child
			c.Invalidate()
		}
	}
}
