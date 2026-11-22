package nativegenui

import (
	_ "embed"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/Bayern99/mark-go-go/internal/openuilang"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

//go:embed icons.tsv
var iconTable string

var icons = sync.OnceValue(func() map[string]string {
	table := map[string]string{}
	for _, line := range strings.Split(iconTable, "\n") {
		name, code, ok := strings.Cut(line, "\t")
		if point, err := strconv.ParseUint(code, 16, 32); ok && err == nil {
			table[name] = string(rune(point))
		}
	}
	return table
})

var iconSizes = map[string]float32{"sm": 16, "md": 20, "lg": 24, "xl": 32}

func (r *Reading) glyph(c *ui.Context, name string, size float32, color ui.Color) {
	name = strings.ToLower(strings.TrimSpace(name))
	if glyph, ok := icons()[name]; ok {
		ui.Text(c.Key("icon:"+name), glyph).Font("Phosphor").FontSize(size).FixedLineHeight(size).TextColor(color).Shrink(0).AccessHidden(true)
	}
}

func (r *Reading) media(c *ui.Context, e *openuilang.Element, p props, path string, depth int, at place) bool {
	theme := c.Theme()
	switch e.Type {
	case "Icon":
		size, ok := iconSizes[p.text("size")]
		if !ok {
			size = 20
		}
		color := theme.Text
		if named, ok := ink(c, p.text("color")); ok {
			color = named
		}
		r.glyph(c, p.text("name"), size, color)
	case "Image":
		r.image(c, p, at)
	case "Table":
		r.table(c, p, path, depth, at)
	case "Chart":
		r.chart(c, p, path, at)
	case "Carousel":
		width, ok := p.number("itemWidth")
		if !ok || width < 80 {
			width = 240
		}
		strip := ui.ScrollHorizontal(c).MinWidth(0).Gap(p.space("gap", groupGap)).AlignItems(ui.Stretch)
		if at.row {
			strip.Grow(1).Basis(rowShare)
		}
		strip.Children(func() {
			for i, item := range flatten(p.value("children"), nil) {
				at := path + "/" + strconv.Itoa(i)
				ui.Column(c.Key(at + "#slide")).Width(width).Shrink(0).Children(func() {
					r.piece(c, item, at, depth, place{compact: true, nested: true}, treatment{})
				})
			}
		})
	case "Popover":
		state := r.state(path)
		anchor := ui.Row(c).MinWidth(0).AlignSelf(ui.Start).Cursor(ui.CursorPointer).Focusable().Role(ui.RoleButton).Expanded(state.open)
		anchor.Children(func() {
			r.stack(c, p.value("trigger"), path+"/trigger", depth, place{compact: true, row: true}, 0, false)
		})
		if anchor.Clicked() {
			state.open = !state.open
		}
		ui.Popover(c, anchor, &state.open, func() {
			ui.Column(c).Padding(14).MaxWidth(320).MinWidth(0).Children(func() {
				r.stack(c, p.value("content"), path+"/content", depth, place{compact: true, nested: true}, groupGap, true)
			})
		})
	default:
		return false
	}
	return true
}

func DocumentImages(source, directory string) func(src string) (ui.ImageSource, error) {
	known := map[string]bool{}
	document := parser.Parse(source)
	for block := range document.Len() {
		for _, inline := range document.Inlines(block) {
			if inline.Kind == parser.Image {
				known[inline.Dest] = true
			}
		}
	}
	load := markdown.FileImages(directory)
	return func(src string) (ui.ImageSource, error) {
		if !known[src] {
			return nil, errors.New("The original document does not contain this image.")
		}
		if parsed, err := url.Parse(src); err != nil || parsed.Scheme != "" || directory == "" {
			return nil, errors.New("Only image files beside the original document are shown.")
		}
		return load(src)
	}
}

type loadedImage struct {
	mu     sync.Mutex
	source ui.ImageSource
	done   bool
}

func (r *Reading) image(c *ui.Context, p props, at place) {
	theme := c.Theme()
	src, alt := p.text("src"), p.text("alt")
	ratio, ok := p.number("aspectRatio")
	if !ok || ratio < 0.2 || ratio > 8 {
		ratio = 0
	}
	radius := float32(12)
	if name, ok := p.value("radius").(string); ok {
		radius = radii[name]
	} else if n, ok := p.number("radius"); ok {
		radius = n
	}
	var source ui.ImageSource
	if r.LoadImage != nil && src != "" {
		value, loading := r.images.LoadOrStore(src, &loadedImage{})
		entry := value.(*loadedImage)
		if !loading {
			load, services := r.LoadImage, c.Services()
			go func() {
				loaded, err := load(src)
				if err != nil {
					loaded = nil
				}
				entry.mu.Lock()
				entry.source, entry.done = loaded, true
				entry.mu.Unlock()
				services.Invalidate()
			}()
		}
		entry.mu.Lock()
		source = entry.source
		entry.mu.Unlock()
	}
	sized := func(picture ui.Element) {
		if at.row {
			ratio = 4.0 / 3
			picture.WidthPercent(33).MinWidth(120).MaxWidth(200).Shrink(0)
		}
		if ratio > 0 {
			picture.AspectRatio(ratio)
		}
	}
	if source == nil {
		empty := ui.Column(c).MinWidth(0).Radius(radius).Background(theme.Text.Alpha(0.04)).AlignItems(ui.Center).Justify(ui.Center).Padding(16).Role(ui.RoleImage).Label(alt)
		if sized(empty); ratio == 0 {
			empty.MinHeight(96)
		}
		empty.Children(func() {
			r.glyph(c, "image", 24, theme.Text.Alpha(0.25))
			if alt != "" {
				ui.Text(c, alt).FontSize(12).TextColor(theme.TextMuted).Wrap().TextAlign(ui.Center).AccessHidden(true)
			}
		})
		return
	}
	fit := ui.Cover
	if p.text("fit") == "contain" {
		fit = ui.Contain
	}
	if ratio == 0 {
		if w, h := ui.ImageSize(source); w > 0 && h > 0 {
			ratio = w / h
		}
	}
	sized(ui.Image(c, source).Fit(fit).Radius(radius).MinWidth(0).Label(alt))
}

var (
	numericCell = regexp.MustCompile(`^[-+−]?[$€£¥₹]?\s?\d[\d,]*(\.\d+)?\s?(%|[kKmMbB]|bn)?$`)
	emptyCell   = regexp.MustCompile(`(?i)^(n/a|—|–|-)?$`)
)

func grouped(value float64) string {
	text := strconv.FormatFloat(math.Round(value*100)/100, 'f', -1, 64)
	whole, fraction, _ := strings.Cut(text, ".")
	sign := ""
	if strings.HasPrefix(whole, "-") {
		sign, whole = "−", whole[1:]
	}
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "," + whole[i:]
	}
	if fraction != "" {
		whole += "." + fraction
	}
	return sign + whole
}

