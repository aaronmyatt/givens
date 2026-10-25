#!/usr/bin/env bash
# Claude Code PostToolUse hook for Edit, Write and MultiEdit: checks the file Claude just
# changed and sends any failure back to Claude to fix. Exit 2 makes stderr Claude's feedback.
# Ref: https://docs.claude.com/en/docs/claude-code/hooks
set -euo pipefail

# The hook gets the tool call as JSON on stdin; tool_input.file_path is the edited file.
file=$(jq -r '.tool_input.file_path // empty')
[ -n "$file" ] || exit 0
root="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}"
cd "$root"

fail() {
  echo "$1" >&2
  echo "$2" >&2
  exit 2
}

case "$file" in
  */testdata/* | */_archive/*)
    exit 0
    ;;
  *.go)
    # Format in place, then lint the edited file's package.
    gofmt -w "$file"
    if ! out=$(go run ./tools/tigerlint "$(dirname "$file")" 2>&1); then
      fail "tigerlint found problems in $file's package (see CONSTRAINTS.md):" "$out"
    fi
    if [[ "$file" == */internal/app/big.go ]] &&
      ! out=$(go test ./acceptance -run 'TestBigFile' 2>&1); then
      fail "big.go's headings don't match the feature files:" "$out"
    fi
    ;;
  *.feature)
    if ! out=$(go test ./acceptance -run 'TestSpecConventions' 2>&1); then
      fail "$file breaks a convention in capabilities/GIVENS.md:" "$out"
    fi
    # A renamed Feature, Rule or Scenario must be renamed in internal/app/big.go's headings.
    if ! out=$(go test ./acceptance -run 'TestBigFile' 2>&1); then
      fail "$file no longer matches internal/app/big.go's headings:" "$out"
    fi
    ;;
  */go.mod)
    if ! out=$(scripts/check.sh deps 2>&1); then
      fail "go.mod adds a dependency that isn't allowed:" "$out"
    fi
    ;;
  *.sh | */.githooks/*)
    if ! out=$(shellcheck "$file" 2>&1); then
      fail "shellcheck found problems in $file:" "$out"
    fi
    ;;
esac
exit 0
