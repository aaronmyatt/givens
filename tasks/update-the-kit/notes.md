# Update the kit

## Goal
Make the 4 scenarios in `capabilities/update-the-kit/update-the-kit.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/update-the-kit/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/kit (versions, compare, replace).
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
- All 4 (2026-10-11), Go and Swift.
- Givens ships the built-in kit (`internal/kit`), or the releases `ShipKits` sets. A project
  window compares `_shared/givens.sh`'s version line: older → "Kit update available (1 → 2)"
  and "Update kit"; newer → "This project uses a newer kit. Update Givens to edit its
  scripts." (reading, search and Copy work as ever).
- "Update kit" replaces the kit when it equals a shipped release of its version; otherwise
  it asks "givens.sh has changes of its own" with "Replace anyway" and "Compare" (opens the
  project's kit in the editor).
- Known gap: with a newer kit, editing scripts isn't blocked yet; the message only says so.