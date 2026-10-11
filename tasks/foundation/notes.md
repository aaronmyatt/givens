# Foundation

## Goal
The core every capability builds on, so capability work is mostly step definitions and small
packages.

## Design
- Step definitions drive `internal/app`, never the UI. The UI shell is a thin client of the same
  calls, so a scenario passing in `acceptance/` passes for the app too.
- Each scenario gets a fresh copy of `testdata/sample-shop` in a temp folder. Nothing touches the
  real repo.
- Limits are constants with units last (`setupDurationSecondsMax`), per CONSTRAINTS.md.
- `internal/project`: wraps `internal/spec`; capabilities, scenarios, tags, setup files.
- `internal/runs`: run key from a scenario (plus Examples slug). One line per run,
  tab-separated: time, run key, result, author (git `user.name`), row values, note.
  Append-only, so `runs/** merge=union` merges cleanly.
- `internal/setup`: runs `scripts/<slug>.sh` with the `GIVENS_*` variables, a time limit,
  line-by-line output, and a stop that kills the whole process group.
- `internal/app`: commands in, events out; no UI types.

- `internal/kit/givens.sh`: the kit's one source. `acceptance/world` copies it into each sample
  shop; `go:embed` makes it the built-in kit that new projects and kit updates get. Its first
  lines carry the version update-the-kit compares.

## Decided
- File watching: polling in Go (`internal/watch`), stdlib only, a bounded interval under 2
  seconds. Keeps the allowlist unchanged and works the same for the CLI, steps and the app.
  FSEvents through the UI shell was the other option; it would leave the Go steps unable to see
  changes.
- The kit skeleton moved into Foundation: the sample shop's scripts source it, so it has to
  exist before them. The-kit's scenarios still fill in its behaviour.
