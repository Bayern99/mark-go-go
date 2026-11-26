package genui

import (
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/export"
	"github.com/Bayern99/mark-go-go/internal/export/assets"
	markdownexport "github.com/Bayern99/mark-go-go/internal/export/markdown"
	"github.com/Bayern99/mark-go-go/internal/export/theme"
	"github.com/Bayern99/mark-go-go/internal/genuikit"
	md "github.com/Bayern99/mark-go-go/internal/markdown"
	"github.com/Bayern99/mark-go-go/internal/openuilang"
	"github.com/Bayern99/mark-go-go/internal/portrait"
)

var supportedComponents = func() map[string]bool {
	names := []string{
		"Col", "Row", "Box", "Grid", "Card", "Divider", "Spacer",
		"Title", "Text", "Caption", "Label", "Badge", "Link", "ListItem", "List", "Markdown", "Quote", "Code", "Source",
		"Table", "Chart", "Icon", "Image", "Svg", "G", "Circle", "Ellipse", "Rect", "Line", "Path", "Polyline", "Polygon", "SvgText",
		"Carousel", "Popover", "Button", "IconButton", "Pressable", "Form", "Input", "TextArea", "Select", "Checkbox", "RadioGroup", "SegmentedControl", "Slider", "DatePicker", "Timer", "Flashcard", "Celebration",
	}
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}()

type SectionSource struct {
	Heading    string
	Occurrence int
}

type noteGroup struct {
	key, id, heading string
	occurrence       int
	notes            []md.Note
	sourceID         string
}

type renderer struct {
	root       *openuilang.Element
	session    *openuilang.Session
	options    export.Options
	assets     *assets.Resolver
	warnings   []string
	icons      bool
	seen       map[*openuilang.Element]bool
	groups     map[string]*noteGroup
	aliases    map[string]*noteGroup
	groupOrder []*noteGroup
	sections   map[string]int
	sourceSeq  int
	sources    map[string]SectionSource
}

func Render(root *openuilang.Element, notes []md.Note, options export.Options) (export.Result, error) {
	return render(root, nil, nil, notes, options, nil)
}

func RenderSession(session *openuilang.Session, notes []md.Note, options export.Options) (export.Result, error) {
	if session == nil {
		return export.Result{}, errors.New("GenUI v3 session is unavailable")
	}
	root, problems := session.Render()
	result, err := render(root, session, problems, notes, options, nil)
	return result, err
}

func RenderSnapshot(root *openuilang.Element, problems []openuilang.Error, sources map[string]SectionSource, notes []md.Note, options export.Options) (export.Result, error) {
	return render(root, nil, problems, notes, options, sources)
}

func render(root *openuilang.Element, session *openuilang.Session, problems []openuilang.Error, notes []md.Note, options export.Options, sources map[string]SectionSource) (export.Result, error) {
	if root == nil {
		return export.Result{}, errors.New("GenUI view has no root component")
	}
	r := &renderer{
		root: root, session: session, options: options, assets: assets.NewWithOptions(options.BaseDir, options.OfflineImages),
		seen: map[*openuilang.Element]bool{}, groups: map[string]*noteGroup{}, aliases: map[string]*noteGroup{}, sections: map[string]int{}, sources: sources,
	}
	defer r.assets.Close()
	for _, problem := range problems {
		r.warnings = append(r.warnings, problem.Message)
	}
	if options.IncludeNotes {
		r.collectNotes(notes)
	}
	var body strings.Builder
	r.value(&body, root, 0)
	if options.IncludeNotes {
		r.writeNotes(&body)
	}
	additional := ""
	if r.icons {
		font, err := portrait.FontFile("Phosphor.ttf")
		if err != nil {
			return export.Result{}, err
		}
		additional = "@font-face{font-family:Phosphor;src:url(data:font/ttf;base64," + base64.StdEncoding.EncodeToString(font) + ") format('truetype');font-weight:400;font-style:normal}"
	}
	r.warnings = append(r.warnings, r.assets.Warnings()...)
	return export.Result{Data: theme.Document(options, body.String(), additional), Warnings: r.warnings}, nil
}

func sectionKey(heading string, occurrence int) string {
	if heading == "" {
		return "document"
	}
	return md.NormalizeHeadingText(heading) + "\x00" + strconv.Itoa(max(1, occurrence))
}

