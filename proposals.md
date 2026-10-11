# Givens: proposed stories and scenarios

Oct 10, 2026 · for review alongside "Givens: stories and scenarios"

## How to read this

- Each proposal from the review becomes a story with scenarios, in the same Given / When / Then format as the spec.
- Each section says which capability it adds to, or whether it's a new one.
- **Changes to existing scenarios** lists what in the current spec would need editing.
- **Decided** blocks record the calls made in review.
- Slugs I made up for the sample shop: `checkout/3ds-challenge`, `checkout/3ds-failed`, `sign-in/magic-link`, `invites/accept-invite`.

## Decisions

1. Trust attaches to the folder path.
2. Renaming a scenario or capability voids its past runs. There are no stable ids.
3. Setup should be idempotent by default. An optional teardown script covers the cases where it can't be.
4. ⇧⌘C copies a setup command from search.
5. The kit isn't part of the setup fingerprint.

| # | Proposal | Adds to | Happy | Error |
|---|---|---|---|---|
| 1 | Trust a project | New: Trust a project | 2 | 2 |
| 2 | `from` errors | Drop into a scenario | 0 | 3 |
| 3 | Rename or delete a capability others build on | Write stories and scenarios | 1 | 1 |
| 4 | Renames start runs over | Write stories and scenarios | 3 | 0 |
| 5 | Wider setup fingerprint | Share setup between scenarios | 4 | 0 |
| 6 | Manual Givens in the checklist | Drop into a scenario | 1 | 0 |
| 7 | Runs from a team | Drop into a scenario | 2 | 0 |
| 8 | Setup that runs twice | Drop into a scenario | 3 | 1 |
| 9 | One path style | Whole spec | – | – |
| 10 | Copy from a nested folder | Copy a setup script | 2 | 0 |
| 11 | Copy shortcut in search | Copy a setup script | 1 | 0 |
| 12 | Starter capability placement | Open a project | – | – |
| 13 | The kit | New: The kit | 5 | 3 |
| 14 | `givens` commands | New: Use Givens from the terminal | 3 | 2 |
| 15 | Kit upgrades | New: Update the kit | 2 | 2 |
| 16 | Front matter fields | New: Front matter | 2 | 1 |
| 17 | Timeout per scenario | Drop into a scenario | 1 | 1 |
| | **Total** | | **32** | **16** |

---

## 1. Trust a project

*New capability.*

As a developer opening a project someone else wrote
I want Givens to ask before it runs any of the project's scripts
So that opening a cloned repo can't run code I haven't looked at

- Trust is stored in Givens's own settings, like the recent projects list, and never in the project folder.
- Reading, searching and copying still work without trust. Only running scripts needs it.

**Asked before the first run** — Happy path · trust-prompt

- Given a developer has never run anything from the sample shop
- When they drop in to "Card declined"
- Then they see "Run scripts from this project?" with the project's path
  - And they're offered "Trust" and "Don't trust"
  - And nothing runs until they choose

**Trust is remembered** — Happy path · trust-remembered

- Given a developer chose "Trust" for the sample shop before
- When they drop in to "Card declined"
- Then setup starts without asking

**Choose not to trust** — Error state · trust-declined

- Given a developer chose "Don't trust" for the sample shop
- When they select "Card declined"
- Then "Drop in" is disabled, with "Scripts are off for this project"
  - And "Copy" still works
  - And they're offered "Trust this project"

**Moved or re-cloned project** — Error state · trust-new-path

- Given a developer trusted the sample shop at ~/code/shop
- When they open a fresh clone of it at ~/code/shop-2 and drop in
- Then they're asked "Run scripts from this project?" again

> **Decided:** trust attaches to the folder path. A re-clone or a moved folder asks again.

---

## 2. `from` errors

*Adds to Drop into a scenario.*

- These are caught before anything runs, the same way missing commands are.
- The kit also guards against loops, so a script run in a terminal can't loop forever either.

**Builds on a scenario that doesn't exist** — Error state · from-missing

- Given a developer has the sample shop project open in Givens
  - And the "3-D Secure challenge" setup starts with "from checkout/pay-with-card"
- When they drop in to "3-D Secure challenge"
- Then nothing in the script runs
  - And they see "from checkout/pay-with-card: there's no scenario with that name"
  - And they're offered "Edit script"

