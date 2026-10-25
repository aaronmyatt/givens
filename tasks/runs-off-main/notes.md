# Runs off the main branch

From the remote txtodo list (2026-10-17): "Explore using git lfs(?) to store the runs/ off
the main branch, but still carried across devices".

## Goal
- `runs/` stops showing up in main's commits, diffs and pull requests.
- Every device (and teammate) that uses Givens still sees every run, files and notes too.
- Someone who doesn't use Givens doesn't download runs when they clone.

## What's in runs/ today
- `runs/YYYY-MM-DD.log`: one line per run, about 100 to 130 bytes. 50 runs a day is about
  6.5 KB a day, or 2.4 MB a year. Small.
- `runs/files/<run key>/<time>/`: attachments, up to 32 a run and 100 MiB each, plus
  `note.md`. 10 screenshots a day at 0.5 to 2 MB is 5 to 20 MB a day, or 2 to 7 GB a year.
  This is the part that grows.
- `capabilities/.gitattributes` has `runs/** merge=union`, so two branches' log lines merge
  without a conflict. Ref: https://git-scm.com/docs/gitattributes#_built_in_merge_drivers
- The code that touches runs is small: `runs.Read` (status, history, rename),
  `RecordRun` (Append, CopyFiles, WriteNote), and history (ReadNote, FilesDir).

## Assessment: Git LFS
- LFS doesn't take anything off main. It swaps a file's content for a small pointer file,
  and the pointer is still committed on whatever branch you commit to. Run commits would
  still land on main. Ref: https://git-lfs.com
- It does help the size half: attachments leave the git history, so clones stay small.
- It doesn't fit the logs. They're small text and need `merge=union`; a union merge of two
  LFS pointer files gives a broken pointer.
- Costs:
  - Every device and CI box needs `git-lfs` installed and `git lfs install` run. Apple's git
    doesn't include it.
  - Hosts meter LFS storage and bandwidth separately from the repo.
  - A checkout without the LFS objects (offline, or no git-lfs) leaves pointer text where the
    screenshot should be.
- Verdict: LFS is the wrong main tool for "off main". At most it's an add-on for attachments,
  later, if size hurts.

## Options for "off main, still synced"
- A. A ref of Givens's own, `refs/givens/runs`, outside `refs/heads`. Not a branch, so it
  isn't in branch lists or PRs, and a plain `git clone` doesn't fetch it. Givens fetches and
  pushes it by name. git-bug keeps its issues in its own refs the same way and pushes them to
  ordinary remotes. Ref: https://git-scm.com/book/en/v2/Git-Internals-Git-References
  Ref: https://github.com/git-bug/git-bug
- B. An orphan branch, `givens-runs`, like `gh-pages`. Works with every host and git GUI.
  But it shows in branch lists, a plain clone fetches it, and branch protection rules can
  get in the way.
- Chosen: A, as it meets all three goals. Fall back to B for a host that refuses refs outside
  `refs/heads`. Waiting on a yes (`@human` in todo.txt), as it changes where runs live.

## How Givens.app would manage it
- Write, without touching the working tree, the index or the checkout. Git's plumbing commands
  build a commit straight from files:
  1. `git hash-object -w` the new day's log (old lines plus the new one), each attachment and
     `note.md`. Ref: https://git-scm.com/docs/git-hash-object
  2. A throwaway index in Givens's data folder (`GIT_INDEX_FILE`): `git read-tree` the ref's
     tip, `git update-index --add --cacheinfo` each new file, `git write-tree`.
     Ref: https://git-scm.com/docs/git-update-index
  3. `git commit-tree -p <tip>`, then `git update-ref refs/givens/runs <new> <old>`. Passing
     the old tip makes it fail if another write got there first, and Givens tries again.
     Ref: https://git-scm.com/docs/git-commit-tree
     Ref: https://git-scm.com/docs/git-update-ref
- Sync, when a project opens, after each Record run, and every few minutes:
  1. `git fetch origin +refs/givens/runs:refs/givens/runs-origin`.
     Ref: https://git-scm.com/docs/git-fetch#_configured_remote_tracking_branches
  2. Merge it in Givens's own code, not with git merge. Log lines are a set, so a day's file
     is the union of both sides' lines; files folders never share a name across runs. No
     conflict is possible. Commit with both tips as parents.
  3. `git push origin refs/givens/runs`. A rejected push means someone else pushed: fetch,
     merge, push again (bounded tries).
