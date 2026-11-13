# Editor typeset — round-1 summary

Measured by `measure.mjs` against the real app in the bench harness
(`web/bench/typeset.html`), viewport 1440×900 @2×. `baseline` = before any
round-1 product change; `after`/`final` = shipped state (run `final`
post-closure; identical table metrics to `after`). Raw output:
`docs/research/editor-typeset/results/<label>/<engine>/` (gitignored).

## Tables — baseline → final (height / median body row / rows per 600px)

| table | Chromium | WebKit |
|---|---|---|
| tech T1 6×3 | 367/61/9.84 → 204/34/17.65 | same |
| tech T2 13×6 | 1130/89/6.74 → 442/34/17.65 | same |
| tech T3 9×9 | 2202/257/2.33, squeezed to 624px → 680/78/7.69, 1163px, h-scrolls | same |
| tech T4 7×3 | 1072/173/3.47 → 392/56/10.71 | 1016/159/3.77, 2 clipped → 414/56/10.71, 0 clipped |
| tech T5 7×4 | 686/104/5.77 → 349/56/10.71 | same |
| paper 5×4 | 306/61/9.84 → 170/34/17.65 | same |
| llm T1 9×5 | 634/61/9.84 → 306/34/17.65 | same |
| llm T2 4×4 | 273/61/9.84 → 136/34/17.65 | same |

Clipped cells: 0 everywhere after; WebKit tech T4 had 2/21 clipped before.
Only tech T3 scrolls horizontally (`scrollLeftAfter100 = 100` in both engines).

## Document totals (ProseMirror scrollHeight)

| fixture | Chromium | WebKit |
|---|---|---|
| paper-bilingual | 5844 → 5604 | 5786 → 5546 |
| tech-report-tables | 7736 → 4306 | 7679 → 4328 |
| llm-research-report | 8927 → 7678 | 8905 → 7678 |

## Rhythm — medians that changed (identical in both engines)

`h4→p` 8→4 · `h4→list` 8→4 · `h4→code-block` 8→4 · `p→h4` 28→24 ·
`list→h4` 28→24 · `code-block→h4` 28→24 · `hr→h1` 32→48 · widget→h1 0→48.
Unchanged: `p→p` 16, `h2→p` 12, `p→h2` 40, `li→li` 4, `p→table` 16,
`table→p` 20.

## Round-trip

`getMarkdown()` byte-identical to the fixture for all 3 docs × both engines,
baseline and final (load unchanged — fidelity concerns only edits; see A below).

## Interaction probes (final, both engines unless noted)

- Table click: one click into a cell → collapsed `TextSelection` at the click
  point (`clicksForCollapsedCaret: 1`); typing inserts instead of replacing the
  cell. Baseline needed the same click but selected the whole cell paragraph
  (NodeSelection), so typing/IME replaced it — fixed by the #2495 filter (B).
- IME (Chromium CDP `Input.imeSetComposition`): composition into a T4 cell
  succeeds, no toolbar flash during composition, one ⌘Z removes it.
  WebKit: BLOCKED — no CDP IME API.
- Toolbar drag (WebKit): `mgo-pointer-selecting` on all sampled frames,
  toolbar never *visible* mid-drag, first visible +2ms after mouseup at
  189×36px (items 28×28). Chromium: drag still produces no selection
  (pre-existing engine behavior); pointer class still applied on all frames.
- Toolbar overlap with the line above: 18px — inherent to Crepe's
  top-placement; Escape dismissal added in round-1 closure (C).
- Code block: placeholder height == mounted height; teardown probe —
  `refTop`/`scrollTop`/`blockHeight` deltas all 0 after TEARDOWN_DELAY both
  engines.

## Round-1 closure results (A–D)

- **A Undo fidelity**: `composeWithRanges` falls back to a structural match
  against `loaded` origins when the identity WeakMap misses (undo re-creates
  equal node objects). Edit cell → undo → markdown byte-identical; redo →
  edit; undo → identical. Same for paragraphs; also verified in a full App
  flow where the host's `edited` flag clears after undo.
- **B Table selection**: `filterTransaction` drops any `!docChanged &&
  selectionSet` transaction whose selection is a `NodeSelection` on a
  paragraph inside `table_cell`/`table_header` (Milkdown #2495 — the deferred
  dispatch can also clobber a newer selection). Verified: held-rAF click in
  cell A + dispatched TextSelection in B stays in B; click+type inserts;
  double-click on text selects the word; in-cell drag selects text;
  cross-cell drag produces a collapsed `TextSelection` — same as baseline
  (Crepe's `stopEvent` swallows the mousedown before prosemirror-tables can
  start a cell-selection drag, so CellSelection-by-drag never worked here).
  Column/row selection uses the handle buttons (real `CellSelection`,
  confirmed in E).
- **C Toolbar Escape**: non-empty selection + Esc → `mgo-toolbar-dismissed`
  hides the toolbar; next selection change or pointerdown clears it.
- **D Wide-table controls**: `.table-wrapper` is the positioned scroll
  container (20px padding ring + equal negative margin keeps the block rhythm
  and hides nothing); the add-row button is `position: sticky` on the x-line
  so it stays at the visible left edge at any `scrollLeft`. Verified on T2/T3
  at scrollLeft 0/200/max in both engines: line handles + add buttons at the
  hovered boundary, centres hit-testable, clicks add the row/column at the
  right index, col/row drag handles track the hovered cell, drag preview
  clamped inside the visible box. When a column is half scrolled out, its
  col-drag-handle tracks the cell's viewport position and straddles the
  wrapper's left edge inside the 20px padding ring.

## Regression checks (E) — both engines

- Column resize: N/A — `columnResizingPlugin` is defined in preset-gfm but not
  registered; no `.column-resize-handle`, no `col-resize` cursor at borders.
- CellSelection copy → paste into empty paragraph: PASS (single-column GFM
  table emitted).
- Whole-table copy (paragraph above → below) → paste: PASS (table node
  round-trips, `copiedHasTable`).
- External `text/html` table paste: PASS → valid GFM table
  (`| A | B | / | :- | :- | / | 1 | 2 | / | 3 | 4 |`).
- Plain-text paste into a cell: PASS.
- Selection visuals: `cellselection-{engine}.png` +
  `cellselection-zoom-{engine}.png` under
  `docs/research/editor-typeset/results/e-checks/` — `selectedCell` outline
  renders correctly with the 0-left-padding first column.

## Table variants (experiment; V2 `rules` shipped)

| table | after-r1 | V1 compact | V2 rules | V3 dense |
|---|---|---|---|---|
| tech T3 | 1474/171, squeezed | 1834/211, squeezed | 680/78, h-scrolls | 592/68, h-scrolls |
| tech T2 | 772/61 | 786/57 | 442/34 | 364/28 |
| tech T1 | 235/39 | 211/35 | 204/34 | 168/28 |

V1 regressed the wide table; V3's 13px cell text was judged too small for
bilingual body text. V2 was ported to `web/src/editor/crepe-theme.css`.
