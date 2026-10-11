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
