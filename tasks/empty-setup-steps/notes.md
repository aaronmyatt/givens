In build-boost, "A visitor reads the newest posts" opened a Drop in stuck at "Given · waiting
on you", with a box holding "1." and a "Type the value" field. Typing gave "not built yet: see
todo.txt". Its scripts/a-visitor-reads-the-newest-posts.md held only "1.", as "Write setup
steps" writes it.

## Goal
- A markdown setup with no steps fails with a message saying so, and offers "Edit setup" and
  "Try again", like a card marked failed.
- "Edit setup" (and "Edit script", "Edit teardown") from a Drop in shows the editor.
- The editor offers "Delete file" on a scenario's setup or teardown file. It asks first, then
  moves the file to the Trash and closes the editor.
- The Drop in page shows its "Type the value" field only for an ask card.

## Design
- Why it hung: the item pattern `^(\d+)\.\s+(.*)$` needs a space after "1.", so "1." became
  intro text and the setup had zero steps. addMarkdownCards still set "waiting", with no card
  to mark, so nothing could end the wait. The intro "1." was drawn as a card.
- Why typing failed: the Mac page draws the text field whenever the session is waiting.
  Window.Type has nowhere to put text unless an ask card waits, so it returned
  ErrNotImplemented.
- Fix in the core: zero steps → fail with "<file> has no steps yet", markdownFailed set, so
  failedOffers gives "Edit setup" and "Try again". Intro isn't kept, so no stray "1." card.
- Leave the parser alone. "1." alone is an empty list item in CommonMark, and an empty item is
  no step either way.
- Edit offers from a Drop in leave the session first. While a session is open, the Mac page
  draws the Drop in over the editor, and Window.Choose sends every action to the session, so
  the editor was opened but never seen. These offers only show once setup has ended, so no
  marks are lost.
- "Delete file" rather than "Delete setup": it works on .sh, .md and .teardown.sh, and the
  editor is about one file. The confirm reuses "Delete" and "Cancel", and the Trash, as
  deleting a scenario does.
- The Mac field shows while the window offers "Continue", which only an ask card offers.
- Rejected: going straight to the checklist on zero steps (option B). The file exists, so
  someone meant to write steps; saying so beats hiding it.

## As built (2026-10-17)
- Spec: "Setup steps not written yet" (drop-into-a-scenario) and "Delete a setup file while
  editing it" (write-stories-and-scenarios). Steps in Go and Swift.
- Core: `addMarkdownCards` fails with "<file> has no steps yet" and sets markdownFailed.
  `Session.choose` leaves the Drop in before "Edit script", "Edit setup" or "Edit teardown".
  `Editor.setupFile` and `Window.deleteFile` in editor_more.go. Branch windows don't offer
  "Delete file", as readOnlyOffers is an allowlist.
- Mac: "Delete file" button in the editor header; the confirm shows in the banner. The
  "Type the value" field shows only while "Continue" is offered.
- Known gap: "Delete file" on a .sh setup that another scenario builds on with `from` doesn't
  warn, as deleting the scenario does. Added to todo.txt.
- Not eyeballed. To check: in build-boost, drop in to "A visitor reads the newest posts". See
  "a-visitor-reads-the-newest-posts.md has no steps yet", with no card and no text field.
  Choose "Edit setup", then "Delete file", then "Delete". The scenario page should offer
  "Create script" and "Write setup steps" again.

## Follow-up: "Delete file" on a script others build on (2026-10-17)
- Spec: "Delete a setup script another builds on" (@error) in write-stories-and-scenarios.
  Steps in Go and Swift; the Go one fails with the fix taken out.
- Core: `deleteFile` finds the scenario whose setup script is open, runs `builtOn` on its key,
  and goes through `askDelete`, as `Delete` does: "<name> builds on this scenario" with
  "Delete anyway", or "Delete <file>?" with "Delete".
- Only the .sh warns: `from` runs a scenario's script (and its teardown, when there is one),
  never its .md, so deleting a .md or a teardown breaks nobody's setup.
- Steps: "a developer has chosen … on …" now keeps every file and the opened path, so
  "nothing is deleted unless they choose" checks the file went to the Trash.
