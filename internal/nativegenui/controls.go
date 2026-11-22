package nativegenui

import (
	"cmp"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Bayern99/mark-go-go/internal/openuilang"
	"github.com/Bayern99/mark-go-go/internal/portrait"
	"github.com/egoist/mygo/ui"
)

const (
	controlHeight = 40
	controlRadius = 10
	dateLayout    = "2006-01-02"
	burstLength   = 1600 * time.Millisecond
)

type press struct {
	label, form string
	action      any
}

type checkedField struct {
	name  string
	rules any
	value any
}

type bound struct {
	r                *Reading
	name, form, kind string
	binding          *openuilang.Binding
	literal          any
}

func (r *Reading) bind(e *openuilang.Element, p props, prop, path string) bound {
	b := bound{r: r, form: r.form, kind: e.Type, name: e.StatementID}
	if b.name == "" {
		b.name = p.text("label")
	}
	if b.name == "" {
		b.name = path
	}
	if binding, ok := p.value(prop).(*openuilang.Binding); ok {
		b.binding = binding
	} else {
		b.literal = p.value(prop)
	}
	return b
}

func (b bound) get() any {
	session := b.r.surface.Session
	if b.binding != nil {
		value, _ := session.Get(b.binding.Target)
		return value
	}
	if b.r.touched[b.form+"\x00"+b.name] {
		return session.Field(b.form, b.name)
	}
	return b.literal
}

func (b bound) set(value any) {
	session := b.r.surface.Session
	if b.binding != nil {
		session.Assign(b.binding, value)
	} else {
		session.SetField(b.form, b.kind, b.name, value)
		b.r.touched[b.form+"\x00"+b.name] = true
	}
	delete(b.r.problems, b.form+"\x00"+b.name)
	b.r.changed = true
}

func (b bound) check(rules, value any) (required bool, problem string) {
	spec, ok := rules.(*openuilang.Object)
	if !ok || b.form == "" || len(spec.Keys) == 0 {
		return false, ""
	}
	b.r.fields[b.form] = append(b.r.fields[b.form], checkedField{b.name, rules, value})
	on, _ := spec.Values["required"].(bool)
	return on, b.r.problems[b.form+"\x00"+b.name]
}

func (b bound) left(rules, value any) {
	if _, ok := rules.(*openuilang.Object); !ok || b.form == "" {
		return
	}
	key := b.form + "\x00" + b.name
	if problem := openuilang.ValidateField(value, rules); problem != "" {
		b.r.problems[key] = problem
	} else {
		delete(b.r.problems, key)
	}
}

func (r *Reading) selectable(text ui.Element) ui.Element {
	if r.pressable > 0 {
		return text
	}
	return text.Selectable()
}

func (r *Reading) press(label string, action any) {
	r.presses = append(r.presses, press{label, r.form, action})
}

func sends(action any) bool {
	plan, ok := action.(*openuilang.ActionPlan)
	if !ok {
		return true
	}
	for _, step := range plan.Steps {
		if step.Type == "continue_conversation" || step.Type == "custom_action" || step.Type == "run" && step.RefType == "mutation" {
			return true
		}
	}
	return false
}

func (r *Reading) valid(form string) bool {
	ok := true
	for _, field := range r.fields[form] {
		key := form + "\x00" + field.name
		if problem := openuilang.ValidateField(field.value, field.rules); problem != "" {
			r.problems[key], ok = problem, false
		} else {
			delete(r.problems, key)
		}
	}
	return ok
}

func webAddress(address string) bool {
	parsed, err := url.Parse(address)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.Host != ""
	case "mailto":
		return true
	}
	return false
}

func (r *Reading) settle(c *ui.Context) {
	presses := r.presses
	r.presses = nil
	for _, press := range presses {
		if press.form != "" && sends(press.action) && !r.valid(press.form) {
			r.changed = true
			continue
		}
		plan, ok := press.action.(*openuilang.ActionPlan)
		if !ok {
			continue
		}
		for _, event := range r.surface.Session.Trigger(press.label, press.form, plan) {
			if address, _ := event.Params.Values["url"].(string); event.Type == "open_url" && webAddress(address) {
				c.OpenURL(address)
			}
		}
		r.changed = true
	}
	if r.changed {
		r.changed = false
		r.surface.refresh()
		c.Services().Invalidate()
	}
}