**Setups that build on each other in a loop** — Error state · from-loop

- Given a developer has the sample shop project open in Givens
  - And "Pay with saved card" starts with "from checkout/3ds-challenge"
  - And "3-D Secure challenge" starts with "from checkout/pay-saved-card"
- When they drop in to "3-D Secure challenge"
- Then nothing in the script runs
  - And they see "These setups build on each other in a loop: 3ds-challenge → pay-saved-card → 3ds-challenge"

**A loop run from the terminal** — Error state · from-loop-terminal

- Given the same loop as from-loop
- When a developer runs "bash capabilities/checkout/scripts/3ds-challenge.sh" from the project's root folder
- Then the kit prints "✗ from loop: 3ds-challenge → pay-saved-card → 3ds-challenge"
  - And the script stops with a non-zero exit code

---

## 3. Rename or delete a capability others build on

*Adds to Write stories and scenarios.*

**Renaming a capability updates `from` lines** — Happy path · rename-capability-updates-from

- Given a developer has the sample shop project open in Givens
  - And the "Accept invite" setup starts with "from sign-in/magic-link"
- When they rename "Sign in" to "Log in"
- Then that line becomes "from log-in/magic-link"
  - And "Magic link" and "Expired link" show "Never run", since a rename starts runs over (see 4)

**Delete a capability another builds on** — Error state · delete-capability-depended-on

- Given a developer has the sample shop project open in Givens
  - And the "Accept invite" setup starts with "from sign-in/magic-link"
- When they delete "Sign in"
- Then they see "Accept invite builds on a scenario in Sign in"
  - And nothing is deleted unless they choose "Delete anyway"

---

## 4. Renames start runs over

*Adds to Write stories and scenarios.*

> **Decided:** renaming a scenario or capability voids its past runs. There are no stable ids.

- Runs are recorded against the scenario's path, like `checkout/card-declined`.
- A rename gives a new path, so the scenario starts at "Never run".
- Old lines stay in `runs/` as history. Givens never rewrites them.

**Warned before a rename** — Happy path · rename-warns

- Given "Card declined" has 3 runs recorded
- When a developer renames it to "Card refused"
- Then they see "Renaming starts Card declined's runs over"
  - And they're offered "Rename" and "Cancel"

**A renamed scenario starts at Never run** — Happy path · rename-voids-runs

- Given "Card declined" has a passing run recorded today
- When a developer renames it to "Card refused" and confirms
- Then "Card refused" shows "Never run"
  - And it's listed in the Unverified view
  - And nothing in runs/ changes

**Scenarios built on it are unverified too** — Happy path · rename-unverifies-dependents

- Given "3-D Secure challenge" has a passing run recorded
- When a developer renames "Pay with saved card" to "Pay with a saved card" and confirms
- Then the from line in 3-D Secure challenge's setup is updated
  - And "3-D Secure challenge" shows "Unverified: setup changed since the last run"

**Changes to existing scenarios**

- rename-scenario: "its earlier runs still count towards it" becomes "it shows 'Never run'".
- rename-capability: add "And its scenarios show 'Never run'".

---

## 5. Wider setup fingerprint

*Adds to Share setup between scenarios.*

- A fingerprint is a hash, a short code worked out from file contents, that changes when they do.
- It now covers:
  - the scenario's own steps
  - every setup it builds on through `from`
  - every helper it calls, and every helper those helpers call

**Changing a step unverifies the scenario** — Happy path · steps-change-unverifies

- Given "Card declined" has a passing run recorded
- When "the order stays pending" is changed to "the order is marked failed"
- Then "Card declined" shows "Unverified: steps changed since the last run"

**Changing a setup unverifies the scenarios built on it** — Happy path · parent-change-unverifies

- Given "3-D Secure challenge" and "3-D Secure failed" both have passing runs recorded
- When the "Pay with saved card" setup script changes
- Then both show "Unverified: checkout/pay-saved-card setup changed since the last run"

**Changing a nested helper unverifies its callers** — Happy path · nested-helper-unverifies

- Given seed_shopper calls seed_address
  - And "Card declined" has a passing run recorded
- When seed_address changes on disk
- Then "Card declined" shows "Unverified: seed_address changed since the last run"

> **Decided:** the kit (givens.sh) isn't in the fingerprint.