const (
	cellInset     = 14
	noWrapEms     = 8
	widestInCell  = 240
	fitCacheLimit = 1 << 15
)

type cellContent int

const (
	blankCell cellContent = iota
	writtenCell
	rigidCell
	flowingCell
)

type measuredText struct {
	content string
	font    ui.Font
}

type textFit struct {
	least, natural float32
	lines          int
	whole          bool
}

func (s textStyle) font(content string) ui.Font {
	return ui.Font{Family: typeface(content), Size: s.size, Weight: s.weight, Italic: s.italic, Features: s.features}
}

func (r *Reading) fitted(content string, style textStyle) textFit {
	font := style.font(content)
	key := measuredText{content, font}
	if known, ok := r.fits[key]; ok {
		return known
	}
	shown := inlineMark.ReplaceAllStringFunc(content, func(mark string) string {
		for _, part := range inlineMark.FindStringSubmatch(mark)[1:7] {
			if part != "" {
				return part
			}
		}
		return mark
	})
	natural, _ := ui.ShapeText(shown, font, 0).Size()
	result := textFit{least: natural, whole: true}
	if floor := noWrapEms * font.Size; natural > floor {
		word, _ := ui.ShapeRichTextWith([]ui.Span{{Text: shown}}, font, ui.TextShape{Width: 1, NoBreakWords: true}).Size()
		result = textFit{least: max(word, floor)}
	}
	result.least = float32(math.Ceil(float64(result.least))) + 1
	result.natural, result.lines = natural, 1+strings.Count(strings.TrimSpace(shown), "\n")
	if r.fits == nil || len(r.fits) >= fitCacheLimit {
		r.fits = map[measuredText]textFit{}
	}
	r.fits[key] = result
	return result
}

