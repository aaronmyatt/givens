# Use Givens from the terminal

## Goal
Make the 8 scenarios in `capabilities/use-givens-from-the-terminal/use-givens-from-the-terminal.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/use-givens-from-the-terminal/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: cmd/givens (list, run, check), internal/spec.
- No UI shell wiring: no window is involved. `givens serve` and `givens <folder>` belong to
  ui-shell and open-a-project.
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