func words(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	return ""
}

func amount(value any) float64 {
	switch v := value.(type) {
	case float64:
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			return v
		}
	case string:
		if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) {
			return n
		}
	}
	return 0
}

type choice struct{ value, label, description string }

func choices(value any) []choice {
	var out []choice
	for _, item := range flatten(value, nil) {
		switch v := item.(type) {
		case *openuilang.Object:
			text := v.Values["label"]
			if text == nil {
				text = v.Values["value"]
			}
			if text == nil {
				continue
			}
			one := choice{value: words(v.Values["value"]), label: words(text), description: words(v.Values["description"])}
			if v.Values["value"] == nil {
				one.value = one.label
			}
			out = append(out, one)
		case string, float64, bool:
			out = append(out, choice{value: words(v), label: words(v)})
		}
	}
	return out
}

func (r *Reading) labelled(c *ui.Context, label string, required bool, problem string, control func()) {
	theme := c.Theme()
	ui.Column(c).MinWidth(0).Gap(6).AlignItems(ui.Stretch).Children(func() {
		if label != "" {
			ui.Row(c.Key("label")).Gap(4).MinWidth(0).AccessHidden(true).Children(func() {
				ui.Text(c, label).FontSize(portrait.Reader.Field).FontWeight(500).TextColor(theme.Text).MinWidth(0).Wrap()
				if required {
					ui.Text(c, "*").FontSize(portrait.Reader.Field).TextColor(theme.Danger)
				}
			})
		}
		control()
		if problem != "" {
			ui.Text(c.Key("problem"), problem).FontSize(portrait.Reader.Label).TextColor(theme.Danger).Role(ui.RoleAlert).MinWidth(0).Wrap()
		}
	})
}

func named(el ui.Element, label, fallback string, required bool) ui.Element {
	name := label
	if name == "" {
		name = fallback
	}
	if required {
		name += ", required"
	}
	if name != "" {
		el.Label(name)
	}
	return el
}

func (r *Reading) button(c *ui.Context, p props, at place) {
	theme := c.Theme()
	tone := theme.Accent
	switch p.text("color") {
	case "danger":
		tone = theme.Danger
	case "success":
		tone = portrait.Reader.Success
	}
	height, size, pad := float32(controlHeight), float32(15), float32(16)
	switch p.text("size") {
	case "sm":
		height, size, pad = 32, 14, 12
	case "lg":
		height, size, pad = 48, 16, 20
	}
	back, front, border := tone, portrait.Colors.OnAccent, ui.Color{}
	switch p.text("variant") {
	case "outline":
		back, front, border = ui.Color{}, tone, tone.Alpha(0.45)
	case "soft":
		back, front = tone.Alpha(0.12), tone
	case "ghost":
		back, front = ui.Color{}, tone
	}
	label, disabled := p.text("label"), p.on("disabled")
	button := ui.Button(c, "").Label(label).Height(height).Padding(0, pad).Radius(controlRadius).Background(back).Border(1, border).Disabled(disabled).Shrink(0)
	if p.on("block") {
		button.AlignSelf(ui.Stretch)
		if at.row {
			button.Grow(1)
		}
	} else if !at.row {
		button.AlignSelf(ui.Start)
	}
	button.Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			r.glyph(c, p.text("icon"), size+2, front)
			ui.Text(c, label).FontSize(size).FontWeight(500).TextColor(front).AccessHidden(true)
		})
	})
	if button.Clicked() && !disabled {
		r.press(label, p.value("action"))
	}
}

