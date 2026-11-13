# Round 3 — Direct Editing Interaction Audit

Scope: interaction audit only — no production changes. Evidence gathered with
`docs/editor-typeset/r3-probe.mjs` (Playwright webkit+chromium against the
bench app), existing vitest coverage, and upstream source inspection
(Crepe 7.23.0, @milkdown/plugin-tooltip, prosemirror-view, prosemirror-tables).

## 1. Interaction architecture

Event path inside `.ProseMirror` (prosemirror-view):

```
DOM event on view.dom
└─ eventBelongsToView()        // walks target→dom; any NodeView stopEvent → swallowed
   └─ runCustomHandler()       // plugin handleDOMEvents (only if gate passed for
      │                        // events PM listens to, e.g. mousedown)
      └─ built-in editHandlers // incl. prosemirror-tables mousedown/CellSelection
```

Selection toolbar (Crepe `ToolbarView` → `@milkdown/plugin-tooltip`):

- `TooltipProvider` hardcoded: `debounce: 20`, `offset: 10`,
  `placement: "top"`, middleware `[flip(), offset(10), shift()]` — **not
  configurable through Crepe's feature config** (config only controls buttons).
- Reference rect = `posToDOMRect(view, from, to)` — union of all line boxes.
- Element appended to `view.dom.parentElement` (inside the scroll viewport) →
  tracks content on scroll. Verified: +60px scroll moved toolbar exactly 60px
  with the selection, gap stayed 10px. No stale-position failure exists.
- `shouldShow` requires non-empty TextSelection + editor focus; `composing`
  gate hides during IME composition (verified).

Table cell clicks (`TableNodeView.stopEvent` in @milkdown/components):

- mousedown in a cell → `handleClick`: same-cell TextSelection → `false`
  (caret/text flows to PM — Round 1 behavior preserved); otherwise → swallows
  the event (`true`) and schedules a paragraph `NodeSelection` in the cell.
- Because the gate runs before PM's handlers, **mousedown never reaches
  prosemirror-tables** → cross-cell drag can never start a CellSelection.

## 2. Confirmed issues

### Issue A — toolbar covers preceding text (~18px) — CONFIRMED, upstream-inherent

Measured (webkit, 1440×900): toolbar `[selTop−46, selTop−10]`, line height 28px
→ the line box above the selection's first line loses exactly 18px
(36px height + 10px offset − 28px leading). Reproduced in EN and ZH docs.
**Mechanism is geometric, not positioning failure**: any 36+10px band above a
selection necessarily lands inside an adjacent line when text is present. This
is baseline Crepe/Floating-UI behavior identical to Notion/Confluence.

Fix comparison:
- **Accept + Escape mitigation** (current): zero risk. Escape handler and
  drag-time `visibility:hidden` already exist and are verified working.
- Offset change: redistributes the 18px to a *different* line (proven: offset 20
  covers 8px of prev line but all 28px of line-2) — no net win.
- Custom toolbar feature (~100 lines) to expose `floatingUIOptions`:
  side-margin placement avoids covering text entirely but changes UX shape;
  hardcoded provider makes this a feature replacement, not a patch.
- **Recommendation: P2 — accept as upstream-inherent.** If design later wants
  zero-overlap, the viable route is a custom tooltip view (side placement or
  end-of-selection anchor), not offset tuning.

### Issue B — cross-cell drag yields no CellSelection — CONFIRMED, Crepe baseline

Runtime trace during a 3-cell horizontal drag: selection never left
`TextSelection:459–461` — no CellSelection, no extension. Root cause:
`TableNodeView.stopEvent` returns `true` for mousedown in a cell whose
selection isn't already inside it, to implement click-selects-paragraph. The
swallow occurs in `eventBelongsToView`, upstream of PM's mousedown handling,
so `prosemirror-tables`' drag watcher never arms. Not a regression from
Round-1 fixes — the filter only drops *stale* selection-only NodeSelections.

Fix comparison:
- **Raw DOM drag watcher (recommended, ~60–80 lines)**: `view.dom` listener in
  our own plugin records a td/th mousedown; on mousemove detect cell-boundary
  crossing via `posAtCoords` + `cellAround`/`TableMap`; dispatch
  `CellSelection.create(doc, anchor, head)`. Event path is unaffected by
  `stopEvent` (raw listeners bypass PM's dispatch). `.selectedCell` styles
  already exist in Crepe's theme. Preserves single-click caret (same-cell
  returns false path untouched) and Crepe's click→NodeSelection.
