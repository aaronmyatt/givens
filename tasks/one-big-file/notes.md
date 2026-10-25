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

## Update the kit moves in (2026-10-25)
- kitupdate.go is gone. All of it is the section, between Trust and Use Givens from the
  terminal: the kit limits, `KitVersionPrefix`, `KitRelease`, `ShipKits` and `shippedKits`
  at the top (every scenario uses them), then `kitState` and `kitMessages` under "An update is
  available" and "The project's kit is newer than Givens", `updateKit` under "Update the kit"
  and "The kit was edited by hand", and `restoreKit` under "A missing kit is put back".
  big.go imports internal/kit now.
- Seam cut: the "Update kit" offer was four lines inline in core glue's `allOffers`
  (offers.go). It's now `kitOffers` in the section, which `allOffers` calls, like
  `todoOffers`. `Choose`'s `case OfferUpdateKit` stays in the dispatcher.
- Not cut: `restoreKit`'s calls in project.go's `reload` and `refresh`, and `kitMessages`'
  call in snapshot.go, are one line each in core glue; the opening comment points at them.
  App's `kits` and Window's `kitProblem` fields stay in app.go and window.go's types.
- `ShipKits`'s callers: the update-the-kit steps and the archived serve. app.go only names
  it in the `kits` field's comment. Nothing else in the live tree calls it.
- `KitVersionPrefix` is also read by Open a project's steps (folders_test.go checks a new
  folder's kit starts with it). That's step code, not app code, so it stayed in the section,
  not big_helpers.go.
- Section 169 lines, big_helpers.go +0. big.go 753, big_helpers.go 134. 8 of 17.

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

## The clash experiment (2026-10-25)
- Update the kit and Search everything were moved at once, by two agents in two worktrees off
  the same commit, then merged. big.go merged by itself: the sections went in at different
  places in folder order, and the two new imports (`kit`, `sort`) landed on different lines.
  big_helpers.go merged by itself (only Search added to it).
- The one conflict was notes.md: both entries appended at its end. Fixed by keeping both.
- Finding: parallel slices on one big file are fine when they're different stories, since
  folder order keeps their sections apart. An append-only log (notes.md) is what clashes.
  Two slices that both add helpers next to each other, or both cut seams in the same core
  file, would clash; neither happened here.
- After the merge: build, TestBigFile (9 of 17) and scripts/check.sh pass.

## Share setup between scenarios moves in (2026-10-25)
- shared.go, helper_edit.go and needs.go are gone. Their Share code is the section, between
  Search everything and Tags: `sharedFolder` and `OfferEdit`, then `elsewhere` under "Calling
  another capability's helper"; `listHelpers`, `callersOf` and `helperCode` under the two
  "same helper" scenarios; `setupTexts` under "See what a scenario needs installed";
  `findHelper`, `selectHelper`, `editHelper` and `EditingHelper` under "Edit a helper";
  `NewHelper` and `openHelperFile` under "Add a helper"; `SaveHelper` and `syntaxProblems`
  under "Save a helper with a syntax error". big.go imports context, os/exec, regexp, strconv
  and unicode/utf8 now.
- To big_helpers.go:
  - `Helpers`, the Shared list: Search's `findHelpers` and Views' `viewItems` and
    `ViewHelpers` call it, besides this section's `findHelper` and `EditingHelper`.
  - `RequiredCommands` and `ChromeHint`: the shared-code test. Its live callers are Drop in's
    `precheck` (dropin_run.go, "A command the setup needs isn't installed", and Mix shell's
    "No Chrome installed") and core glue's `requirementLabels` (snapshot.go), which shows this
    story's "See what a scenario needs installed". Two stories use it and it isn't a seam, so
    it's a helper. The share suite's steps also call it; steps don't count.
  - `syntaxCheckTimeout`, `helperProblemsMax`, `bashErrorPattern` and `bashError`: the
    editor's `shellProblems` (editor_more.go, Write stories) uses them as `syntaxProblems` does.
- `setupTexts` stays in the section, though `RequiredCommands` is its one caller. Same call as
  `readQuery` and `searchGroups`, which stayed in Search under `SearchResults`: the helper is
  the shared way in, and the walk it calls (own script, `from` setups, helpers those call) is
  this story's. Moving it too would have made big_helpers.go about +130, not +88.
- `elsewhere` stays in the section, though its one caller is Drop in's `Session.messages`
  (dropin_page.go). It holds this story's "Calling another capability's helper" and was always
  a named function Drop in calls, like the `trustOf` seam; the opening comment names the call.
- No seam cut. The helper offer in `allOffers` (offers.go) is one line, `return
  []string{OfferEdit}` when a helper is selected, so it stays in core glue with `Choose`'s
  `editHelper` case; no `helperOffers`. The opening comment names both.
- One reshaping: helper_edit.go's const block held the two bash -n limits and `OfferEdit`. It's
  split: the limits went to big_helpers.go as a block, `OfferEdit` to the section as a const of
  its own. Values and comments unchanged.
- helpers.go stays: only the Helper, RequiredCommand, HelperProblem and HelperEdit types. The
  header's "Code outside this file" list names it.
- The four "Changing …" scenarios and "Updating the kit keeps scenarios verified" have no
  heading: they're internal/fingerprint, called from status.go and dropin_record.go.
- Section 370 lines, big_helpers.go +88. big.go 1508, big_helpers.go 270. 10 of 17.

## Read a capability moves in (2026-10-25)
- history.go and copy_steps.go are gone. They're the section, between Copy and Search
  everything: `shownCapability` (from selection.go) under "Read a story" and "A story image
  that isn't on disk", the runs list (`RunsShownMax`, `RunLine`, `History`, `historyOf`,
  `runText`, `shownHistory`) under "See a scenario's runs", and `OfferCopySteps` with
  `gherkinOf` under "Copy a scenario's steps".
- status.go is gone too, all of it to big_helpers.go: `statusOf` is read by every scenario as
  shown (`scenarioOf`, project.go) and by Views; `StatusOf` by `givens list`; `statusFor` by
  an outline's rows (rows.go). Its status constants went with it.
- To big_helpers.go as well: `withImagePlaceholders` and its `aboutImage` regexp, from
  project.go. Read a capability's `capabilityOf` calls it, and so does Write stories' preview
  (preview.go).
- No seams cut. `shownScenario` stays in selection.go, as core glue: `allOffers` builds every
  scenario page's offers from it, and its Drop in branch shows the scenario dropped in to.
  "Read a scenario" and "A scenario with no setup" have no heading: they start there, in
  `scenarioOf` (project.go), and in `noSetup` (offers.go); the opening comment names them.
- `OfferCopySteps` stayed in the section, though Open a project's `readOnlyOffers` (branch.go)
  lists it: a label is the story's, as `SearchScenarios` stayed Search's though Copy reads it.
- Section 181 lines, big_helpers.go +113. big.go 1314, big_helpers.go 295. 10 of 17.

