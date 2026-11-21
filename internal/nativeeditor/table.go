package nativeeditor

import (
	"fmt"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type readingTable struct {
	signature                    string
	seen                         uint64
	focus                        *Document
	cells                        [][]*Document
	widths                       []float32
	minimum                      []float32
	manual                       bool
	bounds                       map[cellPosition]ui.Rect
	anchor, head                 cellPosition
	dragging, selected           bool
	dragX, leftWidth, rightWidth float32
}
type tableCell struct {
	start, end int
	raw        string
}

func (p *previewLayout) standIn(snap editor.Snapshot, start, end int, visible string, block editor.Block) {
	line := snap.LineOf(start)
	first := snap.LineStart(line)
	last := end
	if end < snap.Len() && snap.Slice(end, end+1) == "\n" {
		last++
		visible += "\n"
	}
	p.replace(first, last, "")
	p.readable[first] = visible
	block.Line = line
	block.Interactive = true
	p.blocks = append(p.blocks, block)
}

func (p *previewLayout) table(snap editor.Snapshot, idx int) {
	doc := p.owner
	if doc.tables == nil {
		doc.tables = map[int]*readingTable{}
	}
	b := p.document.Block(idx)
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
	model := doc.tables[b.Start]
	if model == nil {
		for old, t := range doc.tables {
			if t.signature == sig && t.seen != p.version {
				model = t
				delete(doc.tables, old)
				doc.tables[b.Start] = model
				break
			}
		}
	}
	if model == nil || len(model.widths) != ncols {
		model = &readingTable{bounds: map[cellPosition]ui.Rect{}, widths: make([]float32, ncols), minimum: make([]float32, ncols)}
		for i := range model.widths {
			model.minimum[i] = 112
			for _, cells := range rows {
				if i < len(cells) {
					model.minimum[i] = max(model.minimum[i], min(340, 24+float32(utf8.RuneCountInString(cells[i].raw))*8))
				}
			}
		}
		model.signature = sig
		model.fit(624)
		doc.preferences.restoreTable(model)
		doc.tables[b.Start] = model
	}
	model.signature = sig
	model.seen = p.version
	for len(model.cells) < len(rows) {
		model.cells = append(model.cells, make([]*Document, ncols))
	}
	for r, cells := range rows {
		for k, cell := range cells {
			if k >= ncols {
				break
			}
			child := model.cells[r][k]
			if child == nil {
				child = New(cell.raw)
				model.cells[r][k] = child
			} else if !cellMatches(child.Source(), cell.raw) {
				child.Reload(cell.raw)
			}
		}
	}
	width := float32(0)
	for _, w := range model.widths {
		width += w
	}
	version := p.version
	p.standIn(snap, b.Start, snap.LineEnd(snap.LineOf(b.End)), strings.Join(visible, "\n"), editor.Block{Label: "Markdown table", Width: width, Height: float32(len(rows)) * 36, Content: func(c *ui.Context, e ui.Element, _ editor.Block) {
		if !model.manual && e.LayoutBounds().W > 0 {
			model.fit(e.LayoutBounds().W)
		}
		gridWidth := float32(0)
		for _, w := range model.widths {
			gridWidth += w
		}
		e.Children(func() {
			ui.ScrollHorizontal(c).FillWidth().Children(func() {
				ui.Column(c).Width(gridWidth).Shrink(0).Role(ui.RoleTable).Children(func() {
					for r, cells := range rows {
						ui.Row(c.Key(r)).Height(36).Role(ui.RoleRow).Children(func() {
							for k, cell := range cells {
								if k >= ncols {
									break
								}
								child := model.cells[r][k]
								box := ui.Column(c.Key(k)).Width(model.widths[k]).Shrink(0).Padding(2, 4).Justify(ui.Center)
								if r == 0 {
									box.InsetShadow(0, -0.5, 0, 0, portrait.Colors.LineStrong).Role(ui.RoleColumnHeader)
								} else {
									box.InsetShadow(0, -0.5, 0, 0, portrait.Colors.Line).Role(ui.RoleCell)
								}
								if model.chosen(cellPosition{r, k}) {
									box.Background(c.Theme().Selection)
								}
								box.Children(func() {
									child.state.ReadOnly = doc.state.ReadOnly
									child.prepareView()
									child.state.SetBlocks(nil)
									child.state.ReadingColumn = editor.ReadingColumn{}
									child.state.Label = fmt.Sprintf("Table row %d column %d", r+1, k+1)
									child.state.Theme = documentTheme(c.Theme(), Preview)
									child.state.Font = portrait.Type.Small.Font(400)
									if r == 0 {
										child.state.Font.Weight = 600
									}
									child.state.LineHeight = portrait.Type.Small.LineHeight()
									child.state.Theme.Background = unpainted
									pos := cellPosition{r, k}
									child.state.OnInput = model.cellInput(pos, doc, c.Services())
									field := editor.View(c, &child.state).Width(model.widths[k] - 10).Height(31)
									model.bounds[pos] = field.Bounds()
									field.ObserveInput(func(ev ui.InputEvent) {
										if ev.Kind == ui.InputPointerDown {
											model.focus = child
											model.anchor, model.head, model.dragging, model.selected = pos, pos, true, false
										}
									})
									if model.focus == child {
										field.Focus()
										model.focus = nil
									}
									if r == 0 && k+1 < len(model.widths) {
										model.resizeHandle(c, k, doc)
									}
									if next := child.Source(); next != cell.raw && version == doc.Version() {
										replacement := escapeCell(next)
										if replacement != cell.raw {
											idx := doc.sourceIndex()
											a, z := idx.rawAtEditor(cell.start), idx.rawAtEditor(cell.end)
											if err := doc.Apply(version, a, z, idx.raw[a:z], replacement); err != nil {
												child.Reload(cell.raw)
											}
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
				})
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

func (m *readingTable) chosen(pos cellPosition) bool {
	return m.selected && pos.row >= min(m.anchor.row, m.head.row) && pos.row <= max(m.anchor.row, m.head.row) && pos.col >= min(m.anchor.col, m.head.col) && pos.col <= max(m.anchor.col, m.head.col)
}

func (m *readingTable) copyCells() string {
	lines := make([]string, 0)
	for r := min(m.anchor.row, m.head.row); r <= max(m.anchor.row, m.head.row); r++ {
		var cells []string
		for k := min(m.anchor.col, m.head.col); k <= max(m.anchor.col, m.head.col); k++ {
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

func (m *readingTable) cellInput(pos cellPosition, doc *Document, services ui.Services) func(ui.InputEvent) bool {
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
					m.head = target
					m.selected = target != m.anchor
					break
				}
			}
			if ev.Kind == ui.InputPointerUp {
				m.dragging = false
			}
			return m.selected
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

func (m *readingTable) resizeHandle(c *ui.Context, col int, doc *Document) {
	h := ui.Box(c).Absolute().Right(0).Top(0).Width(7).Height(36).Cursor(ui.CursorResizeColumn).Label(fmt.Sprintf("Resize table column %d", col+1))
	left := h.Bounds().X
	h.HandleInput(func(ev ui.InputEvent) bool {

		switch ev.Kind {
		case ui.InputPointerDown:
			m.manual = true
			m.dragX = left + ev.X
			m.leftWidth, m.rightWidth = m.widths[col], m.widths[col+1]
		case ui.InputPointerUp:
			if ev.Canceled {
				m.widths[col], m.widths[col+1] = m.leftWidth, m.rightWidth
			} else {
				doc.saveTables()
			}
			return true
		case ui.InputPointerMove:
			if ev.Button != 0 {
				return false
			}
			dx := max(72-m.leftWidth, min(m.rightWidth-72, left+ev.X-m.dragX))
			if ev.Canceled {
				dx = 0
			}
			m.widths[col], m.widths[col+1] = m.leftWidth+dx, m.rightWidth-dx
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
