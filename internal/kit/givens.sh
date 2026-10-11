# givens kit 1
# shellcheck shell=bash
# The Givens kit. Every setup script sources it on its first line:
#
#   source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
#
# Written for bash 3.2, the bash macOS ships: no associative arrays, no mapfile, no ${x,,}.
# Functions and variables are listed in capabilities/the-kit/about.md in the Givens repo.
# Names starting with _givens or _g_ are the kit's own and may change.

# Strict mode: stop on a failing command, an unset variable or a failed pipe.
# Ref: https://www.gnu.org/software/bash/manual/html_node/The-Set-Builtin.html
set -euo pipefail

# BASH_SOURCE[0] is this file; BASH_SOURCE[1] is the script that sourced it.
# Ref: https://www.gnu.org/software/bash/manual/html_node/Bash-Variables.html#index-BASH_005fSOURCE
GIVENS_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GIVENS_SCRIPT="${BASH_SOURCE[1]:-}"
export GIVENS_ROOT GIVENS_SCRIPT

# --when reruns a script for its When part only; see when_mode. "${1+"$@"}" is "$@" without
# bash 3.2's unbound-variable error when there are no arguments.
# Ref: https://www.gnu.org/software/bash/manual/html_node/Special-Parameters.html
GIVENS_WHEN="${GIVENS_WHEN:-}"
for _g_arg in ${1+"$@"}; do
  if [ "$_g_arg" = "--when" ]; then GIVENS_WHEN=1; fi
done
export GIVENS_WHEN

# Colour only when printing to a terminal, so a log file holds plain text.
# Ref: https://www.gnu.org/software/bash/manual/html_node/Bash-Conditional-Expressions.html
if [ -t 1 ]; then
  _g_green=$'\033[32m' _g_yellow=$'\033[33m' _g_red=$'\033[31m' _g_bold=$'\033[1m'
  _g_reset=$'\033[0m'
else
  _g_green='' _g_yellow='' _g_red='' _g_bold='' _g_reset=''
fi

# ok <message>: one piece of state was set up.
ok() {
  printf '%s✓%s %s\n' "$_g_green" "$_g_reset" "$*"
}

# warn <message>: something is off, but setup carries on.
warn() {
  printf '%s!%s %s\n' "$_g_yellow" "$_g_reset" "$*"
}

# fail <message>: setup can't go on.
fail() {
  printf '%s✗%s %s\n' "$_g_red" "$_g_reset" "$*"
  exit 1
}