func (r *Reading) control(c *ui.Context, e *openuilang.Element, p props, path string, depth int, at place) bool {
	theme := c.Theme()
	state := r.state(path)
	switch e.Type {
	case "Button":
		r.button(c, p, at)
	case "IconButton":
		label := p.text("label")
		if label == "" {
			label = p.text("icon")
		}
		back, front, border := ui.Color{}, theme.Text, theme.Text.Alpha(0.14)
		switch p.text("variant") {
		case "ghost":
			border = ui.Color{}
		case "solid":
			back, front, border = theme.Accent, portrait.Colors.OnAccent, ui.Color{}
		}
		button := ui.Button(c, "").Label(label).Tooltip(p.text("label")).Width(controlHeight).Height(controlHeight).Padding(0).Radius(999).Background(back).Border(1, border).Shrink(0).AlignSelf(ui.Start)
		button.Children(func() {
			ui.Row(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() { r.glyph(c, p.text("icon"), 18, front) })
		})
		if button.Clicked() {
			r.press(label, p.value("action"))
		}
	case "Pressable":
		_, acts := p.value("action").(*openuilang.ActionPlan)
		tile := ui.Column(c).MinWidth(0).Padding(p.space("padding", 0)).AlignItems(ui.Stretch)
		if color, ok := fill(c, p.text("background")); ok {
			tile.Background(color)
		}
		if name, ok := p.value("radius").(string); ok {
			tile.Radius(radii[name]).Clip()
		} else if radius, ok := p.number("radius"); ok {
			tile.Radius(radius).Clip()
		}
		if at.row && wide(p.value("children")) {
			tile.Grow(1).Basis(rowShare)
		}
		if p.on("selected") {
			tile.Border(2, theme.Accent)
		}
		if acts {
			tile.Focusable().Role(ui.RoleButton).Cursor(ui.CursorPointer).Selected(p.on("selected"))
		}
		inner := at
		inner.row = false
		if acts {
			r.pressable++
		}
		tile.Children(func() { r.stack(c, p.value("children"), path, depth, inner, groupGap, true) })
		if acts {
			r.pressable--
		}
		if acts && tile.Clicked() {
			r.press("", p.value("action"))
		}
	case "Form":
		outer := r.form
		r.form = e.StatementID
		if r.form == "" {
			r.form = path
		}
		inner := at
		inner.row, inner.nested = false, true
		ui.Column(c).MinWidth(0).AlignItems(ui.Stretch).Role(ui.RoleGroup).Children(func() {
			r.stack(c, p.value("children"), path, depth, inner, 16, false)
		})
		r.form = outer
	case "Input":
		field := r.bind(e, p, "value", path)
		value := field.get()
		numeric := p.text("type") == "number"
		required, problem := field.check(p.value("rules"), value)
		if shown := words(value); shown != state.shown && !(numeric && state.editing) {
			state.text, state.shown = shown, shown
		}
		r.labelled(c, p.text("label"), required, problem, func() {
			input := named(ui.TextInput(c, &state.text), p.text("label"), p.text("placeholder"), required).Placeholder(p.text("placeholder")).Height(controlHeight).Radius(controlRadius).MinWidth(0).Invalid(problem != "")
			state.editing = input.Focused()
			if state.text != state.shown {
				state.shown = state.text
				switch parsed, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(state.text), ",", ""), 64); {
				case !numeric:
					field.set(state.text)
				case strings.TrimSpace(state.text) == "":
					field.set(nil)
				case err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0):
					field.set(parsed)
				}
			}
			if input.FocusLeft() {
				state.shown = "\x00"
				field.left(p.value("rules"), field.get())
				r.changed = true
			}
		})
	case "TextArea":
		field := r.bind(e, p, "value", path)
		value := field.get()
		required, problem := field.check(p.value("rules"), value)
		if shown := words(value); shown != state.shown {
			state.text, state.shown = shown, shown
		}
		rows, ok := p.number("rows")
		if !ok || rows < 2 {
			rows = 4
		}
		r.labelled(c, p.text("label"), required, problem, func() {
			area := named(ui.TextArea(c, &state.text), p.text("label"), p.text("placeholder"), required).Placeholder(p.text("placeholder")).Height(min(rows, 20)*22 + 18).Radius(controlRadius).MinWidth(0).Invalid(problem != "")
			if state.text != state.shown {
				state.shown = state.text
				field.set(state.text)
			}
			if area.FocusLeft() {
				field.left(p.value("rules"), field.get())
				r.changed = true
			}
		})
	case "Select":
		field := r.bind(e, p, "value", path)
		value := words(field.get())
		required, problem := field.check(p.value("rules"), value)
		options := choices(p.value("options"))
		labels := make([]string, len(options))
		current := ""
		for i, option := range options {
			labels[i] = option.label
			if option.value == value {
				current = option.label
			}
		}
		state.text = current
		r.labelled(c, p.text("label"), required, problem, func() {
			placeholder := p.text("placeholder")
			if placeholder == "" {
				placeholder = "Choose…"
			}
			named(ui.Select(c, &state.text, labels), p.text("label"), placeholder, required).Placeholder(placeholder).Height(controlHeight).Radius(controlRadius).MinWidth(0).Invalid(problem != "")
			for _, option := range options {
				if state.text != current && option.label == state.text {
					field.set(option.value)
					break
				}
			}
		})
	case "SegmentedControl":
		field := r.bind(e, p, "value", path)
		value := words(field.get())
		options := choices(p.value("options"))
		labels := make([]string, len(options))
		current := -1
		for i, option := range options {
			labels[i] = option.label
			if option.value == value && current < 0 {
				current = i
			}
		}
		if len(options) == 0 {
			return true
		}
		state.tab = current
		segments(c, &state.tab, labels, p.on("block"), chosenSegment(path))
		if state.tab != current && state.tab >= 0 && state.tab < len(options) {
			field.set(options[state.tab].value)
		}
	case "Slider":
		r.slider(c, r.bind(e, p, "value", path), p, state)
	case "Checkbox":
		field := r.bind(e, p, "checked", path)
		checked := field.get() == true
		var held any = ""
		if checked {
			held = true
		}
		required, problem := field.check(p.value("rules"), held)
		state.expanded = checked
		label := p.text("label")
		ui.Column(c).MinWidth(0).Gap(4).Children(func() {
			box := named(ui.Checkbox(c, &state.expanded, ""), label, "", required).MinWidth(0)
			box.Children(func() {
				text := ui.Text(c, label).FontSize(portrait.Reader.Body).TextColor(theme.Text).MinWidth(0).Wrap().AccessHidden(true)
				if checked && p.on("lineThrough") {
					text.Strikethrough().TextColor(theme.TextMuted)
				}
			})
			if problem != "" {
				ui.Text(c.Key("problem"), problem).FontSize(portrait.Reader.Label).TextColor(theme.Danger).Role(ui.RoleAlert)
			}
		})
		if state.expanded != checked {
			field.set(state.expanded)
		}
	case "RadioGroup":
		r.radios(c, r.bind(e, p, "value", path), p, state)
	case "DatePicker":
		r.date(c, r.bind(e, p, "value", path), p, state)
	case "Timer":
		r.timer(c, r.bind(e, p, "seconds", path), p, state)
	case "Flashcard":
		side, other := "front", "back"
		if state.flipped {
			side, other = other, side
		}
		card := ui.Column(c).MinWidth(0).MinHeight(160).Padding(20).Gap(8).Radius(14).Border(1, theme.Text.Alpha(0.08)).Background(theme.Surface).AlignItems(ui.Center).Justify(ui.Center).Focusable().Role(ui.RoleButton).Cursor(ui.CursorPointer).Label(strings.TrimSpace(p.text(side) + " Flashcard, " + side + " side. Press to show the " + other + "."))
		if at.row {
			card.Grow(1).Basis(rowShare)
		}
		r.pressable++
		card.Children(func() {
			if text, ok := p.value(side).(string); ok {
				ui.Text(c.Key(side), text).Font(typeface(text)).FontSize(portrait.Reader.Flip).FontWeight(600).TextColor(theme.Text).TextAlign(ui.Center).MinWidth(0).Wrap()
			} else {
				ui.Column(c.Key(side)).MinWidth(0).AlignSelf(ui.Stretch).Children(func() {
					r.stack(c, p.value(side), path+"/"+side, depth, place{compact: true, nested: true}, groupGap, true)
				})
			}
			if hint := p.text("hint"); hint != "" && !state.flipped {
				ui.Text(c.Key("hint"), hint).FontSize(portrait.Reader.Label).TextColor(theme.TextMuted).TextAlign(ui.Center).MinWidth(0).Wrap()
			}
		})
		r.pressable--
		if card.Clicked() {
			state.flipped = !state.flipped
			c.Services().Invalidate()
		}
	case "Celebration":
		r.celebration(c, p.on("when"), state)
	default:
		return false
	}
	return true
}