## Gherkin files moves in (2026-10-25)
- The section is between Copy and Read a capability: `RuleGroup` and `RuleGroups` (from
  selection.go) under "Rules group scenarios"; then, under "A file that isn't valid Gherkin"
  and "Two .feature files in one capability", `capabilityWarning` (project.go), then
  `problemCapabilities`, `capabilityOffers`, `editPrefix` and `moveToOwnCapability` (offers.go).
- Seams cut, both in core glue's offers.go: the fix-offers loop inline in `allOffers` is now
  `gherkinOffers`, which `allOffers` calls, like `kitOffers`; the "Edit <file>" branch inline
  in `Choose` is now `editBrokenFeature`, called as `answer` is. `Choose`'s one-line
  `moveToOwnCapability` call and `OfferMoveToOwnCapability`, in the page offers' const block,
  stay.
- To big_helpers.go: `slugClash`, since Write stories' rename (rename.go) calls it too, and
  `scenarioWarning` with `MessageTimeoutTag`: it warns on every scenario as shown, for this
  story's slug clash and Drop in's @timeout and Examples warnings.
- Plan was wrong here: examples.go isn't this story's. "Name each Examples block" is a Drop in
  scenario (outlines and rows), and Drop in's dropin_record.go and dropin_page.go are its other
  callers. It stays where it is, for slice 12e. `withImagePlaceholders` isn't this story's
  either: it moved to big_helpers.go with Read a capability.
- Not cut: `scenarioOffers`' "Both names make the slug" branch (offers.go) is three lines in a
  function the scenario page shares; the opening comment points at it.
- No file deleted. Section 163 lines, big_helpers.go +30. big.go 1477, big_helpers.go 325.
  11 of 17.

## Open a project moves in (2026-10-25)
- create.go and branch.go are gone. The section, between Gherkin files and Read a capability,
  holds at its top windows.go's limits, `Recents` and `rememberRecent`, then the windows
  Givens keeps (`Launch`, the welcome window, `OpenRecent`, the folder picker, `CloseWindow`,
  `OpenDropped`); `Open` and `toFront` from app.go under "Open a documented project", "Open the
  project folder instead" and "Open a folder with no capabilities"; create.go under "Create a
  capabilities folder"; and branch.go, bar `writable`, under "Read capabilities from another
  branch" and "Open a branch that doesn't exist".
- To big_helpers.go: project.go's loading (`capabilitiesFolderName`, `findCapabilities`,
  `reload`, `refresh`, `findCapability`, `findScenario`, `findIn`), which every story reads
  through, and `writable`, which Share setup's and Write stories' changes call.
- No seams cut. One-line calls into other stories stay where they are and the opening comment
  names them: `openRunStore` in `Open` and `runs.LinkRefCopy` in `OpenBranch` (Keep runs off
  main), `writeNew` in `createCapabilities` (Write stories' authoring.go), and core glue's calls
  (snapshot.go, offers.go).
- Stayed put: `New`, `Windows`, `Front`, `Quit` and `openWindowsFileName` in app.go, the App
  API every story's steps use; project.go's warnings (Gherkin files') and `capabilityOf` and
  `scenarioOf`, which turn a loaded capability into what a window shows and call those
  warnings. windows.go keeps `InstallCommand`, dead as noted above, and `givensCommand`, Drop
  in's.
- `OpenBranch` stays in the section: Keep runs off main reaches it only from its steps
  ("Another branch shows runs kept off main"), and test callers don't count.
- Update the kit's opening comment called reload and refresh core glue in project.go; it now
  says big_helpers.go.
- Section 458 lines, big_helpers.go +150. big.go 2,316, big_helpers.go 561. 13 of 17.
- Surprise: much of windows.go has no live scenario. `Launch`, `OpenRecent`, `OpenDropped` and
  `UseFolderPicker` are called only by `_archive/internal/serve` (and branch_test.go calls
  `Launch`); nothing live opens the welcome window, so its "Open folder…" and Recents are
  reached only from there. They moved in as the plan says, with the opening comment saying so.
  Deleting them with `InstallCommand` is the developer's call.

## Keep runs off main moves in (2026-10-25)
- runs_off_main.go is gone. All of it is the section, between Gherkin files and Open a project:
  the offers, messages, `ErrRunsOnMain` and `offMainText` at the top; `keepRun`, `writeRun`,
  `KeepsRunsOffMain`, `keepsRunsOffMain` and `afterRunsChange` under the recording scenarios
  (kept off main by default, without a remote, two devices, remote can't be reached, old runs
  dropped); the sync on open and `runsMessages` under "Runs show up on another device" and
  "Open a project whose remote doesn't answer"; `runsOffers`, `SyncRuns` and `syncRuns` under
  "Share runs once the remote is back"; `runsOnMain`, `askMoveRunsOffMain` and `moveRunsOffMain`
  under "Move runs off main"; `RunLimits` and `SetRunLimits` under "Change the limits".
  runs_off_main_test.go stays.
- Seam cut: Drop in's `RecordRun` (dropin_record.go) chose where a run goes in an if/else. It's
  now `keepRun` in the section, which `RecordRun` calls; dropin_record.go no longer imports
  runref. With the whole choice in the seam, `writeRun`, `keepsRunsOffMain` and
  `afterRunsChange` have no caller outside the section, so nothing went to big_helpers.go.
- Not cut: `openRunStore` in Open a project's `Open`, `runs.LinkRefCopy` in its `OpenBranch`
  ("Another branch shows runs kept off main", which has no heading here), `pollRunsSync` in
  `refresh`, and core glue's calls; the opening comment names them.
- window.go's field comments pointed at branch.go and runs_off_main.go; they now point at the
  sections.
- Section 342 lines, big_helpers.go +0. big.go 2,659, big_helpers.go 561. 14 of 17.
- Surprise: `writeRun` writes Drop in's note and attached files as well as the run line, so
  it's Drop in's record as much as this story's. It sits here because `keepRun` is its only
  caller; the narrower seam (only the off-main branch) would have sent it, `keepsRunsOffMain`
  and `afterRunsChange` to big_helpers.go.

## Wave A: three agents at once (2026-10-25)
- Share setup; Read a capability + Gherkin files; Open a project + Keep runs off main, each
  in its own worktree off f5eca2a, with no shared file owned by two agents.
- Share setup merged first, by itself. The next two branches clashed in code: both import
  blocks, the end of big_helpers.go (every slice appends its helpers there), project.go (two
  agents removing neighbouring code), plus the end of notes.md. Each clash was "keep both
  sides"; each branch was rebased in turn onto main and re-checked.
- Finding: big.go's sections stay apart by folder order, but the import blocks and the end of
  big_helpers.go are hot spots every slice touches. After this, the developer asked for the
  rest to go one at a time.

## Write stories and scenarios moves in (2026-10-25)
- Three commits, as the plan says. The section is last in big.go, after Work in the terminal
  (folder order: "work" sorts before "write").
- a) New and edit. authoring.go, editor.go and preview.go are gone; editor_more.go keeps only
  `setupFile` and `deleteFile`, for c. The section: the offer constants and
  `MessageChangedOnDisk` at the top; `NewStory` under "Write a new story"; `NewScenario` and
  `blockEnd` under "Add a scenario to a capability" and "A new scenario starts flagged to do";
  the step suggestions under "Reuse an existing step"; `Save`, `conflict`, `ChangedOnDisk`,
  `editorOffers` and `chooseInEditor` under the saving and changed-on-disk scenarios; `Marks`,
  `markParseErrors`, `capitalized`, `Problems` and `shellProblems` under "A line that isn't a
  step"; `previewable` and preview.go under the three Preview scenarios ("Preview a story with
  an image" is Preview's about.md branch); `editShown`, the editor's accessors, `dirty`,
  `closeEditor`, `CloseEditor` and `snapshot` under "Press e to edit a scenario" and "Edit the
  story on show". big.go imports internal/spec now.
- To big_helpers.go: `Edit` and `readEditable`, since Gherkin files' `editBrokenFeature`,
  Share setup's `editHelper` and `openHelperFile`, Update the kit's "Compare" and Drop in's
  `editFile` open files with `Edit`, and Tags' `flagTodo` reads with `readEditable`; and
  `writeNew`, since Open a project's `createCapabilities` writes with it.
- `editShown` came out of input.go: it's one story's function, not the dispatcher, as
  `shownCapability` came out of selection.go. Press's `case KeyEdit` stays. No seam cut.
- `blockEnd` stays in the section though `Edit`, now a helper, calls it: it finds a scenario's
  block, for typing a step there, and preview and delete are its other callers. Same call as
  `setupTexts` under `RequiredCommands`.
- `OfferCompare` stays in the section's const block though Update the kit offers it too: a
  label is the story's. `OfferNewScenario` stays in offers.go's page offers const block.
- Not cut: core glue's calls (`allOffers`' `editorOffers` and `OfferNewStory`, `Choose`'s
  `chooseInEditor`, `Type`'s `typeText`, `Select`'s `closeEditor`, `Snapshot`'s `snapshot`);
  the opening comment names them. edit.go stays: only types and limits; the header lists it.
- Gherkin files', Open a project's, Share setup's and Update the kit's opening comments pointed
  at authoring.go, editor.go or editor_more.go; they now point at the section or big_helpers.go.
- Section 680 lines, big_helpers.go +67. big.go 3,344, big_helpers.go 628. 15 of 17.
- b) Rename and delete. rename.go and delete.go are gone. Between "A line that isn't a step"
  and the Preview scenarios: `Rename`, `renameScenario`, `renameLine`, `moveSetup` and
  `updateFromLines` under "Rename a scenario", "Warned before a rename", "A renamed scenario
  starts at Never run" and "Scenarios built on it are unverified too"; `renameCapability` and
  `renameFeatureLine` under the two capability renames; the delete offers, `Delete`,
  `askDelete`, `builtOn`, `deleteScenario` and `deleteCapability` under the four delete
  scenarios.