func (r *renderer) collectNotes(notes []md.Note) {
	for _, note := range notes {
		heading := note.SectionHeading
		key := note.SectionID
		if heading != "" || key == "" {
			key = sectionKey(heading, note.SectionOccurrence)
		}
		group := r.groups[key]
		if group == nil {
			if heading == "" {
				heading = "Document"
			}
			base := "notes-" + slug(heading)
			if key != "document" {
				base += "-" + strconv.Itoa(max(1, note.SectionOccurrence))
			}
			id := base
			for suffix := 2; r.noteGroupID(id); suffix++ {
				id = base + "-" + strconv.Itoa(suffix)
			}
			group = &noteGroup{key: key, id: id, heading: heading, occurrence: max(1, note.SectionOccurrence)}
			r.groups[key] = group
			r.groupOrder = append(r.groupOrder, group)
		}
		if note.SectionID != "" && r.aliases[note.SectionID] == nil {
			r.aliases[note.SectionID] = group
		}
		group.notes = append(group.notes, note)
	}
}

func (r *renderer) noteGroupID(id string) bool {
	for _, group := range r.groupOrder {
		if group.id == id {
			return true
		}
	}
	return false
}

func (r *renderer) sourceGroup(id string) (string, *noteGroup, SectionSource) {
	source, resolved := r.sources[id]
	key := id
	if resolved {
		key = sectionKey(source.Heading, source.Occurrence)
	}
	group := r.groups[key]
	if !resolved {
		group = r.aliases[id]
		if id == "" {
			group = r.groups["document"]
		}
	}
	return key, group, source
}

func (r *renderer) writeNotes(out *strings.Builder) {
	if len(r.groupOrder) == 0 {
		return
	}
	out.WriteString(`<section class="g-notes" id="reader-notes"><h2>Reader Notes</h2>`)
	for _, group := range r.groupOrder {
		out.WriteString(`<section class="g-note-group" id="` + group.id + `"><h3>` + html.EscapeString(group.heading))
		if group.occurrence > 1 {
			fmt.Fprintf(out, " (%d)", group.occurrence)
		}
		out.WriteString("</h3>")
		if group.sourceID != "" {
			out.WriteString(`<a class="g-note-back" href="#` + group.sourceID + `">↩ Source</a>`)
		}
		for _, note := range group.notes {
			out.WriteString(`<aside class="reader-note"><p>` + strings.ReplaceAll(html.EscapeString(note.Text), "\n", "<br>") + `</p></aside>`)
		}
		out.WriteString("</section>")
	}
	out.WriteString("</section>")
}

type props struct{ object *openuilang.Object }

func (p props) get(name string) any {
	if p.object == nil {
		return nil
	}
	value, _ := p.object.Get(name)
	return value
}

func (p props) text(name string) string { return show(p.get(name)) }
func (p props) on(name string) bool {
	value, _ := p.get(name).(bool)
	return value
}
func (p props) number(name string, fallback float64) float64 {
	if value, ok := number(p.get(name)); ok {
		return value
	}
	return fallback
}

func show(value any) string {
	switch value := value.(type) {
	case nil:
		return ""
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(value)
	case *openuilang.Object:
		if inner, ok := value.Get("value"); ok {
			return show(inner)
		}
	}
	return fmt.Sprint(value)
}

func number(value any) (float64, bool) {
	var n float64
	switch value := value.(type) {
	case float64:
		n = value
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return 0, false
		}
		n = parsed
	default:
		return 0, false
	}
	return n, !math.IsNaN(n) && !math.IsInf(n, 0)
}

func flatten(value any) []any {
	switch value := value.(type) {
	case nil:
		return nil
	case []any:
		var out []any
		for _, item := range value {
			out = append(out, flatten(item)...)
		}
		return out
	default:
		return []any{value}
	}
}

func (r *renderer) bound(value any) any {
	binding, ok := value.(*openuilang.Binding)
	if !ok || r.session == nil {
		return value
	}
	resolved, _ := r.session.Get(binding.Target)
	if object, ok := resolved.(*openuilang.Object); ok {
		if inner, exists := object.Get("value"); exists {
			return inner
		}
	}
	return resolved
}

func (r *renderer) value(out *strings.Builder, value any, depth int) {
	if depth > 128 {
		r.warnings = append(r.warnings, "GenUI nesting exceeds the export limit")
		return
	}
	switch value := value.(type) {
	case nil:
		return
	case string:
		out.WriteString(html.EscapeString(value))
	case float64, bool:
		out.WriteString(html.EscapeString(show(value)))
	case []any:
		for _, item := range value {
			r.value(out, item, depth+1)
		}
	case *openuilang.Element:
		r.element(out, value, depth)
	}
}

