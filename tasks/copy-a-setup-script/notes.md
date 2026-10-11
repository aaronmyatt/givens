# Copy a setup script

## Goal
Make the 5 scenarios in `capabilities/copy-a-setup-script/copy-a-setup-script.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/copy-a-setup-script/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/app (copy command text), clipboard behind an interface.
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
- All 5 (2026-10-11), Go and Swift.
- "Copy" on a scenario: `bash <path from the git repo's root>`, found by walking up to `.git`
  (a folder or a worktree's file), else the folder holding capabilities. A setup done only by
  hand copies its markdown. ⇧⌘C in search copies the first Scenarios result's, leaving the
  selection alone.
- The kit now `cd`s to `git rev-parse --show-toplevel` (or the folder holding capabilities)
  after loading helpers, so a pasted command works from any folder in the project.
- UI: "Copy" was already a scenario-page button; it now does something. Not eyeballed.