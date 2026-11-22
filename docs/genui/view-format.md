# GenUI view file

For `report.md`, write `.report.md.genui.json` beside the original. The app watches the file and reads it as a derived view. The host validates generated output before committing it.

- Set `version` to `2` and put the A2UI 1.0 messages in `a2ui`. Create one surface using `markgogo.dev:genui/v1`, with a tree rooted at `root`. Use identifier names such as `overviewCard` for component IDs. Follow [the file parser](../../internal/genui/update.go), [catalog definitions](../../internal/nativegenui/catalog.txt) and [component schemas](../../internal/nativegenui/surface.go).
- For a source reference, use the plain text of its heading, including the numbering and punctuation. Set `occurrence` from `1` for repeated headings. Components reference these entries by ID. The generator computes hashes from source sections, excluding reader notes. See [source matching](../../internal/nativegenui/panel.go) and [section extraction](../../internal/workspace/sections.go).

## Example

For a document with the headings `Overview`, `Metrics`, `Options`, `History` and `Next steps`:

```json
{
  "version": 2,
  "generatedAt": "2026-11-06T09:30:00Z",
  "model": "gemini-3.8-flash",
  "sources": {
    "overview": { "heading": "Overview" },
    "metrics": { "heading": "Metrics" },
    "options": { "heading": "Options" },
    "history": { "heading": "History" },
    "next": { "heading": "Next steps" }
  },
  "a2ui": [
    {
      "version": "1.0",
      "createSurface": {
        "surfaceId": "view",
        "catalogId": "markgogo.dev:genui/v1",
        "components": [
          { "id": "root", "component": "Tabs", "tabs": [
            { "title": "Summary", "child": "summary" },
            { "title": "Details", "child": "details" }
          ] },
          { "id": "summary", "component": "Column", "children": ["lead", "points", "figures"] },
          { "id": "lead", "component": "Text", "text": "Edits keep **untouched blocks** byte for byte." },
          { "id": "points", "component": "Row", "children": ["overviewCard", "nextCard"] },
          { "id": "overviewCard", "component": "Card", "child": "overviewPoints", "weight": 1 },
          { "id": "overviewPoints", "component": "KeyPoints", "title": "In short",
            "items": ["Untouched blocks keep their original Markdown bytes.", "Only the edited block is written again."],
            "sources": ["overview"] },
          { "id": "nextCard", "component": "Card", "child": "nextSteps", "weight": 1 },
          { "id": "nextSteps", "component": "Checklist", "title": "Next",
            "items": [
              { "text": "Open a real file through the host.", "done": true },
              { "text": "Generate views with an agent." }
            ],
            "sources": ["next"] },
          { "id": "figures", "component": "MetricGrid", "title": "Figures",
            "metrics": [
              { "label": "Documents edited", "value": "12" },
              { "label": "Round trips", "value": "100", "unit": "%", "change": "+4%", "trend": "up" }
            ],
            "sources": ["metrics"] },
          { "id": "details", "component": "Column", "children": ["compare", "history"] },
          { "id": "compare", "component": "ComparisonTable", "title": "Options",
            "columns": ["Option", "Size (MB)", "Native"],
            "rows": [["MyGo", "12", "Yes"], ["Electron", "180", "No"], ["Tauri", "9", "Yes"]],
            "sources": ["options"] },
          { "id": "history", "component": "Timeline", "title": "History",
            "events": [
              { "when": "March", "title": "First prototype", "detail": "Editing in Markdown Preview only." },
              { "when": "November", "title": "GenUI view mode" }
            ],
            "sources": ["history"] }
        ]
      }
    }
  ]
}
```
