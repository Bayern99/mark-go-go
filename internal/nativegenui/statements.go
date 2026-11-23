package nativegenui

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/genuikit"
	"github.com/Bayern99/mark-go-go/internal/openuilang"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

func openStatements(data string) (*Surface, error) {
	view, err := genui.ReadStatementView([]byte(data))
	if err != nil {
		return nil, err
	}
	library, err := genuikit.Library()
	if err != nil {
		return nil, err
	}
	text := ""
	for _, batch := range view.Statements {
		text = openuilang.Overlay(text, strings.Join(batch, "\n"))
	}
	program := library.Parse(text)
	return newStatementSurface(program, text, view.Sources, view.Request, view.Statements[0], true)
}

func newStatementSurface(program *openuilang.Program, text string, sources map[string]genui.StatementSource, request string, created []string, requireRoot bool) (*Surface, error) {
	s := &Surface{Nodes: map[string]Node{}, Sources: map[string]Reference{}, Request: request, Session: openuilang.NewSession(), created: created}
	for id, source := range sources {
		s.Sources[id] = Reference{Heading: source.Heading, Occurrence: source.Occurrence, Hash: source.Hash}
	}
	s.Session.Load(program)
	s.program = program
	orphans := map[string]bool{}
	for _, name := range program.Orphans {
		orphans[name] = true
	}
	for _, use := range openuilang.FirstArgumentStrings(text, "Source") {
		if use.Literal && !orphans[use.Statement] {
			s.sourceUses = append(s.sourceUses, use)
		}
	}
	s.refresh()
	if requireRoot && s.tree == nil {
		return nil, errors.New("The view has no root component.")
	}
	return s, nil
}

func (s *Surface) resume(session *openuilang.Session) {
	session.Load(s.program)
	s.Session = session
	s.refresh()
}

func (s *Surface) refresh() {
	program := s.program
	tree, problems := s.Session.Render()
	if tree == nil && s.tree != nil {
		return
	}
	s.tree, s.notices = tree, nil
	s.Nodes, s.owners = map[string]Node{}, map[*openuilang.Element]string{}
	s.own(tree, "")
	seen := map[string]bool{}
	for _, problem := range append(program.Errors, problems...) {
		if problem.Degraded {
			continue
		}
		notice := problem.Message
		if problem.Code == "unknown-component" {
			notice = fmt.Sprintf("“%s” is not a component this app can show.", problem.Component)
		}
		if !seen[notice] {
			seen[notice] = true
			s.notices = append(s.notices, notice)
		}
	}
	for _, name := range program.Unresolved {
		s.notices = append(s.notices, fmt.Sprintf("“%s” is used but never defined.", name))
	}
}

func (s *Surface) own(value any, owner string) {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			s.own(item, owner)
		}
	case *openuilang.Object:
		for _, key := range v.Keys {
			s.own(v.Values[key], owner)
		}
	case *openuilang.Element:
		if v == nil || len(s.owners) > 1<<16 {
			return
		}
		if v.StatementID != "" {
			owner = v.StatementID
		}
		if _, seen := s.owners[v]; seen {
			return
		}
		s.owners[v] = owner
		if v.Type == "Source" {
			n := Node{ID: "source" + strconv.Itoa(len(s.Nodes)), ComponentID: owner}
			for _, id := range flatten(v.Props.Values["sources"], nil) {
				if text, ok := id.(string); ok {
					n.Sources = append(n.Sources, text)
				}
			}
			s.Nodes[n.ID] = n
		}
		s.own(v.Props, owner)
	}
}

const (
	maxStatementDepth   = 64
	spaceStep           = 4
	sectionGap          = 24
	groupGap            = 12
	innerGap            = 8
	tightGap            = 4
	readingWidth        = 760
	subtitleSize        = 13
	largestCompactTitle = "xl"
	rowShare            = 220
)

type place struct {
	compact, nested, row, card bool
}

type treatment struct {
	subtitle, fill bool
}

func (r *Reading) statements(c *ui.Context) {
	ui.Row(c).Justify(ui.Center).MinWidth(0).Children(func() {
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).MaxWidth(readingWidth).Children(func() {
			r.drawingProblems = map[string]string{}
			r.piece(c, r.surface.tree, "root", 0, place{}, treatment{})
			for i, notice := range r.surface.notices {
				ui.Text(c.Key("notice"+strconv.Itoa(i)), notice).Role(ui.RoleAlert).Margin(groupGap, 0, 0, 0)
			}
			paths := make([]string, 0, len(r.drawingProblems))
			for path := range r.drawingProblems {
				paths = append(paths, path)
			}
			slices.Sort(paths)
			for i, path := range paths {
				ui.Text(c.Key("drawing-notice"+strconv.Itoa(i)), r.drawingProblems[path]).Role(ui.RoleAlert).Margin(groupGap, 0, 0, 0)
			}
		})
	})
}

