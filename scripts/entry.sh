#!/usr/bin/env bash
# Finds the part of the big file, internal/app/big.go, that tells a Feature, Rule or Scenario,
# and prints its lines and the functions in them, so a reader (or an agent) opens only that
# slice. A name matches word for word first, then as part of a heading, ignoring case. A
# scenario with no comment of its own finds its Rule's section, or its Feature's. Line numbers
# are worked out each time, never stored. See tasks/one-big-file/notes.md.
#
#   scripts/entry.sh "Copy from search"          # a Scenario, Rule or Feature, by name
#   scripts/entry.sh --debug "Copy"              # print a delve command for the scenario
#   scripts/entry.sh --debug --run "Copy"        # start it, stopped in the section's functions
#
# Strict mode: stop on any failing command, unset variable or failed pipe.
# Ref: https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

big_file=internal/app/big.go

usage() {
  echo "usage: scripts/entry.sh [--debug [--run]] \"Feature, Rule or Scenario name\"" >&2
  exit 2
}

# heading prints "<line> <kind>" for the big file's first Feature, Rule or Scenario comment
# named $1: word for word if one is, else the first that contains it, ignoring case.
heading() {
  awk -v want="$1" '
    BEGIN { want = tolower(want) }
    /^\/\/ (Feature|Rule|Scenario): / {
      kind = $2; sub(/:$/, "", kind)
      name = tolower(substr($0, length($2) + 5))
      if (name == want) { print NR, kind; found = 1; exit }
      if (first == "" && index(name, want) > 0) first = NR " " kind
    }
    END { if (!found && first != "") print first }
  ' "$big_file"
}

# story prints "<feature path>|<Feature>|<Rule>|<Scenario>" for the scenario named $1, matched
# as heading matches, Rule empty outside one. Ref: https://cucumber.io/docs/gherkin/reference/
story() {
  awk -v want="$1" '
    BEGIN { want = tolower(want) }
    function named(line) { sub(/^[[:space:]]*[A-Za-z ]+:[[:space:]]*/, "", line); return line }
    FNR == 1 { rule = "" }
    /^[[:space:]]*Feature:/ { feature = named($0) }
    /^[[:space:]]*Rule:/ { rule = named($0) }
    /^[[:space:]]*Scenario( Outline)?:/ {
      name = named($0)
      found = FILENAME "|" feature "|" rule "|" name
      if (tolower(name) == want) { exact = found; exit }
      if (first == "" && index(tolower(name), want) > 0) first = found
    }
    END { if (exact != "") print exact; else if (first != "") print first }
  ' capabilities/*/*.feature
}

# bounds prints "<first>-<last>" for the section starting at line $1, a $2 heading. A Feature
# runs to the next Feature; a Rule to the next Rule or Feature; a Scenario comment, with any
# right below it, to the next heading.
bounds() {
  awk -v from="$1" -v kind="$2" '
    NR <= from { last = NR; next }
    /^\/\/ Feature: / { stop = 1 }
    /^\/\/ Rule: / && kind != "Feature" { stop = 1 }
    /^\/\/ Scenario: / && kind == "Scenario" { if (NR == last + 1) { last = NR; next } stop = 1 }
    stop { print from "-" NR - 1; done = 1; exit }
    END { if (!done) print from "-" NR }
  ' "$big_file"
}

# functions prints "<line> <name>" for each func that starts within lines $1 to $2.
functions() {
  awk -v first="$1" -v last="$2" '
    NR >= first && NR <= last && /^func / {
      name = $0
      sub(/\(.*$/, "", name)
      if ($2 ~ /^\(/) { name = $0; sub(/^func \([^)]*\) /, "", name); sub(/\(.*$/, "", name) }
      else sub(/^func /, "", name)
      print NR, name
    }
  ' "$big_file"
}

# breakpoint turns the func on line $1 into delve's name for it:
# "func (w *Window) copySetup(" becomes givens/internal/app.(*Window).copySetup.
# Ref: https://github.com/go-delve/delve/blob/master/Documentation/cli/locspec.md
breakpoint() {
  sed -n "${1}p" "$big_file" | sed -E \
    -e 's/^func \([a-z]+ (\*?)([A-Za-z0-9_]+)\) ([A-Za-z0-9_]+)\(.*/(\1\2).\3/' \
    -e 's/^func ([A-Za-z0-9_]+)\(.*/\1/' \
    -e "s|^|givens/$(dirname "$big_file").|"
}

debug=0
run=0
target=""
for arg in "$@"; do
  case "$arg" in
    --debug) debug=1 ;;
    --run) run=1 ;;
    --*) usage ;;
    *) target="$arg" ;;
  esac
done
[ -n "$target" ] || usage

found=$(heading "$target")
told=$(story "$target")
if [ -z "$found" ] && [ -n "$told" ]; then
  # No comment of its own: find the scenario's Rule's section, else its Feature's.
  IFS='|' read -r _ feature rule _ <<< "$told"
  [ -n "$rule" ] && found=$(heading "$rule")
  [ -n "$found" ] || found=$(heading "$feature")
fi
[ -n "$found" ] || { echo "nothing in $big_file is named \"$target\"; is its story in yet?" >&2
  exit 1; }
start=${found% *}
range=$(bounds "$start" "${found#* }")
listed=$(functions "${range%-*}" "${range#*-}")

echo "$big_file:$range  $(sed -n "${start}p" "$big_file" | sed 's|^// ||')"
if [ -n "$listed" ]; then
  echo "$listed" | while read -r line name; do echo "  $big_file:$line $name"; done
fi
[ "$debug" = 1 ] || exit 0

[ -n "$told" ] || { echo "--debug takes a scenario's name, to know which test to run" >&2
  exit 1; }
[ -n "$listed" ] || { echo "no functions in $big_file:$range to stop in" >&2; exit 1; }
IFS='|' read -r path _ _ scenario <<< "$told"

# godog runs each scenario as a subtest named after it, spaces as underscores; -test.run
# matches each part of the name as a regular expression, so the name's own symbols are escaped.
# Ref: https://pkg.go.dev/cmd/go#hdr-Testing_flags
subtest=$(printf '%s' "$scenario" | sed -e 's/[][\.*^$+?(){}|]/\\&/g' -e 's/ /_/g')
suite="./acceptance/$(basename "$(dirname "$path")")"
commands=""
while read -r line _; do
  commands="${commands}break $(breakpoint "$line")"$'\n'
done <<< "$listed"

# dlv test builds the package's tests and runs them under the debugger; --init runs the
# breakpoints and continue before the prompt. Ref:
# https://github.com/go-delve/delve/blob/master/Documentation/usage/dlv_test.md
if [ "$run" = 0 ]; then
  printf 'dlv test %s -- -test.run %q\n' "$suite" "TestScenarios/^${subtest}\$"
  printf 'then at (dlv):\n%scontinue\n' "$commands"
  exit 0
fi
init=$(mktemp)
printf '%scontinue\n' "$commands" > "$init"
exec dlv test "$suite" --init "$init" -- -test.run "TestScenarios/^${subtest}\$"
