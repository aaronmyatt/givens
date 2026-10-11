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
- All 8 (2026-10-11), Go only (no window, so no Swift suite).
- Decided: `list`, `run` and `check` live in the Go `givens` binary, not plain bash as the
  feature's description says. Options: (A) bash for list/run, Go for check; (B) Go for all.
  Took B: listing needs the Gherkin parser and the status needs runs and fingerprints, which
  bash would have to copy. The "small script that runs `open -a Givens`" for opening a
  folder is still Open a project's; it can call this binary for the rest.
- `givens list [--json] [--tags <expr>]`: path, kind, status from `app.StatusOf`.
  `internal/tags` parses Cucumber tag expressions (shunting yard, no recursion).
- `givens run <key>`: the script with the terminal as stdio, then any `scripts/<slug>.md`
  steps one at a time (`internal/markdown`), plain text, code blocks bare, Enter or f.
- `givens check`: "N scenarios, M problems", each problem as
  "checkout/checkout.feature line 17: expected …" through `spec.Explain`.
- Views' selection and `ListedScenarios` (Error states, Unverified, tag views) landed here, for
  "List by tag". The sidebar doesn't list views yet; that's Views'.