func flatten(value any, into []any) []any {
	switch v := value.(type) {
	case nil:
	case []any:
		for _, item := range v {
			into = flatten(item, into)
		}
	default:
		into = append(into, v)
	}
	return into
}

func kind(value any) string {
	if e, ok := value.(*openuilang.Element); ok && e != nil {
		return e.Type
	}
	return ""
}

func titleSize(value any) string {
	e, _ := value.(*openuilang.Element)
	if e == nil || e.Type != "Title" {
		return ""
	}
	if size := (props{e.Props}).text("size"); size != "" {
		return size
	}
	return "md"
}

func opens(value any) bool {
	if size := titleSize(value); size == "sm" || size == "md" {
		return true
	}
	if kind(value) != "Row" {
		return false
	}
	for _, child := range flatten((props{value.(*openuilang.Element).Props}).value("children"), nil) {
		if size := titleSize(child); size == "sm" || size == "md" {
			return true
		}
	}
	return false
}

func isPlainText(value any) bool {
	if kind(value) != "Text" {
		return false
	}
	_, plain := (props{value.(*openuilang.Element).Props}).value("text").(string)
	return plain
}

type spannedWidth struct {
	frame         ui.Handle
	window, width float32
}

func (spanned *spannedWidth) lastDrawn(c *ui.Context) float32 {
	window, _ := c.Size()
	width := spanned.frame.Bounds(c).W
	if window != spanned.window || width != spanned.width {
		spanned.window, spanned.width = window, width
		c.Invalidate()
	}
	return width
}

func (spanned *spannedWidth) fitsOnOneLine(c *ui.Context, text textFit) bool {
	width := spanned.lastDrawn(c)
	return text.lines == 1 && (width == 0 || text.natural <= width+1)
}

func wide(children any) bool {
	for _, child := range flatten(children, nil) {
		switch kind(child) {
		case "Chart", "Table", "Grid", "Card", "Carousel", "Image", "Svg":
			return true
		}
	}
	return false
}

func holds(children any, parent string, test func(any) bool) bool {
	for _, child := range flatten(children, nil) {
		if kind(child) == parent && test((props{child.(*openuilang.Element).Props}).value("children")) {
			return true
		}
	}
	return false
}

func rigid(children any) bool {
	for _, child := range flatten(children, nil) {
		switch kind(child) {
		case "Badge", "Link", "Label", "Button", "IconButton":
		default:
			return false
		}
	}
	return true
}

func grouping(value any) bool {
	switch kind(value) {
	case "Card", "Source", "Col", "Box":
		return true
	}
	return false
}

func isTitleBlock(value any) bool {
	if kind(value) == "Title" {
		return true
	}
	if kind(value) != "Col" {
		return false
	}
	children := flatten((props{value.(*openuilang.Element).Props}).value("children"), nil)
	for _, child := range children {
		if kind(child) == "Title" {
			return !wide(children)
		}
	}
	return false
}

func openingLength(items []any) int {
	for i, item := range items {
		if isTitleBlock(item) {
			if i == 0 {
				return 0
			}
			return i + 1
		}
		switch kind(item) {
		case "Icon", "Badge", "Label", "IconButton":
		default:
			return 0
		}
	}
	return 0
}

func badgeRow(value any) bool {
	if kind(value) != "Row" {
		return false
	}
	for _, child := range flatten((props{value.(*openuilang.Element).Props}).value("children"), nil) {
		if kind(child) == "Badge" {
			return true
		}
	}
	return false
}

func flowGap(before, after any, tier float32) float32 {
	b, a := kind(before), kind(after)
	switch {
	case b == "Title" && (a == "Text" || a == "Caption" || badgeRow(after)):
		return innerGap
	case b == "Caption" && a == "Title":
		return tightGap
	case b == "Title", b == "Text" && a == "Text":
		return groupGap
	}
	return tier
}

func (r *Reading) stack(c *ui.Context, value any, path string, depth int, at place, gap float32, flows bool) {
	items := flatten(value, nil)
	r.span(c, items, 0, len(items), path, depth, at, gap, flows, treatment{})
}

func (r *Reading) span(c *ui.Context, items []any, from, to int, path string, depth int, at place, gap float32, flows bool, as treatment) {
	for i := from; i < to; i++ {
		item, at, as := items[i], at, as
		at.card = at.card && kind(item) != "Card"
		if i > 0 {
			space := gap
			var spanned *spannedWidth
			if flows {
				space = flowGap(items[i-1], item, gap)
				if at.card && i == 1 && kind(items[0]) == "Title" && opens(items[0]) {
					switch {
					case kind(item) == "Caption":
						as.subtitle = true
					case isPlainText(item):
						p := props{item.(*openuilang.Element).Props}
						spanned = &r.state(path + "/" + strconv.Itoa(i)).spanned
						as.subtitle = spanned.fitsOnOneLine(c, r.fitted(p.text("text"), at.textStyle(c, p, true)))
					}
					if as.subtitle {
						space = 2
					}
				}
			}
			if space > 0 {
				spacer := ui.Column(c.Key(path + "#" + strconv.Itoa(i))).Height(space).Shrink(0).AccessHidden(true)
				if spanned != nil {
					spacer.AlignSelf(ui.Stretch).Bind(&spanned.frame)
				}
			}
		}
		r.piece(c, item, path+"/"+strconv.Itoa(i), depth, at, as)
	}
}

