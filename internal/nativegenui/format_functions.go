package nativegenui

import (
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/currency"
	"golang.org/x/text/language"
	textmessage "golang.org/x/text/message"
)

const maxFormatFractionDigits = 100

var dateFormatTokens = regexp.MustCompile(`yyyy|yy|MMMM|MMM|MM|M|EEEE|E|dd|d|HH|H|hh|h|mm|ss|a`)

func formatNumber(args map[string]any) (any, error) {
	value, ok := numericArgument(args["value"])
	if !ok || math.IsNaN(value) {
		return "", nil
	}
	maximum, hasDecimals := decimalArgument(args, "decimals", 3)
	if !hasDecimals {
		return "", nil
	}
	minimum := 0
	if _, ok := args["decimals"]; ok {
		minimum = maximum
	}
	return formatEnglishNumber(value, minimum, maximum, groupingArgument(args)), nil
}

func formatCurrency(args map[string]any) (any, error) {
	value, ok := numericArgument(args["value"])
	if !ok || math.IsNaN(value) {
		return "", nil
	}
	currencyCode, ok := args["currency"].(string)
	if !ok || currencyCode == "" {
		return "", nil
	}
	currencyCode = strings.ToUpper(currencyCode)
	decimals, ok := decimalArgument(args, "decimals", 2)
	if !ok {
		return "", nil
	}
	formatted := formatEnglishNumber(value, decimals, decimals, groupingArgument(args))
	sign := ""
	if strings.HasPrefix(formatted, "-") {
		sign = "-"
		formatted = strings.TrimPrefix(formatted, "-")
	}
	return sign + englishCurrencyPrefix(currencyCode) + formatted, nil
}

func englishCurrencyPrefix(code string) string {
	switch code {
	case "PHP":
		return "₱"
	case "SAR":
		return "\u200e\u20c1\u00a0"
	case "XAF":
		return "FCFA\u00a0"
	case "XCG":
		return "Cg.\u00a0"
	case "XOF":
		return "F\u202fCFA\u00a0"
	case "XPF":
		return "CFPF\u00a0"
	case "XXX":
		return "¤"
	}
	unit, err := currency.ParseISO(code)
	if err != nil {
		return code + "\u00a0"
	}
	printer := textmessage.NewPrinter(language.AmericanEnglish)
	symbol := printer.Sprint(currency.Symbol.Default(unit)(unit))
	if symbol == code {
		return symbol + "\u00a0"
	}
	return symbol
}

func formatDate(args map[string]any) (any, error) {
	value, ok := args["value"].(string)
	if !ok || value == "" {
		return "", nil
	}
	timestamp, ok := parseFormatTimestamp(value)
	if !ok {
		return "", nil
	}
	pattern, ok := args["format"].(string)
	if !ok || pattern == "" {
		pattern = "yyyy-MM-dd"
	}
	if pattern == "ISO" {
		return timestamp.UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z"), nil
	}
	return dateFormatTokens.ReplaceAllStringFunc(pattern, func(token string) string {
		switch token {
		case "yyyy":
			return strconv.Itoa(timestamp.Year())
		case "yy":
			year := strconv.Itoa(timestamp.Year())
			if len(year) > 2 {
				return year[len(year)-2:]
			}
			return year
		case "MMMM":
			return timestamp.Month().String()
		case "MMM":
			return timestamp.Format("Jan")
		case "MM":
			return twoDigits(int(timestamp.Month()))
		case "M":
			return strconv.Itoa(int(timestamp.Month()))
		case "EEEE":
			return timestamp.Weekday().String()
		case "E":
			return timestamp.Format("Mon")
		case "dd":
			return twoDigits(timestamp.Day())
		case "d":
			return strconv.Itoa(timestamp.Day())
		case "HH":
			return twoDigits(timestamp.Hour())
		case "H":
			return strconv.Itoa(timestamp.Hour())
		case "hh":
			return twoDigits(hour12(timestamp.Hour()))
		case "h":
			return strconv.Itoa(hour12(timestamp.Hour()))
		case "mm":
			return twoDigits(timestamp.Minute())
		case "ss":
			return twoDigits(timestamp.Second())
		case "a":
			if timestamp.Hour() < 12 {
				return "AM"
			}
			return "PM"
		default:
			return token
		}
	}), nil
}

