package genuikit

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

type function func(a arguments) (any, error)

func (env Environment) register(library *openuilang.Library) {
	add := func(table map[string]function, register func(string, openuilang.Function)) {
		for name, fn := range table {
			register(name, func(args *openuilang.Object) (any, error) { return fn(arguments{args}) })
		}
	}
	add(listFunctions, library.RegisterFunction)
	add(numberFunctions, library.RegisterFunction)
	add(textFunctions, library.RegisterFunction)
	add(dataFunctions, library.RegisterFunction)
	add(env.dateFunctions(), library.RegisterFunction)
	add(env.randomFunctions(), library.RegisterActionFunction)
}

func whole(value float64) int {
	if math.IsNaN(value) {
		return 0
	}
	return int(math.Trunc(max(min(value, 1e9), -1e9)))
}

func span(length int, start float64, end any) (int, int) {
	bound := func(value float64) int {
		at := whole(value)
		if at < 0 {
			at += length
		}
		return max(0, min(at, length))
	}
	from, to := bound(start), length
	if value, ok := end.(float64); ok {
		to = bound(value)
	}
	return from, max(from, to)
}

func tooMany(count int) error {
	if count > maxItems {
		return fmt.Errorf("more than %d items", maxItems)
	}
	return nil
}

var listFunctions = map[string]function{
	"Append": func(a arguments) (any, error) {
		list := a.list("list")
		return append(append(make([]any, 0, len(list)+1), list...), a.any("item")), tooMany(len(list) + 1)
	},
	"Prepend": func(a arguments) (any, error) {
		list := a.list("list")
		return append([]any{a.any("item")}, list...), tooMany(len(list) + 1)
	},
	"Concat": func(a arguments) (any, error) {
		list, more := a.list("list"), a.list("more")
		return append(append(make([]any, 0, len(list)+len(more)), list...), more...), tooMany(len(list) + len(more))
	},
	"Remove": func(a arguments) (any, error) {
		out := []any{}
		for _, item := range a.list("list") {
			if !same(read(item, a.text("field", "")), a.any("value")) {
				out = append(out, item)
			}
		}
		return out, nil
	},
	"RemoveAt": func(a arguments) (any, error) {
		list := a.list("list")
		at := whole(a.number("index"))
		if at < 0 {
			at += len(list)
		}
		out := []any{}
		for i, item := range list {
			if i != at {
				out = append(out, item)
			}
		}
		return out, nil
	},
	"Update": func(a arguments) (any, error) {
		out := []any{}
		for _, item := range a.list("list") {
			if _, ok := item.(*openuilang.Object); ok && same(read(item, a.text("field", "")), a.any("value")) {
				item = withField(item, a.text("setField", ""), a.any("setValue"))
			}
			out = append(out, item)
		}
		return out, nil
	},
	"Toggle": func(a arguments) (any, error) {
		out, found := []any{}, false
		for _, item := range a.list("list") {
			if same(item, a.any("item")) {
				found = true
				continue
			}
			out = append(out, item)
		}
		if !found {
			out = append(out, a.any("item"))
		}
		return out, nil
	},
	"Includes": func(a arguments) (any, error) {
		if whole, ok := a.any("list").(string); ok {
			return a.has("item") && strings.Contains(whole, text(a.any("item"))), nil
		}
		for _, item := range a.list("list") {
			if same(item, a.any("item")) {
				return true, nil
			}
		}
		return false, nil
	},
	"Find": func(a arguments) (any, error) {
		for _, item := range a.list("list") {
			if same(read(item, a.text("field", "")), a.any("value")) {
				return item, nil
			}
		}
		return nil, nil
	},
	"IndexOf": func(a arguments) (any, error) {
		for i, item := range a.list("list") {
			if same(item, a.any("item")) {
				return float64(i), nil
			}
		}
		return float64(-1), nil
	},
	"Take": func(a arguments) (any, error) {
		list := a.list("list")
		count := whole(a.number("n"))
		if count >= 0 {
			return append([]any{}, list[:min(count, len(list))]...), nil
		}
		return append([]any{}, list[max(0, len(list)+count):]...), nil
	},
	"Slice": func(a arguments) (any, error) {
		if whole, ok := a.any("list").(string); ok {
			units := utf16.Encode([]rune(whole))
			from, to := span(len(units), a.number("start"), a.any("end"))
			return string(utf16.Decode(units[from:to])), nil
		}
		list := a.list("list")
		from, to := span(len(list), a.number("start"), a.any("end"))
		return append([]any{}, list[from:to]...), nil
	},
	"Unique": func(a arguments) (any, error) {
		out := []any{}
		seen := map[string]bool{}
		var kept []any
	items:
		for _, item := range a.list("list") {
			switch item.(type) {
			case []any, *openuilang.Object, *openuilang.Element:
				for _, other := range kept {
					if same(other, item) {
						continue items
					}
				}
				kept = append(kept, item)
			default:
				if seen[text(item)] {
					continue
				}
				seen[text(item)] = true
			}
			out = append(out, item)
		}
		return out, nil
	},
	"Reverse": func(a arguments) (any, error) {
		list := a.list("list")
		out := make([]any, len(list))
		for i, item := range list {
			out[len(list)-1-i] = item
		}
		return out, nil
	},
	"Range": func(a arguments) (any, error) {
		start, end, step := a.number("start"), a.number("end"), 1.0
		if a.has("step") {
			step = a.number("step")
		}
		if step == 0 || math.IsNaN(step) || math.IsInf(step, 0) {
			return nil, errors.New("step must be a non-zero number")
		}
		steps, _ := strconv.ParseFloat(strconv.FormatFloat((end-start)/step, 'g', 12, 64), 64)
		count := max(0, math.Ceil(steps))
		if math.IsNaN(count) || count > maxItems {
			return nil, tooMany(maxItems + 1)
		}
		exact := start == math.Trunc(start) && step == math.Trunc(step)
		out := make([]any, int(count))
		for i := range out {
			value := start + float64(i)*step
			if !exact {
				value, _ = strconv.ParseFloat(strconv.FormatFloat(value, 'g', 12, 64), 64)
			}
			out[i] = value
		}
		return out, nil
	},
	"Flatten": func(a arguments) (any, error) {
		out := []any{}
		for _, item := range a.list("list") {
			if inner, ok := item.([]any); ok {
				out = append(out, inner...)
			} else {
				out = append(out, item)
			}
		}
		return out, tooMany(len(out))
	},
	"Chunk": func(a arguments) (any, error) {
		list := a.list("list")
		size := whole(a.number("size"))
		if size < 1 {
			return nil, errors.New("size must be 1 or more")
		}
		out := []any{}
		for i := 0; i < len(list); i += size {
			out = append(out, append([]any{}, list[i:min(i+size, len(list))]...))
		}
		return out, nil
	},
}

