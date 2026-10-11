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

## As built
- internal/project (2026-10-11): `Load(root)` reads every `<slug>/<slug>.feature` into
  capabilities and scenarios: story lines, tags (scenario, then Rule, then Feature), Background
  steps (Feature's then Rule's), Examples, and the `scripts/<slug>.{sh,md,teardown.sh}` paths.
  `Find("capability/slug")` looks a scenario up by its run key.
  - A file that isn't valid Gherkin still loads, with `ParseError` set and spec's problems in
    `Project.Problems`, so the window can show it.
  - A `@timeout` tag that can't be read gives `StillRunning == 0`.
  - Parsing moved into `spec.ParseFeature`, shared with `spec.ScenariosIn`.
- internal/runs (2026-10-11): `Append`, `Read`, `Parse`, `Latest`, `Key`, `Author`.
  - Line: time (RFC 3339), key, result, author, `name=value` per row value, `fp:name=hash` per
    fingerprint part, note last. Each row value is its own field, as the steps expect.
  - The file is named by the run's own time, so the fake clock picks the day.
  - A line that can't be read (say, a conflict marker) is skipped, not fatal.
  - `Author` runs `git config user.name` with a 5 s limit; "" when there's no name.
- internal/setup (2026-10-11): `Start(Request)` runs `bash <script>` in its own process group
  with extra `GIVENS_*` variables; `Output`, `Wait(limit)`, `Answer` (one stdin line), `Stop`.
  - Stop: SIGTERM to the group, SIGKILL after 2 s; output so far is kept.
  - `DurationMax` (1 h) stops a setup nobody stopped. "Still running" is app's job, on its clock.
  - A child left in the background can't hold Drop in open: 1 s after the script exits, the
    group is killed and output closed.
  - Output keeps 10 000 lines of up to 64 KiB; later lines are counted in `Dropped`.
- internal/app facade (2026-10-11): `Open` resolves a project folder or a capabilities folder,
  loads it through internal/project, and brings an already-open project to the front;
  `Windows`/`Front` keep front-first order, bounded by `WindowsMax`. `Window.Title`,
  `Capability` and `Scenario` read the loaded project. `ErrNotFound` for unknown names.
  - A folder with no capabilities opens a window with no project; what it shows is Open a
    project's to build.
  - Not safe for concurrent use; serve and the steps call it one command at a time.
  - `Scenario.Status` is still empty: Drop in fills it from runs and fingerprints.
- internal/kit (2026-10-11): `givens.sh` starts `# givens kit 1`; strict mode, `GIVENS_ROOT`,
  `GIVENS_SCRIPT`, `ok`/`warn`/`fail`, colour only on a terminal, and helper loading (shared,
  then the calling capability's `_shared`). `kit.Script` embeds it; `kit.Version` reads the
  first line.
  - Checked under /bin/bash 3.2: the sample shop's scripts run until the first function the
    skeleton doesn't have yet (`when_you`, `from`, `need`, …). The-kit's scenarios add those.
- check.sh kit (2026-10-11): `bats internal/kit` runs `internal/kit/kit.bats`, between shell
  and test; a row in CONSTRAINTS.md says so. The tests run scripts with /bin/bash, so they hold
  the kit to bash 3.2. Four tests cover the skeleton.
- App.MarkTrusted (2026-10-11): resolves the folder like Open, then adds it to
  `<AppDataDir>/trusted.txt`, one absolute capabilities path per line, at most `TrustedMax`.
  `App.Trusted(folder)` reads it. "Don't trust" isn't stored yet; Trust a project adds it.
- testdata/sample-shop (2026-10-11): its 7 helpers and 9 setup scripts were already in the
  scaffold. With the kit skeleton they run until `when_you`, `need` or `from`, which The kit
  adds.
- internal/watch (2026-10-11): `New(root, now)` and `Changed(now)` compare path, size and
  modification time of every file under root, at most once per `Interval` (500 ms), skipping
  hidden folders such as .git. No goroutine: whoever reads a window polls it. Wiring it into
  windows is Open a project's "Files written by other tools appear".
- check.sh shell (2026-10-11): also shellchecks `internal/kit/givens.sh`, `internal/kit/*.bats`
  and every `.sh` under internal/starter and testdata/sample-shop. The after-edit hook still
  skips testdata, so a sample-shop script is only checked by check.sh.