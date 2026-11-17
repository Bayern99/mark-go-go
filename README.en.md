# Mark Go Go

[中文](README.md) · [Development](#development) · [UI source and credits](ACKNOWLEDGMENTS.en.md)

Mark Go Go is a local Markdown editor for macOS. Open a folder as a workspace, edit Markdown in Rich text or Markdown mode, and use GenUI to read a generated view alongside the original document.

- A workspace window opens at 1280 × 820 with a file tree and document tabs. Search paths, titles, excerpts and headings, including changes in open documents. The app restores the previous workspace, tabs and view modes when you reopen it.
- Use `⌘/` to switch between Rich text and Markdown. Rich text preserves the bytes of unedited blocks, renders Mermaid diagrams locally, and lets you edit special blocks in place.
- GenUI is a read-only view derived from the original Markdown. Its cards link back to their sources. The right pane holds the Outline and the controls for generating or updating GenUI through an external ACP agent.
- Portrait is the light interface for all three columns. Fonts and icons ship with the app.

## Development

Use Go 1.27.3 and Node 24.

```sh
npm --prefix web ci
npx --prefix web playwright install chromium-headless-shell

# Desktop development
go tool mygo dev

# Browser preview with an in-memory host
npm --prefix web run dev

# Regenerate bindings after Go API changes
go tool mygo generate
```

## Verification and packaging

Choose the check that exercises the changed behavior. Broaden verification for changes to files, IPC, or the window lifecycle.

```sh
# Focused checks
go test . -run 'Test.*(Close|Quit)'
npm --prefix web test -- test/workspace-restore.test.tsx

# After a Go binding change
go tool mygo generate
npm --prefix web run typecheck

# Integration checks
npm --prefix web test
go vet .
go test -race .

# Release build
go tool mygo build
```

`go tool mygo doctor` checks the development machine when setting up or diagnosing a build. `go tool mygo build` generates bindings, checks and bundles the frontend, and writes the app and DMG under `build/`.

For changes to the vendored document group, run `(cd third_party/mygo && go test -race . -run TestGroupClose)`.

Use `go tool mygo dev` and its separate development identity for native checks. Copy inputs from `web/test/fixtures/` into a temporary workspace. For an installed build check, use a temporary project copy with its own app name and bundle identifier and run `go tool mygo build -skip-dmg`. Browser tests cover page behavior. Use the native app to check menus, save dialogs, and input methods.

The vendored MyGo omits Automation. For a targeted check, copy the v0.7.4 plugin into the temporary project's vendor tree and import it in an `e2e` build-tagged Go file. It can list windows, inspect the WebView and invoke application menu items. Use the native UI for system sheets. [MyGo Automation guide](https://github.com/egoist/mygo/blob/v0.7.4/docs/plugins/automation.md)

## Documentation

- [Changelog](CHANGELOG.md)
- [Acceptance evidence](docs/acceptance.md)
- [Architecture](ARCHITECTURE.md) and [ADRs](docs/adr/)
- [Interfaces](docs/interfaces.md) and [Workspace contract](docs/workspace-contract.md)
- [Vendored MyGo document groups](docs/mygo-document-groups.md)
- [Portrait design boards](docs/design/Mark%20Go%20Go%20界面设计.html)
- [UI component gaps and typography plan](docs/design/ui-component-gaps.md)

## License

Mark Go Go's original code and Portrait UI design files are under the [MIT License](LICENSE). Included third-party code and assets carry their own licenses, listed in [UI source and credits](ACKNOWLEDGMENTS.en.md).
