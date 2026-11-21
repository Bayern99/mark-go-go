package nativegenui

import (
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/Bayern99/mark-go-go/internal/genui"
	"github.com/Bayern99/mark-go-go/internal/openuilang"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

//go:embed catalog_v3.json
var statementCatalog []byte

var statementLibrary = sync.OnceValues(func() (*openuilang.Library, error) {
	return openuilang.LoadLibrary(statementCatalog)
})

func openStatements(data string) (*Surface, error) {
	view, err := genui.ReadStatementView([]byte(data))
	if err != nil {
		return nil, err
	}
	library, err := statementLibrary()
	if err != nil {
		return nil, err
	}
	text := ""
	for _, batch := range view.Statements {
		text = openuilang.Merge(text, strings.Join(batch, "\n"))
	}
	program := library.Parse(text)
	s := &Surface{Nodes: map[string]Node{}, Sources: map[string]Reference{}, Request: view.Request, Session: openuilang.NewSession()}
	for id, source := range view.Sources {
		s.Sources[id] = Reference{Heading: source.Heading, Occurrence: source.Occurrence, Hash: source.Hash}
	}
	s.Session.Load(program)
	var problems []openuilang.Error
	s.tree, problems = s.Session.Render()
	if s.tree == nil {
		return nil, errors.New("The view has no root component.")
	}
	seen := map[string]bool{}
	for _, problem := range append(program.Errors, problems...) {
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
	return s, nil
}

const maxStatementDepth = 64

func (r *Reading) statements(c *ui.Context) {
	r.piece(c, r.surface.tree, "root", 0)
	for i, notice := range r.surface.notices {
		ui.Text(c.Key("notice"+strconv.Itoa(i)), notice).Role(ui.RoleAlert)
	}
}

func (r *Reading) pieces(c *ui.Context, value any, path string, depth int) {
	items, ok := value.([]any)
	if !ok {
		if value != nil {
			r.piece(c, value, path, depth)
		}
		return
	}
	for i, item := range items {
		r.piece(c, item, path+"/"+strconv.Itoa(i), depth)
	}
}

func (r *Reading) piece(c *ui.Context, value any, path string, depth int) {
	if r.remaining <= 0 || depth > maxStatementDepth {
		if r.remaining == 0 {
			ui.Text(c, "This layout repeats too many components to display.").Role(ui.RoleAlert)
			r.remaining--
		}
		return
	}
	r.remaining--
	switch v := value.(type) {
	case nil:
		return
	case []any:
		r.pieces(c, v, path, depth+1)
		return
	case *openuilang.Element:
		r.element(c, v, path, depth+1)
		return
	case string:
		ui.Text(c.Key(path), v).Selectable()
	default:
		ui.Text(c.Key(path), fmt.Sprint(v)).Selectable()
	}
}

type props struct{ values *openuilang.Object }

func (p props) text(name string) string {
	value, _ := p.values.Get(name)
	text, _ := value.(string)
	return text
}

func (p props) number(name string, fallback float32) float32 {
	value, _ := p.values.Get(name)
	if n, ok := value.(float64); ok && n >= 0 && n <= 4096 {
		return float32(n)
	}
	return fallback
}

func (p props) flag(name string) (set, on bool) {
	value, _ := p.values.Get(name)
	on, set = value.(bool)
	return set, on
}

func (p props) value(name string) any {
	value, _ := p.values.Get(name)
	return value
}

var (
	crossAlign = map[string]ui.Align{"start": ui.Start, "center": ui.Center, "end": ui.End, "stretch": ui.Stretch}
	mainAlign  = map[string]ui.Align{"start": ui.Start, "center": ui.Center, "end": ui.End, "between": ui.SpaceBetween, "around": ui.SpaceAround, "evenly": ui.SpaceEvenly}
	titleSizes = map[string]float32{"sm": 16, "md": 18, "lg": 22, "xl": 26, "2xl": 32, "3xl": 40}
	textSizes  = map[string]float32{"xs": 11, "sm": 13, "md": 15, "lg": 17}
	radii      = map[string]float32{"sm": 4, "md": 8, "lg": 12, "xl": 16, "full": 999}
)

func tone(c *ui.Context, name string) (ui.Color, bool) {
	t := c.Theme()
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "":
		return ui.Color{}, false
	case "muted", "secondary", "neutral", "subtle":
		return t.TextMuted, true
	case "accent", "primary", "info", "purple", "pink":
		return t.Accent, true
	case "danger", "error":
		return t.Danger, true
	case "warning":
		return t.Warning, true
	case "success":
		return t.Success, true
	case "surface", "soft":
		return t.Surface, true
	case "border":
		return t.Border, true
	}
	hex := strings.TrimPrefix(strings.TrimSpace(name), "#")
	if len(hex) != len(name)-1 || (len(hex) != 3 && len(hex) != 4 && len(hex) != 6 && len(hex) != 8) {
		return ui.Color{}, false
	}
	if _, err := strconv.ParseUint(hex, 16, 64); err != nil {
		return ui.Color{}, false
	}
	return ui.Hex(name), true
}

func flow(box ui.Element, p props, gap float32) ui.Element {
	box.MinWidth(0).Gap(p.number("gap", gap))
	if a, ok := crossAlign[p.text("align")]; ok {
		box.AlignItems(a)
	}
	if a, ok := mainAlign[p.text("justify")]; ok {
		box.Justify(a)
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

func (r *Reading) element(c *ui.Context, e *openuilang.Element, path string, depth int) {
	p := props{e.Props}
	c = c.Key(path)
	children := func() { r.pieces(c, p.value("children"), path, depth) }
	switch e.Type {
	case "Col":
		flow(ui.Column(c), p, 12).Children(children)
	case "Row":
		row := flow(ui.Row(c), p, 12)
		if set, on := p.flag("wrap"); !set || on {
			row.Wrap()
		}
		row.Children(children)
	case "Box":
		box := ui.Column(c)
		if p.text("direction") == "row" {
			box = ui.Row(c)
			if set, on := p.flag("wrap"); !set || on {
				box.Wrap()
			}
		}
		flow(box, p, 0).Padding(p.number("padding", 0))
		if color, ok := tone(c, p.text("background")); ok {
			box.Background(color)
		}
		switch radius := p.value("radius").(type) {
		case string:
			box.Radius(radii[radius])
		case float64:
			box.Radius(p.number("radius", 0))
		}
		if _, on := p.flag("border"); on {
			box.Border(1, c.Theme().Border)
		}
		if _, ok := p.value("width").(float64); ok {
			box.Width(p.number("width", 0)).Shrink(1)
		}
		if _, ok := p.value("height").(float64); ok {
			box.Height(p.number("height", 0))
		}
		if flex := p.number("flex", 0); flex > 0 {
			box.Grow(flex).Basis(0)
		}
		box.Children(children)
	case "Grid":
		state := r.state(path)
		gap := p.number("gap", 12)
		least := p.number("minChildWidth", 0)
		columns := int(p.number("columns", 0))
		if least == 0 {
			least = 180
			if columns == 0 {
				columns = 3
			}
		}
		fit := columns
		if width := state.frame.Bounds(c).W; width > 0 {
			fit = max(1, int((width+gap)/(least+gap)))
			if columns > 0 {
				fit = min(fit, columns)
			}
		}
		tracks := make([]ui.Track, max(1, fit))
		for i := range tracks {
			tracks[i] = ui.Fr(1)
		}
		ui.Grid(c).Bind(&state.frame).MinWidth(0).ColumnTracks(tracks...).Gap(gap).Children(children)
	case "Card":
		card := flow(ui.Column(c), p, 8).Padding(p.number("padding", 16)).Radius(10)
		if p.text("variant") == "soft" {
			card.Background(c.Theme().Surface)
		} else {
			card.Border(1, c.Theme().Border)
		}
		card.Children(children)
	case "Divider":
		ui.Divider(c)
	case "Spacer":
		ui.Spacer(c)
	case "Title":
		size, ok := titleSizes[p.text("size")]
		if !ok {
			size = 22
		}
		title := ui.Text(c, p.text("text")).Bold().FontSize(size).Role(ui.RoleHeading).MinWidth(0).Wrap().Selectable()
		if color, ok := tone(c, p.text("color")); ok {
			title.TextColor(color)
		}
	case "Text":
		run, inline := p.value("text").([]any)
		if inline {
			ui.Row(c).Wrap().Gap(4).AlignItems(ui.Center).MinWidth(0).Children(func() {
				for i, part := range run {
					at := path + "/" + strconv.Itoa(i)
					if text, ok := part.(string); ok {
						r.words(c.Key(at), p, text)
					} else {
						r.piece(c, part, at, depth)
					}
				}
			})
			return
		}
		text, ok := p.value("text").(string)
		if !ok && p.value("text") != nil {
			text = fmt.Sprint(p.value("text"))
		}
		r.words(c, p, text)
	case "Caption":
		caption := ui.Text(c, p.text("text")).FontSize(12).TextColor(c.Theme().TextMuted).MinWidth(0).Wrap().Selectable()
		if color, ok := tone(c, p.text("color")); ok {
			caption.TextColor(color)
		}
	case "Label":
		ui.Text(c, p.text("text")).FontSize(12).FontWeight(600).TextColor(c.Theme().TextMuted).MinWidth(0).Wrap()
	case "Badge":
		badge := ui.Badge(c, p.text("text"))
		if color, ok := tone(c, p.text("color")); ok && p.text("color") != "neutral" {
			badge.Background(color.Alpha(0.16)).TextColor(color)
		}
	case "Link":
		address := p.text("url")
		if parsed, err := url.Parse(address); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https" || parsed.Scheme == "mailto") {
			ui.Link(c, p.text("text"), address)
		} else {
			ui.Text(c, p.text("text")).Selectable()
		}
	case "List":
		items, _ := p.value("items").([]any)
		_, timeline := p.flag("timeline")
		ui.Column(c).Gap(8).MinWidth(0).Role(ui.RoleList).Children(func() {
			for i, item := range items {
				marker := "•"
				switch {
				case p.text("marker") == "number":
					marker = strconv.Itoa(i+1) + "."
				case p.text("marker") == "none":
					marker = ""
				case timeline:
					marker = "○"
				}
				r.listItem(c, item, marker, path+"/"+strconv.Itoa(i), depth)
			}
		})
	case "ListItem":
		r.listItem(c, e, "•", path, depth)
	case "Markdown":
		ui.Column(c).MinWidth(0).Children(func() { markdown.View(c, p.text("text"), &readOnlyMarkdown) })
	case "Quote":
		ui.Row(c).Gap(12).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Width(3).MinHeight(16).Shrink(0).Radius(2).Background(c.Theme().TextMuted).AccessHidden(true)
			ui.Column(c).Gap(4).Grow(1).Basis(0).MinWidth(0).Children(func() {
				ui.Text(c, p.text("text")).Italic().Wrap().Selectable()
				if cite := p.text("cite"); cite != "" {
					ui.Text(c, "— "+cite).FontSize(12).TextColor(c.Theme().TextMuted).Wrap()
				}
			})
		})
	case "Code":
		name := "Code"
		if language := p.text("language"); language != "" {
			name = language + " code"
		}
		ui.Column(c).Padding(12).Radius(8).Background(c.Theme().Surface).MinWidth(0).Role(ui.RoleGroup).Label(name).Children(func() {
			ui.Text(c, p.text("code")).Font("IBM Plex Mono").FontSize(13).Wrap().Selectable()
		})
	case "Source":
		ui.Column(c).Gap(8).MinWidth(0).Children(func() {
			children()
			n := Node{ID: path, ComponentID: e.StatementID}
			ids, _ := p.value("sources").([]any)
			for _, id := range ids {
				if text, ok := id.(string); ok {
					n.Sources = append(n.Sources, text)
				}
			}
			if r.Sources != nil && len(n.Sources) > 0 {
				r.Sources(c, n)
			}
		})
	default:
		ui.Text(c, fmt.Sprintf("“%s” is not a component this app can show.", e.Type)).Role(ui.RoleAlert)
	}
}

