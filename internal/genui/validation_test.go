package genui

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const validFullView = `{
  "version": 2,
  "generatedAt": "2026-11-10T02:47:20Z",
  "sources": {
    "overview": { "heading": "Overview", "occurrence": 1, "hash": "fnv1a32:00000000" }
  },
  "a2ui": [
    {
      "version": "1.0",
      "createSurface": {
        "surfaceId": "view",
        "catalogId": "markgogo.dev:genui/v1",
        "components": [
          { "id": "root", "component": "Column", "children": ["kp"] },
          { "id": "kp", "component": "KeyPoints", "title": "Points", "items": ["One."], "sources": ["overview"] }
        ]
      }
    }
  ]
}`

func createSurfaceMsg(surfaceID string, components []map[string]any) map[string]any {
	return map[string]any{
		"version": "1.0",
		"createSurface": map[string]any{
			"surfaceId":  surfaceID,
			"catalogId":  CatalogID,
			"components": components,
		},
	}
}

func viewFileJSON(t *testing.T, messages []any, sources map[string]any) string {
	t.Helper()
	if sources == nil {
		sources = map[string]any{}
	}
	data, err := json.Marshal(map[string]any{
		"version":     2,
		"generatedAt": "2026-11-10T02:47:20Z",
		"sources":     sources,
		"a2ui":        messages,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertValidationError(t *testing.T, raw string, wantSubstrings ...string) *ValidationError {
	t.Helper()
	err := ValidateGeneratedView([]byte(raw))
	if err == nil {
		t.Fatalf("ValidateGeneratedView(%s) = nil, want error containing %v", raw, wantSubstrings)
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("ValidateGeneratedView error = %T (%v), want *ValidationError", err, err)
	}
	if ve.Code == "" || ve.Message == "" || ve.Hint == "" {
		t.Errorf("ValidationError missing diagnostic fields: %+v", ve)
	}
	for _, sub := range wantSubstrings {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("ValidateGeneratedView error = %q, want substring %q", err.Error(), sub)
		}
	}
	return ve
}

func TestValidateGeneratedViewAcceptsWholeMergedAndFormatExample(t *testing.T) {
	if err := ValidateGeneratedView([]byte(validFullView)); err != nil {
		t.Fatalf("ValidateGeneratedView(validFullView) = %v, want nil", err)
	}

	merged := viewFileJSON(t, []any{
		createSurfaceMsg("view", []map[string]any{
			{"id": "root", "component": "Column", "children": []string{"a", "b"}},
			{"id": "a", "component": "KeyPoints", "items": []string{"Old A"}},
			{"id": "b", "component": "KeyPoints", "items": []string{"Old B"}},
		}),
		map[string]any{
			"version": "1.0",
			"updateComponents": map[string]any{
				"surfaceId": "view",
				"components": []map[string]any{
					{"id": "b", "component": "Card", "child": "c"},
					{"id": "c", "component": "KeyPoints", "items": []string{"New C"}},
				},
			},
		},
	}, nil)
	if err := ValidateGeneratedView([]byte(merged)); err != nil {
		t.Fatalf("ValidateGeneratedView(merged) = %v, want nil", err)
	}

	formatDoc, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "plugins", "genui", "view-format.md"))
	if err != nil {
		t.Fatalf("read view-format.md: %v", err)
	}
	re := regexp.MustCompile("(?s)```json\\n(.*?)\\n```")
	m := re.FindSubmatch(formatDoc)
	if len(m) < 2 {
		t.Fatal("view-format.md has no JSON example")
	}
	if err := ValidateGeneratedView(m[1]); err != nil {
		t.Fatalf("ValidateGeneratedView(view-format.md example) = %v, want nil", err)
	}

	var parsed struct {
		A2UI []struct {
			CreateSurface struct {
				Components []struct {
					Component string `json:"component"`
				} `json:"components"`
			} `json:"createSurface"`
		} `json:"a2ui"`
	}
	if err := json.Unmarshal(m[1], &parsed); err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, c := range parsed.A2UI[0].CreateSurface.Components {
		used[c.Component] = true
	}
	gotNames := make([]string, 0, len(used))
	for name := range used {
		gotNames = append(gotNames, name)
	}
	sort.Strings(gotNames)
	wantNames := CatalogComponents()
	sort.Strings(wantNames)
	if strings.Join(gotNames, ",") != strings.Join(wantNames, ",") {
		t.Fatalf("catalog components in view-format.md = %v, want %v", gotNames, wantNames)
	}
}