- To big_helpers.go: `trash`, since Keep runs off main's `moveRunsOffMain` sends old run files
  to the Trash too, and `withoutTrashed`, which `reload` and `refresh` call on every load.
- No seams cut. `Rename` and `Delete` are also called from cmd/givens/manage.go, the edge, and
  branch_test.go; the opening comment names manage.go.
- Surprise: moved unchanged, `Rename`'s doc comment broke `TestBigFile`. One of its lines
  started "// Scenario: or Feature: line, …", which reads as a Scenario heading. Rewrapped so
  "the" starts that line; same words. A heading check that keys on a line's start can trip on
  prose, so a comment that wraps onto "Scenario:" or "Feature:" needs the same care.
- Section +338 lines (1,020 in all), big_helpers.go +49. big.go 3,684, big_helpers.go 677.
- c) about.md and setup files. about_edit.go and editor_more.go are gone. At the section's end:
  about_edit.go (its about.md offers, `storyCapability`, `aboutOffers` and `editAbout`) under
  "Edit a capability's about.md" and "Add an about.md"; `createSetup` under "Write setup steps";
  `setupFile` and `deleteFile` under the two "Delete a setup …" scenarios.
- `createSetup` came out of offers.go, though the task named only about_edit.go, `setupFile` and
  `deleteFile`: it's "Write setup steps", and `chooseOnScenarioPage` is its one caller. It isn't
  the dispatcher, so it moved as `moveToOwnCapability` did. The "Create script" and "Write setup
  steps" labels stay in offers.go's const block with `MessageNoSetup` (Read a capability's "A
  scenario with no setup"), and `KitSourceLine` stays there too: Drop in's `addTeardown` uses it.
- `storyCapability` stays in the section, though `editShown` (a) calls it: both are this story's.
- Nothing went to big_helpers.go. No seams cut.
- Section +158 lines, 1,182 in all; big_helpers.go +0. big.go 3,846, big_helpers.go 677.
  15 of 17. internal/app is down to 29 Go files.

## Mix shell, browser and hand steps moves in (2026-10-25)
- cards_run.go is gone. The section is between Keep runs off main and Open a project:
  `cardOutputLinesMax` and `blockRun` at the top; `givensBin` and `givensCommand` (from
  windows.go) under the ten Drop in browser scenarios, since GIVENS_BIN is all the app code
  those have (the rest is the kit, cmd/givens/browser.go and internal/browser); `ChromeHint`
  and `chromeRequired` under "No Chrome installed"; `openLink` (from cards.go) under "A step by
  hand in the Givens browser"; cards_run.go's intro, `runLanguage`, `runNextCard`,
  `startBlock`, `blockScript` and `runState` (from dropin_page.go) under the two card
  scenarios; `pollBlock` and `stopBlock` under "A card's run block fails". big.go imports
  internal/browser, internal/markdown and internal/setup now.
- Seams cut: Drop in's `begin` (dropin_run.go) set GIVENS_BIN inline; it's now `givensBin`,
  which `begin` calls. `RequiredCommands`' Chrome check (big_helpers.go) is now
  `chromeRequired`, which `RequiredCommands` calls; big_helpers.go no longer imports
  internal/browser.
- To big_helpers.go: `runBlocksIn`, since Share setup's `setupTexts` and Drop in's
  `startSession` and `precheck` call it (the `RequiredCommands` call again); `scratchDir`, from
  arguments.go, since Drop in's `writeArguments` and this story's `blockScript` share it.
- `runLanguage` stays in the section though its callers are the helper `runBlocksIn` and Drop
  in's `addMarkdownCards`: it says what a run block is, the same call as `blockEnd` (a helper
  calls it) and `elsewhere` (Drop in calls it).
- `runNextCard` and `openLink` came in with this story's first commit and hold Drop in's part
  too (a card that waits, the checklist, the default browser); they're this story's, called
  from Drop in like `keepRun`, and sit here as `writeRun` does.
