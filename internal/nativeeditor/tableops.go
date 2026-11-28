package nativeeditor

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type tableOpKind uint8

const (
	tableInsertRowAbove tableOpKind = iota
	tableInsertRowBelow
	tableInsertColumnLeft
	tableInsertColumnRight
	tableDeleteRow
	tableDeleteColumn
	tableAlignColumn
)

type tableOp struct {
	kind  tableOpKind
	row   int
	col   int
	align parser.Align
}

type tableLine struct {
	text  string
	eol   string
	cells [][2]int
}

type tableSource struct {
	lines []tableLine
}

func splitTableLinePrefix(text string) (prefix, body string) {
	i := 0
	for i < len(text) {
		j := i
		spaces := 0
		for j < len(text) && text[j] == ' ' && spaces < 3 {
			j++
			spaces++
		}
		if j < len(text) && text[j] == '>' {
			j++
			if j < len(text) && (text[j] == ' ' || text[j] == '\t') {
				j++
			}
			i = j
			continue
		}
		break
	}
	for i < len(text) && isTableSpace(text[i]) {
		i++
	}
	return text[:i], text[i:]
}

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
	_, delimBody := splitTableLinePrefix(lines[1].text)
	if _, ok := tableDelimiterAligns(delimBody); !ok {
		return tableSource{}, false
	}
	for i := range lines {
		lines[i].cells = tableRowCells(lines[i].text)
	}
	return tableSource{lines: lines}, true
}

func (t tableSource) String() string {
	var out strings.Builder
	for _, line := range t.lines {
		out.WriteString(line.text)
		out.WriteString(line.eol)
	}
	return out.String()
}

func (t tableSource) columns() int {
	if len(t.lines) < 2 {
		return 0
	}
	return len(t.lines[1].cells)
}

func (t tableSource) rowPrefix() string {
	if len(t.lines) > 1 {
		prefix, _ := splitTableLinePrefix(t.lines[1].text)
		return prefix
	}
	if len(t.lines) > 0 {
		prefix, _ := splitTableLinePrefix(t.lines[0].text)
		return prefix
	}
	return ""
}

func (t tableSource) with(op tableOp) (tableSource, cellPosition, bool) {
	if t.columns() == 0 {
		return t, cellPosition{}, false
	}
	out := tableSource{lines: slices.Clone(t.lines)}
	columns := out.columns()
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
		out.insertLine(line(row), out.rowPrefix()+tableEmptyRow(columns))
		return out, focus(row, 0), true
	case tableInsertRowBelow:
		row := clamp(op.row, 0, lines-2)
		at := line(row) + 1
		if row == 0 {
			at = 2
		}
		out.insertLine(at, out.rowPrefix()+tableEmptyRow(columns))
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

func (t *tableSource) insertLine(at int, text string) {
	at = clamp(at, 0, len(t.lines))
	eol := t.ending()
	if at == len(t.lines) && at > 0 && t.lines[at-1].eol == "" {
		t.lines[at-1].eol = eol
		eol = ""
	}
	t.lines = slices.Insert(t.lines, at, tableLine{text: text, eol: eol, cells: tableRowCells(text)})
}

func (t *tableSource) removeLine(at int) {
	if at < 0 || at >= len(t.lines) {
		return
	}
	if at == len(t.lines)-1 && at > 0 && t.lines[at].eol == "" {
		t.lines[at-1].eol = ""
	}
	t.lines = slices.Delete(t.lines, at, at+1)
}

func (t *tableSource) ending() string {
	for _, line := range t.lines {
		if line.eol != "" {
			return line.eol
		}
	}
	return "\n"
}

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

func tableRowCellTexts(body string) []string {
	cells := splitTableRow(body)
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = body[c[0]:c[1]]
	}
	return out
}

func ensureTableRowCells(body string, want []string) string {
	wantCols := len(want)
	if wantCols == 0 {
		return body
	}
	matches := func(cand string) bool {
		if wantCols == 1 && strings.IndexByte(cand, '|') < 0 {
			return false
		}
		return slices.Equal(tableRowCellTexts(cand), want)
	}
	if matches(body) {
		return body
	}
	if wantCols == 1 && strings.IndexByte(body, '|') < 0 {
		trimmed := strings.Trim(body, " \t")
		if trimmed == "" {
			return tableEmptyRow(1)
		}
		return "| " + trimmed + " |"
	}
	for _, cand := range []string{
		body + " |",
		"| " + body,
		"| " + body + " |",
		"|" + body + "|",
	} {
		if matches(cand) {
			return cand
		}
	}
	var out strings.Builder
	out.WriteByte('|')
	for _, cell := range want {
		if cell == "" {
			out.WriteString("  |")
		} else {
			out.WriteByte(' ')
			out.WriteString(cell)
			out.WriteString(" |")
		}
	}
	return out.String()
}

func (t *tableSource) removeColumn(col int) {
	columns := max(1, t.columns()-1)
	for i := range t.lines {
		prefix, body := splitTableLinePrefix(t.lines[i].text)
		orig := tableRowCellTexts(body)
		next, ok := removeTableCellBody(body, col, columns)
		if ok {
			var want []string
			if col >= 0 && col < len(orig) {
				want = append(append([]string(nil), orig[:col]...), orig[col+1:]...)
			}
			if len(want) == 0 && (i == 0 || i == 1 || columns == 1) {
				want = make([]string, columns)
			}
			if len(want) > 0 {
				next = ensureTableRowCells(next, want)
			} else if len(splitTableRow(next)) == 0 {
				next = tableEmptyRow(columns)
			}
			t.lines[i].text = prefix + next
		}
		t.lines[i].cells = tableRowCells(t.lines[i].text)
	}
}

