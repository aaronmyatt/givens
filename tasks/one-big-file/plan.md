# One big file: plan for the remaining stories

Written 2026-10-25. Goal: every story in `capabilities/` has a section in `internal/app/big.go`,
and `TestBigFile` fails on a story without one.

## Where it stands

- 2026-10-25: helpers moved out of big.go into `internal/app/big_helpers.go`, at the
  developer's ask. `TestBigFile` fails if that file holds a Feature, Rule or Scenario heading.

- 2 of 17 stories are in big.go: Copy a setup script, and Trust a project.
  - The Trust a project move isn't committed yet: big.go, dropin_page.go, dropin_run.go and
    the deleted trust.go are still in the working tree.
- big.go is 319 lines. The rest of `internal/app` is about 8,100 lines in 45 files.
- Views is next in `todo.txt`.
- No feature file uses `Rule:`, so no section needs a Rule heading yet.
- No CONSTRAINTS.md rule limits a file's length. The 70-line function limit still applies.

## How every slice is done

1. **Take one story per slice, and make one commit for it.** Run slices one after another.
   Big.go is a single file, so two slices at once would clash. The one exception is the
   experiment in slice 4.
2. **Cut seams first.** Sometimes one story's code sits inside another story's function, the
   way trust sat inside `startSession`. Pull it out into a named function first, and leave
   the call in the host's file. That host may not be in big.go yet.
3. **Move the code unchanged.** Change nothing but the file it's in. Delete a file once it's
   empty.
4. **Put the section in folder order.** Sections follow the capabilities folders,
   alphabetically, and `TestBigFile` checks that.
5. **Write the headings.**
   - A `// Feature: <name>` heading starts the section.
   - Its opening comment names the capability folder, and any of the story's code that lives
     elsewhere.
   - A `// Scenario:` heading goes over the function each scenario starts in. Stack the
     headings where scenarios share code.
6. **Decide where each piece goes.**
   - Code that only this story uses goes in its section.
   - Code that 2 or more stories use goes in `internal/app/big_helpers.go`, the helpers'
     own big file. Big.go holds only stories.
   - Types and constants, the data models, stay in their own files. List each of those
     files in the header.
7. **Pass the gate before the commit.**
   - The story's own suite, plus every suite that calls the moved code.
   - `go test ./acceptance -run BigFile -v`, which reports "N of 17".
   - `scripts/entry.sh "<one scenario>"`.
   - `scripts/check.sh`.
8. **Write it down.** Add a "<Story> moves in" entry to `notes.md` with:
   - section lines vs helper lines;
   - the seams cut;
   - the files deleted;
   - anything surprising.

## What stays out of big.go

- **Edges (the outer layers that call into `app`):** `internal/tui`, `cmd/givens` and
  `internal/termedit`.
  - They import `app`. Moving them in would make big.go depend on Bubble Tea, the TUI
    library.
  - Their stories get **pointer sections**: the Feature heading, a comment naming the edge
    files, and any `app` code that only that story uses.
- **The kit:** `internal/kit/givens.sh` is bash. It's already listed in the header.
- **Domain packages:** `spec`, `project`, `runs`, `runref`, `branch`, `fingerprint`, `setup`,
  `tags`, `slug`, `markdown`, `helpers` and `watch`.
  - They already sit apart, each with one job.
  - The header lists them once, as a group, with why.
- **Third-party integration:** `internal/browser`, which wraps chromedp.
- **Data models:** `dropin.go`, `edit.go` and `helpers.go` hold only types and constants. So
  do the type parts of `app.go` and `window.go`.
- **Core glue** (what every story runs on): `snapshot.go`, which is the view the TUI and steps
  read; the dispatcher (the code that turns a label into a call) in `offers.go`, meaning
  `Offers`, `Choose` and `allOffers`; plus `selection.go`, `input.go` and `prompt.go`.
  - Each story's own offer list moves into its section. `aboutOffers`, `todoOffers`,
    `runsOffers` and `readOnlyOffers` already work this way.
  - Choice made: keep the core apart (A) instead of putting it in big_helpers.go (B).
    - A: big.go reads as stories only, and the helper tripwire still means something.
    - B: closer to the essay, but about 1,000 lines of core would swamp the helpers file.
    - Revisit at slice 13 using the measured numbers.

## Slices, in order

The order puts small stories with few seams first, to settle the recipe. Next come the
stories that test the risks: shared code and clashing slices. Stories that sit inside other
stories' functions (guests) move before the story that holds them (the host), the way trust
did.