func measure(value float64, unit string) string {
	text := grouped(value)
	switch unit {
	case "":
		return text
	case "$", "€", "£", "¥":
		return unit + text
	case "%":
		return text + unit
	}
	return text + " " + unit
}

func (r *Reading) slider(c *ui.Context, field bound, p props, state *componentState) {
	theme := c.Theme()
	lo, hi := amount(p.value("min")), amount(p.value("max"))
	if hi <= lo {
		hi = lo + 1
	}
	step := amount(p.value("step"))
	current := min(max(amount(field.get()), lo), hi)
	state.number = current
	label := p.text("label")
	ui.Column(c).MinWidth(0).Gap(8).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).Gap(12).MinWidth(0).Justify(ui.SpaceBetween).AlignItems(ui.Center).AccessHidden(true).Children(func() {
			ui.Text(c, label).FontSize(portrait.Reader.Field).FontWeight(500).TextColor(theme.Text).MinWidth(0).Wrap()
			ui.Text(c, measure(current, p.text("unit"))).FontSize(portrait.Reader.Field).FontWeight(600).FontFeatures("tnum").TextColor(theme.Text).Shrink(0)
		})
		slider := ui.Slider(c, &state.number, lo, hi).MinWidth(0)
		if label != "" {
			slider.Label(label)
		} else {
			slider.Label("Value")
		}
		if step > 0 {
			slider.Step(step)
		}
		slider.ObserveInput(func(ev ui.InputEvent) {
			if ev.Kind == ui.InputPointerDown && ev.Button == 0 {
				state.pointerPressed = true
			}
		})
		thumb := sliderThumbWidth(theme)
		if slider.Clicked() && state.pointerPressed {
			if x, _, over := slider.PointerPosition(); over && slider.Bounds().W > thumb {
				state.number = lo + float64(min(max((x-thumb/2)/(slider.Bounds().W-thumb), 0), 1))*(hi-lo)
			}
		}
		if _, held := slider.PressedButton(); !held {
			state.pointerPressed = false
		}
		r.marks(c, p.value("marks"), lo, hi, thumb/2, &state.spanned)
	})
	next := state.number
	if step > 0 {
		next = lo + math.Round((next-lo)/step)*step
		next, _ = strconv.ParseFloat(strconv.FormatFloat(next, 'g', 12, 64), 64)
	}
	if next = min(max(next, lo), hi); next != current {
		field.set(next)
	}
}

