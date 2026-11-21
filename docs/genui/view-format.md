# GenUI view file

For `report.md`, write `.report.md.genui.json` beside the original. The app watches the file and reads it as a derived view. The host validates generated output before committing it.

- Set `version` to `2` and put the A2UI 1.0 messages in `a2ui`. Create one surface using `markgogo.dev:genui/v1`, with a tree rooted at `root`. Use identifier names such as `overviewCard` for component IDs. Follow [the file parser](viewFile.ts), [catalog definitions](a2ui.tsx) and [component schemas](components.tsx).
- For a source reference, use the plain text of its heading, including the numbering and punctuation. Set `occurrence` from `1` for repeated headings. Components reference these entries by ID. The generator computes hashes from source sections, excluding reader notes. See [source matching](sources.ts) and [section extraction](../../markdown/sections.ts).

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

## Version 3

Version 3 keeps the A2UI envelope and writes each entry of `components` as one [OpenUI Lang 1.0](../../internal/openuilang/) statement. The reader still opens version 2 files.

- Set `version` to `3`, `language` to `openui-lang/1.0` and the surface's `catalogId` to `markgogo.dev:genui/v3`. The first message is `createSurface`; later `updateComponents` messages replace statements by name.
- A statement is `name = Component(arguments)` with arguments by position, and the tree starts at `root`. [The catalog](../../internal/nativegenui/catalog_v3.json) lists each component's properties in order. New properties are only added at the end.
- Wrap content in `Source(["sourceId"], [children])` to attribute it to entries of `sources`.

```json
{
  "version": 3,
  "language": "openui-lang/1.0",
  "generatedAt": "2026-11-21T09:30:00Z",
  "sources": {
    "overview": { "heading": "Overview" },
    "metrics": { "heading": "Metrics" },
    "history": { "heading": "History" }
  },
  "a2ui": [
    {
      "version": "1.0",
      "createSurface": {
        "surfaceId": "view",
        "catalogId": "markgogo.dev:genui/v3",
        "components": [
          "root = Col([header, lead, Divider(), points, figures, history, notes], 16)",
          "header = Row([Title(\"Block-preserving edits\", \"xl\"), Spacer(), Badge(\"Stable\", \"success\")], 8, \"center\")",
          "lead = Source([\"overview\"], [Text(\"Edits keep untouched blocks byte for byte.\", \"lg\")])",
          "points = Grid([inShort, proof], 2)",
          "inShort = Card([Label(\"In short\"), List([\"Untouched blocks keep their original Markdown bytes.\", \"Only the edited block is written again.\"])])",
          "proof = Card([Label(\"Check\"), Code(\"ok := bytes.Equal(before, after)\", \"go\"), Text([\"See \", Link(\"CommonMark\", \"https://commonmark.org\")], \"sm\")], \"soft\")",
          "figures = Source([\"metrics\"], [Box([Text(\"12 documents, 100% round trips\", \"md\", \"\", \"semibold\"), Caption(\"Figures are from the March run.\")], \"col\", 4, 12, \"surface\", \"md\")])",
          "history = Source([\"history\"], [List([ListItem(\"First prototype\", \"Editing in Markdown Preview only.\", \"March\"), ListItem(\"GenUI view mode\", \"\", \"November\")], \"none\", true)])",
          "notes = Col([Markdown(\"### Design notes\\n\\nBlocks are compared **before** writing.\"), Quote(\"What the reader sees is what is saved.\", \"Design review\")])"
        ]
      }
    }
  ]
}
```