func (r *Reading) table(c *ui.Context, p props, path string, depth int, at place) {
	theme := c.Theme()
	var columns []string
	for _, column := range flatten(p.value("columns"), nil) {
		columns = append(columns, (props{}).show(column))
	}
	var rows [][]any
	list, _ := p.value("rows").([]any)
	for _, row := range list {
		if cells, ok := row.([]any); ok {
			rows = append(rows, cells)
		}
	}
	if len(columns) == 0 {
		return
	}
	cell := func(row []any, column int) any {
		if column < len(row) {
			return row[column]
		}
		return nil
	}
	contentOf := func(value any) (cellContent, string) {
		switch value := value.(type) {
		case nil:
			return blankCell, ""
		case float64:
			return writtenCell, grouped(value)
		case string:
			return writtenCell, value
		case *openuilang.Element, []any:
			if rigid(value) {
				return rigidCell, ""
			}
			return flowingCell, ""
		}
		return writtenCell, (props{}).show(value)
	}
	cellAt := func(row, column int) string {
		return path + "/" + strconv.Itoa(row) + "/" + strconv.Itoa(column)
	}
	numeric := make([]bool, len(columns))
	weights := make([]float32, len(columns))
	for i, name := range columns {
		filled, widest := 0, len([]rune(name))
		numeric[i] = true
		for _, row := range rows {
			switch value := cell(row, i).(type) {
			case nil:
			case float64:
				filled++
				widest = max(widest, len(grouped(value)))
			case string:
				widest = max(widest, len([]rune(value)))
				if emptyCell.MatchString(strings.TrimSpace(value)) {
					continue
				}
				filled++
				numeric[i] = numeric[i] && numericCell.MatchString(strings.TrimSpace(value))
			case *openuilang.Element, []any:
				widest = max(widest, 14)
			default:
				filled++
				numeric[i] = false
			}
		}
		numeric[i] = numeric[i] && filled > 0
		weights[i] = float32(min(max(widest, 6), 28))
	}
	head := textStyle{size: 13, weight: 500, color: theme.TextMuted}
	body := func(column int) textStyle {
		style := textStyle{size: 14, line: 1.5, color: theme.Text}
		if numeric[column] {
			style.features = "tnum"
		}
		return style
	}
	widths := make([]float32, len(columns))
	total := float32(0)
	for i, name := range columns {
		least := r.fitted(name, head).least
		for n, row := range rows {
			switch content, text := contentOf(cell(row, i)); content {
			case writtenCell:
				least = max(least, r.fitted(text, body(i)).least)
			case rigidCell:
				least = max(least, float32(math.Ceil(float64(r.state(cellAt(n, i)+"#fit").frame.Bounds(c).W))))
			case flowingCell:
				least = max(least, noWrapEms*body(i).size)
			}
		}
		widths[i] = least + 2*cellInset
		total += widths[i]
	}
	line := func(key string) {
		ui.Column(c.Key(key)).Height(1).Shrink(0).Background(theme.Text.Alpha(0.08)).AccessHidden(true)
	}
	strip := func(key string, role ui.Role, build func(column int)) {
		ui.Row(c.Key(key)).AlignItems(ui.Stretch).Role(ui.RoleRow).Children(func() {
			for i := range columns {
				slot := ui.Column(c.Key(i)).Grow(weights[i]).Basis(widths[i]).Shrink(0).MinWidth(0).Padding(10, cellInset).Justify(ui.Center).Role(role)
				if numeric[i] {
					slot.AlignItems(ui.End)
				}
				slot.Children(func() { build(i) })
			}
		})
	}
	state := r.state(path)
	lines := func() {
		ui.Column(c.Key("head")).Background(theme.Text.Alpha(0.03)).Children(func() {
			strip("header", ui.RoleColumnHeader, func(i int) {
				name := head.apply(ui.Text(c, columns[i]), columns[i])
				if r.fitted(columns[i], head).whole {
					name.NoWrap()
				} else {
					name.Wrap()
				}
			})
		})
		for n, row := range rows {
			line("line" + strconv.Itoa(n))
			strip("row"+strconv.Itoa(n), ui.RoleCell, func(i int) {
				at, value := cellAt(n, i), cell(row, i)
				switch content, text := contentOf(value); content {
				case writtenCell:
					style := body(i)
					style.whole = r.fitted(text, style).whole
					r.prose(c.Key(at), text, style)
				case flowingCell:
					r.piece(c, value, at, depth, place{compact: true, nested: true}, treatment{})
				case rigidCell:
					side := ui.Row(c.Key(at + "#side")).AlignItems(ui.Center)
					if numeric[i] {
						side.Justify(ui.End)
					}
					side.Children(func() {
						ui.Row(c.Key(at + "#fit")).Bind(&r.state(at + "#fit").frame).Shrink(0).MaxWidth(widestInCell).AlignItems(ui.Center).Children(func() {
							r.piece(c, value, at, depth, place{compact: true, nested: true, row: true}, treatment{})
						})
					})
				}
			})
		}
	}
	frame := ui.Column(c).Bind(&state.frame).MinWidth(0).Radius(12).Clip().Border(1, theme.Text.Alpha(0.08))
	if at.row {
		frame.Grow(1).Basis(rowShare)
	}
	frame.Children(func() {
		grid := func(box ui.Element) {
			box.Role(ui.RoleTable).RowCount(len(rows) + 1).ColumnCount(len(columns)).Children(lines)
		}
		if room := state.frame.Bounds(c).W - 2; room > 0 && total > room {
			bar := ui.ScrollbarStyle{}.Resolve(theme).TrackWidth
			ui.ScrollHorizontal(c).MinWidth(0).KeepScrollbars(true).Padding(0, 0, bar, 0).Children(func() {
				grid(ui.Column(c).Width(total).Shrink(0))
			})
			return
		}
		grid(ui.Column(c).MinWidth(0))
	})
}

