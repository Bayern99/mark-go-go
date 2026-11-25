# Mark Go Go

[中文](README.md) · [Credits and licenses](ACKNOWLEDGMENTS.en.md)

Mark Go Go is a native local Markdown editor and reading-research workspace for macOS written in Go. Its window shell, editor, sidebar, GenUI reader and research panel are all drawn natively by MyGo Native UI — **no Electron, no WebView, and no Node.js runtime** — pairing byte-preserving Markdown fidelity with low overhead and calm spatial ergonomics.

## Highlights and advantages

- **Pure Go native runtime (zero WebView)**: Single-process native rendering with fast startup, small memory footprint, and no bundled browser engine; supports macOS hidden title bar integration, native menus, IME composition, and VoiceOver / Accessibility semantics.
- **Byte-preserving in-place Markdown editing**: Editable **Preview** and raw **Markdown** share one text buffer and Undo/Redo stack (`⌘E` switches between them), leaving untouched Markdown bytes intact on save. Supports heading section folding, LaTeX math, Mermaid diagrams, footnotes, in-place block editing, and resizable tables with CJK-aware wrapping.
- **Arc-inspired spatial layout × Wu Xing Spaces**: The window acts as a coloured **Scenery** holding an opaque **Page** sheet. Each folder workspace selects from eight Wu Xing Spaces (*Five Elements*, *Metal–Water*, *Wood–Fire*, *Earth*, *Metal*, *Wood*, *Water*, *Fire*) with complete Light and Dark appearances — all 16 palettes verified automatically against WCAG AA contrast.
- **Calm sidebar & global Command Bar**: Multiple folder workspaces share one window and vertical document tabs (`Ctrl+Tab` cycles recent documents). The sidebar provides **Files / Outline** switching, zero-indent directory drill-down, persistent outline selection with top-aligned heading jumps, smooth pointer collapse/expand and instant `⌘0` toggling. Press `⌘K` or `⌘T` to search open documents, workspace files, headings and every command from one Command Bar.
- **Unified Settings Center (`⌘,`)**: One searchable settings sheet for General, Appearance, Editor, Shortcuts, Plugins, Diagnostics and plugin-registered pages, with live `/` filtering, deep links, and one-click reset.
- **GenUI structured reading view**: Built on A2UI and OpenUI Lang v3, GenUI turns long Markdown documents into read-only cards, tables and charts that jump back to their source passages. Switch to **GenUI** in the centre, use **Request** at the top right to generate, regenerate changed sections or restore the previous view, and use the gear icon to jump to GenUI settings.

## Key shortcuts

| Shortcut | Action |
|---|---|
| `⌘K` / `⌘T` | Open the Command Bar (documents, files, headings and commands) |
| `⌘E` | Toggle between editable Preview and Markdown source |
| `⌘0` | Hide or show the left sidebar (drawer in narrow windows) |
| `⌥⌘0` | Hide or show the right Feynman research panel |
| `⌘,` | Open the Unified Settings Center |
| `Ctrl+Tab` | Switch between recent document tabs |

## Optional plugins

Enable **Feynman** and **Git** independently from **Settings / Plugins** (`⌘,`) or the Command Bar (`⌘K`). Both start disabled.

### Feynman research panel

**Feynman** connects directly to your local Feynman CLI in RPC mode. In wide windows (`≥ 1180 pt`), it docks beside the document as a second resizable paper sheet; in narrower windows it slides out as a glass drawer. Switching documents within the same workspace preserves the conversation and unsent draft. [Feynman setup guide](https://feynman.is/docs/getting-started/setup)

Mark Go Go links existing files and directories in `~/.feynman` (credentials, models, memory, MCP config, skills, prompts, extensions, packages and sessions) into its app profile. Session writes and Settings edits to a linked global MCP config remain visible to the ordinary CLI. Missing links are refreshed before launch without overwriting existing app-profile files; the app's project trust file stays isolated, denies by default, and only approves app-managed work directories. Windows uses symbolic links (requiring Developer Mode or symlink privileges) and does not fall back to copying; Windows runtime behavior is not yet verified.

- **Chat**: Open **Actions** or type `/` to invoke workflows, extensions and session commands by name. Choose a model and thinking level, attach papers or images, inspect collapsible tool activity, follow citations, and start or resume conversations from history. Enter sends, Shift+Enter inserts a newline, and failed drafts/attachments are retained for lossless recovery or swap.
- **Files**: Browse generated reports, plans and drafts and open them as regular document tabs. Inspect proposed Markdown edits in the **Review** diff view and apply them with single-step Undo.
- **alphaXiv**: Search and manage paper context inside the panel, with account and search settings linked straight to the Settings Center.
- **Research settings**: The panel's gear button deep-links into **Models & Providers**, **Search & Papers**, **MCP Servers**, **Packages** and **Diagnostics** inside the Unified Settings Center. Interactive setup and sign-in open in the system Terminal.

### Git local checkpoints

**Git** checkpoints store snapshots as local Git refs keyed by the document's repository path. Outside Git repositories, the app keeps up to 50 local snapshots per document, which remain accessible if you later run `git init`. Save a document, then open **Checkpoints** in the page bar to create a snapshot or diff and restore earlier versions. A renamed document starts a new history.

## Development

The native app uses the Go toolchain pinned in [go.mod](go.mod) with no Node.js or web bundler required:

```sh
go tool mygo dev
go tool mygo build
```

MyGo Native UI draws Preview, Markdown, GenUI and the workspace shell. Go regression tests load the 26 Markdown fixtures and shared section vectors directly; see the [GenUI format guide](docs/genui/view-format.md) for the protocol and examples.

## Verification

```sh
scripts/verify.sh
```

The script checks Go formatting, `go tool mygo vet`, full-package race tests (`-race`) and Windows amd64 cross-compilation. Use the [native Automation and macOS checks](docs/agents/native-automation.md) for native windows, menus and session recovery. The [Portrait performance record](docs/agents/portrait-performance.md) preserves the retired WebKit baseline and same-machine comparison method.

[Architecture and code entry points](ARCHITECTURE.md) · [Interface design](DESIGN.md) · [Changelog](CHANGELOG.md) · [Acceptance evidence](docs/acceptance.md) · [MIT license](LICENSE)
