# Read capabilities from another branch

## Goal
See another branch's capabilities (a spec written on a feature branch, say) without
checking that branch out.

## Design
- Copy, don't check out: `git archive <branch> -- <capabilities path>` streams the folder as
  a tar, which Givens unpacks into its own data folder:
  `<app data>/branches/<hash of repo root>/<branch slug>/`. The working tree, the index and
  .git are never touched. Ref: https://git-scm.com/docs/git-archive
- Reopening a branch unpacks it again, so it's always the branch's latest commit.
- The branch's window is read only: its setup scripts belong to code that isn't checked out,
  and edits would land in a throwaway copy. Offers are cut to "Copy"; Drop in is refused.
- Title: `<capabilities path> @ <branch>`.
- Branch windows aren't added to Recents and aren't reopened after quitting: the copy is
  throwaway.
- Branches: local and remote, from `git for-each-ref`, `*/HEAD` left out.
- Limits: 20 s per git call, 256 MB and 100 000 entries per archive. Only folders and
  regular files are unpacked; any path that would land outside the target is refused.
- Branch names starting with `-` are refused, so a name can't pass as a git option.

## Rejected
- `git worktree add`: a full checkout of the whole repo, and it writes to .git.
- A branch picker that swaps the open window's folder in place: then one window can be
  read only or not, which every offer would have to check twice.
- Asking the human: CLAUDE.md says pick and say why. Chosen: read-only copy in a new window.

## Known gaps (planned)
- Needs methods that write (window.edit, newStory, rename, delete) aren't gated in a branch
  window. The UI doesn't offer them there; a call would only change the throwaway copy.

## As built (2026-10-12)
- `internal/branch`: List (`git for-each-ref`), Extract (`git rev-parse --verify`, then
  `git archive` streamed into a tar reader). Unit-tested against a real repo, including
  option-like names and a `../` entry.
- `internal/app/branch.go`: Window.Branches, Window.OpenBranch, a read-only cut of Offers
  (only Copy and Cancel stay), Drop in refused in startSession, the "Read only: …" message,
  title `<path> @ <branch>`. Branch windows are left out of open-windows.txt and Recents.
  A failed OpenBranch shows "No branch called X" in the window; Select clears it.
- WindowSnapshot has `branch`. Serve: `window.branches`, `window.openBranch` in needs.go.
- Mac: the project toolbar has a Branch menu. In a branch's window it becomes "Back to the
  working tree", which closes that window. The Mac app only shows the front window, so
  without this button you couldn't get back.
- Spec: "Read capabilities from another branch" and "Open a branch that doesn't exist".
- Not eyeballed. To check: in a git project, pick a branch from the Branch menu, look for
  the "Read only" banner and no Drop in button, then click "Back to the working tree".
- Known gaps: the Needs methods that write (edit, newStory, rename, delete) aren't gated in a
  branch's window, but the UI doesn't offer them there. Copies under
  `<app data>/branches/` are never cleaned up.

## Follow-up: clean-up and refused writes (2026-10-17)
- Copies: closing a branch's window removes its copy, and its origin's folder once empty.
  `Launch` removes all of `<app data>/branches/`, since branch windows aren't reopened after
  quitting. Only serve calls Launch, so a CLI run never clears a copy the app is showing.
- Writes: `Window.writable` returns `ErrReadOnly` in a branch's window. Edit, NewStory,
  NewScenario, Rename, Delete, NewHelper and SaveHelper check it first. The editor's own
  calls need an open editor, which only those make, so they're covered too.
- No scenario: nothing here is on screen. Unit tests in internal/app/branch_test.go, against
  a real git repo.
- Rejected: clearing copies in `New`, which the CLI calls too, so a `givens` run could delete
  the folder a branch window shows.