func (props) show(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return grouped(v)
	}
	return fmt.Sprint(value)
}

type series struct {
	name   string
	values []float64
	color  ui.Color
}

func palette(count int) []ui.Color {
	names := []string{"chart1", "chart2", "chart3", "chart4", "chart5", "chart6"}
	colors := make([]ui.Color, max(count, 1))
	for i := range colors {
		colors[i] = ui.Hex(charts[names[i%len(names)]])
	}
	if len(colors) > 2 && len(colors)%2 == 0 {
		colors = append(colors[len(colors)-1:], colors[:len(colors)-1]...)
	}
	return colors
}

func (r *Reading) chart(c *ui.Context, p props, path string, at place) {
	theme := c.Theme()
	var rows []*openuilang.Object
	for _, row := range flatten(p.value("data"), nil) {
		if object, ok := row.(*openuilang.Object); ok && object != nil {
			rows = append(rows, object)
		}
	}
	if len(rows) == 0 {
		return
	}
	category := p.text("x")
	if category == "" {
		for _, key := range rows[0].Keys {
			if _, ok := rows[0].Values[key].(string); ok {
				category = key
				break
			}
		}
		if category == "" && len(rows[0].Keys) > 0 {
			category = rows[0].Keys[0]
		}
	}
	var names []string
	for _, name := range flatten(p.value("series"), nil) {
		if text, ok := name.(string); ok {
			names = append(names, text)
		}
	}
	if len(names) == 0 {
		for _, key := range rows[0].Keys {
			if _, ok := rows[0].Values[key].(float64); ok && key != category {
				names = append(names, key)
			}
		}
	}
	if category == "" || len(names) == 0 {
		return
	}
	names = names[:min(len(names), 12)]
	rows = rows[:min(len(rows), 400)]
	labels := make([]string, len(rows))
	for i, row := range rows {
		labels[i] = (props{}).show(row.Values[category])
	}
	kind := p.text("type")
	switch kind {
	case "bar", "line", "area", "pie", "donut", "scatter":
	default:
		kind = "bar"
	}
	all := make([]series, len(names))
	colors := palette(len(names))
	named, _ := p.value("seriesLabels").(*openuilang.Object)
	for s, name := range names {
		all[s] = series{name: name, color: colors[s], values: make([]float64, len(rows))}
		if named != nil {
			if label := strings.TrimSpace(words(named.Values[name])); label != "" {
				all[s].name = label
			}
		}
		for i, row := range rows {
			if value, ok := row.Values[name].(float64); ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
				all[s].values[i] = value
			}
		}
	}
	var summary strings.Builder
	for i, label := range labels[:min(len(labels), 40)] {
		if i > 0 {
			summary.WriteString("; ")
		}
		summary.WriteString(label)
		for _, one := range all {
			summary.WriteString(", " + one.name + " " + grouped(one.values[i]))
		}
	}
	height, ok := p.number("height")
	if !ok || height < 80 {
		height = 240
	}
	height = min(height, 640)
	state := r.state(path)
	frame := ui.Column(c).Bind(&state.frame).Gap(groupGap).MinWidth(0).Role(ui.RoleImage).Label(strings.ToUpper(kind[:1]) + kind[1:] + " chart").Description(summary.String())
	if at.row {
		frame.Grow(1).Basis(rowShare)
	}
	frame.Children(func() {
		width := state.frame.Bounds(c).W
		switch kind {
		case "pie", "donut":
			r.pie(c, labels, all[0], kind == "donut", p.text("format") == "percent", min(height, 220))
			return
		case "scatter":
			r.scatter(c, rows, category, all, height)
		default:
			r.cartesian(c, kind, labels, all, p.on("stacked"), height, width)
		}
		if len(all) > 1 {
			ui.Row(c.Key("legend")).Wrap().GapY(6).GapX(16).Justify(ui.Center).AccessHidden(true).Children(func() {
				for i, one := range all {
					swatch(c.Key(i), one.name, one.color, theme)
				}
			})
		}
	})
}

