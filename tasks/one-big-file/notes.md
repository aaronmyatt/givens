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

## Views moves in (2026-10-25)
- views.go is gone. Its Views code is the section: the view names and sidebar kinds,
  `selectView`, `viewItems`, then `ListedScenarios`, `ScenariosIn`, `scenariosIn`, `inView` and
  `unverifiedRows` under "See every error state" and "See what needs checking again",
  `ViewHelper` and `ViewHelpers` under "Browse helpers", and `emptyView` under "A view with
  nothing in it".
- `tagViews` and `isViewTag`, which make each tag a view, went to big_helpers.go: Views'
  `selectView` and `viewItems` call them, and they're the Tags story's "each tag is a view".
- Seam cut: `inView`'s step branch was Search everything's ("Find every use of a step"). It's
  now `usesStep` in search_find.go, which `inView` calls; it moves with Search in slice 4.
- The tag branches inside `inView`, `emptyView` and `viewItems` stayed in the section, no seam:
  the Views Feature's description owns "one view per tag" and the Tags tab.
- Section 194 lines, big_helpers.go +30. big.go 425, big_helpers.go 134. 3 of 17.
- Surprise: the plan's "tagViews is tags' too" holds for the story, not the code. Nothing in
  todo.go or search.go calls `tagViews`; only Views' code does. They went to big_helpers.go
  anyway, as the plan says, since a Tags scenario ("todo" is listed on the Tags tab) is what
  they do.

## Tags moves in (2026-10-25)
- todo.go is gone: its offers, `todoOffers`, `flagTodo` and `withTodo` are the section, under
  "Flag a scenario to do" and "Remove a to do flag". todo_test.go stays, still testing
  `withTodo`.
- Seam cut: the tag-expression parse inside Search's `readQuery` (search.go) is now
  `parseTags` in the section, under the four search scenarios; `readQuery` calls it and keeps
  the split into words and tags. Its "a parsed query has no problem" assertion moved with it.
- Not cut: `findScenarios`' one-line `read.expression.Match` filter and `searchGroups`'
  tagged branch stay Search's; the opening comment points at them.
- "A new scenario starts flagged to do" (Write stories) shares no code with this section:
  authoring.go writes `project.TagTodo` itself. Nothing went to big_helpers.go.
- "An error state", "A tag view" and "A tag on the Feature" have no heading here: their code
  is the Views section and `tagViews`. "A runner filters on the same tags" is cucumber-js.
  "A scenario flagged to do" is project's `Todo` and the TUI's ⚑.
- Section 126 lines, big_helpers.go +0. big.go 554, big_helpers.go 134. 4 of 17.

## Pointer sections: The kit, Use Givens from the terminal, Work in the terminal (2026-10-25)
- Three Feature headings with no code, in folder order: The kit before Trust, Use Givens from
  the terminal before Views, Work in the terminal last. Each opening comment names where the
  story lives: internal/kit/givens.sh and kit.bats; cmd/givens list.go, run.go and main.go;
  internal/tui (by file), internal/termedit and cmd/givens' terminal commands.
- No Scenario headings: none of their code is in big.go. `scripts/entry.sh "Quiet mode"` lands
  on the Feature's lines, and `--debug` says "no functions … to stop in", as the plan expected.
- The header's "Code outside this file" list gains the edges.
- Plan was wrong here: install.go and `InstallCommand` serve no story. Their only callers are
  the archived Mac app's serve (`_archive/internal/serve/methods.go`); no feature has an
  install scenario. They stay where they are, not in Use Givens from the terminal. Dead code
  in the live tree: a backlog line to delete them (with `writableFolders`' test in
  app_test.go) is the developer's call.
- `givensCommand` stays in windows.go: Drop in's `startSession` (dropin_run.go) is its one live
  caller, besides the dead `InstallCommand`. It moves with Drop in (slice 12).
- No app code serves only one of these three stories: the edges call other stories' functions.
- Sections 9, 8 and 9 lines. big.go 583, big_helpers.go 134. 7 of 17.

## Search everything moves in (2026-10-25)
- search.go, search_find.go and search_index.go are gone. Their Search code is the section,
  between Copy and Tags: the result kinds and types, then `readQuery`, `searchGroups`,
  `findScenarios`, `matchesIn`, `limit` and the index (`searchIndexOf`, `indexScript`, its types
  and `searchMemo`) under "Search for a phrase" and "Nothing matches"; `findStories`, `findSteps`
  with `usesStep`, `FilterSearch` with `findScripts`, and `findHelpers` under their scenarios.
- To big_helpers.go: `SearchResults`, since Copy's `copyFromSearch` calls it and Tags' four
  search scenarios run through it; `uniqueSteps`, since the editor's step suggestions
  (`suggestionsFor`, editor_more.go, Write stories) call it too.
- No new seams. `usesStep` (cut in the Views slice) and the tag filter in `findScenarios` and
  `searchGroups`' tagged branch stay in the section; `parseTags` stays Tags'. `usesStep` is
  called from Views' `inView`, but as a seam it lives with its story, as `parseTags` and
  `trustOf` do.
- Stays out, as core glue: `Press` and `Type` (input.go) open the panel and fill its box,
  `searchSnapshot` and `searchMessages` (snapshot.go) show it and say "Nothing matches", and
  `selectStep` (selection.go) selects a step result. The opening comment names them.
- The Tags opening comment pointed at search.go and search_find.go; it now points at this
  section.
- Section 377 lines, big_helpers.go +48. big.go 961, big_helpers.go 182. 8 of 17.
- Surprise: the rule for shared code gave two answers here. By "another story's code calls it",
  `usesStep` would go to big_helpers.go; by "a seam lives with its guest", it stays. Seams win,
  so the helpers rule counts callers of non-seam code only.
- Ran in a worktree (obf/search-everything) beside Update the kit's, for the slice-4 clash count.
  The tags suite's "A runner filters on the same tags" fails outside `scripts/check.sh`, which
  installs cucumber-js in the sample shop first; it fails the same on main.