func (env Environment) randomFunctions() map[string]function {
	return map[string]function{
		"Random": func(a arguments) (any, error) {
			low, high := min(a.number("min"), a.number("max")), max(a.number("min"), a.number("max"))
			if math.IsNaN(low) || math.IsInf(low, 0) || math.IsInf(high, 0) {
				return nil, errors.New("min and max must be numbers")
			}
			if !a.flag("integer") {
				return low + env.Random()*(high-low), nil
			}
			from, to := math.Ceil(low), math.Floor(high)
			if from > to {
				return nil, errors.New("no whole number between min and max")
			}
			return from + math.Floor(env.Random()*(to-from+1)), nil
		},
		"Pick": func(a arguments) (any, error) {
			list := a.list("list")
			if len(list) == 0 {
				return nil, nil
			}
			return list[min(int(env.Random()*float64(len(list))), len(list)-1)], nil
		},
		"Shuffle": func(a arguments) (any, error) {
			out := append([]any{}, a.list("list")...)
			for i := len(out) - 1; i > 0; i-- {
				j := min(int(env.Random()*float64(i+1)), i)
				out[i], out[j] = out[j], out[i]
			}
			return out, nil
		},
	}
}

var units = map[string]struct {
	kind   string
	factor float64
}{
	"mm": {"length", 0.001}, "cm": {"length", 0.01}, "m": {"length", 1}, "km": {"length", 1000},
	"in": {"length", 0.0254}, "ft": {"length", 0.3048}, "yd": {"length", 0.9144}, "mi": {"length", 1609.344},
	"mg": {"mass", 1e-6}, "g": {"mass", 0.001}, "kg": {"mass", 1}, "oz": {"mass", 0.028349523125},
	"lb": {"mass", 0.45359237}, "st": {"mass", 6.35029318},
	"ml": {"volume", 0.001}, "l": {"volume", 1}, "tsp": {"volume", 0.00492892159375}, "tbsp": {"volume", 0.0147867647812},
	"floz": {"volume", 0.0295735295625}, "cup": {"volume", 0.2365882365}, "pt": {"volume", 0.473176473},
	"qt": {"volume", 0.946352946}, "gal": {"volume", 3.785411784},
	"m/s": {"speed", 1}, "km/h": {"speed", 1 / 3.6}, "mph": {"speed", 0.44704}, "kn": {"speed", 0.514444},
	"m2": {"area", 1}, "km2": {"area", 1e6}, "ft2": {"area", 0.09290304}, "ha": {"area", 10000}, "acre": {"area", 4046.8564224},
}