- Fork/patch Crepe's table NodeView: heavier, forks upstream.
- `handleDOMEvents.mousedown` plugin: **doesn't work** — the stopEvent gate
  precedes it for events PM listens to.
- **Recommendation: P1** — completes a Word-familiar gesture with a small
  isolated plugin; verify with existing `editor-typeset.test.ts` drag test
  (extend to assert CellSelection across the boundary) + manual check.

### Issue C — native input/IME — automated coverage extended, manual gate stands

New CDP checks (Chromium): composition start in H2 (`composing:true`,
TextSelection kept), candidate commit ("测试" inserted, focus retained),
single-unit undo restores pre-commit text, toolbar stays hidden during
composition. Existing measure.mjs cell-IME probe also passes
(`compositionOk`, no toolbar flash). WebKit has no automation API for IME;
**real macOS Pinyin `zhongwen`→`中文` remains PENDING MANUAL VERIFICATION** —
same status as Round 1/2.

## 3. Verified-working behaviors (no issues)

- Click → caret; double-click → word ("line"); triple-click → paragraph.
- Shift+Arrow selection; Cmd+A → `AllSelection` over full doc.
- Toolbar button click: editor keeps focus, TextSelection preserved, typing
  continues immediately, formatting applied.
- Toolbar hidden during drag via `visibility:hidden` (R1 mitigation verified —
  `data-show` stays "true" internally, that's the provider flag, not a leak).
- Mid-scroll (scrollTop ≈ 2873) click+type: caret stable, scroll drift = 0.
- Paste → insert + focus kept; Cmd+Z removes cleanly.
- Table single-click caret, handles, code-block mount/teardown layout shifts,
  markdown round-trip — all green via existing probes; 330/330 vitest.
- Round-1 `filterTransaction` and toolbar Escape unchanged and passing.

## 4. Recommended scope

| Pri | Item | Size | Verification |
|---|---|---|---|
| P1 | CellSelection on cross-cell drag (raw-listener plugin) | ~60–80 lines in createEditor or sibling plugin | extend `editor-typeset.test.ts` drag test; manual drag check |
| P2 | Issue A — accept upstream-inherent; document | none | — |
| Manual | macOS Pinyin conversion in real app | — | user acceptance |

## 5. Residual risk notes

- The drag watcher must not fire for mousedowns on handles/buttons inside the
  table node view (Crepe stopEvent covers `button` targets; replicate that
  guard).
- Keep the fix inside our plugin layer — do not patch node_modules.
- Any future upstream Crepe fix may supersede the watcher; keep it removable.

---

# Round 3 Implementation — section folding, code collapse, cell drag, toolbar decision

Production diffs: `web/src/editor/folding.ts` (new), `web/src/editor/tableCellDrag.ts` (new),
`web/src/editor/createEditor.ts` (+49), `web/src/editor/crepe-theme.css` (+67),
`web/src/App.tsx` (+1), `web/package.json` (+prosemirror-tables 1.8.5, already the
transitively installed version — no dual-package risk), `web/test/folding.test.ts` (new),
`web/test/editor-typeset.test.ts` (expectation updated to the fixed behaviour).

## Section folding (`folding.ts`)

- Anchors are top-level **document positions** (heading or code_block node start),
  kept in a `mgo-fold` plugin field and mapped through transactions (assoc −1). A
  mark dies when `doc.nodeAt(mapped)` is no longer a foldable node — heading edits
  and moves keep the fold; deleting the heading drops it. Duplicate titles are fine
  because identity is positional, not textual.
- A section fold hides every top-level node from the anchor's end to the next
  heading of equal-or-higher level via `Decoration.node {class:'mgo-fold-hidden'}`
  (`display:none`). The heading stays visible with an inline chevron widget
  (`mgo-fold-toggle`, opacity 0 → hover/folded 1, `aria-hidden` so it does not
  pollute the heading's accessible name).
- Nested folds: children inside a folded parent are hidden by DOM cascade but keep
  their anchors — expanding the parent restores each child's own folded state.
- Keyboard: `Cmd/Ctrl-Alt-←` folds the caret's current section (or enclosing code
  block), `Cmd/Ctrl-Alt-→` expands it.
- Folding is display-only: transactions carry `addToHistory:false`, the doc is
  untouched, so Markdown bytes, undo depth and the dirty flag are all unchanged.
- Folding the section containing the caret moves the caret to the end of the
  heading (visible position) — subsequent typing lands visibly.
- `reveal()` calls `expandAt(pos)` and `select()` calls `expandRange(from,to)` —
  outline clicks, search navigation and programmatic selects always expand the
  folds hiding their target before landing.
- Copy/selection/delete: hidden content participates in `Cmd+A`, copy and delete
  exactly as visible content does — the DOM is `display:none`, the document is
  intact. Selections cannot *start* inside hidden DOM, so no accidental
  hidden-region operations arise.
- Persistence: fold anchors serialize as `kind:level:text#occurrence` keys into
  `localStorage['mgo.folds:'+docKey]` (docKey = `host.id`, passed from App).
  **Deliberate behavior: folds survive close/reopen** — reopening a document
  restores the sections the user chose to collapse. Keys that no longer match any
  node are dropped silently. `load()` (reload/setMarkdown) restores from the live
  session keys, so folds survive in-session reloads too.

## Whole code-block collapse

- Folding an entire block for reading (not CM's internal code folding, out of
  scope as decided).
- Anchor = the `code_block` node; node decoration adds `.mgo-code-folded` → CSS
  hides the nodeview children, `::before` renders a ▸ glyph, `::after` renders
  `code · N lines`; the whole shell is click-to-expand via a delegated listener.
- Collapse affordance: a chevron injected into Crepe's own `.tools` row. The row
  only exists after Crepe's lazy CodeMirror mount, so injection is driven by a
  MutationObserver on `view.dom` (fires on mount without needing a transaction).
- Round-1 stable height is preserved: folding swaps the nodeview for a 42px stub;
  expanding re-shows the same nodeview — no CM rebuild, no jump.

## Cross-cell drag (`tableCellDrag.ts`)

The audited root cause has *two* halves, both fixed locally:

1. `TableNodeView.stopEvent` swallows cell mousedowns before `eventBelongsToView`
   even reaches plugin `handleDOMEvents`, so prosemirror-tables' drag watcher can
   never arm. → A capture-phase `mousedown` listener on `view.dom` records the
   anchor cell; a `mousemove` listener on `window` dispatches
   `CellSelection.create(doc, anchor, target)` once the pointer crosses a cell
   boundary (and keeps updating head until mouseup).
2. Because PM's `LeftMouseDown` never starts, `delayedSelectionSync` is absent —
   the browser's native drag selection still forms and PM syncs it into a
   TextSelection, overwriting ours. → While a cross-cell drag is live
   (`crossed`), `filterTransaction` rejects any selection-set transaction that
   isn't a `CellSelection`; the flag releases ~50ms after mouseup to catch
   trailing syncs. Native highlight is cleared via `removeAllRanges`.

Untouched by design: single click (no crossing → no flag), intra-cell text drag
(flag only arms on boundary crossing), Crepe's delayed paragraph NodeSelection
(Round-1 filter still drops it), row/column controls.

Verified (WebKit + Chromium): drag → `CellSelection` with N `.selectedCell`;
copy produces a real `data-pm-slice` table slice; single click → caret;
double-click → Crepe cell selection; intra-cell drag → `TextSelection`;
blur/mouseup/destroy clean up listeners and the flag.

## Toolbar decision — keep current top placement

Measured at 1440/1100/980/800px (WebKit):

| | current (top, offset 10) | side-margin alternative |
|---|---|---|
| Text occlusion | 18px of the line box above the selection's first line | 0 |
| Mouse travel | ~30px vertical from selection edge | ~250–400px horizontal |
| Discoverability | high — inside the reading column | low — off the text edge |
| Real free margin | — | 137px @1440, ~56px @800 (the left gutter is the outline sidebar, not free space) |
| Narrow windows | fine | needs fallback to top placement |
| Implementation | Crepe provider | replaces hardcoded ToolbarView + a width-dependent fallback branch |

The 18px occlusion is real but bounded; side placement removes it only at very
wide windows, costs ~6–10× pointer travel, and adds a fallback path. Not clearly
superior → **retain current placement**; Escape-to-close, hide-during-drag and
focus continuity are preserved unchanged.
