# Mark Go Go

[中文](README.md) · [Credits and licenses](ACKNOWLEDGMENTS.en.md)

Mark Go Go is a local Markdown editor for macOS. Folder workspaces share a window and its document tabs. `Ctrl+Tab` switches between recent documents, and `⌘/` switches between the editable Preview and Markdown views. Select **GenUI** in the centre, then open **GenUI settings** at the top right of the view to choose a model, generate a reading view or update it.

## Optional plugins

Use **Plugins** in the desktop tab bar to enable Feynman and Git independently. Both start disabled.

**Feynman** uses your local Feynman CLI, Node.js and `pi-acp`. Install the adapter with `npm install -g pi-acp`. Configure your model and sign in using Feynman’s setup guide, then enable the plugin. **Feynman** opens in the right pane, where you can also select **Outline**. Each workspace has a research conversation that follows its document tabs and resumes after restart. Open a report, plan, draft or provenance file under **Files** to edit it in a document tab. Select **Edit this document** to share the current buffer with Feynman. Review the changes and choose **Apply in Markdown** to apply them as one undoable edit. [Feynman setup guide](https://feynman.is/docs/getting-started/setup)

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