var numberFunctions = map[string]function{
	"Pow": func(a arguments) (any, error) { return finite(math.Pow(a.number("base"), a.number("exponent"))) },
	"Sqrt": func(a arguments) (any, error) {
		if a.number("number") < 0 {
			return nil, nil
		}
		return finite(math.Sqrt(a.number("number")))
	},
	"Clamp": func(a arguments) (any, error) {
		low, high := min(a.number("min"), a.number("max")), max(a.number("min"), a.number("max"))
		return finite(min(max(a.number("number"), low), high))
	},
	"Percent": func(a arguments) (any, error) {
		if a.number("total") == 0 {
			return 0.0, nil
		}
		return finite(a.number("part") / a.number("total") * 100)
	},
	"Lerp": func(a arguments) (any, error) {
		return finite(a.number("a") + (a.number("b")-a.number("a"))*a.number("t"))
	},
	"Allocate": func(a arguments) (any, error) {
		weights := a.list("weights")
		sum := 0.0
		for _, weight := range weights {
			value, numeric := weight.(float64)
			if !numeric || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, errors.New("weights must be zero or more")
			}
			sum += value
		}
		if len(weights) == 0 {
			return nil, errors.New("weights must be zero or more")
		}
		out := make([]any, len(weights))
		if sum == 0 {
			for i := range out {
				out[i] = 0.0
			}
			return out, nil
		}
		places := 2.0
		if a.has("decimals") {
			places = a.number("decimals")
		}
		scale := math.Pow(10, min(6, max(0, math.Round(places))))
		total := a.number("total")
		parts := make([]float64, len(weights))
		rest := make([]float64, len(weights))
		order := make([]int, len(weights))
		left := math.Round(math.Abs(total) * scale)
		all := left
		for i, weight := range weights {
			exact := all * weight.(float64) / sum
			parts[i], order[i] = math.Floor(exact), i
			rest[i] = exact - parts[i]
			left -= parts[i]
		}
		sort.SliceStable(order, func(i, j int) bool { return rest[order[i]] > rest[order[j]] })
		for _, i := range order {
			if left <= 0 {
				break
			}
			parts[i]++
			left--
		}
		for i, part := range parts {
			if total < 0 {
				part = -part
			}
			out[i] = part/scale + 0
		}
		return out, nil
	},
	"Payment": func(a arguments) (any, error) {
		periods, rate := a.number("periods"), a.number("rate")/100
		if !(periods > 0) {
			return nil, errors.New("periods must be positive")
		}
		if rate == 0 {
			return finite(a.number("principal") / periods)
		}
		return finite(a.number("principal") * rate / (1 - math.Pow(1+rate, -periods)))
	},
	"Convert": func(a arguments) (any, error) {
		from, to := strings.ToLower(strings.TrimSpace(a.text("from", ""))), strings.ToLower(strings.TrimSpace(a.text("to", "")))
		value := a.number("value")
		if strings.Contains("cfk", from) && strings.Contains("cfk", to) && len(from) == 1 && len(to) == 1 {
			switch from {
			case "f":
				value = (value - 32) * 5 / 9
			case "k":
				value -= 273.15
			}
			switch to {
			case "f":
				value = value*9/5 + 32
			case "k":
				value += 273.15
			}
			return finite(value)
		}
		source, known := units[from]
		target, also := units[to]
		if !known || !also || source.kind != target.kind {
			return nil, fmt.Errorf("cannot convert %s to %s", a.text("from", ""), a.text("to", ""))
		}
		return finite(value * source.factor / target.factor)
	},
}

