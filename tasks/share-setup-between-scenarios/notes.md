# Share setup between scenarios

## Goal
Make the 12 scenarios in `capabilities/share-setup-between-scenarios/share-setup-between-scenarios.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/share-setup-between-scenarios/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/helpers (find `# @helper` functions, call graph), internal/fingerprint.
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