# need <command> [hint]: stops setup, before anything changes, when a command is missing.
# Ref: https://www.gnu.org/software/bash/manual/html_node/Bash-Builtins.html#index-command
need() {
  if command -v "$1" >/dev/null 2>&1; then return 0; fi
  if [ $# -ge 2 ]; then fail "needs $1: $2"; fi
  fail "needs $1"
}

# The "your turn" lines: what the developer does by hand once setup is done. The first of them
# prints a "Your turn" heading; GIVENS_QUIET=1 hides them all, since Givens shows the
# scenario's steps as a checklist instead.
_g_turn_started=''

_givens_turn() {
  if [ "${GIVENS_QUIET:-}" = "1" ]; then return 0; fi
  if [ -z "$_g_turn_started" ]; then
    printf '\n%sYour turn%s\n\n' "$_g_bold" "$_g_reset"
    _g_turn_started=1
  fi
  printf '%s\n' "$*"
}

# first_you <text>: a Given done by hand, like 'select "Checkout"'.
first_you() { _givens_turn "First, $*"; }

# when_you <text>: one When step, like 'click "Place order"'.
when_you() { _givens_turn "When you $*"; }

# then_check <text>: one Then step, like 'you see "Saved"'.
then_check() { _givens_turn "Then check that $*"; }

# when_mode: true when the script was run with --when, to do only its When part.
when_mode() { [ "$GIVENS_WHEN" = "1" ]; }

# when_command: the command that reruns this script with --when.
when_command() { printf 'bash %s --when' "$GIVENS_SCRIPT"; }

# _givens_keyboard: true when someone can answer at a terminal. /dev/tty opens only with a
# controlling terminal, which CI doesn't have.
# Ref: https://man7.org/linux/man-pages/man4/tty.4.html
_givens_keyboard() { { : </dev/tty; } 2>/dev/null; }

# _givens_card <kind> <where> <link> <qr> <text>: shows a card in Givens (GIVENS_UI=1). One
# line starts with the record separator and "givens-card", then tab-separated fields, the last
# being how many lines of text follow. Givens answers on stdin.
_givens_card() {
  local _g_lines=0
  if [ -n "$5" ]; then _g_lines="$(printf '%s\n' "$5" | wc -l | tr -d ' ')"; fi
  printf '\036givens-card\t%s\t%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$4" "$_g_lines"
  if [ -n "$5" ]; then printf '%s\n' "$5"; fi
}

# by_hand [--url <url>] [--qr <url>] <where> [text]: a step done somewhere setup can't reach.
# The text comes from the second argument or from stdin, like a heredoc. Setup waits until
# it's marked done; "f" marks it failed and stops setup. In Givens it's a card instead.
by_hand() {
  local _g_link='' _g_qr=0 _g_where _g_text='' _g_answer
  while [ $# -gt 0 ]; do # bound: each pass shifts the options away
    case "$1" in
      --url) _g_link="$2"; _g_qr=0; shift 2 ;;
      --qr) _g_link="$2"; _g_qr=1; shift 2 ;;
      *) break ;;
    esac
  done
  _g_where="${1:-}"
  if [ $# -ge 2 ]; then _g_text="$2"; elif [ ! -t 0 ] && [ "${GIVENS_UI:-}" != "1" ]; then
    _g_text="$(cat)"
  fi
  if [ "${GIVENS_UI:-}" = "1" ]; then
    _givens_card by_hand "$_g_where" "$_g_link" "$_g_qr" "$_g_text"
    IFS= read -r _g_answer || _g_answer=''
    if [ "$_g_answer" != "done" ]; then fail "Stopped at a step done by hand"; fi
    return 0
  fi
  if ! _givens_keyboard; then fail "by_hand needs someone at the keyboard"; fi
  printf '\n%sBy hand, in %s:%s\n\n' "$_g_bold" "$_g_where" "$_g_reset"
  if [ -n "$_g_text" ]; then printf '%s\n\n' "$_g_text"; fi
  if [ -n "$_g_link" ]; then printf '%s\n\n' "$_g_link"; fi
  printf 'Press Enter when done, or type f if it failed '
  IFS= read -r _g_answer </dev/tty || _g_answer=''
  if [ "$_g_answer" = "f" ]; then fail "Stopped at a step done by hand"; fi
}

# ask <name> <prompt>: stops setup until a value is typed, then sets the variable <name>.
# Ref: https://www.gnu.org/software/bash/manual/html_node/Bash-Builtins.html#index-printf
ask() {
  local _g_name="$1" _g_prompt="$2" _g_value
  case "$_g_name" in
    '' | [0-9]* | *[!A-Za-z0-9_]*) fail "ask: $_g_name isn't a variable name" ;;
  esac
  if [ "${GIVENS_UI:-}" = "1" ]; then
    _givens_card ask "$_g_name" "" 0 "$_g_prompt"
    IFS= read -r _g_value || _g_value=''
    printf -v "$_g_name" '%s' "$_g_value"
    return 0
  fi
  if ! _givens_keyboard; then fail "ask needs someone at the keyboard"; fi
  printf '%s: ' "$_g_prompt"
  IFS= read -r _g_value </dev/tty || _g_value=''
  printf -v "$_g_name" '%s' "$_g_value"
}

