# Open a project

## Goal
Make the 14 scenarios in `capabilities/open-a-project/open-a-project.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/open-a-project/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/app (projects, windows, recents), file watching, the macOS shell.
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built

## Choices the steps already make
- `givens <folder>` runs `$GIVENS_OPEN_COMMAND -a Givens <absolute path>` when that variable is set,
  so tests can record what would open; otherwise `open -a Givens`.
- A missing folder prints "There's no folder at ~/…" with `~` for the home folder, and exits non-zero.
- Kit releases come from `App.ShipKits` in tests; the built-in kit otherwise.
