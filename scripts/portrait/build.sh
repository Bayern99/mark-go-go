#!/usr/bin/env bash
set -euo pipefail
umask 077

revision=${1:?supply the Git revision}
destination=${2:?supply a new absolute directory outside the checkout}
root=$(git rev-parse --show-toplevel)
if [[ "$destination" != /* ]]; then
  echo 'Use an absolute destination' >&2
  exit 2
fi
destination=$(python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).resolve())' "$destination")
root=$(cd "$root" && pwd -P)
if [[ "$destination/" == "$root/"* ]]; then
  echo 'Store evidence outside the checkout' >&2
  exit 2
fi
mkdir -p "$(dirname "$destination")"
mkdir "$destination"
source_dir="$destination/source"
mkdir "$source_dir"
git -C "$root" rev-parse --verify --end-of-options "$revision^{commit}" > "$destination/revision.txt"
git -C "$root" archive "$(cat "$destination/revision.txt")" | tar -x -C "$source_dir"
cd "$source_dir"
python3 - <<'PY'
import json
from pathlib import Path
from uuid import uuid4

config = Path("mygo.json")
data = json.loads(config.read_text())
suffix = uuid4().hex[:12]
data.update(name="Mark Go Go Portrait " + suffix, identifier="dev.markgogo.portrait." + suffix,
            slug="mark-go-go-portrait-" + suffix)
config.write_text(json.dumps(data, indent=2) + "\n")
PY
if [[ -f web/package-lock.json ]] && python3 -c 'import json, sys; sys.exit(not json.load(open("mygo.json")).get("buildCommand"))'; then
  npm --prefix web ci
fi
go tool mygo build -skip-dmg -skip-notarize -o "$destination/bundle"