| # | Story | Scenarios | Moves in | Watch for |
| --- | --- | --- | --- | --- |
| 0 | (commit Trust a project) | — | what's in the working tree | the suites pass first |
| 1 | Views | 4 | views.go (223) | `tagViews` and `isViewTag` are tags' too, so they go to big_helpers.go |
| 2 | Tags | 11 | todo.go (102), the tag parts of views.go and search.go | "flag to do" is also used by Write stories (new scenario starts flagged) |
| 3 | Edge pointers: The kit, Use Givens from the terminal, Work in the terminal | 12 + 8 + 43 | nothing: install.go and `InstallCommand` are dead (only `_archive/` calls them; backlog line to delete) | sections with no functions; `--debug` says "no functions to stop in" |
| 4 | Update the kit **and** Search everything, as two worktrees at once | 5 + 6 | kitupdate.go (152); search.go, search_find.go, search_index.go (426) | the clash experiment: count merge conflicts; both touch the import block |
| 5 | Read a capability | 6 | history.go (120), copy_steps.go (36), story and scenario parts of selection.go | `status.go` is used by 4 files, so it goes to big_helpers.go |
| 6 | Gherkin files | 7 | examples.go (26), warnings from project.go, `moveToOwnCapability`, `RuleGroups` | the parser stays in `internal/spec` |
| 7 | Open a project | 9 | windows.go (217), create.go (63), branch.go (190), `Open` from app.go | loading in project.go (`findCapabilities`, `reload`, `findScenario`) goes to big_helpers.go; Keep runs off main uses `OpenBranch` too |
| 8 | Keep runs off main | 11 | runs_off_main.go (314) | run writes called from Drop in's record: cut a seam |
| 9 | Share setup between scenarios | 12 | shared.go (174), helper_edit.go (197), needs.go (94) | `RequiredCommands` is also used by Drop in and snapshot: the shared-code test |
| 10 | Write stories and scenarios, in 3 commits | 27 | a) authoring.go, editor.go, the editor parts of editor_more.go, preview.go; b) rename.go, delete.go; c) about_edit.go, `setupFile` and `deleteFile` | editor_more.go (402) mixes all three; `Rename` and `Delete` are called from manage.go and fakes.go |
| 11 | Mix shell, browser and hand steps | 19 | cards_run.go (179), run-block parts of cards.go | it lives inside Drop in's `startSession`: cut seams |
| 12 | Drop into a scenario, in 5 commits | 47 | a) run and timeouts: dropin_run.go, session.go, `givensCommand` from windows.go; b) record and attach: dropin_record.go, dropin_files.go; c) teardown and builds-on; d) cards, steps by hand and asks: cards.go; e) outlines and rows: rows.go, arguments.go | the biggest host, about 1,600 lines; by now its guests are gone |
| 13 | Finish | — | whatever is left that isn't core | see below |

Sizes are line counts of the files today.

## Slice 13: finish

- `TestBigFile` stops just counting missing stories: a Feature with no section fails.
- The header lists every file outside big.go and why.
- Measure, then decide core glue A or B:
  - big.go's lines vs big_helpers.go's lines;
  - big.go's total, expected to be about 6,500;
  - gopls and editor speed at that size.
- Write a closing entry in `notes.md`:
  - hops before and after for one scenario per story;
  - conflicts from slice 4;
  - every seam cut.
- Update `~/Development/bigolfile/capabilities`: remove `@todo` from what the work proved.
  Expected:
  - Helpers outgrowing sections is a tripwire;
  - Large files stay responsive;
  - Feedback without leaving the file.

## Stop and rethink if

- A slice needs a behaviour change to move. That breaks "moves unchanged".
- big_helpers.go outgrows big.go. The tripwire has tripped, so pick a group to move to an
  adjacent file.
- Slice 4 merges with more than a trivial import clash. Then parallel agents can't share
  big.go, and the essay needs a scenario about that.
- gopls or the hooks get slow on big.go. Every big.go edit runs tigerlint on `app` plus
  `TestBigFile`.

## Backlog lines, ready to add

```text
One big file: commit Trust a project's move ref:one-big-file
One big file: move Views into big.go ref:one-big-file
One big file: move Tags into big.go ref:one-big-file
One big file: pointer sections for The kit, Use Givens from the terminal, Work in the terminal ref:one-big-file
One big file: move Update the kit and Search everything in parallel worktrees, count conflicts ref:one-big-file
One big file: move Read a capability into big.go ref:one-big-file
One big file: move Gherkin files into big.go ref:one-big-file
One big file: move Open a project into big.go ref:one-big-file
One big file: move Keep runs off main into big.go ref:one-big-file
One big file: move Share setup between scenarios into big.go ref:one-big-file
One big file: move Write stories and scenarios into big.go, in 3 commits ref:one-big-file
One big file: move Mix shell, browser and hand steps into big.go ref:one-big-file
One big file: move Drop into a scenario into big.go, in 5 commits ref:one-big-file
One big file: finish: require every story, decide core glue, measure ref:one-big-file
```
