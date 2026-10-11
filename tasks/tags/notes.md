# Tags

## Goal
Make the 8 scenarios in `capabilities/tags/tags.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/tags/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/tags (Cucumber tag expressions), internal/search.
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