func (r *renderer) element(out *strings.Builder, element *openuilang.Element, depth int) {
	if element == nil || r.seen[element] {
		return
	}
	if r.seen == nil {
		r.seen = map[*openuilang.Element]bool{}
	}
	if !supportedComponents[element.Type] {
		out.WriteString(`<p class="g-caption">Unsupported component: ` + html.EscapeString(element.Type) + `</p>`)
		return
	}
	r.seen[element] = true
	defer delete(r.seen, element)
	p := props{element.Props}
	switch element.Type {
	case "Col":
		r.container(out, p, "g-col", depth)
	case "Row":
		r.container(out, p, "g-row", depth)
	case "Box":
		r.box(out, p, depth)
	case "Grid":
		r.grid(out, p, depth)
	case "Card":
		class := "g-col g-card"
		if p.text("variant") == "soft" {
			class += " g-soft"
		}
		r.container(out, p, class, depth)
	case "Divider":
		out.WriteString("<hr>")
	case "Spacer":
		out.WriteString(`<span class="g-spacer" aria-hidden="true"></span>`)
	case "Title":
		r.title(out, p)
	case "Text":
		r.text(out, p, depth)
	case "Caption":
		out.WriteString(`<p class="g-caption"` + colorStyle("color", p.text("color")) + `>` + html.EscapeString(p.text("text")) + `</p>`)
	case "Label":
		out.WriteString(`<div class="g-label">` + html.EscapeString(p.text("text")) + `</div>`)
	case "Badge":
		r.badge(out, p)
	case "Link":
		if href, ok := assets.SafeLink(p.text("url")); ok {
			out.WriteString(`<a class="g-link" href="` + html.EscapeString(href) + `">` + html.EscapeString(p.text("text")) + `</a>`)
		} else {
			out.WriteString(html.EscapeString(p.text("text")))
		}
	case "ListItem":
		r.listItem(out, p, depth)
	case "List":
		r.list(out, p, depth)
	case "Markdown":
		fragment, warnings, err := markdownexport.Fragment(p.text("text"), r.options, r.assets)
		r.warnings = append(r.warnings, warnings...)
		if err != nil {
			out.WriteString(`<p class="g-caption">` + html.EscapeString(err.Error()) + `</p>`)
		} else {
			out.WriteString(`<div class="g-markdown">` + fragment + `</div>`)
		}
	case "Quote":
		out.WriteString(`<blockquote class="g-quote"><div class="g-quote-body"><div>` + html.EscapeString(p.text("text")) + `</div>`)
		if cite := p.text("cite"); cite != "" {
			out.WriteString(`<div class="g-quote-cite">` + html.EscapeString(cite) + `</div>`)
		}
		out.WriteString(`</div></blockquote>`)
	case "Code":
		out.WriteString(markdownexport.CodeBlockHTML(p.text("code"), p.text("language")))
	case "Source":
		r.source(out, element, p, depth)
	case "Table":
		r.table(out, p, depth)
	case "Chart":
		r.chart(out, p)
	case "Icon":
		r.icon(out, p.text("name"), p.text("size"), p.text("color"))
	case "Image":
		r.image(out, p)
	case "Svg":
		r.svg(out, p, depth)
	case "G", "Circle", "Ellipse", "Rect", "Line", "Path", "Polyline", "Polygon", "SvgText":
		r.svgShape(out, element.Type, p, depth)
	case "Carousel":
		out.WriteString(`<div class="g-carousel">`)
		r.value(out, p.get("children"), depth+1)
		out.WriteString(`</div>`)
	case "Popover":
		out.WriteString(`<div class="g-popover"><div class="g-popover-trigger">`)
		r.value(out, p.get("trigger"), depth+1)
		out.WriteString(`</div><aside class="g-popover-note"><span class="g-caption">Popover contents</span>`)
		r.value(out, p.get("content"), depth+1)
		out.WriteString(`</aside></div>`)
	case "Button":
		r.button(out, p, false)
	case "IconButton":
		r.button(out, p, true)
	case "Pressable":
		style := fmt.Sprintf("padding:%spx", numberString(p.number("padding", 0)*4))
		if color := theme.NamedColor(p.text("background")); color != "" {
			style += ";background:" + color
		}
		style += cssRadius(p.get("radius"))
		out.WriteString(`<div class="g-pressable" aria-pressed="` + strconv.FormatBool(p.on("selected")) + `" style="` + style + `">`)
		r.value(out, p.get("children"), depth+1)
		out.WriteString(`</div>`)
	case "Form":
		out.WriteString(`<div role="group" class="g-form g-col" style="gap:16px">`)
		r.value(out, p.get("children"), depth+1)
		out.WriteString(`</div>`)
	case "Input":
		r.input(out, p, false)
	case "TextArea":
		r.input(out, p, true)
	case "Select":
		r.selectControl(out, p)
	case "Checkbox":
		r.checkbox(out, p)
	case "RadioGroup":
		r.radioGroup(out, p)
	case "SegmentedControl":
		r.segmented(out, p)
	case "Slider":
		r.slider(out, p)
	case "DatePicker":
		r.datePicker(out, p)
	case "Timer":
		r.timer(out, p)
	case "Flashcard":
		r.flashcard(out, p, depth)
	case "Celebration":
	}
}

