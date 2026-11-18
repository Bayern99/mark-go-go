# MyGo native Automation

Mark Go Go uses MyGo's official `plugins/automation` and `mygo-automation` CLI for real macOS verification. Do not add a parallel socket, JavaScript bridge or native-input protocol.

## Install / read the matching guide

The CLI is a Go tool dependency, pinned with the app to MyGo v0.10.0:

```sh
go tool mygo-automation version
go tool mygo-automation skills
```

Always read `skills` before a new automation task. A separately installed CLI is also supported, but its version must match `go list -m github.com/egoist/mygo`.

## Repeatable smoke test

```sh
scripts/native-automation-smoke.sh
```

The script performs **Build → Launch → Inspect → Interact → Assert → Restart → Assert → Cleanup** with the official CLI. It:

- builds an `e2e` + inspector QA app with MyGo v0.10.0;
- uses a temporary project, app name and bundle ID (`dev.markgogo.editor.automation-qa`), so normal Mark Go Go settings and recovery copies are not touched;
- creates only temporary Markdown fixtures and an unused JSON resource;
- checks cross-Workspace opening, one native window, recent-document switching, Finder duplicate reuse, Markdown view state and restart restoration;
- edits the active Markdown editor, uses native File > Save and Edit > Undo / Redo, verifies disk content and restores it after restart;
- uses `windows`, `eval`, `menu`, `wait`/`idle` and `screenshot` from the official plugin;
- saves the last screenshot to `/tmp/mgo-automation-smoke.png` (override with `MYGO_AUTOMATION_REPORT`);
- closes the Automation session, quits the QA app through its native menu, and removes temporary files.

The script exits nonzero on an assertion failure. Browser-only Vitest results are not native evidence.

## Manual development session

Enable the plugin only for an explicitly tagged development build:

```sh
MYGO_AUTOMATION=/tmp/mark-go-go-automation.sock \
GOFLAGS='-tags=e2e' \
go tool mygo dev -background
```

In another shell:

```sh
go tool mygo-automation open --socket /tmp/mark-go-go-automation.sock
go tool mygo-automation windows
go tool mygo-automation eval 'document.title'
go tool mygo-automation eval 'document.querySelector(".doc-tab.sel")?.textContent'
go tool mygo-automation menu File Save
go tool mygo-automation screenshot /tmp/mark-go-go-native.png
go tool mygo-automation close
```

Use `--settle` after actions and `wait` for expected state instead of fixed UI sleeps. Opening again invalidates old refs. Use a distinct `--session` per agent/app and one command at a time in that session.

A pure page window has no native element snapshot and does not accept native UI `press` / `fill`; use `eval` for its DOM. `menu` still operates the application menu. Native MyGo UI/dialog windows use `snapshot`, refs and the ordinary actions. Context menus are not supported by the official plugin.

## Release boundary

- `automation_e2e.go` links the plugin only with the `e2e` build tag.
- `mygo dev` / `mygo build -debug` retain the inspector; `GOFLAGS=-tags=e2e` carries the tag into those builds.
- Normal `mygo build` adds `mygo_noinspector`. Even an `e2e + mygo_noinspector` build links only the official guard that ignores `MYGO_AUTOMATION` and never listens.
- The socket is created only when `MYGO_AUTOMATION` names a user-private path. There is no discovery or release listener.

Useful checks:

```sh
go list -deps . | grep 'plugins/automation'       # default: no output
go test -tags e2e ./...
```

## Current boundary

The smoke test covers the multi-Workspace native path. Native Save As / Rename / Move To system panels, conflict sheets and exact editor scroll/cursor geometry still require targeted verification; do not infer their success from this smoke test.