func (r *Reading) rowChildren(c *ui.Context, row ui.Element, p props, gap float32, path string, depth int, at place) {
	items := flatten(p.value("children"), nil)
	if slices.ContainsFunc(items, isTitleBlock) {
		row.Wrap()
	}
	row.Children(func() {
		leading := openingLength(items)
		if leading > 0 {
			head := ui.Row(c.Key(path + "#opening")).Gap(gap).MaxWidthPercent(100)
			if a, ok := crossAlign[p.text("align")]; ok {
				head.AlignItems(a)
			}
			head.Children(func() { r.span(c, items, 0, leading, path, depth, at, 0, false, treatment{}) })
		}
		r.span(c, items, leading, len(items), path, depth, at, 0, false, treatment{})
	})
}

func (r *Reading) piece(c *ui.Context, value any, path string, depth int, at place, as treatment) {
	if r.remaining <= 0 || depth > maxStatementDepth {
		if r.remaining == 0 {
			ui.Text(c.Key(path), "This layout repeats too many components to display.").Role(ui.RoleAlert)
			r.remaining--
		}
		return
	}
	r.remaining--
	switch v := value.(type) {
	case nil:
	case []any:
		ui.Column(c.Key(path)).MinWidth(0).Children(func() { r.stack(c, v, path, depth+1, at, groupGap, false) })
	case *openuilang.Element:
		if v.StatementID != "" && slices.Contains(r.Replacing, v.StatementID) {
			r.updating(c.Key(path), at)
			return
		}
		r.element(c.Key(path), v, path, depth+1, at, as)
	case string:
		r.prose(c.Key(path), v, textStyle{size: at.bodySize(), color: c.Theme().Text, line: 1.55})
	default:
		r.prose(c.Key(path), fmt.Sprint(v), textStyle{size: at.bodySize(), color: c.Theme().Text, line: 1.55})
	}
}

func (r *Reading) updating(c *ui.Context, at place) {
	const statusHeight, bar = 18, 7
	theme := c.Theme()
	padding, radius := at.cardInset()
	bars := 3
	frame := ui.Column(c).MinWidth(0).Gap(innerGap).Radius(radius).Border(1, theme.Text.Alpha(0.08)).Clip().Busy(true)
	was, inset := frame.LayoutBounds(), padding
	if was.H > 0 {
		inset = min(padding, max(0, (was.H-statusHeight)/2))
		bars = min(bars, max(0, int((was.H-2*inset-statusHeight)/(bar+innerGap))))
		frame.Height(was.H)
	}
	switch {
	case !at.row:
	case was.W > 0:
		frame.Basis(was.W)
	default:
		frame.Grow(1).Basis(rowShare)
	}
	frame.Padding(inset, padding).Children(func() {
		updatingStatus(c).Font(portrait.FontSans).FontSize(portrait.Reader.Label).FixedLineHeight(statusHeight).TextColor(theme.TextMuted)
		skeleton(c, bars)
	})
}

func (at place) cardInset() (padding, radius float32) {
	if at.card || at.compact {
		return 16, 12
	}
	return 20, 14
}

func (at place) bodySize() float32 {
	if at.compact {
		return 15
	}
	return 16
}

func (at place) textStyle(c *ui.Context, p props, subtitle bool) textStyle {
	theme := c.Theme()
	style := textStyle{size: at.bodySize(), line: 1.6, color: theme.Text, weight: 400}
	if face, ok := textType[p.text("size")]; ok {
		style.size, style.line = face.size, face.line
		if at.compact && p.text("size") == "md" {
			style.size, style.line = 15, 22.0/15
		}
	}
	if subtitle {
		style.color = theme.TextMuted
		if style.size > subtitleSize {
			style.size, style.line = subtitleSize, 18.0/subtitleSize
		}
	}
	if named, ok := ink(c, p.text("color")); ok {
		style.color = named
	}
	switch p.text("weight") {
	case "medium":
		style.weight = 500
	case "semibold":
		style.weight = 600
	}
	style.italic, style.struck = p.on("italic"), p.on("lineThrough")
	if lines, ok := p.number("maxLines"); ok {
		style.lines = int(lines + 0.5)
	}
	if p.on("tabularNums") {
		style.features = "tnum"
	}
	return style
}

type props struct{ values *openuilang.Object }

func (p props) value(name string) any {
	if p.values == nil {
		return nil
	}
	value, _ := p.values.Get(name)
	return value
}

