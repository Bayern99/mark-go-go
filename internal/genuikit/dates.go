package genuikit

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

const dayLength = 24 * time.Hour

var (
	dateOnly    = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
	dateLeading = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)
	zoned       = []string{time.RFC3339Nano, "2006-01-02T15:04Z07:00", time.RFC1123Z, time.RFC1123}
	local       = []string{
		"2006-01-02T15:04:05.999999999", "2006-01-02T15:04", "2006-01-02 15:04:05.999999999", "2006-01-02 15:04",
		"January 2, 2006 15:04:05", "January 2, 2006", "Jan 2, 2006 15:04:05", "Jan 2, 2006", "2 January 2006", "2 Jan 2006", "1/2/2006", "2006/01/02",
	}
)

type moment struct {
	day     int
	instant *time.Time
}

func dayOf(year int, month time.Month, day int) int {
	return int(time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix() / int64(dayLength/time.Second))
}

func calendar(day int) time.Time { return time.Unix(int64(day)*int64(dayLength/time.Second), 0).UTC() }

func isoDay(day int) string { return calendar(day).Format("2006-01-02") }

func (env Environment) parse(value any) (moment, error) {
	at := func(instant time.Time) (moment, error) {
		instant = instant.In(env.Location)
		return moment{dayOf(instant.Date()), &instant}, nil
	}
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.Abs(v) > 8.64e15 {
			break
		}
		return at(time.UnixMilli(int64(v)))
	case string:
		trimmed := strings.TrimSpace(v)
		if dateOnly.MatchString(trimmed) {
			day, err := time.Parse("2006-01-02", trimmed)
			if err != nil {
				break
			}
			return moment{day: dayOf(day.Date())}, nil
		}
		for _, layout := range zoned {
			if instant, err := time.Parse(layout, trimmed); err == nil {
				return at(instant)
			}
		}
		for _, layout := range local {
			if instant, err := time.ParseInLocation(layout, trimmed, env.Location); err == nil {
				return at(instant)
			}
		}
	}
	return moment{}, fmt.Errorf("invalid date %q", text(value))
}

var dateStyles = map[string]string{
	"date": "Jan 2, 2006", "short": "Jan 2", "long": "Monday, January 2, 2006", "weekday": "Mon",
	"month": "Jan 2006", "time": "3:04 PM", "datetime": "Jan 2, 3:04 PM",
}

