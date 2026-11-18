# Mark Go Go

[中文](README.md) · [Credits and licenses](ACKNOWLEDGMENTS.en.md)

Mark Go Go is a local Markdown editor for macOS. Folder workspaces share a window and its document tabs. `Ctrl+Tab` switches between recent documents, and `⌘/` switches between the editable Preview and Markdown views. Use the right pane to browse the outline or generate a read-only GenUI view with a configured ACP agent.

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

[Architecture and code entry points](ARCHITECTURE.md) · [Changelog](CHANGELOG.md) · [Acceptance evidence](docs/acceptance.md) · [MIT license](LICENSE)
