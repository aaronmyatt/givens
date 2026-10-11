# Search everything

## Goal
Make the 6 scenarios in `capabilities/search-everything/search-everything.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/search-everything/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/search (index of stories, scenarios, steps, scripts, helpers).
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