**Updating the kit keeps scenarios verified** — Happy path · kit-change-keeps-verified

- Given "Card declined" has a passing run recorded
- When the sample shop's kit is updated from version 1 to 2
- Then "Card declined" still shows as verified

---

## 6. Manual Givens in the checklist

*Adds to Drop into a scenario.*

- `first_you` lines are hidden by `GIVENS_QUIET=1`, the same as the other "your turn" lines.
- Givens shows them at the top of the checklist instead, so steps done by hand aren't lost.

**Steps done by hand come first** — Happy path · manual-given-checklist

- Given the "Card declined" setup ends with first_you 'select "Checkout"'
- When a developer drops in to "Card declined"
- Then the checklist starts with "First, select "Checkout""
  - And the When steps come after it
  - And the Then steps are still the only ones marked "Pass" or "Fail"

**Changes to existing scenarios**

- drop-in: "the When steps are shown as a checklist" becomes "any manual Givens, then the When steps, are shown as a checklist".

---

## 7. Runs from a team

*Adds to Drop into a scenario.*

- Each run line records who ran it, using git's `user.name`.
- New capabilities folders get a `.gitattributes` line, `runs/** merge=union`. It tells git to keep both sides' lines in a merge.

**A run records who ran it** — Happy path · run-author

- Given a developer whose git user.name is "Sam" has dropped in to "Card declined" and marked every Then step "Pass"
- When they choose "Record run"
- Then the run line includes "Sam"
  - And "Card declined" shows "Verified today by Sam"

**Runs on two branches merge cleanly** — Happy path · runs-merge

- Given Sam and Ali each recorded a run for "Card declined" today, on different branches
- When the branches are merged
- Then git merges runs/ with no conflict
  - And both run lines are kept

**Changes to existing scenarios**

- create-capabilities-folder: add "And it contains .gitattributes marking runs/ as merge=union".

---

## 8. Setup that runs twice

*Adds to Drop into a scenario.*

> **Decided:** both. Idempotent setup is the default. A teardown script is there for cases where it can't be.

- Idempotent means running setup twice leaves the same state as running it once.
- GIVENS.md states this as the rule, and the starter capability shows it.
- A teardown is optional: `scripts/<slug>.teardown.sh`, which sources the kit like any setup script.
- When a scenario has one, Givens runs it before every setup run. So teardown must also cope when there's nothing to clean up.

**Running setup twice gives the same state** — Happy path · rerun-setup

- Given "Card declined" has no teardown script
  - And a developer has dropped in to "Card declined" once
- When they choose "Reset state"
- Then setup finishes without errors
  - And the shop has one shopper "sam@example.com", not two

**Teardown runs before setup** — Happy path · teardown-before-setup

- Given "Card declined" has a script called card-declined.teardown.sh
- When a developer drops in, or chooses "Reset state" or "Try again"
- Then the output shows "→ teardown" followed by the teardown's output
  - And then setup runs from the start

**Add a teardown** — Happy path · add-teardown

- Given "Card declined" has no teardown script
- When a developer chooses "Add teardown"
- Then checkout/scripts/card-declined.teardown.sh is created, starting with the line that sources givens.sh
  - And it opens in the editor

**The teardown fails** — Error state · teardown-fails

- Given the "Card declined" teardown stops with an error
- When a developer drops in to "Card declined"
- Then they see "Teardown failed (exit 2)"
  - And setup doesn't run
  - And they're offered "Edit teardown" and "Run setup anyway"

> **Open question:** when "3-D Secure challenge" runs `from checkout/pay-saved-card`, should Pay with saved card's teardown run too? I'd say yes, just before its setup, so each `from` level cleans up its own state.

---

## 9. One path style

*No scenarios. An edit across the spec.*

- Today, some paths start with `capabilities/…` and some with `checkout/…`.
- Rule: in Then steps, write paths from inside the capabilities folder, like `checkout/scenarios/expired-card.md`.
- Exception: anything put on the clipboard is written exactly as it will be pasted.
- Scenarios to edit: new-story (`capabilities/refunds/story.md` → `refunds/story.md`).

---

## 10. Copy from a nested folder

*Adds to Copy a setup script.*

- Copy writes the path from the git repo's root, not from wherever the capabilities folder sits.
- The kit moves into the repo root before running anything, so it doesn't matter which folder the command is run from.