func (p props) text(name string) string {
	switch value := p.value(name).(type) {
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(value)
	}
	return ""
}

func (p props) number(name string) (float32, bool) {
	n, ok := p.value(name).(float64)
	if !ok || n != n || n < 0 {
		return 0, false
	}
	return float32(min(n, 4096)), true
}

func (p props) space(name string, fallback float32) float32 {
	if n, ok := p.number(name); ok {
		return min(n, 64) * spaceStep
	}
	return fallback
}

func (p props) on(name string) bool {
	on, _ := p.value(name).(bool)
	return on
}

var (
	crossAlign = map[string]ui.Align{"start": ui.Start, "center": ui.Center, "end": ui.End, "stretch": ui.Stretch}
	mainAlign  = map[string]ui.Align{"start": ui.Start, "center": ui.Center, "end": ui.End, "between": ui.SpaceBetween}
	titleType  = map[string]struct{ size, line, tracking float32 }{
		"sm": {15, 1.5, 0}, "md": {17, 1.4, -0.2}, "lg": {21, 1.3, -0.55}, "xl": {25, 1.2, -0.8}, "2xl": {32, 1.15, -1}, "3xl": {40, 1.1, -1.4},
	}
	textType = map[string]struct{ size, line float32 }{"xs": {12, 1.6}, "sm": {14, 1.55}, "md": {16, 1.6}, "lg": {18, 1.55}}
	radii    = map[string]float32{"sm": 6, "md": 10, "lg": 12, "xl": 14, "full": 999}
	charts   = map[string]ui.Color{"chart1": portrait.Reader.Charts[0], "chart2": portrait.Reader.Charts[1], "chart3": portrait.Reader.Charts[2], "chart4": portrait.Reader.Charts[3], "chart5": portrait.Reader.Charts[4], "chart6": portrait.Reader.Charts[5]}
	hexColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
)

func ink(c *ui.Context, name string) (ui.Color, bool) {
	t := c.Theme()
	name = strings.TrimSpace(name)
	switch name {
	case "":
		return ui.Color{}, false
	case "ink", "info":
		return t.Text, true
	case "muted", "faint":
		return t.TextMuted, true
	case "canvas":
		return t.Background, true
	case "surface":
		return t.Text.Alpha(0.04), true
	case "success":
		return t.Success.Mix(t.Text, 0.35), true
	case "warning":
		return t.Warning, true
	case "danger":
		return t.Danger, true
	case "purple":
		return portrait.Reader.Purple, true
	case "pink":
		return portrait.Reader.Pink, true
	}
	if color, ok := charts[name]; ok {
		return color, true
	}
	if hexColor.MatchString(name) {
		return portrait.ViewColor(name), true
	}
	return ui.Color{}, false
}

func fill(c *ui.Context, name string) (ui.Color, bool) {
	t := c.Theme()
	switch strings.TrimSpace(name) {
	case "surface", "info":
		return t.Text.Alpha(0.04), true
	case "surface-strong", "border":
		return t.Text.Alpha(0.08), true
	case "success", "warning", "danger", "purple", "pink":
		color, _ := ink(c, name)
		return color.Alpha(0.12), true
	}
	return ink(c, name)
}

func flex(box ui.Element, p props) ui.Element {
	return arrange(box.MinWidth(0), p)
}

func arrange(box ui.Element, p props) ui.Element {
	if a, ok := crossAlign[p.text("align")]; ok {
		box.AlignItems(a)
	}
	if a, ok := mainAlign[p.text("justify")]; ok {
		box.Justify(a)
	}
	if p.on("wrap") {
		box.Wrap()
	}
	return box
}

func (r *Reading) state(path string) *componentState {
	state := r.states[path]
	if state == nil {
		state = &componentState{}
		r.states[path] = state
	}
	return state
}

