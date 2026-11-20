# Mark Go Go

[中文](README.md) · [Credits and licenses](ACKNOWLEDGMENTS.en.md)

Mark Go Go is a local Markdown editor for macOS. Folder workspaces share a window and its document tabs. `Ctrl+Tab` switches between recent documents, and `⌘/` switches between the editable Preview and Markdown views. Select **GenUI** in the centre, then open **GenUI settings** at the top right of the view to choose a model, generate a reading view or update it.

## Optional plugins

Use **Workspace / Outline** in the left sidebar to switch between workspace files and headings in the active document. Use **Plugins** in the desktop tab bar to enable Feynman and Git independently. Both start disabled.

**Feynman** connects directly to your installed Feynman CLI in RPC mode. Install Feynman, enable the plugin, and start setup and sign-in from Feynman settings. Mark Go Go uses a private profile, separate from your existing CLI settings and login. The English Portrait panel occupies the right sidebar. Switching documents in the same workspace preserves the conversation and unsent draft. Drag its left edge to resize it. Narrow windows use a drawer. [Feynman setup guide](https://feynman.is/docs/getting-started/setup)

- **Chat**. Open **Actions** to choose a workflow, skill, extension or session action by name. **Browse research tools** lists tools from the running Feynman installation. Select one to inspect its parameters or add it to the current question. Choose an action, enter any instructions and send. Answer extension forms, choices and confirmations in the conversation. Typing `/` also opens the picker. Enter sends the message. Shift+Enter inserts a line break. Choose a model and thinking level, attach papers or images, inspect tool activity, follow citations, or use history to start and resume conversations in this workspace.
- **Review**. Inspect proposed Markdown edits and apply them in the editor. One Undo restores the preceding text. Completed reports, plans and drafts appear under **Research files** and open in regular document tabs.
- **Feynman settings**. Use the gear button to view status and manage models and MCP servers. Interactive setup and login open in the system Terminal.

**Git** checkpoints use local Git refs associated with the document’s repository path. Outside Git, the app keeps up to 50 local snapshots per document; those snapshots remain available if you later run `git init`. Save the document, then use **Checkpoints** in the document toolbar to create a snapshot or compare and restore an earlier version. A renamed document starts a new history.

## Development

See the [CI configuration](.github/workflows/ci.yml) for Go and Node versions.

```sh
npm --prefix web ci
go tool mygo dev
```

Run `npm --prefix web run dev` for the browser preview. Run `go tool mygo generate` after changing bindings. Build the release with `go tool mygo build`.

## Verification

```sh
npx --prefix web playwright install chromium-headless-shell
npm --prefix web test
go test -race .
```

Use the [native Automation and macOS checks](docs/agents/native-automation.md) for menus, save dialogs, and restart behavior.

[Architecture and code entry points](ARCHITECTURE.md) · [Interface design](DESIGN.md) · [Changelog](CHANGELOG.md) · [Acceptance evidence](docs/acceptance.md) · [MIT license](LICENSE)