func TestValidateGeneratedViewRejectsInvalidEnvelopeAndSources(t *testing.T) {
	assertValidationError(t, "not json", "not valid JSON")
	assertValidationError(t, `{"version":2,"sources":{},"a2ui":[]} trailing`, "not valid JSON")
	assertValidationError(t, `[]`, "not a JSON object")
	assertValidationError(t, `{"version":1,"sources":{},"a2ui":[]}`, "version 2")
	assertValidationError(t, `{"version":2,"sources":[],"a2ui":[]}`, "sources are not an object")
	assertValidationError(t, `{"version":2,"sources":{},"a2ui":[]}`, "no A2UI messages")
	assertValidationError(t, `{"version":2,"sources":{"s1":"bad"},"a2ui":[{}]}`, `Source "s1" needs a "heading" string.`)
	assertValidationError(t, `{"version":2,"sources":{"s1":{"heading":"  "}},"a2ui":[{}]}`, `Source "s1" needs a "heading" string.`)
	assertValidationError(t, `{"version":2,"sources":{"s1":{"heading":"H","occurrence":0}},"a2ui":[{}]}`, `Source "s1": "occurrence" must be a whole number from 1.`)
	assertValidationError(t, `{"version":2,"sources":{"s1":{"heading":"H","occurrence":1.5}},"a2ui":[{}]}`, `Source "s1": "occurrence" must be a whole number from 1.`)
	assertValidationError(t, `{"version":2,"sources":{"s1":{"heading":"H","hash":123}},"a2ui":[{}]}`, `Source "s1": "hash" must be a string.`)
}

func TestValidateGeneratedViewRejectsComponentPropertyViolations(t *testing.T) {
	cases := []struct {
		name      string
		component map[string]any
		want      []string
	}{
		{
			name:      "MetricGrid missing metric value",
			component: map[string]any{"id": "root", "component": "MetricGrid", "metrics": []any{map[string]any{"label": "Size"}}},
			want:      []string{"'MetricGrid' (root)", "metrics.0.value: Expected string, received undefined"},
		},
		{
			name:      "MetricGrid invalid trend enum",
			component: map[string]any{"id": "root", "component": "MetricGrid", "metrics": []any{map[string]any{"label": "Size", "value": "1", "trend": "sideways"}}},
			want:      []string{"'MetricGrid' (root)", "metrics.0.trend: Invalid enum value. Expected up | down | flat, received 'sideways'"},
		},
		{
			name:      "ComparisonTable row cell count mismatch",
			component: map[string]any{"id": "root", "component": "ComparisonTable", "columns": []string{"A", "B"}, "rows": []any{[]string{"1", "2"}, []string{"3"}}},
			want:      []string{"'ComparisonTable' (root)", "Row 2 has 1 cells; there are 2 columns."},
		},
		{
			name:      "ComparisonTable fewer than 2 columns",
			component: map[string]any{"id": "root", "component": "ComparisonTable", "columns": []string{"OnlyOne"}, "rows": []any{[]string{"1"}}},
			want:      []string{"'ComparisonTable' (root)", "columns: Array must contain at least 2 element"},
		},
		{
			name:      "Timeline empty events",
			component: map[string]any{"id": "root", "component": "Timeline", "events": []any{}},
			want:      []string{"'Timeline' (root)", "events: Array must contain at least 1 element"},
		},
		{
			name:      "Checklist unrecognized item key",
			component: map[string]any{"id": "root", "component": "Checklist", "items": []any{map[string]any{"text": "x", "checked": true}}},
			want:      []string{"'Checklist' (root)", "'checked'"},
		},
		{
			name:      "KeyPoints empty items",
			component: map[string]any{"id": "root", "component": "KeyPoints", "items": []any{}},
			want:      []string{"'KeyPoints' (root)", "items: Array must contain at least 1 element"},
		},
		{
			name:      "KeyPoints non-positive weight",
			component: map[string]any{"id": "root", "component": "KeyPoints", "items": []string{"one"}, "weight": 0},
			want:      []string{"'KeyPoints' (root)", "weight: Number must be greater than 0"},
		},
		{
			name:      "Text missing text",
			component: map[string]any{"id": "root", "component": "Text"},
			want:      []string{"'Text' (root)", "text: Expected string, received undefined"},
		},
		{
			name:      "Text invalid variant",
			component: map[string]any{"id": "root", "component": "Text", "text": "hello", "variant": "huge"},
			want:      []string{"'Text' (root)", "variant: Invalid enum value. Expected caption | body, received 'huge'"},
		},
		{
			name:      "Card with children instead of child",
			component: map[string]any{"id": "root", "component": "Card", "children": []string{"a"}},
			want:      []string{"'Card' (root)", "Unrecognized key(s) in object: 'children'"},
		},
		{
			name:      "Tabs empty tabs array",
			component: map[string]any{"id": "root", "component": "Tabs", "tabs": []any{}},
			want:      []string{"'Tabs' (root)", "tabs: Array must contain at least 1 element"},
		},
		{
			name:      "Column invalid justify",
			component: map[string]any{"id": "root", "component": "Column", "children": []string{}, "justify": "invalid"},
			want:      []string{"'Column' (root)", "justify: Invalid enum value"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := viewFileJSON(t, []any{createSurfaceMsg("view", []map[string]any{tc.component})}, nil)
			ve := assertValidationError(t, raw, tc.want...)
			if ve.ComponentID != "root" {
				t.Errorf("ComponentID = %q, want \"root\"", ve.ComponentID)
			}
		})
	}
}

