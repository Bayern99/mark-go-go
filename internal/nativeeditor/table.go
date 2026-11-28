package nativeeditor

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type readingTable struct {
	signature, preferenceKey     string
	seen, version, widthStateID  uint64
	scroll                       ui.ScrollState
	focus                        *Document
	cells                        [][]*Document
	spans                        [][]tableCell
	composing                    map[*Document]tableCell
	pending                      *pendingCellMove
	widths                       []float32
	minimum                      []float32
	manual                       bool
	aligns                       []parser.Align
	rawStart, rawEnd             int
	target                       cellPosition
	hasTarget                    bool
	menuOpen, menuWas            bool
	menuCell                     cellPosition
	pendingEdit                  *tableEdit
	resave                       bool
	bounds                       map[cellPosition]ui.Rect
	kind                         tableSelectionKind
	anchor, head                 cellPosition
	dragging, selected           bool
	dragX, leftWidth, rightWidth float32
	dragCol, hoverCol            int
}
type tableCell struct {
	start, end int
	raw        string
}

func tableCellEdges(row, col int) (top, right, bottom, left bool) {
	return row == 0, true, true, col == 0
}

func tableTextAlign(align parser.Align) editor.TextAlign {
	switch align {
	case parser.AlignCenter:
		return editor.AlignCenter
	case parser.AlignRight:
		return editor.AlignRight
	default:
		return editor.AlignLeft
	}
}

func (p *previewLayout) standIn(snap editor.Snapshot, start, end int, visible string, block editor.Block) {
	block.Interactive = true
	p.hideLines(snap, start, end, visible, block)
}

