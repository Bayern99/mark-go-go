# Mark Go Go view file format

A view file is a JSON reading aid for one Markdown document: a list of
sections, each tied to one heading of the document and made of a few
components. Mark Go Go renders it next to the document; it never changes the
document itself.

The view file of `report.md` is the hidden file `.report.md.genui.json` in the
same directory. Any tool may write it; the app reloads it when it changes.

## Top level

| Field         | Type   | Required | Meaning |
|---------------|--------|----------|---------|
| `version`     | number | yes      | Always `1`. Any other value makes the whole file invalid. |
| `generatedAt` | string | no       | ISO 8601 time the view was made. Mark Go Go writes it when it generates a view. |
| `model`       | string | no       | The model that was asked to make the view. Written by Mark Go Go. |
| `sections`    | array  | yes      | The sections, in the order to show them. |

Any other top-level field is ignored.

## Sections

| Field        | Type   | Required | Meaning |
|--------------|--------|----------|---------|
| `heading`    | string | yes      | The heading's plain text exactly as it reads in the document, without `#` marks or inline formatting (`## **Q3** results` → `Q3 results`). Matching ignores case and repeated spaces. |
| `occurrence` | number | no       | Which heading with that text, counting from 1 in document order, when the document repeats a heading. Default `1`. |
| `sourceHash` | string | no       | Hash of the section's source text when the view was made. Written by Mark Go Go; leave it out. |
| `blocks`     | array  | yes      | The section's components, in display order. |

A section's source text runs from its heading to the next heading of the same
or a higher level, or the end of the document, so it includes its
sub-sections. `sourceHash` is `fnv1a32:` followed by the 8 lowercase hex
digits of the 32-bit FNV-1a hash of that text's UTF-8 bytes. The app compares
it with the current text: a section whose text changed is shown as changed, a
section whose heading is gone as not found.

## Components

Every block is an object with a `component` field naming one of the three
components below. All text is plain text: Markdown, HTML and scripts are shown
literally, never interpreted. A block with any other `component` is not
rendered and is reported as an error in its section.

### `Summary`

Short points that sum up the section.

- `title` (string, optional): defaults to `Summary`.
- `items` (array, required): objects with `text` (string, required).

### `KeyValueTable`

Facts that pair a name with a value: figures, dates, owners, decisions.

- `title` (string, optional): defaults to `Key values`.
- `rows` (array, required): objects with `key` and `value` (both strings, required).

### `Checklist`

Actions, requirements or open questions, and whether each is done.

- `title` (string, optional): defaults to `Checklist`.
- `items` (array, required): objects with `label` (string, required) and
  `checked` (boolean, optional, default `false`).

An item or row that misses a required field, or has one of the wrong type, is
left out and reported as an error in its section; the rest of the view still
renders. A block without its required array is not shown and is reported the
same way. A section whose `heading` is missing or whose `occurrence` is not a
whole number from 1 still shows its blocks, but is not matched to the
document: it cannot be revealed and its freshness is not checked.

## Example

For a document with the headings `Overview`, `Metrics` and `Next steps`:

```json
{
  "version": 1,
  "generatedAt": "2026-11-06T09:30:00Z",
  "model": "gemini-3.8-flash",
  "sections": [
    {
      "heading": "Overview",
      "blocks": [
        {
          "component": "Summary",
          "title": "In short",
          "items": [
            { "text": "Untouched blocks keep their original Markdown bytes." },
            { "text": "Only the edited block is written again." }
          ]
        }
      ]
    },
    {
      "heading": "Metrics",
      "occurrence": 1,
      "blocks": [
        {
          "component": "KeyValueTable",
          "title": "Figures",
          "rows": [
            { "key": "Documents edited", "value": "12" },
            { "key": "Round trips", "value": "100%" }
          ]
        }
      ]
    },
    {
      "heading": "Next steps",
      "blocks": [
        {
          "component": "Checklist",
          "title": "Actions",
          "items": [
            { "label": "Open a real file through the host", "checked": true },
            { "label": "Scroll to a heading from the panel" }
          ]
        }
      ]
    }
  ]
}
```