func (env Environment) dateFunctions() map[string]function {
	day := func(a arguments, name string) (int, error) {
		parsed, err := env.parse(a.any(name))
		return parsed.day, err
	}
	return map[string]function{
		"Now": func(arguments) (any, error) {
			return env.Now().UTC().Format("2006-01-02T15:04:05.000Z"), nil
		},
		"Today": func(arguments) (any, error) {
			return isoDay(dayOf(env.Now().In(env.Location).Date())), nil
		},
		"FormatDate": func(a arguments) (any, error) {
			parsed, err := env.parse(a.any("date"))
			if err != nil {
				return nil, err
			}
			style := a.text("style", "date")
			if style == "iso" {
				return isoDay(parsed.day), nil
			}
			if parsed.instant != nil {
				return parsed.instant.Format(dateStyles[style]), nil
			}
			return calendar(parsed.day).Format(dateStyles[style]), nil
		},
		"AddDays": func(a arguments) (any, error) {
			from, err := day(a, "date")
			if err != nil {
				return nil, err
			}
			return isoDay(from + whole(a.number("days"))), nil
		},
		"DaysBetween": func(a arguments) (any, error) {
			from, err := day(a, "from")
			if err != nil {
				return nil, err
			}
			to, err := day(a, "to")
			return float64(to - from), err
		},
		"AddMonths": func(a arguments) (any, error) {
			from, err := day(a, "date")
			if err != nil {
				return nil, err
			}
			year, month, date := calendar(from).Date()
			target := int(month) + whole(a.number("months"))
			last := time.Date(year, time.Month(target+1), 0, 0, 0, 0, 0, time.UTC).Day()
			return isoDay(dayOf(year, time.Month(target), min(date, last))), nil
		},
		"MonthsBetween": func(a arguments) (any, error) {
			from, err := day(a, "from")
			if err != nil {
				return nil, err
			}
			to, err := day(a, "to")
			if err != nil {
				return nil, err
			}
			y1, m1, d1 := calendar(from).Date()
			y2, m2, d2 := calendar(to).Date()
			months := (y2-y1)*12 + int(m2) - int(m1)
			if months > 0 && d2 < d1 {
				months--
			}
			if months < 0 && d2 > d1 {
				months++
			}
			return float64(months), nil
		},
		"StartOf": func(a arguments) (any, error) {
			from, err := day(a, "date")
			if err != nil {
				return nil, err
			}
			return isoDay(periodStart(from, a.text("unit", ""))), nil
		},
		"Between": func(a arguments) (any, error) {
			out := []any{}
			for _, row := range a.list("rows") {
				value := read(row, a.text("field", ""))
				if value == nil {
					continue
				}
				if (!a.has("min") || env.compare(value, a.any("min")) >= 0) && (!a.has("max") || env.compare(value, a.any("max")) <= 0) {
					out = append(out, row)
				}
			}
			return out, nil
		},
		"Bucket": func(a arguments) (any, error) {
			unit, field := a.text("unit", "day"), a.text("field", "")
			periods := map[int][]any{}
			var starts []int
			for _, row := range a.list("rows") {
				raw := read(row, a.text("dateField", ""))
				switch raw.(type) {
				case string, float64:
				default:
					continue
				}
				parsed, err := env.parse(raw)
				if err != nil {
					continue
				}
				start := periodStart(parsed.day, unit)
				if _, seen := periods[start]; !seen {
					starts = append(starts, start)
				}
				periods[start] = append(periods[start], row)
			}
			out := []any{}
			if len(starts) == 0 {
				return out, nil
			}
			sort.Ints(starts)
			how := defaultAggregate(a)
			label := func(start int) string {
				if unit == "month" {
					return isoDay(start)[:7]
				}
				return isoDay(start)
			}
			for start := starts[0]; start <= starts[len(starts)-1]; start = nextPeriod(start, unit) {
				if members, ok := periods[start]; ok {
					out = append(out, object("key", label(start), "value", aggregate(members, field, how)))
				} else if how == "sum" || how == "count" {
					out = append(out, object("key", label(start), "value", 0.0))
				}
				if len(out) > maxItems {
					return nil, fmt.Errorf("more than %d periods", maxItems)
				}
			}
			return out, nil
		},
	}
}

func periodStart(day int, unit string) int {
	at := calendar(day)
	switch unit {
	case "week":
		return day - (int(at.Weekday())+6)%7
	case "month":
		return dayOf(at.Year(), at.Month(), 1)
	case "year":
		return dayOf(at.Year(), time.January, 1)
	}
	return day
}

func nextPeriod(start int, unit string) int {
	switch unit {
	case "week":
		return start + 7
	case "month":
		at := calendar(start)
		return dayOf(at.Year(), at.Month()+1, 1)
	}
	return start + 1
}

func (env Environment) compare(a, b any) int {
	x, xNumber := toNumber(a)
	y, yNumber := toNumber(b)
	sign := func(difference float64) int {
		switch {
		case difference < 0:
			return -1
		case difference > 0:
			return 1
		}
		return 0
	}
	if xNumber && yNumber {
		return sign(x - y)
	}
	dated := func(value any) bool {
		s, ok := value.(string)
		return ok && dateLeading.MatchString(strings.TrimSpace(s))
	}
	if (dated(a) || dated(b)) && (dated(a) || xNumber) && (dated(b) || yNumber) {
		if xNumber {
			a = x
		}
		if yNumber {
			b = y
		}
		first, err := env.parse(a)
		second, also := env.parse(b)
		if err == nil && also == nil {
			return sign(float64(first.day - second.day))
		}
	}
	return strings.Compare(text(a), text(b))
}