- Read: runs come from the ref (`git ls-tree` and `git cat-file`, or an unpacked copy in
  Givens's data folder, like branch windows), plus any `runs/` still in the working tree, so
  old runs keep counting.
- One seam in the core: a runs store with Read, Record and Files, backed by `runs/` or by
  the ref. Status, history and RecordRun go through it. The run line format doesn't change.
- Turning it on, per project: a "Keep runs off main" action. It copies `runs/` into the ref
  and pushes. Then it tells the developer to `git rm -r capabilities/runs` in a commit of
  their own: Givens doesn't commit to their branch.
- A new device: opening a project whose remote has the ref fetches it, so the runs show up.
- Offline: records commit to the local ref, and push when the remote can be reached. The
  window says "3 runs not shared yet".
- Pushing sends data to a remote, so the first push in a project asks first.
- Git runs as the developer, so their SSH keys and credential helper just work. A push that
  fails (auth, no network) is shown as a message, never a lost run.
- Limits: 20 s per git call (as `internal/branch` does), a bounded number of push retries,
  and the existing 32 files and 100 MiB per file. GitHub refuses files over 100 MiB, so the
  file limit already fits. Ref: https://docs.github.com/en/repositories/working-with-files/managing-large-files/about-large-files-on-github

## Size later
- The ref's history still grows with attachments: 2 to 7 GB a year at the rates above.
- Two ways to deal with it, if it hurts:
  - Keep attachments for N days: Givens rewrites the ref without older files folders. It owns
    the ref, so rewriting it breaks nobody's branch.
  - LFS for `files/` on the ref only. Logs stay plain git.

## Rejected
- LFS on its own: runs stay on main (see the assessment).
- `git notes`: notes hang off commits, and runs aren't about one commit.
  Ref: https://git-scm.com/docs/git-notes
- iCloud Drive or another synced folder outside the repo: covers one person's devices, not a
  team. It also drops the git history GIVENS.md relies on.
- A worktree of the runs branch in the data folder: simpler to write to, but it adds an
  entry to `.git/worktrees`. Branch windows turned down worktrees for the same reason.

## Decided (2026-10-17)
- A: `refs/givens/runs`. Plus an upper limit on what the ref holds: the last X runs or x MB,
  whichever is smaller.

## Design, as decided
- Where things live on a device: `<git common dir>/givens/`.
  - `tree/`: the ref's files, checked out: `tree/runs/*.log`, `tree/runs/files/…`,
    `tree/limits`. `runs.Read` reads it beside `capabilities/runs/`, so the CLI and the app
    both see these runs without a Givens data folder.
  - `index`: git's index for that tree (`GIT_INDEX_FILE`), never the developer's.
  - `lock`: one writer at a time, app or CLI.
  - Inside `.git`, as git-lfs keeps `.git/lfs`: git ignores it, and it goes with the clone.
- The marker: `capabilities/runs/off-main.md`, committed on the branch like any spec file. It
  tells every clone (and anyone reading) that runs are in the ref. Givens syncs on open only
  when it's there, so projects that don't use this pay no network cost.
- Limits, in the ref's `limits` file, so every device prunes the same way (devices with
  different limits would undo each other):
  - Defaults: 1000 runs, 250 MB. GitHub asks for repos under 1 GB, ideally; 250 MB leaves
    most of that for code.
  - A run's size: its line plus its files folder (attachments and note.md).
  - Past either limit, drop the oldest runs until both are at 80% or less. Then the next
    trim is 200 runs or 50 MB away, not on every run.
  - Each scenario's (and outline row's) newest run line is always kept, files or not, so a
    trim never turns a verified scenario into "Never run". Lines are about 130 bytes.
  - A trim writes a commit with no parent, so the dropped files leave the ref's history,
    and pushes with `--force-with-lease`. Other devices fetch the trimmed set once.
- Sync is a three-way merge: base (the remote tip seen last), local, remote. A day's log is
  remote's lines plus local lines base didn't have; files likewise. So a trim on one device
  isn't undone by another that still had the old runs.
