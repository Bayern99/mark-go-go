package nativegenui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Bayern99/mark-go-go/internal/genui"
)

type validationResult struct {
	Valid    bool   `json:"valid"`
	Code     string `json:"code,omitempty"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message,omitempty"`
}

var emailAddress = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

func dataString(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	if text, ok := value.(string); ok {
		return text, nil
	}
	if _, ok := value.(map[string]any); ok {
		return compactJSON(value)
	}
	if _, ok := value.([]any); ok {
		return compactJSON(value)
	}
	if result, ok := value.(validationResult); ok {
		return compactJSON(result)
	}
	return fmt.Sprint(value), nil
}

func compactJSON(value any) (string, error) {
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(data.String(), "\n"), nil
}

func resolveHidden(value *json.RawMessage, dataModel any, dataPath string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(*value, &object); err != nil || object == nil {
		return nil
	}
	if _, path := dataBindingPath(object); !path {
		if _, call := functionCallName(object); !call {
			return nil
		}
	}
	resolved, err := resolveDynamicValue(*value, dataModel, dataPath, 0, 0)
	if err != nil {
		return err
	}
	hidden, _ := resolved.(bool)
	*value, err = json.Marshal(hidden)
	return err
}

func dataBindingPath(object map[string]json.RawMessage) (string, bool) {
	for key := range object {
		if key != "@path" && key != "path" {
			return "", false
		}
	}
	for _, key := range []string{"@path", "path"} {
		if raw, ok := object[key]; ok {
			var path string
			if json.Unmarshal(raw, &path) == nil {
				return path, true
			}
		}
	}
	return "", false
}

func functionCallName(object map[string]json.RawMessage) (string, bool) {
	for _, key := range []string{"@call", "call"} {
		if raw, ok := object[key]; ok {
			var name string
			if json.Unmarshal(raw, &name) == nil && name != "" {
				return name, true
			}
		}
	}
	return "", false
}

const (
	maxFunctionValueDepth     = genui.MaxGlobalDepth
	maxFunctionArguments      = 1_000
	maxFunctionArrayItems     = 10_000
	maxExpressionTemplateSize = 10_000
	maxExpressionParts        = 1_000
)

