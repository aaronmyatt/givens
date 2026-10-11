# The kit

`capabilities/_shared/givens.sh`: one file, written for bash 3.2, copied unchanged into every
project. Every setup script sources it on its first line:

```bash
source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
```

Sourcing it:

1. Turns on strict mode (`set -euo pipefail`), so any failing command stops setup.
2. Sets `GIVENS_ROOT` to the capabilities folder.
3. Loads every `capabilities/_shared/*.sh`, then the calling script's own
   `<capability>/_shared/*.sh`.
4. Warns when two files at the same level define the same helper. A capability's helper
   overriding a shared one is expected, so it stays quiet.
5. Colours its output only when printing to a terminal.

## Functions

| Function | What it does | Example |
| --- | --- | --- |
| `ok <message>` | Prints `✓ <message>` for each piece of state set up | `ok "seeded shopper"` |
| `warn <message>` | Prints `! <message>` and carries on | `warn "Mailpit isn't running"` |
| `fail <message>` | Prints `✗ <message>` and stops setup | `fail "No database found"` |
| `first_you <text>` | Prints "First, …" for a Given done by hand | `first_you 'select "Checkout"'` |
| `when_you <text>` | Prints "When you …" for each When step | `when_you 'click "Place order"'` |
| `then_check <text>` | Prints "Then check that …" for each Then step | `then_check 'you see "Saved"'` |
| `need <command> [hint]` | Stops setup with the install hint if the command is missing | `need bb "brew install borkdude/brew/babashka"` |
| `from <capability>/<slug> [column=value]` | Runs that scenario's setup first, "your turn" lines hidden | `from checkout/pay-with-saved-card` |
| `by_hand [--url <url>] <where> [text]` | Stops setup until a step done elsewhere is marked done | `by_hand "DigitalOcean console" <<'EOF' … EOF` |
| `ask <name> <prompt>` | Stops setup until the user types a value, then sets that variable | `ask INVITE_CODE "Paste the invite code"` |
| `when_mode` | Is true when the script was run with `--when` | `if when_mode; then …; exit 0; fi` |
| `when_command` | Prints the command that runs this script with `--when` | `when_you "run this: $(when_command)"` |

The first of `first_you`, `when_you` or `then_check` to run prints a "Your turn" heading. All
three print nothing when `GIVENS_QUIET=1`.

## Variables

| Variable | Set by | Meaning |
| --- | --- | --- |
| `GIVENS_ROOT` | The kit | The capabilities folder; helpers' child processes see it too |
| `GIVENS_QUIET` | Givens when you drop in, and `from` | `1` hides the "your turn" lines |
| `GIVENS_UI` | Givens when you drop in | `1` shows `by_hand` and `ask` as cards and reads answers from stdin |
| `GIVENS_WHEN` | The kit | `1` when the script was run with `--when`; read it through `when_mode` |
| `GIVENS_SCRIPT` | The kit | The script that sourced the kit; `when_command` uses it |
| `GIVENS_SCENARIO` | Givens, and `from` | The run key, like `checkout/card-declined` |
| `GIVENS_EX_<column>` | Givens, and `from` | The chosen Examples row's value for that column |
| `GIVENS_ARG_<n>` | Givens | Path to a file holding the nth doc string or data table (as CSV) |

Names starting with `_givens` or `_g_` are the kit's own and may change.
