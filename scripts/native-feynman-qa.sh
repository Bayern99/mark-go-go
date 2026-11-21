#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This acceptance helper runs the native macOS application." >&2
  exit 2
fi

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_root="${TMPDIR:-/tmp}"
qa_root="$(mktemp -d "$tmp_root/markgogo-feynman-native-qa.XXXXXX")"
mkdir -p "$qa_root/bin" "$qa_root/home" "$qa_root/workspace"
chmod 700 "$qa_root/home" "$qa_root/workspace"
cat > "$qa_root/workspace/notes.md" <<'EOF'
# Local Research Fixture

A credential-free desktop acceptance fixture.
EOF
chmod 600 "$qa_root/workspace/notes.md"

cd "$repo_root"
go test -c -o "$qa_root/bin/feynman-fixture" ./internal/agent/feynman
go build -o "$qa_root/bin/mark-go-go-qa" .
cat > "$qa_root/bin/feynman" <<'EOF'
#!/bin/sh
set -eu
if [ "${1-}" = "--mode" ] && [ "${2-}" = "rpc" ]; then
  export MGO_FIXTURE_FEYNMAN_RPC=1
else
  export MGO_FIXTURE_FEYNMAN_CLI=1
fi
exec "$MGO_NATIVE_FIXTURE_BINARY" "$@"
EOF
chmod 700 "$qa_root/bin/feynman"

printf 'QA_ROOT=%s\nWORKSPACE_ROOT=%s\n' "$qa_root" "$qa_root/workspace"
unset MGO_FIXTURE_FEYNMAN_RPC MGO_FIXTURE_FEYNMAN_CLI || true
export HOME="$qa_root/home"
export FEYNMAN_HOME="$qa_root/home"
export PATH="$qa_root/bin:$PATH"
export MGO_NATIVE_FIXTURE_BINARY="$qa_root/bin/feynman-fixture"
cd "$qa_root/workspace"
exec "$qa_root/bin/mark-go-go-qa"
