#!/usr/bin/env sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"

: "${GOPATH:=$SCRIPT_DIR/.gopath}"
: "${GOMODCACHE:=$GOPATH/pkg/mod}"
: "${GOCACHE:=$SCRIPT_DIR/.gocache}"
export GOPATH GOMODCACHE GOCACHE

mkdir -p "$GOMODCACHE" "$GOCACHE"

if [ "$#" -eq 0 ]; then
  set -- -config "./tasks.example.csv" -workers 4
fi

exec go run ./cmd/bubblecopy "$@"