func TestValidateGeneratedViewRejectsNestingViolations(t *testing.T) {
	kp := func(id string) map[string]any {
		return map[string]any{"id": id, "component": "KeyPoints", "items": []string{"a"}}
	}

	cases := []struct {
		name       string
		components []map[string]any
		wantCode   string
		want       string
	}{
		{
			name: "Card holding Tabs at root",
			components: []map[string]any{
				{"id": "root", "component": "Card", "child": "tabs"},
				{"id": "tabs", "component": "Tabs", "tabs": []map[string]any{{"title": "A", "child": "k"}}},
				kp("k"),
			},
			wantCode: "UNALLOWED_CHILD",
			want:     "UNALLOWED_CHILD: Container 'root' (Card) cannot contain child 'tabs' (Tabs).",
		},
		{
			name: "Card holding Tabs under Column",
			components: []map[string]any{
				{"id": "root", "component": "Column", "children": []string{"card"}},
				{"id": "card", "component": "Card", "child": "tabs"},
				{"id": "tabs", "component": "Tabs", "tabs": []map[string]any{{"title": "A tab", "child": "kp"}}},
				kp("kp"),
			},
			wantCode: "UNALLOWED_CHILD",
			want:     "UNALLOWED_CHILD: Container 'card' (Card) cannot contain child 'tabs' (Tabs).",
		},
		{
			name: "Card holding Row",
			components: []map[string]any{
				{"id": "root", "component": "Card", "child": "row"},
				{"id": "row", "component": "Row", "children": []string{"k"}},
				kp("k"),
			},
			wantCode: "UNALLOWED_CHILD",
			want:     "UNALLOWED_CHILD: Container 'root' (Card) cannot contain child 'row' (Row).",
		},
		{
			name: "Card holding Card",
			components: []map[string]any{
				{"id": "root", "component": "Card", "child": "inner"},
				{"id": "inner", "component": "Card", "child": "k"},
				kp("k"),
			},
			wantCode: "UNALLOWED_CHILD",
			want:     "UNALLOWED_CHILD: Container 'root' (Card) cannot contain child 'inner' (Card).",
		},
		{
			name: "Row holding Row",
			components: []map[string]any{
				{"id": "root", "component": "Row", "children": []string{"inner"}},
				{"id": "inner", "component": "Row", "children": []string{"k"}},
				kp("k"),
			},
			wantCode: "UNALLOWED_PARENT",
			want:     "UNALLOWED_PARENT: Component 'inner' (Row) cannot be placed under parent 'root' (Row).",
		},
		{
			name: "Row holding Tabs",
			components: []map[string]any{
				{"id": "root", "component": "Row", "children": []string{"tabs"}},
				{"id": "tabs", "component": "Tabs", "tabs": []map[string]any{{"title": "A", "child": "k"}}},
				kp("k"),
			},
			wantCode: "UNALLOWED_PARENT",
			want:     "UNALLOWED_PARENT: Component 'tabs' (Tabs) cannot be placed under parent 'root' (Row).",
		},
		{
			name: "Tabs holding Tabs",
			components: []map[string]any{
				{"id": "root", "component": "Tabs", "tabs": []map[string]any{{"title": "Outer", "child": "inner"}}},
				{"id": "inner", "component": "Tabs", "tabs": []map[string]any{{"title": "Inner", "child": "k"}}},
				kp("k"),
			},
			wantCode: "UNALLOWED_PARENT",
			want:     "UNALLOWED_PARENT: Component 'inner' (Tabs) cannot be placed under parent 'root' (Tabs).",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := viewFileJSON(t, []any{createSurfaceMsg("view", tc.components)}, nil)
			ve := assertValidationError(t, raw, tc.want)
			if ve.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", ve.Code, tc.wantCode)
			}
			if strings.Contains(ve.Message, "MgoUnknown") {
				t.Errorf("Message unexpectedly contains MgoUnknown: %q", ve.Message)
			}
		})
	}
}

