# Work in the terminal

Asked 2026-10-24: flesh out the `givens` command so it mirrors the Mac app, with text edits in
the developer's `$EDITOR` and a Bubble Tea TUI (a text UI: full-screen, keyboard driven) for
Drop in and for browsing a project.

## Goal
- A new capability, `capabilities/work-in-the-terminal/`, whose scenarios pass: each loses
  `@wip`, its Go steps pass, `scripts/check.sh` passes. Go suite only: no window.
- A developer can do from a terminal what they do in the app: browse, read, Drop in (cards,
  checklist, note, attachments, Record run), write stories and scenarios, edit any file,
  rename, delete, flag to do, helpers, history, search, trust, sync.
- `list`, `run` and `check` stay as they are, plain text for agents and CI.

## Design
- Every new command drives `internal/app` with no window, as `givens serve` does
  (`app.New`). No new rules in `cmd/givens`: the CLI and the app agree because they share
  `Window`, `Editor` and `Session`.
- Text edits go through `$EDITOR`, like `git commit` and `kubectl edit`:
  - `Window.Edit(target)` → `Editor.Text()` into a temp file with the same extension, so the
    editor colours it. Ref: https://pkg.go.dev/os#CreateTemp
  - Run `sh -c '${VISUAL:-${EDITOR:-vi}} "$@"' sh <file>`, so `EDITOR="code -w"` works.
    Ref: https://git-scm.com/docs/git-var#Documentation/git-var.txt-GITEDITOR
  - Read it back. Unchanged: "No changes", exit 0. Else `Editor.SetText`; if
    `Editor.Marks()` has problems, put them at the top as `# givens: line N: …` and reopen.
    Valid: `Editor.Save()`, which refuses with "This file changed on disk" as in the app.
    Ref: https://kubernetes.io/docs/reference/kubectl/generated/kubectl_edit/
  - `+N` (open at the scenario's line) only for vi, vim, nvim, nano and emacs.
- TUI: Bubble Tea (Elm style: Model, Update, View) and Bubbles (viewport, spinner, list,
  textinput, table, help, key). Lip Gloss for styles. All of it in `internal/tui`; nothing in
  `internal/app` imports it.
  - Ref: https://github.com/charmbracelet/bubbletea
  - Ref: https://github.com/charmbracelet/bubbles
  - Drop in: `tea.Tick` every 50 ms → `Session.Status()` polls. Only `Update` touches the
    Session: `tea.Cmd`s run on other goroutines and Session has no locks.
    Ref: https://pkg.go.dev/github.com/charmbracelet/bubbletea#Tick
  - `$EDITOR` from inside the TUI (run note, Edit script, Edit teardown):
    `tea.ExecProcess` hands the terminal over and takes it back.
    Ref: https://pkg.go.dev/github.com/charmbracelet/bubbletea#ExecProcess
  - Copy a code block: `pbcopy`, as Givens is macOS only. Ref: https://ss64.com/mac/pbcopy.html
- TUI only when stdout is a terminal (`os.ModeCharDevice`, no new dependency). Otherwise a
  TUI command says "givens drop-in needs a terminal; use givens run" and exits 2, and bare
  `givens` prints usage as now. Ref: https://pkg.go.dev/io/fs#ModeCharDevice
- Tests: steps drive the `internal/tui` models by sending key messages to `Update` and reading
  `View()`, with no pty. `$EDITOR` is a fake in `acceptance/world`: a script that applies a
  given edit, like the `GIVENS_OPEN_COMMAND` stand-in.

## Decided (2026-10-24)
- Bubble Tea or plain prompts: took Bubble Tea. Drop in has live output, cards, offers and
  a checklist at once; with plain prompts that's a state machine written by hand.
- A new capability or more scenarios in use-givens-from-the-terminal: took a new one. That
  story is agents and CI; this one is a person at a terminal. Like mix-shell, it keeps a done
  capability closed.
- Edit a temp file or the real file: took the temp file. A half-typed edit never lands on
  disk, and Save keeps its changed-on-disk guard.
- Bare `givens` in a terminal opens the browser; `givens <folder>` still opens the Mac app.
  The other option was `givens browse`; bare is the shorter habit.

## Open questions
- Which Bubble Tea major to pin (v1 or v2); decide at the deps step.

## Spec (2026-10-24)
- capabilities/work-in-the-terminal: 28 scenarios, all @wip. Browse 4, Drop in 10, editing 5,
  writing and renaming 5, panels as commands 3, no terminal 1. about.md lists the TUI's keys
  and how editing works. In CoverageTests' windowless list.
- Commands it names: bare `givens` (the TUI), `givens drop-in <key>`, `givens edit <key or
  file>`, `givens new story <slug>`, `givens new scenario <capability>`, `givens rename <key>
  <name>`, `givens delete <key> [--yes]`, `givens todo` and `givens done`, `givens helpers`,
  `givens history <key>`, `givens search <words>`, `givens trust`, `givens sync`.
- Search is "/" in the TUI, as it's Bubbles' list filter key too; there's no separate filter.
- The outline scenario uses drop-into-a-scenario's "Card rejected by the bank", as the sample
  shop has no outline of its own.
- A changed-on-disk save prints the problem and keeps the edit in a file it names, rather
  than the app's Keep mine / Use theirs / Compare: a terminal can't show a compare there.

## Deps (2026-10-24)
- Chose v2: charm.land/bubbletea/v2 v2.1.0, charm.land/bubbles/v2 v2.2.1,
  charm.land/lipgloss/v2 v2.0.6. v2 is the maintained major (v1 stops at 1.3.10); v1
  examples online don't compile against it (View returns a tea.View, `tea.NewView(s)`).
  On the allowlist in scripts/check.sh, with why.
- They bring 25 indirect modules (charmbracelet/x/*, ultraviolet, colorprofile, uniseg,
  go-runewidth, atotto/clipboard for textinput, sahilm/fuzzy for list, golang.org/x/sys…).
- Probe (a scratch main using viewport, spinner, textinput, list, lipgloss): 5.7 MB, starting
  in 28 ms. The real growth is measured once cmd/givens links internal/tui.

## CLI: opening a project (2026-10-24)
- cmd/givens/project.go: `openProject()` makes an app with `app.New` and opens the
  capabilities folder found from the working folder. Every new command uses it.
- App data: `GIVENS_APP_DATA`, else `~/Library/Application Support/Givens` (os.UserConfigDir),
  the Mac app's own folder. So trust, recents and kit releases are shared: a project
  trusted in the app is trusted in the terminal, and the other way round. Open windows are
  only written by Quit, which the command never calls.
- Effects, as the Mac app's Platform does them: pbcopy and pbpaste, the Finder's Trash via
  osascript (so a delete can be put back), `open` for links. 10 s each.
- Trust is asked by the core's prompt, as in the app; the TUI shows it with t and d.

## The $EDITOR loop (2026-10-24)
- internal/termedit: `Begin(editor)` writes the app.Editor's text to
  `$TMPDIR/givens-*-<file name>`, so the extension is kept. `Command()` is
  `sh -c '${VISUAL:-${EDITOR:-vi}} "$@"' sh [+N] <file>`, +N (the scenario's line) only the
  first time and only for vi, vim, nvim, nano and emacs. `Finish()` reads it back and says
  what's next; `Run(editor)` loops with the terminal, at most 20 opens. The TUI will hand
  `Command()` to tea.ExecProcess and call `Finish()` when it returns.
- Outcomes: Saved (temp file removed), NoChanges, Abandoned (reopened over problems and
  closed unchanged; the edit stays in the temp file), ChangedOnDisk (another tool saved;
  nothing written; the edit stays in the temp file).
- Problems are checked before saving, unlike the app, which saves anyway and marks: the
  spec asks that the file be "left as it was until they fix it". New core calls for it:
  `Editor.Problems()` (parse errors in the unsaved text, via the new
  `spec.ParseFeatureText`), `Editor.Line()`, `Editor.ChangedOnDisk()`.
- "# givens: line N: …" lines go at the top and are taken off before anything is checked
  or saved. Only .feature files get them; a .md heading starts with # too, but .md files
  have no problems to show.
- Fake editor: acceptance/world/editor.go. `w.FakeEditor(edits...)` writes a script as
  EDITOR that logs its arguments and runs edit N (a bash snippet on "$1") the Nth time it
  opens; with none left it changes nothing. setUp sets EDITOR=false and clears VISUAL, so a
  real editor never opens from a step.
- Unit tests: saved at the scenario's line (+13 for vim), no changes, a broken edit reopens
  with "# givens: line" and isn't saved until fixed, changed on disk.

## CLI: givens edit (2026-10-24)
- `givens edit <capability>/<scenario>` (a run key or name: checkout/card-declined) or a file
  under capabilities/ (checkout/about.md, checkout/scripts/card-declined.sh). Over
  `Window.Edit` and `termedit.Run`. Prints "Saved <file>" or "No changes" (exit 0); "<file>
  isn't saved: it still has problems. Your edit is in <tmp>" or "<file> changed on disk while
  you edited it. Your edit is in <tmp>" (exit 1).
- Tried by hand in a copy of the sample shop with a scripted editor: a change saved, no
  changes, a target that doesn't exist.
- Found by hand: $VISUAL wins over $EDITOR, as designed, so a step or test that sets only
  EDITOR must clear VISUAL. The world's setUp and termedit's tests do.

## CLI: givens new (2026-10-24)
- `givens new story <name>` and `givens new scenario <capability>`: a "Name: value" form in
  the editor (`termedit.Template`, lines starting with # left out), then `Window.NewStory`
  or `Select` + `Window.NewScenario`, so the rules are the app's: a new scenario gets @todo
  and its scripts/<slug>.sh. The story's name starts as the argument, "gift-cards" as
  "Gift cards"; "Error state: yes" tags a scenario @error.
- Tried by hand: both, with scripted editors.

## CLI: rename, delete, todo, done (2026-10-24)
- cmd/givens/manage.go, over `Window.Rename`, `Window.Delete`, and `Select` + `Choose`
  with "Flag as to do" or "Remove to do flag". A question the app would ask stays up in the
  window; new `Window.Question()` gives it and its yes, the command prints it, and `--yes`
  chooses it. Without --yes: "Nothing changed. Run it again with --yes to delete anyway.",
  exit 1. Tried by hand: rename (script renamed too), delete refused then --yes, todo, done,
  done again ("isn't flagged to do").
- Test seam: with GIVENS_TEST_SEAMS=1 and GIVENS_TEST_EFFECTS=<folder>, the command's
  clipboard, Trash and `open` write files in that folder instead (clipboard, Trash/, opened).
  Both must be set. world.Run sets them, and GIVENS_APP_DATA to the scenario's. Found when a
  delete --yes tried by hand sent a scratch copy of pay-with-saved-card.sh to the real Trash.

## CLI: the panels as commands (2026-10-24)
- cmd/givens/panels.go: `givens helpers [--json]` (name, file:line, how many scripts call
  it), `givens helper new <capability|_shared> <name>` and `givens helper edit <name>` (over
  NewHelper, then the editor), `givens history <key> [--json]` (Select, then the snapshot's
  runs; "Never run"), `givens search [--kind K] <words> [--json]` (⌘K and Type, grouped as
  the panel groups), `givens trust`, `givens sync`.
- `Config.NoSyncOnOpen`, set by the command's app: opening a project doesn't start the
  background sync, since a command that ended mid-fetch would leave the store's lock for
  ten minutes. `givens sync` syncs, and says "Runs here go in capabilities/runs…" without a
  remote (new `Window.KeepsRunsOffMain`). Known gap: the TUI doesn't sync on open either.
- Tried by hand on a copy of the sample shop: helpers, history, search, search --kind
  Helpers, trust, sync.

## TUI (2026-10-24)
- internal/tui: one `Model` with three modes (browse, dropIn, search). tui.go (Update,
  View, Tick/IsTick), dropin.go (the page, offers as keys), checklist.go (marks, note,
  attach, ask input), edit.go (files through termedit and Exec), browse.go (tabs, list,
  outline rows, new scenario), search.go ("/").
- `tui.New(window, exec)`: exec runs the developer's editor; nil is tea.ExecProcess. The
  steps pass one that runs the command at once. Only Update touches the Window and the
  Session; the commands returned only tick or run the editor.
- Drop in: a 50 ms tick polls the Session; the page draws the window's snapshot (status
  with bubbles' Dot spinner frames, the last 18 output lines with ↑ ↓ to scroll back, card
  lines, the checklist, messages) and a keys line built from the offers on show: Enter Done
  or Continue, f Failed, o Open, c Copy, k Keep waiting, s Stop, r Record run, R Reset
  state, t Try again or Trust, e Edit script or setup, E Edit teardown, u Run setup anyway,
  A Add teardown, d Don't trust, T Trust this project, S Share runs, y/n for a question.
  Edit offers open the file in the developer's editor (new `Session.FileOf`) instead of the
  app's editor. q or Esc leaves, asking first if marks aren't recorded.
- Not bubbles' viewport or textinput: the page scrolls its own slice, and an ask card's
  value and an attach path are typed into a plain line (Enter gives it, Esc drops it,
  Backspace). Fewer moving parts, and the steps read it as text. A line being typed takes
  every key, q included: a path with a q in it left the Drop in before that.
- Checklist: j k move, Enter done (passed for a Then step), p f pass/fail step, x clear, n the
  note in the developer's editor (new `Session.Note`), a attach, r Record run.
- c on a markdown card copies its first code block, else its first inline code (new
  `Session.CopyCard`, `Session.PendingCard`).
- Browse: Tab cycles Capabilities, Views, Tags. Capabilities are headings over their
  scenarios with status; a view or tag lists its scenarios on Enter. Moving onto a scenario
  selects it, so its steps and setup path show beside; h shows its runs; e edits it; n
  writes a new scenario in its capability (a Name form in the editor); Enter drops in, or,
  for an outline, lists its rows first (DropInRows, ChooseOnRow).
- Search: "/" opens ⌘K's search; typing narrows; ↑ ↓ move; Enter on a scenario shows it in
  the list.
- New core calls, all small: `Window.Question`, `Window.CloseEditor`, `Session.ScenarioKey`,
  `Session.FileOf`, `Session.Note`, `Session.CopyCard`, `Session.PendingCard`.
- givens: bare `givens` and `givens drop-in <key>` open the TUI when stdin and stdout are
  terminals (os.ModeCharDevice). Otherwise bare givens prints usage, as before, and
  drop-in says "givens drop-in needs a terminal; use givens run <key>", exit 2.
- Binary: 13.1 MB to 14.9 MB.
- Tried in a real terminal through `script` (a pty): the list, statuses, moving with j, the
  steps beside, q. Not eyeballed by a person yet; that's the @human line.

## Go steps (2026-10-24)
- acceptance/work-in-the-terminal: tui_test.go (the driver: key messages to Update, ticks
  sent by the steps, commands run as a Program would), dropin_test.go, commands_test.go.
  28 of 28 pass with GIVENS_TAGS=@wip, in 17 s.
- Spec changes while writing them: a Background, "a developer's terminal is in the sample
  shop's folder", as every command scenario needs the shop; "Edit a setup script or
  about.md" gained "Given a developer's editor adds a line to the file it opens", as the
  common "a developer runs" step can't set up the editor; the checklist line reads "When
  they place the order", as the sample shop words it.
- Still running is driven by the world's fake clock: the step sets @timeout:1s and moves
  the clock 2 s once setup starts.
- The sync scenario records a run in process while the bare remote is moved away, then
  runs `givens sync`.

## As built (2026-10-24)
- All 28 scenarios lost @wip and pass; scripts/check.sh passes.
- use-givens-from-the-terminal's description no longer says list and run "stay plain bash"
  (they're Go); it says they print plain text for agents and CI, and points here.

## Browsing, after a first try (2026-10-24)
- Asked for: the cursor's line easier to see, a capability's story and about.md readable,
  the list scrolling, and capabilities folding.
- The cursor's line is a reverse-video bar the list's width (lipgloss Reverse, now a direct
  use). Text, which the steps read, draws with no styling. Same bar in search, the outline
  rows and the checklist.
- Enter on a capability opens its story page: the As a, I want and So that lines, then
  about.md as written (markdown, not rendered: no glamour, one less dependency). j k scroll,
  Space b page, g G ends, e edits about.md through the app's Edit or Add about.md offer.
- Space folds or unfolds; ← folds (from a scenario, its capability), → unfolds. Folds last
  across Tab. MoveTo, and so search's Enter, unfolds a scenario's capability.
- The list scrolls as little as keeps the cursor on show. 100 columns or wider, details sit
  beside the list (64 columns at most) as the spec always said; narrower, under it, with the
  list two thirds of the page. A name past 34 columns ends in "…"; details show it whole.
- On a heading, the details show the story and the top of about.md.
- 3 new scenarios: Read a capability's story, Fold a capability, A long list scrolls. 31 pass.
- Then: capabilities open folded, and z unfolds them all (or folds them all, when any is
  unfolded). z, not a vim pair like zR and zM: one key, and the TUI has no two-key chords.
  The cursor stays on its heading. Browse a project now unfolds Checkout with Space; Fold a
  capability became Unfold a capability; a new scenario covers z. 32 pass.

## Drop in page, after a first try (2026-10-24)
- The Drop in page shows the scenario's Given steps (Background first, an outline row's
  values filled in) just above the checklist, so the whole scenario reads there. They come
  from Session.Givens; setup does them, so they have no box to mark.
- e edits the scenario, at its line, from a Drop in. When setup failed, e still opens the
  setup script (Edit setup, Edit script): that's what needs fixing then, and an existing
  scenario covers it. The keys line says which: "e edit scenario" or "e Edit setup".
- 2 new scenarios: Read the whole scenario in a Drop in, Edit the scenario from a Drop in.
  34 pass.

## Vim keys (2026-10-24)
- j k and the arrows wrap round at a list's ends: the project list, Examples rows, search
  results and the checklist. g G (and Home End) jump to the ends; not in search, where g is
  typed.
- h and ← go up a level: scenario to its heading, then fold the heading. l and → go in:
  unfold, then the first scenario, then Drop in. In a story and on Examples rows, h goes
  back. Not on the Drop in page: q and Esc leave it, and h there is too easy to hit.
- A scenario's runs moved from h to r (r is Record run only on the Drop in page).
- ← used to fold from a scenario in one press; now it's h twice, as vim file trees do.
- 4 new scenarios; A long list scrolls presses Down 11 times, as 20 now wraps. 38 pass.

## Terminal first: the Mac app archived (2026-10-24)
- macos/, internal/serve and the app's build, run and sign scripts moved to _archive/ (Go
  skips folders starting with _). givens serve is gone. check.sh has no swift check; the edit
  hook skips _archive/. install.sh builds givens into ~/.local/share/givens and links it.
- givens <folder> opens the TUI there. With no capabilities folder anywhere above, it opens the
  folder itself: "No capabilities here yet", c creates one. u takes "Update kit".
- GIVENS_TEST_SCREEN=1 (with GIVENS_TEST_SEAMS=1) prints the TUI's first screen with no
  terminal: the suites' stand-in for one.
- e on the Drop in page always edits the scenario; s edits the setup (Edit script / Edit setup
  moved from e to s). s on a scenario in the list edits its setup, or starts a script.
- Editors open at the line: +N (vi kin, micro, kak), file:N (subl, zed, hx), -g file:N (code and
  forks), -l N (mate). Sublime was opening at the top before.
- Helpers tab: Enter or e edits a helper at its function, n writes one (In: a capability's
  folder, or _shared, which NewHelper now takes as ""; givens helper new _shared used to trip an
  assertion).
- A .sh edited from the terminal is checked with bash -n, and reopened over its problems like a
  .feature file.
- ⚑ marks @todo scenarios in the list; scenario details show "Requires …" labels.
- The kit is version 3: its "needs givens" hint names scripts/install.sh, not the app's menu.
- Every capability's spec was reworded for the core and the TUI. 7 open-a-project scenarios were
  deleted as Mac-only: the welcome window, Recent, Dock drop, Install menu, and three about
  several windows. 45 TUI scenarios now.

## Views and Tags keys (2026-10-24)
- Moving on to a scenario under an open view or tag folded the list away: the TUI listed it
  with ListedScenarios, which follows the window's selection, and selecting the scenario
  unselected the view. The core now has ScenariosIn(view), which lists a view's or tag's
  scenarios without selecting it.
- Views and tags now fold as capabilities do: several open at once, kept across Tab, z for all.
  Folds are kept by tab and name, so a tag and a capability with the same name fold apart.
- New scenario: Views and tags take the same keys as capabilities. 46 pass.
- Then key coverage across tabs: three Scenario Outlines, one row per tab (Capabilities /
  Checkout, Views / Error states, Tags / payments): folding (Space, l and h, z), moving (l in,
  j, h up, k and j wrapping, G and g), and a scenario's keys under each (e, s, r, l). They
  replace seven capability-only scenarios (unfold, z, wrap, g G, h and l, h folds) and the one
  Tags scenario. One scenario per rule and per tab, not per key: the full table stays in
  about.md. 48 pass.

## Known gaps
- An edit saved from a Drop in doesn't change that Drop in's checklist: the Session keeps
  the scenario it started with. Leave and drop in again to see the change.
- No TUI key for core offers on a scenario: Copy, Copy steps, Flag as to do, Write setup steps,
  Delete file; nor for Move runs off main, Share runs, Change run limits, or reading another
  branch. Their scenarios word them as core behaviour.
- No step suggestions, Preview, or Keep mine / Use theirs / Compare in the terminal; the core
  still has them.
- internal/app still has Mac words in names and comments: ClickInlineCode, DropInRow.Greyed and
  .Disabled, "button", App.Launch and the multi-window code nothing calls now.
- Not tried by a person: the TUI in Terminal.app and iTerm2, vim and `code -w` (the @human
  line). Tried only through `script`, a pty, with keys piped in.
- The TUI doesn't sync runs kept off main on open or every few minutes, as the app does;
  `givens sync` does it when asked.
- `givens helper edit` opens the helper's file at the top, not at its line.
- A delete or rename asked from the TUI's browser isn't offered; they're commands only.
