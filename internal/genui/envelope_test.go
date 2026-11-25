package genui

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAssembleViewStampsTheVersionTheMessagesAskFor(t *testing.T) {
	at := time.Date(2026, time.July, 28, 10, 30, 0, 0, time.UTC)
	componentMessages := json.RawMessage(`[{"version":"1.0","createSurface":{"surfaceId":"s","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Text","text":"Hi"}]}}]`)
	statementMessages := json.RawMessage(`[{"version":"1.0","createSurface":{"surfaceId":"s","catalogId":"markgogo.dev:genui/v3","components":["root = Text(\"Hi\")"]}}]`)

	for name, tc := range map[string]struct {
		a2ui    json.RawMessage
		version int
		want    string
	}{
		"component view is version 2": {
			a2ui:    componentMessages,
			version: 2,
			want: `{
  "version": 2,
  "generatedAt": "2026-07-28T10:30:00Z",
  "sources": {
    "intro": {
      "heading": "Intro",
      "occurrence": 2,
      "hash": "fnv1a32:0000000f"
    }
  },
  "a2ui": [
    {
      "version": "1.0",
      "createSurface": {
        "surfaceId": "s",
        "catalogId": "markgogo.dev:genui/v1",
        "components": [
          {
            "id": "root",
            "component": "Text",
            "text": "Hi"
          }
        ]
      }
    }
  ]
}
`,
		},
		"statement view is version 3": {
			a2ui:    statementMessages,
			version: 3,
			want: `{
  "version": 3,
  "language": "openui-lang/1.0",
  "generatedAt": "2026-07-28T10:30:00Z",
  "sources": {
    "intro": {
      "heading": "Intro",
      "occurrence": 2,
      "hash": "fnv1a32:0000000f"
    }
  },
  "a2ui": [
    {
      "version": "1.0",
      "createSurface": {
        "surfaceId": "s",
        "catalogId": "markgogo.dev:genui/v3",
        "components": [
          "root = Text(\"Hi\")"
        ]
      }
    }
  ]
}
`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			content, version, err := AssembleView(ViewOptions{
				A2UI:        tc.a2ui,
				Sources:     map[string]AgentSource{"intro": {Heading: "Intro", Occurrence: 2}},
				GeneratedAt: at,
				Hash:        func(string, AgentSource) string { return "fnv1a32:0000000f" },
			})
			if err != nil {
				t.Fatalf("AssembleView: %v", err)
			}
			if version != tc.version {
				t.Errorf("version = %d, want %d", version, tc.version)
			}
			if string(content) != tc.want {
				t.Errorf("content =\n%s\nwant\n%s", content, tc.want)
			}
		})
	}
}

func TestAssembleViewLeavesOutAnEmptyModelAndRequest(t *testing.T) {
	content, _, err := AssembleView(ViewOptions{
		A2UI:        json.RawMessage(`[{"version":"1.0","createSurface":{"surfaceId":"s","catalogId":"markgogo.dev:genui/v1","components":[{"id":"root","component":"Text","text":"Hi"}]}}]`),
		GeneratedAt: time.Date(2026, time.July, 28, 10, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("AssembleView: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(content, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"model", "request"} {
		if _, present := raw[key]; present {
			t.Errorf("view carries an empty %q", key)
		}
	}
}

func TestAssembleViewRefusesAMissingCandidate(t *testing.T) {
	if _, _, err := AssembleView(ViewOptions{A2UI: json.RawMessage(`[]`)}); err == nil {
		t.Fatal("AssembleView accepted an empty candidate")
	}
}
