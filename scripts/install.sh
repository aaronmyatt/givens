#!/usr/bin/env bash
# Installs Givens for everyone on this Mac to use: builds build/Givens.app, puts it in
# /Applications (or ~/Applications when /Applications can't be written), links the givens
# command into a folder on PATH, and registers the app so `open -a Givens`, Spotlight and
# `givens <folder>` find it.
#
#   scripts/install.sh                                # command in ~/.local/bin
#   GIVENS_BIN_DIR=/opt/homebrew/bin scripts/install.sh
#   scripts/install.sh --uninstall                    # removes both again
#
# The app is ad-hoc signed by build-app.sh; it isn't quarantined, since it was built here.
# Signing and notarizing for other Macs is scripts/sign-app.sh's.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

apps=/Applications
if [ ! -w "$apps" ]; then
  apps="$HOME/Applications"
fi
bin="${GIVENS_BIN_DIR:-$HOME/.local/bin}"
app="$apps/Givens.app"
command="$bin/givens"

# Launch Services' register tool; -f registers an app bundle even where it was registered.
# Ref: https://developer.apple.com/documentation/coreservices/launch_services
lsregister=/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister

# Quits a running Givens, so its files can be replaced. Ref: man osascript
quit_givens() {
  osascript -e 'if application "Givens" is running then tell application "Givens" to quit' \
    >/dev/null 2>&1 || true
}

if [ "${1:-}" = "--uninstall" ]; then
  quit_givens
  # Only the link Givens made is removed, never another givens on PATH.
  if [ -L "$command" ]; then rm "$command"; fi
  if [ -d "$app" ]; then
    "$lsregister" -u "$app" >/dev/null 2>&1 || true
    rm -rf "$app"
  fi
  echo "Removed $app and $command. Your projects and capabilities folders are untouched."
  exit 0
fi

if [ -e "$command" ] && [ ! -L "$command" ]; then
  echo "$command exists and isn't a link Givens made; set GIVENS_BIN_DIR to another folder." >&2
  exit 1
fi

scripts/build-app.sh
quit_givens
mkdir -p "$apps" "$bin"
# ditto copies the bundle with its signature and extended attributes intact.
# Ref: man ditto
rm -rf "$app"
ditto build/Givens.app "$app"
ln -sf "$app/Contents/Helpers/givens" "$command"
"$lsregister" -f "$app"

echo "Installed $app"
echo "Linked $command"
case ":$PATH:" in
  *":$bin:"*) echo "Open a new terminal window, then try: givens ." ;;
  *)
    echo "$bin isn't on your PATH. Add this line to ~/.zshrc, then open a new terminal:"
    # $PATH is printed as written, for the shell to expand later.
    printf '  export PATH="%s:%sPATH"\n' "$bin" '$'
    ;;
esac
