# Round 2 — Editorial UI research

Design-review document. Nothing here touches production UI: every variant
lives in `variants/` and only applies through the dev bench
(`?doc=<name>&variant=<name>`). Round-1 results are unchanged.

## Method

- Harness: `measure.mjs`, extended with `--docs`, a wrap section (per-line
  widths → rag, widows, token overflow), a composition section (block edges
  vs the text column), and a typing-reflow probe (per-keystroke line count,
  caret rect, scroll drift).
- Conditions: 1440×900, DSF 2, dev-server bench rendering the real `App`.
  Both engines measured — WebKit (primary, matches WKWebView) and Chromium
  (dev/test runtime).
- Docs: `paper-english` (EN paper), `paper-chinese` (ZH paper),
  `tech-report-tables` (bilingual tech report), `llm-research-report`
  (LLM long report) — the two papers are new fixtures.
- Editing task: type 25 chars mid-paragraph, sample per keystroke, delete
  and verify byte-identical restore.

## Findings

### 1. Text wrapping — the biggest measured win, WebKit only

| Metric (body paragraphs) | Baseline | `pretty` (WebKit) | `pretty` (Chromium) |
|---|---|---|---|
| Widows (`lastLine < 25%`) | EN 5/21 · ZH 9/18 · tech 3–4/9 · LLM 5/16 | **0/21 · 0/18 · 0/9 · 0/16** | unchanged |
| Median last-line fill | EN 0.55 · ZH 0.26 | **EN 0.91 · ZH 0.77** | unchanged |
| Doc height | 5719 (EN) | 5746 (+27, one rebalanced para grew a line) | 5751 (same) |
| Typing stability | 0 caret jumps, 0 drift | 0 / 0 | 0 / 0 |

- **WebKit's `pretty` eliminates every measured widow** and lifts last-line
  fill dramatically — the single most visible improvement of the round.
  Chromium accepts the declaration but changes nothing (its `pretty` is a
  no-op). The production runtime is WKWebView, so the benefit is real where
  it ships — but dev/CI screenshots will not show it. This divergence must
  be documented wherever visual checks run.
- `text-wrap: balance` on headings works in **both** engines: the two-line
  H1 split went 546px/208px (38% fill) → 385px/369px (96%).
- `text-wrap: stable` is a layout no-op by design and the baseline typing
  probe already shows zero caret jumps / zero scroll drift — keep it in
  mind if instability ever appears, but it buys nothing today.
- Long-token rag is structural: a 56-char inline-code identifier that fits
  on its own line strands the previous line (~240px deficit). Neither
  `overflow-wrap: anywhere` (no-op — the token fits, so no break needed)
  nor `word-break: break-all` (fragments the code box across lines —
  visually worse) fixes it. `pretty` partially rebalances around it. No
  wrap-mode change solves it; the honest answer is to leave it alone.
- `hanging-punctuation: allow-end` works in WebKit (legitimate +1–10px
  overhang past the column edge) and is **unsupported in Chromium** — an
  already-shipped engine divergence worth noting in QA docs.

### 2. Editorial rhythm — keep the scale, fix H4–H6

Measured brackets (heading top/bottom gaps, doc height EN paper):

| Variant | h1→next | h2→p / p→h2 | h3→p / p→h3 | h4→h5 | Height |
|---|---|---|---|---|---|
| compact (−30%) | 36 | 10 / 30 | 6 / 20 | 16 | 5531 (−3.3%) |
| **baseline** | 48 | 12 / 40 | 8 / 28 | 24 | 5719 |
| airy (+30%) | 60 | 16 / 52 | 10 / 36 | 28 | 5933 (+3.7%) |

- Baseline sits mid-bracket with above/below ratios of 2.4–3.5 — the
  strongest binding per the research. Compact saves only 3% height but
  squeezes h4→h5 to 16px (level boundaries blur); airy detaches h1→h2 at
  52px. **Keep the current scale.**
- Real gap found: **H3–H6 are visually indistinguishable** — all 16px/600,
  separated only by `ink` vs `ink-2` color (see
  `r2-before-heading-stack.png`). The `r2-rhythm-levels` variant keeps the
  run-in spacing and differentiates by *style* — H4 italic, H5 italic
  muted, H6 light + letterspaced — producing a readable level ladder at
  zero height cost (`r2-levels-heading-stack.png`).

### 3. Document composition — already unified; bleed rejected

- Every top-level block kind already lands at leftDelta = 0 / rightDelta = 0
  against the 624px text column (p, h1–h6, blockquote, table block, code
  block, image). The figure renders at 624 (natural 640, `max-width: 100%`).
- `r2-compose-bleed` (code/table −16px/+16px) applied correctly but the
  visual gain is marginal — mostly extra code-chrome width — at the cost
  of breaking the single continuous left edge. **Not recommended.**