func (r *renderer) container(out *strings.Builder, p props, class string, depth int) {
	gap := p.number("gap", 3) * 4
	style := fmt.Sprintf("gap:%spx", numberString(gap))
	if align := cssAlign(p.text("align")); align != "" {
		style += ";align-items:" + align
	}
	if justify := cssJustify(p.text("justify")); justify != "" {
		style += ";justify-content:" + justify
	}
	if p.on("wrap") {
		class += " g-wrap"
	}
	out.WriteString(`<div class="` + class + `" style="` + style + `">`)
	r.value(out, p.get("children"), depth+1)
	out.WriteString(`</div>`)
}

func (r *renderer) box(out *strings.Builder, p props, depth int) {
	class := "g-col"
	if p.text("direction") == "row" {
		class = "g-row"
	}
	style := fmt.Sprintf("gap:%spx;padding:%spx", numberString(p.number("gap", 0)*4), numberString(p.number("padding", 0)*4))
	if color := theme.NamedColor(p.text("background")); color != "" {
		style += ";background:" + color
	}
	if p.on("border") {
		style += ";border:1px solid var(--line)"
	}
	style += cssRadius(p.get("radius"))
	style += cssLength("width", p.get("width"))
	style += cssLength("height", p.get("height"))
	if flex := p.number("flex", 0); flex > 0 {
		style += ";flex:" + numberString(flex)
	}
	if align := cssAlign(p.text("align")); align != "" {
		style += ";align-items:" + align
	}
	if justify := cssJustify(p.text("justify")); justify != "" {
		style += ";justify-content:" + justify
	}
	if p.on("wrap") {
		class += " g-wrap"
	}
	out.WriteString(`<div class="` + class + `" style="` + style + `">`)
	r.value(out, p.get("children"), depth+1)
	out.WriteString(`</div>`)
}

func (r *renderer) grid(out *strings.Builder, p props, depth int) {
	columns := min(12, max(1, int(p.number("columns", 2)+0.5)))
	style := fmt.Sprintf("gap:%spx;--columns:%d", numberString(p.number("gap", 3)*4), columns)
	if minimum := p.number("minChildWidth", 0); minimum > 0 {
		style = fmt.Sprintf("gap:%spx;grid-template-columns:repeat(auto-fit,minmax(min(100%%,%spx),1fr))", numberString(p.number("gap", 3)*4), numberString(minimum))
	}
	out.WriteString(`<div class="g-grid" style="` + style + `">`)
	r.value(out, p.get("children"), depth+1)
	out.WriteString(`</div>`)
}

func (r *renderer) title(out *strings.Builder, p props) {
	sizes := map[string]float64{"sm": 15, "md": 17, "lg": 21, "xl": 25, "2xl": 32, "3xl": 40}
	size := sizes[p.text("size")]
	if size == 0 {
		size = 17
	}
	out.WriteString(`<h2 class="g-title" style="font-size:` + numberString(size) + `px`)
	if color := theme.NamedColor(p.text("color")); color != "" {
		out.WriteString(`;color:` + color)
	}
	out.WriteString(`">` + html.EscapeString(p.text("text")) + `</h2>`)
}

