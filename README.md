# Givens

Givens is a macOS app and a command for walking through a project's Gherkin scenarios by hand.

You keep a `capabilities/` folder of `.feature` files. Each scenario can have a setup script
that puts your app into the scenario's starting state. Givens runs that setup, then shows the
scenario's steps as a checklist. You do the steps, mark them Pass or Fail, and Givens records
the run. A run marks the scenario verified until its steps or setup change.

## Contents

- [Install](#install)
- [Try it on the sample shop](#try-it-on-the-sample-shop)
- [Set up your project](#set-up-your-project)
- [Write a scenario](#write-a-scenario)
- [Write a setup script](#write-a-setup-script)
- [Drop in to a scenario](#drop-in-to-a-scenario)
- [Use the givens command](#use-the-givens-command)
- [Work on Givens itself](#work-on-givens-itself)

## Install

There's no signed release yet, so you build Givens from source.

You need:

- macOS 14 or later.
- Go 1.27 or later.
- Xcode, or the Swift 6 toolchain.

Build and install the app and the `givens` command in one go:

```bash
scripts/install.sh
```

This puts `Givens.app` in `/Applications`, or `~/Applications` if `/Applications` can't be
written. It links `givens` into `~/.local/bin` and registers the app, so `open -a Givens`,
Spotlight and `givens <folder>` all find it. If `~/.local/bin` isn't on your PATH, it prints
the line to add to `~/.zshrc`.

To link `givens` into another folder on your PATH:

```bash
GIVENS_BIN_DIR=/opt/homebrew/bin scripts/install.sh
```

Run it again after pulling changes to update. To remove both:

```bash
scripts/install.sh --uninstall
```

To build without installing, run `scripts/build-app.sh`. It writes `build/Givens.app`, with the
`givens` command inside it in `Contents/Helpers`. From the app, **Givens → Install givens
command** links the command into a folder you pick.

## Try it on the sample shop

`testdata/sample-shop` is a pretend shop with a few capabilities. To build Givens and open it:

```bash
scripts/run-app.sh testdata/sample-shop
```

Then:

1. Choose **Checkout** in the sidebar, then **Card declined**.
2. Choose **Drop in**. The first time, Givens asks "Run scripts from this project?" Say yes.
3. Watch setup run, then work through the checklist.

## Set up your project

Open your project folder in Givens. You can:

- Choose **Open folder…** in the welcome window.
- Drop the folder on the Givens icon in the Dock.
- Run `givens .` in a terminal.

If the project has no `capabilities/` folder, choose **Create capabilities folder**. You get:

```text
capabilities/
  GIVENS.md              # the rules for this folder; read it first
  .gitattributes         # runs/** merge=union, so run logs merge cleanly
  _shared/givens.sh      # the kit, which every setup script sources
  add-a-story/           # a starter capability to copy from
```

A capability is one thing your app does. It's a folder holding exactly one `.feature` file,
named after the folder:

```text
capabilities/
  _shared/*.sh                   # helpers every capability can call
  checkout/
    checkout.feature             # the story and its scenarios
    about.md                     # optional: notes, diagrams, images
    scripts/card-declined.sh     # setup for "Card declined"
    scripts/card-declined.teardown.sh   # optional: runs before each setup
    scripts/pay-in-app.md        # setup done only by hand, shown as cards
    _shared/*.sh                 # helpers only Checkout's scripts can call
  runs/2026-10-11.log            # one line per recorded run
```

`capabilities/GIVENS.md` holds the full rules. Givens, test runners, people and agents all read
the same files.

## Write a scenario

Write plain Gherkin. The Feature's description is the story, as three lines:

```gherkin
Feature: Checkout
  As a returning shopper
  I want to pay with my saved card
  So that checkout takes seconds

  Scenario: Pay with saved card
    Given a signed-in shopper with one item in the cart
    When they place the order
    Then they see "Order placed"

  @error @timeout:5m
  Scenario: Card declined
    Given a signed-in shopper with one item in the cart
    When they place the order
    Then they see "Your card was declined"
```

Tags:

- `@error` marks an error state. A scenario without it is a happy path.
- `@timeout:5m` sets when Givens says "Still running". The default is 2 minutes.
- Any other tag becomes a view in the sidebar.
- A tag on `Feature:` applies to every scenario in the file.

You can write in Givens too. **New story** and **New scenario** make the files for you. The
editor suggests steps you've already used as you type, and **Preview** shows how the scenario
will look.

## Write a setup script

A scenario's setup lives in `scripts/<slug>.sh`. A slug is the scenario's name in lower case,
with hyphens: `Card declined` gives `card-declined`.

The first line sources the kit. The rest puts the app into the starting state, then says what
you'll do by hand:

```bash
source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"
seed_shopper
seed_cart 1
mock_payments --decline

when_you 'place the order'
then_check 'you see "Your card was declined"'
```

Setup should be idempotent: running it twice leaves the same state as running it once.

### The kit

`_shared/givens.sh` gives every script these functions:

| Function | What it does |
| --- | --- |
| `ok <message>` | Says one piece of state is set up. |
| `warn <message>` | Says something is off, but setup carries on. |
| `fail <message>` | Stops setup. |
| `need <command> [hint]` | Stops setup before anything changes when a command is missing, like `need shopctl "brew install shop/tap/shopctl"`. |
| `first_you`, `when_you`, `then_check` | The steps you do by hand, printed after setup. |
| `by_hand [--url <url>] <where> [text]` | A setup step that can't be scripted, like a server console or a phone. Setup waits until you mark it done. |
| `ask <name> <prompt>` | Waits for you to type a value, then sets the variable `<name>`. |
| `from <capability>/<slug>` | Runs another scenario's teardown and setup first, so one setup can build on another. |
| `when_mode` | True when the script runs with `--when`, to do only its When part. |

The kit runs in strict mode, so a failing command stops setup. It works with bash 3.2, the bash
that ships with macOS. Set `GIVENS_QUIET=1` to hide the "your turn" lines.

### Shared helpers

Put shell functions that several scripts call in a `_shared/` folder:

- Root `capabilities/_shared/` helpers work in every capability.
- A capability's own `_shared/` helpers work only in that capability, and override root
  helpers with the same name.

Document a helper with an `@helper` comment so it shows in Givens's **Shared** list:

```bash
# @helper mock_payments [--decline | --challenge 3ds]
# Points the shop at a fake payment provider that approves, declines or challenges every card.
mock_payments() {
  # …
}
```

### Setup done by hand

When nothing in a setup can be scripted, write `scripts/<slug>.md` instead: a numbered list,
with code blocks indented under their steps. Givens shows each step as a card you can copy from.

## Drop in to a scenario

Choose a scenario, then **Drop in**. Givens:

1. Runs the teardown, if there is one, then the setup.
2. Pauses at any `by_hand` or `ask` card until you answer it.
3. Shows the steps as a checklist: **Done** for Given and When steps, **Pass** or **Fail** for
   Then steps.
4. Records the run when you finish, with an optional note.

A run goes into `capabilities/runs/<date>.log`. Commit it with your code. The scenario stays
verified until its steps, its setup or a helper it calls changes.

For a `Scenario Outline`, each row of the Examples table gets its own Drop in.

Press ⌘K to search stories, scenarios, steps, scripts and helpers.

## Use the givens command

```text
givens <folder>                          open a project in Givens
givens list [--json] [--tags <expr>]     list scenarios with their kind and status
givens run <capability>/<scenario>       run a setup in this terminal
givens check [folder]                    check every .feature file parses
```

Examples:

```bash
givens list --tags @error
```

```bash
givens run checkout/card-declined
```

```bash
givens check
```

`givens check` prints `9 scenarios, 0 problems` when all is well. Exit codes are:

- `0`: all is well.
- `1`: problems were found.
- `2`: the command was used wrongly.

Run `givens check` in CI to catch a broken `.feature` file before it merges.

## Work on Givens itself

Givens is specified by its own `capabilities/` folder. Read `capabilities/GIVENS.md` first.

| Path | What's there |
| --- | --- |
| `capabilities/` | Givens's spec, one `.feature` file per capability. |
| `internal/app/` | The core that the app, the command and the tests all drive. |
| `cmd/givens/` | The `givens` command. |
| `macos/` | The SwiftUI app. It talks to `givens serve` over JSON lines. |
| `acceptance/` | One godog suite per capability. Shared steps are in `acceptance/world/`. |
| `macos/Tests/GivensAcceptanceTests/` | The same scenarios, run against the Swift app. |
| `testdata/sample-shop/` | The pretend project most scenarios start from. |
| `tools/tigerlint/` | The linter that enforces `CONSTRAINTS.md`. |

Run every check before you push:

```bash
scripts/check.sh
```

This runs gofmt, vet, the linter, shellcheck, the kit's bats tests, and both acceptance suites.

Scenarios tagged `@wip` are skipped. To run them:

```bash
GIVENS_TAGS=@wip go test ./acceptance/...
```

To sign and notarize a build, set `GIVENS_SIGN_IDENTITY` and `GIVENS_NOTARY_PROFILE`, then run
`scripts/sign-app.sh`.
