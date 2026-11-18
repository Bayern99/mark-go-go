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
RUN_ID=$(basename "$WORK" | tr 'A-Z' 'a-z')
SLUG_ID=${RUN_ID//./-}
APP_NAME="Mark Go Go Automation QA $RUN_ID"
APP="$WORK/build/darwin-arm64/$APP_NAME.app"
QA_DATA="$HOME/Library/Application Support/$APP_NAME"
PROCESS=""
mkdir -p "$PROJECT" "$FIXTURES/Research" "$FIXTURES/Music Department" "$FIXTURES/Personal Notes" "$FIXTURES/Work"

wait_for_exit() {
  local pid=$1
  local timeout=$2
  local deadline=$((SECONDS + timeout))
  while kill -0 "$pid" 2>/dev/null; do
    (( SECONDS < deadline )) || return 1
    sleep 0.1
  done
}

cleanup() {
  "${AUTO[@]}" close --session "$SESSION" >/dev/null 2>&1 || true
  if [[ -n "$PROCESS" ]] && kill -0 "$PROCESS" 2>/dev/null; then
    kill "$PROCESS" 2>/dev/null || true
    if ! wait_for_exit "$PROCESS" 5; then
      kill -KILL "$PROCESS" 2>/dev/null || true
    fi
    wait "$PROCESS" 2>/dev/null || true
  fi
  if [[ "$APP_NAME" == 'Mark Go Go Automation QA mgo-automation.'* ]]; then
    rm -rf "$QA_DATA"
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT INT TERM

# Only read-only dependencies are linked. Web sources are copied so binding
# generation and Vite output never write through to the caller's checkout.
cp "$ROOT"/go.mod "$ROOT"/go.sum "$PROJECT"/
ln -s "$ROOT"/third_party "$ROOT"/resources "$PROJECT"/
ln -s "$ROOT"/*.go "$PROJECT"/
mkdir -p "$PROJECT/web"
tar -C "$ROOT/web" --exclude=./node_modules --exclude=./dist -cf - . | tar -C "$PROJECT/web" -xf -
ln -s "$ROOT/web/node_modules" "$PROJECT/web/node_modules"
ROOT="$ROOT" PROJECT="$PROJECT" APP_NAME="$APP_NAME" RUN_ID="$RUN_ID" SLUG_ID="$SLUG_ID" node <<'NODE'
const fs = require('fs')
const config = JSON.parse(fs.readFileSync(`${process.env.ROOT}/mygo.json`, 'utf8'))
config.name = process.env.APP_NAME
config.slug = 'mark-go-go-automation-qa-' + process.env.SLUG_ID
config.identifier = 'dev.markgogo.editor.automation-qa.' + process.env.RUN_ID
config.out = 'build'
fs.writeFileSync(`${process.env.PROJECT}/mygo.json`, JSON.stringify(config, null, 2) + '\n')
NODE

printf '# Discussion\n\nResearch notes.\n' > "$FIXTURES/Research/Discussion.md"
printf '# CC129\n\nMusic notes.\n' > "$FIXTURES/Music Department/CC129.md"
printf '# Ideas\n\nPersonal notes.\n' > "$FIXTURES/Personal Notes/Ideas.md"
printf '{}\n' > "$FIXTURES/Research/resource.json"
printf '# Plan\n\nWork notes.\n' > "$FIXTURES/Work/Plan.md"

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
  if ! wait_js "$script"; then
    echo "ASSERT FAIL: $message" >&2
    cat "$WORK/app.log" >&2 || true
    exit 1
  fi
  echo "PASS  $message"
}
assert_active() {
  local document=$1
  local workspace=$2
  local text=$3
  wait_js "document.querySelector('.doc-tab.sel')?.textContent?.includes('$document') && document.querySelector('[data-testid=\"workspace-switcher\"]')?.textContent?.includes('$workspace') && document.querySelector('.app:not([hidden])')?.textContent?.includes('$text')"
  if ! auto windows | grep -q "\"$document\""; then
    echo "ASSERT FAIL: native title does not match $document" >&2
    auto windows >&2
    exit 1
  fi
  echo "PASS  $document / $workspace synchronized across tab, body, sidebar and native title"
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
  if ! wait_for_exit "$PROCESS" 20; then
    echo 'Application did not quit within 20 seconds' >&2
    return 1
  fi
  wait "$PROCESS" 2>/dev/null || true
  PROCESS=""
}

launch "$FIXTURES/Research/Discussion.md"
assert_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 1" 'initial Markdown opens'
assert_js "document.querySelector('[data-testid=\"workspace-switcher\"]')?.textContent?.includes('Research')" 'initial Workspace selected'
wait_js "document.querySelector('[data-testid=\"workspace-file-count\"]')?.textContent?.trim() === '1 file'"
assert_js "!document.body.textContent.includes('resource.json')" 'non-Markdown resource stays out of navigation after indexing'

open -a "$APP" "$FIXTURES/Music Department/CC129.md"
wait_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 2 && document.querySelector('[data-testid=\"workspace-switcher\"]')?.textContent?.includes('Music Department')"
assert_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 2" 'second folder joins the same document group'
assert_js "document.querySelector('.doc-tab.sel')?.textContent?.includes('CC129.md')" 'second document active'

js "document.querySelector('[data-testid=\"view-mode-markdown\"]')?.click(); true" > /dev/null
wait_js "document.querySelector('[data-testid=\"view-mode-markdown\"]')?.getAttribute('aria-checked') === 'true'"
open -a "$APP" "$FIXTURES/Personal Notes/Ideas.md"
wait_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 3"
open -a "$APP" "$FIXTURES/Work/Plan.md"
wait_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 4"
js "document.querySelector('.doc-tab[title$=\"/CC129.md\"]')?.click(); true" > /dev/null
assert_active 'CC129.md' 'Music Department' 'CC129'
# Page windows use eval rather than native-element input in the official CLI.
# Exercise MRU with four open documents and non-adjacent tabs; real-key coverage
# remains a separate native-input check, not a claim made by this script.
js "window.dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', ctrlKey: true, bubbles: true})); true" > /dev/null
assert_active 'Plan.md' 'Work' 'Plan'
js "window.dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', ctrlKey: true, bubbles: true})); true" > /dev/null
assert_active 'CC129.md' 'Music Department' 'CC129'
assert_js "document.querySelector('[data-testid=\"view-mode-markdown\"]')?.getAttribute('aria-checked') === 'true'" 'view mode survives cross-Workspace MRU switches'

open -a "$APP" "$FIXTURES/Research/Discussion.md"
assert_active 'Discussion.md' 'Research' 'Discussion'
assert_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 4" 'Finder duplicate reuses its document session without another tab'
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
if grep -q 'Automation edit' "$FIXTURES/Research/Discussion.md"; then
  echo 'ASSERT FAIL: edit reached disk before File > Save in the AutosaveOff QA build' >&2
  exit 1
fi
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
wait_js "document.querySelectorAll('.doc-tab[role=\"tab\"]').length === 4"
assert_js "document.querySelector('.doc-tab.sel')?.textContent?.includes('Discussion.md')" 'selected document restored after restart'
assert_js "document.querySelector('[data-testid=\"view-mode-markdown\"]')?.getAttribute('aria-checked') === 'true'" 'view mode restored after restart'
assert_js "document.querySelector('[data-testid=\"workspace-switcher\"]')?.textContent?.includes('Research')" 'selected Workspace restored after restart'
assert_js "document.querySelector('.app:not([hidden]) .cm-content')?.textContent?.includes('Automation edit')" 'saved editor content restored after restart'

quit
auto close > /dev/null || true
echo "PASS  native Automation smoke completed; screenshot: $REPORT"