### 4. Editing stability

- `typingReflow` across all docs and all variants: 0 caret jumps, 0 scroll
  drift, byte-identical restore. Rebalanced (`pretty`) paragraphs introduce
  no measurable instability — WebKit rebalance happens without moving the
  caret line.
- All Round-1 probes still pass (one-click caret, toolbar drag/Escape,
  IME via CDP, code-block placeholder↔mount, wide-table scroll).

## Recommendations (for design review; not implemented)

1. **`text-wrap: pretty` on `.ProseMirror`** — the headline improvement;
   WKWebView-only benefit, zero cost elsewhere.
2. **`text-wrap: balance` on h1–h6** — works in both engines; only affects
   multi-line headings.
3. **H4–H6 style differentiation** per `r2-rhythm-levels` — H4 italic ink,
   H5 italic ink-2, H6 500 + 0.04em ink-3 — no size/spacing change.
4. **Keep**: baseline rhythm scale, strict column-edge alignment,
   `overflow-wrap: break-word` (no `anywhere`/`break-all` on code),
   `hanging-punctuation: allow-end` (WebKit progressive enhancement).
5. **Document, don't fix**: Chromium `pretty` no-op and unsupported
   `hanging-punctuation` — dev/CI screenshots will differ from production;
   treat as known engine divergence.

## Files

- `fixtures/paper-english.md`, `fixtures/paper-chinese.md` — new.
- `variants/r2-*.css` — 10 experiment stylesheets (5 wrap, 3 rhythm,
  1 composition, plus levels refinement and breakcode).
- `measure.mjs` — `--docs` flag, wrap/composition sections, typing-reflow
  probe (additive; Round-1 commands still work).
- `screenshots/r2-*.png` — curated before/after captures.
- Raw results: `docs/research/editor-typeset/results/` (gitignored).

---

## Implementation addendum (design-review gate)

These measurements describe the Round 2 changes already in the editor. The
later table layout gives tables more room while keeping the 624px prose limit.

Approved scope applied to `web/src/editor/crepe-theme.css` only (+36/−2):

1. **`text-wrap: pretty`** on `.ProseMirror p` (all prose paragraphs — top-level,
   blockquote, list-item), with `:is(td, th) > p { text-wrap: wrap }` so table
   cells keep greedy wrapping and column layout stays exact. DOM audit: 10
   top-level / 226 cell / 4 list-item paragraph contexts in the tech report —
   cell exclusion is what preserves table layout.
2. **`text-wrap: balance`** on h1–h3 only.
3. **H4–H6 size ladder** (candidate A, chosen over same-size weight/color):
   h4 15px/600/ink, h5 14px/600/ink-2, h6 13px/600/ink-3. Shared body font,
   leading and 24/4 margins unchanged — zero added document height.

Unchanged: rhythm scale, 624px column, strict block edges,
`overflow-wrap: break-word`, `hanging-punctuation: allow-end`, fonts, Crepe
version, architecture.

### After-measurement vs `r2-baseline2` (label `r2-impl`)

| engine | doc | widows | median last-line fill | scroll Δ | typing |
|---|---|---|---|---|---|
| webkit | paper-english | 5/21 → 0/21 | 0.55 → 0.91 | +27px (1 extra line from rewrap) | restored, 0 jumps, 0 drift |
| webkit | paper-chinese | 9/18 → 0/18 | 0.29 → 0.80 | +92px (fixture h5/h6 added) | restored, 0 jumps, 0 drift |
| webkit | tech-report | 4/9 → 0/9 | 0.49 → 0.61 | 0 | restored, 0 jumps, 0 drift |
| webkit | llm-report | 5/16 → 0/16 | 0.48 → 0.70 | 0 | restored, 0 jumps, 0 drift |
| chromium | all 4 | unchanged (pretty no-op) | unchanged | ≤1px | restored, 0 jumps, 0 drift |

- H1 example: last-line fill 0.38 → 0.96 (balanced split, both engines).
- Cell exclusion verified: tech-report table heights identical (scroll Δ = 0).
- Narrow viewport (980px): column scales to 686px PM box, `pretty` on prose,
  `wrap` in cells — confirmed via computed styles.
- `roundTrip.equal = true` both engines; tables/caret/handles/code-mount/
  teardown/layout-shift probes pass; Chromium CDP IME `compositionOk`, no
  toolbar flash; 330/330 vitest, tsc clean, build clean, `go test` pass.
- Native WKWebView (dev app via `mygo dev` + live vite): text inserted into a
  real document and persisted to disk. Full interaction battery (selection,
  paste, undo, Chinese Pinyin conversion) remains **pending manual
  verification** — same status as Round 1.