var dataFunctions = map[string]function{
	"GroupBy": func(a arguments) (any, error) {
		out := []any{}
		for _, group := range groups(a.list("rows"), a.text("key", "")) {
			out = append(out, object("key", group.key, "value", aggregate(group.rows, a.text("field", ""), defaultAggregate(a))))
		}
		return out, nil
	},
	"Pivot": func(a arguments) (any, error) {
		rows, column := a.list("rows"), a.text("colKey", "")
		out := []any{}
		for _, group := range groups(rows, a.text("rowKey", "")) {
			row := object("key", group.key)
			for _, name := range groups(rows, column) {
				row.Set(text(name.key), 0.0)
			}
			for _, cell := range groups(group.rows, column) {
				row.Set(text(cell.key), aggregate(cell.rows, a.text("field", ""), defaultAggregate(a)))
			}
			out = append(out, row)
		}
		return out, nil
	},
	"Top": func(a arguments) (any, error) {
		rows := a.list("rows")
		type ranked struct {
			row   any
			value float64
			known bool
		}
		order := make([]ranked, len(rows))
		for i, row := range rows {
			value, known := toNumber(read(row, a.text("field", "")))
			order[i] = ranked{row, value, known}
		}
		ascending := a.text("direction", "desc") == "asc"
		sort.SliceStable(order, func(i, j int) bool {
			if !order[i].known || !order[j].known {
				return order[i].known && !order[j].known
			}
			if ascending {
				return order[i].value < order[j].value
			}
			return order[i].value > order[j].value
		})
		out := []any{}
		for _, entry := range order[:min(len(order), max(0, whole(a.number("n"))))] {
			out = append(out, entry.row)
		}
		return out, nil
	},
	"Cumulative": func(a arguments) (any, error) {
		total := 0.0
		return mapField(a, func(values []float64, i int) float64 {
			total += values[i]
			return total
		}), nil
	},
	"MovingAvg": func(a arguments) (any, error) {
		window := whole(a.number("window"))
		if window < 1 {
			return nil, fmt.Errorf("window must be at least 1")
		}
		return mapField(a, func(values []float64, i int) float64 {
			from, sum := max(0, i-window+1), 0.0
			for _, value := range values[from : i+1] {
				sum += value
			}
			return sum / float64(i+1-from)
		}), nil
	},
	"Change": func(a arguments) (any, error) {
		if a.number("previous") == 0 {
			return nil, nil
		}
		return finite((a.number("current") - a.number("previous")) / math.Abs(a.number("previous")) * 100)
	},
	"Histogram": func(a arguments) (any, error) {
		values := finiteOnly(a.list("values"))
		bins := whole(a.number("bins"))
		if bins < 1 || bins > 500 {
			return nil, fmt.Errorf("bins must be from 1 to 500")
		}
		low, high := 0.0, 1.0
		if len(values) > 0 {
			low, high = values[0], values[0]
			for _, value := range values {
				low, high = min(low, value), max(high, value)
			}
		}
		if a.has("min") {
			low = a.number("min")
		}
		if a.has("max") {
			high = a.number("max")
		}
		if high <= low {
			high = low + 1
		}
		width := (high - low) / float64(bins)
		places := min(100, max(0, 1-int(math.Floor(math.Log10(width)))))
		counts := make([]float64, bins)
		for _, value := range values {
			if value >= low && value <= high {
				counts[min(bins-1, int(math.Floor((value-low)/width)))]++
			}
		}
		out := make([]any, bins)
		for i, count := range counts {
			scale := math.Pow(10, float64(places))
			out[i] = object("key", numeral(math.Round((low+float64(i)*width)*scale)/scale), "value", count)
		}
		return out, nil
	},
	"Stats": func(a arguments) (any, error) {
		values := finiteOnly(a.list("values"))
		count := len(values)
		if count == 0 {
			return object("count", 0.0, "min", 0.0, "max", 0.0, "mean", 0.0, "median", 0.0, "stdev", 0.0), nil
		}
		sort.Float64s(values)
		sum := 0.0
		for _, value := range values {
			sum += value
		}
		mean := sum / float64(count)
		spread := 0.0
		for _, value := range values {
			spread += (value - mean) * (value - mean)
		}
		median := values[count/2]
		if count%2 == 0 {
			median = (values[count/2-1] + values[count/2]) / 2
		}
		return object("count", float64(count), "min", values[0], "max", values[count-1], "mean", mean, "median", median, "stdev", math.Sqrt(spread/float64(count))), nil
	},
}

func finiteOnly(list []any) []float64 {
	var out []float64
	for _, item := range list {
		if value, ok := item.(float64); ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
			out = append(out, value)
		}
	}
	return out
}

func mapField(a arguments, next func(values []float64, i int) float64) []any {
	rows, field := a.list("rows"), a.text("field", "")
	values := make([]float64, len(rows))
	for i, row := range rows {
		values[i], _ = toNumber(read(row, field))
	}
	out := make([]any, len(rows))
	for i, row := range rows {
		if value := next(values, i); field == "" {
			out[i] = value
		} else {
			out[i] = withField(row, field, value)
		}
	}
	return out
}
