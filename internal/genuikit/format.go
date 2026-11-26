package genuikit

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed format.txt
var format string

type described struct {
	name, signature, description string
}

func listed(raw json.RawMessage) []described {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	if _, err := decoder.Token(); err != nil {
		return nil
	}
	var out []described
	for decoder.More() {
		key, _ := decoder.Token()
		var entry struct{ Signature, Description string }
		if decoder.Decode(&entry) != nil {
			return out
		}
		name, _ := key.(string)
		out = append(out, described{name, entry.Signature, entry.Description})
	}
	return out
}

var ComponentNames = sync.OnceValue(func() []string {
	var top struct{ Components json.RawMessage }
	_ = json.Unmarshal(catalog, &top)
	entries := listed(top.Components)
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.name
	}
	return names
})

var Format = sync.OnceValue(func() string {
	var top struct{ Components, Functions json.RawMessage }
	_ = json.Unmarshal(catalog, &top)
	var components, functions strings.Builder
	for _, entry := range listed(top.Components) {
		components.WriteString("- " + entry.signature)
		if entry.description != "" {
			components.WriteString(": " + entry.description)
		}
		components.WriteString("\n")
	}
	for _, entry := range listed(top.Functions) {
		functions.WriteString("- @" + entry.signature + ": " + entry.description + "\n")
	}
	text := strings.Replace(format, "{{components}}", strings.TrimRight(components.String(), "\n"), 1)
	return strings.TrimSpace(strings.Replace(text, "{{functions}}", strings.TrimRight(functions.String(), "\n"), 1))
})
