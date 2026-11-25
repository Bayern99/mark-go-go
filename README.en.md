# Mark Go Go

[中文](README.md) · [Credits and licenses](ACKNOWLEDGMENTS.en.md)

Mark Go Go is a local Markdown editor for macOS written in Go with MyGo Native UI — no Electron, WebView, or Node.js runtime.

## Core capabilities

- **Byte-preserving in-place Markdown editing**: Editable **Preview** and raw **Markdown** share one text buffer and Undo/Redo history (`⌘E` toggles between them). Saving rewrites only modified blocks and preserves untouched Markdown bytes. Supports heading section folds, math, Mermaid diagrams, footnotes, in-place block editing, and resizable tables with wrapped rows.
- **GenUI derived reading views**: Generates read-only views (cards, tables, charts, and local-state controls) stored beside the document as `.<name>.genui.json`, with source links back to the original headings. Version 3 keeps the **A2UI 1.0** message envelope and section hashes for partial updates (version 2 files remain readable) while expressing `components` as **OpenUI Lang 1.0** statements parsed, evaluated, and rendered natively in Go. Switch to **GenUI** in the centre and use **Request** at the top right to generate, update changed sections, or restore the previous view; the backend, model, and reasoning effort live in `Settings / GenUI` (`⌘,`), which the gear icon links to.
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

## Bring your own local agent

Mark Go Go ships no model and holds no third-party credentials: it runs the agent CLIs already installed on your machine, unmodified, under your own accounts, and each provider's terms govern how you use them. There are two separate things here, and two separate places for them: a **GenUI backend** only writes a document's reading view, while an **external harness** reads and edits the workspace over the local channel.

### GenUI backends: Codex, Pi, Antigravity, Custom ACP

`Settings / GenUI` is the one place a backend is chosen: 2×2 cards with a status dot and `Test & Refresh` (which reads the command, its version, and its models without submitting your document), `Model` and whatever `Reasoning effort` the backend declares under `Generation`, and the command and arguments under `Advanced` (open by default only for Custom ACP). All four are equals; no vendor is the built-in default, and a first run probes `codex-acp` → `pi` → `agy`.

| Backend | Install | Sign in | Notes |
|---|---|---|---|
| Codex | `codex-acp` (it runs the underlying `codex`; point `CODEX_PATH` at that binary; check with `codex-acp --version`) | `codex login`, or `Sign in in Terminal` in `Settings / GenUI`, which runs `codex-acp login` | Starts as an argument-free ACP adapter; the model and reasoning effort come from the `configOptions` the session declares |
| Pi | Install `pi` so your terminal can run it | Choose a model so the app knows its `provider`, then run `pi auth login <provider>` in Terminal | Talks to `pi --mode rpc` directly, without ACP: every turn starts its own process and loads the guard extension this app ships, so reads stay in that turn's isolated folder and writes in its `out.json`; without a confirmed guard it does not generate |
| Antigravity | Install the Antigravity CLI that provides `agy` | `agy acp --login` | An optional preset, no longer the hard-coded default |
| Custom ACP | Any agent that speaks ACP over stdio, including two-hop setups such as `pi-acp` | Sign in with that agent's own command | Set the command and arguments under `Advanced` |

The Pi backend ships in this release; its automated coverage uses a fake Pi, so streaming and denied out-of-bounds writes under a real Pi account still want a machine test. Model refresh, sign-in, and streaming for Codex and Antigravity likewise need a machine that has those CLIs.

### External harnesses: Claude Code, Codex, Pi, anything else

`Settings / AI Integration` hands one window's workspace to an agent you run in your own terminal:

- **The `markgogo` command**: a standalone binary. `Install` links it into `~/.local/bin` and `Uninstall` removes only the link this app made. Nothing goes into `/usr/local/bin`, no sudo is needed, no shell profile is rewritten, and a file another program put there is left alone. A build that does not carry the binary says so and gives the manual route: `go install github.com/Bayern99/mark-go-go/cmd/markgogo@latest`, or build it from source.
- **The local channel**: off by default. Turning it on makes the app speak newline-delimited JSON-RPC on `ipc.sock` in its user data folder (folder `0700`, socket `0600`), reachable only by your own user on your own machine. The page shows whether it is listening and the source and time of the most recent external call. Turning it off stops listening and removes the socket.
- **Connecting**: the page offers a prompt (`Copy prompt`) and a static `SKILL.md` for Claude Code, Codex, Pi, and anything else. The skill is written into the app's own folder for you to copy where your harness reads skills. A shell harness just runs `markgogo`; a client that only speaks MCP over stdio registers the folded `MCP (stdio)` configuration, `{"command":"markgogo","args":["mcp"]}`, which forwards to the same socket and carries no port and no token. A sandboxed client such as Codex may deny the Unix socket; grant the access it needs rather than weakening the app.
- **CLI commands**: `status`, `read` (sections, line ranges, notes), `propose`, `new`, `open`, `genui format`, `genui write`, `checkpoint`, `agent-prompt`, `mcp`. Only `read`, `genui format`, and `agent-prompt` work without the app running; every change goes to the review queue for you to accept or reject in the editor.
- **Protocol**: the external channel speaks MCP `2026-07-28` (`server/discover` and a per-request `_meta` version) and falls back to the `2025-11-25` `initialize` handshake for older clients. This repository's socket and stdio shim tests cover both versions; end-to-end records for specific harnesses and for the MCP Inspector belong to the release checklist.

**Keep the two apart:** connecting an external harness means it reads and edits the workspace from your terminal under its own account. Signing in a GenUI backend means it only writes this document's reading view. The first is in `Settings / AI Integration`, the second in `Settings / GenUI`.

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
- [docs/adr/0016-ai-integration-faces.md](docs/adr/0016-ai-integration-faces.md): Read when asking why AI integration splits into a GenUI plugin, swappable backends, and an external agent channel.
- [CHANGELOG.md](CHANGELOG.md) · [docs/acceptance.md](docs/acceptance.md) · [LICENSE](LICENSE)
