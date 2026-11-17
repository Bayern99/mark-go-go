# Editor typeset measurement harness

Dev-only harness + Playwright measurement script for capturing rendering and
interaction baselines of the Milkdown Crepe editor. Nothing here is part of the
production build (`vite build` only emits `index.html`; `bench/` is outside the
tsconfig `include` set). `SUMMARY.md` records the round-1 baseline/after
numbers and the regression results behind the shipped changes. `ROUND2.md`
records the round-2 editorial research (wrap modes, heading levels, block
composition) — all variants are bench-only.

## Files

- `fixtures/` — five long Markdown fixtures plus `figure-1.svg` (referenced by
  `paper-bilingual.md` and `paper-english.md`).
- `variants/` — CSS experiments: round-1 table variants (`compact-grid`,
  `rules` shipped, `dense`) and round-2 editorial variants (`r2-*`).
- `../../web/bench/typeset.html` + `typeset.tsx` — renders the real `App` with
  `createBrowserWorkspace([createBrowserHost(fixture)])`. The figure is embedded
  as a base64 data URL (image src rewritten in-memory before seeding). Exposes
  `window.__bench = { doc, host, workspace, fixture, source }` and
  `window.__pmView` (the live ProseMirror `EditorView`, captured by patching
  `EditorView.prototype.updateStateInner` in the harness only).
- `measure.mjs` — measurement script.
- `screenshots/` — representative before/after captures kept in the repo.

## Run

```sh
cd web && npx vite                 # dev server on http://localhost:5173
cd web && npx playwright install webkit   # one-time; chromium ships already

# in the repo root
node docs/editor-typeset/measure.mjs --label baseline --engine both
node docs/editor-typeset/measure.mjs --label after    --engine webkit

# table variants (results land in results/variant-<name>/):
node docs/editor-typeset/measure.mjs --variant compact-grid --engine both
node docs/editor-typeset/measure.mjs --variant rules        --engine both
node docs/editor-typeset/measure.mjs --variant dense        --engine both
```

- `--label` — subdirectory name under the results root (e.g. `baseline`,
  `final`).
- `--engine` — `webkit` (primary; the app ships in WKWebView), `chromium`, or
  `both` (default).
- `--variant` — CSS variant; the bench page injects
  `variants/<name>.css` after the app CSS.
- `--docs` — comma-separated doc list; defaults to the three round-1
  fixtures. Round 2 used
  `paper-english,paper-chinese,tech-report-tables,llm-research-report`.

Raw run output (metrics JSON + full-size screenshots) is written to
`docs/research/editor-typeset/results/<label>/<engine>/`, which is gitignored —
only the curated screenshots under `screenshots/` are tracked.

Bench pages: `http://localhost:5173/bench/typeset.html?doc=<paper-bilingual|tech-report-tables|llm-research-report>`.

Fixed conditions: viewport 1440×900, deviceScaleFactor 2, waits for
`document.fonts.ready` + 500 ms before measuring.

## Notes

- Vite dev server needs `server.fs.allow: ['..']` (set in `web/vite.config.ts`)
  so the `?raw` fixture imports outside `web/` resolve. Dev-server only.
- Composed Markdown is read back through `window.__bench.host.load()` (the
  App's own onChange → `host.update` path after its 250 ms debounce; the script
  waits 450 ms).