func tableEmptyRow(columns int) string {
	if columns <= 1 {
		return "|  |"
	}
	return "|" + strings.Repeat("  |", columns)
}

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

func insertTableCell(text string, k int, content string) string {
	prefix, body := splitTableLinePrefix(text)
	return prefix + insertTableCellBody(body, k, content)
}

func insertTableCellBody(body string, k int, content string) string {
	row := splitTableRowParts(body)
	n := len(row.cells)
	if k > n {
		missing := k - n
		at, closed := len(body), false
		if n > 0 {
			closed = row.post[n-1] >= 0
			if closed {
				at = row.post[n-1]
			} else {
				at = row.cells[n-1][1]
			}
		}
		var pad strings.Builder
		for range missing {
			if closed {
				pad.WriteString("|  ")
			} else {
				pad.WriteString(" | ")
			}
		}
		switch {
		case content != "":
			if closed {
				pad.WriteString("| ")
				pad.WriteString(content)
				pad.WriteString(" ")
			} else {
				pad.WriteString(" | ")
				pad.WriteString(content)
			}
		case closed:
			pad.WriteString("| ")
		default:
			pad.WriteString(" | |")
		}
		return body[:at] + pad.String() + body[at:]
	}
	if k < n {
		at := row.cells[k][0]
		if content == "" {
			if k == 0 && row.pre[0] < 0 {
				return "|  | " + body
			}
			return body[:at] + "| " + body[at:]
		}
		return body[:at] + content + " | " + body[at:]
	}
	return appendTableCellBody(body, content)
}

func appendTableCellBody(body, content string) string {
	row := splitTableRowParts(body)
	at, closed := len(body), false
	if n := len(row.cells); n > 0 {
		at, closed = row.cells[n-1][1], row.post[n-1] >= 0
	}
	switch {
	case content != "":
		return body[:at] + " | " + content + body[at:]
	case closed:
		return body[:at] + " |" + body[at:]
	default:
		return body[:at] + " | |" + body[at:]
	}
}

func replaceTableCell(text string, k int, content string) string {
	cells := tableRowCells(text)
	if k < 0 || k >= len(cells) {
		return text
	}
	return text[:cells[k][0]] + content + text[cells[k][1]:]
}

func removeTableCellBody(body string, k int, remainingCols int) (string, bool) {
	row := splitTableRowParts(body)
	if k < 0 || k >= len(row.cells) {
		return body, false
	}
	switch {
	case row.pre[k] >= 0 && row.post[k] >= 0:
		return body[:row.pre[k]] + body[row.post[k]:], true
	case row.post[k] >= 0:
		rest := strings.TrimLeft(body[row.post[k]+1:], " \t")
		return body[:row.cells[k][0]] + rest, true
	case row.pre[k] >= 0:
		return strings.TrimRight(body[:row.pre[k]], " \t"), true
	default:
		return tableEmptyRow(remainingCols), true
	}
}

func tableRowCells(text string) [][2]int {
	prefix, body := splitTableLinePrefix(text)
	lead := len(prefix)
	cells := splitTableRow(body)
	for i := range cells {
		cells[i][0] += lead
		cells[i][1] += lead
	}
	return cells
}

func splitTableRow(line string) [][2]int {
	return splitTableRowParts(line).cells
}

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

func (m *readingTable) editTable(doc *Document, op tableOp) bool {
	if doc.state.ReadOnly || doc.Version() != m.version {
		return false
	}
	if len(m.composing) != 0 || m.rawStart < 0 || m.rawEnd < m.rawStart {
		return false
	}
	idx := doc.sourceIndex()
	start, end := idx.rawAtEditor(m.rawStart), idx.rawAtEditor(m.rawEnd)
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
	for prefix > 0 && ((prefix < len(old) && !utf8.RuneStart(old[prefix])) || (prefix < len(next) && !utf8.RuneStart(next[prefix]))) {
		prefix--
	}
	suffix := tableCommonSuffix(old[prefix:], next[prefix:])
	for suffix > 0 && (!utf8.RuneStart(old[len(old)-suffix]) || !utf8.RuneStart(next[len(next)-suffix])) {
		suffix--
	}
	a, z := start+prefix, end-suffix
	if idx.insideCRLF(a) {
		a--
	}
	if idx.insideCRLF(z) {
		z++
	}
	beforeStateID := doc.state.Buffer.UndoStateID()
	if m.manual && m.preferenceKey != "" {
		doc.recordTableWidths(beforeStateID, m.preferenceKey, m.widths)
	}
	if err := doc.apply(doc.Version(), a, z, idx.raw[a:z], next[a-start:len(next)-(end-z)], nil); err != nil {
		return false
	}
	m.pendingEdit = &tableEdit{op: op, focus: focus}
	return true
}

type tableEdit struct {
	op    tableOp
	focus cellPosition
}

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
