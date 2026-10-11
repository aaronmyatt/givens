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
