package genui

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/export/theme"
)

var (
	svgViewBox   = regexp.MustCompile(`^[-+0-9.eE\s,]+$`)
	svgPoints    = regexp.MustCompile(`^[-+0-9.eE\s,]+$`)
	svgPathData  = regexp.MustCompile(`^[MmLlHhVvCcSsQqTtAaZz0-9\s,+\-.eE]+$`)
	svgTransform = regexp.MustCompile(`(?i)^[\s,]*(?:(?:translate|rotate|scale|skewx|skewy)\([-+0-9.eE\s,]+\)[\s,]*)+$`)
)

func (r *renderer) svg(out *strings.Builder, p props, depth int) {
	viewBox := p.text("viewBox")
	if !svgViewBox.MatchString(viewBox) {
		return
	}
	out.WriteString(`<svg class="g-svg" xmlns="http://www.w3.org/2000/svg" viewBox="` + html.EscapeString(viewBox) + `" role="img"`)
	if label := p.text("label"); label != "" {
		out.WriteString(` aria-label="` + html.EscapeString(label) + `"`)
	}
	style := cssLength("width", p.get("width")) + cssLength("height", p.get("height"))
	if style != "" {
		out.WriteString(` style="` + strings.TrimPrefix(style, ";") + `"`)
	}
	out.WriteString(`>`)
	r.value(out, p.get("children"), depth+1)
	out.WriteString(`</svg>`)
}

func (r *renderer) svgShape(out *strings.Builder, kind string, p props, depth int) {
	if kind == "G" {
		out.WriteString(`<g`)
		if transform := p.text("transform"); svgTransform.MatchString(transform) {
			out.WriteString(` transform="` + html.EscapeString(transform) + `"`)
		}
		writeOpacity(out, p)
		out.WriteString(`>`)
		r.value(out, p.get("children"), depth+1)
		out.WriteString(`</g>`)
		return
	}
	if kind == "SvgText" {
		out.WriteString(`<text x="` + svgNumber(p, "x", 0) + `" y="` + svgNumber(p, "y", 0) + `"`)
		out.WriteString(` font-size="` + svgNumber(p, "size", 14) + `"`)
		if anchor := p.text("anchor"); anchor == "start" || anchor == "middle" || anchor == "end" {
			out.WriteString(` text-anchor="` + anchor + `"`)
		}
		weights := map[string]int{"normal": 400, "medium": 500, "semibold": 600}
		if weight := weights[p.text("weight")]; weight > 0 {
			out.WriteString(` font-weight="` + strconv.Itoa(weight) + `"`)
		}
		writeSVGPaint(out, p, true, false)
		out.WriteString(`>` + html.EscapeString(p.text("text")) + `</text>`)
		return
	}
	if kind == "Polyline" || kind == "Polygon" {
		if !svgPoints.MatchString(p.text("points")) {
			return
		}
	}
	if kind == "Path" && !svgPathData.MatchString(p.text("d")) {
		return
	}
	tag := strings.ToLower(kind)
	out.WriteString(`<` + tag)
	attributes := map[string][]string{
		"Circle":   {"cx", "cy", "r"},
		"Ellipse":  {"cx", "cy", "rx", "ry"},
		"Rect":     {"x", "y", "width", "height", "rx"},
		"Line":     {"x1", "y1", "x2", "y2"},
		"Polyline": {},
		"Polygon":  {},
		"Path":     {},
	}
	for _, name := range attributes[kind] {
		if kind == "Rect" && name == "rx" && p.get(name) == nil {
			continue
		}
		out.WriteString(` ` + name + `="` + svgNumber(p, name, 0) + `"`)
	}
	switch kind {
	case "Polyline", "Polygon":
		out.WriteString(` points="` + html.EscapeString(p.text("points")) + `"`)
	case "Path":
		out.WriteString(` d="` + html.EscapeString(p.text("d")) + `"`)
	}
	writeSVGPaint(out, p, kind == "Circle" || kind == "Ellipse" || kind == "Rect" || kind == "Polygon", kind == "Line" || kind == "Path" || kind == "Polyline")
	out.WriteString(`/>`)
}

func writeSVGPaint(out *strings.Builder, p props, defaultFill, defaultNoFill bool) {
	fillName, strokeName := p.text("fill"), p.text("stroke")
	fill := theme.NamedColor(fillName)
	stroke := theme.NamedColor(strokeName)
	if fillName == "none" {
		fill = "none"
	}
	if strokeName == "none" {
		stroke = "none"
	}
	if fill == "" {
		switch {
		case defaultNoFill || stroke != "":
			fill = "none"
		case defaultFill:
			fill = "currentColor"
		}
	}
	if fill != "" {
		out.WriteString(` fill="` + fill + `"`)
	}
	if stroke == "" && strokeName != "none" && (defaultNoFill || p.get("strokeWidth") != nil) {
		stroke = "currentColor"
	}
	if stroke != "" {
		out.WriteString(` stroke="` + stroke + `"`)
	}
	if width, ok := number(p.get("strokeWidth")); ok && width >= 0 {
		out.WriteString(` stroke-width="` + numberString(width) + `"`)
	} else if defaultNoFill && stroke != "" {
		out.WriteString(` stroke-width="2"`)
	}
	writeOpacity(out, p)
}

func writeOpacity(out *strings.Builder, p props) {
	if opacity, ok := number(p.get("opacity")); ok && opacity >= 0 && opacity <= 1 {
		out.WriteString(` opacity="` + numberString(opacity) + `"`)
	}
}

func svgNumber(p props, name string, fallback float64) string {
	value := p.number(name, fallback)
	if value < -1_000_000 || value > 1_000_000 {
		value = fallback
	}
	return numberString(value)
}
