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