func swatch(c *ui.Context, name string, color ui.Color, theme *ui.Theme) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).MinWidth(0).Children(func() {
		ui.Column(c).Width(8).Height(8).Radius(2).Shrink(0).Background(color)
		ui.Text(c, name).Font(typeface(name)).FontSize(12).TextColor(theme.TextMuted)
	})
}

func tickLabel(c *ui.Context, theme *ui.Theme) func(chart.Side, chart.Tick) {
	return func(_ chart.Side, t chart.Tick) {
		ui.Text(c, t.Label).Font(typeface(t.Label)).FontSize(11).TextColor(theme.TextMuted).FontFeatures("tnum").NoWrap()
	}
}

func abbreviate(ticks []chart.Tick) {
	largest := 0.0
	for _, tick := range ticks {
		largest = max(largest, math.Abs(tick.Value))
	}
	divisor, unit := 1.0, ""
	switch {
	case largest >= 1e9:
		divisor, unit = 1e9, "B"
	case largest >= 1e6:
		divisor, unit = 1e6, "M"
	case largest >= 1e4:
		divisor, unit = 1e3, "k"
	}
	for i, tick := range ticks {
		ticks[i].Label = grouped(tick.Value / divisor)
		if tick.Value != 0 {
			ticks[i].Label += unit
		}
	}
}