- Not cut, one-line calls named in the opening comment: `startSession`'s and `precheck`'s
  `runBlocksIn`, `addMarkdownCards`' `runLanguage` and run-block offers, `markMarkdownCard`'s
  `runNextCard`, `ChooseOnCard`'s `openLink`, `cardLines`' `runState`, `poll`'s and `Wait`'s
  block checks (session.go), and `stopBlock` in `choose` and `leaveSession`.
- Plan was wrong here: `givensCommand` was down for Drop in (12a), but with GIVENS_BIN in
  `givensBin` its one live story caller is this section; the other is the dead
  `InstallCommand`, which windows.go now holds alone. The header gains internal/browser.
- Section 250 lines, big_helpers.go +39. big.go 4,101, big_helpers.go 716. 16 of 17.

## Drop into a scenario moves in (2026-10-25)
- Five commits, as the plan says. The section is between Copy a setup script and Gherkin files.
- a) Run and timeouts. session.go is gone; dropin_run.go keeps only `fromProblem` and
  `outlineWithoutRow`, for c. At the section's top: session.go's states and page offers,
  `pollInterval`, `CheckItem`, the `Session` type and `Window.Session`. Then `DropIn`, `dropIn`,
  `startSession`, `begin`, `setupEnv`, `start`, `poll`, `finishScripts` and the session's
  accessors (`Status`, `Output`, `Wait`, `Checklist`, `ScenarioKey`, `Givens` with `givensOf`,
  `FileOf`, `Note`) under "Drop in" and "The setup script fails"; `precheck` under "A command
  the setup needs isn't installed"; `stillRunning` and `spokenDuration` under the three timeout
  scenarios; `firstYouLine` and `unquote` under "Steps done by hand come first".
- To big_helpers.go: `checklistOf`, `fail` and `restart`. Mix shell's `runNextCard` shows the
  checklist with `checklistOf`, its `startBlock` and `pollBlock` call `fail`, and Trust's
  `trustThisProject` calls `restart`. Same call as `scratchDir`: a Session method a guest
  story calls is shared code.
- `begin` stays, though its one caller is Trust's `runTrusted`. That's a seam cut out of
  `startSession`, which only decides whether `begin` runs; `begin` is the Drop in itself. Rule
  used: a seam's call back into its host doesn't move the host's code. (`writeRun` went the
  other way in Keep runs off main, but half of it was that story's own work.)
- The Session states and the page offers stay in the section, though Mix shell and Trust set
  `SessionWaiting`, `SessionReady` and `SessionStopped`: they name Drop in's states, as a label
  stays with its story.
- No seams cut. The `Session` type and `CheckItem` moved with the code: session.go wasn't a
  types-only file. The header lists dropin.go, which stays: Drop in's data model.
- Mix shell's and Share setup's opening comments pointed at dropin_run.go and session.go; they
  name the functions only now.
- Section 450 lines, big_helpers.go +49. big.go 4,554, big_helpers.go 765.
- b) Record and attach. dropin_record.go and dropin_files.go are gone. After "Steps done by
  hand come first": the marks, `MessageMarkEachCheck`, `Mark`, `SetRunNote`, `RecordRun` and
  `key` under "Record a run", "Record a run before marking every check", "Record a run with a
  note in Markdown" and "A run records who ran it"; dropin_files.go's intro, `Attach`, `Detach`,
  `Attachments` and `runFiles` under "Attach proof to a run". big.go imports
  internal/fingerprint now.
- `runFiles` stays in the section, though its one caller is Read a capability's `historyOf`:
  it's this story's "the run, in the scenario's run history, lists declined.png", the same call
  as `elsewhere` (Share's, called only from Drop in).
- `MarkDone` stays with the marks, though Mix shell's `runState` and `pollBlock` read it.
- "Runs on two branches merge cleanly" has no heading: it's git's merge=union, which Open a
  project's `createCapabilities` writes; the opening comment says so.
- Nothing went to big_helpers.go. No seams cut. Keep runs off main's and Share setup's opening
  comments named dropin_record.go; they name `RecordRun` only now.
- Section +172 lines, 622 in all; big_helpers.go +0. big.go 4,727, big_helpers.go 765.
- c) Teardown and builds-on. dropin_run.go is gone. After "Attach proof to a run": `addTeardown`
  (from offers.go) under "Add a teardown"; `fromProblem` and `outlineWithoutRow` under "Builds on
  a scenario that doesn't exist", "Setups that build on each other in a loop" and "Build on an
  outline without a row". "Teardown runs before setup" and "The teardown fails" join the stack
  over `DropIn` … `Note`: `begin` queues the teardown, `start` prints "→ teardown", `poll` says
  "Teardown failed", and `dropIn`'s skipTeardown is "Run setup anyway".
- To big_helpers.go: `KitSourceLine`, since Write stories' `NewScenario` and `createSetup` start
  a setup script with it, as `addTeardown` starts a teardown. offers.go no longer imports os.
- `addTeardown` came out of offers.go as `createSetup` did: `chooseOnScenarioPage` is its one
  caller, and it isn't the dispatcher. No seams cut.
- No heading, the opening comment says why: "A scenario that builds on another", "Each level
  tears down its own state", "Build on one row of an outline" and "A loop run from the
  terminal" are the kit's `from` (bash); "Running setup twice gives the same state" is the
  setup's own idempotency, with "Reset state" in `restart`.
- Write stories' opening comment said KitSourceLine stays in offers.go; it says big_helpers.go.
- Section +99 lines, 721 in all; big_helpers.go +3. big.go 4,827, big_helpers.go 768.
- d) Cards, steps by hand and asks. cards.go and dropin_page.go are gone. After "Build on an
  outline without a row": `ChecklistPart` and `Parts` under "Steps stay in the order they're
  written"; the page (dropin_page.go's intro, `snapshot`, `messages`, `offers`, `choose`,
  `failedOffers`, then `scenarioFileOf` from offers.go and `leaveSession` from selection.go)
  under "Reset state part way through"; the card offers and messages, `cardHeader`, `Cards`, the
  script cards' reading and answering, `pendingCard` and `PendingCard` under the five by_hand
  and ask scenarios; `UseURLOpener` under "A step in the browser"; `addMarkdownCards`,
  `markMarkdownCard` and `cardLines` under the five markdown-card scenarios; `CopyCodeBlock`,
  `ClickInlineCode` and `CopyCard` under the three copy scenarios. "Waiting doesn't count as
  slow" joins the stack over `stillRunning`, which takes the time waited off.
- To big_helpers.go: `resume`, since Mix shell's `runNextCard` resumes too, and `editFile`,
  since Write stories' `createSetup` opens its new file with it (and core glue's
  `chooseOnScenarioPage` opens setup files).
- `leaveSession` and `scenarioFileOf` came out of core files: each is one story's function, not
  the dispatcher, as `editShown` came out of input.go. `leaveThenSelect` stays in selection.go:
  it's Select's own flow, and calls `marked` and `leaveSession`. No seams cut.
- The opening comment is now in paragraphs: what the story is, the guests it calls, what's in
  big_helpers.go, the bash and git scenarios, and core glue's calls in.