func (p *previewLayout) table(snap editor.Snapshot, idx int) {
	doc := p.owner
	if doc.tables == nil {
		doc.tables = map[int]*readingTable{}
	}
	b := p.document.Block(idx)
	if pending := doc.tables[b.Start]; pending != nil {
		pending.settle(doc)
	}
	ncols := len(b.Align)
	if ncols == 0 {
		return
	}
	rows := [][]tableCell{}
	var visible []string
	var labels [][]string
	for row := range p.document.Children(idx) {
		var cells []tableCell
		var text []string
		for col := range p.document.Children(row) {
			cell := p.document.Block(col)
			from, end := cell.Start, cell.End
			if p.document.LineCount(col) > 0 {
				content, at := p.document.Line(col, 0)
				from, end = at, at+len(content)
			}
			if from < 0 || end > snap.Len() || end < from {
				continue
			}
			raw := snap.Slice(from, end)
			cells = append(cells, tableCell{from, end, raw})
			text = append(text, inlineText(p.document, col))
		}
		rows = append(rows, cells)
		labels = append(labels, text)
		visible = append(visible, strings.Join(text, "\t"))
	}
	sig := tableSignature(ncols, labels)
	preferenceKey := tablePreferenceKey(p.tableOrdinal(idx), ncols, firstTableRow(labels))
	model := doc.tables[b.Start]
	if model == nil {
		for old, t := range doc.tables {
			if (t.signature == sig || t.preferenceKey == preferenceKey) && t.seen != p.version {
				model = t
				delete(doc.tables, old)
				doc.tables[b.Start] = model
				break
			}
		}
	}
	curStateID := doc.state.Buffer.UndoStateID()
	if model == nil || len(model.widths) != ncols {
		model = &readingTable{bounds: map[cellPosition]ui.Rect{}, widths: make([]float32, ncols), minimum: make([]float32, ncols), dragCol: -1, hoverCol: -1}
		for i := range model.widths {
			model.minimum[i] = portrait.Editor.TableColumnMin
			for _, text := range labels {
				if i < len(text) {
					model.minimum[i] = max(model.minimum[i], min(portrait.Editor.TableColumnMax, 2*portrait.Editor.TableCellPadX+textAdvance(text[i], portrait.Type.Small.Size)))
				}
			}
		}
		model.signature = sig
		model.preferenceKey = preferenceKey
		model.fit(portrait.Editor.Column)
		if !doc.restoreTableHistory(curStateID, model) {
			doc.preferences.restoreTable(model)
		}
		doc.tables[b.Start] = model
	} else if curStateID != model.widthStateID && !model.resave {
		model.signature = sig
		model.preferenceKey = preferenceKey
		doc.restoreTableHistoryExact(curStateID, model)
	}
	model.signature = sig
	model.preferenceKey = preferenceKey
	model.widthStateID = curStateID
	model.aligns = append(model.aligns[:0], b.Align...)
	model.rawStart, model.rawEnd = b.Start, snap.LineEnd(snap.LineOf(b.End))
	if model.manual {
		doc.recordTableWidths(curStateID, preferenceKey, model.widths)
	}
	model.seen, model.version = p.version, p.version
	if !model.shaped(len(rows), ncols) {
		model.resize(doc, len(rows), ncols)
	}
	model.spans = rows
	for r, cells := range rows {
		for k, cell := range cells {
			if k >= ncols {
				break
			}
			child := model.cells[r][k]
			if child == nil {
				child = newTableCell(cell.raw)
				model.cells[r][k] = child
			} else if !cellMatches(child.Source(), cell.raw) {
				child.Reload(cell.raw)
			}
		}
		for k := len(cells); k < ncols; k++ {
			model.retire(doc, model.cells[r][k])
			model.cells[r][k] = nil
		}
	}
	gutter := portrait.Editor.TableGutter
	width := gutter
	for _, w := range model.widths {
		width += w
	}
	version := p.version
	p.standIn(snap, b.Start, snap.LineEnd(snap.LineOf(b.End)), strings.Join(visible, "\n"), editor.Block{Label: "Markdown table", Width: width, Height: model.height(labels) + gutter, Content: func(c *ui.Context, e ui.Element, _ editor.Block) {
		if !model.manual && model.dragCol < 0 && e.LayoutBounds().W > gutter {
			model.fit(e.LayoutBounds().W - gutter)
		}
		gridWidth := gutter
		for _, w := range model.widths {
			gridWidth += w
		}
		model.menuClosed(c)
		handles := model.showingHandles(c, e.Hovered())
		e.Children(func() {
			viewport := ui.Column(c).FillWidth().MaxWidth(gridWidth).ClipInset(0, 0, 0, 0, portrait.Editor.TableRadius)
			viewport.Children(func() {
				ui.ScrollHorizontal(c).FillWidth().TrackScroll(&model.scroll).Children(func() {
					ui.Column(c).Width(gridWidth).Shrink(0).Role(ui.RoleTable).Children(func() {
						ui.Row(c.Key("column handles")).Height(gutter).AlignItems(ui.Stretch).Children(func() {
							ui.Box(c.Key("corner")).Width(gutter).Shrink(0).Sticky().Left(0)
							for k := range model.widths {
								model.columnSlot(c, doc, k, handles)
							}
						})
						for r, cells := range rows {
							ui.Row(c.Key(r)).MinHeight(portrait.Editor.TableRowMin).AlignItems(ui.Stretch).Role(ui.RoleRow).Children(func() {
								model.rowSlot(c, doc, r, handles)
								for k, cell := range cells {
									if k >= ncols {
										break
									}
									child := model.cells[r][k]
									pos := cellPosition{r, k}
									box := ui.Column(c.Key(k)).Width(model.widths[k]).Shrink(0).Padding(portrait.Editor.TableCellPadY, portrait.Editor.TableCellPadX).Justify(ui.Center)
									if pos == model.menuCell {
										model.tableMenu(c, doc, box, pos)
									}
									line := portrait.Colors.Line
									if r == 0 {
										box.Background(portrait.Colors.FillStrong).Role(ui.RoleColumnHeader)
										line = portrait.Colors.LineStrong
									} else {
										box.Role(ui.RoleCell)
									}
									top, right, bottom, left := tableCellEdges(r, k)
									hairline := portrait.Editor.Hairline / 2
									if top {
										box.InsetShadow(0, hairline, 0, 0, line)
									}
									if right {
										box.InsetShadow(-hairline, 0, 0, 0, line)
									}
									if bottom {
										box.InsetShadow(0, -hairline, 0, 0, line)
									}
									if left {
										box.InsetShadow(hairline, 0, 0, 0, line)
									}
									if model.chosen(cellPosition{r, k}) {
										box.Background(c.Theme().Selection)
									}
									box.Children(func() {
										child.state.ReadOnly = doc.state.ReadOnly
										child.prepareView(c)
										child.state.SetBlocks(nil)
										child.state.ReadingColumn = editor.ReadingColumn{}
										child.state.Label = fmt.Sprintf("Table row %d column %d", r+1, k+1)
										child.state.Theme = documentTheme(c.Theme(), Preview)
										child.state.Font = portrait.Type.Small.Font(400)
										if r == 0 {
											child.state.Font.Weight = 600
										}
										child.state.LineHeight = portrait.Type.Small.LineHeight()
										child.state.Theme.Background = portrait.Clear
										child.state.FitContent = true
										child.state.SetTextAlign(tableTextAlign(b.Align[k]))
										child.state.OnInput = model.cellInput(child, pos, doc, c.Services())
										field := editor.View(c, &child.state).Width(tableContentWidth(model.widths[k]))
										model.bounds[pos] = field.Bounds()
										field.ObserveInput(func(ev ui.InputEvent) {
											if ev.Kind == ui.InputPointerDown && ev.Button == 0 {
												model.focus = child
												model.kind = tableRange
												model.pending = nil
												model.anchor, model.head, model.dragging, model.selected = pos, pos, true, false
											}
										})
										if model.hasTarget && model.target == pos {
											model.hasTarget = false
											model.focus = child
										}
										if model.focus == child {
											field.Focus()
											model.focus = nil
										}
										_, wasComposing := model.composing[child]
										composing := child.state.Composing()
										var owner any
										if composing || wasComposing {
											owner = &child.state
										}
										if composing {
											if model.composing == nil {
												model.composing = make(map[*Document]tableCell)
											}
											model.composing[child] = cell
										}
										if next := child.Source(); next != cell.raw && version == doc.Version() {
											replacement := escapeCell(next)
											if replacement != cell.raw {
												idx := doc.sourceIndex()
												a, z := idx.rawAtEditor(cell.start), idx.rawAtEditor(cell.end)
												if err := doc.apply(version, a, z, idx.raw[a:z], replacement, owner); err != nil {
													child.Reload(cell.raw)
												}
											}
										}
										if wasComposing && !composing {
											doc.state.Buffer.EndComposition(owner)
											delete(model.composing, child)
											if pending := model.pending; pending != nil && pending.child == child {
												model.pending = nil
												model.move(doc, pending.from, pending.step)
												c.Invalidate()
											}
										}
										if field.Focused() && (!model.selected) {
											sel := child.state.Selection()
											source := child.Source()
											a, z := cell.start+cellRawOffset(source, sel.Anchor), cell.start+cellRawOffset(source, sel.Head)
											if prior := doc.state.Selection(); prior.Anchor != a || prior.Head != z {
												doc.state.Select(a, z)
											}
										}
									})
								}
							})
						}
						model.hoverCol = -1
						boundary := gutter
						for k := 0; k+1 < len(model.widths); k++ {
							boundary += model.widths[k]
							model.resizeHandle(c, k, boundary, doc)
						}
						if at, ok := model.guide(); ok {
							ui.Box(c.Key("resize guide")).Absolute().Left(at-1).Top(0).Bottom(0).Width(2).
								Background(portrait.Colors.Accent).Shadow(0, 0, 0, 1, portrait.Colors.AccentSoft).
								IgnoreInput(true).AccessHidden(true)
						}
					})
				})
				fade := func(left bool) {
					mask, ok := portrait.ScrollEdgeMaskX(&model.scroll, left)
					if !ok {
						return
					}
					name := "right scroll fade"
					if left {
						name = "left scroll fade"
					}
					edge := ui.Box(c.Key(name)).Absolute().Top(0).Bottom(0).Width(portrait.Editor.TableEdgeFade)
					if left {
						edge.Left(0)
					} else {
						edge.Right(0)
					}
					edge.LinearGradient(mask).IgnoreInput(true).AccessHidden(true)
				}
				fade(true)
				fade(false)
			})
		})
	}})
}

