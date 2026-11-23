package nativegenui

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/Bayern99/mark-go-go/internal/openuilang"
	"github.com/egoist/mygo/ui"
)

type svgTransform struct {
	value  ui.Transform
	ox, oy float32
}

func (r *Reading) drawingProblem(path, component string, err error) {
	if r.drawingProblems == nil {
		r.drawingProblems = map[string]string{}
	}
	r.drawingProblems[path] = fmt.Sprintf("%s was left out: %v", component, err)
}

const maxSVGMagnitude = 1e6

func svgNumbers(text string) ([]float64, error) {
	fields := strings.FieldsFunc(text, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	values := make([]float64, len(fields))
	for i, field := range fields {
		value, err := strconv.ParseFloat(field, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > maxSVGMagnitude {
			return nil, fmt.Errorf("%q is not a supported finite number", field)
		}
		values[i] = value
	}
	return values, nil
}

func svgNumber(p props, name string, fallback float32) float32 {
	value, ok := p.value(name).(float64)
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		return fallback
	}
	return float32(max(-maxSVGMagnitude, min(value, maxSVGMagnitude)))
}

func svgOpacity(p props, inherited float32) float32 {
	return inherited * max(0, min(svgNumber(p, "opacity", 1), 1))
}

func svgColor(c *ui.Context, name string) (ui.Color, bool) {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "none") || strings.EqualFold(name, "transparent") {
		return ui.Color{}, false
	}
	if name == "currentColor" {
		return c.Theme().Text, true
	}
	return ink(c, name)
}

func svgPaint(c *ui.Context, p props, opacity float32, defaultFill, defaultStroke bool) (ui.Color, bool, ui.Color, bool, float32) {
	fill, filled := svgColor(c, p.text("fill"))
	stroke, stroked := svgColor(c, p.text("stroke"))
	if defaultFill && p.value("fill") == nil && p.value("stroke") == nil {
		fill, filled = c.Theme().Text, true
	}
	if defaultStroke && p.value("stroke") == nil {
		stroke, stroked = c.Theme().Text, true
	}
	width := max(0, svgNumber(p, "strokeWidth", 2))
	stroked = stroked && width > 0
	return fill.Alpha(opacity), filled, stroke.Alpha(opacity), stroked, width
}

func paintSVGPath(path *ui.Path, fill ui.Color, filled bool, stroke ui.Color, stroked bool, width float32) func(*ui.Painter) {
	return func(p *ui.Painter) {
		if filled {
			p.FillPath(path, fill)
		}
		if stroked {
			p.StrokePath(path, width, stroke)
		}
	}
}

var svgPointCharacters = regexp.MustCompile(`^[\s,+\-.0-9eE]+$`)
var svgPointNumber = regexp.MustCompile(`[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?`)

func svgPoints(text string, close bool) (*ui.Path, error) {
	if !svgPointCharacters.MatchString(text) || len(svgPointNumber.FindAllString(text, -1)) < 4 {
		return nil, errors.New("points must contain at least two coordinate pairs")
	}
	path, err := ui.ParsePath("M" + text)
	if err != nil {
		return nil, err
	}
	if close {
		path.Close()
	}
	return path, nil
}

func ellipsePath(cx, cy, rx, ry float32) *ui.Path {
	const k = 0.5522847498
	path := &ui.Path{}
	path.MoveTo(cx+rx, cy)
	path.CubeTo(cx+rx, cy+ry*k, cx+rx*k, cy+ry, cx, cy+ry)
	path.CubeTo(cx-rx*k, cy+ry, cx-rx, cy+ry*k, cx-rx, cy)
	path.CubeTo(cx-rx, cy-ry*k, cx-rx*k, cy-ry, cx, cy-ry)
	path.CubeTo(cx+rx*k, cy-ry, cx+rx, cy-ry*k, cx+rx, cy)
	return path.Close()
}

