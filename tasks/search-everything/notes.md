# Search everything

## Goal
Make the 6 scenarios in `capabilities/search-everything/search-everything.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/search-everything/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/search (index of stories, scenarios, steps, scripts, helpers).
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
- All 6 (2026-10-11), Go and Swift, built with Tags (tasks/tags/notes.md).
- Results: Stories (first matching story line), Scenarios (capability as second line), Steps
  (each text once, with uses; keyword from its first use), Scripts (named for the words first,
  then mentions, first matching line as second line), Helpers (description as second line).
  Matches list every occurrence in title and second line, as written there.
- Selecting a step result lists the scenarios using it; selecting a helper shows its
  signature, description, file and callers.
- Not built yet: ↵, ⇧⌘C, ⌘↵ and ⌘E on a result (no scenario here needs them).