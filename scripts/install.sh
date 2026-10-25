#!/usr/bin/env bash
# Installs the givens command for everyone on this Mac to use: builds it into a folder of its
# own and links it into a folder on PATH, so `givens .` opens the TUI on any project.
#
#   scripts/install.sh                                # command in ~/.local/bin
#   GIVENS_BIN_DIR=/opt/homebrew/bin scripts/install.sh
#   scripts/install.sh --uninstall                    # removes it again
#
# --uninstall also removes a Givens.app an earlier install put in Applications: the Mac app is
# archived in _archive/macos.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

bin="${GIVENS_BIN_DIR:-$HOME/.local/bin}"
home="${GIVENS_HOME:-$HOME/.local/share/givens}"
command="$bin/givens"

if [ "${1:-}" = "--uninstall" ]; then
  # Only the link Givens made is removed, never another givens on PATH.
  if [ -L "$command" ]; then rm "$command"; fi
  rm -f "$home/givens"
  for app in /Applications/Givens.app "$HOME/Applications/Givens.app"; do
    if [ -d "$app" ]; then rm -rf "$app"; fi
  done
  echo "Removed $command. Your projects and capabilities folders are untouched."
  exit 0
fi

if [ -e "$command" ] && [ ! -L "$command" ]; then
  echo "$command exists and isn't a link Givens made; set GIVENS_BIN_DIR to another folder." >&2
  exit 1
fi

mkdir -p "$home" "$bin"
# Ref: https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies
go build -o "$home/givens" ./cmd/givens
ln -sf "$home/givens" "$command"

echo "Installed $home/givens"
echo "Linked $command"
case ":$PATH:" in
  *":$bin:"*) echo "Open a new terminal window, then try: givens ." ;;
  *)
    echo "$bin isn't on your PATH. Add this line to ~/.zshrc, then open a new terminal:"
    # $PATH is printed as written, for the shell to expand later.
    printf '  export PATH="%s:%sPATH"\n' "$bin" '$'
    ;;
esac
