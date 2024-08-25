#!/bin/sh

set -e

(
  cd "$(dirname "$0")"
  go build -buildvcs="false" -o /tmp/ben/git ./cmd/mygit
)

exec /tmp/ben/git "$@"
