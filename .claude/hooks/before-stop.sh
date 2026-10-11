#!/usr/bin/env bash
# Claude Code Stop hook: runs every check before Claude finishes a turn. On failure, exit 2
# sends the output back so Claude fixes it instead of stopping.
# Ref: https://docs.claude.com/en/docs/claude-code/hooks
set -euo pipefail

# stop_hook_active is true when Claude is already continuing because of this hook.
# Letting that second stop through prevents an endless loop; the git hook still guards commits.
active=$(jq -r '.stop_hook_active // false')
[ "$active" = "true" ] && exit 0

root="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}"
if ! out=$("$root/scripts/check.sh" 2>&1); then
  echo "scripts/check.sh failed; fix this before finishing:" >&2
  echo "$out" | tail -40 >&2
  exit 2
fi
exit 0
