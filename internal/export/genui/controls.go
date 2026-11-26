package genui

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

func fieldLabel(out *strings.Builder, label string) {
	if label != "" {
		out.WriteString(`<label class="g-label">` + html.EscapeString(label) + `</label>`)
	}
}

func (r *renderer) button(out *strings.Builder, p props, iconOnly bool) {
	variant := p.text("variant")
	color := p.text("color")
	class := "g-button g-readonly"
	switch variant {
	case "outline", "soft", "ghost":
		class += " g-button-" + variant
	}
	if color == "danger" || color == "success" {
		class += " g-button-" + color
	}
	if p.on("disabled") {
		class += " g-disabled"
	}
	if p.on("block") {
		class += " g-block"
	}
	out.WriteString(`<span class="` + class + `" aria-disabled="true"`)
	if iconOnly && p.text("label") != "" {
		out.WriteString(` aria-label="` + html.EscapeString(p.text("label")) + `"`)
	}
	out.WriteString(`>`)
	if iconOnly {
		r.icon(out, p.text("icon"), "md", "")
		out.WriteString(`<span class="g-hidden">` + html.EscapeString(p.text("label")) + `</span>`)
	} else {
		if p.text("icon") != "" {
			r.icon(out, p.text("icon"), "sm", "")
		}
		out.WriteString(html.EscapeString(p.text("label")))
	}
	out.WriteString(`</span>`)
}

func (r *renderer) input(out *strings.Builder, p props, multiline bool) {
	out.WriteString(`<div class="g-control">`)
	fieldLabel(out, p.text("label"))
	value := show(r.bound(p.get("value")))
	if value == "" {
		value = p.text("placeholder")
	}
	if multiline {
		rows := max(2, min(20, int(p.number("rows", 4))))
		out.WriteString(`<pre class="g-field" aria-readonly="true" style="min-height:` + strconv.Itoa(rows*24) + `px;margin:0;white-space:pre-wrap">` + html.EscapeString(value) + `</pre>`)
	} else {
		out.WriteString(`<div class="g-field" role="textbox" aria-readonly="true">` + html.EscapeString(value) + `</div>`)
	}
	out.WriteString(`</div>`)
}

type choice struct{ value, label, description string }

func choices(value any) []choice {
	var result []choice
	for _, item := range flatten(value) {
		switch item := item.(type) {
		case *openuilang.Object:
			if item == nil {
				continue
			}
			label := show(item.Values["label"])
			if label == "" {
				label = show(item.Values["value"])
			}
			value := show(item.Values["value"])
			if value == "" {
				value = label
			}
			result = append(result, choice{value: value, label: label, description: show(item.Values["description"])})
		default:
			text := show(item)
			if text != "" {
				result = append(result, choice{value: text, label: text})
			}
		}
	}
	return result
}

func (r *renderer) selectControl(out *strings.Builder, p props) {
	selected := show(r.bound(p.get("value")))
	out.WriteString(`<div class="g-control">`)
	fieldLabel(out, p.text("label"))
	label := selected
	for _, option := range choices(p.get("options")) {
		if option.value == selected {
			label = option.label
		}
	}
	if label == "" {
		label = p.text("placeholder")
	}
	out.WriteString(`<div class="g-field" aria-readonly="true">` + html.EscapeString(label) + `</div></div>`)
}

func (r *renderer) checkbox(out *strings.Builder, p props) {
	checked, _ := r.bound(p.get("checked")).(bool)
	mark := "☐"
	style := ""
	if checked {
		mark = "☑"
		if p.on("lineThrough") {
			style = ` style="text-decoration:line-through;color:var(--ink-2)"`
		}
	}
	out.WriteString(`<div class="g-choice"><span class="g-choice-marker" aria-hidden="true">` + mark + `</span><span` + style + `>` + html.EscapeString(p.text("label")) + `</span></div>`)
}

func (r *renderer) radioGroup(out *strings.Builder, p props) {
	selected := show(r.bound(p.get("value")))
	out.WriteString(`<div class="g-control" role="radiogroup">`)
	fieldLabel(out, p.text("label"))
	for _, option := range choices(p.get("options")) {
		mark := "○"
		if option.value == selected {
			mark = "●"
		}
		out.WriteString(`<div class="g-choice"><span class="g-choice-marker" aria-hidden="true">` + mark + `</span><span>` + html.EscapeString(option.label))
		if option.description != "" {
			out.WriteString(`<span class="g-list-description">` + html.EscapeString(option.description) + `</span>`)
		}
		out.WriteString(`</span></div>`)
	}
	out.WriteString(`</div>`)
}