func (r *Reading) element(c *ui.Context, e *openuilang.Element, path string, depth int, at place, as treatment) {
	p := props{e.Props}
	theme := c.Theme()
	_, gapped := p.number("gap")
	switch e.Type {
	case "Col":
		inner := at
		inner.row, inner.nested = false, true
		tier := float32(sectionGap)
		if at.nested || at.compact {
			tier = groupGap
		}
		col := ui.Column(c)
		if at.row && isTitleBlock(e) {
			arrange(col.MaxWidthPercent(100), p)
		} else {
			flex(col, p)
		}
		if at.row && wide(p.value("children")) {
			col.Grow(1).Basis(rowShare)
		}
		col.Children(func() {
			r.stack(c, p.value("children"), path, depth, inner, p.space("gap", tier), !gapped)
		})
	case "Row":
		inner := at
		inner.row = true
		gap := p.space("gap", innerGap)
		row := flex(ui.Row(c), p).Gap(gap)
		if p.text("align") == "" {
			row.AlignItems(ui.Center)
		}
		if _, set := p.value("wrap").(bool); !set && (rigid(p.value("children")) || wide(p.value("children")) || holds(p.value("children"), "Col", wide) || holds(p.value("children"), "Source", wide)) {
			row.Wrap()
		}
		r.rowChildren(c, row, p, gap, path, depth, inner)
	case "Box":
		inner := at
		box := ui.Column(c)
		inner.row = p.text("direction") == "row"
		if inner.row {
			box = ui.Row(c)
		}
		flex(box, p).Padding(p.space("padding", 0))
		if !inner.row {
			box.AlignItems(ui.Stretch)
			if a, ok := crossAlign[p.text("align")]; ok {
				box.AlignItems(a)
			}
		}
		if color, ok := fill(c, p.text("background")); ok {
			box.Background(color)
		}
		if name, ok := p.value("radius").(string); ok {
			box.Radius(radii[name]).Clip()
		} else if radius, ok := p.number("radius"); ok {
			box.Radius(radius).Clip()
		}
		if p.on("border") {
			box.Border(1, theme.Text.Alpha(0.08))
		}
		length(box.Width, box.WidthPercent, p.value("width"))
		length(box.Height, box.HeightPercent, p.value("height"))
		if grow, ok := p.number("flex"); ok && grow > 0 {
			box.Grow(grow).Basis(0)
		}
		if items := flatten(p.value("children"), nil); at.row && len(items) == 1 && kind(items[0]) == "Image" {
			box.WidthPercent(33).MinWidth(120).MaxWidth(200).Shrink(0)
			inner.row = false
		}
		gap := p.space("gap", 0)
		if inner.row {
			r.rowChildren(c, box.Gap(gap), p, gap, path, depth, inner)
		} else {
			box.Children(func() { r.stack(c, p.value("children"), path, depth, inner, gap, false) })
		}
	case "Grid":
		state := r.state(path)
		gap := p.space("gap", groupGap)
		columns := 2
		if n, ok := p.number("columns"); ok {
			columns = min(12, max(1, int(n+0.5)))
		}
		least, _ := p.number("minChildWidth")
		least = max(least, 132)
		if width := state.frame.Bounds(c).W; width > 0 {
			columns = max(1, min(columns, int((width+gap)/(least+gap))))
		}
		tracks := make([]ui.Track, columns)
		for i := range tracks {
			tracks[i] = ui.Fr(1)
		}
		inner := at
		inner.row, inner.compact, inner.nested = false, true, true
		grid := ui.Grid(c).Bind(&state.frame).MinWidth(0).ColumnTracks(tracks...).Gap(gap)
		if at.row {
			grid.Grow(1).Basis(rowShare)
		}
		items := flatten(p.value("children"), nil)
		inCells := len(items)
		if columns > 1 && inCells%columns == 1 && grouping(items[inCells-1]) {
			inCells--
		}
		grid.Children(func() {
			r.span(c, items, 0, inCells, path, depth, inner, 0, false, treatment{fill: true})
			if inCells < len(items) {
				ui.Column(c.Key(path + "#alone")).ColumnSpan(-1).MinWidth(0).Children(func() {
					r.span(c, items, inCells, len(items), path, depth, inner, 0, false, treatment{})
				})
			}
		})
	case "Card":
		padding, radius := at.cardInset()
		inner := place{compact: true, nested: true, card: true}
		card := flex(ui.Column(c), p).Padding(p.space("padding", padding)).Radius(radius)
		if at.row {
			card.Grow(1).Basis(rowShare).AlignSelf(ui.Stretch)
		}
		if as.fill {
			card.Grow(1)
		}
		if p.text("variant") == "soft" {
			card.Background(theme.Text.Alpha(0.04))
		} else {
			card.Background(theme.Background).Border(1, theme.Text.Alpha(0.08))
		}
		card.Children(func() {
			r.stack(c, p.value("children"), path, depth, inner, p.space("gap", groupGap), !gapped)
		})
	case "Divider":
		ui.Column(c).Height(1).Shrink(0).Background(theme.Text.Alpha(0.08)).Role(ui.RoleSeparator)
	case "Spacer":
		ui.Column(c).Grow(1).Basis(0).MinWidth(0).AccessHidden(true)
	case "Title":
		face, ok := titleType[p.text("size")]
		if !ok {
			face = titleType["md"]
		}
		if largest := titleType[largestCompactTitle]; at.compact && face.size > largest.size {
			face = largest
		}
		color := theme.Text
		if named, ok := ink(c, p.text("color")); ok {
			color = named
		}
		title := ui.Text(c, p.text("text")).Font(typeface(p.text("text"))).FontSize(face.size).FontWeight(600).LineHeight(face.line).LetterSpacing(face.tracking).TextColor(color).Role(ui.RoleHeading).Wrap()
		if at.row {
			title.MaxWidthPercent(100)
		} else {
			title.MinWidth(0)
		}
		r.selectable(title)
	case "Text":
		style := at.textStyle(c, p, as.subtitle)
		run, inline := p.value("text").([]any)
		if !inline {
			r.prose(c, p.text("text"), style)
			return
		}
		ui.Row(c).Wrap().GapX(3).AlignItems(ui.Center).MinWidth(0).Children(func() {
			for i, part := range flatten(run, nil) {
				at := path + "/" + strconv.Itoa(i)
				if _, ok := part.(*openuilang.Element); ok {
					r.piece(c, part, at, depth, place{compact: true, row: true}, treatment{})
					continue
				}
				text, ok := part.(string)
				if !ok {
					text = fmt.Sprint(part)
				}
				if text = strings.TrimSpace(text); text != "" {
					r.prose(c.Key(at), text, style)
				}
			}
		})
	case "Caption":
		style := textStyle{size: 12, line: 1.6, color: theme.TextMuted, weight: 400}
		if as.subtitle {
			style.size, style.line = subtitleSize, 18.0/subtitleSize
		}
		if named, ok := ink(c, p.text("color")); ok {
			style.color = named
		}
		r.prose(c, p.text("text"), style)
	case "Label":
		r.prose(c, p.text("text"), textStyle{size: 13, line: 1.4, color: theme.Text, weight: 500})
	case "Badge":
		color, tint := theme.Text, theme.Text.Alpha(0.06)
		if name := p.text("color"); name != "" && name != "neutral" && name != "info" {
			color, _ = ink(c, name)
			tint, _ = fill(c, name)
		}
		pill := ui.Row(c).Gap(5).Padding(3, 10).MinHeight(26).Radius(999).Background(tint).AlignItems(ui.Center).Shrink(0)
		if !at.row {
			pill.AlignSelf(ui.Start)
		}
		pill.Children(func() {
			r.glyph(c, p.text("icon"), 14, color)
			ui.Text(c, p.text("text")).Font(typeface(p.text("text"))).FontSize(portrait.Reader.Label).FontWeight(500).FixedLineHeight(18).TextColor(color)
		})
	case "Link":
		address := p.text("url")
		if parsed, err := url.Parse(address); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https" || parsed.Scheme == "mailto") {
			ui.Link(c, p.text("text"), address).FontSize(at.bodySize())
		} else {
			r.prose(c, p.text("text"), textStyle{size: at.bodySize(), line: 1.55, color: theme.Text})
		}
	case "List":
		r.list(c, p, flatten(p.value("items"), nil), path, depth)
	case "ListItem":
		r.list(c, props{}, []any{e}, path, depth)
	case "Markdown":
		ui.Column(c).MinWidth(0).Children(func() { markdown.View(c, p.text("text"), &readOnlyMarkdown) })
	case "Quote":
		ui.Row(c).MinWidth(0).AlignItems(ui.Stretch).Radius(0, 12, 12, 0).Clip().Background(theme.Text.Alpha(0.04)).Children(func() {
			ui.Column(c).Width(3).Shrink(0).Background(theme.Text).AccessHidden(true)
			ui.Column(c).Gap(4).Padding(14, 16).Grow(1).Basis(0).MinWidth(0).Children(func() {
				r.prose(c.Key("text"), p.text("text"), textStyle{size: 16, line: 1.6, color: theme.Text})
				if cite := p.text("cite"); cite != "" {
					r.prose(c.Key("cite"), cite, textStyle{size: 13, line: 1.5, color: theme.TextMuted})
				}
			})
		})
	case "Code":
		name := "Code"
		if language := p.text("language"); language != "" {
			name = language + " code"
		}
		ui.Column(c).Radius(12).Clip().Border(1, theme.Text.Alpha(0.08)).Background(theme.Text.Alpha(0.04)).MinWidth(0).Role(ui.RoleGroup).Label(name).Children(func() {
			if language := p.text("language"); language != "" {
				ui.Row(c).Padding(6, 14).Children(func() {
					ui.Text(c, language).FontSize(portrait.Reader.Caption).TextColor(theme.TextMuted).AccessHidden(true)
				})
				ui.Column(c).Height(1).Shrink(0).Background(theme.Text.Alpha(0.08)).AccessHidden(true)
			}
			ui.Column(c).Padding(14, 16).MinWidth(0).Children(func() {
				ui.Text(c, p.text("code")).Font(portrait.FontMono).FontSize(portrait.Type.Code.Size).LineHeight(1.6).MinWidth(0).Wrap().Selectable()
			})
		})
	case "Source":
		frame := ui.Column(c).MinWidth(0)
		fill := as.fill
		if at.row && wide(p.value("children")) {
			frame.Grow(1).Basis(rowShare).AlignSelf(ui.Stretch)
			fill = true
		}
		frame.Children(func() {
			tier := float32(sectionGap)
			if at.nested || at.compact {
				tier = groupGap
			}
			inner := at
			inner.row = false
			items := flatten(p.value("children"), nil)
			r.span(c, items, 0, len(items), path, depth, inner, tier, true, treatment{fill: fill})
			n := Node{ID: path, ComponentID: r.surface.owners[e]}
			for _, id := range flatten(p.value("sources"), nil) {
				if text, ok := id.(string); ok {
					n.Sources = append(n.Sources, text)
				}
			}
			if r.Sources != nil && len(n.Sources) > 0 {
				ui.Column(c.Key("sources")).Padding(innerGap, 0, 0, 0).MinWidth(0).Children(func() { r.Sources(c, n) })
			}
		})
	default:
		if !r.media(c, e, p, path, depth, at) && !r.control(c, e, p, path, depth, at) {
			ui.Text(c, fmt.Sprintf("“%s” is not a component this app can show.", e.Type)).Role(ui.RoleAlert)
		}
	}
}

