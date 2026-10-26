source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"

# A scratch copy of the sample shop, so writing the skill leaves testdata/sample-shop as it is.
# macOS's TMPDIR ends in a slash; drop it so the printed path has no "//".
# Ref: https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html
shop="${TMPDIR:-/tmp}"
shop="${shop%/}/givens-sample-shop-skill"
if [ ! -d "$GIVENS_ROOT/../testdata/sample-shop/capabilities" ]; then fail "no sample shop to copy"; fi
rm -rf "$shop"
mkdir -p "$shop"
# "src/." copies the folder's contents, dotfiles too, not the folder.
# Ref: https://pubs.opengroup.org/onlinepubs/9799919799/utilities/cp.html
cp -R "$GIVENS_ROOT/../testdata/sample-shop/." "$shop"
rm -rf "$shop/.claude/skills/givens"
ok "copied the sample shop to $shop, with no .claude/skills/givens"
printf '# Sample shop\n\nRun npm test before pushing.\n' > "$shop/CLAUDE.md"
ok "gave it a CLAUDE.md of its own"

# %q quotes the path so it pastes into a shell as one word.
# Ref: https://www.gnu.org/software/bash/manual/html_node/Bash-Builtins.html#index-printf
by_hand "Terminal" "Open a new terminal tab and run:

  $(printf 'cd %q' "$shop")"
when_you 'run "givens skill"'
then_check ".claude/skills/givens/SKILL.md is there and says to read capabilities/GIVENS.md first"
then_check 'CLAUDE.md still starts with "Run npm test before pushing.", then has a "## Givens" paragraph'
then_check 'running "givens skill" again leaves SKILL.md and CLAUDE.md as they were'