var expressionNumber = regexp.MustCompile(`^[+-]?(?:[0-9]+\.?[0-9]*|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

func resolveDynamicValue(raw json.RawMessage, dataModel any, dataPath string, functionDepth, valueDepth int) (any, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	if valueDepth > maxFunctionValueDepth {
		return nil, errors.New("GenUI function arguments exceed the nesting limit")
	}
	switch raw[0] {
	case '[':
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		if len(items) > maxFunctionArrayItems {
			return nil, errors.New("GenUI function array exceeds the item limit")
		}
		values := make([]any, len(items))
		for i, item := range items {
			value, err := resolveDynamicValue(item, dataModel, dataPath, functionDepth, valueDepth+1)
			if err != nil {
				return nil, err
			}
			values[i] = value
		}
		return values, nil
	case '{':
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return nil, err
		}
		if path, ok := dataBindingPath(object); ok {
			value, found, err := dataModelAtPath(dataModel, resolveDataPath(path, dataPath))
			if err != nil || !found {
				return nil, err
			}
			return value, nil
		}
		if name, ok := functionCallName(object); ok {
			if functionDepth >= genui.MaxFuncCallDepth {
				return nil, errors.New("GenUI function calls exceed the nesting limit")
			}
			if name != "formatString" && name != "and" && name != "or" && name != "not" &&
				name != "required" && name != "length" && name != "email" {
				return nil, nil
			}
			if rawCatalog, ok := object["catalogId"]; ok {
				var catalog string
				if err := json.Unmarshal(rawCatalog, &catalog); err != nil {
					return nil, err
				}
				if catalog != genui.CatalogID {
					return nil, nil
				}
			}
			var rawArgs map[string]json.RawMessage
			if args, ok := object["args"]; ok {
				if err := json.Unmarshal(args, &rawArgs); err != nil {
					return nil, err
				}
			}
			if len(rawArgs) > maxFunctionArguments {
				return nil, errors.New("GenUI function has too many arguments")
			}
			for key := range rawArgs {
				if !pureFunctionArgumentAllowed(name, key) {
					return nil, nil
				}
			}
			args := make(map[string]any, len(rawArgs))
			for key, rawArg := range rawArgs {
				value, err := resolveDynamicValue(rawArg, dataModel, dataPath, functionDepth+1, valueDepth+1)
				if err != nil {
					return nil, err
				}
				args[key] = value
			}
			return evaluatePureFunction(name, args, dataModel, dataPath)
		}
		resolved := make(map[string]any, len(object))
		for key, item := range object {
			value, err := resolveDynamicValue(item, dataModel, dataPath, functionDepth, valueDepth+1)
			if err != nil {
				return nil, err
			}
			resolved[key] = value
		}
		return resolved, nil
	default:
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return value, nil
	}
}

func pureFunctionArgumentAllowed(function, argument string) bool {
	switch function {
	case "formatString", "required", "email":
		return argument == "value"
	case "length":
		return argument == "value" || argument == "min" || argument == "max"
	case "and", "or":
		return argument == "values"
	case "not":
		return argument == "value"
	default:
		return false
	}
}

func evaluatePureFunction(name string, args map[string]any, dataModel any, dataPath string) (any, error) {
	switch name {
	case "required":
		value, ok := args["value"]
		if !ok {
			return nil, nil
		}
		valid := value != nil
		switch value := value.(type) {
		case string:
			valid = value != ""
		case []any:
			valid = len(value) != 0
		}
		if !valid {
			return validationResult{Valid: false, Message: "This field is required."}, nil
		}
		return validationResult{Valid: true}, nil
	case "length":
		value, ok := args["value"]
		if !ok {
			return nil, nil
		}
		minimum, hasMinimum := args["min"].(float64)
		maximum, hasMaximum := args["max"].(float64)
		if !hasMinimum && !hasMaximum {
			return nil, nil
		}
		length := 0
		switch value := value.(type) {
		case string:
			length = utf16Length(value)
		case []any:
			length = len(value)
		}
		if hasMinimum && float64(length) < minimum {
			return validationResult{Valid: false, Message: fmt.Sprintf("Minimum length is %s.", fmt.Sprint(minimum))}, nil
		}
		if hasMaximum && float64(length) > maximum {
			return validationResult{Valid: false, Message: fmt.Sprintf("Maximum length is %s.", fmt.Sprint(maximum))}, nil
		}
		return validationResult{Valid: true}, nil
	case "email":
		value, ok := args["value"]
		if !ok {
			return nil, nil
		}
		text, ok := value.(string)
		if ok && emailAddress.MatchString(text) {
			return validationResult{Valid: true}, nil
		}
		return validationResult{
			Valid:    false,
			Code:     "INVALID_EMAIL",
			Severity: "error",
			Message:  "Please enter a valid email address (e.g. user@example.com).",
		}, nil
	case "formatString":
		value, ok := args["value"]
		if !ok {
			return nil, nil
		}
		template, err := dataString(value)
		if err != nil {
			return nil, err
		}
		return interpolateJSONPointers(template, dataModel, dataPath)
	case "and", "or":
		values, ok := args["values"].([]any)
		if !ok || len(values) < 2 || len(values) > maxFunctionArrayItems {
			return nil, nil
		}
		result := name == "and"
		for _, value := range values {
			if name == "and" && !a2uiTruthy(value) {
				return false, nil
			}
			if name == "or" && a2uiTruthy(value) {
				return true, nil
			}
		}
		return result, nil
	case "not":
		value, ok := args["value"]
		if !ok {
			return nil, nil
		}
		return !a2uiTruthy(value), nil
	default:
		return nil, nil
	}
}

func a2uiTruthy(value any) bool {
	if result, ok := value.(validationResult); ok {
		return result.Valid
	}
	if result, ok := value.(map[string]any); ok {
		if valid, ok := result["valid"].(bool); ok {
			return valid
		}
	}
	switch value := value.(type) {
	case nil:
		return false
	case bool:
		return value
	case float64:
		return value != 0
	case string:
		return value != ""
	default:
		return true
	}
}

func interpolateJSONPointers(template string, dataModel any, dataPath string) (string, error) {
	if utf16Length(template) > maxExpressionTemplateSize {
		return "", errors.New("GenUI formatString template exceeds the length limit")
	}
	var output strings.Builder
	parts := 0
	for position := 0; position < len(template); {
		if position+2 < len(template) && template[position] == '\\' && template[position+1] == '$' && template[position+2] == '{' {
			output.WriteString("${")
			position += 3
			parts++
		} else if position+1 < len(template) && template[position] == '$' && template[position+1] == '{' {
			end, err := interpolationEnd(template, position+2)
			if err != nil {
				return "", err
			}
			value, err := interpolationValue(strings.TrimSpace(template[position+2:end]), dataModel, dataPath)
			if err != nil {
				return "", err
			}
			text, err := dataString(value)
			if err != nil {
				return "", err
			}
			output.WriteString(text)
			position = end + 1
			parts++
		} else {
			start := position
			for position < len(template) {
				if position+1 < len(template) && template[position] == '$' && template[position+1] == '{' {
					break
				}
				if position+2 < len(template) && template[position] == '\\' && template[position+1] == '$' && template[position+2] == '{' {
					break
				}
				position++
			}
			output.WriteString(template[start:position])
			parts++
		}
		if parts > maxExpressionParts || output.Len() > genui.MaxViewBytes {
			return "", errors.New("GenUI formatString output exceeds the size limit")
		}
	}
	return output.String(), nil
}

func interpolationEnd(template string, start int) (int, error) {
	depth := 1
	var quote byte
	for position := start; position < len(template); position++ {
		character := template[position]
		if quote != 0 {
			if character == '\\' {
				position++
			} else if character == quote {
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return position, nil
			}
		}
	}
	return 0, errors.New("unclosed GenUI formatString interpolation")
}

func interpolationValue(expression string, dataModel any, dataPath string) (any, error) {
	if expression == "" || expression == "null" {
		return nil, nil
	}
	if expression == "true" {
		return true, nil
	}
	if expression == "false" {
		return false, nil
	}
	if expressionNumber.MatchString(expression) {
		return strconv.ParseFloat(expression, 64)
	}
	if len(expression) >= 2 && (expression[0] == '\'' || expression[0] == '"') && expression[len(expression)-1] == expression[0] {
		return expressionStringLiteral(expression)
	}
	if strings.ContainsAny(expression, "()") || strings.ContainsAny(expression, " \t\r\n") {
		return nil, nil
	}
	value, found, err := dataModelAtPath(dataModel, resolveDataPath(expression, dataPath))
	if err != nil || !found {
		return nil, err
	}
	return value, nil
}

func expressionStringLiteral(expression string) (string, error) {
	var result strings.Builder
	for position := 1; position < len(expression)-1; position++ {
		character := expression[position]
		if character == '\\' {
			position++
			if position >= len(expression)-1 {
				return "", errors.New("invalid GenUI formatString string literal")
			}
			switch expression[position] {
			case 'n':
				result.WriteByte('\n')
			case 'r':
				result.WriteByte('\r')
			case 't':
				result.WriteByte('\t')
			default:
				result.WriteByte(expression[position])
			}
		} else {
			result.WriteByte(character)
		}
	}
	return result.String(), nil
}