func valueScale(lo, hi float64) (chart.Linear, []chart.Tick) {
	lo, hi = min(lo, 0), max(hi, 0)
	if lo == hi {
		hi = lo + 1
	}
	scale := chart.NewLinear(lo, hi).Nice(5)
	ticks := scale.Ticks(5)
	abbreviate(ticks)
	return scale, ticks
}

func (r *Reading) cartesian(c *ui.Context, kind string, labels []string, all []series, stacked bool, height, width float32) {
	theme := c.Theme()
	stacked = stacked && kind != "line" && len(all) > 1
	var piles [][]chart.Segment
	lo, hi := 0.0, 0.0
	if stacked {
		values := make([][]float64, len(all))
		for s, one := range all {
			values[s] = one.values
		}
		piles = chart.Stack(values, chart.StackStandard)
		for _, pile := range piles {
			for _, part := range pile {
				lo, hi = min(lo, part.Lo, part.Hi), max(hi, part.Lo, part.Hi)
			}
		}
	} else {
		for _, one := range all {
			for _, value := range one.values {
				lo, hi = min(lo, value), max(hi, value)
			}
		}
	}
	y, yTicks := valueScale(lo, hi)
	band := chart.NewBand(labels...)
	if kind == "bar" {
		band = band.Padding(0.28, 0.14)
	}
	xTicks := band.Ticks()
	longest := 1
	for _, label := range labels {
		longest = max(longest, len([]rune(label)))
	}
	room := max(1, int(max(width-48, 120)/float32(min(longest, 14)*7+14)))
	if step := (len(xTicks) + room - 1) / room; step > 1 {
		kept := xTicks[:0:0]
		for i := 0; i < len(xTicks); i += step {
			kept = append(kept, xTicks[i])
		}
		xTicks = kept
	}
	for i := range xTicks {
		if text := []rune(xTicks[i].Label); len(text) > 14 {
			xTicks[i].Label = string(text[:13]) + "…"
		}
	}
	xs := make([]float32, len(labels))
	for i := range labels {
		xs[i] = band.At(i)
	}
	chart.Plot(c, chart.Axes{Left: yTicks, Bottom: xTicks, Label: tickLabel(c, theme)}, func(area ui.Element) {
		area.Draw(func(p *ui.Painter, box ui.Rect) {
			for _, tick := range yTicks {
				p.Line(box.X, chart.Y(box, tick.At), box.X+box.W, chart.Y(box, tick.At), 1, theme.Text.Alpha(0.07))
			}
			for s, one := range all {
				switch kind {
				case "bar":
					for i, value := range one.values {
						slot := band.Band(i)
						from, to := y.At(0), y.At(value)
						left, right := slot.Lo, slot.Hi
						if stacked {
							from, to = y.At(piles[s][i].Lo), y.At(piles[s][i].Hi)
						} else {
							share := (slot.Hi - slot.Lo) / float32(len(all))
							left = slot.Lo + share*float32(s)
							right = left + share - min(share*0.12, 0.004)
						}
						top, bottom := chart.Y(box, max(from, to)), chart.Y(box, min(from, to))
						bar := ui.Rect{X: chart.X(box, left), Y: top, W: (right - left) * box.W, H: bottom - top}
						radius := min(4, bar.W/2, bar.H/2)
						if stacked && s < len(all)-1 {
							radius = 0
						}
						p.Fill(bar, one.color, radius)
					}
				default:
					ys := make([]float32, len(one.values))
					floor := make([]float32, len(one.values))
					for i, value := range one.values {
						ys[i], floor[i] = y.At(value), y.At(0)
						if stacked {
							ys[i], floor[i] = y.At(piles[s][i].Hi), y.At(piles[s][i].Lo)
						}
					}
					if kind == "area" {
						p.FillPath(chart.Area(box, xs, floor, ys, chart.CurveMonotone), one.color.Alpha(0.16))
					}
					if len(xs) == 1 {
						p.Fill(ui.Rect{X: chart.X(box, xs[0]) - 3, Y: chart.Y(box, ys[0]) - 3, W: 6, H: 6}, one.color, 3)
						continue
					}
					p.StrokePath(chart.Line(box, xs, ys, chart.CurveMonotone), 2, one.color)
				}
			}
		})
	}).Height(height).Gap(8).MinWidth(0).AccessHidden(true)
}

