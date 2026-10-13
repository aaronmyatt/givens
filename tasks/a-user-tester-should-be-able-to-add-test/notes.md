Along side submitting a "Record run", the screen should present:

- "Add Attachment", or some sort of attachment capability to upload files related to taht run

## Goal
A tester attaches files (screenshots, logs) to a Drop in; "Record run" keeps them with the run,
and the scenario's run history lists them.

## Design
- Where: `runs/files/<capability>/<slug>[/<examples>]/<time>/<name>`, time as
  `2006-01-02T150405Z` (UTC). Grouped by scenario, sorted by time, found from the run line's
  key and time. The run line format doesn't change, so old lines and old readers still work.
- Copy, don't move: the tester's file stays where it was.
- Copy before the line is appended. A failed copy removes the half-made folder and appends
  nothing, so no line points at missing files.
- Limits: 32 files a run, 100 MB a file. Checked at attach time and again at copy time.
- Two files with the same name: the second replaces the first in the list.
- Merge: names under a run's own folder never collide across branches, so `merge=union`
  never touches them.
- Swift: "Add attachment…" opens a file picker (fileImporter); files can be dropped on the
  Then part. Chips list them, each with a remove button. The scenario page's run history
  shows a run's files; clicking one opens it.

## Rejected
- Listing files on the run line (`file:` fields): changes the line format for nothing the
  folder doesn't already say.
- An `@error` scenario for a too-big file: a 100 MB fixture is slow. Unit test the limit.

## As built (2026-10-12)
- Spec: "Attach proof to a run" in drop-into-a-scenario.feature. Go and Swift steps pass.
  GIVENS.md's layout lists `runs/files/<run key>/<time>/`.
- `internal/runs/files.go`: FilesDir, CheckFile, CopyFiles, Files. Unit tests cover the copy,
  the clean-up after a failed copy, and the 100 MB limit (a sparse file, so it's quick).
- `internal/app/dropin_files.go`: Session.Attach, Detach, Attachments. RecordRun copies the
  files, then appends the line. The Drop in snapshot has `attachments`; each history
  RunLine has `files` (full paths).
- Serve: `session.attach {path}`, `session.detach {name}` in needs.go.
- Mac: "Add attachment…" under the note opens a file picker; files dropped on the foot of
  the page are attached too. Each attached file shows with a remove button. The scenario page's runs
  list each file as a link that opens it in its default app.
- Not eyeballed: the layout of the attachment list and the drop target. To check: Drop in to
  any scenario, drag a screenshot onto the foot of the page, record the run, then click the file on
  the scenario page.
- Fixed after closing: the GIVENS.md layout line wasn't copied to internal/starter/GIVENS.md,
  so TestGuideMatchesGivens failed. Copied over while doing the Drop in order task.
- Known gap: two runs of one scenario in the same second share a folder.