type chosenSegment string

func segments(c *ui.Context, chosen *int, labels []string, block bool, group chosenSegment) {
	theme := c.Theme()
	parts := ui.SegmentedBase(c, chosen, len(labels))
	track := parts.Track.Label("Choice").MinWidth(0).AlignItems(ui.Stretch).Padding(2).Gap(2).Radius(9).Background(theme.Text.Alpha(0.08)).InsetShadow(0, 1, 2, 0, portrait.Reader.TrackShade)
	track.Children(func() {
		for i, label := range labels {
			segment := parts.Segment(i).MinWidth(0).MinHeight(28).Padding(4, 12).Radius(7).Cursor(ui.CursorPointer)
			ink := theme.TextMuted
			if i == *chosen || segment.Hovered() {
				ink = theme.Text
			}
			segment.Children(func() {
				if i == *chosen {
					raised := ui.Row(c.Key("chosen")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(7).Background(theme.Background).Border(0.5, theme.Text.Alpha(0.10)).Shadow(0, 1, 2, 0, portrait.Reader.RaisedShade).AccessHidden(true)
					if !segment.FocusVisible() && !c.Preferences().ReduceMotion {
						raised.Transition(ui.ElementTransition{Match: group, Duration: 160 * time.Millisecond})
					}
				}
				ui.Text(c, label).ZIndex(1).Font(typeface(label)).FontSize(portrait.Reader.Label).FontWeight(500).TextColor(ink).MaxLines(1).Ellipsis("…")
			})
		}
	})
	if block {
		track.FillEqually()
	} else {
		track.AlignSelf(ui.Start)
	}
}

func sliderThumbWidth(theme *ui.Theme) float32 { return theme.Space(5) }

const (
	markLabelSize = 12
	markLabelGap  = 4
)

type sliderMark struct {
	at          float32
	label       string
	left, right float32
	drawn       bool
}

func (r *Reading) marks(c *ui.Context, listed any, lo, hi float64, thumbInset float32, track *spannedWidth) {
	theme := c.Theme()
	var marks []sliderMark
	for _, item := range flatten(listed, nil) {
		object, ok := item.(*openuilang.Object)
		if !ok {
			continue
		}
		value, ok := object.Values["value"].(float64)
		if label := words(object.Values["label"]); ok && value >= lo && value <= hi && label != "" {
			marks = append(marks, sliderMark{at: float32((value - lo) / (hi - lo)), label: label})
		}
	}
	if len(marks) == 0 {
		return
	}
	slices.SortStableFunc(marks, func(a, b sliderMark) int { return cmp.Compare(a.at, b.at) })
	first, last := &marks[0], &marks[len(marks)-1]
	first.drawn, last.drawn = true, true
	if width := track.lastDrawn(c); width > 0 {
		for i := range marks {
			mark := &marks[i]
			labelWidth, _ := c.MeasureText(0, ui.Span{Text: mark.label, Size: markLabelSize})
			switch mark.at {
			case 0:
				mark.left = -thumbInset
			case 1:
				mark.left = width + thumbInset - labelWidth
			default:
				mark.left = mark.at*width - labelWidth/2
			}
			mark.right = mark.left + labelWidth
		}
		before := first
		for i := 1; i < len(marks)-1; i++ {
			if mark := &marks[i]; mark.left >= before.right+markLabelGap && mark.right+markLabelGap <= last.left {
				mark.drawn, before = true, mark
			}
		}
	}
	ui.Column(c).MinWidth(0).Height(16).MarginX(thumbInset).AccessHidden(true).Bind(&track.frame).Children(func() {
		for i, mark := range marks {
			if !mark.drawn {
				continue
			}
			label := ui.Text(c.Key("mark"+strconv.Itoa(i)), mark.label).FontSize(markLabelSize).TextColor(theme.TextMuted).NoWrap().Absolute()
			switch mark.at {
			case 0:
				label.Left(-thumbInset)
			case 1:
				label.Right(-thumbInset)
			default:
				label.LeftPercent(mark.at*100).TranslatePercent(-50, 0)
			}
		}
	})
}

func (r *Reading) radios(c *ui.Context, field bound, p props, state *componentState) {
	theme := c.Theme()
	value := words(field.get())
	required, problem := field.check(p.value("rules"), value)
	options := choices(p.value("options"))
	variant := p.text("variant")
	state.text = value
	r.labelled(c, p.text("label"), required, problem, func() {
		group := named(ui.RadioGroup(c, func() {
			for i, option := range options {
				key := c.Key("option" + strconv.Itoa(i))
				if variant != "pills" && variant != "cards" {
					ui.Radio(key, &state.text, option.value, option.label)
					continue
				}
				chosen := option.value == value
				border, back := theme.Text.Alpha(0.14), ui.Color{}
				if chosen {
					border, back = theme.Accent, theme.Accent.Alpha(0.10)
				}
				name := option.label
				if option.description != "" {
					name += ", " + option.description
				}
				item := ui.Column(key).Focusable().Role(ui.RoleRadio).Checked(chosen).Label(name).Cursor(ui.CursorPointer).Border(1, border).Background(back).MinWidth(0)
				if variant == "pills" {
					item.Radius(999).Padding(6, 14)
				} else {
					item.Radius(12).Padding(12, 14).Gap(2).AlignSelf(ui.Stretch)
				}
				item.Children(func() {
					ui.Text(c, option.label).FontSize(portrait.Reader.Field).FontWeight(500).TextColor(theme.Text).MinWidth(0).Wrap().AccessHidden(true)
					if option.description != "" && variant == "cards" {
						ui.Text(c, option.description).FontSize(portrait.Reader.Label).TextColor(theme.TextMuted).MinWidth(0).Wrap().AccessHidden(true)
					}
				})
				if item.Clicked() {
					state.text = option.value
				}
			}
		}), p.text("label"), "Choice", required).MinWidth(0)
		if variant == "pills" {
			group.Row().Wrap().Gap(8)
		} else {
			group.Gap(8)
		}
	})
	if state.text != value {
		field.set(state.text)
	}
}

func spoken(day string) string {
	if parsed, err := time.Parse(dateLayout, day); err == nil {
		return parsed.Format("Jan 2, 2006")
	}
	return day
}

func (r *Reading) date(c *ui.Context, field bound, p props, state *componentState) {
	value := words(field.get())
	required, problem := field.check(p.value("rules"), value)
	now := c.Now()
	shown := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if parsed, err := time.ParseInLocation(dateLayout, value, now.Location()); err == nil {
		shown = parsed
	}
	state.date = shown
	if state.note != "" {
		problem = state.note
	}
	picked := false
	r.labelled(c, p.text("label"), required, problem, func() {
		picked = named(ui.DateInput(c, &state.date), p.text("label"), "Date", required).Height(controlHeight).Radius(controlRadius).MinWidth(0).Invalid(problem != "").Changed()
	})
	if !picked {
		return
	}
	day := state.date.Format(dateLayout)
	low, high := p.text("min"), p.text("max")
	_, lowErr := time.Parse(dateLayout, low)
	_, highErr := time.Parse(dateLayout, high)
	switch {
	case lowErr == nil && day < low:
		state.note = "Choose " + spoken(low) + " or later"
		r.changed = true
	case highErr == nil && day > high:
		state.note = "Choose " + spoken(high) + " or earlier"
		r.changed = true
	default:
		state.note = ""
		field.set(day)
	}
}

func (r *Reading) timer(c *ui.Context, field bound, p props, state *componentState) {
	theme := c.Theme()
	left := math.Max(0, math.Round(amount(field.get())))
	if !state.started {
		state.started, state.number = true, left
	}
	if left > state.number {
		state.number = left
	}
	if state.running && left > 0 {
		if passed := math.Floor(c.Now().Sub(state.date).Seconds()); passed >= 1 {
			left = math.Max(0, left-passed)
			state.date = state.date.Add(time.Duration(passed) * time.Second)
			field.set(left)
		}
		c.After(time.Second - c.Now().Sub(state.date))
	}
	if left == 0 {
		state.running = false
	}
	label := p.text("label")
	clock := fmt.Sprintf("%02d:%02d", int(left)/60, int(left)%60)
	ui.Column(c).MinWidth(0).Gap(8).Role(ui.RoleGroup).Label(strings.TrimSpace(label + " timer")).Children(func() {
		if label != "" {
			ui.Text(c, label).FontSize(portrait.Reader.Field).FontWeight(500).TextColor(theme.TextMuted).MinWidth(0).Wrap()
		}
		ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().MinWidth(0).Children(func() {
			ui.Text(c, clock).Font(portrait.FontMono).FontSize(portrait.Reader.Clock).FontWeight(600).TextColor(theme.Text).Shrink(0)
			toggle := "Start"
			if state.running {
				toggle = "Pause"
			}
			if ui.Button(c.Key("toggle"), toggle).Disabled(left == 0).Clicked() && left > 0 {
				state.running = !state.running
				state.date = c.Now()
				c.Services().Invalidate()
			}
			if ui.Button(c.Key("reset"), "Reset").Disabled(left == state.number && !state.running).Clicked() {
				state.running = false
				field.set(state.number)
			}
		})
	})
}

var confetti = portrait.Reader.Charts

func (r *Reading) celebration(c *ui.Context, when bool, state *componentState) {
	turned := when && !state.flipped && state.started
	state.flipped, state.started = when, true
	if c.Preferences().ReduceMotion {
		state.running = false
		return
	}
	if turned {
		state.running, state.date = true, c.Now()
	}
	if !state.running {
		return
	}
	passed := c.Now().Sub(state.date)
	if passed >= burstLength || !when {
		state.running = false
		return
	}
	c.After(16 * time.Millisecond)
	progress := float32(passed) / float32(burstLength)
	ui.Column(c).Height(0).AlignSelf(ui.Stretch).AccessHidden(true).Draw(func(painter *ui.Painter, box ui.Rect) {
		for i := range 48 {
			seed := float64(i)
			spread := float32(math.Sin(seed*12.9898)*0.5 + 0.5)
			speed := float32(math.Sin(seed*78.233)*0.5+0.5)*0.6 + 0.4
			sway := float32(math.Sin(seed+float64(progress)*8)) * 10
			x := box.X + box.W*spread + sway
			y := box.Y + progress*speed*260 - 20
			painter.Fill(ui.Rect{X: x, Y: y, W: 6, H: 10}, confetti[i%len(confetti)].Alpha(1-progress), 2)
		}
	})
}
