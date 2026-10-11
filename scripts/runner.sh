#!/usr/bin/env bash
# Installs the sample shop's test runner, cucumber-js, into .cache/runner (gitignored), from
# testdata/sample-shop's package.json and package-lock.json. acceptance/world links it into
# each scenario's copy of the sample shop, so runner scenarios never reach the network.
# The first run needs the network or a warm npm cache; later runs do nothing.
#
#   scripts/runner.sh
#
# Ref: https://docs.npmjs.com/cli/commands/npm-ci
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

cache=.cache/runner
lock=testdata/sample-shop/package-lock.json

# npm ci writes node_modules/.package-lock.json; matching the lock means it's installed.
if cmp -s "$lock" "$cache/package-lock.json" && [ -x "$cache/node_modules/.bin/cucumber-js" ]; then
  exit 0
fi
mkdir -p "$cache"
cp testdata/sample-shop/package.json "$lock" "$cache/"
# --ignore-scripts: nothing in the lock gets to run install scripts on this machine.
(cd "$cache" && npm ci --prefer-offline --ignore-scripts --no-audit --no-fund)
