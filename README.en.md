# Mark Go Go

[中文](README.md) · [Credits and licenses](ACKNOWLEDGMENTS.en.md)

Mark Go Go is a local Markdown editor. Your documents are ordinary `.md` files on disk, and the app opens, edits, and saves them in place. There is no account and no cloud storage. The interface is drawn natively with Go and MyGo Native UI, without Electron, a WebView, or Node.js. Releases are currently published for macOS on Apple silicon only.

Beyond the editor there are three optional features: **GenUI**, which turns a document into a structured reading view; **Feynman**, a literature research assistant; and per-document **Git checkpoints**. In addition, an AI agent you run in your terminal can read the workspace through the `markgogo` command and send edit proposals to the editor. Everything involving AI calls an agent CLI already installed on your machine. The app ships no model and stores no third-party credentials.

## Design choices

- **The Markdown file is the only source.** Editing rewrites only the blocks you touched and keeps every other byte as it was. Reading views and snapshots live outside the file, and deleting them leaves it unchanged. Notes are written into the file as comments, so it stays plain Markdown.
- **GenUI: AI organizes a document without rewriting it.** What it produces is a read-only reading view; the source is never edited. Every card in the view points to the section it is based on, so you can always go back and check. When the source changes, you can see which cards are out of date and regenerate only those. The view is a plain JSON file beside the document in a documented format. With the local channel on, other agents can write a view too, with `markgogo genui write`.
- **Feynman: research and writing in the same window.** The research conversation belongs to the folder workspace and carries on as you switch documents. Every change Feynman suggests to the text arrives as a proposal that you compare, accept, or reject. The app reuses the Feynman configuration you already have on the command line and shares sessions with it.
- **Your own AI, no vendor lock-in.** GenUI works with Codex, Pi, Antigravity, or any ACP agent, and the four backends are equals. External agents connect through a local socket, with no network port and no token.
- **A native interface.** The window is drawn directly by Go, with no embedded browser engine. Each workspace has its own colour scheme to tell folders apart.

## What it does

