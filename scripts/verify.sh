#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "==> Checking Go formatting"
unformatted="$(find . -name '*.go' -not -path './third_party/*' -not -path './.git/*' -print0 | xargs -0 gofmt -l)"
if [ -n "$unformatted" ]; then
  printf 'Go files need gofmt:\n%s\n' "$unformatted" >&2
  exit 1
fi

echo "==> Running go vet ./..."
go vet ./...

echo "==> Running go tool mygo vet"
go tool mygo vet

echo "==> Running go test -race ./..."
go test -race ./...

echo "==> Cross-compiling Windows host"
win_out="$(mktemp -d)"
trap 'rm -rf "$win_out"' EXIT
GOOS=windows GOARCH=amd64 go build -o "$win_out/" ./...
rm -rf "$win_out"

echo "==> All verification checks passed"
