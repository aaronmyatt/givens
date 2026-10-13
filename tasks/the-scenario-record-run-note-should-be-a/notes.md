Replete with preview/beautified tabs. Images should be drag and droppable into the text editor and copied to the run folder for future rendering

## Goal
The run note is Markdown, written in a Write tab and read in a Preview tab. Images dropped
on it are copied into the run's folder and shown in the note.

## Design
- A run line is one line, so a long note can't live there. A note with a line break is kept
  whole as `note.md` in the run's folder (`runs/files/<key>/<time>/`, from the attachments
  task). The run line keeps its first line, `#`s trimmed, as a summary, so old readers and
  "Verified" still work.
- Images: dropping one on the note attaches it (the attachments task copies it on "Record
  run") and adds `![name](name)` to the note. The link is relative, so it resolves against
  the run's folder later. In the Preview tab, before recording, a name is looked up among
  the attached files instead.
- A file called note.md can't be attached: that name is the note's.
- Rendering: SwiftUI's Text renders inline Markdown only. A small block parser in GivensUI
  (headings, lists, quotes, fenced code, images on their own line, paragraphs) feeds a
  MarkdownView in the app; inline parts go through AttributedString(markdown:). The same
  view can render about.md for the later "richly rendered" task.
  Ref: https://developer.apple.com/documentation/foundation/attributedstring/init(markdown:options:baseurl:)
- Limit: 64 KB a note.

## Rejected
- A third-party Markdown package: new dependency; the parser needed is small.
- Inserting the image link at the cursor: SwiftUI's TextEditor doesn't expose the cursor.
  The link goes at the end of the note.

## As built (2026-10-12)
- Spec: "Record a run with a note in Markdown" in drop-into-a-scenario.
- Core: `runs.Summary`, `WriteNote`, `ReadNote` in internal/runs/files.go. RecordRun writes
  note.md when the note has a line break, and the line keeps the summary. `Files` leaves
  note.md out; Attach refuses a file called note.md. RunLine has `noteMarkdown` and
  `folder`. `session.setRunNote` and `recordRun` take up to 64 KB, line breaks included.
- Swift: `Markdown.blocks` in GivensUI (headings, paragraphs, bullet and numbered lists,
  quotes, fenced code, images alone on a line, rules), unit-tested in the new GivensUITests
  target. `MarkdownView` in the app draws them; inline parts via AttributedString.
- Drop in page: a Write/Preview switch over the note. Write is a monospaced TextEditor;
  dropping files on it attaches them, and each image gets `![name](file)` at the end of the
  note. Preview finds linked images among the attached files.
- Scenario page: a run with note.md shows it rendered, images read from the run's folder.
- Not eyeballed. To check: drop in, drag a screenshot onto the note, switch to Preview, record
  the run, then look at the run on the scenario page.
- Known gaps: the image link goes at the end, not at the cursor. Nested lists and tables
  aren't drawn as such.
