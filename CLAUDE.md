# Givens

A terminal app (a TUI) and CLI that reads a project's `capabilities/` folder of Gherkin
scenarios, sets up each scenario's starting state, and walks a developer through it.

## Where things are

- `capabilities/`: Givens's own spec, one `.feature` file per capability. Read
  `capabilities/GIVENS.md` first.
- `acceptance/<capability>/`: one godog suite per capability, with its step definitions.
  Shared steps, fixtures and fakes are in `acceptance/world/`. `@wip` scenarios are skipped;
  `GIVENS_TAGS=@wip go test ./acceptance/...` runs them, and they fail until built.
- `internal/app/`: the core the TUI, the CLI and the steps all drive; mostly stubs that
  return `ErrNotImplemented` until built. `internal/tui/`: the TUI. `cmd/givens/`: the CLI.
  `tools/tigerlint/`: the linter.
- `testdata/sample-shop/`: the pretend project most scenarios start from.
- `todo.txt` and `tasks/<slug>/`: the backlog, worked with the `txtodo-backlog` skill.

## Working rules

- Follow `CONSTRAINTS.md`. Hooks enforce it on every edit and when you stop; don't work around
  a hook, fix the code.
- Take the next task in file order from `todo.txt` (see the `txtodo-backlog` skill). Claim it,
  read its `notes.md`, and close it with a one-line summary.
- A scenario is done when its `@wip` tag is gone, its steps in `acceptance/<capability>/`
  pass against a real implementation (not a stub), and `scripts/check.sh` passes.
- Don't weaken a step to make it pass. If a step is wrong, fix it and say why in the task's
  notes.
- Step definitions drive Givens through `internal/` packages, never through the UI.
- Never write a debt marker in code. Add a line to `todo.txt` instead.
- Run `scripts/check.sh` before saying work is finished.
