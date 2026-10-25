# Helpers for Work in the terminal's setup scripts. The kit sources this file for every script
# in this capability; see capabilities/the-kit/about.md.

# sample_shop_tab [command]: copies the sample shop to a scratch folder and asks the tester to
# open a terminal tab there, then run command if one is given. A copy, so a scenario that edits,
# renames or deletes leaves testdata/sample-shop as it is.
sample_shop_tab() {
  local _ws_from="$GIVENS_ROOT/../testdata/sample-shop"
  local _ws_tmp="${TMPDIR:-/tmp}"
  local _ws_to _ws_run
  # macOS's TMPDIR ends in a slash; drop it so the printed path has no "//".
  # Ref: https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html
  _ws_to="${_ws_tmp%/}/givens-sample-shop"
  if [ ! -d "$_ws_from/capabilities" ]; then fail "no sample shop at $_ws_from"; fi
  # Empty the folder rather than remove it, so a tab already in it from an earlier run is still
  # in a folder that exists. -mindepth 1 keeps the folder itself.
  # Ref: https://man.freebsd.org/cgi/man.cgi?query=find
  mkdir -p "$_ws_to"
  find "$_ws_to" -mindepth 1 -delete
  # "src/." copies the folder's contents, dotfiles too, not the folder.
  # Ref: https://pubs.opengroup.org/onlinepubs/9799919799/utilities/cp.html
  cp -R "$_ws_from/." "$_ws_to"
  ok "copied the sample shop to $_ws_to"
  # %q quotes the path so it pastes into a shell as one word.
  # Ref: https://www.gnu.org/software/bash/manual/html_node/Bash-Builtins.html#index-printf
  _ws_run="$(printf 'cd %q' "$_ws_to")"
  if [ -n "${1:-}" ]; then _ws_run="$_ws_run && $1"; fi
  by_hand "Terminal" "Open a new terminal tab and run:

  $_ws_run"
}