func TestValidateGeneratedViewRejectsMissingReferencesCyclesOrphansAndMissingRoot(t *testing.T) {
	assertValidationError(t, viewFileJSON(t, []any{createSurfaceMsg("view", []map[string]any{
		{"id": "root", "component": "Column", "children": []string{"gone"}},
	})}, nil), "non-existent component 'gone'")

	assertValidationError(t, viewFileJSON(t, []any{createSurfaceMsg("view", []map[string]any{
		{"id": "root", "component": "Column", "children": []string{"root"}},
	})}, nil), "Circular reference", "Self-reference")

	assertValidationError(t, viewFileJSON(t, []any{createSurfaceMsg("view", []map[string]any{
		{"id": "root", "component": "Column", "children": []string{"a"}},
		{"id": "a", "component": "Column", "children": []string{"root"}},
	})}, nil), "Circular reference")

	assertValidationError(t, viewFileJSON(t, []any{createSurfaceMsg("view", []map[string]any{
		{"id": "root", "component": "Column", "children": []string{"kp"}},
		{"id": "kp", "component": "KeyPoints", "items": []string{"One."}},
		{"id": "extra", "component": "KeyPoints", "items": []string{"Two."}},
	})}, nil), "not reachable from 'root'")

	assertValidationError(t, viewFileJSON(t, []any{createSurfaceMsg("view", []map[string]any{
		{"id": "top", "component": "KeyPoints", "items": []string{"One."}},
	})}, nil), "Missing root component")

	assertValidationError(t, viewFileJSON(t, []any{createSurfaceMsg("view", []map[string]any{
		{"id": "root", "component": "KeyPoints", "items": []string{"One."}},
		{"id": "root", "component": "KeyPoints", "items": []string{"Two."}},
	})}, nil), "Duplicate component ID: 'root'")

	assertValidationError(t, viewFileJSON(t, []any{createSurfaceMsg("view", []map[string]any{
		{"id": "123invalid", "component": "KeyPoints", "items": []string{"One."}},
	})}, nil), "must be a valid UAX #31 identifier")
}

