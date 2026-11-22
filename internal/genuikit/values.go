package genuikit

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/openuilang"
)

const maxItems = 10000

type arguments struct{ values *openuilang.Object }

func (a arguments) any(name string) any {
	value, _ := a.values.Get(name)
	return value
}

func (a arguments) has(name string) bool { return a.any(name) != nil }

func (a arguments) list(name string) []any {
	list, _ := a.any(name).([]any)
	return list
}

func (a arguments) number(name string) float64 {
	number, _ := a.any(name).(float64)
	return number
}

func (a arguments) text(name, fallback string) string {
	if text, ok := a.any(name).(string); ok {
		return text
	}
	return fallback
}

func (a arguments) flag(name string) bool {
	on, _ := a.any(name).(bool)
	return on
}

func finite(value float64) (any, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, errors.New("result is not a finite number")
	}
	return value, nil
}

func text(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return numeral(v)
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			if item != nil {
				parts[i] = text(item)
			}
		}
		return strings.Join(parts, ",")
	}
	return "[object Object]"
}

func numeral(value float64) string {
	switch {
	case math.IsNaN(value):
		return "NaN"
	case math.IsInf(value, 1):
		return "Infinity"
	case math.IsInf(value, -1):
		return "-Infinity"
	case value == 0:
		return "0"
	}
	if magnitude := math.Abs(value); magnitude >= 1e21 || magnitude < 1e-6 {
		return strings.Replace(strconv.FormatFloat(value, 'e', -1, 64), "e-0", "e-", 1)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func encoded(value any) string {
	var plain func(any) any
	plain = func(value any) any {
		switch v := value.(type) {
		case []any:
			out := make([]any, len(v))
			for i, item := range v {
				out[i] = plain(item)
			}
			return out
		case *openuilang.Object:
			return ordered{v, plain}
		case nil, string, bool, float64:
			return v
		}
		return nil
	}
	data, err := json.Marshal(plain(value))
	if err != nil {
		return ""
	}
	return string(data)
}

type ordered struct {
	object *openuilang.Object
	plain  func(any) any
}

func (o ordered) MarshalJSON() ([]byte, error) {
	var out strings.Builder
	out.WriteByte('{')
	for i, key := range o.object.Keys {
		if i > 0 {
			out.WriteByte(',')
		}
		name, _ := json.Marshal(key)
		value, err := json.Marshal(o.plain(o.object.Values[key]))
		if err != nil {
			return nil, err
		}
		out.Write(name)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return []byte(out.String()), nil
}

func read(item any, field string) any {
	if field == "" {
		return item
	}
	current := item
	for _, key := range strings.Split(field, ".") {
		switch v := current.(type) {
		case *openuilang.Object:
			current, _ = v.Get(key)
		case []any:
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || index >= len(v) {
				return nil
			}
			current = v[index]
		default:
			return nil
		}
	}
	return current
}

func same(a, b any) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case float64:
		switch y := b.(type) {
		case float64:
			return x == y
		case string:
			return numeral(x) == y
		}
		return false
	case string:
		switch y := b.(type) {
		case string:
			return x == y
		case float64:
			return x == numeral(y)
		}
		return false
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !same(x[i], y[i]) {
				return false
			}
		}
		return true
	case *openuilang.Object:
		y, ok := b.(*openuilang.Object)
		if !ok || x == nil || y == nil || len(x.Keys) != len(y.Keys) {
			return ok && x == y
		}
		for _, key := range x.Keys {
			other, has := y.Get(key)
			if !has || !same(x.Values[key], other) {
				return false
			}
		}
		return true
	}
	return a == b
}

func toNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, !math.IsNaN(v) && !math.IsInf(v, 0)
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		number, err := strconv.ParseFloat(trimmed, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || strings.ContainsAny(trimmed, "_xXpPnNiI") {
			return 0, false
		}
		return number, true
	}
	return 0, false
}

func aggregate(rows []any, field, how string) float64 {
	if how == "count" {
		return float64(len(rows))
	}
	var values []float64
	for _, row := range rows {
		if value, ok := toNumber(read(row, field)); ok {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return 0
	}
	total, low, high := 0.0, values[0], values[0]
	for _, value := range values {
		total += value
		low, high = min(low, value), max(high, value)
	}
	switch how {
	case "avg":
		return total / float64(len(values))
	case "min":
		return low
	case "max":
		return high
	}
	return total
}

type group struct {
	key  any
	rows []any
}

func groups(rows []any, key string) []*group {
	var out []*group
	index := map[string]*group{}
	for _, row := range rows {
		var name any
		switch raw := read(row, key).(type) {
		case float64:
			name = raw
		case nil:
			name = "Unknown"
		case string:
			name = raw
			if raw == "" {
				name = "Unknown"
			}
		default:
			name = text(raw)
		}
		id := fmt.Sprintf("%T:%v", name, name)
		if index[id] == nil {
			index[id] = &group{key: name}
			out = append(out, index[id])
		}
		index[id].rows = append(index[id].rows, row)
	}
	return out
}

func defaultAggregate(a arguments) string {
	if how := a.text("agg", ""); how != "" {
		return how
	}
	if a.text("field", "") != "" {
		return "sum"
	}
	return "count"
}

func object(pairs ...any) *openuilang.Object {
	out := openuilang.NewObject()
	for i := 0; i+1 < len(pairs); i += 2 {
		out.Set(pairs[i].(string), pairs[i+1])
	}
	return out
}

func withField(row any, field string, value any) any {
	out := openuilang.NewObject()
	if source, ok := row.(*openuilang.Object); ok && source != nil {
		for _, key := range source.Keys {
			out.Set(key, source.Values[key])
		}
	}
	out.Set(field, value)
	return out
}
