# Mark Go Go

[中文](README.md) · [Credits and licenses](ACKNOWLEDGMENTS.en.md)

Mark Go Go is a local Markdown editor for macOS. Folder workspaces share a window and its document tabs. `Ctrl+Tab` switches between recent documents, and `⌘/` switches between the editable Preview and Markdown views. Select **GenUI** in the centre, then open **GenUI settings** at the top right of the view to choose a model, generate a reading view or update it.

## Optional plugins

Use **Workspace / Outline** in the left sidebar to switch between workspace files and headings in the active document. Use **Plugins** in the desktop tab bar to enable Feynman and Git independently. Both start disabled.

**Feynman** connects directly to your installed Feynman CLI in RPC mode. Install Feynman, enable the plugin, and use Feynman settings to sign in if needed. The English Portrait panel occupies the right sidebar. Switching documents in the same workspace preserves the conversation and unsent draft. Drag its left edge to resize it. Narrow windows use a drawer. [Feynman setup guide](https://feynman.is/docs/getting-started/setup)

Mark Go Go links existing files and directories in `~/.feynman`, including login, models, memory, MCP, skills, prompts, extensions, packages, and sessions, into its app profile. Session writes and Settings changes to a linked global MCP file are shared with the ordinary CLI. The app checks for additional links before launching Feynman, without overwriting anything already in the app profile. New state files and any name conflicts with existing private files can therefore stay separate. Feynman keeps access to user-level Pi resources, but the app's project trust file is private, denies by default, and only approves app-managed work directories. It does not change your global trust. Windows uses symbolic links, which require Developer Mode or permission to create symbolic links; it does not copy files when linking fails. Windows runtime behavior is not yet verified.

- **Chat**. Open **Actions** to choose a workflow, skill, extension or session action by name. **Browse research tools** lists tools from the running Feynman installation. Select one to inspect its parameters or add it to the current question. Choose an action, enter any instructions and send. Answer extension forms, choices and confirmations in the conversation. Typing `/` also opens the picker. Enter sends the message. Shift+Enter inserts a line break. Choose a model and thinking level, attach papers or images, inspect tool activity, follow citations, or use history to start and resume conversations in this workspace.
- **Review**. Inspect proposed Markdown edits and apply them in the editor. One Undo restores the preceding text. Completed reports, plans and drafts appear under **Research files** and open in regular document tabs.
- **Feynman settings**. Use the gear button to view status and manage models and MCP servers. Interactive setup and login open in the system Terminal.

**Git** checkpoints use local Git refs associated with the document’s repository path. Outside Git, the app keeps up to 50 local snapshots per document; those snapshots remain available if you later run `git init`. Save the document, then use **Checkpoints** in the document toolbar to create a snapshot or compare and restore an earlier version. A renamed document starts a new history.

## Development

The native app uses the Go version in [go.mod](go.mod).

```sh
go tool mygo dev
go tool mygo build
```

The migration keeps the original web implementation and tests for comparison. To run the browser reference, use `npm --prefix web ci` followed by `npm --prefix web run dev`.

## Verification

```sh
npx --prefix web playwright install chromium-headless-shell
npm --prefix web test
go test -race ./...
```

Use the [native Automation and macOS checks](docs/agents/native-automation.md) for menus, opening from Finder, saving and restoring a session.

[Architecture and code entry points](ARCHITECTURE.md) · [Interface design](DESIGN.md) · [Changelog](CHANGELOG.md) · [Acceptance evidence](docs/acceptance.md) · [MIT license](LICENSE)
