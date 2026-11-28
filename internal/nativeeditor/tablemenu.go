package nativeeditor

import (
	"strconv"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

func (m *readingTable) showingHandles(c *ui.Context, hovered bool) bool {
	return hovered || m.selected || m.focusedCell() || !c.Preferences().Hover()
}

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

func (m *readingTable) rowSlot(c *ui.Context, doc *Document, row int, visible bool) ui.Element {
	slot := ui.Column(c.Key("row slot " + strconv.Itoa(row))).Width(portrait.Editor.TableGutter).Shrink(0).
		Sticky().Left(0).Justify(ui.Center).AlignItems(ui.Center)
	slot.Children(func() {
		if !visible {
			return
		}
		adding := m.rowPin(c, doc, row)
		m.tableGrip(c, doc, tableGripSpec{
			key:      "row grip",
			label:    "Select table row " + strconv.Itoa(row+1),
			selected: m.kind == tableRows && m.chosen(cellPosition{row, 0}),
			shown:    !adding,
			menuPos:  cellPosition{row: row, col: m.anchor.col},
			focusPos: cellPosition{row: row, col: 0},
			onSelect: func() { m.selectRows(row, row) },
			onExtend: func() { m.extendRows(row) },
		})
	})
	return slot
}

func (m *readingTable) rowPin(c *ui.Context, doc *Document, row int) bool {
	label := "Insert table row below row " + strconv.Itoa(row+1)
	zone := ui.Box(c.Key("row pin")).Absolute().Bottom(0).Left(0).FillWidth().
		Height(portrait.Editor.TableInsertZone).Label(label).Role(ui.RoleButton).KeepFocus()
	return m.insertPin(c, doc, zone, true, tableOp{kind: tableInsertRowBelow, row: row, col: m.anchor.col})
}

func (m *readingTable) columnSlot(c *ui.Context, doc *Document, col int, visible bool) ui.Element {
	slot := ui.Column(c.Key("column slot " + strconv.Itoa(col))).Width(m.widths[col]).Shrink(0).
		AlignItems(ui.Center).Justify(ui.Center)
	slot.Children(func() {
		if !visible {
			return
		}
		adding := m.columnPin(c, doc, col)
		m.tableGrip(c, doc, tableGripSpec{
			key:      "column grip",
			label:    "Select table column " + strconv.Itoa(col+1),
			selected: m.kind == tableCols && m.chosen(cellPosition{0, col}),
			shown:    !adding,
			menuPos:  cellPosition{row: m.anchor.row, col: col},
			focusPos: cellPosition{row: 0, col: col},
			onSelect: func() { m.selectColumns(col, col) },
			onExtend: func() { m.extendColumns(col) },
		})
	})
	return slot
}

type tableGripSpec struct {
	key      string
	label    string
	selected bool
	shown    bool
	menuPos  cellPosition
	focusPos cellPosition
	onSelect func()
	onExtend func()
}

func (m *readingTable) tableGrip(c *ui.Context, doc *Document, spec tableGripSpec) {
	grip := ui.Box(c.Key(spec.key)).Width(portrait.Editor.TableHandle).Height(portrait.Editor.TableHandle).
		Radius(portrait.Radii.XS).Label(spec.label).Role(ui.RoleButton).KeepFocus()
	if spec.selected {
		grip.Background(portrait.Colors.Accent)
	} else {
		grip.Background(portrait.Colors.LineStrong)
	}
	if !spec.shown {
		grip.Opacity(0).IgnoreInput(true).AccessHidden(true)
	}
	m.bindTableContextMenu(c, doc, grip, spec.menuPos)
	grip.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputPointerDown || ev.Button != 0 {
			return false
		}
		if ev.Mods&ui.Shift != 0 {
			spec.onExtend()
		} else {
			spec.onSelect()
		}
		if len(m.composing) == 0 {
			m.focusSelectionAnchor(spec.focusPos)
		}
		c.Invalidate()
		return true
	})
}

func (m *readingTable) bindTableContextMenu(c *ui.Context, doc *Document, target ui.Element, pos cellPosition) {
	open := ui.Local(target, "Portrait context menu", func() bool { return false })
	if len(m.composing) != 0 {
		*open = false
		return
	}
	portrait.ContextMenu(c, target, open, func(menu *portrait.MenuBuilder) {
		m.menuItems(c, doc, menu, pos)
	})
}

func (m *readingTable) columnPin(c *ui.Context, doc *Document, col int) bool {
	label := "Insert table column right of column " + strconv.Itoa(col+1)
	zone := ui.Box(c.Key("column pin")).Absolute().Right(0).Top(0).Bottom(0).
		Width(portrait.Editor.TableInsertZone).Label(label).Role(ui.RoleButton).KeepFocus()
	return m.insertPin(c, doc, zone, false, tableOp{kind: tableInsertColumnRight, col: col, row: m.anchor.row})
}

func (m *readingTable) insertPin(c *ui.Context, doc *Document, zone ui.Element, horizontal bool, op tableOp) bool {
	hot := zone.Hovered()
	if hot {
		zone.Children(func() {
			line := ui.Box(c.Key("insert line")).Absolute().Background(portrait.Colors.Accent).IgnoreInput(true).AccessHidden(true)
			pin := ui.Box(c.Key("insert pin")).Absolute().IgnoreInput(true).AccessHidden(true).Radius(portrait.Radii.Pill).Background(portrait.Colors.Accent)
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
	m.bindTableContextMenu(c, doc, zone, cellPosition{row: op.row, col: op.col})
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

func (m *readingTable) extendRows(row int) {
	if m.selected && m.kind == tableRows {
		m.selectRows(m.anchor.row, row)
		return
	}
	m.selectRows(row, row)
}

func (m *readingTable) extendColumns(col int) {
	if m.selected && m.kind == tableCols {
		m.selectColumns(m.anchor.col, col)
		return
	}
	m.selectColumns(col, col)
}

func (m *readingTable) tableMenu(c *ui.Context, doc *Document, target ui.Element, pos cellPosition) ui.Element {
	return portrait.ContextMenu(c, target, &m.menuOpen, func(menu *portrait.MenuBuilder) {
		m.menuItems(c, doc, menu, pos)
	})
}

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
