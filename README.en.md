# Mark Go Go

[中文](README.md) · [Credits and licenses](ACKNOWLEDGMENTS.en.md)

Mark Go Go is a local Markdown editor for macOS. Folder workspaces share a window and its document tabs. `Ctrl+Tab` switches between recent documents, and `⌘/` switches between the editable Preview and Markdown views. Select **GenUI** in the centre, then open **GenUI settings** at the top right of the view to choose a model, generate a reading view or update it.

## Optional plugins

Use **Plugins** in the desktop tab bar to enable Feynman and Git independently. Both start disabled.

**Feynman** uses your local Feynman CLI and Node.js. Install its adapter with `npm install -g pi-acp`, then enable the plugin. Drag the left edge of the research pane to resize it. Narrow windows use a drawer. [Feynman setup guide](https://feynman.is/docs/getting-started/setup)

- **Chat**. Type `/` to find the workflows and skills advertised by your session. Enter selects a command or sends the message. Shift+Enter inserts a line break. Choose a model and thinking level, attach papers or images, inspect tool activity, follow citations, or use history to start and resume conversations in this workspace.
- **Files**. Reports, plans, drafts and provenance records open in regular Markdown tabs. The folder button opens other research outputs in Finder. To revise a document, choose **Edit document**, review the diff, then select **Apply in Markdown**. One Undo restores the preceding text.
- **CLI**. Run Feynman itself inside the pane. Leave the arguments empty and click **Run** to open the interactive interface with the currently selected saved conversation, or run a command such as `setup`, `doctor`, `model login` or `mcp list`. Switching tabs keeps the process available. A successful exit publishes completed Markdown. **Stop CLI** interrupts the run. CLI and Chat take turns using the workspace’s research directory.

**Git** uses an existing local repository. Save the document, then create a checkpoint in the **Git** panel. Choose a version to compare it with the current buffer and restore its text. Checkpoints use local Git refs associated with the document’s repository path. A renamed document starts a new history.

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
