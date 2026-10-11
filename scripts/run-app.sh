#!/usr/bin/env bash
# Builds and runs the Givens app for a look on screen, with the givens command in Helpers/
# beside it, where the app looks for it in a development build.
#
#   scripts/run-app.sh                       # opens on the welcome window
#   scripts/run-app.sh testdata/sample-shop  # then opens a folder, as Dock drop would
#
# Ref: https://docs.swift.org/swiftpm/documentation/packagemanagerdocs/swiftbuild
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

swift build --package-path macos --product Givens
bin="$(swift build --package-path macos --show-bin-path)"
mkdir -p "$bin/Helpers"
go build -o "$bin/Helpers/givens" ./cmd/givens
if [ $# -gt 0 ]; then
  folder="$(cd "$1" && pwd)"
  # A dev build isn't a bundle `open -a` can find, so the folder goes on the command line,
  # which AppKit may hand to application(_:open:). If no window opens it, use Open folder….
  exec "$bin/Givens" "$folder"
fi
exec "$bin/Givens"