func parseFormatTimestamp(value string) (time.Time, bool) {
	layouts := [...]string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999999-0700",
		"2006-01-02T15:04-0700",
		"2006-01-02T15-0700",
		"2006-01-02T15:04Z07:00",
		"2006-01-02T15Z07:00",
		"2006-01-02T15:04-07:00",
		"2006-01-02T15-07:00",
		"2006-01-02Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04Z07:00",
		"2006-01-02 15Z07:00",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02T15",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02 15",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if timestamp, err := time.Parse(layout, value); err == nil {
			return timestamp, true
		}
	}
	return time.Time{}, false
}

func pluralize(args map[string]any) (any, error) {
	value, ok := numericArgument(args["value"])
	category := "other"
	if ok {
		switch value {
		case 0:
			if _, exists := args["zero"]; exists {
				category = "zero"
			}
		case 1:
			category = "one"
		case 2:
			if _, exists := args["two"]; exists {
				category = "two"
			}
		}
		if category == "other" && formatEnglishNumber(math.Abs(value), 0, 3, false) == "1" {
			category = "one"
		}
	}
	if form, exists := args[category]; exists && form != nil {
		return dataString(form)
	}
	if form, exists := args["other"]; exists && form != nil {
		return dataString(form)
	}
	return "", nil
}

func numericArgument(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case int32:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint64:
		return float64(number), true
	case uint32:
		return float64(number), true
	default:
		return 0, false
	}
}

func decimalArgument(args map[string]any, name string, defaultValue int) (int, bool) {
	value, exists := args[name]
	if !exists {
		return defaultValue, true
	}
	number, ok := numericArgument(value)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number ||
		number < 0 || number > maxFormatFractionDigits {
		return 0, false
	}
	return int(number), true
}

func groupingArgument(args map[string]any) bool {
	grouping, ok := args["grouping"].(bool)
	return !ok || grouping
}

func formatEnglishNumber(value float64, minimum, maximum int, grouping bool) string {
	if math.IsInf(value, 1) {
		return "∞"
	}
	if math.IsInf(value, -1) {
		return "-∞"
	}
	negative := math.Signbit(value)
	decimal := strconv.FormatFloat(math.Abs(value), 'f', -1, 64)
	integer, fraction, _ := strings.Cut(decimal, ".")
	if len(fraction) < maximum {
		fraction += strings.Repeat("0", maximum-len(fraction))
	}
	digits := integer + fraction[:maximum]
	rounded, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return ""
	}
	if len(fraction) > maximum && fraction[maximum] >= '5' {
		rounded.Add(rounded, big.NewInt(1))
	}
	digits = rounded.String()
	for len(digits) <= maximum {
		digits = "0" + digits
	}
	integer = digits
	fraction = ""
	if maximum > 0 {
		integer = digits[:len(digits)-maximum]
		fraction = digits[len(digits)-maximum:]
		for len(fraction) > minimum && strings.HasSuffix(fraction, "0") {
			fraction = strings.TrimSuffix(fraction, "0")
		}
	}
	if grouping {
		integer = groupEnglishDigits(integer)
	}
	formatted := integer
	if fraction != "" {
		formatted += "." + fraction
	}
	if negative {
		formatted = "-" + formatted
	}
	return formatted
}

func groupEnglishDigits(digits string) string {
	first := len(digits) % 3
	if first == 0 {
		first = 3
	}
	var grouped strings.Builder
	grouped.Grow(len(digits) + len(digits)/3)
	grouped.WriteString(digits[:first])
	for i := first; i < len(digits); i += 3 {
		grouped.WriteByte(',')
		grouped.WriteString(digits[i : i+3])
	}
	return grouped.String()
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

func hour12(hour int) int {
	if hour%12 == 0 {
		return 12
	}
	return hour % 12
}