// escapeCell writes text as one table cell: pipes escaped, line breaks as
// the HTML breaks Markdown tables read.
func escapeCell(s string) string {
	var out strings.Builder
	slashes := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		switch r {
		case '|':
			if slashes%2 == 0 {
				out.WriteByte('\\')
			}
		case '\r':
			if i < len(s) && s[i] == '\n' {
				i++
			}
			out.WriteString("<br>")
			slashes = 0
			continue
		case '\n':
			out.WriteString("<br>")
			slashes = 0
			continue
		}
		out.WriteString(s[i-size : i])
		if r == '\\' {
			slashes++
		} else {
			slashes = 0
		}
	}
	return out.String()
}

// cellMatches reports whether the child text and the raw cell differ only
// by table escaping, so a rebuild keeps its caret, selection and history.
func cellMatches(source, raw string) bool {
	return source == raw || escapeCell(source) == raw
}

// cellRawOffset maps an offset in the child text to the escaped cell source.
func cellRawOffset(source string, off int) int {
	return len(escapeCell(source[:max(0, min(off, len(source)))]))
}

type cellPosition struct{ row, col int }

func (m *readingTable) copyCells() string {
	top, bottom, left, right, ok := m.selectedBounds()
	if !ok {
		return ""
	}
	lines := make([]string, 0)
	for r := top; r <= bottom; r++ {
		var cells []string
		for k := left; k <= right; k++ {
			content := ""
			if r < len(m.cells) && k < len(m.cells[r]) && m.cells[r][k] != nil {
				content = m.cells[r][k].preview.display
			}
			cells = append(cells, content)
		}
		lines = append(lines, strings.Join(cells, "\t"))
	}
	return strings.Join(lines, "\n")
}

