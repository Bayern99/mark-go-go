#!/usr/bin/env bash
# Build and drive a disposable Mark Go Go app with MyGo's official Automation.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
if [[ -n "${MYGO_AUTOMATION_BIN:-}" ]]; then
  AUTO=("$MYGO_AUTOMATION_BIN")
elif command -v mygo-automation >/dev/null 2>&1; then
  AUTO=("$(command -v mygo-automation)")
else
  AUTO=(go -C "$ROOT" tool mygo-automation)
fi
EXPECTED=$(cd "$ROOT" && go list -m -f '{{.Version}}' github.com/egoist/mygo)
ACTUAL=$("${AUTO[@]}" version)
ACTUAL=${ACTUAL%% *}
if [[ "$ACTUAL" != "$EXPECTED" ]]; then
  echo "Automation version $ACTUAL does not match MyGo $EXPECTED" >&2
  exit 2
fi

WORK=$(mktemp -d /tmp/mgo-automation.XXXXXX)
PROJECT="$WORK/project"
FIXTURES="$WORK/documents"
SOCKET="$WORK/automation.sock"
SESSION="mgo-$(basename "$WORK")"
REPORT=${MYGO_AUTOMATION_REPORT:-/tmp/mgo-automation-smoke.png}
APP_NAME="Mark Go Go Automation QA"
APP="$WORK/build/darwin-arm64/$APP_NAME.app"
PROCESS=""
mkdir -p "$PROJECT" "$FIXTURES/Research" "$FIXTURES/Music Department" "$FIXTURES/Personal Notes"