**Capabilities folder isn't at the root** — Happy path · copy-nested

- Given the sample shop's capabilities folder is at docs/capabilities in its git repo
- When a developer chooses "Copy" on "Card declined"
- Then the clipboard holds "bash docs/capabilities/checkout/scripts/card-declined.sh"
  - And running it from the repo's root folder sets up the scenario

**Run from a subfolder** — Happy path · run-from-subfolder

- Given a developer's terminal is in the sample shop's src folder
- When they run "bash ../capabilities/checkout/scripts/card-declined.sh"
- Then setup runs the same as it would from the root folder

---

## 11. Copy shortcut in search

*Adds to Copy a setup script.*

> **Decided:** ⇧⌘C copies the setup command. ⌘C always copies selected text.

**Copy text in the search box** — Happy path · copy-search-text

- Given a developer has typed "card declined" into search and selected it
- When they press ⌘C
- Then the clipboard holds "card declined"

**Copy a setup command from search** — Happy path · copy-from-search (edited)

- Given a developer has the sample shop project open in Givens
  - And "Sign in" is selected
- When they press ⌘K, type "card declined" and press ⇧⌘C
- Then the clipboard holds "bash capabilities/checkout/scripts/card-declined.sh"
  - And "Sign in" is still selected

**Changes to existing scenarios**

- copy-from-search: ⌘C → ⇧⌘C.
- Search everything intro: "⌘C copies its setup command" → "⇧⌘C".

---

## 12. Starter capability placement

*No scenarios. An edit to the layout.*

- Move "Starter capability: Add a story" out of "Open a project" into its own section after it.
- step-without-keyword repeats not-a-step. That's fine, since the starter is a worked example. Add a line saying it mirrors the spec on purpose.

---

## 13. The kit

*New capability. Each scenario maps to one bats test (bats is a test runner for bash). None need the app.*

As a developer writing or running setup scripts
I want the kit to behave the same in Givens and in a terminal
So that a script I try by hand acts the same when Givens runs it

**Strict mode stops at the first failure** — Error state · kit-strict-mode

- Given a setup script's second command fails
- When a developer runs the script in a terminal
- Then setup stops at that command
  - And the lines after it don't run
  - And the script exits with a non-zero code

**"Your turn" heading prints once** — Happy path · kit-your-turn

- Given a setup script calls first_you, then when_you twice, then then_check
- When a developer runs it in a terminal
- Then "Your turn" is printed once, just before the first_you line

**Quiet mode** — Happy path · kit-quiet

- Given the same script as kit-your-turn
- When it runs with GIVENS_QUIET=1
- Then no "Your turn" heading or first_you, when_you or then_check lines are printed
  - And ok, warn and fail lines are still printed

**No colour when not in a terminal** — Happy path · kit-no-colour

- Given a setup script calls ok "seeded shopper"
- When its output is sent to a file
- Then the file holds "✓ seeded shopper" with no colour codes

**A missing command** — Error state · kit-need

- Given a setup script has need shopctl "brew install shop/tap/shopctl"
  - And shopctl isn't installed
- When a developer runs the script in a terminal
- Then it prints "✗ needs shopctl: brew install shop/tap/shopctl"
  - And nothing after the need line runs

**fail stops setup** — Error state · kit-fail

- Given a setup script calls fail "No database found"
- When it runs
- Then it prints "✗ No database found"
  - And the script exits with a non-zero code

**warn carries on** — Happy path · kit-warn

- Given a setup script calls warn "Mailpit isn't running", then ok "seeded shopper"
- When it runs
- Then both lines are printed
  - And the script exits with 0

**When mode** — Happy path · kit-when-mode

- Given a setup script ends with when_you "run this: $(when_command)"
  - And it has if when_mode; then …; exit 0; fi near the top
- When a developer runs it, then runs the command it prints
- Then the first run prints "When you run this: bash capabilities/…/<slug>.sh --when"
  - And the second run does only the When part, then exits

> **Open question:** the spec doesn't say whether `--when` reruns setup first. kit-when-mode assumes it doesn't.

---

## 14. `givens` commands

*New capability.*

As a developer in a terminal, or an agent working for one
I want to list, run and check scenarios without opening the app
So that agents and CI (automated checks on each push) can use the same files