func digits(value float64, least, most int) string {
	out := halfUp(strconv.FormatFloat(math.Abs(value), 'f', -1, 64), most)
	if most > least {
		out = strings.TrimRight(out, "0")
		whole, fraction, _ := strings.Cut(out, ".")
		for len(fraction) < least {
			fraction += "0"
		}
		out = whole
		if fraction != "" {
			out += "." + fraction
		}
	}
	whole, fraction, cut := strings.Cut(out, ".")
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "," + whole[i:]
	}
	if cut {
		whole += "." + fraction
	}
	return whole
}

func signed(value float64, body string) string {
	if value < 0 && strings.Trim(body, "0.,") != "" {
		return "-" + body
	}
	return body
}

var currencySymbols = map[string]string{
	"USD": "$", "EUR": "€", "GBP": "£", "JPY": "¥", "INR": "₹", "CNY": "CN¥", "KRW": "₩", "CAD": "CA$", "AUD": "A$",
	"NZD": "NZ$", "MXN": "MX$", "BRL": "R$", "HKD": "HK$", "TWD": "NT$", "ILS": "₪", "VND": "₫", "PHP": "₱",
}

var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

func currencySymbol(code string) string {
	code = strings.ToUpper(code)
	if !currencyCode.MatchString(code) {
		code = "USD"
	}
	if symbol, ok := currencySymbols[code]; ok {
		return symbol
	}
	return code + "\u00a0"
}

func compactNumber(value float64, least, most int) (string, string) {
	magnitude := math.Abs(value)
	steps := []struct {
		size   float64
		suffix string
	}{{1e12, "T"}, {1e9, "B"}, {1e6, "M"}, {1e3, "K"}}
	for i, step := range steps {
		if magnitude < step.size {
			continue
		}
		body := digits(magnitude/step.size, least, most)
		if i > 0 && strings.HasPrefix(body, "1,000") {
			return digits(magnitude/steps[i-1].size, least, most), steps[i-1].suffix
		}
		return body, step.suffix
	}
	body := digits(magnitude, least, most)
	if strings.HasPrefix(body, "1,000") {
		return digits(magnitude/1e3, least, most), "K"
	}
	return body, ""
}

var wordStart = regexp.MustCompile(`(^|[\s\-(/"])(\p{L})`)

