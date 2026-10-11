#!/usr/bin/env bash
# Runs the checks in CONSTRAINTS.md, fastest first, and stops at the first failure.
# The git pre-commit hook and the Claude Code Stop hook both run it.
#
#   scripts/check.sh             # every check
#   scripts/check.sh deps lint   # only the named checks
#
# Checks: fmt deps vet lint shell kit runner test swift
# Strict mode: stop on any failing command, unset variable or failed pipe.
# Ref: https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

# Direct dependencies allowed in go.mod. Tiger Style: "a zero dependencies policy"; adding one
# means adding it here, on purpose, in review.
# Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#dependencies-and-tooling
allowed_dependencies='^(github\.com/cucumber/godog|github\.com/cucumber/gherkin/go/v[0-9]+|github\.com/cucumber/messages/go/v[0-9]+|golang\.org/x/tools)$'

check_fmt() {
  local unformatted
  unformatted=$(gofmt -l .)
  if [ -n "$unformatted" ]; then
    echo "not gofmt-formatted (run gofmt -w on them):"
    echo "$unformatted"
    return 1
  fi
}

check_deps() {
  local direct unexpected
  # -m all lists every module; keep the direct (non-indirect) ones that aren't this module.
  # Ref: https://go.dev/ref/mod#go-list-m
  direct=$(go list -m -f '{{if not (or .Main .Indirect)}}{{.Path}}{{end}}' all)
  unexpected=$(printf '%s\n' "$direct" | grep -Ev "$allowed_dependencies" | grep -v '^$' || true)
  if [ -n "$unexpected" ]; then
    echo "dependencies not on the allowlist in scripts/check.sh:"
    echo "$unexpected"
    return 1
  fi
}

check_vet() { go vet ./...; }

check_lint() { go run ./tools/tigerlint ./...; }

check_shell() {
  # Ref: https://www.shellcheck.net/wiki/
  shellcheck scripts/*.sh .githooks/* .claude/hooks/*.sh internal/kit/givens.sh \
    internal/kit/*.bats
  # Setup scripts and helpers: the starter project's and the sample shop's. Their folders'
  # .shellcheckrc says they're bash.
  find internal/starter testdata/sample-shop -name '*.sh' -print0 | xargs -0 shellcheck
}

# The kit's bats tests, one per The kit scenario as they're built.
# Ref: https://bats-core.readthedocs.io/en/stable/usage.html
check_kit() { bats internal/kit; }

# The sample shop's cucumber-js, for scenarios that run a test runner over the same files.
check_runner() { scripts/runner.sh; }

check_test() { go test ./...; }

check_swift() {
  # swift-format ships with the Swift 6 toolchain; its config is macos/.swift-format.
  # Ref: https://github.com/swiftlang/swift-format#readme
  swift format lint --strict --recursive macos/Package.swift macos/Sources macos/Tests
  # Builds the macOS app and runs its Gherkin suites; GIVENS_TAGS picks scenarios as for go test.
  # Ref: https://docs.swift.org/swiftpm/documentation/packagemanagerdocs/swifttest
  swift test --package-path macos
}

checks=("$@")
if [ ${#checks[@]} -eq 0 ]; then
  checks=(fmt deps vet lint shell kit runner test swift)
fi
for name in "${checks[@]}"; do
  echo "== $name"
  "check_$name"
done
echo "== all checks passed"
