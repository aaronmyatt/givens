# The kit

## Goal
Make the 12 scenarios in `capabilities/the-kit/the-kit.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/the-kit/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/kit/givens.sh (bash 3.2; the skeleton comes from Foundation) +
  bats tests in internal/kit/.
- No UI shell wiring: the kit runs in scripts. Its output shows through Drop into a scenario.
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built
- All 12 (2026-10-11): `internal/kit/givens.sh` gained `need`, `first_you`/`when_you`/
  `then_check` with one "Your turn" heading and `GIVENS_QUIET`, `when_mode`/`when_command`
  (`--when` read from the script's own arguments), and terminal `by_hand` and `ask`, both
  reading `/dev/tty` so a missing terminal fails with "needs someone at the keyboard".
  - Bash 3.2: `${1+"$@"}` stands in for `"$@"` under `set -u`; `printf -v` sets ask's variable.
  - One bats test per scenario in `internal/kit/kit.bats`. Its terminal helper keeps stdin
    open: at a pipe's end macOS `script(1)` types ^D, which a waiting read takes as no answer.
  - Not yet: `GIVENS_UI=1` (by_hand and ask as Drop in cards) and `from`; both are Drop into
    a scenario's.