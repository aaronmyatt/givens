# Read a capability

## Goal
Make the 4 scenarios in `capabilities/read-a-capability/read-a-capability.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/read-a-capability/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/app (selection, about.md, setup lookup).
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
- All 4 (2026-10-11), Go and Swift.
- Steps show with keywords lined up ("Given …", " When …"): `linedUp` in Go and
  `WindowModel.linedUp` in Swift pad visible lines; `Scenario.Steps` stays as written.
- about.md: an image that isn't on disk becomes "Image not found: <path>", so the rest reads.
- A scenario with no setup files shows "No setup yet" and offers "Create script" (kit source
  line) and "Write setup steps" (an empty "1." list), each opening the new file.
- UI: the scenario view draws lined-up steps and the setup script under them. Not eyeballed.