cleanup() {
  "${AUTO[@]}" close --session "$SESSION" >/dev/null 2>&1 || true
  if [[ -n "$PROCESS" ]] && kill -0 "$PROCESS" 2>/dev/null; then
    kill "$PROCESS" 2>/dev/null || true
    wait "$PROCESS" 2>/dev/null || true
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT INT TERM

# Symlink the real source, but use an isolated app identity, user-data store,
# output directory, document tree and automation socket.
ln -s "$ROOT"/go.mod "$ROOT"/go.sum "$PROJECT"/
ln -s "$ROOT"/web "$ROOT"/third_party "$ROOT"/resources "$PROJECT"/
ln -s "$ROOT"/*.go "$PROJECT"/
ROOT="$ROOT" PROJECT="$PROJECT" APP_NAME="$APP_NAME" node <<'NODE'
const fs = require('fs')
const config = JSON.parse(fs.readFileSync(`${process.env.ROOT}/mygo.json`, 'utf8'))
config.name = process.env.APP_NAME
config.slug = 'mark-go-go-automation-qa'
config.identifier = 'dev.markgogo.editor.automation-qa'
config.out = 'build'
fs.writeFileSync(`${process.env.PROJECT}/mygo.json`, JSON.stringify(config, null, 2) + '\n')
NODE

printf '# Discussion\n\nResearch notes.\n' > "$FIXTURES/Research/Discussion.md"
printf '# CC129\n\nMusic notes.\n' > "$FIXTURES/Music Department/CC129.md"
printf '# Ideas\n\nPersonal notes.\n' > "$FIXTURES/Personal Notes/Ideas.md"
printf '{}\n' > "$FIXTURES/Research/resource.json"

echo "BUILD  MyGo $EXPECTED with official Automation"
(
  cd "$PROJECT"
  GOFLAGS='-tags=e2e' go tool mygo build -debug -skip-dmg -skip-notarize -o "$WORK/build" > "$WORK/build.log"
)
APP=$(find "$WORK/build" -name '*.app' | head -n 1)
[[ -n "$APP" ]] || { echo "Missing QA app under $WORK/build" >&2; exit 1; }
EXECUTABLE=$(find -L "$APP/Contents/MacOS" -type f -perm -111 | head -n 1)
[[ -n "$EXECUTABLE" && -x "$EXECUTABLE" ]] || { echo "Missing QA executable in $APP" >&2; exit 1; }

auto() { "${AUTO[@]}" "$@" --session "$SESSION"; }
js() { auto eval "$1"; }
wait_js() {
  local script=$1
  local deadline=$((SECONDS + 20))
  while (( SECONDS < deadline )); do
    if [[ $(js "$script" 2>/dev/null || true) == 'true' ]]; then return 0; fi
    sleep 0.1
  done
  echo "Timed out waiting for: $script" >&2
  auto windows || true
  js 'document.body.innerText' >&2 || true
  js 'document.querySelector("[data-testid=\"doc-tabs\"]")?.outerHTML ?? "no tabs"' >&2 || true
  return 1
}
assert_js() {
  local script=$1
  local message=$2
  local value
  local deadline=$((SECONDS + 5))
  while (( SECONDS <= deadline )); do
    value=$(js "$script")
    if [[ "$value" == 'true' ]]; then
      echo "PASS  $message"
      return 0
    fi
    sleep 0.1
  done
  echo "ASSERT FAIL: $message (got $value)" >&2
  auto windows >&2 || true
  js 'document.querySelectorAll("[role=\"tab\"]").length' >&2 || true
  js 'document.querySelectorAll("[role=\"tab\"]").length === 1' >&2 || true
  js 'document.body.innerText' >&2 || true
  js 'document.querySelector("[data-testid=\"doc-tabs\"]")?.outerHTML ?? "no tabs"' >&2 || true
  cat "$WORK/app.log" >&2 || true
  exit 1
}
launch() {
  MYGO_AUTOMATION="$SOCKET" "$EXECUTABLE" "${1:-}" > "$WORK/app.log" 2>&1 &
  PROCESS=$!
  local deadline=$((SECONDS + 20))
  while [[ ! -S "$SOCKET" ]]; do
    (( SECONDS < deadline )) || { echo 'Automation socket did not open' >&2; exit 1; }
    kill -0 "$PROCESS" 2>/dev/null || { cat "$WORK/app.log" >&2; exit 1; }
    sleep 0.1
  done
  auto open --socket "$SOCKET" > /dev/null
  while ! auto windows 2>/dev/null | grep -q '^w[0-9]'; do
    (( SECONDS < deadline )) || { echo 'Application window did not open' >&2; exit 1; }
    kill -0 "$PROCESS" 2>/dev/null || { cat "$WORK/app.log" >&2; exit 1; }
    sleep 0.1
  done
  auto wait --timeout 20s > /dev/null
}
quit() {
  auto menu "$APP_NAME" "Quit $APP_NAME" > /dev/null
  wait "$PROCESS"
  PROCESS=""
}

launch "$FIXTURES/Research/Discussion.md"
wait_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 1"
assert_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 1" 'initial Markdown opens'
assert_js "document.querySelector('[data-testid=\"workspace-switcher\"]')?.textContent?.includes('Research')" 'initial Workspace selected'
assert_js "!document.body.textContent.includes('resource.json')" 'non-Markdown resource stays out of navigation'

open -a "$APP" "$FIXTURES/Music Department/CC129.md"
wait_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 2 && document.querySelector('[data-testid=\"workspace-switcher\"]')?.textContent?.includes('Music Department')"
assert_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 2" 'second folder joins the same document group'
assert_js "document.querySelector('.doc-tab.sel')?.textContent?.includes('CC129.md')" 'second document active'

js "document.querySelector('[data-testid=\"view-mode-markdown\"]')?.click(); true" > /dev/null
wait_js "document.querySelector('[data-testid=\"view-mode-markdown\"]')?.getAttribute('aria-checked') === 'true'"
js "window.dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', ctrlKey: true, bubbles: true})); true" > /dev/null
assert_js "document.querySelector('.doc-tab.sel')?.textContent?.includes('Discussion.md')" 'Ctrl+Tab returns to recent cross-Workspace document'
js "window.dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', ctrlKey: true, bubbles: true})); true" > /dev/null
assert_js "document.querySelector('.doc-tab.sel')?.textContent?.includes('CC129.md')" 'Ctrl+Tab switches forward again'
assert_js "document.querySelector('[data-testid=\"view-mode-markdown\"]')?.getAttribute('aria-checked') === 'true'" 'view mode survives document switches'

open -a "$APP" "$FIXTURES/Personal Notes/Ideas.md"
wait_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 3"
open -a "$APP" "$FIXTURES/Research/Discussion.md"
wait_js "document.querySelector('.doc-tab.sel')?.textContent?.includes('Discussion.md')"
assert_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 3" 'Finder duplicate reuses its document session'
if [[ $(auto windows | grep -c '^w[0-9]') != 1 ]]; then
  echo 'ASSERT FAIL: expected one native window' >&2
  auto windows >&2
  exit 1
fi
echo 'PASS  one native window hosts every Workspace'
js "document.querySelector('[data-testid=\"view-mode-markdown\"]')?.click(); true" > /dev/null
wait_js "document.querySelector('[data-testid=\"view-mode-markdown\"]')?.getAttribute('aria-checked') === 'true'"

js "const editor = document.querySelector('.doc-pane:not([hidden]) .cm-content, .app:not([hidden]) .cm-content'); editor?.focus(); document.execCommand('insertText', false, '\\nAutomation edit'); true" > /dev/null
wait_js "document.querySelector('.app:not([hidden]) .cm-content')?.textContent?.includes('Automation edit')"
auto menu File Save --settle > /dev/null
save_deadline=$((SECONDS + 10))
while ! grep -q 'Automation edit' "$FIXTURES/Research/Discussion.md"; do
  if (( SECONDS >= save_deadline )); then
    echo 'ASSERT FAIL: native Save did not write the edit' >&2
    js 'document.querySelector(".app:not([hidden]) .cm-content")?.textContent' >&2 || true
    cat "$FIXTURES/Research/Discussion.md" >&2
    exit 1
  fi
  sleep 0.1
done
echo 'PASS  native File > Save writes the active Markdown document'
auto menu Edit Undo --settle > /dev/null
wait_js "!document.querySelector('.app:not([hidden]) .cm-content')?.textContent?.includes('Automation edit')"
echo 'PASS  native Edit > Undo targets the active editor history'
auto menu Edit Redo --settle > /dev/null
wait_js "document.querySelector('.app:not([hidden]) .cm-content')?.textContent?.includes('Automation edit')"
echo 'PASS  native Edit > Redo restores the same editor edit'
auto menu File Save --settle > /dev/null

auto screenshot "$WORK/multi-workspace.png" > /dev/null
cp "$WORK/multi-workspace.png" "$REPORT"

quit
rm -f "$SOCKET"
launch
wait_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 3"
assert_js "document.querySelector('.doc-tab.sel')?.textContent?.includes('Discussion.md')" 'selected document restored after restart'
assert_js "document.querySelector('[data-testid=\"view-mode-markdown\"]')?.getAttribute('aria-checked') === 'true'" 'view mode restored after restart'
assert_js "document.querySelector('[data-testid=\"workspace-switcher\"]')?.textContent?.includes('Research')" 'selected Workspace restored after restart'
assert_js "document.querySelector('.app:not([hidden]) .cm-content')?.textContent?.includes('Automation edit')" 'saved editor content restored after restart'

quit
auto close > /dev/null || true
echo "PASS  native Automation smoke completed; screenshot: $REPORT"
