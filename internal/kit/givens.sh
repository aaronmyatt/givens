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

# Colour only when printing to a terminal, so a log file holds plain text.
# Ref: https://www.gnu.org/software/bash/manual/html_node/Bash-Conditional-Expressions.html
if [ -t 1 ]; then
  _g_green=$'\033[32m' _g_yellow=$'\033[33m' _g_red=$'\033[31m' _g_reset=$'\033[0m'
else
  _g_green='' _g_yellow='' _g_red='' _g_reset=''
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
