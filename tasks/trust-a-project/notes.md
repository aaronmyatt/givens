# Trust a project

## Goal
Make the 4 scenarios in `capabilities/trust-a-project/trust-a-project.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/trust-a-project/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/trust (folder paths in app data, outside the project).
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
- All 4 (2026-10-11), Go and Swift.
- The prompt is "Run scripts from this project? <project folder>" with "Trust" and "Don't
  trust" (prompts can now name both buttons). Trust and "Don't trust" are folder lists in app
  data (trusted.txt, untrusted.txt), by capabilities path, so a move or re-clone asks again.
- "Don't trust" opens the Drop in page with nothing run: "Scripts are off for this project",
  Copy still copies the setup command, and "Trust this project" trusts and runs it.
- The Drop in page now shows its scenario, steps and setup script; both visible lists include
  the setup script's text after the steps.
- `App.Quit` writes the open project windows to app data and closes them; reopening them at
  launch is Open a project's.
- `Session.Wait` returns at once while no script runs (a question is up), instead of waiting
  out its limit.