# Work in the terminal

The givens command's TUI (a text UI) and `$EDITOR` are how a person works in Givens: browse,
Drop in, write and edit. `givens` opens it over the project the terminal is in, `givens
<folder>` over that folder's. `list`, `run` and `check` stay plain text for agents and CI; see
use-givens-from-the-terminal.

## Keys in the TUI

| Key | Where | What it does |
| --- | --- | --- |
| ↑ ↓ or j k | anywhere | move; a list wraps round at its ends |
| g, G | a list, a story | top, bottom |
| Tab | the list | Capabilities, Views, Tags, Helpers |
| Enter | a capability | read its story and about.md |
| Space | the list | unfold or fold a capability's, view's or tag's scenarios |
| h or ← | the list | up to the heading, then fold it |
| l or → | the list | unfold, then down to the first scenario, then Drop in |
| z | the list | unfold every heading on the tab, or fold them all |
| Enter | a scenario | Drop in |
| e | a scenario, a Drop in | edit the scenario in your editor, at its line |
| s | a scenario, a Drop in | edit its setup script or steps done by hand; on a scenario with none, start a script |
| r | a scenario | its runs |
| n | the list | a new scenario |
| Enter, e, n | Helpers | edit a helper at its function, or write a new one |
| / | the list | search everything |
| Enter, f | a card | done, failed |
| c | a card | copy its code |
| p, f, Enter | the checklist | pass, fail, done |
| n, a, r | the checklist | note, attach a file, Record run |
| k, s | setup that runs long | keep waiting, stop |
| s, E, A | setup that failed | edit the setup, edit or add the teardown |
| Space, b | a story | page down, page up |
| e | a story | edit about.md, or add one |
| q, Esc, h | anywhere | back, then quit (h goes back from a story or Examples rows) |

Capabilities, views and tags open folded, one line each, and take the same keys. The cursor's line is a bar across the list. A list
longer than the terminal scrolls to keep the cursor on show. On a terminal 100 columns or wider,
what's under the cursor shows beside the list; narrower, under it. The Drop in page shows the
scenario's Given steps above the checklist, so the whole scenario reads there.

## Editing

`givens edit <capability>/<scenario>`, or a file under capabilities/, opens its text in
`$VISUAL`, else `$EDITOR`, else `vi`, run through `sh -c` so `EDITOR="code -w"` works. Editors
Givens knows open at the scenario's line: +N for vi, vim, nvim, nano, emacs, micro and kak;
file:N for Sublime Text (subl), Zed and Helix; -g file:N for VS Code and its forks; -l N for
TextMate. A file that doesn't parse opens again with its problems at the top as `# givens:`
lines, which Givens takes out before saving.
