#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
WORK=$(mktemp -d /tmp/mgo-native.XXXXXX)
SESSION=$(basename "$WORK")
APP_NAME="Mark Go Go QA $SESSION"
SOCKET="$WORK/app.sock"
PROJECT="$WORK/project"
REPORT=${MYGO_AUTOMATION_REPORT:-/tmp/mgo-native-smoke.png}
SOURCE="$WORK/notes/Discussion.md"
AUTO="$WORK/mygo-automation"
PROCESS=""

cleanup() {
  if [[ -x "$AUTO" ]]; then "$AUTO" close --session "$SESSION" >/dev/null 2>&1 || true; fi
  if [[ -n "$PROCESS" ]] && kill -0 "$PROCESS" 2>/dev/null; then
    kill "$PROCESS" 2>/dev/null || true
    deadline=$((SECONDS+3))
    while kill -0 "$PROCESS" 2>/dev/null && (( SECONDS < deadline )); do sleep 0.1; done
    if kill -0 "$PROCESS" 2>/dev/null; then kill -KILL "$PROCESS" 2>/dev/null || true; fi
    wait "$PROCESS" 2>/dev/null || true
  fi
  rm -rf "$WORK" "$HOME/Library/Application Support/$APP_NAME"
}
trap cleanup EXIT
mkdir -p "$PROJECT" "$(dirname "$SOURCE")"
cp "$ROOT"/go.{mod,sum} "$PROJECT/"
ln -s "$ROOT"/third_party "$ROOT"/internal "$ROOT"/scripts "$ROOT"/cmd "$PROJECT/"
mkdir -p "$PROJECT/resources"
ln -s "$ROOT"/resources/icon.png "$PROJECT/resources/icon.png"
ln -s "$ROOT"/*.go "$PROJECT/"
python3 - "$ROOT/mygo.json" "$PROJECT/mygo.json" "$APP_NAME" "$SESSION" <<'PY'
import json, sys
src, dest, name, session = sys.argv[1:]
with open(src) as f:
    cfg = json.load(f)
cfg.update(name=name, slug="mgo-" + session.replace(".", "-").lower(),
           identifier="dev.markgogo.qa." + session.replace(".", "-").lower())
with open(dest, "w") as f:
    json.dump(cfg, f)
PY
(cd "$PROJECT" && go build -o "$AUTO" github.com/egoist/mygo/plugins/automation/cmd/mygo-automation &&
    GOFLAGS=-tags=e2e go tool mygo build -debug -skip-dmg -skip-notarize -o "$WORK/build")
APP=$(find "$WORK/build" -name '*.app' -type d -print -quit)
EXECUTABLE=$(find -L "$APP/Contents/MacOS" -type f -perm -111 -print -quit)
auto() { "$AUTO" "$@" --session "$SESSION"; }
launch() {
  MYGO_AUTOMATION="$SOCKET" "$EXECUTABLE" "$@" >"$WORK/app.log" 2>&1 &
  PROCESS=$!
  local deadline=$((SECONDS+25))
  until [[ -S "$SOCKET" ]] && auto open --socket "$SOCKET" >/dev/null 2>&1; do
    (( SECONDS < deadline )) && kill -0 "$PROCESS" || { cat "$WORK/app.log" >&2; return 1; }
    sleep 0.1
  done
  until auto windows | grep -q '\[content\]'; do
    (( SECONDS < deadline )) && kill -0 "$PROCESS" || { cat "$WORK/app.log" >&2; return 1; }
    sleep 0.1
  done
}
quit() {
  auto menu "$APP_NAME" "Quit $APP_NAME"
  local deadline=$((SECONDS+25))
  while kill -0 "$PROCESS" 2>/dev/null; do
    (( SECONDS < deadline )) || { cat "$WORK/app.log" >&2; return 1; }
    sleep 0.1
  done
  wait "$PROCESS"
  PROCESS=""
  auto close >/dev/null
  rm -f "$SOCKET"
}
document_window() {
  auto wait --text "$1" --timeout 20s
  auto windows | tee "$WORK/windows"
  grep -F '[content]' "$WORK/windows" | grep -F "$2"
  auto snapshot --full >"$WORK/tree"
  grep -F '[textField]' "$WORK/tree" | grep -F "$3" | grep -F "$1"
  if grep -i -q 'webview' "$WORK/tree"; then return 1; fi
}
printf '# Welcome\n' >"$WORK/notes/Welcome.md"
printf '# Discussion\r\n\nOriginal bytes.\n' >"$SOURCE"
cp "$SOURCE" "$WORK/original"
printf '前🙂' >"$WORK/expected"
cat "$SOURCE" >>"$WORK/expected"

launch "$WORK/notes/Welcome.md"
document_window 'Welcome' 'Welcome.md' 'Markdown Preview'
open -a "$APP" "$SOURCE"
document_window 'Original bytes.' 'Discussion.md' 'Markdown Preview'
auto press key=Markdown --settle
document_window 'Original bytes.' 'Discussion.md' 'Markdown source'
auto press 'key="Close Discussion.md"' --settle
auto press key=file:Discussion.md --settle
document_window 'Original bytes.' 'Discussion.md' 'Markdown source'
cmp "$WORK/original" "$SOURCE"
field=$(awk '/\[textField\] "Markdown source"/ {print $1; exit}' "$WORK/tree")
auto focus "$field"
auto key CmdOrCtrl+Home
auto type '前🙂' --settle
auto wait --text '前🙂# Discussion'
cmp "$WORK/original" "$SOURCE"
auto menu File Save --settle
deadline=$((SECONDS+10))
until cmp -s "$WORK/expected" "$SOURCE"; do (( SECONDS < deadline )) || { cat "$WORK/app.log" >&2; exit 1; }; sleep 0.1; done
auto menu Edit Undo --settle
auto wait --text '前🙂# Discussion' --gone
auto menu Edit Redo --settle
auto wait --text '前🙂# Discussion'
auto menu File Save --settle
auto screenshot "$REPORT"
quit
launch
document_window '前🙂# Discussion' 'Discussion.md' 'Markdown source'
cmp "$WORK/expected" "$SOURCE"
field=$(awk '/\[textField\] "Markdown source"/ {print $1; exit}' "$WORK/tree")
auto focus "$field"
auto key CmdOrCtrl+End
auto type $'\nDiscard this draft'
auto wait --text 'Discard this draft'
auto press 'key="Close Discussion.md"'
auto wait --text "Don't Save"
# Cancel dismisses the sheet and keeps the dirty document open, edits and all.
auto press 'key="Cancel"' --settle
auto wait --text "Don't Save" --gone
auto wait --text 'Discard this draft'
auto wait --text 'Close Discussion.md'
# It is still unsaved, so closing again asks again.
auto press 'key="Close Discussion.md"'
auto wait --text "Don't Save"
auto press 'key="Don'"'"'t Save"'
# Closing runs on a goroutine under the modal progress sheet: wait for the
# sidebar to return (Close Welcome.md visible) and Discussion's tab to be gone.
auto wait --text 'Close Welcome.md' --timeout 15s
auto wait --text 'Close Discussion.md' --gone
auto press key=file:Discussion.md --settle
document_window '前🙂# Discussion' 'Discussion.md' 'Markdown source'
if grep -F 'Discard this draft' "$WORK/tree"; then exit 1; fi
cmp "$WORK/expected" "$SOURCE"
quit
echo "PASS  native Content, Finder open, close/reopen, view restore, cancel and discard, editable source, menu save, undo, redo, and fresh-process restore. Screenshot $REPORT"
