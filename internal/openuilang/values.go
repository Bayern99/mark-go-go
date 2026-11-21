package openuilang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type Object struct {
	Keys   []string
	Values map[string]any
}

func NewObject() *Object { return &Object{Values: map[string]any{}} }

func (o *Object) Get(key string) (any, bool) {
	if o == nil {
		return nil, false
	}
	value, ok := o.Values[key]
	return value, ok
}

func (o *Object) Set(key string, value any) {
	if _, ok := o.Values[key]; !ok {
		o.Keys = append(o.Keys, key)
	}
	o.Values[key] = value
}

func (o *Object) Delete(key string) {
	if _, ok := o.Values[key]; !ok {
		return
	}
	delete(o.Values, key)
	for i, existing := range o.Keys {
		if existing == key {
			o.Keys = append(o.Keys[:i:i], o.Keys[i+1:]...)
			return
		}
	}
}

func (o *Object) clone() *Object {
	out := NewObject()
	for _, key := range o.Keys {
		out.Set(key, o.Values[key])
	}
	return out
}

type Element struct {
	Type        string
	StatementID string
	Props       *Object
	dynamic     bool
}

type Binding struct {
	Target string
	expr   *node
}

type ActionStep struct {
	Type        string
	Message     string
	Context     any
	URL         string
	Target      string
	Targets     []string
	StatementID string
	RefType     string
	Name        string
	Params      *Object
	value       *node
	custom      bool
}

type ActionPlan struct {
	Steps []ActionStep
}

type undefinedValue struct{}

var undefined = undefinedValue{}

func isNullish(value any) bool {
	if value == nil {
		return true
	}
	_, ok := value.(undefinedValue)
	return ok
}

func orNull(value any) any {
	if isNullish(value) {
		return nil
	}
	return value
}

func decodeOrdered(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := decodeOrderedValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err == nil {
		return nil, fmt.Errorf("unexpected data after JSON value")
	}
	return value, nil
}

func decodeOrderedValue(decoder *json.Decoder) (any, error) {
	tok, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch v := tok.(type) {
	case json.Delim:
		if v == '{' {
			out := NewObject()
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				value, err := decodeOrderedValue(decoder)
				if err != nil {
					return nil, err
				}
				out.Set(key.(string), value)
			}
			_, err := decoder.Token()
			return out, err
		}
		out := []any{}
		for decoder.More() {
			value, err := decodeOrderedValue(decoder)
			if err != nil {
				return nil, err
			}
			out = append(out, value)
		}
		_, err := decoder.Token()
		return out, err
	case json.Number:
		return v.Float64()
	}
	return tok, nil
}

func truthy(value any) bool {
	switch v := value.(type) {
	case nil, undefinedValue:
		return false
	case bool:
		return v
	case float64:
		return v != 0 && !math.IsNaN(v)
	case string:
		return v != ""
	}
	return true
}

var decimalLiteral = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

func stringToNumber(s string) float64 {
	s = strings.TrimFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f' || r == 0xa0 || r == 0xfeff || r == 0x2028 || r == 0x2029
	})
	switch s {
	case "":
		return 0
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	if len(s) > 2 && s[0] == '0' {
		base := 0
		switch s[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			if strings.ContainsAny(s[2:], "_+-") {
				return math.NaN()
			}
			value, err := strconv.ParseUint(s[2:], base, 64)
			if err != nil {
				return math.NaN()
			}
			return float64(value)
		}
	}
	if !decimalLiteral.MatchString(s) {
		return math.NaN()
	}
	value, _ := strconv.ParseFloat(s, 64)
	return value
}

func toNumber(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case string:
		n := stringToNumber(v)
		if math.IsNaN(n) {
			return 0
		}
		return n
	case bool:
		if v {
			return 1
		}
	}
	return 0
}

func numberString(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case f == 0:
		return "0"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	sign := ""
	if f < 0 {
		sign = "-"
		f = -f
	}
	sci := strconv.FormatFloat(f, 'e', -1, 64)
	mantissa, exponent, _ := strings.Cut(sci, "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	exp, _ := strconv.Atoi(exponent)
	k, n := len(digits), exp+1
	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits
	}
	e := strconv.Itoa(n - 1)
	if n-1 > 0 {
		e = "+" + e
	}
	if k == 1 {
		return sign + digits + "e" + e
	}
	return sign + digits[:1] + "." + digits[1:] + "e" + e
}

func toString(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case undefinedValue:
		return "undefined"
	case string:
		return v
	case float64:
		return numberString(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			if !isNullish(item) {
				parts[i] = toString(item)
			}
		}
		return strings.Join(parts, ",")
	}
	return "[object Object]"
}

func stringOrEmpty(value any) string {
	if isNullish(value) {
		return ""
	}
	return toString(value)
}

func isPrimitive(value any) bool {
	switch value.(type) {
	case string, float64, bool:
		return true
	}
	return false
}

func sameReference(a, b any) bool {
	switch x := a.(type) {
	case []any:
		y, ok := b.([]any)
		return ok && len(x) == len(y) && len(x) > 0 && &x[0] == &y[0]
	case *Object:
		y, ok := b.(*Object)
		return ok && x == y
	case *Element:
		y, ok := b.(*Element)
		return ok && x == y
	}
	return false
}

func looseEqual(a, b any) bool {
	if isNullish(a) || isNullish(b) {
		return isNullish(a) && isNullish(b)
	}
	switch x := a.(type) {
	case float64:
		switch y := b.(type) {
		case float64:
			return x == y
		case string:
			return x == stringToNumber(y)
		case bool:
			return x == toNumber(y)
		}
		return looseEqual(a, toString(b))
	case string:
		switch y := b.(type) {
		case string:
			return x == y
		case float64:
			return stringToNumber(x) == y
		case bool:
			return stringToNumber(x) == toNumber(y)
		}
		return x == toString(b)
	case bool:
		if y, ok := b.(bool); ok {
			return x == y
		}
		return looseEqual(toNumber(x), b)
	}
	if isPrimitive(b) {
		return looseEqual(b, a)
	}
	return sameReference(a, b)
}

func strictEqual(a, b any) bool {
	switch x := a.(type) {
	case string:
		y, ok := b.(string)
		return ok && x == y
	case float64:
		y, ok := b.(float64)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	}
	return false
}