func (r *renderer) segmented(out *strings.Builder, p props) {
	selected := show(r.bound(p.get("value")))
	out.WriteString(`<div class="g-segments" aria-readonly="true">`)
	for _, option := range choices(p.get("options")) {
		class := "g-segment"
		if option.value == selected {
			class += " g-selected"
		}
		out.WriteString(`<span class="` + class + `">` + html.EscapeString(option.label) + `</span>`)
	}
	out.WriteString(`</div>`)
}

func (r *renderer) slider(out *strings.Builder, p props) {
	minimum, _ := number(p.get("min"))
	maximum, _ := number(p.get("max"))
	if maximum <= minimum {
		maximum = minimum + 1
	}
	current, ok := number(r.bound(p.get("value")))
	if !ok {
		current = minimum
	}
	current = min(maximum, max(minimum, current))
	percent := (current - minimum) / (maximum - minimum) * 100
	out.WriteString(`<div class="g-control"><div class="g-row" style="justify-content:space-between">`)
	fieldLabel(out, p.text("label"))
	out.WriteString(`<strong>` + html.EscapeString(measure(current, p.text("unit"))) + `</strong></div>`)
	out.WriteString(`<div class="g-slider-track" role="slider" aria-valuemin="` + numberString(minimum) + `" aria-valuemax="` + numberString(maximum) + `" aria-valuenow="` + numberString(current) + `" aria-readonly="true">`)
	out.WriteString(`<div class="g-slider-fill" style="width:` + numberString(percent) + `%"></div><span class="g-slider-thumb" style="left:` + numberString(percent) + `%"></span></div>`)
	for _, raw := range flatten(p.get("marks")) {
		if mark, ok := raw.(*openuilang.Object); ok && mark != nil {
			value, valid := number(mark.Values["value"])
			if valid && value >= minimum && value <= maximum {
				at := (value - minimum) / (maximum - minimum) * 100
				out.WriteString(`<span class="g-caption" style="margin-left:` + numberString(at) + `%">` + html.EscapeString(show(mark.Values["label"])) + `</span>`)
			}
		}
	}
	out.WriteString(`</div>`)
}

func measure(value float64, unit string) string {
	text := numberString(math.Round(value*100) / 100)
	switch unit {
	case "$", "€", "£", "¥":
		return unit + text
	case "%":
		return text + unit
	case "":
		return text
	default:
		return text + " " + unit
	}
}

func (r *renderer) datePicker(out *strings.Builder, p props) {
	out.WriteString(`<div class="g-control">`)
	fieldLabel(out, p.text("label"))
	out.WriteString(`<time class="g-field" datetime="` + html.EscapeString(show(r.bound(p.get("value")))) + `">` + html.EscapeString(show(r.bound(p.get("value")))) + `</time></div>`)
}

func (r *renderer) timer(out *strings.Builder, p props) {
	seconds, _ := number(r.bound(p.get("seconds")))
	seconds = max(0, seconds)
	whole := int(seconds)
	clock := fmt.Sprintf("%02d:%02d", whole/60, whole%60)
	if whole >= 3600 {
		clock = fmt.Sprintf("%d:%02d:%02d", whole/3600, whole/60%60, whole%60)
	}
	out.WriteString(`<div class="g-control">`)
	fieldLabel(out, p.text("label"))
	out.WriteString(`<div class="g-timer" aria-readonly="true">` + clock + `</div></div>`)
}

func (r *renderer) flashcard(out *strings.Builder, p props, depth int) {
	out.WriteString(`<div class="g-flashcard">`)
	for _, side := range []string{"front", "back"} {
		out.WriteString(`<section class="g-flash-side"><div class="g-flash-label">` + strings.ToUpper(side[:1]) + side[1:] + `</div>`)
		r.value(out, p.get(side), depth+1)
		if side == "front" && p.text("hint") != "" {
			out.WriteString(`<p class="g-caption">` + html.EscapeString(p.text("hint")) + `</p>`)
		}
		out.WriteString(`</section>`)
	}
	out.WriteString(`</div>`)
}
