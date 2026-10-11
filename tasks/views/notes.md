# Views

## Goal
Make the 4 scenarios in `capabilities/views/views.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/views/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/app (error states, unverified, shared, tag views).
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
- All 4 (2026-10-11), Go and Swift. Views' sidebar, selection and `ListedScenarios` came
  with Tags (tasks/tags/notes.md); this adds the Shared view (`ViewHelpers`, each helper's
  signature, description, file and caller count as labels) and empty views ("No error states
  yet" with "New scenario").
- Fixed a Go step: "the other 7 scenarios are listed" counted names it had just added to
  `s.named` through listedWith, so 7 + 1 came out as 7 + 8. It now counts against the names
  earlier steps gave. The Swift step didn't have the bug.