func (r *Reading) svgShape(c *ui.Context, e *openuilang.Element, path string, inheritedOpacity float32) func(*ui.Painter) {
	p := props{e.Props}
	opacity := svgOpacity(p, inheritedOpacity)
	if e.Type == "SvgText" {
		content := p.text("text")
		size := max(1, svgNumber(p, "size", 14))
		color, ok := svgColor(c, p.text("fill"))
		if !ok {
			color = c.Theme().Text
		}
		color = color.Alpha(opacity)
		weight := map[string]int{"normal": 400, "medium": 500, "semibold": 600}[p.text("weight")]
		if weight == 0 {
			weight = 400
		}
		span := ui.Span{Text: content, Font: typeface(content), Size: size, Weight: weight, Color: color}
		x, y := svgNumber(p, "x", 0), svgNumber(p, "y", 0)
		anchor := p.text("anchor")
		return func(painter *ui.Painter) {
			width, _ := painter.MeasureText(0, span)
			left := x
			switch anchor {
			case "middle":
				left -= width / 2
			case "end":
				left -= width
			}
			painter.RichText(left, y-size, 0, span)
		}
	}

	defaultFill := e.Type == "Circle" || e.Type == "Ellipse" || e.Type == "Rect" || e.Type == "Polygon"
	defaultStroke := e.Type == "Line" || e.Type == "Path" || e.Type == "Polyline"
	fill, filled, stroke, stroked, strokeWidth := svgPaint(c, p, opacity, defaultFill, defaultStroke)
	if e.Type == "Rect" {
		box := ui.Rect{X: svgNumber(p, "x", 0), Y: svgNumber(p, "y", 0), W: max(0, svgNumber(p, "width", 0)), H: max(0, svgNumber(p, "height", 0))}
		radius := min(max(0, svgNumber(p, "rx", 0)), box.W/2, box.H/2)
		return func(painter *ui.Painter) {
			if filled {
				painter.Fill(box, fill, radius)
			}
			if stroked {
				painter.Stroke(box, stroke, radius, strokeWidth)
			}
		}
	}

	var shape *ui.Path
	var err error
	switch e.Type {
	case "Circle":
		radius := max(0, svgNumber(p, "r", 0))
		shape = (&ui.Path{}).Circle(svgNumber(p, "cx", 0), svgNumber(p, "cy", 0), radius)
	case "Ellipse":
		shape = ellipsePath(svgNumber(p, "cx", 0), svgNumber(p, "cy", 0), max(0, svgNumber(p, "rx", 0)), max(0, svgNumber(p, "ry", 0)))
	case "Line":
		shape = (&ui.Path{}).MoveTo(svgNumber(p, "x1", 0), svgNumber(p, "y1", 0)).LineTo(svgNumber(p, "x2", 0), svgNumber(p, "y2", 0))
	case "Path":
		shape, err = ui.ParsePath(p.text("d"))
	case "Polyline":
		shape, err = svgPoints(p.text("points"), false)
	case "Polygon":
		shape, err = svgPoints(p.text("points"), true)
	default:
		return nil
	}
	if err != nil {
		r.drawingProblem(path, e.Type, err)
		return nil
	}
	return paintSVGPath(shape, fill, filled, stroke, stroked, strokeWidth)
}

func parseSVGTransforms(text string) ([]svgTransform, error) {
	var out []svgTransform
	for at := 0; at < len(text); {
		for at < len(text) && (text[at] == ',' || unicode.IsSpace(rune(text[at]))) {
			at++
		}
		if at == len(text) {
			break
		}
		start := at
		for at < len(text) && unicode.IsLetter(rune(text[at])) {
			at++
		}
		name := text[start:at]
		for at < len(text) && unicode.IsSpace(rune(text[at])) {
			at++
		}
		if name == "" || at == len(text) || text[at] != '(' {
			return nil, fmt.Errorf("invalid transform near %q", text[start:])
		}
		at++
		end := strings.IndexByte(text[at:], ')')
		if end < 0 {
			return nil, fmt.Errorf("transform %q has no closing parenthesis", name)
		}
		arguments := text[at : at+end]
		at += end + 1
		values, err := svgNumbers(arguments)
		if err != nil {
			return nil, fmt.Errorf("transform %q: %w", name, err)
		}
		one := svgTransform{}
		switch strings.ToLower(name) {
		case "translate":
			if len(values) < 1 || len(values) > 2 {
				return nil, errors.New("translate takes one or two numbers")
			}
			y := 0.0
			if len(values) == 2 {
				y = values[1]
			}
			one.value = ui.Translation(float32(values[0]), float32(y))
		case "scale":
			if len(values) < 1 || len(values) > 2 {
				return nil, errors.New("scale takes one or two numbers")
			}
			y := values[0]
			if len(values) == 2 {
				y = values[1]
			}
			one.value = ui.Scaling(float32(values[0]), float32(y))
		case "rotate":
			if len(values) != 1 && len(values) != 3 {
				return nil, errors.New("rotate takes one or three numbers")
			}
			one.value = ui.Rotation(float32(values[0]))
			if len(values) == 3 {
				one.ox, one.oy = float32(values[1]), float32(values[2])
			}
		case "skewx", "skewy":
			if len(values) != 1 {
				return nil, errors.New("skewX and skewY take one number")
			}
			x, y := 0.0, 0.0
			if strings.EqualFold(name, "skewX") {
				x = values[0]
			} else {
				y = values[0]
			}
			one.value = ui.Skewing(float32(x), float32(y))
		default:
			return nil, fmt.Errorf("transform %q is not supported", name)
		}
		out = append(out, one)
	}
	return out, nil
}

