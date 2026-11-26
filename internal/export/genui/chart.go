package genui

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

type chartSeries struct {
	key, label string
	values     []float64
}

type chartModel struct {
	kind, format string
	labels       []string
	series       []chartSeries
	xValues      []float64
	stacked      bool
	height       float64
}

func (r *renderer) chart(out *strings.Builder, p props) {
	model, ok := makeChart(p)
	if !ok {
		return
	}
	out.WriteString(`<figure class="g-chart"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 ` + numberString(model.height) + `" role="img" aria-label="` + html.EscapeString(model.kind) + ` chart">`)
	switch model.kind {
	case "pie", "donut":
		pieChart(out, model)
	case "scatter":
		cartesianChart(out, model, true)
	default:
		cartesianChart(out, model, false)
	}
	out.WriteString(`</svg>`)
	chartLegend(out, model)
	out.WriteString(`</figure>`)
}

func makeChart(p props) (chartModel, bool) {
	var rows []*openuilang.Object
	for _, raw := range flatten(p.get("data")) {
		if row, ok := raw.(*openuilang.Object); ok && row != nil {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return chartModel{}, false
	}
	rows = rows[:min(len(rows), 400)]
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
	for _, raw := range flatten(p.get("series")) {
		if name, ok := raw.(string); ok {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		for _, key := range rows[0].Keys {
			if _, ok := number(rows[0].Values[key]); ok && key != category {
				names = append(names, key)
			}
		}
	}
	if category == "" || len(names) == 0 {
		return chartModel{}, false
	}
	names = names[:min(len(names), 12)]
	labelsObject, _ := p.get("seriesLabels").(*openuilang.Object)
	model := chartModel{kind: p.text("type"), format: p.text("format"), stacked: p.on("stacked"), height: min(640, max(160, p.number("height", 260)))}
	switch model.kind {
	case "bar", "line", "area", "pie", "donut", "scatter":
	default:
		model.kind = "bar"
	}
	for _, row := range rows {
		model.labels = append(model.labels, show(row.Values[category]))
	}
	for _, name := range names {
		series := chartSeries{key: name, label: name, values: make([]float64, len(rows))}
		if labelsObject != nil && show(labelsObject.Values[name]) != "" {
			series.label = show(labelsObject.Values[name])
		}
		for i, row := range rows {
			if value, ok := number(row.Values[name]); ok {
				series.values[i] = value
			}
		}
		model.series = append(model.series, series)
	}
	for i, row := range rows {
		value, ok := number(row.Values[category])
		if !ok {
			value = float64(i)
		}
		model.xValues = append(model.xValues, value)
	}
	return model, true
}

func chartColor(index int) string {
	return fmt.Sprintf("var(--chart-%d)", index%6+1)
}

func chartLegend(out *strings.Builder, model chartModel) {
	if model.kind == "pie" || model.kind == "donut" {
		out.WriteString(`<figcaption class="g-chart-legend">`)
		for i, label := range model.labels {
			out.WriteString(`<span><i class="g-legend-swatch" style="background:` + chartColor(i) + `"></i>` + html.EscapeString(label) + `</span>`)
		}
		out.WriteString(`</figcaption>`)
		return
	}
	if len(model.series) < 2 {
		return
	}
	out.WriteString(`<figcaption class="g-chart-legend">`)
	for i, series := range model.series {
		out.WriteString(`<span><i class="g-legend-swatch" style="background:` + chartColor(i) + `"></i>` + html.EscapeString(series.label) + `</span>`)
	}
	out.WriteString(`</figcaption>`)
}

type chartScale struct {
	min, max, left, top, width, height float64
}

func newChartScale(model chartModel) chartScale {
	scale := chartScale{left: 54, top: 16, width: 570, height: model.height - 58}
	for _, series := range model.series {
		for _, value := range series.values {
			scale.min = min(scale.min, value)
			scale.max = max(scale.max, value)
		}
	}
	if model.stacked && model.kind != "line" && len(model.series) > 1 {
		for i := range model.labels {
			positive, negative := 0.0, 0.0
			for _, series := range model.series {
				if series.values[i] >= 0 {
					positive += series.values[i]
				} else {
					negative += series.values[i]
				}
			}
			scale.min = min(scale.min, negative)
			scale.max = max(scale.max, positive)
		}
	}
	if scale.max == scale.min {
		scale.max = scale.min + 1
	}
	padding := (scale.max - scale.min) * 0.06
	scale.max += padding
	if scale.min < 0 {
		scale.min -= padding
	}
	return scale
}

func (s chartScale) y(value float64) float64 {
	return s.top + (s.max-value)/(s.max-s.min)*s.height
}

func (s chartScale) x(value, minimum, maximum float64) float64 {
	if maximum == minimum {
		return s.left + s.width/2
	}
	return s.left + (value-minimum)/(maximum-minimum)*s.width
}

func cartesianChart(out *strings.Builder, model chartModel, scatter bool) {
	scale := newChartScale(model)
	for i := 0; i <= 4; i++ {
		value := scale.min + (scale.max-scale.min)*float64(i)/4
		y := scale.y(value)
		fmt.Fprintf(out, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="var(--line)" stroke-width="1"/>`, numberString(scale.left), numberString(y), numberString(scale.left+scale.width), numberString(y))
		fmt.Fprintf(out, `<text x="%s" y="%s" text-anchor="end" font-size="11" fill="var(--ink-2)">%s</text>`, numberString(scale.left-7), numberString(y+4), html.EscapeString(chartValue(value, model.format)))
	}
	if scatter {
		scatterChart(out, model, scale)
		chartCategoryLabels(out, model, scale)
		return
	}
	zero := scale.y(max(scale.min, min(scale.max, 0)))
	switch model.kind {
	case "bar":
		barChart(out, model, scale, zero)
	case "line", "area":
		lineChart(out, model, scale, zero)
	}
	chartCategoryLabels(out, model, scale)
}

func chartCategoryLabels(out *strings.Builder, model chartModel, scale chartScale) {
	count := len(model.labels)
	if count == 0 {
		return
	}
	step := max(1, int(math.Ceil(float64(count)/10)))
	for i, label := range model.labels {
		if i%step != 0 {
			continue
		}
		x := scale.left + (float64(i)+0.5)*scale.width/float64(count)
		if model.kind == "scatter" {
			minimum, maximum := scatterXRange(model)
			x = scale.x(model.xValues[i], minimum, maximum)
		}
		fmt.Fprintf(out, `<text x="%s" y="%s" text-anchor="middle" font-size="11" fill="var(--ink-2)">%s</text>`, numberString(x), numberString(scale.top+scale.height+22), html.EscapeString(label))
	}
}

func barChart(out *strings.Builder, model chartModel, scale chartScale, zero float64) {
	groups := len(model.labels)
	if groups == 0 {
		return
	}
	groupWidth := scale.width / float64(groups)
	seriesCount := len(model.series)
	if model.stacked {
		seriesCount = 1
	}
	barWidth := max(2, groupWidth*0.72/float64(seriesCount))
	for i := range model.labels {
		positive, negative := 0.0, 0.0
		for j, series := range model.series {
			value := series.values[i]
			x := scale.left + float64(i)*groupWidth + groupWidth*0.14
			if !model.stacked {
				x += float64(j) * barWidth
			}
			y := scale.y(value)
			height := zero - y
			if model.stacked {
				if value >= 0 {
					y = scale.y(positive + value)
					height = scale.y(positive) - y
					positive += value
				} else {
					y = scale.y(negative)
					height = y - scale.y(negative+value)
					negative += value
				}
			}
			if height < 0 {
				y += height
				height = -height
			}
			fmt.Fprintf(out, `<rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s"><title>%s: %s</title></rect>`, numberString(x), numberString(y), numberString(barWidth), numberString(max(0.5, height)), chartColor(j), html.EscapeString(series.label), html.EscapeString(chartValue(value, model.format)))
		}
	}
}

func lineChart(out *strings.Builder, model chartModel, scale chartScale, zero float64) {
	if len(model.labels) == 0 {
		return
	}
	stacked := model.stacked && model.kind == "area"
	positive := make([]float64, len(model.labels))
	negative := make([]float64, len(model.labels))
	for j, series := range model.series {
		var points, baseline strings.Builder
		type point struct{ x, y float64 }
		bases := make([]point, len(series.values))
		uppers := make([]point, len(series.values))
		for i, value := range series.values {
			lower, upper := 0.0, value
			if stacked {
				if value >= 0 {
					lower = positive[i]
					positive[i] += value
					upper = positive[i]
				} else {
					lower = negative[i]
					negative[i] += value
					upper = negative[i]
				}
			}
			x := scale.left + (float64(i)+0.5)*scale.width/float64(len(model.labels))
			bases[i] = point{x, scale.y(lower)}
			uppers[i] = point{x, scale.y(upper)}
			if i > 0 {
				points.WriteByte(' ')
			}
			points.WriteString(numberString(x) + "," + numberString(scale.y(upper)))
		}
		if model.kind == "area" {
			for i := len(bases) - 1; i >= 0; i-- {
				fmt.Fprintf(&baseline, " %s,%s", numberString(bases[i].x), numberString(bases[i].y))
			}
			if !stacked {
				firstX := scale.left + 0.5*scale.width/float64(len(model.labels))
				lastX := scale.left + (float64(len(model.labels))-0.5)*scale.width/float64(len(model.labels))
				baseline.Reset()
				fmt.Fprintf(&baseline, " %s,%s %s,%s", numberString(lastX), numberString(zero), numberString(firstX), numberString(zero))
			}
			fmt.Fprintf(out, `<polygon points="%s%s" fill="%s" opacity=".16"/>`, points.String(), baseline.String(), chartColor(j))
		}
		fmt.Fprintf(out, `<polyline points="%s" fill="none" stroke="%s" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round"/>`, points.String(), chartColor(j))
		for i, value := range series.values {
			x := scale.left + (float64(i)+0.5)*scale.width/float64(len(model.labels))
			y := uppers[i].y
			fmt.Fprintf(out, `<circle cx="%s" cy="%s" r="3" fill="%s"><title>%s: %s</title></circle>`, numberString(x), numberString(y), chartColor(j), html.EscapeString(series.label), html.EscapeString(chartValue(value, model.format)))
		}
	}
}

func scatterXRange(model chartModel) (float64, float64) {
	if len(model.xValues) == 0 {
		return 0, 1
	}
	minimum, maximum := model.xValues[0], model.xValues[0]
	for _, value := range model.xValues {
		minimum = min(minimum, value)
		maximum = max(maximum, value)
	}
	return minimum, maximum
}

func scatterChart(out *strings.Builder, model chartModel, scale chartScale) {
	minimum, maximum := scatterXRange(model)
	for j, series := range model.series {
		for i, value := range series.values {
			x := scale.x(model.xValues[i], minimum, maximum)
			y := scale.y(value)
			fmt.Fprintf(out, `<circle cx="%s" cy="%s" r="4" fill="%s" opacity=".85"><title>%s: %s</title></circle>`, numberString(x), numberString(y), chartColor(j), html.EscapeString(series.label), html.EscapeString(chartValue(value, model.format)))
		}
	}
}

func pieChart(out *strings.Builder, model chartModel) {
	if len(model.series) == 0 {
		return
	}
	values := model.series[0].values
	total := 0.0
	for _, value := range values {
		total += max(0, value)
	}
	if total == 0 {
		return
	}
	cx, cy, radius := 320.0, model.height/2, min(100, model.height/2-16)
	start := -math.Pi / 2
	for i, value := range values {
		if value <= 0 {
			continue
		}
		angle := value / total * 2 * math.Pi
		formatted := chartValue(value, model.format)
		if model.format == "percent" {
			formatted = chartValue(value/total, model.format)
		}
		if angle >= 2*math.Pi-1e-9 {
			fmt.Fprintf(out, `<circle cx="%s" cy="%s" r="%s" fill="%s"><title>%s: %s</title></circle>`, numberString(cx), numberString(cy), numberString(radius), chartColor(i), html.EscapeString(model.labels[i]), html.EscapeString(formatted))
			continue
		}
		end := start + angle
		x1, y1 := cx+radius*math.Cos(start), cy+radius*math.Sin(start)
		x2, y2 := cx+radius*math.Cos(end), cy+radius*math.Sin(end)
		large := 0
		if angle > math.Pi {
			large = 1
		}
		fmt.Fprintf(out, `<path d="M %s %s L %s %s A %s %s 0 %d 1 %s %s Z" fill="%s"><title>%s: %s</title></path>`, numberString(cx), numberString(cy), numberString(x1), numberString(y1), numberString(radius), numberString(radius), large, numberString(x2), numberString(y2), chartColor(i), html.EscapeString(model.labels[i]), html.EscapeString(formatted))
		start = end
	}
	if model.kind == "donut" {
		fmt.Fprintf(out, `<circle cx="%s" cy="%s" r="%s" fill="var(--paper)"/>`, numberString(cx), numberString(cy), numberString(radius*0.56))
	}
}

func chartValue(value float64, format string) string {
	if format == "percent" {
		return strconv.FormatFloat(value*100, 'f', -1, 64) + "%"
	}
	return strconv.FormatFloat(math.Round(value*100)/100, 'f', -1, 64)
}
