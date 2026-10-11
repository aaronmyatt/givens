# Write stories and scenarios

## Goal
Make the 20 scenarios in `capabilities/write-stories-and-scenarios/write-stories-and-scenarios.feature` pass: each loses its `@wip` tag,
its steps, already written in `acceptance/write-stories-and-scenarios/`, pass, and `scripts/check.sh` passes.

## Design
- Likely packages: internal/edit (minimal-diff writes to .feature files, renames, deletes to Trash).
- One scenario per commit: run it with `GIVENS_TAGS=@wip`, replace the stubs it hits in
  `internal/app` with real code until it passes, then remove `@wip`.
- Reuse step text across scenarios; a step used word for word is defined once.

## As built

## Choices the steps already make
- "Keywords lined up" means keywords right-aligned to the widest one: `  And the order stays pending`.
- "A developer is editing X" opens X and appends " until the bank replies" to its last step, unsaved.
- "and confirms" chooses "Rename" only when it's offered; "they confirm" chooses "Delete".
- The preview's parser message is exactly "Expected a step, a tag, a comment or a new scenario".
- "An empty numbered list" means the only non-blank line is `1.`.
- All 20 (2026-10-11), Go and Swift. The editor, scenario rename and the confirm prompt came
  with Gherkin files (tasks/gherkin-files/notes.md).
- New story writes `<slug>/<slug>.feature` and selects it; New scenario appends a block
  (tagged @error when asked) and `scripts/<slug>.sh` sourcing the kit, then opens the editor at
  the block. "New story" is always offered in a project window, "New scenario" with a
  capability selected; choosing either opens the dialog.
- Typing in the editor starts a step line at the end of the open scenario's block;
  `Suggestions` offers steps starting with what's typed, most used first, "used in N".
- Conflicts: while the file on disk differs from what the editor opened, it shows "This file
  changed on disk" and offers "Keep mine", "Use theirs" and "Compare".
- Save marks the line the parser stops at ("Expected a step, …") and saves anyway.
- Preview: the scenario's block with keywords lined up and unsaved changes, a non-step line
  marked; about.md as the library renders it.
- Capability rename moves the folder and .feature file and updates from lines. Delete asks
  ("Delete" or, when something builds on it, "Delete anyway"), removes the block with its
  tags and blank line, and trashes setup files or the folder.
- Pending Trash: under serve the Mac app does the move after the reply, so the core keeps
  trashed paths out of loads until they're gone from disk.
- Fixed: from-line updates read scripts from disk (a just-renamed script was looked up under
  its old name).
- Swift steps: "they see" now waits up to 2 s, since the app learns of another tool's change
  only from a pushed snapshot (the spec allows 2 s). The test world's calls get 5 s over the
  10 s setup wait, so a wait can't time out its own call.
- UI: New story and New scenario sheets; the editor shows its preview. Known gap: step
  suggestions aren't shown while typing in the SwiftUI text editor (the core only sees typing
  through Type). Not eyeballed.
- Suggestions in the Mac editor (2026-10-11): TextEditor doesn't expose its cursor, so the
  editor takes the line where the text last changed as the line being typed. After a 150 ms
  pause it sends that line to `window.stepSuggestions` (`text` param), which calls
  `Editor.SuggestionsFor`; with no `text` the call still answers for the core's typed line, as
  the steps use it. Choosing a suggestion swaps the line's words in the local text, keeping its
  indent and keyword; Save writes it as before. A step already written out word for word
  offers nothing.