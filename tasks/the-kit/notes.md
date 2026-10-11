# The kit

## Goal
Make the 12 scenarios in `capabilities/the-kit/the-kit.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/the-kit/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/kit/givens.sh (bash 3.2; the skeleton comes from Foundation) +
  bats tests in internal/kit/.
- No UI shell wiring: the kit runs in scripts. Its output shows through Drop into a scenario.
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
