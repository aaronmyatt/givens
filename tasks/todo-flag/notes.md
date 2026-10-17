# To do flag

Three root lines in todo.txt (2026-10-12) ask for one thing: flag scenarios that need work,
show the flag in red, toggle it, collect flagged ones in a view, and flag new scenarios.

## Goal
- `@todo` on a scenario (or its Rule or Feature) flags it: a red flag on its row, its page and
  under Tags in the sidebar.
- The scenario page offers "Flag as to do", or "Remove to do flag" when it's flagged.
- "New scenario" writes the block tagged @todo (after @error, when it's an error state).
- Flagged scenarios are listed under "todo" in Tags, as any tag is a view.

## Design
- A Gherkin tag, not a Givens-only file. The user's line says "(tag)", GIVENS.md already says
  "any other tag is a view", and a runner can skip flagged ones with `not @todo`.
  Ref: https://cucumber.io/docs/cucumber/api/#tags
- Name: `@todo`, from the user's own word. Rejected `@wip`: Givens's own repo uses `@wip` to
  mean "not built yet" for godog, but in a user's project @wip has no meaning to Givens, and
  the user asked for TODO.
- Still a tag view ("todo" under Tags), not a new fixed view. Rejected a fixed "To do" view:
  it would list the same scenarios twice, under Views and under Tags.
- Core: `project.TagTodo`, `project.Scenario.Todo` (like Error), and `OwnTags` (the tags on
  the lines above its Scenario: line). `app.Scenario.Todo` goes to the UI as "todo".
- Toggle edits only the scenario's own tag lines: add `@todo` to the last tag line above
  Scenario:, or put a new tag line there at the same indent; remove drops the word, and the
  line if nothing is left. One line changes in git diff.
- Inherited @todo (from Feature or Rule): flagged, but no toggle offer, as removing it would
  unflag every scenario under that line.
- Mac: red `flag.fill` on the row, a red "To do" badge on the page (and @todo left out of the
  tag badges), a flag button in the page header, a flag icon for "todo" under Tags.

## Spec
- tags: "A scenario flagged to do", "Flag a scenario to do", "Remove a to do flag".
- write-stories-and-scenarios: "A new scenario starts flagged to do".

## As built (2026-10-17)
- Spec: 3 scenarios in tags, 1 in write-stories-and-scenarios. Steps in Go and Swift. The red
  flag step is shared: world/todo.go and World/CommonSteps.swift, as both capabilities use it.
- Core: `project.TagTodo`, `Scenario.Todo` and `OwnTags`. internal/app/todo.go holds the
  offers (`todoOffers`), the toggle (`flagTodo`) and the line edit (`withTodo`, unit-tested).
  `NewScenario` writes `@todo`, or `@error @todo` for an error state.
- "Nothing else changes" is checked against the sample shop as checked in: the file less the
  scenario's tag lines must match, and its other tags must be the same.
- Mac: `TodoFlag` (red flag.fill) after the name in every scenario list, and beside a red
  "To do" badge on the page; @todo left out of the tag badges. A flag / flag.slash button in
  the page header, drawn from the offers. "todo" under Tags has a flag icon.
- GIVENS.md (both copies) says what @todo means.
- Branch windows don't offer the toggle: readOnlyOffers is an allowlist.
- Known gap: a @todo on the Feature or Rule line flags the scenario but offers no toggle, by
  design; the flag comes off by editing that line.
- Not eyeballed. To check: open the sample shop, select "Pay with saved card", press the flag
  button in the page header. Expect a red flag beside its name in the Checkout list, a red
  "To do" badge on the page, and "todo" with a flag icon under Tags. Press it again: all gone,
  and `git diff` shows only the tag line.