func drawSVGTransforms(painter *ui.Painter, transforms []svgTransform, draw func()) {
	if len(transforms) == 0 {
		draw()
		return
	}
	one := transforms[0]
	painter.Transform(one.value, one.ox, one.oy, func() { drawSVGTransforms(painter, transforms[1:], draw) })
}

func (r *Reading) svgShapes(c *ui.Context, children any, path string, inheritedOpacity float32) []func(*ui.Painter) {
	var out []func(*ui.Painter)
	for i, child := range flatten(children, nil) {
		e, ok := child.(*openuilang.Element)
		if !ok || e == nil {
			continue
		}
		at := path + "/" + strconv.Itoa(i)
		p := props{e.Props}
		if e.Type == "G" {
			transforms, err := parseSVGTransforms(p.text("transform"))
			if err != nil {
				r.drawingProblem(at, "G", err)
				continue
			}
			shapes := r.svgShapes(c, p.value("children"), at, svgOpacity(p, inheritedOpacity))
			out = append(out, func(painter *ui.Painter) {
				drawSVGTransforms(painter, transforms, func() {
					for _, draw := range shapes {
						draw(painter)
					}
				})
			})
			continue
		}
		if draw := r.svgShape(c, e, at, inheritedOpacity); draw != nil {
			out = append(out, draw)
		}
	}
	return out
}

func (r *Reading) svg(c *ui.Context, p props, path string, at place) {
	values, err := svgNumbers(p.text("viewBox"))
	if err == nil && (len(values) != 4 || values[2] <= 0 || values[3] <= 0) {
		err = errors.New("viewBox must be min-x, min-y and a positive width and height")
	}
	if err != nil {
		r.drawingProblem(path, "Svg", err)
	}
	frame := ui.Column(c).MinWidth(0).Clip()
	if at.row {
		frame.Grow(1).Basis(rowShare)
	}
	label := p.text("label")
	if label == "" {
		frame.AccessHidden(true)
	} else {
		frame.Role(ui.RoleImage).Label(label)
	}
	length(frame.Width, frame.WidthPercent, p.value("width"))
	length(frame.Height, frame.HeightPercent, p.value("height"))
	var viewBox [4]float32
	if err == nil {
		for i := range viewBox {
			viewBox[i] = float32(values[i])
		}
		ratio := viewBox[2] / viewBox[3]
		if viewBox[2] <= 0 || viewBox[3] <= 0 || ratio <= 0 || math.IsInf(float64(ratio), 0) || math.IsNaN(float64(ratio)) {
			err = errors.New("viewBox is outside the supported coordinate range")
			r.drawingProblem(path, "Svg", err)
		} else if p.value("height") == nil {
			frame.AspectRatio(ratio)
		}
	}
	shapes := r.svgShapes(c, p.value("children"), path, 1)
	frame.Draw(func(painter *ui.Painter, box ui.Rect) {
		if err != nil || box.W <= 0 || box.H <= 0 {
			return
		}
		scale := min(box.W/viewBox[2], box.H/viewBox[3])
		ox := box.X + (box.W-viewBox[2]*scale)/2
		oy := box.Y + (box.H-viewBox[3]*scale)/2
		transform := ui.Translation(-viewBox[0], -viewBox[1]).Then(ui.Scaling(scale, scale)).Then(ui.Translation(ox, oy))
		painter.Clip(box, 0, func() {
			painter.Transform(transform, 0, 0, func() {
				for _, draw := range shapes {
					draw(painter)
				}
			})
		})
	})
}