func length(fixed, percent func(float32) ui.Element, value any) {
	switch v := value.(type) {
	case float64:
		if v >= 0 {
			fixed(float32(min(v, 4096)))
		}
	case string:
		v = strings.TrimSpace(v)
		if share, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 32); err == nil && share >= 0 && strings.HasSuffix(v, "%") {
			percent(float32(min(share, 100)))
		} else if size, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 32); err == nil && size >= 0 {
			fixed(float32(min(size, 4096)))
		}
	}
}

func typeface(text string) string {
	for _, r := range text {
		if r >= 0x2e80 && r <= 0x9fff || r >= 0xf900 && r <= 0xfaff || r >= 0xff00 && r <= 0xffef {
			return "IBM Plex Sans SC"
		}
	}
	return "IBM Plex Sans"
}

type textStyle struct {
	size, line     float32
	color          ui.Color
	weight, lines  int
	italic, struck bool
	whole          bool
	features       string
}

var inlineMark = regexp.MustCompile("\\*\\*([^*\n]+)\\*\\*|__([^_\n]+)__|\\*([^*\n]+)\\*|`([^`\n]+)`|~~([^~\n]+)~~|\\[([^\\]\n]+)\\]\\(((?:[^()\\s]|\\([^()\\s]*\\))+)\\)")

