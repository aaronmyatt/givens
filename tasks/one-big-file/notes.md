# One big file: pilot

## Goal
Try the workflow in `essay.md` on one capability, copy-a-setup-script, and decide whether to
grow it. `internal/app/big.go` tells the stories in order, as literate code: a section per
Feature, with `// Feature:`, `// Rule:` and `// Scenario:` comments as its headings.
`TestBigFile` in `acceptance/` keeps the headings in step with the feature files;
`scripts/entry.sh` finds a section by name and sets up delve.

## Design
- Slice: copy-a-setup-script. 5 scenarios; one piece of code, `copySetup`, serves three.
  Rejected: trust-a-project and views, 4 scenarios each, with one When phrasing.
- Big file in `internal/app`, not a new package: the `Window` state lives in `app`, and Go
  resolves names across a package's files, so moving code between files changes no behaviour.
- The feature files stay as they are; the big file points at them, not the other way round.
  First built with an `@entry:<name>` tag on each scenario and an `Entry: <name>.` marker on
  each function; dropped the same day: the tags were fluff in the scenarios, and they made a
  workflow convention a rule in GIVENS.md that every Givens project reads.
- Headings are the Gherkin names word for word, so a search for a scenario's name lands on
  its code. They don't have to be exact about where the code starts: a section is enough for a
  reader, and an agent fills the gap.
- What's enforced: each section names a real Feature; sections follow the capabilities
  folders' order; a section has every Rule of its Feature, in order; a Scenario heading names
  one of the Feature's scenarios. Scenario headings are optional. Features not yet moved in are
  counted, not failed.
- The check is a Go test, not a bash script: it uses the real gherkin parser, and it runs in
  `go test ./...`, on every `.feature` edit and on every edit to big.go.
- godog and the step definitions stay (CLAUDE.md). The essay drops step definitions; this
  pilot doesn't test that part.
- Line savings from bundling alone are small: about 480 of `internal/app`'s 7,716 lines are
  per-file package and import headers. Not a reason on its own.

## As built (2026-10-25)
- `internal/app/big.go`: header (the layout, what's checked, code that lives elsewhere and
  why), then "Feature: Copy a setup script" with Scenario headings over `copySetup` (Copy;
  Capabilities folder isn't at the root; Copy steps done by hand) and `copyFromSearch` (Copy
  from search), then "Helpers" with `repoRoot`. The rest of copy.go, which is
  read-a-capability's, is now copy_steps.go.
- "Run from a subfolder" has no heading: its code is the kit's `cd` to the repo root. The
  Feature's opening comment says so, and `scripts/entry.sh "Run from a subfolder"` lands on
  the Feature's section.
- `TestBigFile` in acceptance/bigfile_test.go. Checked by breaking it: a misspelt Scenario
  heading, a misspelt Feature heading and a made-up Rule each fail, naming the line.
- `scripts/entry.sh "<name>"`: word for word, then part of a heading, ignoring case; a
  scenario with no heading finds its Rule's section, then its Feature's. Prints the lines and
  the functions in them. `--debug` prints the dlv command and a breakpoint on each function in
  the section; `--debug --run` starts it. Checked: delve stops at big.go in `copySetup`.

## Findings
- Hops, scenario to code. Before: step regex (copy_test.go or world/common.go), then
  `Window.Choose` (offers.go), then `chooseOnScenarioPage`, then copy.go, which you had to know
  about. After: `scripts/entry.sh "Copy"`, or a plain search for "Copy" in big.go.
- Lines saved: none to speak of. big.go's imports are 7 lines; copy_steps.go lost 5.
- tigerlint didn't fight it. Moved code moved unchanged; the 70-line function limit keeps
  sections in small functions.
- Where the essay doesn't fit Givens:
  - Most Whens are one generic step, "they choose X", that goes through `Window.Choose`, so a
    scenario's code is what a label dispatches to, not a function per step phrasing. Sections
    by story suit that better than one entry point per When.
  - godog stays; the essay's "no step definitions" part isn't tested here.
  - `app` imports `browser`, which imports chromedp: the big file isn't framework-free.
  - `Window` holds caches and UI state, so sections share state through it.
  - Some of a story's code is bash (the kit). A comment points at it; `--debug` only helps
    with Go.
- Next, if it grows: trust-a-project (4 scenarios), then views (4).

## Trust a project moves in (2026-10-25)
- trust.go is gone: its folder lists' trust half is the section, and `remember`, `forget`,
  `writeFolders` and `folders` are Helpers, since recents and open windows use them too.
- The trust gate was a `switch` inside Drop in's `startSession` (dropin_run.go). It's now two
  functions in the section, `trustOf` and `runTrusted`, which `startSession` calls; the trust
  constants and `scriptsOff` moved with it. "Trust this project" on the Drop in page calls
  `trustThisProject`. No behaviour changed; the trust, Drop in and copy suites pass.
- Finding: a story's code can live inside another story's function, here Drop in's. Moving it
  meant cutting a seam (`trustOf`, `runTrusted`) first, so a migration can touch the code of a
  story that isn't in the big file yet.
- The header said helpers are for "every story", but repoRoot is copy's alone: it now says
  helpers sit below the sections so a section reads as what its story does.

## Helpers get their own file (2026-10-25)
- At the developer's ask, the Helpers part of big.go is now `internal/app/big_helpers.go`, so
  big.go holds only stories. Its header says what goes there and names the tripwire: the
  helpers file growing past big.go.
- `TestBigFile` reads the whole of big.go (no more `// Helpers` stop) and fails if
  big_helpers.go holds a Feature, Rule or Scenario heading; checked by adding one.
  `scripts/entry.sh` no longer stops at a Helpers heading.
- big.go 229 lines, big_helpers.go 104.
