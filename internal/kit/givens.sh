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

# by_hand [--url <url>] [--qr <url>] <where> [text]: a step done somewhere setup can't reach.
# The text comes from the second argument or from stdin, like a heredoc. Setup waits until
# it's marked done; "f" marks it failed and stops setup.
by_hand() {
  local _g_link='' _g_where _g_text='' _g_answer
  while [ $# -gt 0 ]; do # bound: each pass shifts the options away
    case "$1" in
      --url | --qr) _g_link="$2"; shift 2 ;;
      *) break ;;
    esac
  done
  _g_where="${1:-}"
  if [ $# -ge 2 ]; then _g_text="$2"; elif [ ! -t 0 ]; then _g_text="$(cat)"; fi
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
  if ! _givens_keyboard; then fail "ask needs someone at the keyboard"; fi
  printf '%s: ' "$_g_prompt"
  IFS= read -r _g_value </dev/tty || _g_value=''
  printf -v "$_g_name" '%s' "$_g_value"
}

# _givens_load <folder>: sources every <folder>/*.sh but the kit itself, in name order.
_givens_load() {
  local _g_file
  for _g_file in "$1"/*.sh; do
    [ -f "$_g_file" ] || continue
    [ "$(basename "$_g_file")" = "givens.sh" ] && continue
    # shellcheck source=/dev/null
    source "$_g_file"
  done
}

# Shared helpers first, then the calling script's capability's own, which may override them.
_givens_load "$GIVENS_ROOT/_shared"
if [ -n "$GIVENS_SCRIPT" ]; then
  _givens_load "$(cd "$(dirname "$GIVENS_SCRIPT")/.." && pwd)/_shared"
fi