func (s textStyle) apply(text ui.Element, content string) ui.Element {
	text.Font(typeface(content)).FontSize(s.size).TextColor(s.color)
	if s.weight > 0 {
		text.FontWeight(s.weight)
	}
	if s.italic {
		text.Italic()
	}
	if s.struck {
		text.Strikethrough()
	}
	if s.features != "" {
		text.FontFeatures(s.features)
	}
	return text
}

func (r *Reading) prose(c *ui.Context, content string, style textStyle) {
	marks := inlineMark.FindAllStringSubmatchIndex(content, -1)
	block := func(text ui.Element) {
		if r.selectable(text.LineHeight(style.line)); style.whole {
			text.NoWrap()
		} else {
			text.MinWidth(0).Wrap()
		}
		if style.lines > 0 {
			text.MaxLines(style.lines)
		}
	}
	if len(marks) == 0 {
		block(style.apply(ui.Text(c, content), content))
		return
	}
	paragraph := style.apply(ui.RichText(c), content)
	block(paragraph)
	paragraph.Children(func() {
		at := 0
		for i, m := range marks {
			if m[0] > at {
				style.apply(ui.Text(c.Key("t"+strconv.Itoa(i)), content[at:m[0]]), content)
			}
			at = m[1]
			part := func(group int) (string, bool) {
				if m[2*group] < 0 {
					return "", false
				}
				return content[m[2*group]:m[2*group+1]], true
			}
			key := c.Key("m" + strconv.Itoa(i))
			if text, ok := part(1); ok {
				style.apply(ui.Text(key, text), content).FontWeight(600)
			} else if text, ok := part(2); ok {
				style.apply(ui.Text(key, text), content).FontWeight(600)
			} else if text, ok := part(3); ok {
				style.apply(ui.Text(key, text), content).Italic()
			} else if text, ok := part(4); ok {
				style.apply(ui.Text(key, text), content).Font(portrait.FontMono).FontSize(style.size - 1).TextBackground(c.Theme().Text.Alpha(0.06))
			} else if text, ok := part(5); ok {
				style.apply(ui.Text(key, text), content).Strikethrough()
			} else if text, ok := part(6); ok {
				address, _ := part(7)
				if parsed, err := url.Parse(address); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https" || parsed.Scheme == "mailto") {
					ui.Link(key, text, address).FontSize(style.size)
				} else {
					style.apply(ui.Text(key, text), content)
				}
			}
		}
		if at < len(content) {
			style.apply(ui.Text(c.Key("tail"), content[at:]), content)
		}
	})
}

const (
	widestMarkerColumn = 72
	markerSize         = 13
	markerFeatures     = "tnum"
)

func breaksLine(r rune) bool {
	switch r {
	case '\u00a0', '\u2007', '\u202f':
		return false
	}
	return unicode.IsSpace(r)
}