var textFunctions = map[string]function{
	"Format": func(a arguments) (any, error) {
		value := a.number("value")
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, errors.New("value is not a finite number")
		}
		places := -1
		if a.has("decimals") {
			places = min(max(whole(a.number("decimals")), 0), 10)
		}
		bounds := func(most, least int) (int, int) {
			if places < 0 {
				return least, most
			}
			if least > 0 {
				return places, places
			}
			return 0, places
		}
		switch a.text("kind", "number") {
		case "currency":
			least, most := bounds(2, 2)
			return signed(value, currencySymbol(a.text("currency", "USD"))+digits(value, least, most)), nil
		case "percent":
			least, most := bounds(1, 0)
			return signed(value, digits(value, least, most)) + "%", nil
		case "compact":
			least, most := bounds(1, 0)
			body, suffix := compactNumber(value, least, most)
			if a.has("currency") && a.text("currency", "") != "" {
				body = currencySymbol(a.text("currency", "")) + body
			}
			return signed(value, body) + suffix, nil
		}
		least, most := bounds(2, 0)
		return signed(value, digits(value, least, most)), nil
	},
	"Join": func(a arguments) (any, error) {
		var parts []string
		for _, item := range a.list("list") {
			switch value := read(item, a.text("field", "")).(type) {
			case nil:
			case []any, *openuilang.Object:
				parts = append(parts, encoded(value))
			default:
				parts = append(parts, text(value))
			}
		}
		return strings.Join(parts, a.text("sep", ", ")), nil
	},
	"Plural": func(a arguments) (any, error) {
		count, word := a.number("n"), a.text("word", "")
		if math.Abs(count) != 1 {
			word = a.text("plural", word+"s")
		}
		return signed(count, digits(count, 0, 2)) + " " + word, nil
	},
	"Pad": func(a arguments) (any, error) {
		value := text(a.any("value"))
		fill := a.text("char", "0")
		if fill == "" {
			fill = "0"
		}
		length := min(whole(a.number("length")), 1000)
		units, padding := utf16.Encode([]rune(value)), utf16.Encode([]rune(fill))
		var lead []uint16
		for len(units)+len(lead) < length {
			lead = append(lead, padding[len(lead)%len(padding)])
		}
		return string(utf16.Decode(append(lead, units...))), nil
	},
	"Upper": func(a arguments) (any, error) { return strings.ToUpper(a.text("text", "")), nil },
	"Lower": func(a arguments) (any, error) { return strings.ToLower(a.text("text", "")), nil },
	"Trim":  func(a arguments) (any, error) { return strings.TrimFunc(a.text("text", ""), blank), nil },
	"Capitalize": func(a arguments) (any, error) {
		return wordStart.ReplaceAllStringFunc(a.text("text", ""), func(match string) string {
			runes := []rune(match)
			runes[len(runes)-1] = unicode.ToUpper(runes[len(runes)-1])
			return string(runes)
		}), nil
	},
	"Split": func(a arguments) (any, error) {
		out := []any{}
		for _, part := range strings.Split(a.text("text", ""), a.text("sep", ",")) {
			if part = strings.TrimFunc(part, blank); part != "" {
				out = append(out, part)
			}
		}
		return out, tooMany(len(out))
	},
	"Replace": func(a arguments) (any, error) {
		if a.text("find", "") == "" {
			return a.text("text", ""), nil
		}
		return strings.ReplaceAll(a.text("text", ""), a.text("find", ""), a.text("replacement", "")), nil
	},
	"Duration": func(a arguments) (any, error) {
		seconds := a.number("seconds")
		if math.IsNaN(seconds) || math.IsInf(seconds, 0) || math.Abs(seconds) > 1e15 {
			return nil, errors.New("seconds is not a finite number")
		}
		sign := ""
		if seconds < 0 {
			sign = "-"
		}
		total := int64(math.Round(math.Abs(seconds)))
		h, m, s := total/3600, total%3600/60, total%60
		switch {
		case a.text("style", "short") == "clock" && h > 0:
			return fmt.Sprintf("%s%d:%02d:%02d", sign, h, m, s), nil
		case a.text("style", "short") == "clock":
			return fmt.Sprintf("%s%d:%02d", sign, m, s), nil
		case h > 0 && m > 0:
			return fmt.Sprintf("%s%dh %dm", sign, h, m), nil
		case h > 0:
			return fmt.Sprintf("%s%dh", sign, h), nil
		case m > 0 && s > 0:
			return fmt.Sprintf("%s%dm %02ds", sign, m, s), nil
		case m > 0:
			return fmt.Sprintf("%s%dm", sign, m), nil
		}
		return fmt.Sprintf("%s%ds", sign, s), nil
	},
	"Default": func(a arguments) (any, error) {
		value := a.any("value")
		if number, ok := value.(float64); value == nil || value == "" || ok && math.IsNaN(number) {
			return a.any("fallback"), nil
		}
		return value, nil
	},
	"MixColor": func(a arguments) (any, error) {
		from, err := channels(a.text("from", ""))
		if err != nil {
			return nil, err
		}
		to, err := channels(a.text("to", ""))
		if err != nil {
			return nil, err
		}
		share := min(1, max(0, a.number("t")))
		out := "#"
		for i := range from {
			out += fmt.Sprintf("%02x", int(math.Floor(from[i]+(to[i]-from[i])*share+0.5)))
		}
		return out, nil
	},
}

func blank(r rune) bool { return r == '\ufeff' || r != '\u0085' && unicode.IsSpace(r) }

var hexColor = regexp.MustCompile(`^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

func channels(color string) ([3]float64, error) {
	match := hexColor.FindStringSubmatch(strings.TrimFunc(color, blank))
	if match == nil {
		return [3]float64{}, fmt.Errorf("not a hex color: %s", color)
	}
	hex := match[1]
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	var out [3]float64
	for i := range out {
		value, _ := strconv.ParseUint(hex[2*i:2*i+2], 16, 8)
		out[i] = float64(value)
	}
	return out, nil
}

func halfUp(decimal string, places int) string {
	whole, fraction, _ := strings.Cut(decimal, ".")
	for len(fraction) <= places {
		fraction += "0"
	}
	up := fraction[places] >= '5'
	kept := []byte(whole + fraction[:places])
	for i := len(kept) - 1; up && i >= 0; i-- {
		if up = kept[i] == '9'; up {
			kept[i] = '0'
		} else {
			kept[i]++
		}
	}
	if up {
		kept = append([]byte{'1'}, kept...)
	}
	if places == 0 {
		return string(kept)
	}
	return string(kept[:len(kept)-places]) + "." + string(kept[len(kept)-places:])
}