- A commit's parent is the remote tip it was merged with, so history stays a line and a trim
  never comes back through a parent.
- Commits are made as "Givens <givens@localhost>": a run line already names its author, and
  a missing user.email mustn't stop a run being recorded.
- `git add -A --force` in the tree, so a global ignore like `*.log` can't drop a run.
- Remote: `origin`. Fetch with `+refs/givens/*:refs/givens-origin/*`, so a remote without
  the ref isn't an error.
- Timeouts: 20 s a local git call, 120 s a fetch or push, 8 s for the fetch on open.

## Open questions
- Check your host takes pushes to `refs/givens/*`. GitHub and GitLab take custom refs
  (git-bug relies on it). Not checked: Bitbucket and Azure DevOps.

## As built (2026-10-17)
- Spec: capabilities/keep-runs-off-main, 7 scenarios: keep off main, record, another device,
  two devices, offline, share later, old runs dropped. Go and Swift steps, against real repos
  and a bare remote in the scenario's folder.
- `internal/runs`: `Read` also reads `<git common dir>/givens/tree` (found without running
  git, worktrees included), once per run; `FolderOf` finds a run's files in either place;
  `Prune` and `Limits`; `MergeLines`.
- `internal/runref`: `Store.Change(write, timeout)` takes the lock, lets write add to the
  copy, then fetches, merges three ways, trims, commits and pushes, starting over (3 times
  at most) when another device pushed first. Unreachable: committed here, `ErrUnreachable`.
  `Unshared` counts lines the remote doesn't have. `Adopt` brings runs/ in.
- git gets `--work-tree` only for `add` and `checkout-index`: it refuses a work tree whose
  folder isn't there yet, which broke `remote get-url` on a fresh repository.
- `internal/app`: runs_off_main.go. "Keep runs off main" (asks first), "Share runs" (also
  inside a Drop in, right after recording), "N runs not shared yet". RecordRun writes through
  `writeRun` into the copy when off-main.md is there. Open syncs then, with an 8 s limit.
- Serve: `window.syncRuns`. Mac: a toolbar button while "Keep runs off main" is offered;
  "Share runs" in the banner; a sync every 300 s per window.
- GIVENS.md (both copies): the layout line and a rule.

## Known gaps
- The sync on open blocks the window for up to 8 s when the remote can't be reached: serve
  answers one call at a time. Added to todo.txt. Fixed 2026-10-24, see below.
- The remote must be called `origin`.
- No way to change the limits but writing `.git/givens/tree/limits` and choosing "Share
  runs". Added to todo.txt. Done 2026-10-24, see below.
- A branch's window doesn't show runs kept off main: its copy isn't in a repository. Added to
  todo.txt. Done 2026-10-24, see below.
- The host keeps a trimmed ref's old objects until its own garbage collection runs; the
  reachable size is what the limits bound.
- `givens list` shows what this device last fetched; it doesn't sync.
- Not eyeballed. To check: in a project with an origin remote, press the toolbar's "Keep runs
  off main" button and confirm; `git status` shows runs/ gone and off-main.md added. Record a
  run, then `git ls-remote origin refs/givens/runs` shows the ref. Turn Wi-Fi off, record
  another: the banner says "1 run not shared yet" with "Share runs".

## Made the default (2026-10-17)
- Asked for: "Just make this the default". Now any project in a git repository with an
  `origin` remote keeps its runs off main from its first run; nothing to choose.
- Without a remote, or outside git, runs go in capabilities/runs as before. Chosen over a
  local-only ref: with no remote there's nowhere to share to, and "N runs not shared yet"
  would show for ever.
- Runs already in capabilities/runs (from before the remote, or an older Givens) still count.
  "Move runs off main" (was "Keep runs off main") moves them, and is offered only while
  there are some. It still asks, since it sends files to the Trash.
- off-main.md is only written by a move now; nothing reads it. GIVENS.md (in every
  capabilities folder) says where runs go.
- Every git project with a remote fetches the ref on open. A sync with nothing here and no
  ref on the remote makes and pushes nothing (`TestNothingToShare`), so opening a project
  never pushes.
- No opt-out. If one's wanted: a line in GIVENS.md's rules and a file Givens checks.
- Spec: "Runs are kept off main by default", "Without a remote, runs stay in
  capabilities/runs", "Move runs off main"; the other five now start from "a git repository
  with a remote". 8 of 8 in Go and Swift.
