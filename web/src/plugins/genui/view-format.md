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
that text's UTF-8 bytes.

Components name the sources they draw on in their `sources` property, a list
of ids from this table.

Each component with sources shows where it came from, and is marked when its
source moved on since the view was made: **Source changed** when a section's
hash no longer matches, **Source not found** when a heading is gone (this wins
over changed). A source without a `hash` is never marked. **Edit in source**
selects the component's first source section that is still in the document in
the editor.

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
reference to no component, a missing `root`) makes the whole view one error.

## Components

Layout and text come from the A2UI basic catalog, with its properties:

- `Column`, `Row`: `children` (list of component ids), optional `justify`,
  `align`, `weight`.
- `Tabs`: `tabs`, a list of `{ "title": string, "child": id }`.
- `Card`: `child` (one component id).
- `Text`: `text`, optional `variant` (`h1`–`h5`, `caption`, `body`). Body
  text is Markdown: bold, italic, code, lists and quotes are shown; links show
  as their text only, images and HTML are not shown.

Mark Go Go's own components hold content. Each takes optional `sources` (list
of source ids) and `weight` (share of a `Row`'s width) and has no children.

### `KeyPoints`

The few points a reader should take away.

- `title` (string, optional).
- `items` (list of strings, at least one): the points, as plain text.

## Example

For a document with the headings `Overview`, `Metrics` and `Next steps`:

```json
{
  "version": 2,
  "generatedAt": "2026-11-06T09:30:00Z",
  "model": "gemini-3.8-flash",
  "sources": {
    "overview": { "heading": "Overview" },
    "metrics": { "heading": "Metrics" },
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
          { "id": "summary", "component": "Column", "children": ["lead", "points"] },
          { "id": "lead", "component": "Text", "text": "Edits keep **untouched blocks** byte for byte." },
          { "id": "points", "component": "Row", "children": ["overviewCard", "nextCard"] },
          { "id": "overviewCard", "component": "Card", "child": "overviewPoints" },
          { "id": "overviewPoints", "component": "KeyPoints", "title": "In short",
            "items": ["Untouched blocks keep their original Markdown bytes.", "Only the edited block is written again."],
            "sources": ["overview"], "weight": 1 },
          { "id": "nextCard", "component": "Card", "child": "nextPoints" },
          { "id": "nextPoints", "component": "KeyPoints", "title": "Next",
            "items": ["Open a real file through the host."], "sources": ["next"], "weight": 1 },
          { "id": "details", "component": "KeyPoints", "title": "Figures",
            "items": ["12 documents edited", "100% round trips"], "sources": ["metrics"] }
        ]
      }
    }
  ]
}
```