func (r *renderer) text(out *strings.Builder, p props, depth int) {
	sizes := map[string]float64{"xs": 12, "sm": 14, "md": 16, "lg": 18}
	size := sizes[p.text("size")]
	if size == 0 {
		size = 16
	}
	weights := map[string]int{"normal": 400, "medium": 500, "semibold": 600}
	weight := weights[p.text("weight")]
	style := "font-size:" + numberString(size) + "px"
	if weight > 0 {
		style += ";font-weight:" + strconv.Itoa(weight)
	}
	if p.on("italic") {
		style += ";font-style:italic"
	}
	if p.on("lineThrough") {
		style += ";text-decoration:line-through"
	}
	if p.on("tabularNums") {
		style += ";font-variant-numeric:tabular-nums"
	}
	if color := theme.NamedColor(p.text("color")); color != "" {
		style += ";color:" + color
	}
	if lines := int(p.number("maxLines", 0)); lines > 0 {
		style += ";display:-webkit-box;-webkit-line-clamp:" + strconv.Itoa(lines) + ";-webkit-box-orient:vertical;overflow:hidden"
	}
	if _, inline := p.get("text").([]any); inline {
		out.WriteString(`<span class="g-text" style="` + style + `">`)
		r.value(out, p.get("text"), depth+1)
		out.WriteString(`</span>`)
		return
	}
	out.WriteString(`<p class="g-text" style="` + style + `">` + html.EscapeString(p.text("text")) + `</p>`)
}

func (r *renderer) badge(out *strings.Builder, p props) {
	style := ""
	if color := theme.NamedColor(p.text("color")); color != "" && p.text("color") != "neutral" {
		style = ` style="color:` + color + `;border:1px solid ` + color + `"`
	}
	out.WriteString(`<span class="g-badge"` + style + `>`)
	if name := p.text("icon"); name != "" {
		r.icon(out, p.text("icon"), "sm", p.text("color"))
	}
	out.WriteString(html.EscapeString(p.text("text")) + `</span>`)
}

func (r *renderer) list(out *strings.Builder, p props, depth int) {
	class := "g-list"
	if p.text("marker") == "number" {
		class += " g-list-number"
	}
	if p.on("timeline") {
		class += " g-timeline"
	}
	tag := "ul"
	if p.text("marker") == "number" || p.on("timeline") {
		tag = "ol"
	}
	out.WriteString(`<` + tag + ` class="` + class + `">`)
	for _, item := range flatten(p.get("items")) {
		out.WriteString(`<li class="g-list-item">`)
		if element, ok := item.(*openuilang.Element); ok && element != nil && element.Type == "ListItem" {
			r.element(out, element, depth+1)
		} else {
			out.WriteString(`<span class="g-list-body">`)
			r.value(out, item, depth+1)
			out.WriteString(`</span>`)
		}
		out.WriteString(`</li>`)
	}
	out.WriteString(`</` + tag + `>`)
}

func (r *renderer) listItem(out *strings.Builder, p props, depth int) {
	out.WriteString(`<div class="g-list-body"><div class="g-list-title">` + html.EscapeString(p.text("title")) + `</div>`)
	if description := p.text("description"); description != "" {
		out.WriteString(`<div class="g-list-description">` + html.EscapeString(description) + `</div>`)
	}
	if marker := p.text("marker"); marker != "" {
		out.WriteString(`<div class="g-caption">` + html.EscapeString(marker) + `</div>`)
	}
	r.value(out, p.get("children"), depth+1)
	out.WriteString(`</div>`)
}

func (r *renderer) source(out *strings.Builder, element *openuilang.Element, p props, depth int) {
	ids := flatten(p.get("sources"))
	out.WriteString(`<div class="g-source">`)
	r.value(out, p.get("children"), depth+1)
	if len(ids) > 0 {
		out.WriteString(`<div class="g-sources">`)
		for _, raw := range ids {
			id := show(raw)
			key, group, source := r.sourceGroup(id)
			label := id
			if source.Heading != "" {
				label = source.Heading
				if source.Occurrence > 1 {
					label += fmt.Sprintf(" (%d)", source.Occurrence)
				}
			}
			if group != nil {
				label = group.heading
				if group.occurrence > 1 {
					label += fmt.Sprintf(" (%d)", group.occurrence)
				}
				r.sourceSeq++
				anchor := "source-" + slug(key) + "-" + strconv.Itoa(r.sourceSeq)
				r.sections[key]++
				if group.sourceID == "" {
					group.sourceID = anchor
				}
				out.WriteString(`<a class="g-source-link" id="` + anchor + `" href="#` + group.id + `">§ ` + html.EscapeString(label) + `</a>`)
				continue
			}
			out.WriteString(`<span class="g-source-link">§ ` + html.EscapeString(label) + `</span>`)
		}
		out.WriteString(`</div>`)
	}
	out.WriteString(`</div>`)
}

