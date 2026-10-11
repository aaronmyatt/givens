# Drop into a scenario

## Goal
Make the 43 scenarios in `capabilities/drop-into-a-scenario/drop-into-a-scenario.feature` pass: each loses its `@wip` tag,
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
- All 43 (2026-10-11), Go and Swift. The core (session, status, fingerprints) came in with
  Share setup; see tasks/share-setup-between-scenarios/notes.md.
- Teardown: "→ teardown" heads its output; a failing one shows "Teardown failed (exit N)" and
  offers "Edit teardown" and "Run setup anyway". "Reset state" and "Try again" start a new
  session for the same scenario and row. "Add teardown" writes one that sources the kit.
- Before anything runs: missing commands, a from line naming nothing, an outline named
  without a row ("…is an outline: pick a row, like card=expired"), and loops ("These setups
  build on each other in a loop: a → b → a").
- `GIVENS_ARG_<n>`: each doc string and data table (CSV, header first) in a temp folder the
  next Drop in removes.
- Cards: under `GIVENS_UI=1` the kit prints `\x1egivens-card<TAB>kind<TAB>where<TAB>link<TAB>lines`
  then the text, and reads the answer on stdin ("done", "failed", or the value).
  The session strips those lines from output, waits, and pauses the Still running clock.
  Markdown setups become cards after the scripts, no trust needed when there's no script.
  The waiting card's buttons are also the window's own.
- Outline rows: `DropInRows`, one live row, others greyed with "Switch to this row" (asks
  "Leave the expired row? Its marks aren't recorded" when marked). Runs key on the Examples
  slug and keep the row's values; the outline sums up as "1 of 2 rows verified"; Unverified
  lists rows as "<outline>: <value> — <status>". Unnamed blocks: warning, no Record run.
- Listed scenarios now show as one line "<name> — <status>" in both visible lists.
- Fixed a Swift step: "Record run is disabled until named" named the blocks first (through
  rowAllPassed), unlike Go. It now drops in to the row as Go does.
- UI: output, card text, an answer field while waiting, and Done/Pass/Fail per checklist
  item. Known gap: rows aren't drawn (they need a dropInRows call per refresh). Not eyeballed.
- Removed `by_hand --qr` and "A step on another device" (2026-10-11): never in the original
  spec. The card line lost its qr field.
- Fixed a Swift flake (1 run in 2 under the full suite): ServeClient parsed each chunk of
  serve's output in its own Task, so an older pushed snapshot could land after a newer reply
  and the window lost its session. Chunks now go through one AsyncStream to one reader, in
  order; a dead serve fails pending calls after the reader drains. 5 full runs clean since.