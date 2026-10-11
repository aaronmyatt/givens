# Open a project

## Goal
Make the 14 scenarios in `capabilities/open-a-project/open-a-project.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/open-a-project/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/app (projects, windows, recents), file watching, the macOS shell.
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built

## Choices the steps already make
- `givens <folder>` runs `$GIVENS_OPEN_COMMAND -a Givens <absolute path>` when that variable is set,
  so tests can record what would open; otherwise `open -a Givens`.
- A missing folder prints "There's no folder at ~/…" with `~` for the home folder, and exits non-zero.
- Kit releases come from `App.ShipKits` in tests; the built-in kit otherwise.
- All 14 (2026-10-11), Go and Swift.
- Launch reopens the windows open at the last quit (app data `open-windows.txt`), else shows
  the welcome window: "Open folder…" (sets FolderPickerShown; under serve, a `folderPicker`
  effect) and, with any, "Recent" and the recent folders (`recents.txt`, newest first, 16).
  Opening a project closes the welcome window. A Dock drop launches first if nothing's open.
- A project opens on its first capability's story until something is selected.
- No capabilities: "No capabilities here yet" and "Create capabilities folder", which writes
  GIVENS.md (`internal/starter/GIVENS.md`, kept equal to ours up to "This repository" by a
  test), the kit, the starter with a setup script per scenario, and `runs/** merge=union`.
- "Install givens command" symlinks the givens binary (`Config.Command`, else the running
  program) into the chosen folder and says where.
- `givens <folder>` runs `open -a Givens <absolute folder>` (or `$GIVENS_OPEN_COMMAND`); a
  missing folder prints "There's no folder at ~/…" and exits 1.
- `scripts/run-app.sh [folder]` builds the app with givens beside it and runs it, for a look
  on screen. Known gap: whether a dev build opens a folder passed on the command line isn't
  checked; "Open folder…" works either way. Not eyeballed.
- Install folder (2026-10-11): "Install givens command" no longer assumes /usr/local/bin. It
  asks serve's `installFolders` (App.InstallFolders) for the PATH folders the developer can
  write to, then opens a folder picker at the first. The PATH comes from `$SHELL -l -i -c`
  (3 s cap), since a Mac app gets launchd's short PATH; a marker skips anything rc files print.
  ~/.local/bin, ~/bin, /opt/homebrew/bin and /usr/local/bin rank first, then the rest in PATH
  order, so nvm's and cargo's folders aren't offered first. Picking a folder that isn't on PATH
  still installs; Givens doesn't warn about it yet.