func TestValidateGeneratedViewRetiresOldChildInMergedUpdateWhileRejectingOrphanInFullView(t *testing.T) {
	baseComponents := []map[string]any{
		{"id": "root", "component": "Column", "children": []string{"card", "b"}},
		{"id": "card", "component": "Card", "child": "leaf"},
		{"id": "leaf", "component": "KeyPoints", "items": []string{"Old point."}},
		{"id": "b", "component": "KeyPoints", "items": []string{"B."}},
	}
	merged := viewFileJSON(t, []any{
		createSurfaceMsg("view", baseComponents),
		map[string]any{
			"version": "1.0",
			"updateComponents": map[string]any{
				"surfaceId": "view",
				"components": []map[string]any{
					{"id": "card", "component": "KeyPoints", "items": []string{"Flattened."}},
				},
			},
		},
	}, nil)
	if err := ValidateGeneratedView([]byte(merged)); err != nil {
		t.Fatalf("ValidateGeneratedView(merged with retired child) = %v, want nil", err)
	}

	orphaned := viewFileJSON(t, []any{createSurfaceMsg("view", []map[string]any{
		{"id": "root", "component": "Column", "children": []string{"card", "b"}},
		{"id": "card", "component": "KeyPoints", "items": []string{"A."}},
		{"id": "b", "component": "KeyPoints", "items": []string{"B."}},
		{"id": "leaf", "component": "KeyPoints", "items": []string{"Old point."}},
	})}, nil)
	assertValidationError(t, orphaned, "not reachable from 'root'")
}

func TestValidateGeneratedViewRejectsUnknownComponentsAndMissingSources(t *testing.T) {
	unknownRaw := viewFileJSON(t, []any{createSurfaceMsg("view", []map[string]any{
		{"id": "root", "component": "FancyBox"},
	})}, nil)
	assertValidationError(t, unknownRaw, "'FancyBox'", "not a component this app can show")

	missingSrcRaw := viewFileJSON(t, []any{createSurfaceMsg("view", []map[string]any{
		{"id": "root", "component": "Column", "children": []string{"kp"}},
		{"id": "kp", "component": "KeyPoints", "title": "Points", "items": []string{"One."}, "sources": []string{"overview"}},
	})}, nil)
	assertValidationError(t, missingSrcRaw, "source", "overview", "does not have")
}

func TestValidateGeneratedViewRejectsUnsupportedMessagesForeignVersionAndMultipleSurfaces(t *testing.T) {
	validComps := []map[string]any{
		{"id": "root", "component": "Column", "children": []string{"kp"}},
		{"id": "kp", "component": "KeyPoints", "title": "Points", "items": []string{"One."}},
	}
	one := createSurfaceMsg("view", validComps)

	assertValidationError(t, viewFileJSON(t, []any{
		one,
		map[string]any{"version": "1.0", "deleteSurface": map[string]any{"surfaceId": "view"}},
	}, nil), "not permitted")

	assertValidationError(t, viewFileJSON(t, []any{
		one,
		map[string]any{"version": "1.0", "updateDataModel": map[string]any{"surfaceId": "view", "value": map[string]any{}}},
	}, nil), "not permitted")

	assertValidationError(t, viewFileJSON(t, []any{
		map[string]any{
			"version": "0.9",
			"createSurface": map[string]any{
				"surfaceId":  "view",
				"catalogId":  CatalogID,
				"components": validComps,
			},
		},
	}, nil), "does not match expected target version")

	assertValidationError(t, viewFileJSON(t, []any{
		createSurfaceMsg("one", validComps),
		createSurfaceMsg("two", validComps),
	}, nil), "exactly one surface")

	assertValidationError(t, viewFileJSON(t, []any{
		map[string]any{
			"version": "1.0",
			"createSurface": map[string]any{
				"surfaceId":  "view",
				"catalogId":  "other.dev:catalog/v1",
				"components": validComps,
			},
		},
	}, nil), "Catalog not found: other.dev:catalog/v1")
}

func TestValidateCandidateContextAndFailureWrapping(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ValidateCandidate(ctx, []byte(validFullView)); !errors.Is(err, ErrCancelled) {
		t.Fatalf("ValidateCandidate(cancelled) = %v, want ErrCancelled", err)
	}

	if err := ValidateCandidate(context.Background(), []byte(validFullView)); err != nil {
		t.Fatalf("ValidateCandidate(validFullView) = %v, want nil", err)
	}

	err := ValidateCandidate(context.Background(), []byte("not json"))
	var ge *GenError
	if !errors.As(err, &ge) {
		t.Fatalf("ValidateCandidate(invalid) = %T (%v), want *GenError", err, err)
	}
	if ge.Kind != FailInvalidOutput || ge.Hint != HintInvalidOutput || !strings.HasPrefix(ge.Message, "The generated view is not valid: ") {
		t.Errorf("GenError = %+v, want FailInvalidOutput with HintInvalidOutput", ge)
	}
}
