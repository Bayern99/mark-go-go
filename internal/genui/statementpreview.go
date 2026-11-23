package genui

import (
	"bytes"
	"encoding/json"
)

// StatementPreview is the part of a version 3 createSurface message that has
// been written completely enough to read. Statements contains only complete
// JSON strings; the OpenUI Lang stream parser handles the statements from
// there.
type StatementPreview struct {
	Sources    map[string]StatementSource `json:"sources"`
	Statements []string                   `json:"statements"`
}

// ReadStatementPreview reads a possibly incomplete out.json. It deliberately
// does not validate the candidate: a preview is provisional until the whole
// file passes CheckCandidate.
func ReadStatementPreview(data []byte) StatementPreview {
	preview := StatementPreview{Sources: map[string]StatementSource{}}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if !jsonDelim(decoder, '{') {
		return preview
	}
	for decoder.More() {
		key, ok := jsonKey(decoder)
		if !ok {
			return preview
		}
		switch key {
		case "sources":
			var entries map[string]struct {
				Heading    string `json:"heading"`
				Occurrence int    `json:"occurrence"`
			}
			if decoder.Decode(&entries) != nil {
				return preview
			}
			for id, entry := range entries {
				preview.Sources[id] = StatementSource{Heading: entry.Heading, Occurrence: entry.Occurrence}
			}
		case "a2ui":
			readPreviewMessages(decoder, &preview)
		default:
			skipPreviewValue(decoder)
		}
	}
	return preview
}

func readPreviewMessages(decoder *json.Decoder, preview *StatementPreview) {
	if !jsonDelim(decoder, '[') {
		return
	}
	for decoder.More() {
		if !jsonDelim(decoder, '{') {
			return
		}
		for decoder.More() {
			key, ok := jsonKey(decoder)
			if !ok {
				return
			}
			if key == "createSurface" && len(preview.Statements) == 0 {
				readPreviewSurface(decoder, preview)
			} else {
				skipPreviewValue(decoder)
			}
		}
		if !jsonDelim(decoder, '}') {
			return
		}
	}
	if !jsonDelim(decoder, ']') {
		return
	}
}

func readPreviewSurface(decoder *json.Decoder, preview *StatementPreview) {
	if !jsonDelim(decoder, '{') {
		return
	}
	for decoder.More() {
		key, ok := jsonKey(decoder)
		if !ok {
			return
		}
		if key != "components" {
			skipPreviewValue(decoder)
			continue
		}
		if !jsonDelim(decoder, '[') {
			return
		}
		for decoder.More() {
			var statement string
			if decoder.Decode(&statement) != nil {
				return
			}
			preview.Statements = append(preview.Statements, statement)
		}
		if !jsonDelim(decoder, ']') {
			return
		}
	}
	if !jsonDelim(decoder, '}') {
		return
	}
}

func skipPreviewValue(decoder *json.Decoder) {
	var value json.RawMessage
	_ = decoder.Decode(&value)
}

func jsonKey(decoder *json.Decoder) (string, bool) {
	token, err := decoder.Token()
	if err != nil {
		return "", false
	}
	key, ok := token.(string)
	return key, ok
}

func jsonDelim(decoder *json.Decoder, want json.Delim) bool {
	token, err := decoder.Token()
	if err != nil {
		return false
	}
	delim, ok := token.(json.Delim)
	return ok && delim == want
}