func (m *readingTable) cellInput(child *Document, pos cellPosition, doc *Document, services ui.Services) func(ui.InputEvent) bool {
	return func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputKeyDown && ev.Mods == (ui.Cmd|ui.Shift) && ev.Key == ui.KeyZ {
			doc.Redo()
			return true
		}
		switch ev.Kind {
		case ui.InputPointerMove, ui.InputPointerUp:
			if !m.dragging {
				return false
			}
			rect := m.bounds[pos]
			x, y := rect.X+ev.X, rect.Y+ev.Y
			for target, b := range m.bounds {
				if x >= b.X && x < b.X+b.W && y >= b.Y && y < b.Y+b.H {
					m.kind, m.head = tableRange, target
					m.selected = target != m.anchor
					break
				}
			}
			if ev.Kind == ui.InputPointerUp {
				m.dragging = false
			}
			return m.selected
		case ui.InputPointerDown:
			if ev.Button != 0 {
				if len(m.composing) != 0 || child.state.Composing() {
					return false
				}
				if m.chosen(pos) {
					m.focus = child
				} else {
					m.focusCell(pos)
				}
				m.menuCell = pos
				m.menuOpen = true
				return true
			}
			return false
		case ui.InputCommand:
			switch ev.Text {
			case "copy":
				if m.selected {
					services.WriteClipboard(m.copyCells())
					return true
				}
			case "undo":
				doc.Undo()
				return true
			case "redo":
				doc.Redo()
				return true
			}
		case ui.InputKeyDown:
			if step, ok := cellStepForKey(ev); ok {
				if child.state.Composing() {
					m.pending = &pendingCellMove{child: child, from: pos, step: step}
					return true
				}
				return m.navigate(doc, child, pos, step)
			}
			if ev.Mods == 0 && m.selected && ev.Key == ui.KeyBackspace {
				return m.clearCells(doc)
			}
			if ev.Mods == ui.Cmd {
				switch ev.Key {
				case ui.KeyC:
					if m.selected {
						services.WriteClipboard(m.copyCells())
						return true
					}
				case ui.KeyZ:
					doc.Undo()
					return true
				}
			}
		}
		return false
	}
}

func (m *readingTable) guide() (float32, bool) {
	col := m.dragCol
	if col < 0 {
		col = m.hoverCol
	}
	if col < 0 || col+1 >= len(m.widths) {
		return 0, false
	}
	at := portrait.Editor.TableGutter
	for k := 0; k <= col; k++ {
		at += m.widths[k]
	}
	return at, true
}

func (m *readingTable) resizeHandle(c *ui.Context, col int, boundary float32, doc *Document) {
	h := ui.Box(c.Key("resize " + fmt.Sprint(col))).Absolute().Left(boundary - 4).Top(portrait.Editor.TableGutter).Bottom(0).Width(8).
		Cursor(ui.CursorResizeColumn).KeepFocus().Label(fmt.Sprintf("Resize table column %d", col+1))
	if h.Hovered() && !m.dragging && m.dragCol < 0 {
		m.hoverCol = col
	}
	handle := ui.Local(h, "resize handle", func() ui.Handle { return ui.Handle{} })
	h.Bind(handle)
	h.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Button != 0 {
			return false
		}
		switch ev.Kind {
		case ui.InputPointerDown:
			m.dragCol = col
			m.dragX = handle.Bounds(c).X + ev.X
			m.leftWidth, m.rightWidth = m.widths[col], m.widths[col+1]
		case ui.InputPointerMove, ui.InputPointerUp:
			if m.dragCol != col {
				return false
			}
			dx := max(portrait.Editor.TableColumnResizeMin-m.leftWidth, min(m.rightWidth-portrait.Editor.TableColumnResizeMin, handle.Bounds(c).X+ev.X-m.dragX))
			if ev.Canceled {
				dx = 0
			}
			m.widths[col], m.widths[col+1] = m.leftWidth+dx, m.rightWidth-dx
			if ev.Kind == ui.InputPointerUp {
				m.dragCol = -1
				if dx != 0 {
					m.manual = true
					m.widthStateID = doc.state.Buffer.UndoStateID()
					doc.recordTableWidths(m.widthStateID, m.preferenceKey, m.widths)
					doc.saveTables()
				}
			}
		default:
			return false
		}
		return true
	})
}

func (m *readingTable) fit(width float32) {
	total := float32(0)
	for _, v := range m.minimum {
		total += v
	}
	extra := max(0, width-total) / float32(len(m.widths))
	for k, minW := range m.minimum {
		m.widths[k] = minW + extra
	}
}