- Mix shell's, Share setup's and Write stories' opening comments named cards.go and
  dropin_page.go; they name the functions only now, or big_helpers.go for `editFile`.
- Section +569 lines, 1,290 in all; big_helpers.go +25. big.go 5,395, big_helpers.go 793.
- e) Outlines and rows. rows.go, arguments.go and examples.go are gone. At the section's end:
  the row offers, `fillPlaceholders`, `rowNamed`, `DropInRows`, `ChooseOnRow` and `marked` under
  "See every row on one page", "Drop in to one row" and "Switch to another row";
  `outlineStatus` under "Record a run for one row"; `writeArguments` and `writeArgument` under
  "Step data reaches the script"; `MessageNameExamples` and `examplesWarning` under "Several
  unnamed Examples blocks". big.go imports encoding/csv now.
- To big_helpers.go: `outlineRow`, `rowsOf`, `rowStatus` and `rowName`, since Views'
  `unverifiedRows` lists an outline's rows with them; `removeArguments`, since Open a project's
  `CloseWindow` calls it (it sits next to `scratchDir`, the folder it removes).
- `outlineStatus` and `examplesWarning` stay in the section, though their callers are helpers
  (`StatusOf`, `scenarioWarning`): same call as `blockEnd` and `setupTexts`. `marked` stays,
  though core glue's `leaveThenSelect` calls it. No seams cut.
- Section +224 lines, 1,514 in all; big_helpers.go +54. big.go 5,620, big_helpers.go 847.
  17 of 17. 41 of the 47 scenarios have a heading; the other six are the kit's `from`, git's
  merge=union and the setup's own idempotency, and the opening comment names them.
- In all: 9 files gone (session.go, dropin_run.go, dropin_record.go, dropin_files.go, cards.go,
  dropin_page.go, rows.go, arguments.go, examples.go), plus `addTeardown` and `scenarioFileOf`
  from offers.go and `leaveSession` from selection.go. big_helpers.go +131 against the
  section's 1,514. No seams cut: the guests had gone first, as the plan meant.
- Surprise: the guests call back into their host. Mix shell's cards call `fail`, `resume` and
  `checklistOf`, and Trust's seams call `begin` and `restart`. By the callers rule those Session
  methods are shared, so four of them went to big_helpers.go; `begin` stayed, as the run a seam
  only gates. A host's state machine its guests drive is split by the rule, not by the story.
- Left in internal/app beside big.go and big_helpers.go, 2,127 lines in 17 files: core glue
  (snapshot.go 357, offers.go 224, selection.go 123, prompt.go 68, input.go 59, project.go 52);
  App and Window (app.go 138, window.go 169); data models (dropin.go 66, edit.go 63, helpers.go
  51); the dead install code (install.go 95, windows.go 32); and tests (app_test.go 414,
  branch_test.go 107, runs_off_main_test.go 58, todo_test.go 51).

## Finish (2026-10-25)
- All 17 stories have a section. `TestBigFile` now fails on a Feature with no section (checked
  by hiding Views' heading), instead of counting stories still to come.
- big.go's header lists every file outside it and why: the edges, the kit, the three data
  models, internal/browser, the core glue, app.go and window.go, the domain packages, and the
  dead install code.
- Sizes: big.go 5,620 lines, big_helpers.go 847; internal/app's non-test code 7,964 lines in
  15 files (46 files at the start). Left out of the two big files: core glue 883 (snapshot.go
  357, offers.go 224, selection.go 123, prompt.go 68, input.go 59, project.go 52), the App and
  Window types 307, data models 180, dead code 127.
- Core glue, decided: it stays apart (plan.md's A). B would put 883 lines of plumbing into
  big_helpers.go (to 1,730): still under big.go, so not a tripwire, but the helpers file would
  then be mostly dispatcher and view code rather than code stories share.
- Tripwire: never tripped. big_helpers.go ended at 15% of big.go.
- Speed at 5,620 lines, measured from a cold Go build cache: `go build ./internal/app` 5.6 s
  cold and 0.1 s warm; tigerlint on internal/app 3.2 s; TestBigFile 1.4 s; `scripts/entry.sh`
  0.03 s; `gopls check big.go` 2.4 s and `gopls references` 1.6 s from the command line
  (both include gopls starting up); one scenario's test (Record a run) 1.5 s.
- Hops: `scripts/entry.sh` on each story's first scenario lands in that story's section for
  all 17; Gherkin files' and Tags' first scenarios land on their Feature's section (no heading
  of their own), and the three edge stories on their pointer sections.
- What the work found, beyond the pilot:
  - Most moves needed no seam. Seams were needed where one story's code sat inline in a
    shared or host function (trust in Drop in, the kit offer in allOffers, the run-store
    choice in RecordRun, GIVENS_BIN in begin, the Chrome check in RequiredCommands).
  - Guests before hosts paid off: by Drop in's turn, its five slices cut no seams.
  - The rules the agents settled (a seam stays with its story; live callers only; labels stay
    with their story; core one-line calls stay and are named) were enough for every case.
  - Dead code surfaced twice (install code, most of the window code), each found because a
    move asked "who calls this?".
  - Parallel slices: fine when the stories and their files are apart; clashes came from the
    import blocks, the end of big_helpers.go, and notes.md. The rest went one at a time.
  - A moved doc comment with a wrapped line starting "// Scenario:" read as a heading.

## Dead code deleted (2026-10-25)
- Gone: install.go and windows.go (InstallFolders, InstallCommand), and from Open a project's
  section Launch, the welcome window, OpenRecent, the folder picker (UseFolderPicker,
  openFolder, FolderPickerShown) and OpenDropped. Only `_archive/internal/serve` called them.
- What fell away with them: Recents and recents.txt (written on every Open, read only by the
  welcome window), Quit's open-windows.txt (read only by Launch), the WindowWelcome kind and its
  branches in window.go, snapshot.go and offers.go, "Open folder…", and the snapshot's Recents
  and FolderPickerShown fields. Tests: TestWritableFolders, and TestBranchCopiesCleanedUp's
  Launch half.
- Launch also cleared stale branch copies; the TUI never called it, so that wasn't happening.
  A todo.txt line covers it.
- `deadcode` (golang.org/x/tools) didn't find these: reflection (JSON snapshots, chromedp)
  keeps every exported method alive to it. Grepping callers outside `_archive/` did.

## Data models get a package (2026-10-25)
- At the developer's ask, the type-only files moved to `internal/app/model` (package
  `model`): dropin.go, edit.go and helpers.go, plus CheckItem from Drop in's section, which
  DropInRow holds. 176 references were found with `gopls references` and qualified as
  `model.X` (or `app.X` became `model.X` outside the package); goimports fixed the imports.
- Two exceptions stayed in `app`: Editor, which holds a *Window and has 24 methods (a type's
  methods must be in its package), now opens Write stories' section; and
  StorySentenceLengthMax, which is NameLengthMax from app.go.
