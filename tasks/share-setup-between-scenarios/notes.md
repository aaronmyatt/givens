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
- All 12 (2026-10-11), in Go and Swift. Built on Drop in's core, done here because every
  scenario but the helper-editing ones drops in or reads a status:
  - `internal/helpers`: functions in `_shared/*.sh` by pattern (`name() {` to a lone `}`),
    `# @helper` docs, `Visible` (capability wins), `Called` (nested, iterative), `need` and
    `from` lines.
  - `internal/fingerprint`: parts are steps, setup, each `from` setup and each called helper;
    `Changed` names the first that differs. The kit isn't a part.
  - `internal/app`: Session (teardown then setup, Still running on the app clock, Stop, Try
    again, Edit script, marks, Record run with fingerprint and author), status ("Never run",
    "Verified today by Sam", "Unverified: X changed since the last run", "Last run failed"),
    `RequiredCommands`, the Shared list (`Helpers`), helper editing with `bash -n` marks, and
    "X is defined in <file>, which only <Capability>'s scripts can call".
  - The kit gained `from` (teardown then setup, loop check through `GIVENS_FROM_CHAIN`) and a
    warning when two files at one level define a helper, the one in effect named first.
- Fixed on the way: windows looked at files only every 500 ms, so a step that edited then read
  saw old steps. Every read looks now; a snapshot looks once. And the snapshot dropped its
  messages and labels, so prompts were never shown.
- Swift World never trusted the sample shop, unlike Go's; it now trusts through the existing
  `test.markTrusted` seam unless `untrusted` is set, which Trust a project's suite and the
  markdown-setup step set, as in Go.
- UI: labels ("Requires bb") and helper syntax errors are drawn. Known gap: the Shared list
  isn't in the sidebar yet; it's the "Shared" view, which Views builds. Not eyeballed.