# from <capability>/<slug> [column=value …]: runs that scenario's teardown, then its setup,
# before this one, with its "your turn" lines hidden. GIVENS_FROM_CHAIN holds the run keys on
# the way here, so setups that build on each other in a loop stop instead of running forever.
from() {
  local _g_key="${1:-}" _g_script _g_chain _g_pair
  if [ -z "$_g_key" ]; then fail "from needs a scenario, like checkout/pay-with-saved-card"; fi
  shift
  _g_script="$GIVENS_ROOT/${_g_key%%/*}/scripts/${_g_key#*/}.sh"
  if [ ! -f "$_g_script" ]; then fail "from $_g_key: there's no scenario with that name"; fi
  _g_chain="${GIVENS_FROM_CHAIN:-$(_givens_key "$GIVENS_SCRIPT")}"
  case " $_g_chain " in
    *" $_g_key "*) fail "from loop: $(_givens_slugs "$_g_chain $_g_key")" ;;
  esac
  printf '→ from %s\n' "$_g_key${1+ $*}"
  (
    export GIVENS_QUIET=1 GIVENS_SCENARIO="$_g_key" GIVENS_FROM_CHAIN="$_g_chain $_g_key"
    for _g_pair in ${1+"$@"}; do
      case "${_g_pair%%=*}" in
        '' | [0-9]* | *[!A-Za-z0-9_]*) fail "from: $_g_pair isn't column=value" ;;
      esac
      export "GIVENS_EX_${_g_pair%%=*}=${_g_pair#*=}"
    done
    if [ -f "${_g_script%.sh}.teardown.sh" ]; then bash "${_g_script%.sh}.teardown.sh"; fi
    bash "$_g_script"
  )
}

# _givens_key <script>: the run key of a setup script's path, like checkout/card-declined.
_givens_key() {
  local _g_dir
  _g_dir="$(dirname "$(dirname "$1")")"
  printf '%s/%s' "$(basename "$_g_dir")" "$(basename "$1" .sh)"
}

# _givens_slugs <keys>: the keys' slugs joined with arrows, for the loop message.
_givens_slugs() {
  local _g_key _g_out=''
  for _g_key in $1; do
    _g_out="${_g_out:+$_g_out → }${_g_key#*/}"
  done
  printf '%s' "$_g_out"
}

# _givens_load <folder>: sources every <folder>/*.sh but the kit itself, in name order. Two
# files in one folder defining the same function get a warning naming both, the one in effect
# first; a capability's helper overriding a shared one is expected, so it's quiet.
_givens_load() {
  local _g_file _g_name _g_other _g_seen=''
  for _g_file in "$1"/*.sh; do
    [ -f "$_g_file" ] || continue
    [ "$(basename "$_g_file")" = "givens.sh" ] && continue
    while IFS= read -r _g_name; do
      _g_other="$(printf '%s' "$_g_seen" | awk -v name="$_g_name" '$1 == name { print $2 }')"
      if [ -n "$_g_other" ]; then
        warn "$_g_name is defined in both ${_g_file#"$GIVENS_ROOT"/} and $_g_other"
      fi
      _g_seen="$_g_seen$_g_name ${_g_file#"$GIVENS_ROOT"/}
"
    done < <(sed -n 's/^\([A-Za-z_][A-Za-z0-9_]*\)[[:space:]]*()[[:space:]]*{.*/\1/p' "$_g_file")
    # shellcheck source=/dev/null
    source "$_g_file"
  done
}

# Shared helpers first, then the calling script's capability's own, which may override them.
_g_own=''
if [ -n "$GIVENS_SCRIPT" ]; then _g_own="$(cd "$(dirname "$GIVENS_SCRIPT")/.." && pwd)"; fi
_givens_load "$GIVENS_ROOT/_shared"
if [ -n "$_g_own" ]; then _givens_load "$_g_own/_shared"; fi

# Setup runs from the git repo's root, or the folder holding capabilities outside one, so a
# script copied as `bash capabilities/…` works from any folder inside the project.
# Ref: https://git-scm.com/docs/git-rev-parse#Documentation/git-rev-parse.txt---show-toplevel
cd "$(git -C "$GIVENS_ROOT" rev-parse --show-toplevel 2>/dev/null || dirname "$GIVENS_ROOT")"