- Today `givens` is a small script that runs `open -a Givens`. `list` and `run` can stay plain bash. `check` needs the app's parser, so it would run the app in headless mode (no window).

**List scenarios** — Happy path · cli-list

- Given a developer's terminal is in the sample shop's folder
- When they run "givens list"
- Then each scenario is printed as its path, like "checkout/card-declined", with its kind and status
  - And "givens list --json" prints the same as JSON

**Run a setup by name** — Happy path · cli-run

- Given a developer's terminal is anywhere inside the sample shop's folder
- When they run "givens run checkout/card-declined"
- Then the setup runs, with its "your turn" lines

**Check passes** — Happy path · cli-check-pass

- Given every file in the sample shop is valid
- When a developer runs "givens check"
- Then it prints "9 scenarios, 0 problems"
  - And it exits with 0

**Check finds a problem** — Error state · cli-check-fail

- Given the "Card declined" front matter never closes
- When a developer runs "givens check"
- Then it prints "checkout/scenarios/card-declined.md: Front matter starts on line 1 but never closes"
  - And it exits with 1

**Run a scenario that doesn't exist** — Error state · cli-run-unknown

- Given a developer's terminal is in the sample shop's folder
- When they run "givens run checkout/nope"
- Then it prints "There's no scenario checkout/nope"
  - And it exits with 1

---

## 15. Kit upgrades

*New capability.*

As a developer on a project started with an older Givens
I want to update the kit from inside Givens
So that I get fixes without copying files by hand

- givens.sh starts with a version line, for example `# givens kit 2`.

**An update is available** — Happy path · kit-outdated

- Given the sample shop's givens.sh is version 1
  - And Givens ships version 2
- When a developer opens the sample shop
- Then they see "Kit update available (1 → 2)"
  - And they're offered "Update kit"

**Update the kit** — Happy path · update-kit

- Given the sample shop's givens.sh is version 1 and hasn't been edited
- When a developer chooses "Update kit"
- Then givens.sh is replaced with version 2
  - And git diff shows only givens.sh changed

**The kit was edited by hand** — Error state · kit-edited

- Given the sample shop's givens.sh has local edits
- When a developer chooses "Update kit"
- Then they see "givens.sh has changes of its own"
  - And they're offered "Compare" and "Replace anyway"

**The project's kit is newer than Givens** — Error state · kit-newer

- Given the sample shop's givens.sh is version 3
  - And Givens ships version 2
- When a developer opens the sample shop
- Then they see "This project uses a newer kit. Update Givens to edit its scripts."
  - And reading, searching and copying still work

---

## 16. Front matter fields

*New capability.*

As a developer, or an agent, writing scenarios by hand
I want the front matter fields written down
So that files from any tool load the same way

| Field | Values | Meaning |
|---|---|---|
| `kind` | `happy` or `error` | Happy path or error state |
| `tags` | list of words | Groups scenarios into tag views |
| `timeout` | like `5m` | When to show "Still running" (see 17) |

**Unknown fields are kept** — Happy path · unknown-field

- Given the "Card declined" front matter has "owner: payments"
- When a developer edits its steps in Givens and saves
- Then "owner: payments" is still in the file
  - And git diff shows only the step that changed

**Tag views** — Happy path · tags-view

- Given "Card declined" and "Pay with saved card" both have "tags: [payments]"
- When a developer selects "payments" under Tags in the sidebar
- Then only those two scenarios are listed

**A kind that isn't happy or error** — Error state · bad-kind

- Given the "Card declined" front matter has "kind: eror"
- When a developer opens the sample shop
- Then "Card declined" is listed with a warning
  - And selecting it shows "kind must be happy or error"

---

## 17. Timeout per scenario

*Adds to Drop into a scenario.*

**A longer timeout** — Happy path · custom-timeout

- Given the "Card declined" front matter has "timeout: 5m"
  - And its setup takes 3 minutes
- When a developer drops in to "Card declined"
- Then no "Still running" message is shown
  - And setup finishes as normal

**A timeout Givens can't read** — Error state · bad-timeout

- Given the "Card declined" front matter has "timeout: soon"
- When a developer drops in to "Card declined"
- Then setup runs with the usual 2-minute timeout
  - And "Card declined" is marked "timeout must look like 90s or 5m"

**Changes to existing scenarios**

- slow-setup: add "And 'Card declined' has no timeout in its front matter".
