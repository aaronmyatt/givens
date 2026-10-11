# Gherkin files

## Goal
Make the 7 scenarios in `capabilities/gherkin-files/gherkin-files.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/gherkin-files/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/spec (parse, check), internal/project (load a folder).
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
