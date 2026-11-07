# Mark Go Go view file format

A view file is a reading view of one Markdown document, made of components
from Mark Go Go's catalog and written as [A2UI 1.0](https://a2ui.org)
messages. Mark Go Go shows it in the GenUI view mode; it never changes the
document itself.

The view file of `report.md` is the hidden file `.report.md.genui.json` in the
same directory. Any tool may write it; the app reloads it when it changes.

## Top level

| Field         | Type   | Required | Meaning |
|---------------|--------|----------|---------|
| `version`     | number | yes      | Always `2`. A file with `1` is an older view and is not shown; any other value makes the file invalid. |
| `generatedAt` | string | no       | ISO 8601 time the view was made. Written by Mark Go Go. |
| `model`       | string | no       | The model that was asked to make the view. Written by Mark Go Go. |
| `request`     | string | no       | What the reader asked for when the view was generated. |
| `sources`     | object | no       | Source references: the parts of the document the view draws on, by id. |
| `a2ui`        | array  | yes      | A2UI 1.0 messages, applied in order. |

Any other top-level field is ignored.

## Sources

`sources` maps an id the view chooses (`"s1"`, `"metrics"`) to a section of
the document:

| Field        | Type   | Required | Meaning |
|--------------|--------|----------|---------|
| `heading`    | string | yes      | The heading's plain text exactly as it reads in the document, without `#` marks or inline formatting (`## **Q3** results` → `Q3 results`). Matching ignores case and repeated spaces. |
| `occurrence` | number | no       | Which heading with that text, counting from 1 in document order, when the document repeats a heading. Default `1`. |
| `hash`       | string | no       | Hash of the section's source text when the view was made. Written by Mark Go Go; leave it out. |

A section runs from its heading to the next heading of the same or a higher
level, or the end of the document, so it includes its sub-sections. `hash` is
`fnv1a32:` followed by the 8 lowercase hex digits of the 32-bit FNV-1a hash of
that text's UTF-8 bytes, leaving out the reader's notes.

A note is a top-level HTML comment block in the document, `<!-- note: text -->`,
optionally with `key="value"` attributes before the colon. Notes are the
reader's comments on the document, not its content: do not present them as
what the document says. For the hash, each note is left out together with the
whitespace between it and the block before it.

Components name the sources they draw on in their `sources` property, a list
of ids from this table.

Each component with sources shows where it came from, and is marked when its
source moved on since the view was made: **Source changed** when a section's
hash no longer matches, **Source not found** when a heading is gone (this wins
over changed). A source without a `hash` is never marked. **Source not found**
still shows when only some of a component's sources are gone. **Edit in source**
and **Note** then use the first of those sources that is still in the document;
they are disabled only when none is, and the frame says so. An exact quote is
selected where it still occurs in a remaining source; if it is gone, the first
remaining source section is selected. A missing heading is never replaced by
another heading that happens to share its text.

## Messages

`a2ui` holds A2UI 1.0 messages, each with `"version": "1.0"`. The first is a
`createSurface` with the whole component tree; later `updateComponents`
messages replace components by id. Other message types are not allowed.

- There is exactly one surface. Its `catalogId` is `markgogo.dev:genui/v1`.
- Components form one tree from the component with id `root`: every
  reference must name a component that exists.
- A component is an object with an `id`, a `component` naming a catalog
  component, and that component's properties, nothing else.

A component the catalog does not have is shown as an error where it is; the
rest of the view still renders. Anything else that is wrong (a message that is
not A2UI 1.0, a property of the wrong type, a missing or extra property, a
reference to no component, a missing `root`, a component placed where the
nesting rules below do not allow it) makes the whole view one error, which
names the component and the reason.

## Components

The catalog has ten components. The definition of each, with its JSON Schema
and nesting rules, is in the catalog definition Mark Go Go gives with this
document.

### Layout

Layout and text come from the A2UI basic catalog, with its properties:

- `Column`: `children` (list of component ids), optional `justify`, `align`,
  `weight`. Stacks its children.
- `Row`: the same properties. Places its children side by side while each
  still has room, and wraps them otherwise. Allowed at the top, in a
  `Column` or in a tab, not in a `Row` or a `Card`.
- `Tabs`: `tabs`, a list of `{ "title": string, "child": id }`. Allowed at the
  top or in a `Column` only.
- `Card`: `child` (one component id): a `Column`, a `Text` or a content
  component.
- `Text`: `text`, optional `variant` (`h1`–`h5`, `caption`, `body`). Body
  text is Markdown: bold, italic, code, lists and quotes are shown; links show
  as their text only, images and HTML are not shown.

### Content

Mark Go Go's own components hold the content and lay it out themselves. Each
takes an optional `title` (string), `sources` (list of source ids) and
`weight` (share of a `Row`'s width), and contains no other component. Text in
them is plain text, not Markdown.

**`KeyPoints`**: the few points a reader should take away.

- `items` (list of strings, at least one).

**`MetricGrid`**: headline figures in a grid.

- `metrics` (list, at least one) of objects with `label` (string), `value`
  (string, the figure as it should read), and optional `unit` (string),
  `change` (string, such as `"+4%"`) and `trend` (`"up"`, `"down"` or
  `"flat"`, which only colours the change).

**`ComparisonTable`**: items compared side by side. The reader can sort by
any column from the keyboard or by activating the heading (ascending,
descending, then original order); sorting changes only the display. Cells
that read as numbers sort by value.

- `columns` (list of at least two strings): the headings; the first names
  what each row compares.
- `rows` (list, at least one) of lists of strings, one cell per column.

**`Timeline`**: events or phases in order.

- `events` (list, at least one) of objects with `when` (string, such as
  `"2002"` or `"Phase 2"`), `title` (string) and optional `detail` (string).

**`Checklist`**: items and their state in the document. It is read-only:
clicking an item selects it in the document and never changes it.

- `items` (list, at least one) of objects with `text` (string), optional
  `done` (boolean, default `false`) and optional `quote` (string): text
  copied exactly from a remaining source section that clicking the item selects;
  without it the item's `text` is looked for, and failing that the first
  remaining source section is selected.

## Example

For a document with the headings `Overview`, `Metrics`, `Options`, `History`
and `Next steps`, a view that uses every component:

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
              { "when": "March", "title": "First prototype", "detail": "Rich text editing only." },
              { "when": "November", "title": "GenUI view mode" }
            ],
            "sources": ["history"] }
        ]
      }
    }
  ]
}
```
