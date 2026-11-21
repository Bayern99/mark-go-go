package openuilang

import (
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

type Event struct {
	Type      string
	Message   string
	Params    *Object
	FormName  string
	FormState *Object
}

type Session struct {
	program  *Program
	store    *Object
	pristine map[string]bool
}

func NewSession() *Session {
	return &Session{store: NewObject(), pristine: map[string]bool{}}
}

func (s *Session) Load(program *Program) {
	s.program = program
	for _, key := range program.State.Keys {
		value := program.State.Values[key]
		if _, ok := s.store.Get(key); !ok || s.pristine[key] {
			s.store.Set(key, value)
			s.pristine[key] = true
		}
	}
}

func (s *Session) Get(name string) (any, bool) { return s.store.Get(name) }

func (s *Session) Set(name string, value any) {
	delete(s.pristine, name)
	s.store.Set(name, value)
}

func fieldValue(value any) any {
	if wrapped, ok := value.(*Object); ok {
		if inner, ok := wrapped.Get("value"); ok {
			return inner
		}
	}
	return value
}

func (s *Session) Field(form, name string) any {
	if form == "" {
		value, _ := s.store.Get(name)
		return fieldValue(value)
	}
	fields, _ := s.store.Values[form].(*Object)
	value, _ := fields.Get(name)
	return fieldValue(value)
}

func (s *Session) SetField(form, componentType, name string, value any) {
	wrapped := NewObject()
	wrapped.Set("value", value)
	wrapped.Set("componentType", componentType)
	if form == "" {
		s.Set(name, wrapped)
		return
	}
	fields := NewObject()
	if existing, ok := s.store.Values[form].(*Object); ok {
		fields = existing.clone()
	}
	fields.Set(name, wrapped)
	s.Set(form, fields)
}

func (s *Session) Render() (*Element, []Error) {
	if s.program == nil {
		return nil, nil
	}
	return s.program.Evaluate(s.Get)
}

func (s *Session) formState(form string) *Object {
	if fields, ok := s.store.Values[form].(*Object); ok && form != "" {
		out := NewObject()
		out.Set(form, fields)
		return out
	}
	return s.store.clone()
}

func (s *Session) Trigger(label, form string, action any) []Event {
	event := func(kind, message string, params *Object) Event {
		return Event{Type: kind, Message: message, Params: params, FormName: form, FormState: s.formState(form)}
	}
	plan, ok := action.(*ActionPlan)
	if !ok {
		return []Event{event("continue_conversation", label, NewObject())}
	}
	var events []Event
	for _, step := range plan.Steps {
		switch step.Type {
		case "run":
			if step.RefType == "mutation" {
				return events
			}
		case "continue_conversation":
			params := NewObject()
			if step.Context != nil {
				params.Set("context", step.Context)
			}
			events = append(events, event(step.Type, step.Message, params))
		case "open_url":
			params := NewObject()
			params.Set("url", step.URL)
			events = append(events, event(step.Type, "", params))
		case "custom_action":
			events = append(events, event(step.Name, "", step.Params))
		case "set":
			if s.program != nil {
				s.Set(step.Target, s.program.EvaluateExpression(step, s.Get))
			}
		case "reset":
			for _, target := range step.Targets {
				var value any
				if s.program != nil {
					value, _ = s.program.State.Get(target)
				}
				s.store.Set(target, value)
				s.pristine[target] = true
			}
		}
	}
	return events
}

func emptyValue(value any) bool {
	switch v := value.(type) {
	case nil, undefinedValue:
		return true
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case *Object:
		if inner, ok := v.Get("value"); ok {
			return emptyValue(inner)
		}
		return len(v.Keys) == 0
	}
	return false
}

var (
	emailShape  = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	floatPrefix = regexp.MustCompile(`^[+-]?(Infinity|\d+\.?\d*([eE][+-]?\d+)?|\.\d+([eE][+-]?\d+)?)`)
)

func parseFloatPrefix(text string) float64 {
	match := floatPrefix.FindString(strings.TrimLeft(text, " \t\r\n\v\f"))
	if match == "" {
		return math.NaN()
	}
	if strings.HasSuffix(match, "Infinity") {
		return math.Inf(1)
	}
	value, _ := strconv.ParseFloat(match, 64)
	return value
}

func ruleNumber(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case string:
		return stringToNumber(v)
	}
	return toNumber(value)
}

func valueNumber(value any) float64 {
	if n, ok := value.(float64); ok {
		return n
	}
	return parseFloatPrefix(toString(value))
}

func ValidateField(value, rules any) string {
	spec, ok := rules.(*Object)
	if !ok {
		return ""
	}
	for _, rule := range spec.Keys {
		arg := spec.Values[rule]
		if enabled, isBool := arg.(bool); arg == nil || isBool && !enabled {
			continue
		}
		if message := checkRule(rule, value, arg); message != "" {
			return message
		}
	}
	return ""
}

func checkRule(rule string, value, arg any) string {
	if rule == "required" {
		if emptyValue(value) {
			return "This field is required"
		}
		if options, ok := value.(*Object); ok {
			for _, key := range options.Keys {
				if checked, isBool := options.Values[key].(bool); !isBool || checked {
					return ""
				}
			}
			return "At least one option is required"
		}
		return ""
	}
	if emptyValue(value) {
		return ""
	}
	text, isText := value.(string)
	switch rule {
	case "email":
		if !isText || !emailShape.MatchString(text) {
			return "Please enter a valid email"
		}
	case "url":
		parsed, err := url.Parse(text)
		if !isText || err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "Please enter a valid URL"
		}
	case "numeric":
		number, isNumber := value.(float64)
		if isNumber && !math.IsNaN(number) {
			return ""
		}
		if !isText || strings.TrimSpace(text) == "" || math.IsNaN(parseFloatPrefix(text)) {
			return "Must be a number"
		}
	case "min":
		if n := valueNumber(value); !math.IsNaN(n) && !(n >= ruleNumber(arg)) {
			return "Must be at least " + numberString(ruleNumber(arg))
		}
	case "max":
		if n := valueNumber(value); !math.IsNaN(n) && !(n <= ruleNumber(arg)) {
			return "Must be no more than " + numberString(ruleNumber(arg))
		}
	case "minLength":
		if isText && !(float64(len(utf16.Encode([]rune(text)))) >= ruleNumber(arg)) {
			return "Must be at least " + numberString(ruleNumber(arg)) + " characters"
		}
	case "maxLength":
		if isText && !(float64(len(utf16.Encode([]rune(text)))) <= ruleNumber(arg)) {
			return "Must be no more than " + numberString(ruleNumber(arg)) + " characters"
		}
	case "pattern":
		source, ok := arg.(string)
		if !isText || !ok {
			return ""
		}
		if pattern, err := regexp.Compile(source); err == nil && !pattern.MatchString(text) {
			return "Invalid format"
		}
	}
	return ""
}