func (r *Reading) words(c *ui.Context, p props, text string) {
	size, ok := textSizes[p.text("size")]
	if !ok {
		size = 15
	}
	words := ui.Text(c, text).FontSize(size).MinWidth(0).Wrap().Selectable()
	if color, ok := tone(c, p.text("color")); ok {
		words.TextColor(color)
	}
	switch p.text("weight") {
	case "medium":
		words.FontWeight(500)
	case "semibold":
		words.FontWeight(600)
	}
	if _, on := p.flag("italic"); on {
		words.Italic()
	}
	if _, on := p.flag("lineThrough"); on {
		words.Strikethrough()
	}
	if lines := int(p.number("maxLines", 0)); lines > 0 {
		words.MaxLines(lines)
	}
}

func (r *Reading) listItem(c *ui.Context, item any, marker, path string, depth int) {
	ui.Row(c.Key(path + "#item")).Gap(8).MinWidth(0).Role(ui.RoleListItem).Children(func() {
		entry, ok := item.(*openuilang.Element)
		if ok && entry.Type == "ListItem" {
			if own := (props{entry.Props}).text("marker"); own != "" {
				marker = own
			}
		}
		if marker != "" {
			ui.Text(c, marker).TextColor(c.Theme().TextMuted).Shrink(0).AccessHidden(true)
		}
		ui.Column(c).Gap(4).Grow(1).Basis(0).MinWidth(0).Children(func() {
			if !ok || entry.Type != "ListItem" {
				r.piece(c, item, path, depth)
				return
			}
			p := props{entry.Props}
			ui.Text(c, p.text("title")).FontWeight(600).Wrap().Selectable()
			if description := p.text("description"); description != "" {
				ui.Text(c, description).TextColor(c.Theme().TextMuted).Wrap().Selectable()
			}
			r.pieces(c, p.value("children"), path, depth)
		})
	})
}
