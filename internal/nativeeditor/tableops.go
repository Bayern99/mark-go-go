package nativeeditor

import (
	"slices"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// tableOpKind names one GFM structural edit of a table's own bytes.
type tableOpKind uint8

const (
	// tableInsertRowAbove adds a data row above the row it names. The
	// header has no row above it: a table keeps one header row.
	tableInsertRowAbove tableOpKind = iota
	tableInsertRowBelow
	tableInsertColumnLeft
	tableInsertColumnRight
	// tableDeleteRow removes a data row; the header row stays.
	tableDeleteRow
	// tableDeleteColumn removes a column, the last one excepted.
	tableDeleteColumn
	tableAlignColumn
)

// tableOp is one structural edit: the row it works from, the column it
// takes, and the alignment an alignment edit writes.
type tableOp struct {
	kind  tableOpKind
	row   int
	col   int
	align parser.Align
}

// tableLine is a line of a table's own bytes: its text without the line
// ending, the ending it keeps, and its cells as [start, end) byte ranges of
// the text.
type tableLine struct {
	text  string
	eol   string
	cells [][2]int
}

// tableSource is a table's bytes as its header line, its delimiter row and
// its data rows, none of them rewritten but the ones an edit takes.
type tableSource struct {
	lines []tableLine
}

// readTableSource reads a table's bytes, or reports false when they are no
// GFM table: a header line and a delimiter row of dashes.
func readTableSource(raw string) (tableSource, bool) {
	var lines []tableLine
	for at := 0; at < len(raw); {
		end := at
		for end < len(raw) && raw[end] != '\n' && raw[end] != '\r' {
			end++
		}
		eol := ""
		switch {
		case end == len(raw):
		case raw[end] == '\r' && end+1 < len(raw) && raw[end+1] == '\n':
			eol = "\r\n"
		case raw[end] == '\r':
			eol = "\r"
		default:
			eol = "\n"
		}
		lines = append(lines, tableLine{text: raw[at:end], eol: eol})
		at = end + len(eol)
	}
	if len(lines) < 2 {
		return tableSource{}, false
	}
	if _, ok := tableDelimiterAligns(lines[1].text); !ok {
		return tableSource{}, false
	}
	for i := range lines {
		lines[i].cells = tableRowCells(lines[i].text)
	}
	return tableSource{lines: lines}, true
}

// String returns the table as its bytes.
func (t tableSource) String() string {
	var out strings.Builder
	for _, line := range t.lines {
		out.WriteString(line.text)
		out.WriteString(line.eol)
	}
	return out.String()
}

// columns is the grid's width, as the delimiter row counts it.
func (t tableSource) columns() int {
	if len(t.lines) < 2 {
		return 0
	}
	return len(t.lines[1].cells)
}

// with returns the table one structural edit makes of it, and the cell to
// focus afterwards, or reports false when the table cannot take the edit.
func (t tableSource) with(op tableOp) (tableSource, cellPosition, bool) {
	if t.columns() == 0 {
		return t, cellPosition{}, false
	}
	out := tableSource{lines: slices.Clone(t.lines)}
	columns := out.columns()
	// A row is a row of the grid: 0 is the header, 1 the first data row.
	// Its source line is its own, but for the delimiter row after the
	// header, which no grid row names.
	lines := len(out.lines)
	line := func(row int) int {
		if row == 0 {
			return 0
		}
		return row + 1
	}
	focus := func(row, col int) cellPosition {
		return cellPosition{
			row: clamp(row, 0, max(0, len(out.lines)-2)),
			col: clamp(col, 0, max(0, out.columns()-1)),
		}
	}
	switch op.kind {
	case tableInsertRowAbove:
		row := clamp(op.row, 0, lines-2)
		if row == 0 {
			return t, cellPosition{}, false
		}
		out.insertLine(line(row), tableEmptyRow(columns))
		return out, focus(row, 0), true
	case tableInsertRowBelow:
		row := clamp(op.row, 0, lines-2)
		at := line(row) + 1
		if row == 0 {
			// Below the header is the first data row, after the delimiter.
			at = 2
		}
		out.insertLine(at, tableEmptyRow(columns))
		return out, focus(row+1, 0), true
	case tableDeleteRow:
		row := clamp(op.row, 0, lines-2)
		if row == 0 {
			return t, cellPosition{}, false
		}
		out.removeLine(line(row))
		return out, focus(row, op.col), true
	case tableInsertColumnLeft, tableInsertColumnRight:
		col := clamp(op.col, 0, columns-1)
		if op.kind == tableInsertColumnRight {
			col++
		}
		out.insertColumn(col)
		return out, focus(op.row, col), true
	case tableDeleteColumn:
		if columns == 1 {
			return t, cellPosition{}, false
		}
		col := clamp(op.col, 0, columns-1)
		out.removeColumn(col)
		return out, focus(op.row, min(col, columns-2)), true
	case tableAlignColumn:
		col := clamp(op.col, 0, columns-1)
		out.lines[1].text = replaceTableCell(out.lines[1].text, col, tableAlignMark(op.align))
		out.lines[1].cells = tableRowCells(out.lines[1].text)
		return out, focus(op.row, col), true
	}
	return t, cellPosition{}, false
}

// insertLine puts a line at at, keeping the table's own line endings: a line
// added at the end takes the ending the table writes, so that a file ending
// in a newline still does; one that ends without does too.
func (t *tableSource) insertLine(at int, text string) {
	at = clamp(at, 0, len(t.lines))
	eol := t.ending()
	if at == len(t.lines) && at > 0 && t.lines[at-1].eol == "" {
		// The file ends here: the row above takes the ending, and the new
		// row keeps the file ending where it did.
		t.lines[at-1].eol = eol
		eol = ""
	}
	t.lines = slices.Insert(t.lines, at, tableLine{text: text, eol: eol, cells: tableRowCells(text)})
}

// removeLine cuts a line, its ending with it: the line before the cut takes
// the ending of the line that now ends the table.
func (t *tableSource) removeLine(at int) {
	if at < 0 || at >= len(t.lines) {
		return
	}
	if at == len(t.lines)-1 && at > 0 {
		t.lines[at-1].eol = t.lines[at].eol
	}
	t.lines = slices.Delete(t.lines, at, at+1)
}

// ending returns the line ending the table writes: the one of its first line
// that has one, or a plain newline.
func (t *tableSource) ending() string {
	for _, line := range t.lines {
		if line.eol != "" {
			return line.eol
		}
	}
	return "\n"
}

// insertColumn adds an empty cell to every row at column at, and a cell of
// dashes to the delimiter row.
func (t *tableSource) insertColumn(at int) {
	for i := range t.lines {
		content := ""
		if i == 1 {
			content = tableAlignMark(parser.AlignNone)
		}
		t.lines[i].text = insertTableCell(t.lines[i].text, at, content)
		t.lines[i].cells = tableRowCells(t.lines[i].text)
	}
}

// removeColumn cuts column col from every row that has it.
func (t *tableSource) removeColumn(col int) {
	columns := max(1, t.columns()-1)
	for i := range t.lines {
		if text, ok := removeTableCell(t.lines[i].text, col); ok {
			t.lines[i].text = text
			if len(tableRowCells(text)) == 0 {
				t.lines[i].text = tableEmptyRow(columns)
			}
		}
		t.lines[i].cells = tableRowCells(t.lines[i].text)
	}
}

// tableEmptyRow returns a row of as many empty cells.
func tableEmptyRow(columns int) string {
	if columns <= 1 {
		return "||"
	}
	return "|" + strings.Repeat("  |", columns)
}

// tableAlignMark returns the delimiter cell of an alignment.
func tableAlignMark(align parser.Align) string {
	switch align {
	case parser.AlignLeft:
		return ":---"
	case parser.AlignCenter:
		return ":---:"
	case parser.AlignRight:
		return "---:"
	default:
		return "---"
	}
}

// insertTableCell returns a row with a new cell of content at column k,
// adding the empty cells a shorter row lacks before it.
func insertTableCell(text string, k int, content string) string {
	cells := tableRowCells(text)
	if k > len(cells) {
		// A row shorter than the column takes the cells it lacks, rewritten
		// as the cells it has.
		parts := make([]string, 0, k+1)
		for _, cell := range cells {
			parts = append(parts, text[cell[0]:cell[1]])
		}
		for len(parts) < k {
			parts = append(parts, "")
		}
		return "| " + strings.Join(append(parts, content), " | ") + " |"
	}
	if k < len(cells) {
		at := cells[k][0]
		if content == "" {
			return text[:at] + "| " + text[at:]
		}
		return text[:at] + content + " | " + text[at:]
	}
	return appendTableCell(text, content)
}

// appendTableCell returns a row with a cell of content after its last cell:
// a row that closes with a pipe takes the new cell inside it, one that does
// not takes a pipe of its own.
func appendTableCell(text, content string) string {
	row := splitTableRowParts(text)
	at, closed := len(text), false
	if n := len(row.cells); n > 0 {
		at, closed = row.cells[n-1][1], row.post[n-1] >= 0
	}
	switch {
	case content != "":
		return text[:at] + " | " + content + text[at:]
	case closed:
		return text[:at] + " |" + text[at:]
	default:
		return text[:at] + " | |" + text[at:]
	}
}

// replaceTableCell returns a row with column k's cell text replaced.
func replaceTableCell(text string, k int, content string) string {
	cells := tableRowCells(text)
	if k < 0 || k >= len(cells) {
		return text
	}
	return text[:cells[k][0]] + content + text[cells[k][1]:]
}

// removeTableCell returns a row without column k's cell, or reports false
// when the row's cells do not split where its pipes are.
func removeTableCell(text string, k int) (string, bool) {
	row := splitTableRowParts(text)
	if k < 0 || k >= len(row.cells) {
		return text, false
	}
	switch {
	case row.pre[k] >= 0 && row.post[k] >= 0:
		// The pipe before the cell goes with it, the one after it stays.
		return text[:row.pre[k]] + text[row.post[k]:], true
	case row.post[k] >= 0:
		// The first cell of a row that opens without a pipe.
		return text[:row.cells[k][0]] + text[row.post[k]+1:], true
	case row.pre[k] >= 0:
		// The last cell of a row that closes without a pipe.
		return strings.TrimRight(text[:row.pre[k]], " \t"), true
	default:
		return "", false
	}
}

// tableRowCells returns a row's cells as [start, end) byte ranges of its
// text, trimmed: an optional pipe, then cells between pipes. The parser reads
// a row without its indentation, so its cells start after it.
func tableRowCells(text string) [][2]int {
	lead := len(text) - len(strings.TrimLeft(text, " \t"))
	cells := splitTableRow(text[lead:])
	for i := range cells {
		cells[i][0] += lead
		cells[i][1] += lead
	}
	return cells
}

// splitTableRow splits a row into its cells' byte ranges, trimmed: an
// optional pipe, then cells between pipes, whose \| do not split. It reads a
// row exactly as the Markdown parser does, inline code spans included: a |
// inside backticks still divides cells.
func splitTableRow(line string) [][2]int {
	return splitTableRowParts(line).cells
}

// tableRowParts are a row's cells and the pipes around them, as the parser
// finds them.
type tableRowParts struct {
	cells     [][2]int
	pre, post []int
}

func splitTableRowParts(line string) tableRowParts {
	var row tableRowParts
	at, prev := 0, -1
	if at < len(line) && line[at] == '|' {
		prev = at
		at++
		for at < len(line) && isTableSpace(line[at]) {
			at++
		}
	}
	for at < len(line) {
		end := at
		for end < len(line) && line[end] != '|' {
			if line[end] == '\\' && end+1 < len(line) && isTableEscapable(line[end+1]) {
				end++
			}
			end++
		}
		pipe := end < len(line)
		start, stop := at, end
		for start < stop && isTableSpace(line[start]) {
			start++
		}
		for stop > start && isTableSpace(line[stop-1]) {
			stop--
		}
		if end > at || pipe {
			after := -1
			if pipe {
				after = end
			}
			row.cells = append(row.cells, [2]int{start, stop})
			row.pre = append(row.pre, prev)
			row.post = append(row.post, after)
			prev = after
		}
		if !pipe {
			break
		}
		at = end + 1
		for at < len(line) && isTableSpace(line[at]) {
			at++
		}
	}
	return row
}

// tableDelimiterAligns reads a delimiter row the way the parser does: cells
// of dashes, a colon on either end for alignment.
func tableDelimiterAligns(line string) ([]parser.Align, bool) {
	line = strings.TrimRight(line, " \t")
	if line == "" {
		return nil, false
	}
	pipes := strings.Count(line, "|")
	s := line
	if s[0] == '|' {
		s = s[1:]
	}
	if s != "" && s[len(s)-1] == '|' && pipes > 0 && (len(line) > 1 || line[0] != '|') {
		if len(s) > 0 {
			s = s[:len(s)-1]
		}
	}
	var aligns []parser.Align
	for part := range strings.SplitSeq(s, "|") {
		part = strings.Trim(part, " \t")
		left := strings.HasPrefix(part, ":")
		right := strings.HasSuffix(part, ":") && len(part) > 1
		if left {
			part = part[1:]
		}
		if right {
			part = part[:len(part)-1]
		}
		if part == "" || strings.Trim(part, "-") != "" {
			return nil, false
		}
		align := parser.AlignNone
		switch {
		case left && right:
			align = parser.AlignCenter
		case left:
			align = parser.AlignLeft
		case right:
			align = parser.AlignRight
		}
		aligns = append(aligns, align)
	}
	if len(aligns) == 0 {
		return nil, false
	}
	return aligns, true
}

func isTableSpace(c byte) bool { return c == ' ' || c == '\t' }
func isTableEscapable(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' || c >= '[' && c <= '`' || c >= '{' && c <= '~'
}

// editTable applies one structural edit to the table's bytes through the
// document's buffer: one edit of one range, the bytes it does not touch kept
// as they were, and one undo to take it back.
func (m *readingTable) editTable(doc *Document, op tableOp) bool {
	if doc.state.ReadOnly || doc.Version() != m.version {
		return false
	}
	if len(m.composing) != 0 || m.rawStart < 0 || m.rawEnd < m.rawStart {
		return false
	}
	idx := doc.sourceIndex()
	start, end := idx.rawAtEditor(m.rawStart), idx.rawAtEditor(m.rawEnd)
	// The edit owns the last row's line ending: a row added at the end of
	// the table takes the place the following block had.
	switch {
	case end+1 < len(idx.raw) && idx.raw[end] == '\r' && idx.raw[end+1] == '\n':
		end += 2
	case end < len(idx.raw) && (idx.raw[end] == '\n' || idx.raw[end] == '\r'):
		end++
	}
	old := idx.raw[start:end]
	table, isTable := readTableSource(old)
	if !isTable {
		return false
	}
	source, focus, ok := table.with(op)
	if !ok {
		return false
	}
	next := source.String()
	if next == old {
		return false
	}
	prefix := tableCommonPrefix(old, next)
	suffix := tableCommonSuffix(old[prefix:], next[prefix:])
	a, z := start+prefix, end-suffix
	// One edit never splits a CRLF line ending.
	if idx.insideCRLF(a) {
		a--
	}
	if idx.insideCRLF(z) {
		z++
	}
	if err := doc.apply(doc.Version(), a, z, idx.raw[a:z], next[a-start:len(next)-(end-z)], nil); err != nil {
		return false
	}
	m.pendingEdit = &tableEdit{op: op, focus: focus}
	return true
}

// tableEdit is a structural edit a table settles into its state on the frame
// after it: the width it adds or takes, and the cell the caret goes to.
type tableEdit struct {
	op    tableOp
	focus cellPosition
}

// settle carries the table's state over a structural edit: the column widths
// it keeps, the records it saved, and the cell the caret goes to. The frame
// after the edit rebuilds the cells alone.
func (m *readingTable) settle(doc *Document) {
	edit := m.pendingEdit
	if edit == nil {
		return
	}
	m.pendingEdit = nil
	op, focus := edit.op, edit.focus
	switch op.kind {
	case tableInsertColumnLeft, tableInsertColumnRight:
		at := clamp(focus.col, 0, len(m.widths))
		m.widths = slices.Insert(m.widths, at, portrait.Editor.TableColumnMin)
		m.minimum = slices.Insert(m.minimum, at, portrait.Editor.TableColumnMin)
	case tableDeleteColumn:
		if op.col >= 0 && op.col < len(m.widths) {
			m.widths = slices.Delete(m.widths, op.col, op.col+1)
			m.minimum = slices.Delete(m.minimum, op.col, op.col+1)
		}
	}
	m.bounds = map[cellPosition]ui.Rect{}
	m.hoverCol, m.dragCol = -1, -1
	m.selected, m.dragging = false, false
	m.anchor, m.head = focus, focus
	m.focus, m.pending = nil, nil
	m.menuOpen, m.menuWas = false, false
	m.target, m.hasTarget = focus, true
	// The record's key names the columns the table had: the layout after the
	// edit writes it anew.
	m.resave = m.manual
}

func tableCommonPrefix(a, b string) int {
	at := 0
	for at < len(a) && at < len(b) && a[at] == b[at] {
		at++
	}
	return at
}

func tableCommonSuffix(a, b string) int {
	at := 0
	for at < len(a) && at < len(b) && a[len(a)-1-at] == b[len(b)-1-at] {
		at++
	}
	return at
}
