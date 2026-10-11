# Drop into a scenario

## Goal
Make the 44 scenarios in `capabilities/drop-into-a-scenario/drop-into-a-scenario.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/drop-into-a-scenario/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/setup (run scripts with limits), internal/runs (run log), internal/app (checklist state).
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built

## Choices the steps already make
- Run lines: one tab-separated line per run in `capabilities/runs/YYYY-MM-DD.log`, holding the run
  key, `pass`, the git `user.name`, row values like `card=expired`, and the note.
- Drop in timers read `Config.Clock`; the steps move a fake clock instead of waiting.
- Scenarios start trusted through `App.MarkTrusted`, except the markdown-only setup scenario.