func (r *Reading) scatter(c *ui.Context, rows []*openuilang.Object, key string, all []series, height float32) {
	theme := c.Theme()
	xs := make([]float64, len(rows))
	xLo, xHi, yLo, yHi := math.Inf(1), math.Inf(-1), 0.0, 0.0
	for i, row := range rows {
		xs[i], _ = row.Values[key].(float64)
		xLo, xHi = min(xLo, xs[i]), max(xHi, xs[i])
		for _, one := range all {
			yLo, yHi = min(yLo, one.values[i]), max(yHi, one.values[i])
		}
	}
	if xLo == xHi {
		xLo, xHi = xLo-1, xHi+1
	}
	x := chart.NewLinear(xLo, xHi).Nice(6)
	xTicks := x.Ticks(6)
	abbreviate(xTicks)
	y, yTicks := valueScale(yLo, yHi)
	chart.Plot(c, chart.Axes{Left: yTicks, Bottom: xTicks, Label: tickLabel(c, theme)}, func(area ui.Element) {
		area.Draw(func(p *ui.Painter, box ui.Rect) {
			for _, tick := range yTicks {
				p.Line(box.X, chart.Y(box, tick.At), box.X+box.W, chart.Y(box, tick.At), 1, theme.Text.Alpha(0.07))
			}
			for _, one := range all {
				for i, value := range one.values {
					p.Fill(ui.Rect{X: chart.X(box, x.At(xs[i])) - 4, Y: chart.Y(box, y.At(value)) - 4, W: 8, H: 8}, one.color.Alpha(0.8), 4)
				}
			}
		})
	}).Height(height).Gap(8).MinWidth(0).AccessHidden(true)
}

func (r *Reading) pie(c *ui.Context, labels []string, one series, ring, share bool, size float32) {
	theme := c.Theme()
	total := 0.0
	for _, value := range one.values {
		total += max(value, 0)
	}
	colors := palette(len(labels))
	slices := chart.Pie(one.values, 0.02)
	ui.Row(c).Wrap().GapY(16).GapX(28).AlignItems(ui.Center).Justify(ui.Center).MinWidth(0).AccessHidden(true).Children(func() {
		ui.Column(c).Width(size).Height(size).Shrink(0).Draw(func(p *ui.Painter, box ui.Rect) {
			outer := min(box.W, box.H) / 2
			inner := float32(0)
			if ring {
				inner = outer * 0.62
			}
			for i, slice := range slices {
				if slice.End > slice.Start {
					p.FillPath(chart.Sector(box.X+box.W/2, box.Y+box.H/2, inner, outer, slice), colors[i])
				}
			}
		})
		ui.Column(c).Gap(8).MinWidth(0).Children(func() {
			for i, label := range labels {
				ui.Row(c.Key(i)).Gap(8).AlignItems(ui.Center).MinWidth(0).Children(func() {
					ui.Column(c).Width(8).Height(8).Radius(2).Shrink(0).Background(colors[i])
					ui.Text(c, label).Font(typeface(label)).FontSize(13).TextColor(theme.Text).MinWidth(0).Wrap()
					value := grouped(one.values[i])
					if share && total > 0 {
						value = grouped(math.Round(max(one.values[i], 0)/total*1000)/10) + "%"
					}
					ui.Text(c, value).FontSize(13).TextColor(theme.TextMuted).FontFeatures("tnum").Shrink(0)
				})
			}
		})
	})
}
