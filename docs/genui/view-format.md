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

- Set `version` to `3`, `language` to `openui-lang/1.0` and the surface's `catalogId` to `markgogo.dev:genui/v3`. The first message is `createSurface`; later `updateComponents` messages name the same `surfaceId` and replace statements by name. Statements they leave out stay.
- A statement is `name = Component(arguments)` with arguments by position, and the tree starts at `root`. [The catalog](../../internal/genuikit/catalog.json) lists each component's properties in order. `gap` and `padding` count 4-point steps; a `Col` or `Card` without `gap` spaces its children like an article. New properties are only added at the end.
- `Table`, `Chart`, `Icon`, `Image`, `Carousel` and `Popover` show data and media. `Icon` takes a [Phosphor](https://phosphoricons.com) name or a common lucide name. `Image` shows only an image file that the original document itself shows; other addresses show the description instead.
- `Button`, `IconButton`, `Pressable`, `Form`, `Input`, `TextArea`, `Select`, `Checkbox`, `RadioGroup`, `SegmentedControl`, `Slider`, `DatePicker`, `Timer`, `Flashcard` and `Celebration` are controls. A control bound to a `$variable` changes it, and everything computed from it follows at once. What the reader changes is never written to the view file. It lasts while the view keeps its `createSurface` message, for every variable the view still declares: through an update of part of the view and through going back from one. A view with another `createSurface` message, whether generated again in full, written by a tool or restored by undoing a full generation, starts from its own values, as does a view opened again after it is closed. While part of a view is updated, a placeholder stands in for each statement being replaced and the rest stays readable; if the update fails or is cancelled, the statement shows again as it was. `@OpenUrl` opens `http`, `https` and `mailto` addresses; `@ToAssistant` does nothing in a saved view. Inside a `Form`, an action that sends first checks each field's `rules` and shows what is wrong.
- Expressions may call the catalog's 59 functions, such as `@Format`, `@Plural`, `@GroupBy` and `@AddDays`. `@Random`, `@Pick` and `@Shuffle` give a value only inside an action. Dates without a time zone are read in the reader's time zone.
- Wrap content in `Source(["sourceId"], [children])` to attribute it to entries of `sources`.
- A generated view is accepted when it has a `root`, every `Source` names an entry of `sources`, and its faulty statements number at most two, or a tenth of its statements when that is more. An optional property holding a value the catalog does not allow, such as a `Text` size of `"xl"`, is drawn with its default and reported: the generation run's `validation.result` trace event lists the statement, component, property and message. It does not make its statement faulty. An unknown component, a missing or mistyped required property and a name that is never defined do. [The catalog](../../internal/genuikit/catalog.json) decides what is valid, and its signatures and [format text](../../internal/genuikit/format.txt) make up the generation prompt, so adding a component touches the catalog and the renderer only.
- An update of part of a view names statements to replace. It may replace those and add new ones; every other statement stays. The app then adds `name = null` to the update for each statement that `root` no longer reaches, which deletes it. `$variable` declarations are never deleted, and the earlier messages stay byte for byte.

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
          "root = Col([header, lead, Divider(), points, figures, history, notes])",
          "header = Row([Title(\"Block-preserving edits\", \"xl\"), Spacer(), Badge(\"Stable\", \"success\")])",
          "lead = Source([\"overview\"], [Text(\"Edits keep **untouched blocks** byte for byte.\", \"lg\")])",
          "points = Grid([inShort, proof], 2, 240)",
          "inShort = Card([Title(\"In short\", \"sm\"), List([\"Untouched blocks keep their original Markdown bytes.\", \"Only the edited block is written again.\"])])",
          "proof = Card([Label(\"Check\"), Code(\"ok := bytes.Equal(before, after)\", \"go\"), Text([\"See\", Link(\"CommonMark\", \"https://commonmark.org\")], \"sm\")], \"soft\")",
          "figures = Source([\"metrics\"], [Box([Text(\"12 documents, 100% round trips\", \"md\", null, \"semibold\"), Caption(\"Figures are from the March run.\")], \"col\", 1, 4, \"surface\", \"lg\")])",
          "history = Source([\"history\"], [List([ListItem(\"First prototype\", \"Editing in Markdown Preview only.\", \"March\"), ListItem(\"GenUI view mode\", \"A derived reading view.\", \"November\")], null, true)])",
          "notes = Col([Markdown(\"### Design notes\\n\\nBlocks are compared **before** writing.\"), Quote(\"What the reader sees is what is saved.\", \"Design review\")])"
        ]
      }
    }
  ]
}
```
