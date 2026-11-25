# Mark Go Go

[中文](README.md) · [Credits and licenses](ACKNOWLEDGMENTS.en.md)

Mark Go Go is a local Markdown editor for macOS written in Go with MyGo Native UI — no Electron, WebView, or Node.js runtime.

## Core capabilities

- **Byte-preserving in-place Markdown editing**: Editable **Preview** and raw **Markdown** share one text buffer and Undo/Redo history (`⌘E` toggles between them). Saving rewrites only modified blocks and preserves untouched Markdown bytes. Supports heading section folds, math, Mermaid diagrams, footnotes, in-place block editing, and resizable tables with wrapped rows.
- **GenUI derived reading views**: Generates read-only views (cards, tables, charts, and local-state controls) stored beside the document as `.<name>.genui.json`, with source links back to the original headings. Version 3 keeps the **A2UI 1.0** message envelope and section hashes for partial updates (version 2 files remain readable) while expressing `components` as **OpenUI Lang 1.0** statements parsed, evaluated, and rendered natively in Go. Switch to **GenUI** in the centre, use **Request** at the top right to generate, update changed sections, or restore the previous view, and use the gear icon to pick a model in Settings.
- **Multi-workspace window & navigation**: Folder workspaces share one window and vertical document tabs (`Ctrl+Tab` cycles recent documents). The sidebar provides **Files / Outline** switching, zero-indent directory drill-down, and top-aligned outline jumps. `⌘K` / `⌘T` opens the Command Bar for open documents, workspace files, headings, and commands; `⌘,` opens the unified Settings Center. Each workspace can choose Light/Dark appearance and a colour theme (from the Wu Xing traditional palette) to tell workspaces apart.

## Shortcuts

| Shortcut | Action |
|---|---|
| `⌘K` / `⌘T` | Open Command Bar (documents, files, headings, commands) |
| `⌘E` | Switch between Preview and Markdown source |
| `⌘0` | Hide or show left sidebar (drawer in narrow windows) |
| `⌥⌘0` | Hide or show right Feynman panel |
| `⌘,` | Open Settings Center |
| `Ctrl+Tab` | Switch between recent documents |

## Optional plugins

Enable **Feynman**, **GenUI**, and **Git** independently from **Settings / Plugins** (`⌘,`) or the Command Bar (`⌘K`). Feynman and Git start disabled; GenUI starts enabled, and turning it off returns documents to `Preview` without deleting saved views. An enabled plugin that owns a Settings page offers a `Configure →` deep link.

### Feynman

[Feynman](https://feynman.is) is an open-source (MIT) AI research agent CLI developed by **Companion**, built on the **Pi** agent runtime and **alphaXiv**, with workflows for literature review, auditing, writing, and citation verification. Mark Go Go connects directly to your local `feynman --mode rpc` installation. In wide windows (`≥ 1180 pt`) it sits beside the document as a resizable panel; narrow windows use a drawer. Switching documents within a workspace preserves the conversation and unsent draft. [Feynman setup guide](https://feynman.is/docs/getting-started/setup)

- **Chat / Files / alphaXiv**: Use **Actions** or `/` in Chat to run workflows, extensions, and session commands; select model and thinking level, attach papers or images, inspect tool calls, and follow citations (Enter sends, Shift+Enter inserts a newline, failed drafts are retained). Open generated reports and drafts from Files as regular tabs, review and apply proposed Markdown edits with single-step Undo, and manage paper context in alphaXiv. The gear icon jumps to Models & Providers, Search & Papers, MCP Servers, Packages, and Diagnostics in Settings; interactive login opens in the system Terminal.
- **Profile linking**: Before launch, Mark Go Go links existing credentials, models, memory, MCP config, skills, prompts, extensions, packages, and sessions from `~/.feynman` into its app profile without overwriting private files; linked session and global MCP changes stay shared with the CLI. Project trust stays private, denies by default, and approves only app-managed work directories. Windows uses symlinks (requiring Developer Mode or symlink privileges) and never falls back to copying; Windows runtime behavior is not yet verified.

### Git checkpoints

Stores document snapshots as local Git refs keyed by repository path. Outside Git repositories, the app keeps up to 50 local snapshots per document, which stay accessible if you later run `git init`. Save a document, then use **Checkpoints** in the page bar to create, diff, or restore snapshots. Renamed documents start a new history.

## Development and verification

Requires the Go version declared in [go.mod](go.mod); no Node.js required:

```sh
go tool mygo dev      # run locally
go tool mygo build    # package for current OS
scripts/verify.sh     # gofmt, mygo vet, full -race tests, Windows amd64 cross-build
```

## Documentation map

- [ARCHITECTURE.md](ARCHITECTURE.md): Read when locating packages or entry points (shell, editor, GenUI, Feynman, Portrait).
- [DESIGN.md](DESIGN.md): Read before changing layout, colours, controls, motion, or accessibility (Portrait 2.0 rules and contracts).
- [docs/genui/view-format.md](docs/genui/view-format.md): Read when authoring, debugging, or validating `.genui.json` views and OpenUI Lang v3 statements.
- [docs/agents/native-automation.md](docs/agents/native-automation.md): Use when verifying macOS native windows, menus, or session recovery.
- [CHANGELOG.md](CHANGELOG.md) · [docs/acceptance.md](docs/acceptance.md) · [LICENSE](LICENSE)