func (r *Reading) list(c *ui.Context, p props, items []any, path string, depth int) {
	theme := c.Theme()
	timeline, numbered, bare := p.on("timeline"), p.text("marker") == "number", p.text("marker") == "none"
	widestMarker, longestMarkerWord := float32(0), float32(0)
	for _, item := range items {
		if kind(item) != "ListItem" {
			continue
		}
		label := (props{item.(*openuilang.Element).Props}).text("marker")
		if label == "" {
			continue
		}
		measured := func(text string) float32 {
			width, _ := c.MeasureText(0, ui.Span{Text: text, Font: typeface(label), Size: markerSize, Features: markerFeatures})
			return float32(math.Ceil(float64(width)))
		}
		whole := measured(label)
		widestMarker = max(widestMarker, whole)
		if whole <= widestMarkerColumn {
			continue
		}
		for _, word := range strings.FieldsFunc(label, breaksLine) {
			longestMarkerWord = max(longestMarkerWord, measured(word))
		}
	}
	markerColumn := max(min(widestMarker, widestMarkerColumn), longestMarkerWord)
	labelled := markerColumn > 0
	gap := float32(10)
	if timeline {
		gap = 0
	}
	ui.Column(c).Gap(gap).MinWidth(0).Role(ui.RoleList).Children(func() {
		for i, item := range items {
			last := i == len(items)-1
			at := path + "/" + strconv.Itoa(i)
			label := ""
			if kind(item) == "ListItem" {
				label = (props{item.(*openuilang.Element).Props}).text("marker")
			}
			ui.Row(c.Key(at + "#row")).MinWidth(0).AlignItems(ui.Stretch).Role(ui.RoleListItem).Children(func() {
				if labelled {
					ui.Column(c.Key("label")).Width(markerColumn).Shrink(0).Children(func() {
						ui.Text(c, label).Font(typeface(label)).FontSize(markerSize).FixedLineHeight(24).TextColor(theme.TextMuted).Wrap().FontFeatures(markerFeatures)
					})
					ui.Column(c.Key("labelgap")).Width(12).Shrink(0)
				}
				switch {
				case timeline:
					ui.Column(c.Key("rail")).Width(9).Shrink(0).AlignItems(ui.Center).AccessHidden(true).Children(func() {
						ui.Column(c.Key("lead")).Height(8).Shrink(0)
						ui.Column(c.Key("dot")).Width(9).Height(9).Shrink(0).Radius(5).Border(2, theme.Text)
						if !last {
							ui.Column(c.Key("line")).Width(1).Grow(1).Margin(4, 0, 2, 0).Background(theme.Text.Alpha(0.10))
						}
					})
					ui.Column(c.Key("gap")).Width(14).Shrink(0)
				case numbered:
					ui.Column(c.Key("number")).Width(24).Height(24).Shrink(0).Radius(12).Background(theme.Text).AlignItems(ui.Center).Justify(ui.Center).AccessHidden(true).Children(func() {
						ui.Text(c, strconv.Itoa(i+1)).FontSize(portrait.Reader.Label).FontWeight(500).TextColor(theme.Background).FontFeatures("tnum")
					})
					ui.Column(c.Key("gap")).Width(12).Shrink(0)
				case !bare && !labelled:
					ui.Column(c.Key("bullet")).Width(14).Height(24).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).AccessHidden(true).Children(func() {
						ui.Column(c).Width(5).Height(5).Radius(3).Background(theme.TextMuted)
					})
					ui.Column(c.Key("gap")).Width(8).Shrink(0)
				}
				body := ui.Column(c.Key("body")).Grow(1).Basis(0).MinWidth(0)
				if timeline && !last {
					body.Padding(0, 0, 18, 0)
				}
				body.Children(func() {
					if kind(item) == "ListItem" {
						r.listItem(c.Key(at), item.(*openuilang.Element), at, depth, place{compact: true, nested: true})
						return
					}
					r.piece(c, item, at, depth, place{compact: true, nested: true}, treatment{})
				})
			})
		}
	})
}

func (r *Reading) listItem(c *ui.Context, e *openuilang.Element, path string, depth int, at place) {
	p := props{e.Props}
	theme := c.Theme()
	description, children := p.text("description"), flatten(p.value("children"), nil)
	weight := 400
	if description != "" || len(children) > 0 {
		weight = 500
	}
	ui.Column(c).Gap(2).MinWidth(0).Children(func() {
		r.prose(c.Key("title"), p.text("title"), textStyle{size: 15, line: 1.6, color: theme.Text, weight: weight})
		if description != "" {
			r.prose(c.Key("description"), description, textStyle{size: 14, line: 1.55, color: theme.TextMuted})
		}
		if len(children) > 0 {
			ui.Column(c.Key("children")).Padding(6, 0, 0, 0).MinWidth(0).Children(func() {
				r.stack(c, children, path, depth, place{compact: true, nested: true}, innerGap, false)
			})
		}
	})
}