- Known gap, now wider: the fetch on open waits up to 8 s for a remote that can't be reached,
  for every git project with a remote.

## Sync on open in the background (2026-10-24)
- `App.Open` no longer waits for the sync. `openRunStore` starts `Store.Change(nil,
  OpenTimeout)` on a goroutine, with a buffered channel for its result. The window takes the
  result in on its next `refresh` (every snapshot), so serve's pusher shows it within 250 ms.
- Chosen over fetching in the background and merging on the main thread: `Store` keeps no
  state in memory, and its lock file already keeps a Record run's change from running beside
  the sync. Simpler, and nothing new to get wrong.
- While it runs, the window says "Syncing runs with origin". An error that isn't
  `ErrUnreachable` goes in `runsNote` ("Runs couldn't be synced: …"), as there's no caller
  to return it to now.
- `OpenTimeout` stays 8 s: a Record run or "Share runs" right after opening waits on the
  lock meanwhile, holding serve, so the sync stays short.
- runref's git calls set `WaitDelay` (1 s). Without it, a timed-out git whose helper (ssh, or
  ext's sh) kept the output pipes open held the call until that helper ended, so the timeout
  didn't hold. Ref: https://pkg.go.dev/os/exec#Cmd.WaitDelay
- `Window.WaitForRunsSync(limit)`: the steps wait with it after opening (Go's
  `world.WaitForRunsSync`, and `World.waitForRunsSync` in Swift, which polls until the
  message goes).
- Spec: "Open a project whose remote doesn't answer", @error. The remote is `ext::sh -c
  sleep% 30` with `protocol.ext.allow always` in the repository's config. It costs 8 s a run,
  in Go and in Swift. 9 of 9 in both.
- Known gap: a Record run in the first 8 s after opening, with a remote that doesn't answer,
  still holds the window while it waits for the lock.

## Change the limits from Givens (2026-10-24)
- Core: `Window.RunLimits()` reads the copy's limits file (defaults until one is written);
  `Window.SetRunLimits(runs.Limits)` writes it through `Store.Change` and shares it, so
  lower limits trim at once. Each limit is 1 to `runs.LimitMax` (was `limitMax`); outside
  that is a returned error. Outside a repository with a remote: `ErrRunsOnMain`.
- Offer: "Change run limits", whenever runs are kept off main. Choosing it does nothing in
  the core; the dialog opens, as "New story" does.
- Serve: `window.runLimits` (value `{runs, megabytes}`) and `window.setRunLimits` (both
  numbers as strings, as every param is).
- Chosen over opening `tree/limits` in Givens's editor: the editor's save doesn't share, and
  a typo there makes every later sync fail on `ReadLimits`.
- Mac: a toolbar button (gauge) opens `RunLimitsSheet`: two number fields, filled from
  `runLimits()`, and Save. Builds; not eyeballed. To check: open a project with an origin
  remote, press the gauge button, see 1000 and 250, set 3 and 100, Save; then
  `git show origin/refs/givens/runs:limits` (after `git fetch origin refs/givens/runs`) says
  `runs 3` and `megabytes 100`.
- Spec: "Change the limits", in Go and Swift. 10 of 10 in both.

## A branch's window shows the ref's runs (2026-10-24)
- Runs aren't kept per branch, so a branch's window reads the ref the checked-out project
  keeps. `OpenBranch` writes `runs/.givens-ref-copy` into the branch's copy, naming the
  origin's `<git common dir>/givens/tree`; `runs.RefCopy` reads that link first. Status,
  history, notes and files all go through `RefCopy`, so they all see it.
- Chosen over passing the ref's copy through `StatusOf`, history and `FolderOf`: those are
  shared with the givens command, and the link is one stat at a fixed place. Rejected: a
  `.git` file in the copy pointing at the origin's git dir; git itself would then treat the
  copy as a work tree of the real repository.
- The link is taken only when it's absolute and ends in `tree`.
- Spec: "A branch's window shows runs kept off main", Go and Swift. 11 of 11 in both.
- Known gap: a branch's window keeps the statuses it worked out until a file in its copy
  changes or the day turns, so a sync in the main window doesn't show there at once.
