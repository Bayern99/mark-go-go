package genui

import (
	"encoding/json"
	"errors"
	"time"
)

// ViewSource is a source reference as it is stamped into a view file.
type ViewSource struct {
	Heading    string `json:"heading"`
	Occurrence int    `json:"occurrence"`
	Hash       string `json:"hash,omitempty"`
}

// ViewOptions is what the app knows about a candidate view: the messages an
// agent wrote, where they came from, and the hash of each cited section.
type ViewOptions struct {
	A2UI        json.RawMessage
	Sources     map[string]AgentSource
	Model       string
	Request     string
	GeneratedAt time.Time
	Hash        func(id string, src AgentSource) string
}

// AssembleView wraps agent messages in the view file envelope and reports the
// version it stamped: 3 for openui-lang statements, 2 for A2UI components.
func AssembleView(opts ViewOptions) ([]byte, int, error) {
	var messages []json.RawMessage
	if err := json.Unmarshal(opts.A2UI, &messages); err != nil || len(messages) == 0 {
		return nil, 0, errors.New(`"a2ui" must be a non-empty array of A2UI messages`)
	}
	at := opts.GeneratedAt
	if at.IsZero() {
		at = time.Now()
	}
	version, language := ViewVersion, ""
	if WritesStatements(opts.A2UI) {
		version, language = StatementViewVersion, StatementLanguage
	}
	sources := make(map[string]ViewSource, len(opts.Sources))
	for key, src := range opts.Sources {
		entry := ViewSource{Heading: src.Heading, Occurrence: src.Occurrence}
		if opts.Hash != nil {
			entry.Hash = opts.Hash(key, src)
		}
		sources[key] = entry
	}
	view := struct {
		Version     int                   `json:"version"`
		Language    string                `json:"language,omitempty"`
		GeneratedAt string                `json:"generatedAt"`
		Model       string                `json:"model,omitempty"`
		Request     string                `json:"request,omitempty"`
		Sources     map[string]ViewSource `json:"sources"`
		A2UI        json.RawMessage       `json:"a2ui"`
	}{
		Version:     version,
		Language:    language,
		GeneratedAt: at.UTC().Format(time.RFC3339),
		Model:       opts.Model,
		Request:     opts.Request,
		Sources:     sources,
		A2UI:        opts.A2UI,
	}
	data, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return nil, 0, err
	}
	return append(data, '\n'), version, nil
}