- SetupCard's unexported inline and runBlocks fields became Inline and RunBlocks (gopls
  rename), tagged `json:"-"` so the JSON is unchanged.
- app.go and window.go stay: App and Window carry methods, so they aren't data-only.

## internal/fingerprint folds in (2026-10-25)
- internal/fingerprint is gone. Renames: `Of` -> `FingerprintOf`, `Changed` ->
  `FingerprintChanged`, `PartSteps` and `PartSetup` -> `FingerprintPartSteps` and
  `FingerprintPartSetup` (in app, "part" reads as Drop in's checklist parts); the test's
  `copyShop` -> `copyShopCapabilities` (it copies only the capabilities folder; app_test.go's
  copies the whole shop).
- To big_helpers.go, after the status code: the part names, `FingerprintOf` and `hash`, since
  statusFor and Drop in's `RecordRun` both call `FingerprintOf`.
- To Share setup's section, before "See what a scenario needs installed": `stepsText`,
  `setupText`, `fromChain`, `calledHelpers`, `sortedParts`, `FingerprintChanged` and `partWords`,
  under the four "Changing …" headings and "Updating the kit keeps scenarios verified", which had
  none. The package doc is their opening comment. Same call as `setupTexts` under
  `RequiredCommands`: the helper is the shared way in, the walk it calls is the story's.
  `FingerprintChanged`'s one caller is statusFor, a helper, as `outlineStatus`'s is `StatusOf`.
- fingerprint_test.go is internal/app/fingerprint_test.go. Lines the longer names pushed past
  100 columns were rewrapped; nothing else changed.
- big.go 5,651 lines, big_helpers.go 900 (+52).
- Surprise: fold.py's `gopls -remote=auto` can't reach a daemon here; a copy without
  `-remote=auto` worked, about 18 s for this package.

## internal/branch folds in (2026-10-25)
- internal/branch is gone. All of it is at the end of Open a project's section, after
  `shortHash`, under its package doc: only `Branches` and `OpenBranch` call it.
- Renames: `List` -> `ListBranches`, `Extract` -> `ExtractBranch`, `checkName` ->
  `checkBranchName`, `EntriesMax` -> `ArchiveEntriesMax`, and `NameLengthMax` ->
  `BranchNameLengthMax`. That last one clashed with app's `NameLengthMax`, but the values differ
  (a branch name 256, app's names 1024), so it was renamed, not merged: merging would change
  which branch names `OpenBranch` turns down.
- branch_test.go is internal/app/branch_archive_test.go, beside app's own branch_test.go.
- Open a project's opening comment and the "Read capabilities from another branch" note named
  internal/branch; they point at the section's end now.
- big.go 5,873 lines, big_helpers.go 900 (+0).

## internal/watch folds in (2026-10-25)
- internal/watch is gone. All of it is in Open a project's section under a new "Files written
  by other tools appear" heading, before the branch scenarios, with its package doc as the
  opening comment.
- Not big_helpers.go, though every story reads through it: its only callers are reload and
  refresh, helpers. Same call as `outlineStatus` under `StatusOf`: the helper is the shared way
  in, and the watcher is this one scenario's.
- Renames: `New` -> `NewWatcher` (clashed with app's `New`), `Interval` -> `WatchInterval`,
  `FilesMax` -> `WatchFilesMax`, the type `file` -> `watchedFile`; the test `TestChanged` ->
  `TestWatcherChanged` (clashed with fingerprint's, folded in first). `look` and `same` kept.
- watch_test.go is internal/app/watch_test.go. big.go's header list and Open a project's
  opening comment updated; `_archive/internal/serve` still says `watch.Interval`, left as is.
- big.go 5,989 lines, big_helpers.go 899 (-1, an import line).

## internal/runref folds in (2026-10-25)
- internal/runref is gone. runref.go, sync.go and merge.go, in that order, are the end of Keep
  runs off main's section, under the package doc: the section is its only caller (core glue's
  "Share runs" in offers.go passes `RunsNetworkTimeout`; window.go holds the `*RunStore`).
- No clashes, but names that leaned on the qualifier were renamed: `Store` -> `RunStore`,
  `Open` -> `RunStoreOf` (app already has `openRunStore` and `App.Open`), `Ref` -> `RunsRef`,
  `Remote` -> `RunsRemote`, `NetworkTimeout` -> `RunsNetworkTimeout`, `OpenTimeout` ->
  `RunsOpenTimeout`, `Adopt` -> `AdoptRuns`, `ErrUnreachable` -> `ErrRemoteUnreachable` (alone,
  "unreachable" reads as a code path). The unexported names and the methods kept theirs.
- The steps in acceptance/keep-runs-off-main and work-in-the-terminal call `app.RunStoreOf`,
  `app.RunsRef` and the timeouts now. internal/runs/ref.go's comment named internal/runref; it
  names RunStore now.
- runref_test.go is internal/app/runref_test.go; two lines the longer timeout name pushed past
  100 columns were rewrapped.
- big.go 6,731 lines, big_helpers.go 899 (+0).

## internal/helpers folds in (2026-10-25)
- internal/helpers is gone. Reading the helpers (the limits, `HelperDefinition`,
  `HelpersInFolder` and its file walk, `ReadScript`) and the from lines (`FromLine`,
  `FromLines`) are in big_helpers.go after RequiredCommands: Copy, Drop in, Search, Share setup
  and Write stories call them. Which helpers a script calls (`VisibleHelpers`, `HelpersCalled`,
  `callsIn`, `shellWord`) and the need lines (`NeedLine`, `NeedLines`) are the end of Share
  setup's section: their callers are the fingerprint walk, setupTexts' neighbours and
  RequiredCommands, as `setupTexts` is. `UsesBrowser` is the end of Mix shell's section; its one
  caller is chromeRequired.
- The package doc's first paragraph became the big_helpers.go block's comment; the
  "reads bash by pattern" paragraph moved with it. The const and var blocks were split by where
  each pattern went.
- Renames: `Helper` -> `HelperDefinition` (beside model.Helper, the sidebar's view of one),
  `InFolder` -> `HelpersInFolder`, `inFile` -> `helpersInFile`, `Visible` -> `VisibleHelpers`
  (Window has a `Visible` method), `Called` -> `HelpersCalled` (not CalledHelpers: Share setup
  has a `calledHelpers`), `FileSizeBytesMax` -> `ScriptSizeBytesMax`, `KitFile` ->
  `KitFileName` (Update the kit has `kitFile`, the path), `Need`/`Needs` -> `NeedLine`/`NeedLines`,
  `From`/`Froms` -> `FromLine`/`FromLines`, and the vars `definition` -> `helperDefinition`,
  `word` -> `shellWord`, `needLine`/`fromLine` -> `needLineRegexp`/`fromLineRegexp` (they'd
  differ from the types only by case). `ReadScript`, `HelpersMax`, `UsesBrowser`, `docAbove`,
  `bodyBelow` and `callsIn` kept theirs. The test `TestInFolderAndCalled` ->
  `TestHelpersInFolderAndCalled`.
- The two-regexp block's `//tiger:allow-global` moved to the block's doc comment: the longer
  name pushed the line past 100 columns.
- helpers_test.go is internal/app/helpers_test.go; lines_browser_test.go is
  helpers_browser_test.go. `cmd/givens` calls `app.ReadScript`.
- big.go 6,846 lines (+115), big_helpers.go 1,084 (+185).

## internal/markdown folds in (2026-10-25)
- internal/markdown is gone. The reader (limits, patterns, `FencedBlock`, `MarkdownStep`,
  `MarkdownSetup`, `ParseMarkdownSetup` and its three helpers) is in big_helpers.go before
  runBlocksIn, under the package doc: Drop in's addMarkdownCards, runBlocksIn (Drop in, Share
  setup, Mix shell) and `givens run` call it. `RenderedMarkdown` is the end of Drop in's
  section (addMarkdownCards is its one caller); `PlainMarkdown` is the end of Use Givens from
  the terminal's pointer section (printStep in cmd/givens/run.go is its one caller). Drop in's
  and the pointer's opening comments say so.
- Renames: `Parse` -> `ParseMarkdownSetup`, `Setup` -> `MarkdownSetup`, `Step` ->
  `MarkdownStep` (beside project.Step), `Block` -> `FencedBlock` (model has CodeBlock, the card's
  view), `Plain` -> `PlainMarkdown`, `Rendered` -> `RenderedMarkdown`, `TextBytesMax` ->
  `MarkdownSetupBytesMax`, `StepsMax` -> `MarkdownStepsMax`, `finish` -> `finishMarkdownStep`,
  and the patterns and regexps `item`, `link`, `inline`, `fence` get a `markdown` prefix. The
  test `TestParse` -> `TestParseMarkdownSetup`. `closeOrAdd` and `leadingSpaces` kept theirs.
- Three lines the longer names pushed past 100 columns were wrapped (closeOrAdd's signature and
  two errors in ParseMarkdownSetup).
- markdown_test.go is internal/app/markdown_test.go; run_test.go is markdown_run_test.go.
- big.go 6,884 lines (+38), big_helpers.go 1,242 (+158).

## internal/setup folds in (2026-10-25)
- internal/setup is gone. setup.go and process.go, in that order, are the end of
  big_helpers.go under the package doc: Drop in runs its scripts with `StartSetup` and
  `SetupProcess`, and Mix shell its run blocks. blocks.go (`ScriptAsEnv`, `BlockLanguage`,
  `WriteRunBlock`, `QuoteBashWord`) is the end of Mix shell's section: its other caller is
  runBlocks in cmd/givens/run.go, which is Mix shell's "Run cards in a terminal". Drop in's
  and Mix shell's opening comments name the big_helpers.go part.
- Renames: `Process` -> `SetupProcess`, `Start` -> `StartSetup`, `Request` -> `SetupRequest`,
  `Result` -> `SetupResult`, `ErrStopped` -> `ErrSetupStopped` (its text is "setup was
  stopped"), `DurationMax`, `OutputLinesMax`, `LineLengthBytesMax` and `EnvMax` get a `Setup`
  prefix, `WriteBlock` -> `WriteRunBlock`, `Quote` -> `QuoteBashWord`; the test `TestAnswer` ->
  `TestSetupAnswer` (Drop in's ask cards have answers too). `pipes`, `newPipes`, `stopGrace`,
  `drainWait`, `ScriptAsEnv`, `BlockLanguage` and the methods kept theirs.
- The package doc's "internal/app chooses the script" became "its callers choose the script".
- Surprise: cmd/givens imported internal/setup as `setupkit`, since runSetup has a local
  `setup`; fold.py expects the package's own name before each reference, so the alias was
  turned back to `setup` first (runBlocks, the user, has no local of that name). It calls
  `app.BlockLanguage`, `app.WriteRunBlock` and `app.ScriptAsEnv` now.
- setup_test.go is internal/app/setup_process_test.go. Two lines the longer names pushed past
  100 columns were wrapped.
- big.go 6,937 lines (+53), big_helpers.go 1,522 (+280).

## internal/tags folds in (2026-10-25)
- internal/tags is gone. All of it is in big_helpers.go after tagViews and isViewTag, under
  the package doc: Tags' parseTags and `givens list --tags` (cmd/givens/list.go, Use Givens
  from the terminal) parse an expression, and Search's findScenarios matches one, so three
  stories use it. parseTags stays in Tags' section, where the search-by-tag scenarios start.
- Renames: `Expression` -> `TagExpression`, `Parse` -> `ParseTagExpression`, `ErrSyntax` ->
  `ErrTagSyntax`, `TokensMax` -> `TagExpressionTokensMax`, and the unexported `tokenize` ->
  `tokenizeTagExpression`, `precedence` -> `tagPrecedence`, `contains` -> `hasTag`, the
  operator constants get a `tag` prefix (`tagOperatorNot` …). `shunt` and the methods kept
  theirs. Tests: `TestMatch`, `TestParseErrors` and `TestParseErrorMessage` ->
  `TestTagExpressionMatch`, `TestParseTagExpressionErrors` and
  `TestParseTagExpressionErrorMessage`.
- Tags' opening comment, the terminal pointer's "List by tag" and big.go's header list
  updated; the header's domain packages are now spec, project, runs, slug, kit and starter.
- tags_test.go is internal/app/tag_expression_test.go. Two errors the longer names pushed past
  100 columns were wrapped.
- big.go 6,937 lines (+0), big_helpers.go 1,718 (+196), 25% of big.go.

## internal/runs folds in (2026-10-25)
- internal/runs is gone. The run log is in big_helpers.go before the scenario statuses, under
  the package doc: the line format and its limits, `Run`, `RunPair`, `RunKey`, `RunsDir`,
  reading (`ReadRuns`, `ReadRunsFolder`, `ParseRun`, `LatestRun`), the attached files' folder
  and checks (`AttachedFilesDir`, `CheckAttachedFile`, `FilesPerRunMax`, the note limits), and
  the ref's copy (`RunsRefCopy`, `GitCommonDir`, `AttachedFilesFolder`): Drop in, Keep runs off
  main, Read a capability, Share setup and Write stories read it.
- One-story parts went to their sections' ends: writing a run (`AppendRun`), `FreeRunTime`,
  `CopyAttachedFiles`, the note's summary and file, `OffMainFileName`, `MergeRunLines` and all
  of prune.go to Keep runs off main, after the run store; `RunAuthor` and `AttachedFiles` to
  Drop in; `ReadRunNote` to Read a capability; `LinkRunsRefCopy` to Open a project. The
  attached-files const block was split: `FreeRunSecondsMax` went with `FreeRunTime`.
- Renames (most leaned on the qualifier): `Read` -> `ReadRuns`, `ReadFolder` ->
  `ReadRunsFolder`, `Append`/`Parse`/`Latest`/`Prune` -> `AppendRun`/`ParseRun`/`LatestRun`/
  `PruneRuns`, `Key` -> `RunKey`, `Dir` -> `RunsDir`, `Pair` -> `RunPair`, `Author` ->
  `RunAuthor`, `ResultPass`/`ResultFail` -> `RunResultPass`/`RunResultFail`, `ErrBadLine` ->
  `ErrBadRunLine`, `RunFilesMax` -> `RunLogFilesMax` (beside the attached files, "run files"
  misled), `LineLengthBytesMax`/`FieldsMax` get a `Run` prefix; attached files say so:
  `Files`/`FilesDir`/`FolderOf`/`CheckFile`/`CopyFiles`/`FileBytesMax` ->
  `AttachedFiles`/`AttachedFilesDir`/`AttachedFilesFolder`/`CheckAttachedFile`/
  `CopyAttachedFiles`/`AttachedFileBytesMax`, and the clash `copyFile` -> `copyAttachedFile`
  (Keep runs off main's `copyFile` copies runs/files into the ref); `FreeTime` -> `FreeRunTime`;
  notes: `Summary` -> `NoteSummary`, `WriteNote`/`ReadNote`/`NoteFileName`/`NoteBytesMax` get a
  `Run`; the ref: `RefCopy`/`LinkRefCopy`/`RefCopyLinkName`/`RefDirName` -> `RunsRefCopy`/
  `LinkRunsRefCopy`/`RunsRefCopyLinkName`/`RunsRefDirName` (like runref's `RunsRef`); limits:
  `Limits`/`DefaultLimits`/`ReadLimits`/`WriteLimits`/`LimitMax`/`LimitsFileName` get a `Run`;
  `ReadLines` -> `ReadDayLogLines`; unexported: `format` -> `formatRun`, `readFile` ->
  `readRunFile`, `timeTaken` -> `runTimeTaken`, `rowKey` -> `runRowKey` (app's rows are outline
  rows), `removeFolder` -> `removeRunFilesFolder`, `folderBytes` -> `runFilesFolderBytes`,
  `writeLog` -> `writeDayLog`. `Run`, `GitCommonDir`, `FilesPerRunMax`, `OffMainFileName`,
  `RunsKeptDefault`, `MegabytesKeptDefault`, `PruneToPercent` and the other unexported names
  kept theirs: `Run` is the word every story uses.
- Tests: runs_test.go, files_test.go and prune_test.go are internal/app/runs_test.go,
  runs_files_test.go and runs_prune_test.go. Lines the longer names pushed past 100 columns
  were rewrapped. Comments that said `runs.X` or internal/runs (Drop in's attached files, Keep
  runs off main's opening comment and run store, Open a project's branch note) name the new
  names; big.go's header list drops runs.
- big.go 7,541 lines (+604), big_helpers.go 2,120 (+402), 28% of big.go.

## internal/project folds in (2026-10-25)
- internal/project is gone. project.go and scenario.go, in that order, are in big_helpers.go
  before `capabilitiesFolderName` and findCapabilities, under the package doc: reload and
  findCapabilities load the project, `givens list` and `givens run` too, and every story reads
  its types.
- Renames: the clashes `Capability` -> `LoadedCapability` and `Scenario` -> `LoadedScenario`.
  app's Capability and Scenario are what a window shows (JSON-tagged, a status, rendered
  about.md, steps as text); project's are what was read from disk (folder, paths, setup files,
  parsed steps), and `capabilityOf`/`scenarioOf` turn a loaded one into a shown one, so
  "Loaded" says which side of that a value is on. `Project` -> `LoadedProject` and `Load` ->
  `LoadProject` to match (a bare `Project` in app reads as the window's project). `Step` ->
  `GherkinStep` (beside `MarkdownStep`), `Examples` -> `ExamplesBlock` (it's one block; the
  field stays `Examples`), the clash `rowsOf` -> `tableRowsOf` (app's `rowsOf` is an outline's
  rows), `scenarioOf`/`scenariosOf` -> `loadScenario`/`loadScenarios` (Window's `scenarioOf`
  goes the other way), `readTags` -> `readScenarioTags` (Tags has `parseTags`), `existing` ->
  `existingFile`. The limits, the `Tag*` constants, `ErrNoScenario`, `Find`, `Key` and the
  other unexported names kept theirs.
- project_test.go is internal/app/project_test.go (app had none). Gherkin files' and Tags'
  opening comments named internal/project; they name LoadProject and loadScenario now. big.go's
  header list drops project.
- big.go 7,540 lines (-1, an import), big_helpers.go 2,585 (+465), 34% of big.go.

## internal/spec folds in (2026-10-25)
- internal/spec is gone. Parsing is in big_helpers.go before the project loader, under the
  package doc: the limits, `ScenariosIn`, `ParseFeature`, `ErrNoFeature`, `ParseFeatureText`,
  `FeatureScenarios` and explain.go (`ExplainParseError` and its pattern): the loader, Write
  stories' editor and `givens list` parse with it, and the steps list scenarios with
  `ScenariosIn`. The conventions check (`ConventionProblem`, `CheckCapabilities`,
  `checkCapability`, `CheckFeature`, `checkStory`, `checkScenarios`, `timeoutTagRegexp`) is
  the end of Use Givens from the terminal's pointer section: LoadProject runs it into
  Problems, and only runCheck (`givens check`, "Check passes" and "Check finds a problem") reads
  them. Same call as the watcher: the helper is the shared way in, the check is one story's.
- No clashes. Renames: `Check` -> `CheckCapabilities`, `Problem` -> `ConventionProblem`
  (beside model's HelperProblem), `Explain` -> `ExplainParseError`, `Scenarios` ->
  `FeatureScenarios` (app's `Scenarios` reads as the window's), `timeoutTag` ->
  `timeoutTagRegexp` and `parseErrorLine` -> `parseErrorLineRegexp` (beside
  `MessageTimeoutTag` and the loader's `parseErrorOf`). `ScenariosIn`, `ParseFeature`,
  `ParseFeatureText`, `CheckFeature`, `ErrNoFeature` and the limits kept theirs. The test
  `TestExplain` -> `TestExplainParseError`.
- timeoutTagRegexp's `//tiger:allow-global` moved to its doc comment, as helpers' regexps did:
  the longer name pushed the line past 100 columns.
- 11 acceptance suites, the acceptance root (TestBigFile, the spec test), cmd/givens and
  tools/pickles call `app.` now; tools/pickles now builds with all of app's imports, chromedp
  included. Gherkin files', the terminal pointer's opening comments and the loader's doc name
  ParseFeature and CheckCapabilities; big.go's header list is internal/slug, kit and starter.
- explain_test.go is internal/app/explain_parse_error_test.go.
- Surprise: a failed fold script's `rm` ran anyway (two shell lines, not `&&`); spec.go and
  explain.go were rebuilt from HEAD with the same renames, and the build matched.
- big.go 7,696 lines (+156), big_helpers.go 2,740 (+155), 36% of big.go.