- **Edit Markdown**: edit directly in the rendered **Preview**, or press `⌘E` for the raw **Markdown** source. Both views share one text buffer and one undo history. Saving rewrites only the blocks you edited and keeps the original bytes of everything else.
- **Work across folders**: one window can hold several folder workspaces, with open documents listed vertically in the sidebar. The sidebar switches between a file tree and an outline, and the `⌘K` Command Bar searches documents, files, headings, and commands. Open documents and scroll positions are restored after a restart.
- **Generate reading views (GenUI, on by default)**: have a local AI agent turn one document into a read-only view made of cards, tables, and charts. Every item names the source section it was derived from. When the source changes, the view shows which parts are out of date and can update just those parts.
- **Research (Feynman, off by default)**: talk to your installed [Feynman](https://feynman.is) in a panel beside the document, for literature search, review, writing, and citation checks. Edits Feynman proposes go to review first and reach the document only after you accept them.
- **Snapshots (Git checkpoints, off by default)**: save local snapshots of a single document, compare them with the current text, or restore an earlier version.
- **External agents (off by default)**: once the local channel is on, agents running in your terminal, such as Codex or Pi, can use the `markgogo` command to read the current workspace, propose edits, create documents, or write reading views.

## What it does not do

- It provides no model, AI account, or subscription. AI features depend on CLIs you install and sign in to yourself, and each provider's terms govern usage and data handling.
- There is no cloud sync, collaboration, or account system.
- It does not send documents to any service on its own. Document content goes to the local agent you configured only when you turn on GenUI, Feynman, or the external channel and actually use them. Remote images referenced in Markdown are downloaded over HTTPS for display.
- There are no Linux or Windows packages. CI runs the full automated test suite on Linux and only cross-compiles for Windows; neither platform has been verified on real hardware.

## Install

Download `mark-go-go-<version>-arm64.dmg` from [Releases](https://github.com/Bayern99/mark-go-go/releases) and drag the app into Applications. The package is ad hoc signed and not notarized by Apple. macOS blocks the first launch; go to System Settings → Privacy & Security and choose Open Anyway.

Running from source needs the Go version declared in [go.mod](go.mod); Node.js is not required:

```sh
go tool mygo dev      # run locally
go tool mygo build    # package for the current platform
scripts/verify.sh     # gofmt, mygo vet, -race tests for every package, Windows amd64 cross-build
```

## Features in detail

### Editing

- Preview supports heading section folds, math, Mermaid diagrams, footnotes, resizable tables with wrapped rows, and editing a special block's Markdown in place.
- Notes are stored in the document as `<!-- note: … -->` comments, so any Markdown tool still shows them.
- The File menu offers Save As, Duplicate, Rename, Move To, and Revert. `.md` and `.markdown` files open directly from Finder.

### Workspaces and appearance

- In the sidebar, **Files** is the file tree and **Outline** is the current document's headings; click a heading to jump to it. `Ctrl+Tab` cycles recent documents.
- Each workspace can pick a colour Space (eight in total, drawn from traditional Chinese Wu Xing colours) and an appearance of System, Light, or Dark, which helps tell folders apart.
- `⌘,` opens the Settings Center, where app and plugin settings live, with search.

| Shortcut | Action |
|---|---|
| `⌘K` / `⌘T` | Command Bar: documents, files, headings, and commands |
| `⌘E` | Switch between Preview and Markdown source |
| `⌘0` | Hide or show the left sidebar (a drawer in narrow windows) |
| `⌥⌘0` | Hide or show the right Feynman panel |
| `⌘,` | Settings Center |
| `Ctrl+Tab` | Cycle recent documents |

### Plugins

Each plugin is switched on or off in **Settings / Plugins** or from the Command Bar. GenUI starts on; Feynman and Git start off. Turning GenUI off returns documents to Preview without deleting saved views.

**GenUI reading views.** Switch the centre of the window to **GenUI** and use **Request** at the top right to generate a view. Cards appear one by one while it generates. The view is saved beside the document as `.<name>.genui.json`; the Markdown itself is never changed. Source buttons in the view show the section each item came from: hover or focus one with the keyboard to preview the source excerpt, and click it to jump back to the text. When the source changes, the affected cards are marked out of date; you can update them one at a time or return to the previous view. Generation is done by the local agent you choose in **Settings / GenUI**; see [GenUI backends](#genui-backends) below. The view format is described in [docs/genui/view-format.md](docs/genui/view-format.md).

**Feynman.** [Feynman](https://feynman.is) is an open-source (MIT) research agent CLI developed by Companion and built on the Pi agent runtime and alphaXiv. Mark Go Go connects to your installed Feynman through `feynman --mode rpc` ([setup guide](https://feynman.is/docs/getting-started/setup)). In windows at least 1180 pt wide it appears as a resizable second page beside the document; narrower windows show it as a drawer.

- **Chat**: run workflows and commands from the **Actions** button or by typing `/`; choose the model and thinking level, attach papers or images, and inspect tool calls and citations.
- **Files**: open reports and drafts Feynman produced.
- **alphaXiv**: manage paper context.
- **Review**: compare, accept, or reject proposed Markdown edits; an accepted edit can be undone in one step.

Switching documents within a workspace keeps the conversation and any unsent draft.

**Git checkpoints.** After saving a document, use **Checkpoints** at the top of the page to create a snapshot, view a diff, or restore. Inside a Git repository, snapshots are stored as separate local Git refs and do not touch your branches or staging area. Outside a repository, each document keeps up to 50 snapshots, which remain readable if you later run `git init`. A renamed document starts a new history from its new path.

## AI integration reference

AI settings live in two places that do different jobs:

- **Settings / GenUI**: choose which agent generates a reading view for the current document. The agent receives only that document, and the app validates the view it produces before saving it.
- **Settings / AI Integration**: let an agent you run in your terminal read and edit the whole workspace.

### GenUI backends

Four backends are available, and none is the designated default vendor. On first use the app probes for installed commands in the order `codex-acp`, `pi`, `agy`. **Test & Refresh** reads only the command version and the model list and never sends document content. Model and reasoning effort are chosen under **Generation**; the command and arguments are edited under **Advanced**.

| Backend | Install | Sign in | Notes |
|---|---|---|---|
| Codex | `codex-acp`, which runs the underlying `codex`. Point `CODEX_PATH` at `codex` if needed; check with `codex-acp --version` | `codex login`, or **Sign in in Terminal** in the settings page | Connects over ACP; model and reasoning effort follow the options the session declares |
| Pi | Install `pi` so your terminal can run it | Choose a model in the settings page, then run `pi auth login <provider>` in Terminal | Talks to `pi --mode rpc` directly, without ACP. Each generation starts a new process and loads the guard extension the app ships, which limits reads to that generation's temporary folder and writes to its `out.json`. Without a confirmed guard it does not generate |
| Antigravity | Install the Antigravity CLI that provides `agy` | `agy acp --login` | Connects over ACP |
| Custom ACP | Any agent that speaks ACP over standard input and output | Use that agent's own sign-in command | Fill in the command and arguments under **Advanced** |

Each generation runs in its own temporary folder, and the result replaces the current view only after it passes format validation.

### External agents and the `markgogo` command

**Settings / AI Integration** hands one window's workspace to an agent you run in your terminal.

- **The `markgogo` command**: the app package carries this standalone command-line program at `Contents/Resources/markgogo`. **Install** creates a link to it in `~/.local/bin`, and **Uninstall** removes only that link. No sudo is needed, no shell profile is changed, and files other programs put there are left alone. When the app runs straight from source there is no bundled program and the page shows `Not in this build`; build it by hand from a checkout with `go build -o ~/.local/bin/markgogo ./cmd/markgogo`. `go install …@latest` cannot build it, because `go.mod` uses `replace` to point at the bundled MyGo.
- **The local channel**: off by default. When on, the app exchanges JSON-RPC messages, one per line, on `ipc.sock` in its user data folder. The folder is `0700` and the socket `0600`, so only your own account can connect. The channel opens no network port and uses no token. The page shows whether it is listening and the source and time of the most recent external call. Turning it off stops listening and removes the socket.
- **Connecting**: the page provides a prompt and a `SKILL.md` for Codex, Pi, and other agents. An agent that can run shell commands calls `markgogo` directly. A client that only speaks MCP can register `{"command":"markgogo","args":["mcp"]}`, which forwards MCP over standard input and output to the same socket. A sandboxed client such as Codex may not be allowed to reach a Unix socket; grant that access on the client side.
- **Commands**: `status`, `read` (sections, line ranges, notes), `propose`, `new`, `open`, `genui format`, `genui write`, `checkpoint`, `agent-prompt`, `mcp`. `read`, `genui format`, and `agent-prompt` work without the app running.
- **When changes take effect**: only edit proposals (`markgogo propose`, MCP tool `propose_edit`) go to the review queue and change the document after you accept them. The other writes take effect immediately:
  - `markgogo new` (`create_document`) creates a document and never overwrites an existing file;
  - `markgogo genui write` (`write_genui_view`) validates and then replaces the reading view;
  - `markgogo checkpoint` (`create_checkpoint`) snapshots a saved document.
- **Protocol**: the channel speaks MCP `2026-07-28` (`server/discover` and a per-request `_meta` version) and also accepts the `2025-11-25` `initialize` handshake from older clients.

### Feynman configuration

Before Feynman starts, the app links the credentials, models, memory, MCP configuration, skills, prompts, extensions, packages, and sessions already in `~/.feynman` into its own profile, without overwriting private files already there. Once linked, session and global MCP changes are shared with the command-line Feynman. The project trust list is kept separately by the app, denies by default, and approves only work folders the app manages. On Windows this step uses symbolic links, which need Developer Mode or the symlink privilege; if linking fails, the app does not fall back to copying.

## Verification

Every commit runs format checks, `mygo vet`, `-race` tests for every package, and a Windows cross-build in CI. Acceptance records from real macOS machines are in [docs/acceptance.md](docs/acceptance.md), covering actual generation with all four GenUI backends, the Pi guard blocking out-of-bounds access, both MCP protocol versions, and external agents connected through Pi and Codex.

## Documentation map

- [ARCHITECTURE.md](ARCHITECTURE.md): code entry points and package boundaries.
- [DESIGN.md](DESIGN.md): interface rules (Portrait 2.0) and test contracts; read before changing layout, colours, controls, motion, or accessibility.
- [GLOSSARY.md](GLOSSARY.md): product terms.
- [docs/adr/](docs/adr/): technical decision records. For why AI integration is split into three parts, see [ADR 0016](docs/adr/0016-ai-integration-faces.md).
- [docs/genui/view-format.md](docs/genui/view-format.md): the `.genui.json` view format.
- [docs/agents/native-automation.md](docs/agents/native-automation.md): how to verify native macOS windows, menus, and session recovery.
- [CHANGELOG.md](CHANGELOG.md) · [LICENSE](LICENSE)