func (r *renderer) table(out *strings.Builder, p props, depth int) {
	columns := flatten(p.get("columns"))
	if len(columns) == 0 {
		return
	}
	out.WriteString(`<div class="table-wrap"><table><thead><tr>`)
	for _, column := range columns {
		out.WriteString(`<th>` + html.EscapeString(show(column)) + `</th>`)
	}
	out.WriteString(`</tr></thead><tbody>`)
	rows, _ := p.get("rows").([]any)
	for _, raw := range rows {
		cells, ok := raw.([]any)
		if !ok {
			continue
		}
		out.WriteString(`<tr>`)
		for i := range columns {
			var cell any
			if i < len(cells) {
				cell = cells[i]
			}
			align := ""
			if _, numeric := cell.(float64); numeric {
				align = ` align="right"`
			}
			out.WriteString(`<td` + align + `>`)
			r.value(out, cell, depth+1)
			out.WriteString(`</td>`)
		}
		out.WriteString(`</tr>`)
	}
	out.WriteString(`</tbody></table></div>`)
}

func (r *renderer) image(out *strings.Builder, p props) {
	resolved := r.assets.Image(p.text("src"))
	if resolved.Missing {
		name := resolved.Name
		if name == "" {
			name = p.text("alt")
		}
		out.WriteString(`<span class="image-missing" role="img">Image unavailable: ` + html.EscapeString(name) + `</span>`)
		return
	}
	style := cssRadius(p.get("radius"))
	if ratio := p.number("aspectRatio", 0); ratio >= 0.2 && ratio <= 8 {
		style += ";aspect-ratio:" + numberString(ratio)
	}
	fit := "cover"
	if p.text("fit") == "contain" {
		fit = "contain"
	}
	style += ";object-fit:" + fit
	out.WriteString(`<img src="` + html.EscapeString(resolved.URL) + `" alt="` + html.EscapeString(p.text("alt")) + `" style="` + style + `">`)
}

func (r *renderer) icon(out *strings.Builder, name, size, color string) {
	glyph, ok := genuikit.IconGlyph(name)
	if !ok {
		return
	}
	sizes := map[string]float64{"sm": 16, "md": 20, "lg": 24, "xl": 32}
	pixels := sizes[size]
	if pixels == 0 {
		pixels = 20
	}
	r.icons = true
	out.WriteString(`<span class="g-icon" aria-hidden="true" style="font-size:` + numberString(pixels) + `px`)
	if named := theme.NamedColor(color); named != "" {
		out.WriteString(`;color:` + named)
	}
	out.WriteString(`">` + html.EscapeString(glyph) + `</span>`)
}

func colorStyle(property, name string) string {
	if color := theme.NamedColor(name); color != "" {
		return ` style="` + property + `:` + color + `"`
	}
	return ""
}

func cssAlign(value string) string {
	switch value {
	case "start", "end":
		return "flex-" + value
	case "center", "stretch", "baseline":
		return value
	}
	return ""
}

func cssJustify(value string) string {
	switch value {
	case "start", "end":
		return "flex-" + value
	case "center":
		return value
	case "between", "around", "evenly":
		return "space-" + value
	}
	return ""
}

func cssRadius(value any) string {
	if name, ok := value.(string); ok {
		values := map[string]float64{"sm": 6, "md": 10, "lg": 12, "xl": 14, "full": 999}
		if radius := values[name]; radius > 0 {
			return ";border-radius:" + numberString(radius) + "px;overflow:hidden"
		}
	}
	if radius, ok := number(value); ok && radius >= 0 {
		return ";border-radius:" + numberString(min(radius, 4096)) + "px;overflow:hidden"
	}
	return ""
}

func cssLength(property string, value any) string {
	if n, ok := number(value); ok && n >= 0 {
		return ";" + property + ":" + numberString(min(n, 4096)) + "px"
	}
	if text, ok := value.(string); ok {
		if strings.HasSuffix(text, "%") {
			if n, err := strconv.ParseFloat(strings.TrimSuffix(text, "%"), 64); err == nil && n >= 0 && n <= 100 {
				return ";" + property + ":" + numberString(n) + "%"
			}
		}
		if strings.HasSuffix(text, "px") {
			if n, err := strconv.ParseFloat(strings.TrimSuffix(text, "px"), 64); err == nil && n >= 0 {
				return ";" + property + ":" + numberString(min(n, 4096)) + "px"
			}
		}
	}
	return ""
}

func numberString(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func slug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		return "section"
	}
	if len(id) > 80 {
		id = id[:80]
	}
	return id
}
